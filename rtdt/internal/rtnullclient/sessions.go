package main

import (
	"math/rand/v2"
	"sync"
	"time"
)

type sizeDistribution interface {
	Next() int
	Max() int
}

type uniformDistribution struct {
	minVal int
	maxVal int
}

func (ud uniformDistribution) Max() int {
	return ud.maxVal
}

func (ud uniformDistribution) Next() int {
	return ud.minVal + rand.IntN(ud.maxVal-ud.minVal)
}

type outBurst struct {
	// ID         uint16
	Interval   time.Duration
	Packets    uint16
	PacketSize sizeDistribution
}

type inBurstIntervalPackets struct {
	Count int
}

type inBurstInterval struct {
	Seq          uint32
	CompleteTime time.Time
	Packets      []inBurstIntervalPackets
}

func (b *inBurstInterval) isComplete() bool {
	for i := range b.Packets {
		if b.Packets[i].Count == 0 {
			return false
		}
	}
	return true
}

func (b *inBurstInterval) clear() {
	b.Seq = 0
	b.CompleteTime = time.Time{}
	for i := range b.Packets {
		b.Packets[i].Count = 0
	}
}

func (b *inBurstInterval) clearToSeq(seq uint32) {
	if b.Seq == seq {
		return
	}
	b.clear()
	b.Seq = seq
}

type burstReportFunc func(c *inBurst, missed, duplicated int, burstDelay time.Duration, packetDelays []time.Duration)
type burstReportDiscarded func(c *inBurst, seq uint32)

type inBurst struct {
	Interval time.Duration
	Packets  uint16

	buffer []inBurstInterval
	hi     int

	report          burstReportFunc
	reportDiscarded burstReportDiscarded
}

func (b *inBurst) clearBufferRange(start, end int) {
	for i := start; i < end; i++ {
		b.buffer[i].clear()
	}
}

func floorMod(a, b int) int {
	return (a%b + b) % b
}

func (b *inBurst) reportLeaving(i int, newSeq uint32) {
	burst := &b.buffer[i]
	if burst.Seq == 0 || burst.Seq == newSeq {
		// Empty or already reported.
		return
	}

	var missed, duplicated int
	for i := range burst.Packets {
		packet := &burst.Packets[i]
		if packet.Count == 0 {
			missed++
		} else {
			duplicated += packet.Count - 1
		}
	}

	if missed != 0 || duplicated != 0 {
		b.report(b, missed, duplicated, 0, nil)
	}
}

func (b *inBurst) reportCompleted(i int) {
	burst := &b.buffer[i]

	// Find the first older that completed to measure time against. If none
	// completed, consider this a new baseline.
	for delta := 1; delta < len(b.buffer); delta++ {
		prevIdx := floorMod(i-delta, len(b.buffer))
		prevBurst := &b.buffer[prevIdx]
		if prevBurst.Seq == 0 || prevBurst.Seq != burst.Seq-uint32(delta) || !prevBurst.isComplete() {
			continue
		}

		burstDelay := burst.CompleteTime.Sub(prevBurst.CompleteTime)
		b.report(b, 0, 0, burstDelay, nil)
		return
	}
}

func (b *inBurst) received(recvTime time.Time, bucketSeq uint32, pktIndex uint16) {
	if pktIndex >= b.Packets {
		return // Avoid panic
	}

	bseq, hiseq := int(bucketSeq), -1
	if b.hi > -1 {
		hiseq = int(b.buffer[b.hi].Seq)
	}

	var i int = int(bucketSeq) % len(b.buffer)
	if b.hi == -1 {
		b.hi = i
	} else if hiseq-bseq >= len(b.buffer) {
		// Timestamp too old, outside tracking window interval.
		b.reportDiscarded(b, bucketSeq)
		return
	} else if bseq-hiseq >= len(b.buffer) {
		// The entirety of the current buffer must be discarded,
		// because we received something too far in the future. Report
		// discarded in seq order.
		for i := b.hi + 1; i <= b.hi+len(b.buffer); i++ {
			im := i % len(b.buffer)
			if b.buffer[im].Seq != 0 {
				b.reportDiscarded(b, b.buffer[im].Seq)
			}
		}
		b.clearBufferRange(0, len(b.buffer))
		b.hi = i
	} else if bseq > hiseq {
		// Mark everything between seq and hiseq as leaving and
		// potentially missed (if not received in time).
		for seq := hiseq + 1; seq <= bseq; seq++ {
			i := seq % len(b.buffer)
			b.reportLeaving(i, uint32(seq))
			b.buffer[i].clearToSeq(uint32(seq))
		}
		b.hi = i
	}

	// Mark the received packet.
	burst := &b.buffer[i]
	burst.Seq = bucketSeq
	pkt := &burst.Packets[pktIndex]
	pkt.Count++

	// Time it first completed.
	if burst.CompleteTime.IsZero() && burst.isComplete() {
		burst.CompleteTime = recvTime
		b.reportCompleted(i)
	}
}

func newInBurst(burstInterval, windowInterval time.Duration, packets uint16,
	report burstReportFunc, reportDiscarded burstReportDiscarded) *inBurst {

	if burstInterval > windowInterval {
		panic("burstInterval > windowInterval")
	}
	if packets == 0 {
		panic("packets == 0")
	}
	bufferSize := int(windowInterval / burstInterval)
	buffer := make([]inBurstInterval, bufferSize)
	for i := range buffer {
		buffer[i].Packets = make([]inBurstIntervalPackets, packets)
	}
	return &inBurst{
		Interval:        burstInterval,
		Packets:         packets,
		buffer:          buffer,
		report:          report,
		reportDiscarded: reportDiscarded,
		hi:              -1,
	}
}

type inSession struct {
	mtx    sync.Mutex
	Bursts []*inBurst
}
