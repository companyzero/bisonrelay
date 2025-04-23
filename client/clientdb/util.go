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
	"github.com/companyzero/bisonrelay/internal/jsonfile"
	"github.com/companyzero/bisonrelay/internal/strescape"
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

func (db *DB) mustRandomInt31() int32 {
	var b [4]byte
	if n, err := db.rnd.Read(b[:]); n < 4 || err != nil {
		panic("out of entropy")
	}
	return int32(binary.LittleEndian.Uint32(b[:]) & 0x7fffffff)
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

// seqPathNotExistsFile returns a filename for a file that does not exist in the
// given dir. If dir/prefix+ext exists, starts appending numbers until a file
// that does not exist is found (e.g. dir/prefix+"_1"+ext).
func (db *DB) seqPathNotExistsFile(dir string, prefix, ext string) (string, error) {
	const max = 999999999

	// Ensure dir exists.
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}

	if len(ext) > 0 && ext[0] != '.' {
		ext = "." + ext
	}

	fname := fmt.Sprintf("%s%s", prefix, ext)
	for i := 1; i < max; i++ {
		fullPath := filepath.Join(dir, fname)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			return fname, nil
		} else if err != nil {
			db.log.Warnf("Error attempting to evaluate existence file %s: %v", fname, err)
		}

		fname = fmt.Sprintf("%s_%d%s", prefix, i, ext)
	}
	return "", errors.New("too many attempts at finding a unique filename")
}

// saveJsonFile saves the data to a temp file, then renames the temp file to
// the passed filename.
func (db *DB) saveJsonFile(fname string, data interface{}) error {
	return jsonfile.Write(fname, data, db.log)
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

// escapeNickForFname escapes a nick to be used as part of a filename.
func escapeNickForFname(nick string) string {
	return strescape.PathElement(strescape.Nick(nick))
}

// readJsonFile reads the first json message from the given filename and
// decodes it into data.
func (db *DB) readJsonFile(fname string, data interface{}) error {
	err := jsonfile.Read(fname, data)
	if errors.Is(err, jsonfile.ErrNotFound) {
		return ErrNotFound
	}
	return err
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
