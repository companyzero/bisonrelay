package clientdb

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/companyzero/bisonrelay/client/clientintf"
)

// multiCtx returns a context that gets canceled when any one of the passed
// contexts are canceled.
func multiCtx(ctxs ...context.Context) (context.Context, func()) {
	gctx, gcancel := context.WithCancel(context.Background())
	var once sync.Once
	cancel := func() {
		once.Do(gcancel)
	}
	for _, ctx := range ctxs {
		ctx := ctx
		go func() {
			select {
			case <-gctx.Done():
			case <-ctx.Done():
				cancel()
			}
		}()
	}
	return gctx, cancel
}

func itoa(i uint64) string {
	return strconv.FormatUint(i, 10)
}

func atoi(s string) (uint64, error) {
	return strconv.ParseUint(s, 10, 64)
}

func fileExists(fname string) bool {
	if _, err := os.Stat(fname); err != nil {
		return false
	}
	return true
}

// removeIfExists removes the filename if it exists. If it does not exist, this
// doesn't return an error.
func removeIfExists(fname string) error {
	err := os.Remove(fname)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// copyFile copies the src file to dst. Any existing file will be overwritten
// and will not copy file attributes.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err

	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err

	}
	defer out.Close()

	_, err = io.Copy(out, in)
	if err != nil {
		return err

	}
	return out.Close()
}

func sha256File(fname string) ([]byte, error) {
	f, err := os.Open(fname)
	if err != nil {
		return nil, err
	}
	h := sha256.New()
	_, err = io.Copy(h, f)
	if err != nil {
		return nil, err
	}
	return h.Sum(nil), nil
}

func (db *DB) mustRandomUint64() uint64 {
	var b [8]byte
	if n, err := db.rnd.Read(b[:]); n < 8 || err != nil {
		panic("out of entropy")
	}
	return binary.LittleEndian.Uint64(b[:])
}

// randomIDInDir generates a random id that does not yet exist as a file in the
// given dir.
func (db *DB) randomIDInDir(dir string) (clientintf.ID, error) {
	var res clientintf.ID
	const max = 999999999
	for i := 0; i < max; i++ {
		if n, err := db.rnd.Read(res[:]); n < 32 || err != nil {
			return res, fmt.Errorf("out of entropy: %v", err)
		}

		if _, err := os.Stat(filepath.Join(dir, res.String())); os.IsNotExist(err) {
			return res, nil
		} else if err != nil {
			return res, err
		}
	}

	return res, fmt.Errorf("could not find random id in dir %s", dir)
}

// saveJsonFile saves the data to a temp file, then renames the temp file to
// the passed filename.
func (db *DB) saveJsonFile(fname string, data interface{}) error {
	dir := filepath.Dir(fname)
	base := filepath.Base(fname)
	tempFname := filepath.Join(dir, "."+base+".new")

	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("unable to create dest dir: %w", err)
	}

	f, err := os.Create(tempFname)
	if err != nil {
		return fmt.Errorf("unable to create temp file: %w", err)
	}

	// From this point on, there are no more early returns, so that the
	// temp file is removed in case of errors.

	enc := json.NewEncoder(f)
	err = enc.Encode(data)
	if err != nil {
		err = fmt.Errorf("unable to encode json contents: %w", err)
	}
	if err == nil {
		err = f.Sync()
		if err != nil {
			err = fmt.Errorf("unable to fsync temp file: %w", err)
		}
	}
	if err == nil {
		err = f.Close()
		f = nil
		if err != nil {
			err = fmt.Errorf("unable to close temp file: %w", err)
		}
	}
	if err == nil {
		err = os.Rename(tempFname, fname)
		if err != nil {
			err = fmt.Errorf("unable to rename temp file to final file: %w", err)
		}
	}
	if err != nil {
		if f != nil {
			closeErr := f.Close()
			if closeErr != nil {
				db.log.Warnf("Unable to close temp file prior to cleanup: %v", err)
			}
		}
		if remErr := os.Remove(tempFname); remErr != nil {
			db.log.Warnf("Unable to remove temp file %s: %v", tempFname, err)
		}
	}

	return err
}

// dirExistsEmpty returs true if the given dir exists and is empty.
func dirExistsEmpty(dir string) bool {
	f, err := os.Open(dir)
	if err != nil {
		return false
	}

	i, err := f.Readdir(1)
	if err != nil && !errors.Is(err, io.EOF) {
		return false
	}
	return len(i) == 0
}

// readJsonFile reads the first json message from the given filename and
// decodes it into data.
func (db *DB) readJsonFile(fname string, data interface{}) error {
	f, err := os.Open(fname)
	if os.IsNotExist(err) {
		return ErrNotFound
	} else if err != nil {
		return err
	}

	defer f.Close()
	dec := json.NewDecoder(f)
	return dec.Decode(data)
}

// appendToJsonFile appends the given data to the file as a json entry.
func (db *DB) appendToJsonFile(fname string, data interface{}) error {
	if err := os.MkdirAll(filepath.Dir(fname), 0o0700); err != nil {
		return err
	}

	f, err := os.OpenFile(fname, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(f)
	err = enc.Encode(data)
	if closeErr := f.Close(); closeErr != nil {
		db.log.Warnf("Unable to close appended file %s: %v", fname, closeErr)
	}
	return err
}
