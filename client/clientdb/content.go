package clientdb

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/companyzero/bisonrelay/internal/strescape"
	"github.com/companyzero/bisonrelay/rpc"
	"github.com/companyzero/bisonrelay/zkidentity"
)

const (
	sharedContentDir      = "shared"
	sharedEveryone        = "everyone"
	contentMetaExt        = ".cr-meta"
	chunkDirSuffix        = ".chunks"
	uploadsDir            = "uploads"
	contentHashSuffix     = ".filehash"
	contentMetaHashSuffix = ".metahash"
	downloadingDir        = "downloading"
)

// chunkFile creates a directory with appropriate chunks of the source file.
// Returns the full hash of the file and final size.
func (db *DB) chunkFile(srcFile, chunkDir string) ([]rpc.FileManifest, []byte, uint64, error) {
	f, err := os.Open(srcFile)
	if err != nil {
		return nil, nil, 0, err
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return nil, nil, 0, err
	}

	fsize := uint64(fi.Size())
	chunkSize := uint64(db.cfg.ChunkSize)
	if chunkSize == 0 || chunkSize > fsize {
		chunkSize = fsize
	}

	var (
		chunks uint64
		fm     = make([]rpc.FileManifest, 0,
			(uint64(fi.Size())/chunkSize)+1)
		fHasher = sha256.New()
		size    uint64
	)

	if err := os.MkdirAll(chunkDir, 0o700); err != nil {
		return nil, nil, 0, err
	}

	buffer := make([]byte, chunkSize)
	for {
		n, err := f.Read(buffer)
		if err != nil && err != io.EOF {
			return nil, nil, 0, err
		}
		if n == 0 {
			// Done with exact sized chunk
			break
		}

		// Chunk digest
		chunk := buffer[:n]
		hash := sha256.Sum256(chunk)

		fm = append(fm, rpc.FileManifest{
			Index: chunks,
			Size:  uint64(n),
			Hash:  hash[:],
		})
		chunks++
		size += uint64(n)

		// Write chunk
		chunkFilename := filepath.Join(chunkDir,
			hex.EncodeToString(hash[:]))
		err = os.WriteFile(chunkFilename, chunk, 0o600)
		if err != nil {
			return nil, nil, 0, fmt.Errorf("unable to write chunk file: %w", err)
		}

		// Accumulate into global file hasher.
		fHasher.Write(chunk)
	}
	return fm, fHasher.Sum(nil), size, nil
}

