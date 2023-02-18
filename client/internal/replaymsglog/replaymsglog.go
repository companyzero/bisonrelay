package replaymsglog

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/decred/slog"
)

// ID is the ID of a message stored in a log. It is monotonically increasing,
// except when the log is entirely cleared (in which case, the set of IDs is
// reset).
type ID uint64

func (id ID) fileIndex() uint32 { return uint32(id >> 32) }
func (id ID) endOffset() uint32 { return uint32(id) }
func (id ID) String() string {
	return fmt.Sprintf("%06x/%08x", id.fileIndex(), id.endOffset())
}
func makeID(fileIndex uint32, endOffset uint32) ID {
	return ID(fileIndex)<<32 | ID(endOffset)
}

// fileID is a helper to log the fileID in a prettier way.
type fileID uint32

func (id fileID) String() string {
	return fmt.Sprintf("%06x", uint32(id))
}
func makeFileID(fileIndex uint32) fileID {
	return fileID(fileIndex)
}

// Config is the configuration needed to initialize a Log object.
type Config struct {
	RootDir string
	Prefix  string
	MaxSize uint32
	Log     slog.Logger
}

// logFile is an individual log file.
type logFile struct {
	f        *os.File
	fid      uint32
	fileName string
	log      slog.Logger

	mtx             sync.Mutex
	clearedToOffset int64
	writtenToOffset int64
}

// clearBuffer is the buffer used to clear up data.
var clearBuffer = bytes.Repeat([]byte{' '}, 1<<14) // 1<<14 == 16KiB

// clearUpTo clears data in the file, from clearedUpToOffset to targetOffset.
func (lf *logFile) clearUpTo(targetOffset int64) error {
	lf.mtx.Lock()
	defer lf.mtx.Unlock()

	if lf.f == nil {
		// Consider already cleared.
		return nil
	}

	if targetOffset > lf.writtenToOffset {
		lf.log.Warnf("replaymsglog logfile %s clearing past writtenToOffset: "+
			"clear target: %d, writtenToOffset: %d", lf.fileName, targetOffset,
			lf.writtenToOffset)
	}

	// Clear in len(clearBuffer) blocks.
	nBytes := targetOffset - lf.clearedToOffset
	n := int64(len(clearBuffer))
	if _, err := lf.f.Seek(lf.clearedToOffset, 0); err != nil {
		return err
	}
	for nBytes > 0 {
		if nBytes < n {
			n = nBytes
		}
		_, err := lf.f.Write(clearBuffer[:n])
		if err != nil {
			return err
		}
		nBytes -= n
	}
	lf.clearedToOffset = targetOffset
	return nil
}

// append appends the byte slice to the file.
func (lf *logFile) append(b []byte) error {
	lf.mtx.Lock()
	defer lf.mtx.Unlock()

	if lf.f == nil {
		return fmt.Errorf("file already closed to append")
	}

	// Seek to end of file to append.
	off, err := lf.f.Seek(0, 2)
	if err != nil {
		return err
	}
	if off != lf.writtenToOffset {
		// Usage error or filesystem error.
		lf.log.Warnf("replaymsglog logfile %s invalid seek assertion: "+
			"offset %d != old offset %d", lf.fileName, off,
			lf.writtenToOffset)
	}
	if _, err := lf.f.Write(b); err != nil {
		return err
	}
	lf.writtenToOffset = off + int64(len(b))
	return nil
}

// readNextMessage reads the next message after startOffset into buf. Returns
// the offset after the end of the message.
func (lf *logFile) readNextMessage(startOffset int64, buf *bytes.Buffer) (int64, error) {
	lf.mtx.Lock()
	defer lf.mtx.Unlock()

	if lf.f == nil {
		return 0, io.EOF
	}

	offset := startOffset

	var b [4096]byte
	_, err := lf.f.Seek(offset, 0)
	if err != nil {
		return 0, err
	}
	for {
		n, err := lf.f.Read(b[:])
		if err != nil {
			return 0, err
		}

		for i := 0; i < n; i++ {
			if b[i] == '\n' {
				offset += int64(i + 1)
				buf.Write(b[:i])
				return offset, nil
			}
		}
		buf.Write(b[:])
		offset += int64(n + 1)
	}
}

// iterateMessages iterates over the messages in the logfile, starting at the
// passed offset.
func (lf *logFile) iterateMessages(startOffset int64, f func(b []byte, id ID) error) error {
	offset := startOffset
	buf := bytes.NewBuffer(nil)
	for {
		var err error
		offset, err = lf.readNextMessage(offset, buf)
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		id := makeID(lf.fid, uint32(offset))
		f(buf.Bytes(), id)
		buf.Reset()
	}
}

