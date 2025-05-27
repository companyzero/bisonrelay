package seqtracker

import (
	"math"
	"sync"
)

// Tracker tracks uint32 sequence numbers inside a 16-packet wide window.
//
// Assumes tracking starts at zero. On overflow, any sequence number <
// MaxUint32/2 will be accepted to reset.
//
// An empty sequence tracker is ready for use.
type Tracker struct {
	mtx sync.Mutex

	// seq is the highest sequence number received.
	seq int64

	// win is the bitmap that tracks received packets witin the receiving
	// window (i.e. packets [seq-16..seq]).
	win uint16
}

// MayAccept returns true if the packet with sequence number s should be
// accepted. This advances the state of the sequence tracker.
func (st *Tracker) MayAccept(s uint32) (accept bool) {
	const winSize = 16 // MUST match size of st.win
	const wrapSeq = math.MaxUint32 - winSize
	const wrapAcceptSeq = 1 << 31

	is := int64(s)

	st.mtx.Lock()
	d := is - st.seq
	if d > 0 {
		// Moving seq window forward.
		accept = true
		st.seq = is
		if d > winSize {
			// Completely reset window tracking.
			st.win = 1
		} else {
			// Moving forward < window size.
			st.win = st.win<<byte(d) | 1
		}
	} else if d > -winSize {
		// Seq in the past and inside window. Accept if not received
		// yet.
		mask := uint16(1) << byte(-d)
		accept = (st.win & mask) == 0
		st.win = st.win | mask
	} else if st.seq > wrapSeq && is < wrapAcceptSeq {
		// Seq wrapped. Accept it.
		accept = true
		st.seq = is
		st.win = 1
	} // Other cases are rejected.
	st.mtx.Unlock()
	return
}