// ShareFile registers the given file as a shared file.
//
// If uid is nil, then the file is registered as shared among all users.
func (db *DB) ShareFile(tx ReadWriteTx, fname string, uid *UserID,
	cost uint64, descr string, sign func([]byte) ([]byte, error)) (SharedFile, rpc.FileMetadata, error) {

	var f SharedFile
	var md rpc.FileMetadata

	baseName := filepath.Base(fname)
	if baseName == "" {
		return f, md, fmt.Errorf("empty basename for file %s", fname)
	}

	// First, deal with the file contents. If this file already exists,
	// it's already been shared (either globally or with someone). Verify
	// the file is actually the same as the one previously shared and error
	// if it's not.
	f.Filename = baseName
	chunksPath := filepath.Join(db.root, contentDir, baseName)
	if fileExists(chunksPath) {
		// There needs to exists a file
		// content/<baseName>/<fileHash>.fileHash, in the chunks dir,
		// otherwise the files are different.
		fileHash, err := sha256File(fname)
		if err != nil {
			return f, md, err
		}
		copy(f.FileHash[:], fileHash)
		wantFileHashFile := f.FileHash.String() + contentHashSuffix
		metaFname := filepath.Join(chunksPath, wantFileHashFile)
		if !fileExists(metaFname) {
			return f, md, fmt.Errorf("already shared a different file with name %s",
				baseName)
		}

		// The same file is being shared again (possibly to a different
		// user). Read the existing metadata.
		if err := db.readJsonFile(metaFname, &md); err != nil {
			return f, md, fmt.Errorf("unable to read existing file metadata: %v", err)
		}
	} else {
		md = rpc.FileMetadata{
			Version:     rpc.FileMetadataVersion,
			Description: descr,
			Cost:        cost,
			Filename:    baseName,
		}

		// File is being shared for the first time. Chunk the file.
		var err error
		var fhash []byte
		md.Manifest, fhash, md.Size, err = db.chunkFile(fname, chunksPath)
		if err != nil {
			return f, md, err
		}
		copy(f.FileHash[:], fhash)
		md.Hash = f.FileHash.String()

		// Sign the hash.
		sig, err := sign(fhash)
		if err != nil {
			return f, md, fmt.Errorf("unable to sign hash of file: %w", err)
		}
		md.Signature = hex.EncodeToString(sig)

		// Save the content metadata in the special file.
		metaFname := filepath.Join(chunksPath, md.Hash+contentHashSuffix)
		if err := db.saveJsonFile(metaFname, md); err != nil {
			return f, md, err
		}
	}

	// Create or update the list of people this file is shared with.
	f.FID = md.MetadataHash()
	metaMetaFname := filepath.Join(chunksPath, f.FID.String()+contentMetaHashSuffix)
	var shares []string
	thisShare := sharedEveryone
	if uid != nil {
		thisShare = uid.String()
	}
	if fileExists(metaMetaFname) {
		if err := db.readJsonFile(metaMetaFname, &shares); err != nil {
			return f, md, err
		}
		found := false
		for _, s := range shares {
			if s == thisShare {
				found = true
				break
			}
		}
		if !found {
			shares = append(shares, thisShare)
		}
	} else {
		shares = append(shares, thisShare)
	}
	if err := db.saveJsonFile(metaMetaFname, shares); err != nil {
		return f, md, err
	}

	// Now deal with the actual sharing of the file. If it's a global share,
	// put in the global share dir, otherwise put in the user's share dir.
	shareDir := sharedContentDir
	if uid != nil {
		shareDir = filepath.Join(inboundDir, uid.String(), sharedContentDir)
	}
	shareDir = filepath.Join(db.root, shareDir)
	shareFname := filepath.Join(shareDir, f.FID.String())
	if err := db.saveJsonFile(shareFname, f); err != nil {
		return f, md, err
	}

	return f, md, nil
}

// FindSharedFileID is used to find the file ID of a file shared with the given
// filename.
func (db *DB) FindSharedFileID(tx ReadTx, fname string) (FileID, error) {
	var fid FileID
	chunksPath := filepath.Join(db.root, contentDir, fname)
	files, err := filepath.Glob(chunksPath + "/*." + contentMetaHashSuffix)
	if err != nil {
		return fid, err
	}
	if len(files) == 0 {
		return fid, fmt.Errorf("ID for file %q: %w", fname, ErrNotFound)
	}
	if len(files) > 1 {
		return fid, fmt.Errorf("found %d meta files in dir for file %q",
			len(files), fname)
	}

	maybeID := files[0][:len(files[0])-len(contentMetaHashSuffix)-1]
	if err := fid.FromString(maybeID); err != nil {
		return fid, fmt.Errorf("file %q is not a valid file id", files[0])
	}

	return fid, nil
}

// Unshare the file with the given user or globally. If the file is no longer
// noted as shared with anyone, the content is removed.
func (db *DB) UnshareFile(tx ReadWriteTx, fid FileID, uid *UserID) error {
	// First, read the SharedFile metadata from the share.
	shareDir := sharedContentDir
	thisShare := sharedEveryone
	if uid != nil {
		shareDir = filepath.Join(inboundDir, uid.String(), sharedContentDir)
		thisShare = uid.String()
	}
	shareDir = filepath.Join(db.root, shareDir)
	shareFname := filepath.Join(shareDir, fid.String())

	var sf SharedFile
	if err := db.readJsonFile(shareFname, &sf); err != nil {
		return err
	}

	// Now, remove this share.
	if err := os.Remove(shareFname); err != nil {
		return err
	}

	// Remove this share from list of content shares.
	chunksPath := filepath.Join(db.root, contentDir, sf.Filename)
	metaMetaFname := filepath.Join(chunksPath, sf.FID.String()+contentMetaHashSuffix)
	if !fileExists(metaMetaFname) {
		// Shouldn't happen, but unshare was successful.
		return nil
	}

	shares := []string{}
	if err := db.readJsonFile(metaMetaFname, &shares); err != nil {
		return err
	}
	for i, s := range shares {
		if s != thisShare {
			continue
		}
		if len(shares) == 1 {
			shares = []string{}
		} else {
			shares[i] = shares[len(shares)-1]
			shares = shares[:len(shares)-1]
		}
		break
	}

	if len(shares) == 0 {
		// No more shares, remove content.
		db.log.Infof("Removing content due to no more shares: %q", sf.Filename)
		return os.RemoveAll(chunksPath)
	}

	// Still some shares. Save updated list of shares of this content.
	return db.saveJsonFile(metaMetaFname, shares)
}