// close the log file.
func (lf *logFile) close() error {
	lf.mtx.Lock()
	err := lf.f.Close()
	lf.f = nil
	lf.mtx.Unlock()
	return err
}

// Log is a replay message log that stores messages (encoded as json objects)
// sequentially in a set of files.
//
// Once messages are ack'd (by calling ClearUpTo), they are removed from the
// log.
//
// One log instance (with a specific config) is meant to store only a single
// type of message.
//
// The log functions are safe for concurrent access.
type Log struct {
	cfg         Config
	log         slog.Logger
	maxFileSize int64

	mtx       sync.Mutex
	fileIndex uint32
	current   *logFile

	// oldFiles tracks all existing files with unacked data.
	oldFiles map[uint32]*logFile
}

// rotateFile create a new logging file and sets it as the current file being
// written to.
//
// Must be called with the mutex held.
func (l *Log) rotateFile() error {
	l.fileIndex += 1
	fileName := fmt.Sprintf("%s_%08x", l.cfg.Prefix, l.fileIndex)
	filePath := filepath.Join(l.cfg.RootDir, fileName)
	f, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o0600)
	if err != nil {
		return fmt.Errorf("unable to open new replaymsglog file: %v", err)
	}

	l.current = &logFile{f: f, fileName: fileName, fid: l.fileIndex, log: l.log}
	l.oldFiles[l.fileIndex] = l.current
	l.log.Debugf("Rotating replaymsglog to new file %s (FID %s)", filePath,
		makeFileID(l.fileIndex))

	return nil
}

// writingRequiresRotation returns true if the current file requires rotation
// to be written lenToWrite bytes.
//
// Must be called with the mutex held.
func (l *Log) writingRequiresRotation(lenToWrite int64) bool {
	if l.current == nil {
		return true
	}

	return l.current.writtenToOffset+lenToWrite > l.maxFileSize
}

// Store stores the given value in the log, as a JSON object. It returns the ID
// under which the message was stored, and which can be used to delete messages
// up to this one in a future call to ClearUpTo.
func (l *Log) Store(v interface{}) (ID, error) {
	var id ID
	data, err := json.Marshal(v)
	if err != nil {
		return id, fmt.Errorf("unable to encode %T to store in replaylog: %v", v, err)
	}
	data = append(data, '\n')

	// Figure out the file to write to.
	l.mtx.Lock()
	writeLen := int64(len(data))
	if l.writingRequiresRotation(writeLen) {
		if err := l.rotateFile(); err != nil {
			l.mtx.Unlock()
			return id, err
		}
	}
	lf := l.current
	l.mtx.Unlock()

	// Write to the target file.
	if err := lf.append(data); err != nil {
		return id, err
	}

	// Figure out the ID.
	fid := l.fileIndex
	endOffset := l.current.writtenToOffset
	id = makeID(fid, uint32(endOffset))
	l.log.Debugf("Stored message %s (len %d) with prefix %s", id, writeLen,
		l.cfg.Prefix)
	return id, nil
}

// removeFile removes the given file (which is assumed to be empty).
//
// Must be called with the mutex held.
func (l *Log) removeFile(fid uint32) error {
	lf := l.oldFiles[fid]
	if lf == nil {
		return fmt.Errorf("cannot remove inexisted fid %s_%d",
			l.cfg.Prefix, fid)
	}

	// Remove entire file.
	if err := lf.close(); err != nil {
		return err
	}
	err := os.Remove(filepath.Join(l.cfg.RootDir, lf.fileName))
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	delete(l.oldFiles, fid)
	if l.current == lf {
		l.current = nil
		l.fileIndex = 0
	}
	return nil
}

// ClearUpTo removes all existing log entries up to (and including) the passed
// target ID.
func (l *Log) ClearUpTo(target ID) error {
	targetFID := target.fileIndex()

	// Remove entire files under the mutex. The final file will be cleared
	// outside the mutex.
	l.mtx.Lock()

	// Clear in a sorted manner, so that if this fails, it maintains the
	// invariant of having cleared old files first.
	oldIDs := make([]uint32, 0, len(l.oldFiles))
	for id := range l.oldFiles {
		if id > targetFID {
			l.log.Tracef("Skipping file %s_%s while clearing up to ID %s",
				l.cfg.Prefix, makeFileID(id), target)
		} else {
			oldIDs = append(oldIDs, id)
		}
	}
	sort.Slice(oldIDs, func(i, j int) bool { return oldIDs[i] < oldIDs[j] })

	for _, id := range oldIDs {
		lf := l.oldFiles[id]

		if id < targetFID {
			if err := l.removeFile(id); err != nil {
				l.mtx.Unlock()
				return err
			}
			l.log.Debugf("Removed file %s_%s while clearing up to ID %s",
				l.cfg.Prefix, makeFileID(id), target)
			continue
		}

		// At this point, we cleared all files with id < targetID.
		// Unlock the list of files, as clearUpTo will acquire a
		// file-local lock.
		l.mtx.Unlock()

		// Clear target file up to the offset.
		if err := lf.clearUpTo(int64(target.endOffset())); err != nil {
			return err
		}

		l.log.Debugf("Cleared target file with prefix %s up to ID %s",
			l.cfg.Prefix, target)
		return nil
	}

	// We can reach this point if there isn't a file with fid == targetFID.
	l.mtx.Unlock()
	return nil
}

