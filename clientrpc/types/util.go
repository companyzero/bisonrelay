package types

import "fmt"

// DebugSequenceID returns a debug string for a given sequence ID.
func DebugSequenceID(seq uint64) string {
	// This follows the convention of the client/internal/replaymsglog
	// package.
	fid := uint32(seq >> 32)
	off := uint32(seq)
	return fmt.Sprintf("%06x/%08x", fid, off)
}
