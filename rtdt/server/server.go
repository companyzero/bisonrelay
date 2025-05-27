package rtdtserver

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"slices"
	"sync"
	"time"

	"github.com/companyzero/bisonrelay/rpc"
	"github.com/companyzero/bisonrelay/zkidentity"
	"github.com/decred/slog"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/sync/errgroup"
)

// config determines a server config.
type config struct {
	listenAddrs      []*net.UDPAddr
	listeners        []*net.UDPConn
	privKey          *zkidentity.FixedSizeSntrupPrivateKey
	cookieKey        *zkidentity.FixedSizeSymmetricKey
	decodeCookieKeys []*zkidentity.FixedSizeSymmetricKey
	promAddr         string
	log              slog.Logger
	readRoutines     int

	// ignoreKernelStats is set to true during testing to avoid wasting
	// time tracking kernel stats.
	ignoreKernelStats bool

	// maxPingInterval defines when conns are dropped due to not receiving
	// any messages.
	// minPingInterval defines when conns are dropped due to sending pings
	// too often.
	maxPingInterval time.Duration
	minPingInterval time.Duration

	// timeoutLoopTickerInterval is the interval between iterations of the
	// timeoutStaleConns.
	timeoutLoopTickerInterval time.Duration

	// dropPaymentLoopInterval is the interval to run the loop to drop
	// payments that have elpased their time.
	dropPaymentLoopInterval time.Duration

	// replyErrorCodes is set to true to send back error codes when the
	// client fails some server action. This is only set to true during
	// tests.
	replyErrorCodes bool

	// maxBanScore is the ban score after which connections are dropped.
	maxBanScore uintptr

	// logReadLoopErrors is set to true to log errors triggered inside the
	// read loop. Clients can cause a large number of errors to be generated
	// in the read loop, so they are only logged during testing.
	logReadLoopErrors bool

	// sessListingInterval is the interval to send repeated reports about
	// current members of a session to all participants.
	// minSessListingInterval is the minimum time to wait before sending a
	// listing.
	sessListingInterval    time.Duration
	minSessListingInterval time.Duration
	disableForceListing    bool // Set to true in some tests

	// rotateCookieLifetime is how long a rotation cookie is valid for.
	rotateCookieLifetime time.Duration

	// statsReportInterval is the interval to log stats. If zero, stats are
	// not logged.
	statsReportInterval time.Duration
}

