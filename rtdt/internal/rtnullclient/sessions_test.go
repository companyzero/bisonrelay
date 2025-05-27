package main

import (
	"reflect"
	"testing"
	"time"
)

type clock struct {
	now time.Time
	d   time.Duration
}

func (c *clock) tick() time.Time {
	c.now = c.now.Add(c.d)
	return c.now
}

func (c *clock) tickN(n int) time.Time {
	c.now = c.now.Add(time.Duration(n) * c.d)
	return c.now
}

func TestInBurstReportsSinglePacket(t *testing.T) {
	t.Parallel()

	sec := time.Second
	c := &clock{
		now: time.Date(2024, 10, 1, 10, 0, 0, 0, time.Local),
		d:   sec,
	}

	type report struct {
		missed int
		duped  int
		delay  time.Duration
	}

	steps := []struct {
		rt   time.Time // recvTime
		bs   uint32    // bucketSeq
		reps []report
		dis  []uint32 // report discarded
	}{
		// Fill entire window.
		{rt: c.tick(), bs: 10},
		{rt: c.tick(), bs: 11, reps: []report{{delay: sec}}},
		{rt: c.tick(), bs: 12, reps: []report{{delay: sec}}},
		{rt: c.tick(), bs: 13, reps: []report{{delay: sec}}},
		{rt: c.tick(), bs: 14, reps: []report{{delay: sec}}},
		{rt: c.tick(), bs: 15, reps: []report{{delay: sec}}},

		// Receive old, outside window, reported discarded.
		{rt: c.tick(), bs: 0, dis: []uint32{0}},
		{rt: c.tick(), bs: 1, dis: []uint32{1}},
		{rt: c.tick(), bs: 9, dis: []uint32{9}},
		{rt: c.tick(), bs: 10, dis: []uint32{10}},

		// Receive new, completely erasing the window. Reported
		// discarded.
		{rt: c.tick(), bs: 20, dis: []uint32{11, 12, 13, 14, 15}},
		{rt: c.tick(), bs: 33, dis: []uint32{20}},

		// Missed one during a window.
		{rt: c.tick(), bs: 43, dis: []uint32{33}}, // Reset
		{rt: c.tick(), bs: 44, reps: []report{{delay: sec}}},
		// 45 is missing
		{rt: c.tickN(2), bs: 46, reps: []report{{delay: 2 * sec}}},
		{rt: c.tick(), bs: 47, reps: []report{{delay: sec}}},
		{rt: c.tick(), bs: 48, reps: []report{{delay: sec}}},
		{rt: c.tick(), bs: 49, reps: []report{{delay: sec}}},
		{rt: c.tick(), bs: 50, reps: []report{{missed: 1}, {delay: sec}}},

		// Missed 3 during a window.
		{rt: c.tick(), bs: 60, dis: []uint32{46, 47, 48, 49, 50}}, // Reset
		// Missed 61, 62, 63
		{rt: c.tickN(3), bs: 64, reps: []report{{delay: 3 * sec}}},
		{rt: c.tick(), bs: 65, reps: []report{{delay: sec}}},
		{rt: c.tick(), bs: 66, reps: []report{{missed: 1}, {delay: sec}}},
		{rt: c.tick(), bs: 67, reps: []report{{missed: 1}, {delay: sec}}},
		{rt: c.tick(), bs: 68, reps: []report{{missed: 1}, {delay: sec}}},
		{rt: c.tick(), bs: 69, reps: []report{{delay: sec}}},

		// Received out of order. Delay is calculated based on a prior
		// bucket because we need ordering for the next layer.
		{rt: c.tick(), bs: 80, dis: []uint32{65, 66, 67, 68, 69}}, // Reset
		{rt: c.tick(), bs: 84, reps: []report{{delay: sec}}},
		{rt: c.tick(), bs: 83, reps: []report{{delay: 2 * sec}}},
		{rt: c.tick(), bs: 82, reps: []report{{delay: 3 * sec}}},
		{rt: c.tick(), bs: 81, reps: []report{{delay: 4 * sec}}}, // 4 seconds elpased between 80 and 81

		// Duplicated packets.
		{rt: c.tick(), bs: 90, dis: []uint32{80, 81, 82, 83, 84}}, // Reset
		{rt: c.tick(), bs: 91, reps: []report{{delay: sec}}},
		{rt: c.now, bs: 91}, // "Simulateneous" duplication
		{rt: c.tick(), bs: 92, reps: []report{{delay: sec}}},
		{rt: c.tick(), bs: 92}, // Delayed duplication
		{rt: c.tick(), bs: 93, reps: []report{{delay: 2 * sec}}},
		{rt: c.tick(), bs: 94, reps: []report{{delay: sec}}},
		{rt: c.tick(), bs: 93}, // Out of order duplication
		{rt: c.tick(), bs: 95, reps: []report{{delay: 2 * sec}}},
		{rt: c.tick(), bs: 96, reps: []report{{duped: 1}, {delay: sec}}}, // 91 duplicated
		{rt: c.tick(), bs: 97, reps: []report{{duped: 1}, {delay: sec}}}, // 92 duplicated
		{rt: c.tick(), bs: 98, reps: []report{{duped: 1}, {delay: sec}}}, // 93 duplicated
	}

	var gotReports []report
	addReport := func(c *inBurst, missed, duplicated int, burstDelay time.Duration, packetDelays []time.Duration) {
		gotReports = append(gotReports, report{missed: missed, duped: duplicated, delay: burstDelay})
	}

	var gotDiscarded []uint32
	addDiscarded := func(c *inBurst, seq uint32) {
		gotDiscarded = append(gotDiscarded, seq)
	}

	burst := newInBurst(sec, 5*sec, 1, addReport, addDiscarded)
	if len(burst.buffer) != 5 {
		t.Fatalf("Wrong len: got %d, want %d", len(burst.buffer), 5)
	}

	for i, step := range steps {
		// Received this packet.
		burst.received(step.rt, step.bs, 0)

		// Check if the reports match.
		t.Logf("At %02d - reps %#v, dis %#v", i, gotReports, gotDiscarded)
		if len(step.reps) != len(gotReports) || (len(step.reps) > 0 && !reflect.DeepEqual(step.reps, gotReports)) {
			t.Errorf("At %02d (seq %d) failed reps check", i, step.bs)
			t.Fatalf("At %02d - got reps %#v, want reps %#v", i,
				gotReports, step.reps)
		}
		if len(step.dis) != len(gotDiscarded) || (len(step.dis) > 0 && !reflect.DeepEqual(step.dis, gotDiscarded)) {
			t.Errorf("At %02d (seq %d) failed dis check", i, step.bs)
			t.Fatalf("At %02d - got dis %#v, want dis %#v", i,
				gotDiscarded, step.dis)
		}

		// Clear for next iteration.
		gotReports = gotReports[:0]
		gotDiscarded = gotDiscarded[:0]
	}
}

func BenchmarkInBurstReceived(b *testing.B) {
	addReport := func(c *inBurst, missed, duplicated int, burstDelay time.Duration, packetDelays []time.Duration) {}

	addDiscarded := func(c *inBurst, seq uint32) {}

	sec := time.Second
	burst := newInBurst(sec, 5*sec, 1, addReport, addDiscarded)

	now := time.Date(2024, 10, 1, 10, 0, 0, 0, time.Local)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		burst.received(now, uint32(i), 0)
	}
}
