package server

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/companyzero/bisonrelay/ratchet"
	"github.com/companyzero/bisonrelay/rpc"
	"github.com/decred/slog"
)

// TestIgnoresDupeRVPush ensures sending data to the same RV twice doesn't cause
// an error.
func TestIgnoresDupeRVPush(t *testing.T) {
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

	kx := kxServerConn(t, conn)

	// Read loop.
	readErr := make(chan error)
	routeMsgErr := make(chan error)
	go func() {
		for {
			rawMsg, err := kx.Read()
			if err != nil {
				select {
				case readErr <- err:
				case <-ctx.Done():
				}
			}
			if len(rawMsg) == 0 {
				return
			}
			_, msg := decodeServerMsg(t, rawMsg)
			if msg, ok := msg.(*rpc.RouteMessageReply); ok {
				if msg.Error != "" {
					select {
					case routeMsgErr <- errors.New(msg.Error):
					case <-ctx.Done():
					}
				}
			}
		}
	}()

	msg0 := rpc.Message{
		Command: rpc.TaggedCmdRouteMessage,
		Tag:     0,
	}
	msg1 := rpc.Message{
		Command: rpc.TaggedCmdRouteMessage,
		Tag:     1,
	}
	rm := rpc.RouteMessage{
		Rendezvous: ratchet.RVPoint{31: 0xff},
		Message:    []byte{0x01, 0x02, 0x03},
	}

	writeServerMsg(t, kx, msg0, rm)
	writeServerMsg(t, kx, msg1, rm)

	// Ensure client wasn't failed by server.
	select {
	case err := <-readErr:
		t.Fatalf("unexpected read error: %v", err)
	case err := <-errChan:
		t.Fatalf("unexpected run() error: %v", err)
	case err := <-routeMsgErr:
		t.Fatalf("unexpected route message error: %v", err)
	case <-time.After(3 * time.Second):
		// Success.
	}
}
