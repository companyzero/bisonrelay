package jsonrpc

import (
	"context"
	"errors"
	"io"
	stdlog "log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/companyzero/bisonrelay/clientrpc/types"
	"github.com/decred/slog"
	"github.com/gorilla/websocket"
	"golang.org/x/sync/errgroup"
)

func (s *Server) wrapWithBasicAuth(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Check if provided credentials match the server's credentials
		if s.authMode == "" {
			// If authentication is not required, proceed with the request
			handler(w, r)
		}

		// If Basic Auth is required
		// Check if rpcuser or rpcpass is not set, respond with Unauthorized
		if s.rpcUser == "" || s.rpcPass == "" {
			http.Error(w, "Forbidden", http.StatusUnauthorized)
			return
		}
		// Extract the username and password from the request's Authorization header
		username, password, ok := r.BasicAuth()

		if !ok {
			// If the credentials are missing or malformed, respond with Unauthorized
			w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Check if provided credentials match the server's credentials
		if username != s.rpcUser || password != s.rpcPass {
			// If they don't match, respond with StatusForbidden
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		handler(w, r)
	}
}

// isServerExpectedCloseErr returns true if the error is expected and should
// not be logged.
func isServerExpectedCloseErr(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, io.EOF) {
		return true
	}

	errStr := err.Error()
	return strings.HasSuffix(errStr, "unexpected EOF") || strings.HasSuffix(errStr, "broken pipe")
}

// handlePostRequest is the start of hadling a POST-based JSON-RPC request.
func (s *Server) handlePostRequest(w http.ResponseWriter, req *http.Request) {
	// TODO: hijack connection, manually write the response headers and
	// then stream the responses. This would allow creating a POST-based
	// client in go.
	p := newServerPostPeer(w, req, s.services, s.log)
	err := p.run(req.Context())
	if !isServerExpectedCloseErr(err) {
		s.log.Warnf("POST request error: %v", err)
	}
}

// handleWebsocketRequest is the start of handling a websockets-based JSON-RPC
// request.
func (s *Server) handleWebsocketRequest(ctx context.Context, conn *websocket.Conn) error {
	p := newServerWSPeer(conn, s.services, s.log)
	return p.run(ctx)
}

// Server is a JSON-RPC 2.0 server. It supports both POST-based and
// websockets-based requests.
type Server struct {
	services  *types.ServersMap
	listeners []net.Listener
	log       slog.Logger
	rpcUser   string
	rpcPass   string
	authMode  string
}

// Run the server, responding to requests until the context is closed.
func (s *Server) Run(ctx context.Context) error {
	g, gctx := errgroup.WithContext(ctx)

	serveMux := &http.ServeMux{}

	// Handler for POST JSON-RPC requests.
	serveMux.HandleFunc("/", s.wrapWithBasicAuth(s.handlePostRequest))

	// Handler for WebSocket JSON-RPC requests.
	wsHandler := func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				// Allow requests with no origin header set.
				origin := r.Header["Origin"]
				if len(origin) == 0 {
					return true
				}

				// Reject requests with origin headers that are not valid URLs.
				originURL, err := url.Parse(origin[0])
				if err != nil {
					return false
				}

				// Allow local resources on browsers that set the origin header
				// for them.  In particular:
				// - Firefox which sets it to "null"
				// - Chrome which sets it to "file://"
				// - Edge which sets it to "file://"
				if originURL.Scheme == "file" || originURL.Path == "null" {
					return true
				}

				// Strip the port from both the origin and request hosts.
				originHost := originURL.Host
				requestHost := r.Host
				if host, _, err := net.SplitHostPort(originHost); err != nil {
					originHost = host
				}
				if host, _, err := net.SplitHostPort(requestHost); err != nil {
					requestHost = host
				}

				// Reject mismatched hosts.
				return equalASCIIFold(originHost, requestHost)
			},
		}
		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			var herr websocket.HandshakeError
			if !errors.As(err, &herr) {
				s.log.Errorf("Unexpected websocket error: %v", err)
			}
			return
		}

		err = s.handleWebsocketRequest(r.Context(), ws)
		if err != nil {
			if !isServerExpectedCloseErr(err) {
				// Unexpected errors.
				s.log.Errorf("Error while handling websocket "+
					"request from %s: %v", r.RemoteAddr,
					err)
			} else {
				// Graceful disconnection errors.
				s.log.Tracef("Error while handling websocket "+
					"request from %s: %v", r.RemoteAddr,
					err)
			}
		}
	}
	serveMux.HandleFunc("/ws", s.wrapWithBasicAuth(wsHandler))

	httpServer := &http.Server{
		Handler:     serveMux,
		BaseContext: func(_ net.Listener) context.Context { return gctx },
		ErrorLog:    stdlog.New(slogWriter{f: s.log.Warn}, "", 0),
	}

	// Listen on network interfaces.
	for _, l := range s.listeners {
		l := l
		g.Go(func() error {
			s.log.Infof("Listening for clientrpc JSON-RPC requests on %s", l.Addr())
			return httpServer.Serve(l)
		})
	}

	// Wait to shutdown listeners.
	g.Go(func() error {
		<-gctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		return httpServer.Shutdown(shutdownCtx)
	})

	return g.Wait()
}

type serverConfig struct {
	services  *types.ServersMap
	listeners []net.Listener
	log       slog.Logger
	authMode  string
	rpcUser   string
	rpcPass   string
}

// ServerOption defines an option when configuring a JSON-RPC server.
type ServerOption func(*serverConfig)

// WithServices defines the service map to use on the server. Services may be
// added or removed from this as needed.
func WithServices(s *types.ServersMap) ServerOption {
	return func(cfg *serverConfig) {
		cfg.services = s
	}
}

// WithListeners defines which listeners to bind the server to. The listeners
// must have been configured with TLS, client-side authentication or any other
// needed configuration.
func WithListeners(listeners []net.Listener) ServerOption {
	return func(cfg *serverConfig) {
		cfg.listeners = listeners
	}
}

// WithServerLog defines the logger to use to log server debug messages.
func WithServerLog(log slog.Logger) ServerOption {
	return func(cfg *serverConfig) {
		cfg.log = log
	}
}

func WithAuth(username, password, authMode string) ServerOption {
	return func(cfg *serverConfig) {
		cfg.rpcUser = username
		cfg.rpcPass = password
		cfg.authMode = authMode
	}
}

// NewServer returns a new JSON-RPC server.
//
// This is usually only used inside Bison Relay clients.
func NewServer(options ...ServerOption) *Server {
	cfg := &serverConfig{
		log: slog.Disabled,
	}
	for _, opt := range options {
		opt(cfg)
	}
	return &Server{
		services:  cfg.services,
		listeners: cfg.listeners,
		log:       cfg.log,
		authMode:  cfg.authMode,
		rpcUser:   cfg.rpcUser,
		rpcPass:   cfg.rpcPass,
	}
}
