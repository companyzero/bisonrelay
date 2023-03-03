package jsonrpc

import (
	"context"
	"io"
	"time"
	"unicode/utf8"

	"github.com/decred/slog"
)

// debugWriter logs all Write() calls to the logger.
type debugWriter struct {
	log   slog.Logger
	inner io.Writer
}

func (dw debugWriter) Write(b []byte) (int, error) {
	dw.log.Tracef("Wrote %s", b)
	return dw.inner.Write(b)
}

// debugReader logs all Read() calls to the logger.
type debugReader struct {
	log   slog.Logger
	inner io.Reader
}

func (dw debugReader) Read(b []byte) (int, error) {
	n, err := dw.inner.Read(b)
	dw.log.Tracef("Read %s", b[:n])
	return n, err
}

// equalASCIIFold returns true if s is equal to t with ASCII case folding as
// defined in RFC 4790.  This function was lifted and from the gorilla websocket
// code since it's not exported.
func equalASCIIFold(s, t string) bool {
	for s != "" && t != "" {
		sr, size := utf8.DecodeRuneInString(s)
		s = s[size:]
		tr, size := utf8.DecodeRuneInString(t)
		t = t[size:]
		if sr == tr {
			continue
		}
		if 'A' <= sr && sr <= 'Z' {
			sr = sr + 'a' - 'A'
		}
		if 'A' <= tr && tr <= 'Z' {
			tr = tr + 'a' - 'A'
		}
		if sr != tr {
			return false
		}
	}
	return s == t
}

// requestsSemaphore is a semaphore to limit the number of outsanding requests.
type requestsSemaphore chan struct{}

// acquire a semaphore element. It returns false if the context is canceled
// before the semaphore is acquired.
func (r requestsSemaphore) acquire(ctx context.Context) bool {
	select {
	case <-r:
		return true
	case <-ctx.Done():
		return false
	}
}

// release a previously acquired semaphore element.
func (r requestsSemaphore) release() {
	r <- struct{}{}
}

// drain the semaphore. This blocks until all semaphore items have been
// released or until the context is canceled. Returns the number of outstanding
// elements in the semaphore if the context was canceled before all elements
// were drained.
func (r requestsSemaphore) drain(ctx context.Context) int {
	for i := 0; i < cap(r); i++ {
		select {
		case <-r:
		case <-ctx.Done():
			return cap(r) - i
		}
	}
	return 0
}

// makeRequestsSemaphore creates a semaphore with the given max capacity.
func makeRequestsSemaphore(max int) requestsSemaphore {
	res := make(requestsSemaphore, max)
	for i := 0; i < max; i++ {
		res <- struct{}{}
	}
	return res
}

// delayedCancelCtx returns a context that gets canceled when the passed
// context is canceled _and_ after a delay has passed (or if the returned
// cancel function is called).
func delayedCancelCtx(ctx context.Context, timeout time.Duration) (context.Context, func()) {
	outer, cancelOuter := context.WithCancel(context.Background())
	go func() {
		select {
		case <-ctx.Done():
		case <-outer.Done():
			return
		}

		select {
		case <-time.After(timeout):
			cancelOuter()
		case <-outer.Done():
		}
	}()
	return outer, cancelOuter
}

type slogWriter struct {
	f func(...interface{})
}

func (sw slogWriter) Write(b []byte) (int, error) {
	sw.f(string(b))
	return len(b), nil
}