// sharedFilesFromDirs goes over the specified dirs and returns all shared
// files in them.
func (db *DB) sharedFilesFromDirs(dirs []string) ([]SharedFile, error) {
	// Grab list of metadata files.
	var files []string

	for _, v := range dirs {
		dirEntries, err := os.ReadDir(v)
		if err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("unable to read content dir: %v", err)
		}
		for _, f := range dirEntries {
			skip := f.IsDir() || len(f.Name()) != 64
			if skip {
				continue
			}
			files = append(files, filepath.Join(v, f.Name()))
		}
	}

	// Read all metadata files.
	res := make([]SharedFile, len(files))
	for i, f := range files {
		err := db.readJsonFile(f, &res[i])
		if err != nil {
			return nil, err
		}
	}

	return res, nil
}

// ListSharedFile lists the files shared with a given user or shared files with
// all users.
func (db *DB) ListSharedFiles(tx ReadTx, uid *UserID) ([]rpc.FileMetadata, error) {
	shareDir := sharedContentDir
	if uid != nil {
		shareDir = filepath.Join(inboundDir, uid.String(), sharedContentDir)
	}
	shareDir = filepath.Join(db.root, shareDir)

	// List shares.
	dirs := []string{shareDir}
	shares, err := db.sharedFilesFromDirs(dirs)
	if err != nil {
		return nil, fmt.Errorf("unable to list files from dir: %v", err)
	}

	// Convert to metadata.
	res := make([]rpc.FileMetadata, 0, len(shares))
	for _, sf := range shares {
		var md rpc.FileMetadata
		chunksDir := filepath.Join(db.root, contentDir, sf.Filename)
		metaFname := filepath.Join(chunksDir, sf.FileHash.String()+contentHashSuffix)
		err := db.readJsonFile(metaFname, &md)
		if err != nil {
			return nil, fmt.Errorf("unable to read file metadata: %v", err)
		}
		res = append(res, md)
	}

	return res, nil
}

// ListAllSharedFiles lists both globally and user shared files for all files.
func (db *DB) ListAllSharedFiles(tx ReadTx) ([]SharedFileAndShares, error) {
	dir := filepath.Join(db.root, contentDir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("unable to make share dir: %v", err)
	}

	// List all .filehash files, which contains the file metadata.
	pattern := filepath.Join(dir, "*", "*"+contentHashSuffix)
	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("unable to execute glob: %v", err)
	}

	// Read each file and create the list.
	var res []SharedFileAndShares
	for _, fname := range files {
		var fm rpc.FileMetadata
		if err := db.readJsonFile(fname, &fm); err != nil {
			// Ignore errors and go to the next file.
			db.log.Warnf("Unable to read %s: %v", fname, err)
			continue
		}

		var fid FileID = fm.MetadataHash()
		var shares []string
		sharesFname := filepath.Join(filepath.Dir(fname),
			fid.String()+contentMetaHashSuffix)
		if err := db.readJsonFile(sharesFname, &shares); err != nil {
			db.log.Warnf("Unable to read shares file %s: %v", err)
			continue
		}

		var filehash FileID
		if err := filehash.FromString(fm.Hash); err != nil {
			db.log.Warnf("Hash in file %s not a valid id: %v", fname,
				err)
			continue
		}

		// Check if it was globally shared.
		global := false
		uids := make([]clientintf.ID, 0, len(shares))
		for i := range shares {
			if shares[i] == sharedEveryone {
				// Globally shared! Remove from list.
				global = true
				continue
			}

			// Shared to a user.
			var uid clientintf.ID
			if err := uid.FromString(shares[i]); err != nil {
				db.log.Warnf("Not a UID (%q) in shares file %s: %v",
					shares[i], sharesFname, err)
				continue
			}
			uids = append(uids, uid)
		}

		res = append(res, SharedFileAndShares{
			SF: SharedFile{
				FileHash: filehash,
				FID:      fid,
				Filename: fm.Filename,
			},
			Cost:   fm.Cost,
			Size:   fm.Size,
			Global: global,
			Shares: uids,
		})
	}

	return res, nil
}

