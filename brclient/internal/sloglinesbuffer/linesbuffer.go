package sloglinesbuffer

import (
	"container/list"
	"sync"
)

// Listener holds a callback that is called every time a new log line is
// received.
type Listener struct {
	b  *Buffer
	cb func(s string)
}

// Close stops this listener from receiving new callbacks.
func (l *Listener) Close() {
	l.b.mtx.Lock()
	delete(l.b.listeners, l)
	l.b.mtx.Unlock()
}

// Buffer buffers slog Write() calls into a ring buffer, up to some
// number of max lines.
type Buffer struct {
	mtx       sync.Mutex
	logLines  *list.List
	listeners map[*Listener]struct{}
}

func (buff *Buffer) Write(b []byte) (int, error) {
	// Hardcoded max nb of log lines.
	maxLogLines := 100

	// Add to in-memory list of last log lines.
	buff.mtx.Lock()
	if buff.logLines == nil {
		buff.logLines = list.New()
	}
	if buff.logLines.Len() < maxLogLines {
		cb := make([]byte, len(b))
		copy(cb, b)
		buff.logLines.PushBack(cb)
	} else {
		el := buff.logLines.Front()
		cb := el.Value.([]byte)
		buff.logLines.MoveToBack(el)

		// Resize down if too large, resize up if needed. Resizing down
		// has some hysteresis to avoid too many re-allocations.
		if (cap(cb) > 512 && len(b) < 256) || cap(cb) < len(b) {
			cb = make([]byte, 0, len(b))
		} else {
			cb = cb[:0]
		}
		el.Value = append(cb, b...)
	}

	listeners := make([]*Listener, 0, len(buff.listeners))
	for lis := range buff.listeners {
		listeners = append(listeners, lis)
	}

	buff.mtx.Unlock()

	// Call any listeners.
	if len(listeners) > 0 {
		line := string(b)
		for _, lis := range listeners {
			lis.cb(line)
		}
	}

	return len(b), nil
}

// LastLogLines returns the last n log lines from the buffer.
func (buff *Buffer) LastLogLines(n int) []string {
	buff.mtx.Lock()
	if buff.logLines == nil {
		buff.mtx.Unlock()
		return []string{}
	}

	loglen := buff.logLines.Len()
	if n < 0 || loglen < n {
		n = loglen
	}

	res := make([]string, n)
	line := buff.logLines.Back()
	for i := n - 1; i >= 0; i-- {
		res[i] = string(line.Value.([]byte))
		line = line.Prev()
	}
	buff.mtx.Unlock()

	return res
}

// Listen to any Write() events by calling the specificed callback function.
// Note the callbacks are called synchronously with the Write() call, so care
// must be taken to not block for a significant amount of time.
//
// The listener should be closed by calling its Close() method once it's no
// longer needed.
func (buff *Buffer) Listen(cb func(s string)) *Listener {
	if buff.listeners == nil {
		buff.listeners = make(map[*Listener]struct{}, 1)
	}
	l := &Listener{
		b:  buff,
		cb: cb,
	}
	buff.listeners[l] = struct{}{}
	return l
}