// fillConfig fills a new config with the default config values, then applies
// all specified options.
func fillConfig(opts ...Option) config {
	cfg := config{
		log:                       slog.Disabled,
		maxPingInterval:           rpc.RTDTMaxPingInterval,
		minPingInterval:           rpc.RTDTDefaultMinPingInterval,
		readRoutines:              1,
		dropPaymentLoopInterval:   time.Hour,
		maxBanScore:               50,
		timeoutLoopTickerInterval: 10 * time.Second,
		sessListingInterval:       30 * time.Second,
		minSessListingInterval:    5 * time.Second,
		rotateCookieLifetime:      5 * time.Minute,
		statsReportInterval:       10 * time.Second,
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	return cfg
}

// Option is a functional server config option.
type Option func(c *config)

// WithListenAddrs establishes the listening addresses of the server. Both
// listen addresses and raw listeners can be used to start the server.
func WithListenAddrs(addrs ...*net.UDPAddr) Option {
	return func(c *config) {
		c.listenAddrs = addrs
	}
}

// WithListeners sets raw UDP listeners to be used with the server. Both listen
// addresses and raw listeners can be used to start the server.
func WithListeners(listeners ...*net.UDPConn) Option {
	return func(c *config) {
		c.listeners = listeners
	}
}

// WithPrometheusListenAddr sets the address to offer Prometheus metrics
// endpoint collection.
func WithPrometheusListenAddr(addr string) Option {
	return func(c *config) {
		c.promAddr = addr
	}
}

// WithPrivateKey sets the private key used for client-server encryption.
func WithPrivateKey(pk *zkidentity.FixedSizeSntrupPrivateKey) Option {
	return func(c *config) {
		c.privKey = pk
	}
}

// WithCookieKey sets the symmetric key to use for encrypting cookies.
func WithCookieKey(key *zkidentity.FixedSizeSymmetricKey, decodeKeys []*zkidentity.FixedSizeSymmetricKey) Option {
	return func(c *config) {
		c.cookieKey = key
		c.decodeCookieKeys = decodeKeys
	}
}

// WithPerListenerReadRoutines sets the number of reading goroutines to use for
// processing each listener's incoming packets.
func WithPerListenerReadRoutines(i int) Option {
	if i <= 0 {
		panic("must have at least one read routine per listener")
	}

	return func(c *config) {
		c.readRoutines = i
	}
}

// WithLogger sets up the server to use the logger. Logger MUST NOT be nil.
func WithLogger(l slog.Logger) Option {
	return func(c *config) {
		c.log = l
	}
}

// WithIgnoreKernelStats disables tracking kernel stats. Only useful for
// testing.
func WithIgnoreKernelStats() Option {
	return func(c *config) {
		c.ignoreKernelStats = true
	}
}

// WithLogAndReplyErrors enables logging and replying of action errors.
//
// NOTE: this should only be used for temporary debugging or automated testing.
func WithLogAndReplyErrors() Option {
	return func(c *config) {
		c.logReadLoopErrors = true
		c.replyErrorCodes = true
	}
}

// WithLogErrors enables logging of remotely generated errors.
//
// NOTE: this should only be used for temporary debugging, because it may
// significantly increase log sizes.
func WithLogErrors() Option {
	return func(c *config) {
		c.logReadLoopErrors = true
	}
}

// WithReportStatsInterval sets the interval to log stats. If set to zero,
// reporting is disabled.
func WithReportStatsInterval(interval time.Duration) Option {
	return func(c *config) {
		c.statsReportInterval = interval
	}
}

// Server is an RTDT server.
type Server struct {
	cfg config
	log slog.Logger

	stats    *stats
	sessions *sessions

	// payments tracks payments redeemed by their payment tag.
	paymentsMtx sync.Mutex
	payments    map[uint64]*payment

	forceSessListChan    chan *session
	forceAllSessListChan chan struct{} // Used in tests.

	// rotPayments tracks payments made to rotate session cookies.
	rotPaymentsMtx sync.Mutex
	rotPayments    map[uint64]time.Time

	listenersMtx sync.Mutex
	listeners    []*listener
}

func newServer(cfg *config) (*Server, error) {
	// Hints to size the various containers. Sizing these for large(ish)
	// numbers at startup time makes benchmarking easier.
	const sessionsSizeHint = 60000
	const paymentsSizeHint = 240000

	s := &Server{
		cfg:   *cfg,
		log:   cfg.log,
		stats: newStats(cfg.promAddr != ""),
		sessions: &sessions{
			sessions: make(map[sessionID]*session, sessionsSizeHint),
		},
		payments:             make(map[uint64]*payment, paymentsSizeHint),
		forceSessListChan:    make(chan *session, 100),
		forceAllSessListChan: make(chan struct{}, 1),
		rotPayments:          make(map[uint64]time.Time, 1000),
	}
	return s, nil
}

// New creates a new RTDT server.
func New(opts ...Option) (*Server, error) {
	cfg := fillConfig(opts...)
	return newServer(&cfg)
}

// totalConnCounts returns the total number of existing and pending connections
// across all listeners.
func (s *Server) totalConnCounts() (conns int64, pending int64) {
	s.listenersMtx.Lock()
	for _, l := range s.listeners {
		conns += l.connsCount.Load()
		pending += l.pendingCount.Load()
	}
	s.listenersMtx.Unlock()
	return
}

// runPrometheusListener runs the Prometheus metrics endpoint in the given
// address.
func (s *Server) runPrometheusListener(ctx context.Context, addr string) error {
	mux := http.NewServeMux()
	promHandler := promhttp.InstrumentMetricHandler(
		s.stats.reg, promhttp.HandlerFor(s.stats.reg, promhttp.HandlerOpts{}),
	)
	mux.Handle("/metrics", promHandler)
	hs := http.Server{
		Addr:        addr,
		BaseContext: func(net.Listener) context.Context { return ctx },
		Handler:     mux,
	}
	s.log.Infof("Exposing prometheus metrics on %s", addr)
	go func() {
		<-ctx.Done()
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		hs.Shutdown(ctx)
	}()
	return hs.ListenAndServe()
}

// collectListenerStats collects kernel stats for the given listener.
func (s *Server) collectListenerStats(ctx context.Context, l *listener) error {
	ticker := time.NewTicker(5 * time.Second)

	addr := l.Addr().String()
	rxQueueStat := s.stats.kernelRXQueue.With(prometheus.Labels{"addr": addr})
	txQueueStat := s.stats.kernelTXQueue.With(prometheus.Labels{"addr": addr})
	dropsStat := s.stats.kernelDrops.With(prometheus.Labels{"addr": addr})

	var prev UDPProcStats
	var prevConns, prevPending int64
	for {
		select {
		case <-ticker.C:
			stats := l.UDPStats()
			rxQueueStat.Set(float64(stats.RXQueue))
			txQueueStat.Set(float64(stats.TXQueue))
			dropsStat.Set(float64(stats.Drops))

			if stats.RXQueue != prev.RXQueue || stats.TXQueue != prev.TXQueue || stats.Drops != prev.Drops {
				s.log.Infof("%s kernel stats: Queues RX %d TX %d, Drops %d",
					addr, stats.RXQueue, stats.TXQueue, stats.Drops)
			}
			prev = stats

			conns, pending := l.connsCount.Load(), l.pendingCount.Load()
			if prevConns != conns || prevPending != pending {
				s.log.Infof("%s connections: %d, pending conns: %d",
					addr, conns, pending)
			}
			prevConns, prevPending = conns, pending

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// dropStalePayments runs a loop to remove entries from the payments map after
// they can no longer be redeemed due to timeout.
func (s *Server) dropStalePayments(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(s.cfg.dropPaymentLoopInterval):
			now := time.Now()
			s.paymentsMtx.Lock()
			for tag, p := range s.payments {
				if p.endTime.Before(now) {
					s.log.Tracef("Dropping payment tag %d from redeemed payments table", tag)
					delete(s.payments, tag)
				}
			}
			s.paymentsMtx.Unlock()

			s.rotPaymentsMtx.Lock()
			for tag, endTime := range s.rotPayments {
				if endTime.Before(now) {
					s.log.Tracef("Dropping payment tag %d from redeemed rotation payments", tag)
					delete(s.rotPayments, tag)
				}
			}
			s.rotPaymentsMtx.Unlock()
		}
	}
}

// Run the server.
func (s *Server) Run(ctx context.Context) error {
	g, gctx := errgroup.WithContext(ctx)

	// Run loop that removes payments that are no longer valid.
	g.Go(func() error { return s.dropStalePayments(gctx) })

	// Determine full list of listeners.
	listeners := slices.Clone(s.cfg.listeners)
	for _, addr := range s.cfg.listenAddrs {
		inner, err := net.ListenUDP("udp", addr)
		if err != nil {
			return fmt.Errorf("unable to listen on '%s': %v",
				addr, err)
		}
		listeners = append(listeners, inner)
	}

	// Run the listeners.
	for _, inner := range listeners {
		l, err := listen(inner, s.cfg.ignoreKernelStats)
		if err != nil {
			return err
		}

		s.log.Infof("Listening on %s. Starting %d read routines", l.Addr(), s.cfg.readRoutines)

		g.Go(func() error { return s.timeoutStaleConns(gctx, l) })
		g.Go(func() error { return s.collectListenerStats(gctx, l) })
		g.Go(func() error { return s.listenerHandshakeLoop(gctx, l) })
		for i := 0; i < s.cfg.readRoutines; i++ {
			g.Go(func() error { return s.listenerReadLoop(gctx, l) })
		}

		// Close the socket once the server is done.
		g.Go(func() error {
			<-gctx.Done()
			s.log.Debugf("Group context done. Closing socket %s", l.Addr())
			return l.l.Close()
		})

		s.listenersMtx.Lock()
		s.listeners = append(s.listeners, l)
		s.listenersMtx.Unlock()
	}

	if s.cfg.promAddr != "" {
		g.Go(func() error { return s.runPrometheusListener(gctx, s.cfg.promAddr) })
	}

	// Run loop to report session members.
	g.Go(func() error {
		return s.runSessionListingLoop(gctx, s.cfg.sessListingInterval,
			s.cfg.minSessListingInterval)
	})
	g.Go(func() error { return s.runReportStatsLoop(gctx, s.cfg.statsReportInterval) })

	return g.Wait()
}
