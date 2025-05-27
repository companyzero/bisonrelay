package main

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type stats struct {
	burstHisto        prometheus.Histogram
	burstNegHisto     prometheus.Histogram
	burstFactorHisto  prometheus.Histogram
	outBurstHisto     prometheus.Histogram
	bytesRead         prometheus.Counter
	bytesWritten      prometheus.Counter
	inBursts          prometheus.Gauge
	outBursts         prometheus.Gauge
	missedPackets     prometheus.Counter
	duplicatedPackets prometheus.Counter
	absDelay          prometheus.Histogram
	discardedPackets  prometheus.Counter
	inPackets         prometheus.Counter
	outPackets        prometheus.Counter

	bytesReadAtomic     atomic.Uint64
	bytesWrittenAtomic  atomic.Uint64
	pktsReadAtomic      atomic.Uint64
	pktsWrittenAtomic   atomic.Uint64
	discardedPktsAtomic atomic.Uint64
}

func newStats() *stats {
	return &stats{
		burstHisto: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "rtdt_burst_delay_delta",
			Help:    "Difference (in milliseconds) between target and observed inter-burst completion delay",
			Buckets: prometheus.ExponentialBucketsRange(1, 1000, 20),
		}),
		burstNegHisto: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "rtdt_burst_delay_delta_neg",
			Help:    "Difference (in milliseconds) between target and observed inter-burst completion delay, when it is negative",
			Buckets: prometheus.ExponentialBucketsRange(1, 2*1000, 5),
		}),
		burstFactorHisto: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "rtdt_burst_delay_ratio",
			Help:    "Ratio between target and observed inter-burst completion delay. 1 == perfect timing.",
			Buckets: []float64{0.5, 0.75, 0.9, 1, 1.05, 1.1, 1.25, 1.5, 1.75, 2, 2.5, 3, 4, 5, 7, 10},
		}),

		absDelay: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "rtdt_absolute_delay",
			Help:    "Absolute packet latency in milliseconds. Assumes the clocks are synchronized.",
			Buckets: prometheus.ExponentialBucketsRange(10, 1000, 20),
		}),

		outBurstHisto: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "rtdt_out_burst_interval",
			Help:    "Interval (milliseconds) between consecutive successful outbound bursts",
			Buckets: prometheus.ExponentialBucketsRange(18, 1000, 20),
		}),

		bytesRead: promauto.NewCounter(prometheus.CounterOpts{
			Name: "rtdt_bytes_read",
			Help: "Total bytes read in the random stream",
		}),
		bytesWritten: promauto.NewCounter(prometheus.CounterOpts{
			Name: "rtdt_bytes_written",
			Help: "Total bytes written in the random stream",
		}),

		inBursts: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "rtdt_in_bursts",
			Help: "Number of active inbound bursts",
		}),
		outBursts: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "rtdt_out_bursts",
			Help: "Number of active outbound bursts",
		}),

		missedPackets: promauto.NewCounter(prometheus.CounterOpts{
			Name: "rtdt_missed_packets",
			Help: "Count of missed packets",
		}),
		duplicatedPackets: promauto.NewCounter(prometheus.CounterOpts{
			Name: "rtdt_duplicated_packets",
			Help: "Count of duplicated packets",
		}),
		discardedPackets: promauto.NewCounter(prometheus.CounterOpts{
			Name: "rtdt_discarded_packets",
			Help: "Count of discarded packets (packets outside the receiving window)",
		}),

		inPackets: promauto.NewCounter(prometheus.CounterOpts{
			Name: "rtdt_in_packets",
			Help: "Count of received packets",
		}),
		outPackets: promauto.NewCounter(prometheus.CounterOpts{
			Name: "rtdt_out_packets",
			Help: "Count of sent packets",
		}),
	}
}

// runReportStatsLoop runs a loop to report basic stats.
func (c *client) runReportStatsLoop(ctx context.Context, reportInterval time.Duration) error {
	if reportInterval <= 0 {
		c.log.Infof("Logging of stats is disabled")
		return nil
	}

	ticker := time.NewTicker(reportInterval)
	var tickTime, lastTick time.Time
	tickTime = time.Now()

	c.log.Infof("Running report stats loop with interval %s", reportInterval)

	var bytesRead, pktsRead, bytesWritten, pktsWritten, pktsLost uint64
	for {
		lastTick = tickTime

		select {
		case <-ctx.Done():
			return ctx.Err()
		case tickTime = <-ticker.C:
		}

		bytesRead = c.stats.bytesReadAtomic.Swap(0)
		bytesWritten = c.stats.bytesWrittenAtomic.Swap(0)
		pktsRead = c.stats.pktsReadAtomic.Swap(0)
		pktsWritten = c.stats.pktsWrittenAtomic.Swap(0)
		pktsLost = c.stats.discardedPktsAtomic.Swap(0)

		if bytesRead|bytesWritten|pktsRead|pktsWritten|pktsLost == 0 {
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

		c.log.Infof("Stats for the last %s - "+
			"IN: %8s (%7sB/sec) %8s Pkt (%7s/sec) ; "+
			"OUT: %8s (%7sB/sec) %8s Pkt (%7s/sec) ; LOST: %s",
			dt.Round(time.Millisecond),
			hbytes(bytesRead), hrate(rbr), hcount(pktsRead), hrate(rpr),
			hbytes(bytesWritten), hrate(wbr), hcount(pktsWritten), hrate(wpr),
			hcount(pktsLost),
		)
	}
}
