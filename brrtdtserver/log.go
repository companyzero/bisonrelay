package main

import (
	"io"

	"github.com/jrick/logrotate/rotator"
)

type logBackend struct {
	stdOut     io.Writer
	logRotator *rotator.Rotator
}

func (bknd *logBackend) Write(b []byte) (int, error) {
	if bknd.stdOut != nil {
		bknd.stdOut.Write(b)
	}
	if bknd.logRotator != nil {
		bknd.logRotator.Write(b)
	}

	return len(b), nil
}
