package jsonrpc

import (
	"errors"
	"sync"

	"google.golang.org/protobuf/proto"
)

// requestStream is the server side of a stream: it sends replies via Send().
type requestStream struct {
	method string
	p      *peer
}

func (s *requestStream) Send(m proto.Message) error {
	return s.p.queueNotification(s.method, m)
}

type responseEvent struct {
	v    interface{}
	last bool
}

// responseStream is the client side of a stream: it receives notifications from
// the server and processes them in sequence.
type responseStream struct {
	method string
	p      *peer
	c      chan struct{}
	done   chan struct{}
	mtx    sync.Mutex
	q      []responseEvent
}

func (s *responseStream) push(v interface{}, last bool) {
	// Early check that the stream is not done.
	select {
	case <-s.done:
		return
	default:
	}

	// Enqueue.
	s.mtx.Lock()
	s.q = append(s.q, responseEvent{v: v, last: last})
	s.mtx.Unlock()

	// Alert Recv() of new item.
	go func() {
		select {
		case <-s.done:
		case s.c <- struct{}{}:
		}
	}()
}

// Recv the next value. Must only be called from a single goroutine and cannot
// be called concurrently to Close().
func (s *responseStream) Recv(msg proto.Message) error {
	// Wait until there's an item.
	var e responseEvent
	select {
	case <-s.c:
	case <-s.done:
		// Should not happen (calling Recv() after receiving an error).
		return errors.New("stream is done")
	}

	// Fetch the first queued item.
	s.mtx.Lock()
	l := len(s.q)
	if l > 0 {
		e = s.q[0]
		copy(s.q, s.q[1:])
		s.q[l-1] = responseEvent{}
		s.q = s.q[:l-1]
	} else {
		// Should not happen.
		s.mtx.Unlock()
		return errors.New("underflow in ResponseStream.q")
	}
	s.mtx.Unlock()

	// After dequeing the last item, close done to signal no more work
	// items.
	if e.last {
		close(s.done)
	}

	// Handle actual event.
	switch val := e.v.(type) {
	case error:
		return val
	case inboundMsg:
		return unmarshalOpts.Unmarshal(val.Params, msg)
	default:
		panic("unhandled case")
	}
}

// Close the stream if no more calls to Recv() will be made. Cannot be called
// concurrently to Recv().
func (s *responseStream) Close() error {
	select {
	case <-s.done:
		return errors.New("stream already closed")
	default:
		close(s.done)
	}

	// Clear the data to avoid large leaks.
	s.mtx.Lock()
	s.q = nil
	s.mtx.Unlock()
	return nil
}

func newResponseStream(p *peer, method string) *responseStream {
	return &responseStream{
		p:      p,
		method: method,
		c:      make(chan struct{}),
		done:   make(chan struct{}),
	}
}
