package main

import (
	"context"
	"math/rand"
	"sync/atomic"
	"time"

	"github.com/companyzero/bisonrelay/internal/audio"
	"github.com/decred/slog"
)

type inputPacket struct {
	data []byte
	ts   uint32
}

type simstream struct {
	cs        *audio.CaptureStream
	ps        *audio.PlaybackStream
	inputChan chan inputPacket
	bufChan   chan []byte // ring of buffers

	minDelayMs      atomic.Uint32
	meanDelayMs     atomic.Uint32
	stdDevDelayMs   atomic.Uint32
	packetLossMilli atomic.Int32
}

func (ss *simstream) run(ctx context.Context) error {
	for {
		var data []byte
		var ts uint32

		// Wait for next packet.
		select {
		case in := <-ss.inputChan:
			data, ts = in.data, in.ts
		case <-ctx.Done():
			return nil
		}

		// Actually process.
		ss.ps.Input(data, ts)

		// Return buffer for reuse.
		select {
		case ss.bufChan <- data[:0]:
		default:
			// Ok if bufchan is full.
		}
	}
}

func (ss *simstream) capturedPacket(ctx context.Context, data []byte, timestamp uint32) error {
	// Check if this packet should be dropped.
	packetLossMilli := ss.packetLossMilli.Load()
	if rand.Int31n(1000) < packetLossMilli {
		// Drop.
		return nil
	}

	// Simulate long pause at the start: play from timestamps 1s to 5s,
	// then drop 5s and resume playing.
	if timestamp < 1000 || (timestamp > 5000 && timestamp < 10000) {
		return nil
	}

	var buf []byte
	select {
	case buf = <-ss.bufChan:
	default:
		buf = make([]byte, 0, max(1024, len(data)))
	}

	// Copy (so the capture stream can reuse).
	buf = append(buf, data...)

	// Determine the delay for this packet.
	minDelay, meanDelay, stdDevDelay := ss.minDelayMs.Load(), ss.meanDelayMs.Load(), ss.stdDevDelayMs.Load()
	extraDelay := uint32(rand.NormFloat64()*float64(stdDevDelay) + float64(meanDelay))
	extraDelay = max(extraDelay, 0)
	delay := minDelay + extraDelay

	// Delay this packet in the network.
	go func() {
		select {
		case <-time.After(time.Duration(delay) * time.Millisecond):
			// Packet arrived in destination.
			ss.inputChan <- inputPacket{data: buf, ts: timestamp}
		case <-ctx.Done():
		}
	}()

	return nil
}

func newSimStream(ctx context.Context, noterec *audio.NoteRecorder, log slog.Logger) (*simstream, error) {
	soundChanged := func(hasSound bool) {
		if hasSound {
			log.Infof("Sound detected as starting")
		} else {
			log.Infof("Sound detected as ending")
		}
	}
	ps := noterec.PlaybackStream(ctx, soundChanged)
	ss := &simstream{
		ps:        ps,
		inputChan: make(chan inputPacket, 10*50), // 10 seconds
		bufChan:   make(chan []byte, 10*50),
	}
	ss.minDelayMs.Store(200)
	ss.meanDelayMs.Store(100)
	ss.stdDevDelayMs.Store(40)
	ss.packetLossMilli.Store(100)

	// Create a few buffers.
	for i := 0; i < 10; i++ {
		ss.bufChan <- make([]byte, 0, 1024)
	}

	var err error
	ss.cs, err = noterec.CaptureStream(ctx, ss.capturedPacket)
	if err != nil {
		return nil, err
	}

	return ss, nil
}
