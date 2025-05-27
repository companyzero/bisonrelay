package rtdtserver

import (
	"sync"
	"time"
)

// atomicTime provides atomic access to a time.Time value.
type atomicTime struct {
	mtx    sync.Mutex
	stored bool
	t      time.Time
}

// Store the given time.
func (at *atomicTime) Store(v time.Time) {
	at.mtx.Lock()
	at.t, at.stored = v, true
	at.mtx.Unlock()
}

// Load the current time.
func (at *atomicTime) Load() (v time.Time, loaded bool) {
	at.mtx.Lock()
	v, loaded = at.t, at.stored
	at.mtx.Unlock()
	return
}

// LoadAndStore returns the current time and stores a new one.
func (at *atomicTime) LoadAndStore(v time.Time) (previous time.Time, loaded bool) {
	at.mtx.Lock()
	previous, loaded = at.t, at.stored
	at.t, at.stored = v, true
	at.mtx.Unlock()
	return
}
