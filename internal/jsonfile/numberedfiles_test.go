package jsonfile

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	"github.com/companyzero/bisonrelay/internal/assert"
	"github.com/companyzero/bisonrelay/internal/testutils"
)

func touchFile(t testing.TB, fname string, isDir bool) {
	t.Helper()
	if isDir {
		os.Mkdir(fname, 0o755)
	} else {
		err := os.WriteFile(fname, nil, 0o644)
		assert.NilErr(t, err)
	}
}

func testNumberedFiles(t *testing.T, prefix, suffix string, base int, isDir bool) {
	dir := testutils.TempTestDir(t, "nbf-")

	var format, padFormat string
	var nfp NumberedFilePattern
	switch base {
	case 10:
		format, padFormat = "%s%d%s", "%s%08d%s"
		nfp = MakeDecimalFilePattern(prefix, suffix, isDir)
	case 16:
		format, padFormat = "%s%x%s", "%s%08x%s"
		nfp = MakeHexFilePattern(prefix, suffix, isDir)
	default:
		t.Fatalf("unsupported base %d", base)
	}

	// Test filename generation.
	gotFname := nfp.FilenameFor(987654)
	wantFname := fmt.Sprintf(padFormat, prefix, 987654, suffix)
	assert.DeepEqual(t, gotFname, wantFname)

	// Helpers.
	touch := func(i uint64) {
		fname := filepath.Join(dir, fmt.Sprintf(format, prefix, i, suffix))
		touchFile(t, fname, isDir)
	}
	mnf := func(i uint64) MatchedNumberedFile {
		fname := fmt.Sprintf(format, prefix, i, suffix)
		return MatchedNumberedFile{ID: i, Filename: fname}
	}

	touchPad := func(i uint64) {
		fname := filepath.Join(dir, fmt.Sprintf(padFormat, prefix, i, suffix))
		touchFile(t, fname, isDir)
	}
	mnfPad := func(i uint64) MatchedNumberedFile {
		fname := fmt.Sprintf(padFormat, prefix, i, suffix)
		return MatchedNumberedFile{ID: i, Filename: fname}
	}

	touchRandom := func() {
		i := rand.Int63()
		fname := filepath.Join(dir, fmt.Sprintf(padFormat, "not-", i, "-valid.ext"))
		_, err := os.Create(fname)
		assert.NilErr(t, err)

	}

	assertFiles := func(want ...MatchedNumberedFile) {
		t.Helper()
		got, err := nfp.MatchFiles(dir)
		assert.NilErr(t, err)
		if want == nil && got != nil {
			t.Fatalf("unexpected matched files: got %v, want nil", got)
		}
		if len(got) != len(want) {
			t.Fatalf("unexpected len of matched files: got %v, want %v",
				got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("unexpected element %d: got %v, want %v",
					i, got[i], want[i])
			}
		}
		gotLast, err := nfp.Last(dir)
		assert.NilErr(t, err)
		var wantLast MatchedNumberedFile
		if len(want) > 0 {
			wantLast = want[len(want)-1]
		}
		if gotLast != wantLast {
			t.Fatalf("unexpected Last() element: got %v, want %v",
				gotLast, wantLast)
		}
	}

	// Before any files exist.
	assertFiles()

	// Before any valid files exist.
	touchRandom()
	assertFiles()

	// Some random number exists.
	touch(100)
	assertFiles(mnf(100))

	// An earlier number is created.
	touch(50)
	assertFiles(mnf(50), mnf(100))

	// The next number is created.
	touch(101)
	assertFiles(mnf(50), mnf(100), mnf(101))

	// An earlier number with padding is created.
	touchPad(75)
	assertFiles(mnf(50), mnfPad(75), mnf(100), mnf(101))

	// A later number with padding is created.
	touchPad(200)
	assertFiles(mnf(50), mnfPad(75), mnf(100), mnf(101), mnfPad(200))

	// A sequential number with padding is created.
	touchPad(201)
	assertFiles(mnf(50), mnfPad(75), mnf(100), mnf(101), mnfPad(200), mnfPad(201))

	// A sequential number without padding is created.
	touch(202)
	assertFiles(mnf(50), mnfPad(75), mnf(100), mnf(101), mnfPad(200), mnfPad(201), mnf(202))

	// Files are modified.
	touchPad(75)
	touch(100)
	assertFiles(mnf(50), mnfPad(75), mnf(100), mnf(101), mnfPad(200), mnfPad(201), mnf(202))
}

