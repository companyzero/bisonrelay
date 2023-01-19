package jsonrpc

import (
	"context"
	"unicode/utf8"
)

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
// released or until the context is canceled.
func (r requestsSemaphore) drain(ctx context.Context) {
	for i := 0; i < cap(r); i++ {
		select {
		case <-r:
		case <-ctx.Done():
			return
		}
	}
}

// makeRequestsSemaphore creates a semaphore with the given max capacity.
func makeRequestsSemaphore(max int) requestsSemaphore {
	res := make(requestsSemaphore, max)
	for i := 0; i < max; i++ {
		res <- struct{}{}
	}
	return res
}

type slogWriter struct {
	f func(...interface{})
}

func (sw slogWriter) Write(b []byte) (int, error) {
	sw.f(string(b))
	return len(b), nil
}