// GetSharedFile returns information about the given shared file. If uid is
// nil, then it's assumed the shared file is on the global dir.
func (db *DB) GetSharedFile(tx ReadTx, uid *UserID, fid FileID) (SharedFile, rpc.FileMetadata, error) {
	shareDir := sharedContentDir
	if uid != nil {
		shareDir = filepath.Join(inboundDir, uid.String(), sharedContentDir)
	}
	metaFname := filepath.Join(db.root, shareDir, fid.String())
	var res SharedFile
	var md rpc.FileMetadata
	if err := db.readJsonFile(metaFname, &res); errors.Is(err, ErrNotFound) {
		return res, md, fmt.Errorf("shared file %s: %w", fid.String(), err)
	} else if err != nil {
		return res, md, fmt.Errorf("unable to read file metadata: %w", err)
	}

	md, err := db.fileMetadataForSharedFile(&res)
	if err != nil {
		return res, md, fmt.Errorf("unable to load file metadata of shared file: %v", err)
	}

	return res, md, nil
}

// GetSharedFileForUpload returns information about the given file if the user is allowed
// to fetch it (either by the file having been shared with the user or if the
// file is globally shared)
func (db *DB) GetSharedFileForUpload(tx ReadTx, uid UserID, fid FileID) (SharedFile, rpc.FileMetadata, error) {
	// Check if it's globally shared first.
	f, md, err := db.GetSharedFile(tx, nil, fid)
	if err == nil {
		return f, md, nil
	}

	// Not globally shared. See if shared with user.
	return db.GetSharedFile(tx, &uid, fid)
}

// readOrNewChunkUpload reads an existing or creates a new chunk upload
// structure.
func (db *DB) readOrNewChunkUpload(uid UserID, fid FileID, cid ChunkID, index int) (ChunkUpload, error) {
	fname := filepath.Join(db.root, inboundDir, uid.String(), uploadsDir,
		fid.String(), cid.String())
	var cup ChunkUpload
	err := db.readJsonFile(fname, &cup)
	if errors.Is(err, ErrNotFound) {
		cup = ChunkUpload{
			UID:   uid,
			FID:   fid,
			CID:   cid,
			Index: index,
		}
		err = nil
	}
	return cup, err
}

// saveChunkUpload saves the given chunk upload structure.
func (db *DB) saveChunkUpload(cup *ChunkUpload) error {
	fname := filepath.Join(db.root, inboundDir, cup.UID.String(), uploadsDir,
		cup.FID.String(), cup.CID.String())
	return db.saveJsonFile(fname, *cup)
}

// removeChunkUpload deletes the given chunk upload structure from the
// filesystem.  If this is the last chunk, it also removes the parent file
// upload dir.
func (db *DB) removeChunkUpload(cup *ChunkUpload) error {
	dir := filepath.Join(db.root, inboundDir, cup.UID.String(), uploadsDir,
		cup.FID.String())
	fname := filepath.Join(dir, cup.CID.String())
	if err := os.Remove(fname); err != nil {
		return err
	}

	// Remove upload dir if empty.
	if dirExistsEmpty(dir) {
		return os.Remove(dir)
	}
	return nil
}

// GetFileChunkUpload returns an existing chunk upload info.
func (db *DB) GetFileChunkUpload(tx ReadTx, uid UserID, fid FileID, cid ChunkID) (ChunkUpload, error) {
	fname := filepath.Join(db.root, inboundDir, uid.String(), uploadsDir,
		fid.String(), cid.String())
	var cup ChunkUpload
	err := db.readJsonFile(fname, &cup)
	return cup, err
}

