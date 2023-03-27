package jsonrpc

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"os"
	"sync"
	"time"

	"github.com/companyzero/bisonrelay/clientrpc/types"
	"github.com/decred/slog"
	"github.com/gorilla/websocket"
	"golang.org/x/sync/errgroup"
	"google.golang.org/protobuf/proto"
)

const (
	websocketPongTimeout  = time.Second * 10
	websocketPingInterval = time.Second * 30
	pingPayloadSize       = 16
)

// wsPeer is a websocket-based peer. It supports both the server and client
// ends.
type wsPeer struct {
	p    *peer
	conn *websocket.Conn
	log  slog.Logger

	lastWriter io.WriteCloser
}

func (p *wsPeer) close() error {
	return p.conn.Close()
}

func (p *wsPeer) request(ctx context.Context, method string, req, res proto.Message) error {
	return p.p.request(ctx, method, req, res)
}

func (p *wsPeer) stream(ctx context.Context, method string, params proto.Message) (types.ClientStream, error) {
	return p.p.requestStream(ctx, method, params)
}

func (p *wsPeer) nextDecoder() (*json.Decoder, error) {
	_, r, err := p.conn.NextReader()
	if err != nil {
		return nil, err
	}
	return json.NewDecoder(r), nil
}

func (p *wsPeer) nextEncoder() (*json.Encoder, error) {
	w, err := p.conn.NextWriter(websocket.BinaryMessage)
	if err != nil {
		return nil, err
	}
	p.lastWriter = w
	return json.NewEncoder(w), nil
}

func (p *wsPeer) flushLastWrite() error {
	return p.lastWriter.Close()
}

func (p *wsPeer) run(ctx context.Context) error {
	defer p.close()

	g, gctx := errgroup.WithContext(ctx)

	pongChan := make(chan [pingPayloadSize]byte)

	p.conn.SetPingHandler(func(payload string) error {
		var pingData [pingPayloadSize]byte
		copy(pingData[:], []byte(payload))
		p.log.Tracef("ping payload (len %d): %x", len(payload), pingData)
		var netErr net.Error
		err := p.conn.WriteControl(websocket.PongMessage, pingData[:],
			time.Now().Add(websocketPongTimeout))
		if err != nil && !errors.Is(err, websocket.ErrCloseSent) &&
			!(errors.As(err, &netErr) && netErr.Timeout()) {

			p.log.Errorf("Failed to send pong: %v", err)
			return err
		}
		return nil
	})
	p.conn.SetPongHandler(func(payload string) error {
		var pongData [pingPayloadSize]byte
		copy(pongData[:], []byte(payload))
		p.log.Tracef("pong (original len: %d): %x", len(payload), pongData)
		select {
		case <-gctx.Done():
		case pongChan <- pongData:
		}
		return nil
	})

	// Ping loop.
	g.Go(func() error {
		var pingData [pingPayloadSize]byte
		for {
			select {
			case <-gctx.Done():
				return gctx.Err()
			case <-time.After(websocketPingInterval):
			}
			_, _ = rand.Read(pingData[:])
			err := p.conn.WriteControl(websocket.PingMessage,
				pingData[:], time.Now().Add(time.Second))
			if err != nil {
				return err
			}

			// Wait for pong ack.
			select {
			case <-gctx.Done():
				return gctx.Err()
			case <-time.After(websocketPongTimeout):
				return fmt.Errorf("pong timeout")
			case pongData := <-pongChan:
				if pingData != pongData {
					return fmt.Errorf("ping data != pong data")
				}
			}
		}
	})
	g.Go(func() error { return p.p.run(gctx) })
	return g.Wait()
}

// newServerWSPeer creates a server side websocket JSON-RPC peer. It can run
// any function from the services ServersMap.
func newServerWSPeer(conn *websocket.Conn, services *types.ServersMap, log slog.Logger) *wsPeer {
	p := &wsPeer{
		conn: conn,
		log:  log,
	}
	p.p = newPeer(services, log, p.nextDecoder, p.nextEncoder, p.flushLastWrite)

	return p
}

// dialClientWSPeer returns a client-side websocket peer by connecting to the
// specified URL.
func dialClientWSPeer(ctx context.Context, dialer func(context.Context) (*websocket.Conn, error), log slog.Logger) (*wsPeer, error) {
	conn, err := dialer(ctx)
	if err != nil {
		return nil, err
	}
	p := &wsPeer{
		log:  log,
		conn: conn,
	}
	p.p = newPeer(nil, log, p.nextDecoder, p.nextEncoder, p.flushLastWrite)
	return p, nil
}

// WSClient is a websockets-based bidirectional JSON-RPC 2.0 client. It can
// connect to compatible servers to perform requests.
//
// After it is started by its [Run] method, the client will attempt to maintain
// a connection to the server to be able to perform its requests.
type WSClient struct {
	dialer func(context.Context) (*websocket.Conn, error)
	log    slog.Logger

	mtx         sync.Mutex
	peer        *wsPeer
	waitingPeer []chan *wsPeer
}

func (c *WSClient) Close() error {
	c.mtx.Lock()
	var err error
	if c.peer != nil {
		err = c.peer.close()
	}
	c.mtx.Unlock()
	return err
}

