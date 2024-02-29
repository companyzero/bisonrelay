package assert

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

var timeout = 30 * time.Second

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
