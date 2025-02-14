package clientintf

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"math"
	"regexp"
	"strings"

	"github.com/companyzero/bisonrelay/ratchet"
	"github.com/companyzero/bisonrelay/rpc"
	"github.com/companyzero/bisonrelay/zkidentity"
)

// firstLineRE is a regexp to select the first non-empty line.
var firstLineRE = regexp.MustCompile(`(?m)^[\s]*(.+)$`)

// nonEmptyString returns true if has non-space and non-newline chars.
func nonEmptyString(s string) bool {
	for _, c := range s {
		switch c {
		case '\n', '\r', '\t', ' ', '\f':
		default:
			return true
		}
	}
	return false
}

// PostTitle returns a suggested title for a given post. It fetches from the
// "title" attribute (if it exists) or from the first non-empty line of the
// main post content.
func PostTitle(pm *rpc.PostMetadata) string {
	var title string
	if v, ok := pm.Attributes[rpc.RMPTitle]; ok && nonEmptyString(v) {
		title = v
	} else if v, ok := pm.Attributes[rpc.RMPMain]; ok && nonEmptyString(v) {
		title = v
	}

	// Cannonicalize \r as \n.
	title = strings.Replace(title, "\r", "\n", -1)

	// Limit to first non-empty line.
	subs := firstLineRE.FindStringSubmatch(title)
	if len(subs) < 2 {
		return strings.TrimSpace(title)
	}
	return strings.TrimSpace(subs[1])
}

// ChunkIndexMatches returns true if the hash of the manifest file at the
// specified index matches the given hash.
func ChunkIndexMatches(fm *rpc.FileMetadata, index int, hash []byte) bool {
	if fm == nil {
		return false
	}
	if len(fm.Manifest) <= index {
		return false
	}
	return bytes.Equal(fm.Manifest[index].Hash, hash)
}

// FileChunkMAtoms returns the cost to download the specified chunk from the
// file.
func FileChunkMAtoms(chunkIdx int, fm *rpc.FileMetadata) uint64 {
	if chunkIdx >= len(fm.Manifest) {
		return 0
	}
	chunkSize := fm.Manifest[chunkIdx].Size
	if chunkSize < 0 {
		return 0
	}
	fileSize := fm.Size
	if fileSize < 0 {
		return 0
	}
	matoms := fm.Cost * chunkSize * 1000 / fileSize
	if matoms < 1000 {
		return 1000
	}
	return matoms
}

// dummySigner dummy signer used for estimation.
func dummySigner(message []byte) zkidentity.FixedSizeSignature {
	return zkidentity.FixedSizeSignature{}
}

