package lockfile

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/companyzero/bisonrelay/internal/assert"
)

// TestSingleUse tests that locking using a single caller works.
func TestSingleUse(t *testing.T) {
	fname := filepath.Join(t.TempDir(), "lockfile")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	lf, err := Create(ctx, fname)
	assert.NilErr(t, err)
	err = lf.Close()
	assert.NilErr(t, err)
}

// TestConcurrentLock tests the behavior of the lockfile when multiple
// concurrent attempts are made to open it.
func TestConcurrentLock(t *testing.T) {
	fname := filepath.Join(t.TempDir(), "lockfile")
	testCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	ctx1, cancel1 := context.WithCancel(testCtx)

	// The first attempt should succeed immediately.
	lf, err := Create(ctx1, fname)
	assert.NilErr(t, err)

	// Canceling the context now should not interfere in further tests.
	cancel1()

	// The second attempt should block, so we'll run with a small timeout
	// context.
	ctx2, cancel2 := context.WithTimeout(testCtx, 50*time.Millisecond)
	defer cancel2()
	_, err = Create(ctx2, fname)
	assert.ErrorIs(t, err, context.DeadlineExceeded)

	// The third attempt should block until the first lockfile is closed.
	ctx3, cancel3 := context.WithCancel(testCtx)
	defer cancel3()
	cf3, cerr3 := make(chan *LockFile), make(chan error)
	go func() {
		lf, err := Create(ctx3, fname)
		if err != nil {
			cerr3 <- err
		} else {
			cf3 <- lf
		}
	}()

	// Verify it is indeed blocked and it did not error.
	assert.Chan2NotWritten(t, cf3, cerr3, time.Second)

	// Closing the original lockfile should not error
	err = lf.Close()
	assert.NilErr(t, err)

	// The third attempt should now unblock and can be closed.
	lf3 := assert.ChanWritten(t, cf3)
	err = lf3.Close()
	assert.NilErr(t, err)
}

// TestLocksForever tests that when the process ends, the lock file is released.
// This test needs to be manually performed, by running go test -count=1 twice
// so that the same file is attempted to be read.
func TestLocksForever(t *testing.T) {
	fname := filepath.Join(os.TempDir(), "br_clientdb_lockfile_test_file")
	Create(context.Background(), fname)
}
