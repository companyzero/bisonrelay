package testutils

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/decred/slog"
)

// TestLogBackend is a slog backend suitable for using with tests.
type TestLogBackend struct {
	mtx     sync.Mutex
	tb      testing.TB
	w       io.Writer
	done    bool
	showLog bool
}

func (tlb *TestLogBackend) Write(b []byte) (int, error) {
	tlb.mtx.Lock()
	if !tlb.done && tlb.showLog {
		tlb.tb.Log(string(b[:len(b)-1]))
	}
	tlb.mtx.Unlock()

	if tlb.w != nil {
		tlb.w.Write(b)
	}
	return len(b), nil
}

// NamedSubLogger returns a sub logger backend suitable for passing to clients,
// prefixed with the specified name.
func (tlb *TestLogBackend) NamedSubLogger(name string, w io.Writer) func(subsys string) slog.Logger {
	if w != nil {
		w = io.MultiWriter(w, tlb)
	} else {
		w = tlb
	}
	bknd := slog.NewBackend(w)
	return func(subsys string) slog.Logger {
		logg := bknd.Logger(fmt.Sprintf("%7s - %s", name, subsys))
		if strings.HasPrefix(subsys, "RMPL") {
			// Don't log RM payloads by default as it's too
			// verbose.
			logg.SetLevel(slog.LevelDebug)
		} else {
			logg.SetLevel(slog.LevelTrace)
		}
		return logg
	}
}

type TestLogBackendOption func(t *TestLogBackend)

func WithShowLog(showLog bool) TestLogBackendOption {
	return func(t *TestLogBackend) {
		t.showLog = showLog
	}
}

func WithMiddlewareWriter(w io.Writer) TestLogBackendOption {
	return func(t *TestLogBackend) {
		t.w = w
	}
}

// NewTestLogBackend returns a log backend that can be used as an io.Writer to
// write logs to during a test.
func NewTestLogBackend(t testing.TB, opts ...TestLogBackendOption) *TestLogBackend {
	tlb := &TestLogBackend{tb: t}
	for _, opt := range opts {
		opt(tlb)
	}
	t.Cleanup(func() {
		tlb.mtx.Lock()
		tlb.done = true
		tlb.mtx.Unlock()
	})
	return tlb
}

// TestLoggerSys returns an slog.Logger that logs by issuing t.Log calls.
func TestLoggerSys(t testing.TB, sys string) slog.Logger {
	bknd := slog.NewBackend(NewTestLogBackend(t))
	logg := bknd.Logger(sys)
	logg.SetLevel(slog.LevelTrace)
	return logg
}

// TestLoggerBackend returns a function that generates loggers for subsystems,
// all of which log by calling t.Log.
func TestLoggerBackend(t testing.TB, name string) func(subsys string) slog.Logger {
	tlb := NewTestLogBackend(t)
	bknd := slog.NewBackend(tlb)
	return func(subsys string) slog.Logger {
		logg := bknd.Logger(fmt.Sprintf("%7s - %s", name, subsys))
		if strings.HasPrefix(subsys, "RMPL") {
			// Don't log RM payloads by default as it's too
			// verbose.
			logg.SetLevel(slog.LevelDebug)
		} else {
			logg.SetLevel(slog.LevelTrace)
		}
		return logg
	}
}
