package rpc

import (
	"errors"
	"io"
)

var errLimitedReaderExhausted = errors.New("errLimitedReaderExhausted")

// limitedReader is a port of the stdlib LimitedReader implementation
// that returns a specific error when more bytes are requested then available.
//
// This is used to differentiate between a standard EOF and an EOF caused
// by the limitedReader exceeding its read budget.
type limitedReader struct {
	R io.Reader // underlying reader
	N uint      // max bytes remaining
}

func (l *limitedReader) Read(p []byte) (n int, err error) {
	if l.N <= 0 {
		return 0, errLimitedReaderExhausted
	}
	if uint(len(p)) > l.N {
		p = p[0:l.N]
	}
	n, err = l.R.Read(p)
	l.N -= uint(n)
	return
}
