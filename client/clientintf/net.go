package clientintf

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"

	"github.com/decred/slog"
)

type DialFunc func(context.Context, string, string) (net.Conn, error)

// tlsDialer creates the inner TLS client dialer, based on the outer network
// dialer.
func tlsDialer(addr string, log slog.Logger, dialFunc DialFunc) func(context.Context) (Conn, *tls.ConnectionState, error) {
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
		},
		InsecureSkipVerify: true,
	}

	return func(ctx context.Context) (Conn, *tls.ConnectionState, error) {
		nconn, err := dialFunc(ctx, "tcp", addr)
		if err != nil {
			return nil, nil, err
		}

		conn := tls.Client(nconn, tlsConfig)

		// Force handshake to collect the completed connection state.
		if err := conn.Handshake(); err != nil {
			return nil, nil, err
		}

		cs := conn.ConnectionState()
		if len(cs.PeerCertificates) != 1 {
			conn.Close()
			return nil, nil, fmt.Errorf("unexpected certificate chain")
		}

		log.Infof("Connected to server %s", addr)
		return conn, &cs, nil
	}

}

// NetDialer returns a client dialer function that always connects to a
// specific server address.
func NetDialer(addr string, log slog.Logger) func(context.Context) (Conn, *tls.ConnectionState, error) {
	netDialer := &net.Dialer{}
	return tlsDialer(addr, log, netDialer.DialContext)
}

// WithDialer returns a client dialer function that uses the given dialer.
func WithDialer(addr string, log slog.Logger, dialFunc DialFunc) func(context.Context) (Conn, *tls.ConnectionState, error) {
	return tlsDialer(addr, log, dialFunc)
}
