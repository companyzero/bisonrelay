package seqtracker

import (
	"math"
	randv2 "math/rand/v2"
	"testing"

	"github.com/companyzero/bisonrelay/internal/assert"
)

// TestSeqTracking tests the behavior of the seqTracker structure.
func TestSeqTracking(t *testing.T) {
	state := func(seq uint32, win uint16) *Tracker {
		return &Tracker{seq: int64(seq), win: win}
	}
	const maxseq = math.MaxUint32

	tests := []struct {
		name       string
		state      *Tracker
		seq        uint32
		wantAccept bool
		wantState  *Tracker
	}{{
		name:       "zero to one",
		state:      &Tracker{},
		seq:        1,
		wantAccept: true,
		wantState:  state(1, 0b00000000000000001),
	}, {
		name:       "one to two",
		state:      state(1, 0b00000000000000001),
		seq:        2,
		wantAccept: true,
		wantState:  state(2, 0b00000000000000011),
	}, {
		name:       "two to four", // 3 is missed
		state:      state(2, 0b00000000000000011),
		seq:        4,
		wantAccept: true,
		wantState:  state(4, 0b00000000000001101),
	}, {
		name:       "four to ten", // missed 5-10
		state:      state(4, 0b00000000000001101),
		seq:        10,
		wantAccept: true,
		wantState:  state(10, 0b0000001101000001),
	}, {
		name:       "seven", // was previously missed
		state:      state(10, 0b0000001101000001),
		seq:        7,
		wantAccept: true,
		wantState:  state(10, 0b0000001101001001),
	}, {
		name:       "two again", // was already processed
		state:      state(10, 0b0000001101001001),
		seq:        2,
		wantAccept: false,
		wantState:  state(10, 0b0000001101001001),
	}, {
		name:       "10 to 16", // missed 11-15 (filled window)
		state:      state(10, 0b0000001101001001),
		seq:        16,
		wantAccept: true,
		wantState:  state(16, 0b1101001001000001),
	}, {
		name:       "16 to 17", // moved window
		state:      state(16, 0b1101001001000001),
		seq:        17,
		wantAccept: true,
		wantState:  state(17, 0b1010010010000011),
	}, {
		name:       "17 to 23", // missed 18-22
		state:      state(17, 0b1010010010000011),
		seq:        23,
		wantAccept: true,
		wantState:  state(23, 0b0010000011000001),
	}, {
		name:       "four again", // before the window
		state:      state(23, 0b0010000011000001),
		seq:        4,
		wantAccept: false,
		wantState:  state(23, 0b0010000011000001),
	}, {
		name:       "eight", // last still on window
		state:      state(23, 0b0010000011000001),
		seq:        8,
		wantAccept: true,
		wantState:  state(23, 0b1010000011000001),
	}, {
		name:       "100", // reset window
		state:      state(23, 0b0010000011000001),
		seq:        100,
		wantAccept: true,
		wantState:  state(100, 0b0000000000000001),
	}, {
		name:       "near wrap",
		state:      state(maxseq-5, 0b0010010000001001),
		seq:        maxseq - 3,
		wantAccept: true,
		wantState:  state(maxseq-3, 0b1001000000100101),
	}, {
		name:       "wrap",
		state:      state(maxseq-5, 0b0010010000001001),
		seq:        1000,
		wantAccept: true,
		wantState:  state(1000, 0b0000000000000001),
	}, {
		name:       "dont wrap",
		state:      state(maxseq-5, 0b0010010000001001),
		seq:        maxseq/2 + 1,
		wantAccept: false,
		wantState:  state(maxseq-5, 0b0010010000001001),
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotAccept := tc.state.MayAccept(tc.seq)
			assert.DeepEqual(t, gotAccept, tc.wantAccept)
			assert.DeepEqual(t, tc.state.seq, tc.wantState.seq)
			if tc.state.win != tc.wantState.win {
				t.Fatalf("Unexpected state: got %016b, want %016b",
					tc.state.win, tc.wantState.win)
			}
		})
	}
}

// BenchmarkSeqTracking benchmarks the seqTracker structure.
func BenchmarkSeqTracking(b *testing.B) {
	start := uint32(1 << 16)
	state := &Tracker{
		seq: int64(start),
		win: 1,
	}

	rng := randv2.New(randv2.NewPCG(randv2.Uint64(), randv2.Uint64()))

	seq := start
	for i := 0; i < b.N; i++ {
		d := uint32(rng.NormFloat64() * 16)
		state.MayAccept(seq + d)
		seq++
	}
}
