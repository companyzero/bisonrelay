package jsonfile

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/decred/slog"
)

var ErrNotFound = errors.New("json file not found")

// Write data to a temp file, then renames the temp file to the passed
// filename in json format.
//
// log is used to log warnings that are not fatal to the Write() operation.
func Write(fname string, data interface{}, log slog.Logger) error {
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
			if log != nil && closeErr != nil {
				log.Warnf("Unable to close temp file prior to cleanup: %v", err)
			}
		}
		if remErr := os.Remove(tempFname); log != nil && remErr != nil {
			log.Warnf("Unable to remove temp file %s: %v", tempFname, err)
		}
	}

	return err
}

// Read the first json message from the given filename and decodes it into
// data.
func Read(fname string, data interface{}) error {
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

// Exists returns true if the specified file exists.
func Exists(fname string) bool {
	if _, err := os.Stat(fname); err != nil {
		return false
	}
	return true
}

// RemoveIfExists removes the filename if it exists. If it does not exist, this
// doesn't return an error.
func RemoveIfExists(fname string) error {
	err := os.Remove(fname)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
