package session

import (
	"math/big"
	"unsafe"
)

var (
	one = new(big.Int).SetUint64(1)
	two = new(big.Int).SetUint64(2)
	//stupid = new(big.Int).SetUint64(10923849328492384000)

	halfForClient = true
	halfForServer = !halfForClient
)

type sequence struct {
	x *big.Int
}

func (s sequence) Nonce() *[24]byte {
	return (*[24]byte)(unsafe.Pointer(&s.x.Bytes()[0]))
}

func (s *sequence) Decrease() {
	s.x = s.x.Sub(s.x, one)
	//s.x = s.x.Sub(s.x, stupid)
}

func newSequence(half bool) *sequence {
	max := make([]byte, 24)
	for k := range max {
		max[k] = 0xff
	}
	x := new(big.Int).SetBytes(max)
	if half {
		x = x.Div(x, two)
	}
	//spew.Dump(x.Bytes())
	//x = x.Add(x, new(big.Int).SetInt64(2))
	//x = x.Sub(x, new(big.Int).SetInt64(1024*82))
	//xx := *(*[24]byte)(unsafe.Pointer(&x.Bytes()[0]))
	//spew.Dump(xx)
	return &sequence{
		x: x,
	}
}