// TestNumberedFiles tests that the numbered file system works with hex
// encoded numbers.
func TestNumberedFiles(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		prefix string
		suffix string
		base   int
	}{
		{name: "decimal", prefix: "pre-", suffix: "-pos.ext", base: 10},
		{name: "decimal no prefix", suffix: "-pos.ext", base: 10},
		{name: "decimal no suffix", suffix: "-pos.ext", base: 10},
		{name: "decimal no extra", base: 10},
		{name: "hex", prefix: "pre-", suffix: "-pos.ext", base: 16},
		{name: "hex no prefix", suffix: "-pos.ext", base: 16},
		{name: "hex no suffix", prefix: "pre-", base: 16},
		{name: "hex no extra", base: 16},
	}
	for _, isDir := range []bool{false, true} {
		isDir := isDir
		for _, tc := range tests {
			tc := tc
			name := tc.name
			if isDir {
				name += " dir"
			} else {
				name += " file"
			}
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				testNumberedFiles(t, tc.prefix, tc.suffix, tc.base, isDir)
			})
		}
	}
}

// TestNumberedFilesDirDoesNotExist tests that invoking the functions in an
// inexistent dir does not fail.
func TestNumberedFilesDirDoesNotExist(t *testing.T) {
	dir := "/path/to/dir/that/does/not/exist"
	nfp := MakeDecimalFilePattern("", "", false)
	_, err := nfp.Last(dir)
	assert.NilErr(t, err)
	_, err = nfp.MatchFiles(dir)
	assert.NilErr(t, err)
}

func testSequential(t *testing.T, prefix, suffix string, base int, isDir bool) {
	var padFormat string
	var nfp NumberedFilePattern
	switch base {
	case 10:
		padFormat = "%s%08d%s"
		nfp = MakeDecimalFilePattern(prefix, suffix, isDir)
	case 16:
		padFormat = "%s%08x%s"
		nfp = MakeHexFilePattern(prefix, suffix, isDir)
	default:
		t.Fatalf("unsupported base %d", base)
	}

	dir := testutils.TempTestDir(t, "nbf-")
	max := 101
	for i := 0; i < max; i++ {
		last, err := nfp.Last(dir)
		assert.NilErr(t, err)
		next := last.ID + 1
		fname := fmt.Sprintf(padFormat, prefix, next, suffix)
		touchFile(t, filepath.Join(dir, fname), isDir)
	}

	last, err := nfp.Last(dir)
	assert.NilErr(t, err)
	assert.DeepEqual(t, last.ID, uint64(max))
}

// TestSequential tests that creating sequential entries works as expected.
func TestSequential(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		prefix string
		suffix string
		base   int
	}{
		{name: "decimal", prefix: "pre-", suffix: "-pos.ext", base: 10},
		{name: "decimal no prefix", suffix: "-pos.ext", base: 10},
		{name: "decimal no suffix", suffix: "-pos.ext", base: 10},
		{name: "decimal no extra", base: 10},
		{name: "hex", prefix: "pre-", suffix: "-pos.ext", base: 16},
		{name: "hex no prefix", suffix: "-pos.ext", base: 16},
		{name: "hex no suffix", prefix: "pre-", base: 16},
		{name: "hex no extra", base: 16},
	}
	for _, isDir := range []bool{false, true} {
		isDir := isDir
		for _, tc := range tests {
			tc := tc
			name := tc.name
			if isDir {
				name += " dir"
			} else {
				name += " file"
			}
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				testSequential(t, tc.prefix, tc.suffix, tc.base, isDir)
			})
		}
	}

}
