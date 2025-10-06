package seederserver

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/companyzero/bisonrelay/rpc"
	"github.com/decred/slog"
	"github.com/gorilla/websocket"
	"golang.org/x/sync/errgroup"
)

// Server is a runnable BR seeder server.
type Server struct {
	cfg         config
	log         slog.Logger
	upgrader    websocket.Upgrader
	timeStarted time.Time
	mux         *http.ServeMux

	// mtx protects the following fields.
	mtx sync.Mutex

	// serverMaster is the current master brserver instance.
	serverMaster smi

	// serverMap tracks the available brserver instances. It's a map from
	// token to last brserver status report.
	serverMap map[string]rpc.SeederCommandStatus
}

// New creates a new seeder server.
func New(opts ...Option) (*Server, error) {
	cfg := config{
		log:             slog.Disabled,
		tokens:          map[string]struct{}{},
		waitForMaster:   5 * time.Minute,
		appName:         "brseeder",
		httpTimeout:     20 * time.Second,
		shutdownTimeout: time.Second,
		offlineLimit:    time.Minute,
	}
	for _, o := range opts {
		o(&cfg)
	}

	s := &Server{
		cfg:         cfg,
		log:         cfg.log,
		timeStarted: time.Now(),
		upgrader: websocket.Upgrader{
			HandshakeTimeout: 20 * time.Second,
			ReadBufferSize:   1024,
			WriteBufferSize:  1024,
		},
		mux:       http.NewServeMux(),
		serverMap: make(map[string]rpc.SeederCommandStatus),
	}

	// serve api for brservers
	s.mux.HandleFunc("/api/v1/status", s.handleBRServerStatus)

	// serve api for brclients
	s.mux.HandleFunc("/api/v1/live", s.handleClientStatusQuery)

	return s, nil
}

// Run runs the seeder server. Listeners are closed once this server is
// commanded to stop (if ctx is canceled) or when any error occurs.
func (s *Server) Run(ctx context.Context) error {
	srv := &http.Server{
		Handler:      s.mux,
		ReadTimeout:  s.cfg.httpTimeout, // slow requests should not hold connections opened
		WriteTimeout: s.cfg.httpTimeout, // request to response time
	}

	g, ctx := errgroup.WithContext(ctx)
	for i := range s.cfg.listeners {
		lis := s.cfg.listeners[i]
		g.Go(func() error {
			s.log.Infof("Serving seeder API on %s", lis.Addr())
			err := srv.Serve(lis)
			if err != nil && !errors.Is(err, http.ErrServerClosed) {
				s.log.Errorf("unexpected (http.Server).Serve error on interface %s: %v",
					lis.Addr(), err)
			}
			return err
		})
	}

	g.Go(func() error {
		<-ctx.Done()

		// Complete graceful shutdown or force exit after
		// shutdownTimeout.
		shutCtx, cancel := context.WithTimeout(context.Background(), s.cfg.shutdownTimeout)
		defer cancel()
		err := srv.Shutdown(shutCtx)
		if err != nil {
			s.log.Errorf("Ungraceful shutdown: %v", err)
		}
		return err
	})

	return g.Wait()
}
