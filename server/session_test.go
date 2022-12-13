package server

import (
	"context"
	"testing"
	"time"

	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/companyzero/bisonrelay/ratchet"
	"github.com/companyzero/bisonrelay/rpc"
	"github.com/decred/slog"
)

// TestServerPingPong asserts the ping/pong semantics of the server.
func TestServerPingPong(t *testing.T) {
	pingLimit := time.Millisecond * 50
	svr := newTestServer(t)
	svr.pingLimit = pingLimit
	svr.logPings = true
	errChan := runTestServer(t, svr)
	addr := serverBoundAddr(t, svr)
	dialer := clientintf.NetDialer(addr, slog.Disabled)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	conn, _, err := dialer(ctx)
	if err != nil {
		t.Fatal(err)
	}

	kx := kxServerConn(t, conn)

	// Read loop. Send errors to readErr and pongs received to gotPong.
	readErr := make(chan error, 2)
	gotPong := make(chan struct{}, 2)
	go func() {
		for {
			rawMsg, err := kx.Read()
			if err != nil {
				readErr <- err
				return
			}
			_, msg := decodeServerMsg(t, rawMsg)
			if _, ok := msg.(*rpc.Pong); ok {
				gotPong <- struct{}{}
			}
		}
	}()

	// Helper to read pongs.
	assertGotPong := func() {
		t.Helper()
		select {
		case err := <-readErr:
			t.Fatal(err)
		case <-gotPong:
		case <-time.After(5 * svr.pingLimit):
			t.Fatal("timeout")
		}
	}

	// Ping message.
	pingMsg := rpc.Message{
		Command: rpc.TaggedCmdPing,
		Tag:     0,
	}
	pingPayload := rpc.Ping{}

	// Dummy message.
	msg1 := rpc.Message{
		Command: rpc.TaggedCmdRouteMessage,
		Tag:     1,
	}
	rm := rpc.RouteMessage{
		Rendezvous: ratchet.RVPoint{31: 0xff},
		Message:    []byte{0x01, 0x02, 0x03},
	}

	// Send a ping. Expect a pong.
	writeServerMsg(t, kx, pingMsg, pingPayload)
	assertGotPong()

	// Wait 3/4 of the way to the ping limit, send a dummy msg.
	time.Sleep(pingLimit * 3 / 4)
	writeServerMsg(t, kx, msg1, rm)

	// Wait 3/4 of the way to the ping limit again. Expect not to be
	// disconnected yet.
	time.Sleep(pingLimit * 3 / 4)
	select {
	case err := <-readErr:
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
	default:
	}

	// Send second ping.
	writeServerMsg(t, kx, pingMsg, pingPayload)
	assertGotPong()
	lastPongTime := time.Now()

	// Wait until we're disconnected from the server due to lack of
	// messages.
	select {
	case err := <-readErr:
		if err == nil {
			t.Fatal("Unexpected nil error")
		}
	case <-time.After(5 * pingLimit):
		t.Fatal("timeout")
	}

	// The time until the disconnection should be greater than the pingLimit
	// interval.
	gotPingInterval := time.Since(lastPongTime)
	if gotPingInterval < pingLimit {
		t.Fatalf("Unexpected interval between ping and disconnection: "+
			"got %s, want %s", gotPingInterval, pingLimit)
	}

	// Server shouldn't have errored.
	select {
	case err := <-errChan:
		if err != nil {
			t.Fatalf("Unexpected run error: %v", err)
		}
	case <-time.After(pingLimit):
	}
}