// Returns the estimate cost (in milliatoms) to upload a file of the given size
// to a remote user. The feeRate must be specified in milliatoms/byte.
func EstimateUploadCost(size int64, policy *ServerPolicy) (uint64, error) {
	if size <= 0 {
		return 0, fmt.Errorf("size cannot be <= 0")
	}

	if policy.PushPayRateMAtoms == 0 {
		return 0, fmt.Errorf("fee rate cannot be 0")
	}
	if policy.PushPayRateBytes == 0 {
		return 0, fmt.Errorf("fee rate bytes cannot be 0")
	}

	// Estimate number of chunks.
	maxChunkSize := int64(policy.MaxPayloadSize())
	nbChunks := int(size / maxChunkSize)
	lastChunkUneven := size%maxChunkSize > 0
	if lastChunkUneven {
		nbChunks += 1
	}

	// Random data for some fields.
	randData := make([]byte, 726+32*nbChunks)
	if _, err := rand.Read(randData); err != nil {
		return 0, err
	}
	randBts := func(size int) []byte {
		res := randData[:size]
		randData = randData[size:]
		return res
	}
	randStr := func(size int) string {
		return string(randBts(size))
	}

	// The way we estimate the cost here is to figure out the size of each
	// msg related to uploading a chunk of the file, then summing it all up
	// and applying the fee rate.
	//
	// This requires generating some dummy messages and encoding them.
	var totalSize uint64

	// Use a medium level compression level for the estimate.
	const compressLevel = 4

	// The file upload request will require 1 FTGetReply.
	ftGetReply := rpc.RMFTGetReply{
		Tag: math.MaxUint32,
		Metadata: rpc.FileMetadata{
			Version:   1000,
			Cost:      10000000,
			Size:      uint64(size),
			Directory: randStr(10),
			Filename:  randStr(20),
			Hash:      randStr(64),
			Signature: randStr(256),
			Manifest:  make([]rpc.FileManifest, nbChunks),
		},
	}
	for i := 0; i < nbChunks; i++ {
		ftGetReply.Metadata.Manifest[i] = rpc.FileManifest{
			Index: uint64(i),
			Size:  uint64(maxChunkSize),
			Hash:  randBts(32),
		}
	}
	rm, err := rpc.ComposeCompressedRM(dummySigner, ftGetReply, compressLevel)
	if err != nil {
		return 0, err
	}
	rmSize := uint64(ratchet.EncryptedSize(len(rm)))
	totalSize += rmSize

	// Each chunk will require 1 FTPayForChunk.
	ftPayForChunk := rpc.RMFTPayForChunk{
		FileID:  UserID{}.String(),
		Tag:     math.MaxUint32,
		Invoice: randStr(280),
		Index:   nbChunks,
		Hash:    randBts(32),
	}
	rm, err = rpc.ComposeCompressedRM(dummySigner, ftPayForChunk, compressLevel)
	if err != nil {
		return 0, err
	}
	rmSize = uint64(ratchet.EncryptedSize(len(rm)))
	totalSize += rmSize * uint64(nbChunks)

	// Get some random data for our chunk estimate.
	chunk := make([]byte, maxChunkSize)
	if _, err := rand.Read(chunk[:]); err != nil {
		return 0, err
	}

	// Each chunk will require 1 FTGetChunkReply (with the last one being
	// smaller than the max chunk size). We'll start with the last one.
	ftGetChunkReply := rpc.RMFTGetChunkReply{
		FileID: randStr(64),
		Index:  nbChunks,
		Chunk:  chunk[:size%maxChunkSize],
		Tag:    math.MaxUint32,
	}
	if !lastChunkUneven {
		ftGetChunkReply.Chunk = chunk
	}
	rm, err = rpc.ComposeCompressedRM(dummySigner, ftGetChunkReply, compressLevel)
	if err != nil {
		return 0, err
	}
	rmSize = uint64(ratchet.EncryptedSize(len(rm)))
	totalSize += rmSize

	// Finally, if there were more chunks before the last one, add them up.
	if nbChunks > 0 {
		ftGetChunkReply.Chunk = chunk
		rm, err = rpc.ComposeCompressedRM(dummySigner, ftGetChunkReply, compressLevel)
		if err != nil {
			return 0, err
		}
		rmSize = uint64(ratchet.EncryptedSize(len(rm)))
		totalSize += rmSize * uint64(nbChunks-1)
	}

	// Calculate cost based on total size.
	cost, err := policy.CalcPushCostMAtoms(int(totalSize))
	return uint64(cost), err
}

// EstimatePostSize estimates the final size of a post share message, given the
// specified contents of the post.
func EstimatePostSize(content, descr string) (uint64, error) {
	// The final size of the message necessary to share the given post with
	// a remote user is determined by creating a dummy post RM, then encoding
	// it to account for all the overheads.

	// Use a medium level compression level for the estimate.
	const compressLevel = 4

	// Constant, fixed-size fields of the estimate.
	const statusFrom = "ead8d8303f3aa727d403add3b6ecc83a20269a1e1babebc35bba7b044f7dbe07"
	const fromNick = "a longish nick to account for most user's nicks"
	const signature = "91791ab420c36ab9f1260b1064692db08fe61b84825cabefcdb1cbbb93e473fd3b5c92c7bfe8af59f0e98bf669ba3c43f3f846c0553fdb29d01cc10c2aa88c0a"
	const identifier = "eb872a22891b1fc3f2a1ddc4ec08c0f99eea510d0387237192a39d61e614e2a4"

	// Create the dummy RM with the fields needed to share a post of the
	// specified size.
	rmps := rpc.RMPostShare{
		Version: 99999,
		Attributes: map[string]string{
			rpc.RMPStatusFrom:  statusFrom,
			rpc.RMPFromNick:    fromNick,
			rpc.RMPMain:        content,
			rpc.RMPSignature:   signature,
			rpc.RMPIdentifier:  identifier,
			rpc.RMPDescription: descr,
		},
	}

	// Encode it and return the size of an encrypted version of it.
	rm, err := rpc.ComposeCompressedRM(dummySigner, rmps, compressLevel)
	if err != nil {
		return 0, err
	}
	return uint64(ratchet.EncryptedSize(len(rm))), nil
}

// Returns the estimate cost (in milliatoms) to send the given PM message to a
// remote user. The feeRate must be specified in milliatoms/byte.
func EstimatePMCost(msg string, policy *ServerPolicy) (uint64, error) {
	// Use a medium level compression level for the estimate.
	const compressLevel = 4

	// Encode it and return the size of an encrypted version of it.
	rmpm := rpc.RMPrivateMessage{Message: msg}
	rm, err := rpc.ComposeCompressedRM(dummySigner, rmpm, compressLevel)
	if err != nil {
		return 0, err
	}

	rmSize := ratchet.EncryptedSize(len(rm))
	cost, err := policy.CalcPushCostMAtoms(rmSize)
	return uint64(cost), err
}
