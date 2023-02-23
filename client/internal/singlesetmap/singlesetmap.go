package singlesetmap

import "sync"

// Map is a (bit)map with entries that can only be set once. The Set()
// operation returns whether the entry was previously set.
type Map[K comparable] struct {
	m   map[K]struct{}
	mtx sync.Mutex
}

// Set marks the passed entry key as set. Returns true if the entry was
// previously set, false if this operation actually set the entry to true.
func (m *Map[K]) Set(k K) (wasSet bool) {
	m.mtx.Lock()
	if m.m == nil {
		m.m = make(map[K]struct{}, 1)
	}
	_, wasSet = m.m[k]
	if !wasSet {
		m.m[k] = struct{}{}
	}
	m.mtx.Unlock()
	return
}
