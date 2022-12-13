package testutils

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/decred/slog"
)

type testLogBackend struct {
	mtx  sync.Mutex
	tb   testing.TB
	done bool
}

func (tlb *testLogBackend) Write(b []byte) (int, error) {
	tlb.mtx.Lock()
	if !tlb.done {
		tlb.tb.Log(string(b[:len(b)-1]))
	}
	tlb.mtx.Unlock()
	return len(b), nil
}

// NewTestLogBackend returns a log backend that can be used as an io.Writer to
// write logs to during a test.
func NewTestLogBackend(t testing.TB) *testLogBackend {
	tlb := &testLogBackend{tb: t}
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
	bknd := slog.NewBackend(NewTestLogBackend(t))
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