// nextPeer returns the currently running peer or waits until the next peer
// is available on which to execute a request.
func (c *WSClient) nextPeer(ctx context.Context) (*wsPeer, error) {
	c.mtx.Lock()
	peer := c.peer
	if peer != nil {
		c.mtx.Unlock()
		return peer, nil
	}

	ch := make(chan *wsPeer, 1)
	c.waitingPeer = append(c.waitingPeer, ch)
	c.mtx.Unlock()

	select {
	case peer = <-ch:
		return peer, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Request performs a unary request to the server, filling in the passed
// response object.
//
// The request will fail with an error if the connection drops before the
// response is received.
func (c *WSClient) Request(ctx context.Context, method string, req, res proto.Message) error {
	peer, err := c.nextPeer(ctx)
	if err != nil {
		return err
	}

	return peer.request(ctx, method, req, res)
}

// Stream performs a streaming request to the server. It returns a stream from
// which individual responses can be read.
//
// The stream closes when the passed context is closed, when the server sends
// and error (including EOF at the end of the stream) or when the connection to
// the server on which the request was made is broken.
func (c *WSClient) Stream(ctx context.Context, method string, params proto.Message) (types.ClientStream, error) {
	peer, err := c.nextPeer(ctx)
	if err != nil {
		return nil, err
	}
	return peer.stream(ctx, method, params)
}

// Run the client. This needs to be called for requests to be performed.
func (c *WSClient) Run(ctx context.Context) error {
	const minReconnectInterval = time.Second
	const maxReconnectInterval = time.Second * 30
	reconnectInterval := minReconnectInterval

	ctxDone := func() bool {
		select {
		case <-ctx.Done():
			return true
		default:
			return false
		}
	}

	// Maintain connection.
loop:
	for {
		peer, err := dialClientWSPeer(ctx, c.dialer, c.log)
		if err != nil {
			if !errors.Is(err, context.Canceled) {
				c.log.Warnf("Unable to connect to RPC server due to %v. "+
					"Delaying next attempt by %s", err, reconnectInterval)
			}
			// Exponential backoff to attempt reconnect.
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(reconnectInterval):
				reconnectInterval *= 2
				if reconnectInterval > maxReconnectInterval {
					reconnectInterval = maxReconnectInterval
				}
				continue loop
			}
		}
		reconnectInterval = minReconnectInterval

		c.mtx.Lock()
		c.peer = peer
		waiting := c.waitingPeer
		c.waitingPeer = nil
		c.mtx.Unlock()

		go func() {
			for _, w := range waiting {
				w <- peer
			}
		}()

		err = peer.run(ctx)
		if ctxDone() {
			return ctx.Err()
		}
		c.mtx.Lock()
		c.peer = nil
		c.mtx.Unlock()

		if err != nil {
			c.log.Debugf("Peer has finished running: %v", err)
		}
	}
}

type clientConfig struct {
	log            slog.Logger
	url            string
	serverCertPath string
	clientCertPath string
	clientKeyPath  string
}

// makeDialer creates the per-conn dialer, based on the config.
func (cfg *clientConfig) makeDialer() (func(context.Context) (*websocket.Conn, error), error) {
	tlsConfig := &tls.Config{
		MinVersion:       tls.VersionTLS12,
		CurvePreferences: []tls.CurveID{tls.X25519, tls.CurveP256},
		CipherSuites: []uint16{
			tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		},
	}
	if cfg.serverCertPath != "" {
		serverCert, err := os.ReadFile(cfg.serverCertPath)
		if err != nil {
			return nil, fmt.Errorf("unable to load server cert file: %v", err)
		}
		pool := x509.NewCertPool()
		pool.AppendCertsFromPEM(serverCert)
		tlsConfig.RootCAs = pool
	}

	if cfg.clientCertPath != "" {
		cert, err := tls.LoadX509KeyPair(cfg.clientCertPath, cfg.clientKeyPath)
		if err != nil {
			return nil, fmt.Errorf("unable to read client keypair: %v", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	netDialer := net.Dialer{}
	inner := netDialer.DialContext
	wsDialer := websocket.Dialer{
		NetDialContext:  inner,
		TLSClientConfig: tlsConfig,
	}
	dialer := func(ctx context.Context) (*websocket.Conn, error) {
		//nolint:bodyclose
		conn, _, err := wsDialer.DialContext(ctx, cfg.url, nil)
		return conn, err
	}
	return dialer, nil
}

// ClientOption is a configuration option for JSON-RPC clients.
type ClientOption func(cfg *clientConfig)

// WithWebsocketURL defines the URL to use to connect to the clientrpc server.
func WithWebsocketURL(url string) ClientOption {
	return func(cfg *clientConfig) {
		cfg.url = url
	}
}

// WithClientLog defines the logger to use to log client-related debug messages.
func WithClientLog(log slog.Logger) ClientOption {
	return func(cfg *clientConfig) {
		cfg.log = log
	}
}

// WithServerTLSCertPath defines the path to the certificate file to use to
// connect to the server. If this option is not defined, only system
// certificates will be used to verify the server connection.
func WithServerTLSCertPath(certPath string) ClientOption {
	return func(cfg *clientConfig) {
		cfg.serverCertPath = certPath
	}
}

// WithClientTLSCert defines the path to the client certificate and key to use
// to authenticate against the server. If the server requires client
// authentication, then providing this option is necessary.
func WithClientTLSCert(certPath, keyPath string) ClientOption {
	return func(cfg *clientConfig) {
		cfg.clientCertPath = certPath
		cfg.clientKeyPath = keyPath
	}
}

// NewWSClient creates a new Websockets-based JSON-RPC 2.0 client.
func NewWSClient(options ...ClientOption) (*WSClient, error) {
	cfg := &clientConfig{
		log: slog.Disabled,
	}
	for _, opt := range options {
		opt(cfg)
	}

	dialer, err := cfg.makeDialer()
	if err != nil {
		return nil, err
	}

	c := &WSClient{
		log:    cfg.log,
		dialer: dialer,
	}
	return c, nil
}