// AddChunkUploadInvoice registers the given invoice as one intended to pay for
// the upload of a file chunk.
func (db *DB) AddChunkUploadInvoice(tx ReadWriteTx, uid UserID, fid FileID,
	cid ChunkID, index int, invoice string) error {

	cup, err := db.readOrNewChunkUpload(uid, fid, cid, index)
	if err != nil {
		return err
	}

	cup.Invoices = append(cup.Invoices, invoice)
	cup.State = ChunkStateHasInvoice
	return db.saveChunkUpload(&cup)
}

// MarkChunkUploadInvoiceExpired registers the given invoice as expired and thus
// unusable to pay for a chunk upload.
func (db *DB) MarkChunkUploadInvoiceExpired(tx ReadWriteTx, uid UserID, fid FileID,
	cid ChunkID, index int, invoice string) error {

	cup, err := db.readOrNewChunkUpload(uid, fid, cid, index)
	if err != nil {
		return err
	}

	// Drop this invoice from list of outstanding invoices.
	for i, inv := range cup.Invoices {
		if inv != invoice {
			continue
		}
		l := len(cup.Invoices)
		cup.Invoices[i] = cup.Invoices[l-1]
		cup.Invoices = cup.Invoices[:l-1]
	}

	// Remove chunk upload if no more uploads exist for it.
	if cup.Paid <= 0 && len(cup.Invoices) == 0 {
		return db.removeChunkUpload(&cup)
	}

	return db.saveChunkUpload(&cup)
}

// MarkChunkUploadInvoiceSent registers the given upload chunk as having had its
// latest invoice sent to the remote user.
func (db *DB) MarkChunkUploadInvoiceSent(tx ReadWriteTx, uid UserID, fid FileID,
	cid ChunkID, index int) error {

	cup, err := db.readOrNewChunkUpload(uid, fid, cid, index)
	if err != nil {
		return err
	}

	cup.State = ChunkStateSentInvoice
	return db.saveChunkUpload(&cup)
}

// MarkChunkUploadPaid registers the given invoice as having been paid for
// the given chunk upload.
func (db *DB) MarkChunkUploadPaid(tx ReadWriteTx, uid UserID, fid FileID,
	cid ChunkID, index int, invoice string) error {
	cup, err := db.readOrNewChunkUpload(uid, fid, cid, index)
	if err != nil {
		return err
	}

	// Drop this invoice from list of outstanding invoices.
	for i, inv := range cup.Invoices {
		if inv != invoice {
			continue
		}
		l := len(cup.Invoices)
		cup.Invoices[i] = cup.Invoices[l-1]
		cup.Invoices = cup.Invoices[:l-1]
	}

	// Inc count of paid invoices for this chunk.
	cup.Paid += 1
	return db.saveChunkUpload(&cup)
}

// MarkChunkUploadSent registers the given upload as having been sent to the
// remote user.
func (db *DB) MarkChunkUploadSent(tx ReadWriteTx, uid UserID, fid FileID,
	cid ChunkID, index int) error {

	cup, err := db.readOrNewChunkUpload(uid, fid, cid, index)
	if err != nil {
		return err
	}

	// Dec count of paid invoices for this chunk.
	cup.Paid -= 1

	// Remove chunk upload if no more uploads exist for it.
	if cup.Paid <= 0 && len(cup.Invoices) == 0 {
		return db.removeChunkUpload(&cup)
	}
	return db.saveChunkUpload(&cup)
}

// fileMetadataForSharedFile returns the corresponding FileMetadata info of the
// given shared file.
func (db *DB) fileMetadataForSharedFile(sf *SharedFile) (rpc.FileMetadata, error) {
	chunksPath := filepath.Join(db.root, contentDir, sf.Filename)
	metaFname := filepath.Join(chunksPath, sf.FileHash.String()+contentHashSuffix)
	var md rpc.FileMetadata
	err := db.readJsonFile(metaFname, &md)
	return md, err
}

