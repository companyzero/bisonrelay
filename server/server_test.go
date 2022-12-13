package server

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/decred/slog"
)

// TestDisconnectsWithoutSession asserts that failing to create a session in a
// timely manner causes the server to disconnect the client.
func TestDisconnectsWithoutSession(t *testing.T) {
	svr := newTestServer(t)
	errChan := runTestServer(t, svr)
	addr := serverBoundAddr(t, svr)
	dialer := clientintf.NetDialer(addr, slog.Disabled)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	conn, _, err := dialer(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// Read loop (it should receive an EOF when the server closes the conn).
	readErr := make(chan error)
	go func() {
		var b [1]byte
		_, err := conn.Read(b[:])
		select {
		case readErr <- err:
		case <-ctx.Done():
		}
	}()

	// Wait until server drops the conn.
	select {
	case err := <-readErr:
		if !errors.Is(err, io.EOF) {
			t.Fatalf("unexpected error: got %v, want %v", err, io.EOF)
		}
	case err := <-errChan:
		t.Fatalf("unexpected run() error: %v", err)
	}
}
