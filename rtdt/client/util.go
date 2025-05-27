package rtdtclient

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"

	"github.com/companyzero/bisonrelay/rpc"
)

// sourceTargetPairKey is a comparable key that combines a source and target
// peer id.
type sourceTargetPairKey uint64 // source + target ids

func (key sourceTargetPairKey) String() string {
	return fmt.Sprintf("%016x", uint64(key))
}

func makeSourceTargetPairKey(source, target rpc.RTDTPeerID) sourceTargetPairKey {
	return sourceTargetPairKey(source)<<32 + sourceTargetPairKey(target)
}

func mustRandomUint64() uint64 {
	var b [8]byte
	n, err := rand.Read(b[:])
	if n != 8 {
		panic("crypto reader read too few bytes")
	}
	if err != nil {
		panic("crypto reader panicked")
	}
	return binary.LittleEndian.Uint64(b[:])
}