// GetSharedFileChunkData returns the actual chunk data for a given shared file.
func (db *DB) GetSharedFileChunkData(tx ReadTx, sf *SharedFile, chunkIdx int) ([]byte, error) {
	md, err := db.fileMetadataForSharedFile(sf)
	if err != nil {
		return nil, err
	}
	if chunkIdx >= len(md.Manifest) {
		return nil, fmt.Errorf("chunkIdx %d > len(chunks) %d",
			chunkIdx, len(md.Manifest))
	}
	chunkHash := hex.EncodeToString(md.Manifest[chunkIdx].Hash)
	chunksPath := filepath.Join(db.root, contentDir, sf.Filename)
	chunkFname := filepath.Join(chunksPath, chunkHash)
	return os.ReadFile(chunkFname)
}

func (db *DB) ListOutstandingUploads(tx ReadTx) ([]ChunkUpload, error) {
	// db/inbound/<userid>/uploads/<fid>/<cid>
	pattern := filepath.Join(db.root, inboundDir, "*", uploadsDir, "*", "*")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}

	res := make([]ChunkUpload, 0, len(files))
	for _, fname := range files {
		var cup ChunkUpload
		err := db.readJsonFile(fname, &cup)
		if err != nil {
			db.log.Warnf("Unable to decode %s as a file upload: %v",
				fname, err)
			continue
		}

		res = append(res, cup)
	}

	// Return sorted chunks to bias towards sending senquentially.
	sort.Slice(res, func(i, j int) bool {
		if res[i].UID.Less(&res[j].UID) {
			return true
		}
		if res[i].FID.Less(&res[j].FID) {
			return true
		}
		return res[i].Index < res[j].Index
	})

	return res, nil
}

func (db *DB) StartFileDownload(tx ReadWriteTx, uid UserID, fid FileID, isSentFile bool) (FileDownload, error) {
	var fd FileDownload

	// Save download metadata.
	diskDir := filepath.Join(db.root, downloadingDir)
	metaPath := filepath.Join(diskDir, fid.String()+contentMetaExt)
	fd = FileDownload{
		UID:        uid,
		FID:        fid,
		IsSentFile: isSentFile,
	}
	if err := db.saveJsonFile(metaPath, fd); err != nil {
		return fd, err
	}

	return fd, nil
}

func (db *DB) ReadFileDownload(tx ReadTx, uid UserID, fid FileID) (FileDownload, error) {
	var fd FileDownload

	diskDir := filepath.Join(db.root, downloadingDir)
	metaPath := filepath.Join(diskDir, fid.String()+contentMetaExt)
	if err := db.readJsonFile(metaPath, &fd); err != nil {
		return fd, err
	}

	// TODO: support downloading from multiple users at the same time?
	if fd.UID != uid {
		return fd, fmt.Errorf("specified user not the download user")
	}
	return fd, nil
}

// CancelFileDownload removes the in-progress download from the DB.
func (db *DB) CancelFileDownload(tx ReadWriteTx, fid FileID) error {
	diskDir := filepath.Join(db.root, downloadingDir)
	metaPath := filepath.Join(diskDir, fid.String()+contentMetaExt)
	chunkDir := filepath.Join(diskDir, fid.String()+chunkDirSuffix)

	err := os.Remove(metaPath)
	if os.IsNotExist(err) {
		return fmt.Errorf("download of file %s: %v", fid, ErrNotFound)
	}
	if err != nil {
		return err
	}

	// Ignore errors when removing chunk dir since we've already removed the
	// metadata file.
	os.RemoveAll(chunkDir)
	return nil
}

func (db *DB) UpdateFileDownloadMetadata(tx ReadWriteTx, fd *FileDownload,
	md rpc.FileMetadata) error {

	if fd.Metadata != nil {
		return fmt.Errorf("cannot update file metadata: metadata already filled")
	}
	fd.Metadata = &md

	diskDir := filepath.Join(db.root, downloadingDir)
	metaPath := filepath.Join(diskDir, fd.FID.String()+contentMetaExt)
	if err := db.saveJsonFile(metaPath, *fd); err != nil {
		return err
	}

	return nil
}

