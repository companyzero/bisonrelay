package main

import (
	"context"
	cryptorand "crypto/rand"
	"encoding/binary"
	"math/rand/v2"
	"slices"
	"time"

	list "github.com/bahlo/generic-list-go"
	rtdtclient "github.com/companyzero/bisonrelay/rtdt/client"
	"github.com/decred/slog"
)

const routinesPerInterval = 4

type outboundBurst struct {
	BurstID    uint16
	Packets    uint16
	PacketSize sizeDistribution
	RTSess     *rtdtclient.Session

	lastSent time.Time
	seq      uint32
}

type outboundGoroutine struct {
	log      slog.Logger
	stats    *stats
	interval time.Duration

	c chan outboundBurst
}

func newOutboundGoroutine(interval time.Duration, log slog.Logger, stats *stats) *outboundGoroutine {
	return &outboundGoroutine{
		interval: interval,
		log:      log,
		stats:    stats,
		c:        make(chan outboundBurst),
	}
}

func (g *outboundGoroutine) Run(ctx context.Context) error {
	l := list.New[*outboundBurst]()
	timer := time.NewTimer(time.Hour * 24 * 365 * 10)
	var nextTimeChan <-chan time.Time

	var seed [32]byte
	cryptorand.Read(seed[:])
	rng := rand.NewChaCha8(seed)
	var buf []byte

	// Layout of data sent on every packet:
	// Offset 00: [2 bytes]: Burst index
	// Offset 02: [8 bytes]: Burst Interval
	// Offset 10: [2 bytes]: Burst Packet Count
	// Offset 12: [4 bytes]: Burst sequence
	// Offset 16: [2 bytes]: Packet index
	// Offset 18: [N bytes]: Random data
	const headerSize = 2 + 8 + 2 + 4 + 2

	for {
		var now time.Time

		select {
		case now = <-nextTimeChan:
			// Time to send.
			toSend := l.Front()
			b := toSend.Value
			ts := uint32(now.Sub(epochUnixMilli).Milliseconds())

			// Prepare header.
			sendBuf := binary.BigEndian.AppendUint16(buf, b.BurstID)
			sendBuf = binary.BigEndian.AppendUint64(sendBuf, uint64(g.interval))
			sendBuf = binary.BigEndian.AppendUint16(sendBuf, b.Packets)
			sendBuf = binary.BigEndian.AppendUint32(sendBuf, b.seq)

			for i := uint16(0); i < b.Packets; i++ {
				pktBuf := binary.BigEndian.AppendUint16(sendBuf, i)

				sendSize := b.PacketSize.Next()
				prefix := len(pktBuf)
				pktBuf = pktBuf[:prefix+sendSize]
				rng.Read(pktBuf[prefix:])

				err := b.RTSess.SendRandomData(ctx, pktBuf, ts)
				if err != nil {
					return err
				}
			}

			// This is now the last one that will need to be sent.
			b.seq++
			l.MoveToBack(toSend)
			nextTimeChan = nil
			b.lastSent = now
			g.stats.outBurstHisto.Observe(float64(now.Sub(b.lastSent).Milliseconds()))

		case newBurst := <-g.c:
			newBurst.seq = 1
			buf = slices.Grow(buf, headerSize+newBurst.PacketSize.Max())
			l.PushBack(&newBurst)
			g.stats.outBursts.Add(1)

			g.log.Infof("Running outbound burst with %d packets at "+
				"%s interval on session %s", newBurst.Packets,
				g.interval, newBurst.RTSess.LocalID())

			newBurst.lastSent = now

		case <-ctx.Done():
			return ctx.Err()
		}

		if l.Len() > 0 && nextTimeChan == nil {
			nextToSend := l.Front()
			intervalToNext := g.interval - time.Since(nextToSend.Value.lastSent) // now.Sub(nextToSend.Value.lastSent)
			timer.Reset(intervalToNext)
			nextTimeChan = timer.C
		}
	}
}
