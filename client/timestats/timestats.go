package timestats

import (
	"sort"
	"sync"
	"time"
)

// Tracker tracks statistics for an event source in a concurrent safe manner.
// Tracking is done at the millisecond resolution.
//
// This is useful only for low numbers of events.
type Tracker struct {
	mtx    sync.Mutex
	i, n   int
	events []int64
}

func (t *Tracker) Add(d time.Duration) {
	ms := d.Milliseconds()

	t.mtx.Lock()
	t.events[t.i] = ms
	t.i = (t.i + 1) % len(t.events)
	t.n += 1
	t.mtx.Unlock()
}

type Quantile struct {
	N   int64
	Max int64
	Rel string
}

func (t *Tracker) Quantiles() []Quantile {
	t.mtx.Lock()
	n := int64(t.n)
	if n > int64(len(t.events)) {
		n = int64(len(t.events))
	}
	if n == 0 {
		t.mtx.Unlock()
		return []Quantile{}
	}

	// Sort.
	sorted := make([]int64, n)
	copy(sorted[:], t.events[:n])
	t.mtx.Unlock()
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

	percs := []struct {
		n int64
		d int64
		r string
	}{
		{n: 10, d: 100, r: "10%"},
		{n: 1, d: 4, r: "25%"},
		{n: 1, d: 2, r: "50%"},
		{n: 3, d: 4, r: "75%"},
		{n: 9, d: 10, r: "90%"},
		{n: 99, d: 100, r: "99%"},
	}
	res := make([]Quantile, 1, len(percs)+1)
	res[0] = Quantile{Rel: "100%", N: n, Max: sorted[len(sorted)-1]}
	lastIdx := int64(len(sorted) - 1)
	for i := len(percs) - 1; i >= 0; i-- {
		idx := n * percs[i].n / percs[i].d
		if idx == lastIdx {
			continue
		}
		if sorted[idx] == res[len(res)-1].Max {
			continue
		}
		res = append(res, Quantile{
			Rel: percs[i].r,
			Max: sorted[idx],
			N:   idx + 1,
		})
		lastIdx = idx
	}
	sort.Slice(res, func(i, j int) bool { return res[i].Max < res[i].Max })

	/*
		res := []Quantile{
			{Rel: "10%"},   // 10%
			{Rel: "25%"},   // 25%
			{Rel: "50%"},   // 50%
			{Rel: "75%"},   // 75%
			{Rel: "90%"},   // 90%
			{Rel: "99%"},   // 99%
			{Rel: "99.9%"}, // 99.9%
			{Rel: "100%"},  // 100%
		}

		for i := 0; i < n; i++ {
			e := t.events[i]
			for d := 0; d < len(res); d++ {
				if e <= res[d].Max {
					res[d].N += 1
				}
			}
		}
	*/

	return res
}

func NewTracker(n int) *Tracker {
	return &Tracker{
		events: make([]int64, n),
	}
}