func (db *DB) ReplaceFileDownloadInvoices(tx ReadWriteTx, fd *FileDownload,
	invoices map[int]string) error {

	if fd.Invoices == nil {
		fd.Invoices = make(map[int]string, len(invoices))
	}

	if fd.ChunkStates == nil {
		fd.ChunkStates = make(map[int]ChunkState, len(invoices))
	}
	if fd.ChunkUpdatedTime == nil {
		fd.ChunkUpdatedTime = make(map[int]time.Time)
	}

	for i, newInv := range invoices {
		if newInv == "" {
			delete(fd.Invoices, i)
			fd.ChunkStates[i] = ""
		} else {
			fd.Invoices[i] = newInv
			fd.ChunkStates[i] = ChunkStateHasInvoice
		}
		fd.ChunkUpdatedTime[i] = time.Now()
	}

	diskDir := filepath.Join(db.root, downloadingDir)
	metaPath := filepath.Join(diskDir, fd.FID.String()+contentMetaExt)
	return db.saveJsonFile(metaPath, fd)
}

func (db *DB) ReplaceFileDownloadChunkState(tx ReadWriteTx, fd *FileDownload,
	chunkIdx int, chunkState ChunkState) error {

	if fd.ChunkStates == nil {
		fd.ChunkStates = make(map[int]ChunkState)
	}
	fd.ChunkStates[chunkIdx] = chunkState

	if fd.ChunkUpdatedTime == nil {
		fd.ChunkUpdatedTime = make(map[int]time.Time)
	}
	fd.ChunkUpdatedTime[chunkIdx] = time.Now()

	diskDir := filepath.Join(db.root, downloadingDir)
	metaPath := filepath.Join(diskDir, fd.FID.String()+contentMetaExt)
	return db.saveJsonFile(metaPath, fd)
}

func (db *DB) SaveFileDownloadChunk(tx ReadWriteTx, user string, fd *FileDownload,
	chunkIdx int, data []byte) (string, error) {

	// Hash the chunk.
	hasher := sha256.New()
	hasher.Write(data)
	hash := hasher.Sum(nil)
	hashStr := hex.EncodeToString(hash)

	if fd.Metadata == nil {
		return "", fmt.Errorf("file metadata is nil")
	}

	// Verify chunk index is correct.
	if !clientintf.ChunkIndexMatches(fd.Metadata, chunkIdx, hash[:]) {
		return "", fmt.Errorf("data does not hash to specified chunk index")
	}

	// Save the chunk.
	diskDir := filepath.Join(db.root, downloadingDir)
	chunkDir := filepath.Join(diskDir, fd.FID.String()+chunkDirSuffix)
	chunkPath := filepath.Join(chunkDir, hashStr)
	if err := os.MkdirAll(chunkDir, 0o700); err != nil {
		return "", err
	}
	if err := os.WriteFile(chunkPath, data, 0o600); err != nil {
		return "", err
	}

	// Update chunk state and save metadata.
	if err := db.ReplaceFileDownloadChunkState(tx, fd, chunkIdx, ChunkStateDownloaded); err != nil {
		return "", err
	}

	// If not all chunks have been downloaded, keep going.
	if len(db.MissingFileDownloadChunks(tx, fd)) != 0 {
		return "", nil
	}

	// Assemble final file. First: figure out final name.
	baseDestFileName := filepath.Join(db.downloadsDir, strescape.PathElement(user),
		strescape.PathElement(fd.Metadata.Filename))
	destFileName := baseDestFileName
	ext := filepath.Ext(baseDestFileName)
	if len(ext) > 0 {
		baseDestFileName = baseDestFileName[:len(baseDestFileName)-len(ext)]
	}
	for i := 1; fileExists(destFileName); i++ {
		destFileName = fmt.Sprintf("%s_%.2d%s", baseDestFileName, i, ext)
	}
	if err := os.MkdirAll(filepath.Dir(destFileName), 0o700); err != nil {
		return "", err
	}
	destFile, err := os.Create(destFileName)
	if err != nil {
		return "", err
	}

	defer destFile.Close()

	// Next: Copy over chunks, while accumulating final hash.
	hasher = sha256.New()
	for _, ch := range fd.Metadata.Manifest {
		chunkFname := filepath.Join(chunkDir, hex.EncodeToString(ch.Hash))
		data, err := os.ReadFile(chunkFname)
		if err != nil {
			return "", err
		}
		hasher.Write(data)
		if _, err := destFile.Write(data); err != nil {
			return "", err
		}
	}

	// Ensure final file hash is correct.
	hash = hasher.Sum(nil)
	hashStr = hex.EncodeToString(hash)
	if hashStr != fd.Metadata.Hash {
		return "", fmt.Errorf("unexpected final file hash (got %s, want %s)",
			hashStr, fd.Metadata.Hash)
	}
	fd.CompletedName = filepath.Base(destFileName)
	metaPath := filepath.Join(diskDir, fd.FID.String()+contentMetaExt)
	if err := db.saveJsonFile(metaPath, fd); err != nil {
		return "", err
	}

	// Finally, clean up the chunks.
	if err := os.RemoveAll(chunkDir); err != nil {
		db.log.Errorf("Unable to remove chunk dir of completed download: %v", err)
	}

	return destFileName, nil
}

