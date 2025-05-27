package rtdtserver

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// UDPProcStats tracks kernel stats.
type UDPProcStats struct {
	TXQueue int
	RXQueue int
	Drops   int
}

// kernelStatsTracker is a tracker for kernel UDP stats. This is concretely
// defined on a per-OS basis.
type kernelStatsTracker interface {
	stats() (UDPProcStats, error)
}

// nullKernelStatsTracker does no tracking.
type nullKernelStatsTracker struct{}

func (_ nullKernelStatsTracker) stats() (UDPProcStats, error) {
	return UDPProcStats{}, nil
}

// stats holds server statistics.
type stats struct {
	// promEnabled is true if prometheus has been enabled.
	//
	// Note: currently, not every call site checks for this before trying
	// to access prometheus metrics.
	promEnabled bool

	reg *prometheus.Registry

	bytesRead    prometheus.Counter
	bytesWritten prometheus.Counter
	pktsRead     prometheus.Counter
	pktsWritten  prometheus.Counter

	bytesReadAtomic    atomic.Uint64
	bytesWrittenAtomic atomic.Uint64
	pktsReadAtomic     atomic.Uint64
	pktsWrittenAtomic  atomic.Uint64

	conns          prometheus.Gauge
	pendingConns   prometheus.Gauge
	sessions       prometheus.Gauge
	peers          prometheus.Gauge
	fwdDelay       prometheus.Histogram
	kernelRXQueue  *prometheus.GaugeVec
	kernelTXQueue  *prometheus.GaugeVec
	kernelDrops    *prometheus.GaugeVec
	handshakeStall *prometheus.CounterVec
	decryptFails   *prometheus.CounterVec

	noAllowanceBytes       prometheus.Counter
	noAllowanceBytesAtomic atomic.Uint64

	skippedSeqPackets *prometheus.CounterVec
}

func newStats(promEnabled bool) *stats {
	reg := prometheus.NewRegistry()
	f := promauto.With(reg)
	reg.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	reg.MustRegister(collectors.NewGoCollector())
	return &stats{
		promEnabled: promEnabled,
		reg:         reg,

		bytesRead: f.NewCounter(prometheus.CounterOpts{
			Name: "rtdt_bytes_read",
			Help: "Total bytes read",
		}),
		bytesWritten: f.NewCounter(prometheus.CounterOpts{
			Name: "rtdt_bytes_written",
			Help: "Total bytes written",
		}),
		pktsRead: f.NewCounter(prometheus.CounterOpts{
			Name: "rtdt_packets_read",
			Help: "Total number of packets read",
		}),
		pktsWritten: f.NewCounter(prometheus.CounterOpts{
			Name: "rtdt_packets_written",
			Help: "Total number of packets written",
		}),
		conns: f.NewGauge(prometheus.GaugeOpts{
			Name: "rtdt_connections",
			Help: "Active connections count",
		}),
		pendingConns: f.NewGauge(prometheus.GaugeOpts{
			Name: "rtdt_pending_connections",
			Help: "Pending handshake connections",
		}),
		sessions: f.NewGauge(prometheus.GaugeOpts{
			Name: "rtdt_sessions",
			Help: "Active session count",
		}),
		peers: f.NewGauge(prometheus.GaugeOpts{
			Name: "rtdt_peers",
			Help: "Active peer count",
		}),
		fwdDelay: f.NewHistogram(prometheus.HistogramOpts{
			Name: "rtdt_fwd_delay_microseconds",
			Help: "Histogram of per-packet forwarding delay (how long it takes to forward each inbound packet)",
			Buckets: []float64{
				1, 5, 50, 100, 250, 500, 750, 1_000, 2_500, 5_000, 10_000, 20_000, 50_000, 100_000,
			},
		}),
		kernelRXQueue: f.NewGaugeVec(prometheus.GaugeOpts{
			Name: "kernel_rx_queue_size",
			Help: "Size of the kernel RX queue for each bound UDP socket",
		}, []string{"addr"}),
		kernelTXQueue: f.NewGaugeVec(prometheus.GaugeOpts{
			Name: "kernel_tx_queue_size",
			Help: "Size of the kernel TX queue for each bound UDP socket",
		}, []string{"addr"}),
		kernelDrops: f.NewGaugeVec(prometheus.GaugeOpts{
			Name: "kernel_packet_drops",
			Help: "Number of packets dropped by kernel",
		}, []string{"addr"}),

		handshakeStall: f.NewCounterVec(prometheus.CounterOpts{
			Name: "rtdt_handshake_stalls",
			Help: "Count of handshake messages delayed due to stall",
		}, []string{"addr"}),
		decryptFails: f.NewCounterVec(prometheus.CounterOpts{
			Name: "rtdt_decryption_failures",
			Help: "Count of inbound messages that failed decryption",
		}, []string{"addr"}),
		noAllowanceBytes: f.NewCounter(prometheus.CounterOpts{
			Name: "rtdt_noallawance_bytes",
			Help: "Count of number of bytes that were not relayed due to sender having no allwance",
		}),
		skippedSeqPackets: f.NewCounterVec(prometheus.CounterOpts{
			Name: "rtdt_skipped_seq_packets",
			Help: "Count of packets ignored because their sequence number is outside the acceptable window",
		}, []string{"addr"}),
	}
}

// runReportStatsLoop runs a loop to report basic stats.
func (s *Server) runReportStatsLoop(ctx context.Context, reportInterval time.Duration) error {
	if reportInterval <= 0 {
		s.log.Infof("Logging of stats is disabled")
		return nil
	}

	ticker := time.NewTicker(reportInterval)
	var tickTime, lastTick time.Time
	tickTime = time.Now()

	s.log.Infof("Running report stats loop with interval %s", reportInterval)

	var bytesRead, pktsRead, bytesWritten, pktsWritten uint64
	for {
		lastTick = tickTime

		select {
		case <-ctx.Done():
			return ctx.Err()
		case tickTime = <-ticker.C:
		}

		bytesRead = s.stats.bytesReadAtomic.Swap(0)
		bytesWritten = s.stats.bytesWrittenAtomic.Swap(0)
		pktsRead = s.stats.pktsReadAtomic.Swap(0)
		pktsWritten = s.stats.pktsWrittenAtomic.Swap(0)

		if bytesRead|bytesWritten|pktsRead|pktsWritten == 0 {
			// Skip if there are no stats.
			continue
		}

		dt := tickTime.Sub(lastTick)
		if dt == 0 {
			continue // Should not happen.
		}

		dts := float64(dt.Milliseconds()) / 1000

		rbr := float64(bytesRead) / dts
		rpr := float64(pktsRead) / dts
		wbr := float64(bytesWritten) / dts
		wpr := float64(pktsWritten) / dts

		s.log.Infof("Stats for the last %s - "+
			"IN: %8s (%7sB/sec) %8s Pkt (%7s/sec) ; "+
			"OUT: %8s (%7sB/sec) %8s Pkt (%7s/sec)",
			dt.Round(time.Millisecond),
			hbytes(bytesRead), hrate(rbr), hcount(pktsRead), hrate(rpr),
			hbytes(bytesWritten), hrate(wbr), hcount(pktsWritten), hrate(wpr),
		)
	}
}
