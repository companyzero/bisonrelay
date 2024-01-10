package server

import (
	"context"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/decred/slog"
)

type stats struct {
	bytesSent      atomic.Int64
	bytesRecv      atomic.Int64
	matomsRecv     atomic.Int64
	invoicesSent   atomic.Int64
	invoicesRecv   atomic.Int64
	subsRecv       atomic.Int64
	rmsSent        atomic.Int64
	rmsRecv        atomic.Int64
	activeSubs     atomic.Int64
	connections    atomic.Int64
	disconnections atomic.Int64
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
			bs := s.bytesSent.Load()
			br := s.bytesRecv.Load()
			mr := s.matomsRecv.Load()
			ivs := s.invoicesSent.Load()
			ivr := s.invoicesRecv.Load()
			sr := s.subsRecv.Load()
			rs := s.rmsRecv.Load()
			rr := s.rmsSent.Load()
			as := s.activeSubs.Load()
			conn := s.connections.Load()
			disc := s.disconnections.Load()
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