func (db *DB) MissingFileDownloadChunks(tx ReadTx, fd *FileDownload) []int {
	if fd.Metadata == nil {
		return nil
	}

	diskDir := filepath.Join(db.root, downloadingDir)
	chunkDir := filepath.Join(diskDir, fd.FID.String()+chunkDirSuffix)
	files, _ := os.ReadDir(chunkDir) // Safe to ignore error

	// Aux map to know which files exist in chunk dir.
	filesMap := make(map[string]struct{}, len(files))
	for _, f := range files {
		filesMap[f.Name()] = struct{}{}
	}

	// Verify which chunk files already exist.
	res := make([]int, 0, len(fd.Metadata.Manifest)-len(filesMap))
	for i, ch := range fd.Metadata.Manifest {
		if _, ok := filesMap[hex.EncodeToString(ch.Hash)]; !ok {
			res = append(res, i)
		}
	}
	return res
}

func (db *DB) HasDownloadedFile(tx ReadTx, fid zkidentity.ShortID) (bool, error) {
	downDir := filepath.Join(db.root, downloadingDir)
	metaFname := filepath.Join(downDir, fid.String()+contentMetaExt)
	if !fileExists(metaFname) {
		return false, nil
	}
	var fd FileDownload
	if err := db.readJsonFile(metaFname, &fd); err != nil {
		return false, err
	}

	return fd.CompletedName != "", nil
}

// HasDownloadedFiles converts the given list of file metadata (possibly
// received) from a remote user into a list of files that we may have already
// downloaded.
func (db *DB) HasDownloadedFiles(tx ReadTx, user string, uid UserID, files []rpc.FileMetadata) ([]RemoteFile, error) {

	downDir := filepath.Join(db.root, downloadingDir)

	res := make([]RemoteFile, len(files))
	for i, m := range files {
		res[i] = RemoteFile{
			Metadata: m,
			FID:      m.MetadataHash(),
			UID:      uid,
		}

		metaFname := filepath.Join(downDir, res[i].FID.String()+contentMetaExt)
		if !fileExists(metaFname) {
			continue
		}

		var fd FileDownload
		if err := db.readJsonFile(metaFname, &fd); err != nil {
			return nil, err
		}
		res[i].UID = fd.UID
		if fd.CompletedName != "" {
			res[i].DiskPath = filepath.Join(db.downloadsDir,
				strescape.PathElement(user), fd.CompletedName)
		}
	}

	return res, nil
}

func (db *DB) ListOutstandingDownloads(tx ReadTx) ([]FileDownload, error) {
	diskDir := filepath.Join(db.root, downloadingDir)

	pattern := diskDir + "/*" + contentMetaExt
	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}

	res := make([]FileDownload, 0, len(files))
	for _, fname := range files {
		var fd FileDownload
		err := db.readJsonFile(fname, &fd)
		if err != nil {
			db.log.Warnf("Unable to decode %s as a file download: %v",
				fname, err)
			continue
		}

		if fd.CompletedName != "" {
			// Already completed this download.
			continue
		}

		res = append(res, fd)
	}

	return res, nil
}
