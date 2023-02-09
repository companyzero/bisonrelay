package testutils

import (
	"os"
	"path/filepath"
	"testing"
)

// TempTestDir returns a temp dir for a test that only gets cleaned up if the
// test does not fail.
func TempTestDir(t testing.TB, prefix string) string {
	dir, err := os.MkdirTemp("", prefix)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		if !t.Failed() {
			err := os.RemoveAll(dir)
			if err != nil {
				t.Logf("Unable to remove temp dir %s: %v", dir, err)
			}
		} else {
			t.Logf("Test data dir: %s", dir)
		}
	})

	return dir
}

// FixedTempDir creates a temp dir with a fixed name (without any random parts)
// and cleans it up on a successful test. The dir is cleared before this
// function returns.
func FixedTempDir(t testing.TB, name string) string {
	dir := filepath.Join(os.TempDir(), name)
	if s, err := os.Stat(dir); err != nil && !os.IsNotExist(err) {
		t.Fatalf("unable to stat temp dir: %v", err)
	} else if err == nil && !s.IsDir() {
		t.Fatalf("%s exists and is not a dir", dir)
	} else if err == nil {
		if err := os.RemoveAll(dir); err != nil {
			t.Fatalf("unable to clear temp dir %s before test: %v", dir, err)
		}
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("unable to make temp dir %s: %v", dir, err)
	}

	t.Cleanup(func() {
		if !t.Failed() {
			err := os.RemoveAll(dir)
			if err != nil {
				t.Logf("Unable to remove temp dir %s: %v", dir, err)
			}
		} else {
			t.Logf("Test data dir: %s", dir)
		}
	})

	return dir
}
