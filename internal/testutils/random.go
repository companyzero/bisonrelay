package testutils

import (
	"io"
	"math/rand"
	"os"
	"testing"
	"time"
)

// RandomFile creates a random file for testing and removes it after the test
// ends.
func RandomFile(t testing.TB, sz int) string {
	t.Helper()
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	f, err := os.CreateTemp("", "test-random-file")
	if err != nil {
		t.Fatal(err)
	}
	var b [4096]byte
	for i := 0; i < sz; i += sz {
		end := len(b)
		if sz-i < end {
			end = sz - i
		}
		_, err = io.ReadFull(rng, b[:end])
		if err != nil {
			t.Fatal(err)
		}
		_, err = f.Write(b[:end])
		if err != nil {
			t.Fatal(err)
		}
	}

	err = f.Close()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Remove(f.Name()) })
	return f.Name()
}
