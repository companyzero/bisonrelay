package server

import (
	"context"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/decred/slog"
)

type counter int64

func (c *counter) add(amt int64) {
	atomic.AddInt64((*int64)(c), amt)
}

func (c *counter) value() int64 {
	return atomic.LoadInt64((*int64)(c))
}

type stats struct {
	bytesSent      counter
	bytesRecv      counter
	matomsRecv     counter
	invoicesSent   counter
	invoicesRecv   counter
	subsRecv       counter
	rmsSent        counter
	rmsRecv        counter
	activeSubs     counter
	connections    counter
	disconnections counter
}

// hbytes == "human bytes"
func hbytes(i int64) string {
	switch {
	case i < 1e3:
		return strconv.FormatInt(i, 10) + "B"
	case i < 1e6:
		f := float64(i)
		return strconv.FormatFloat(f/1e3, 'f', 2, 64) + "KB"
	case i < 1e9:
		f := float64(i)
		return strconv.FormatFloat(f/1e6, 'f', 2, 64) + "MB"
	case i < 1e12:
		f := float64(i)
		return strconv.FormatFloat(f/1e9, 'f', 2, 64) + "GB"
	case i < 1e15:
		f := float64(i)
		return strconv.FormatFloat(f/1e12, 'f', 2, 64) + "TB"
	default:
		return strconv.FormatInt(i, 10)
	}
}

func (s *stats) runPrinter(ctx context.Context, log slog.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case <-time.After(10 * time.Second):
			// Not fetching all under a single lock makes the stats
			// less exact, but more performant.
			bs := s.bytesSent.value()
			br := s.bytesRecv.value()
			mr := s.matomsRecv.value()
			ivs := s.invoicesSent.value()
			ivr := s.invoicesRecv.value()
			sr := s.subsRecv.value()
			rs := s.rmsRecv.value()
			rr := s.rmsSent.value()
			as := s.activeSubs.value()
			conn := s.connections.value()
			disc := s.disconnections.value()
			online := conn - disc

			log.Infof("Server Stats: "+
				"bytes %s in / %s out, "+
				"invoices: %d gen / %d paid, "+
				"dcr recv: %.8f, "+
				"subs: %d total / %d active, "+
				"RMs recv %d / sent %d, "+
				"conns %d in / %d out / %d online",
				hbytes(br), hbytes(bs),
				ivs, ivr,
				float64(mr)/1e11,
				sr, as,
				rs, rr,
				conn, disc, online)
		}
	}
}
