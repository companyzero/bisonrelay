package jsonfile

import (
	"fmt"
	"os"
	"regexp"
	"strconv"

	"golang.org/x/exp/slices"
)

// MatchedNumberedFile is a matched numbered file.
type MatchedNumberedFile struct {
	Filename string
	ID       uint64
}

func cmpMatchedNumberedFiles(a MatchedNumberedFile, id uint64) int {
	return int(a.ID - id)
}

// NumberedFilePattern can be used to match files in a dir.
type NumberedFilePattern struct {
	base    int
	re      *regexp.Regexp
	nameFmt string
	dir     bool
}

func (nfp NumberedFilePattern) walkFiles(dir string, cb func(string, uint64) error) error {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}

	for _, e := range entries {
		entryIsDir := e.Type().IsDir()
		if nfp.dir != entryIsDir {
			continue
		}

		name := e.Name()
		match := nfp.re.FindStringSubmatch(name)
		if len(match) < 2 {
			continue
		}

		i, err := strconv.ParseUint(match[1], nfp.base, 64)
		if err != nil {
			// Log error?
			continue
		}

		cb(name, i)
	}
	return nil
}

// MatchFiles returns the files that match the pattern in the given dir.
func (nfp NumberedFilePattern) MatchFiles(dir string) ([]MatchedNumberedFile, error) {
	var res []MatchedNumberedFile
	err := nfp.walkFiles(dir, func(name string, i uint64) error {
		idx, found := slices.BinarySearchFunc(res, i, cmpMatchedNumberedFiles)
		if found {
			res[idx].Filename = name
		} else {
			res = slices.Insert(res, idx, MatchedNumberedFile{ID: i, Filename: name})
		}
		return nil
	})
	return res, err
}

// Last returns the number and filename of the file with highest number
// in the dir.
//
// If no file is found, the returned filename is empty.
func (nfp NumberedFilePattern) Last(dir string) (MatchedNumberedFile, error) {
	var res MatchedNumberedFile
	err := nfp.walkFiles(dir, func(name string, i uint64) error {
		if i >= res.ID {
			res.ID = i
			res.Filename = name
		}
		return nil
	})
	return res, err
}

// FilenameFor returns the filename for a given number.
func (nfp NumberedFilePattern) FilenameFor(i uint64) string {
	return fmt.Sprintf(nfp.nameFmt, i)
}

// MakeHexFilePattern creates a numbered file pattern that can handle hex
// encoded numbers. It panics if prefix+suffix cannot be made into a valid
// regexp.
func MakeHexFilePattern(prefix, suffix string, isDir bool) NumberedFilePattern {
	pattern := "^" + prefix + `([0-9a-fA-F]+)` + suffix + "$"
	re, err := regexp.Compile(pattern)
	if err != nil {
		panic(err)
	}
	nameFmt := prefix + "%08x" + suffix
	return NumberedFilePattern{re: re, base: 16, nameFmt: nameFmt, dir: isDir}
}

// MakeDecimalFilePattern creates a numbered file pattern that uses decimal
// encoded numbers. It panics if prefix+suffix cannot be made into a valid
// regexp.
func MakeDecimalFilePattern(prefix, suffix string, isDir bool) NumberedFilePattern {
	pattern := "^" + prefix + `([0-9a-fA-F]+)` + suffix + "$"
	re, err := regexp.Compile(pattern)
	if err != nil {
		panic(err)
	}
	nameFmt := prefix + "%08d" + suffix
	return NumberedFilePattern{re: re, base: 10, nameFmt: nameFmt, dir: isDir}
}
