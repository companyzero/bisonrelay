package assert

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"reflect"
	"testing"
	"time"

	"golang.org/x/exp/slices"
)

// ChanWritten returns the value written to chan c or times out.
func ChanWritten[T any](t testing.TB, c chan T) T {
	t.Helper()
	var v T
	select {
	case v = <-c:
	case <-time.After(timeout):
		t.Fatal("timeout waiting for chan read")
	}
	return v
}

// ChanWrittenWithVal asserts the chan c was written with a value that
// DeepEquals v.
func ChanWrittenWithVal[T any](t testing.TB, c chan T, want T) T {
	t.Helper()
	var got T
	select {
	case got = <-c:
	case <-time.After(timeout):
		t.Fatal("timeout waiting for chan read")
	}
	DeepEqual(t, got, want)
	return got
}

// ChanWrittenWithValTimeout asserts the chan c was written with a value that
// DeepEquals v before the timeout expires.
func ChanWrittenWithValTimeout[T any](t testing.TB, c chan T, want T, timeout time.Duration) T {
	t.Helper()
	var got T
	select {
	case got = <-c:
	case <-time.After(timeout):
		t.Fatal("timeout waiting for chan read")
	}
	DeepEqual(t, got, want)
	return got
}

// ChanNotWritten asserts that the chan is not written at least until the passed
// timeout value.
func ChanNotWritten[T any](t testing.TB, c chan T, timeout time.Duration) {
	t.Helper()
	select {
	case v := <-c:
		t.Fatalf("channel was written with value %v", v)
	case <-time.After(timeout):
	}
}

// Chan2NotWritten asserts that the chans are not written at least until the
// passed timeout value.
func Chan2NotWritten[T any, U any](t testing.TB, c chan T, d chan U, timeout time.Duration) {
	t.Helper()
	select {
	case v := <-c:
		t.Fatalf("channel 1 was written with value %v", v)
	case v := <-d:
		t.Fatalf("channel 2 was written with value %v", v)
	case <-time.After(timeout):
	}
}

// DeepEqual asserts got is reflect.DeepEqual to want.
func DeepEqual[T any](t testing.TB, got, want T) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Unexpected values: got %v, want %v", got, want)
	}
}

// NotDeepEqual asserts got is not reflect.DeepEqual to want.
func NotDeepEqual[T any](t testing.TB, got, want T) {
	t.Helper()
	if reflect.DeepEqual(got, want) {
		t.Fatalf("Unexpected equal values: got %v, want %v", got, want)
	}
}

// ErrorIs asserts that errors.Is(got, want).
func ErrorIs(t testing.TB, got, want error) {
	t.Helper()
	if !errors.Is(got, want) {
		t.Fatalf("Unexpected error: got %v, want %v", got, want)
	}
}

// NilErr fails the test if err is non-nil.
func NilErr(t testing.TB, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("Unexpected non-nil error: %v", err)
	}
}

// NilErrFromChan fails the test if a non-nil error is received in the chan or
// if the channel fails to be written to in 30 seconds.
func NilErrFromChan(t testing.TB, errChan chan error) {
	t.Helper()
	select {
	case err := <-errChan:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(timeout):
		t.Fatal("timeout waiting for errChan read")
	}
}

// NonNilErr asserts that err is not nil. It's preferable to use a specific
// error check instead of this one.
func NonNilErr(t testing.TB, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("unexpected nil error")
	}
}

// DoesNotBlock asserts that calling f() does not block for an inordinate amount
// of time.
func DoesNotBlock(t testing.TB, f func()) {
	t.Helper()
	done := make(chan struct{})
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	go func() {
		f()
		close(done)
	}()
	select {
	case <-done:
	case <-ctx.Done():
		t.Fatal("timeout waiting for function to finish")
	}
}

// ContextDone asserts the passed context is done.
func ContextDone(t testing.TB, ctx context.Context) {
	t.Helper()
	select {
	case <-ctx.Done():
	default:
		t.Fatal("context is not done yet")
	}
}

// BoolIs asserts the given bool value.
func BoolIs(t testing.TB, got, want bool) {
	t.Helper()
	if got != want {
		t.Fatalf("unexpected bool. got %v, want %v", got, want)
	}
}

// Contains asserts that s contains e.
func Contains[S ~[]E, E comparable](t testing.TB, s S, e E) {
	t.Helper()
	if !slices.Contains(s, e) {
		t.Fatalf("slice %v does not contain element %v", s, e)
	}
}

// EqualFiles asserts that the files in path1 and path2 have the same content.
func EqualFiles(t testing.TB, path1, path2 string) {
	f1, err := os.Open(path1)
	if err != nil {
		t.Fatalf("file1 failed to open: %v", err)
	}

	f2, err := os.Open(path2)
	if err != nil {
		t.Fatalf("file2 failed to open: %v", err)
	}

	var b1 [4096]byte
	var b2 [4096]byte
	var p int
	for {
		n1, err1 := io.ReadFull(f1, b1[:])
		n2, err2 := io.ReadFull(f2, b2[:])
		if n1 != n2 {
			t.Fatalf("read different nb of bytes: n1 %d n2 %d", n1, n2)
		}

		if !bytes.Equal(b1[:n1], b2[:n2]) {
			t.Fatalf("bytes are different starting around position %d", p)
		}

		if errors.Is(err1, io.ErrUnexpectedEOF) && errors.Is(err2, io.ErrUnexpectedEOF) {
			// Done reading.
			break
		}
		if errors.Is(err1, io.EOF) && errors.Is(err2, io.EOF) {
			// Done reading.
			break
		}
		if err1 != nil {
			t.Fatalf("error reading from file1: %v", err1)
		}
		if err2 != nil {
			t.Fatalf("error reading from file2: %v", err2)
		}

		p += n1
	}

	if err := f1.Close(); err != nil {
		t.Fatalf("error closing file1: %v", err)
	}
	if err := f2.Close(); err != nil {
		t.Fatalf("error closing file2: %v", err)
	}
}
