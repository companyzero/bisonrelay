package clientintf

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/decred/slog"
)

type DialFunc func(context.Context, string, string) (net.Conn, error)

type ServerGroup struct {
	Server   string `json:"brserver"`
	LND      string `json:"lnd"`
	IsMaster bool   `json:"isMaster"`
}

type ClientAPI struct {
	ServerGroups []ServerGroup `json:"serverGroups"`
}

func querySeeder(ctx context.Context, apiURL string, dialFunc DialFunc) (string, error) {
	httpClient := http.Client{
		Transport: &http.Transport{
			DialContext: dialFunc,
		},
		Timeout: time.Minute,
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to make a seeder request: %w", err)
	}
	rep, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to query seeder: %w", err)
	}
	defer rep.Body.Close()

	if rep.StatusCode != 200 {
		return "", fmt.Errorf("seeder returned %v", rep.Status)
	}
	body, err := io.ReadAll(rep.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read seeder response: %w", err)
	}
	var api ClientAPI
	if err = json.Unmarshal(body, &api); err != nil {
		return "", fmt.Errorf("failed to unmarshal seeder response: %w", err)
	}
	var server string
	for i := range api.ServerGroups {
		if api.ServerGroups[i].IsMaster {
			server = api.ServerGroups[i].Server
			break
		}
	}
	if server == "" {
		return "", fmt.Errorf("seeder returned no master servers")
	}
	return server, nil
}

// tlsDialer creates the inner TLS client dialer, based on the outer network
// dialer.
func tlsDialer(addr string, log slog.Logger, dialFunc DialFunc, useSeeder bool) func(context.Context) (Conn, *tls.ConnectionState, error) {
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
		},
		InsecureSkipVerify: true,
	}

	return func(ctx context.Context) (Conn, *tls.ConnectionState, error) {
		var serverAddr string
		if useSeeder {
			apiURL := fmt.Sprintf("https://%s/api/live", addr)
			log.Infof("Querying seeder at %v", apiURL)
			server, err := querySeeder(ctx, apiURL, dialFunc)
			if err != nil {
				return nil, nil, err
			}
			serverAddr = server
		} else {
			serverAddr = addr
		}

		nconn, err := dialFunc(ctx, "tcp", serverAddr)
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
	return tlsDialer(addr, log, netDialer.DialContext, false)
}

// WithDialer returns a client dialer function that uses the given dialer.
func WithDialer(addr string, log slog.Logger, dialFunc DialFunc) func(context.Context) (Conn, *tls.ConnectionState, error) {
	return tlsDialer(addr, log, dialFunc, false)
}

// WithSeeder returns a client dialer function that queries a seeder
// for the server address using the given dialer.
func WithSeeder(addr string, log slog.Logger, dialFunc DialFunc) func(context.Context) (Conn, *tls.ConnectionState, error) {
	return tlsDialer(addr, log, dialFunc, true)
}