// ReadAfter reads all entries stored after (i.e. NOT including) the passed
// fromID, calling the passed function for each entry.
//
// If needed, it's the responsibility of the f function to reset or copy the
// contents of the m object.
//
// Reads from this function may interact with calls to Store() such that it is
// undefined if a call to Store() concurrent to a call to ReadAfter will cause
// the message to be generated.
func (l *Log) ReadAfter(fromID ID, m interface{}, f func(ID) error) error {
	targetFID := fromID.fileIndex()

	l.mtx.Lock()
	lastFID := l.fileIndex
	l.mtx.Unlock()

	for fid := fromID.fileIndex(); fid <= lastFID; fid++ {
		l.mtx.Lock()
		lf, ok := l.oldFiles[fid]
		lastFID = l.fileIndex
		l.mtx.Unlock()

		if !ok {
			// Doesn't have any data for this fid.
			l.log.Tracef("Log file %s does not exist when reading after %s",
				makeFileID(fid), fromID)
			continue
		}

		var startReadOffset int64
		if fid == targetFID {
			startReadOffset = int64(fromID.endOffset())
		}

		l.log.Debugf("Reading entries from file %s", makeID(fid, uint32(startReadOffset)))

		err := lf.iterateMessages(startReadOffset, func(b []byte, id ID) error {
			if err := json.Unmarshal(b, m); err != nil {
				return err
			}
			l.log.Tracef("Read entry %s while reading after target %s",
				id, fromID)
			return f(id)
		})
		if err != nil {
			return err
		}
	}

	l.log.Debugf("Read all entries of prefix %s after id %s", l.cfg.Prefix, fromID)

	return nil
}

// New creates a new message replay log.
func New(cfg Config) (*Log, error) {
	if cfg.MaxSize == 0 {
		return nil, fmt.Errorf("MaxSize cannot be zero")
	}

	log := slog.Disabled
	if cfg.Log != nil {
		log = cfg.Log
	}

	// Create dir.
	if err := os.MkdirAll(cfg.RootDir, 0o700); err != nil {
		return nil, err
	}

	// Read existing entries.
	var totalOldSize int64
	oldFiles := make(map[uint32]*logFile)
	files, err := os.ReadDir(cfg.RootDir)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("unable to list dir %s: %v", cfg.RootDir, err)
	}
	var lastFID int64
	for _, entry := range files {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return nil, err
		}

		fname := info.Name()
		if !strings.HasPrefix(fname, cfg.Prefix+"_") {
			continue
		}

		strFid := fname[len(cfg.Prefix)+1:]
		fid, err := strconv.ParseInt(strFid, 16, 32)
		if err != nil {
			return nil, fmt.Errorf("filename %s does not contain valid FID for prefix %s: %v", fname, cfg.Prefix, err)
		}

		if fid > lastFID {
			lastFID = fid
		}

		fullFname := filepath.Join(cfg.RootDir, fname)
		f, err := os.OpenFile(fullFname, os.O_RDWR, 0o600)
		if err != nil {
			return nil, err
		}
		lf := &logFile{
			fid:             uint32(fid),
			f:               f,
			fileName:        fname,
			writtenToOffset: info.Size(),
			log:             log,
		}
		oldFiles[uint32(fid)] = lf

		log.Debugf("Found log file %s with FID %s (size %d)", fname,
			makeFileID(uint32(fid)), lf.writtenToOffset)
		totalOldSize += lf.writtenToOffset
	}

	log.Infof("Opened %d replay log files with prefix %s, total size %d, last FID %s",
		len(oldFiles), cfg.Prefix, totalOldSize, makeFileID(uint32(lastFID)))

	current := oldFiles[uint32(lastFID)]

	l := &Log{
		cfg:         cfg,
		log:         log,
		maxFileSize: int64(cfg.MaxSize),
		oldFiles:    oldFiles,
		fileIndex:   uint32(lastFID),
		current:     current,
	}

	return l, nil
}
