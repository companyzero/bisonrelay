package lockfile

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/rogpeppe/go-internal/lockedfile"
)

// LockFile holds the lockfile.
type LockFile struct {
	f       *lockedfile.File
	pid     int
	host    string
	process string
}

// Close closes the lockfile.
func (lf *LockFile) Close() error {
	if lf.f == nil {
		return fmt.Errorf("nil internal locked file")
	}
	return lf.f.Close()
}

func (lf *LockFile) String() string {
	return fmt.Sprintf("pid=%d, host=%s, process=%s", lf.pid, lf.host, lf.process)
}

// ProcInfo returns info about the current process.
func ProcInfo() (pid int, host string, process string) {
	pid = os.Getpid()
	host, _ = os.Hostname()
	if len(os.Args) > 0 {
		process = os.Args[0]
	}
	return
}

// Create initializes a new lock file.
func Create(ctx context.Context, filePath string) (*LockFile, error) {
	if err := os.MkdirAll(filepath.Dir(filePath), 0o0700); err != nil {
		return nil, err
	}
	cf := make(chan *lockedfile.File)
	cerr := make(chan error)
	go func() {
		f, err := lockedfile.Create(filePath)
		if err != nil {
			cerr <- err
		} else {
			cf <- f
		}
	}()

	select {
	case f := <-cf:
		pid, host, process := ProcInfo()

		// Opened the locked file. Write out the current host name and
		// pid to ease debugging. We ignore errors here as it's
		// not fatal for our purposes.
		f.WriteString(fmt.Sprintf("PID=%d\n", pid))
		f.WriteString(fmt.Sprintf("Host=%q\n", host))
		f.WriteString(fmt.Sprintf("Process=%q\n", process))

		lf := &LockFile{f: f, pid: pid, host: host, process: process}
		return lf, nil

	case err := <-cerr:
		// Opening errored out.
		return nil, err

	case <-ctx.Done():
		// When the context is done before we get a reply, the file may
		// still (eventually) open, so make sure we close it if it ever
		// returns.
		go func() {
			select {
			case <-cerr:
			case f := <-cf:
				f.Close()
			}
		}()
		return nil, ctx.Err()
	}
}
