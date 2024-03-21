package server

import (
	"compress/zlib"
	"context"
	"crypto/rand"
	"io"
	"testing"
	"time"

	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/companyzero/bisonrelay/internal/assert"
	"github.com/companyzero/bisonrelay/ratchet"
	"github.com/companyzero/bisonrelay/rpc"
	"github.com/companyzero/bisonrelay/zkidentity"
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

func dummySigner(_ []byte) zkidentity.FixedSizeSignature {
	var res zkidentity.FixedSizeSignature
	_, _ = io.ReadFull(rand.Reader, res[:])
	return res
}

// TestServerRecvMaxMsgSize tests how the server handles messages around its
// max message size.
func TestServerRecvMaxMsgSize(t *testing.T) {
	tests := []struct {
		name    string
		version rpc.MaxMsgSizeVersion
	}{{
		name:    "v0",
		version: rpc.MaxMsgSizeV0,
	}, {
		name:    "v1",
		version: rpc.MaxMsgSizeV1,
	}}

	for i := range tests {
		tc := tests[i]
		t.Run(tc.name, func(t *testing.T) {
			svr := newTestServer(t)
			svr.settings.MaxMsgSizeVersion = tc.version

			errChan := runTestServer(t, svr)
			addr := serverBoundAddr(t, svr)
			dialer := clientintf.NetDialer(addr, slog.Disabled)
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			conn, _, err := dialer(ctx)
			assert.NilErr(t, err)

			kx := kxServerConn(t, conn)

			// Message that has max possible payload.
			maxPayload := rpc.MaxPayloadSizeForVersion(tc.version)
			data := make([]byte, maxPayload)
			n, err := rand.Read(data[:])
			assert.NilErr(t, err)
			if n != len(data) {
				t.Fatal("too few bytes read")
			}
			rm := rpc.RMFTGetChunkReply{
				FileID: zkidentity.ShortID{}.String(),
				Index:  1<<32 - 1,
				Chunk:  data,
				Tag:    1<<32 - 1,
			}
			compressed, err := rpc.ComposeCompressedRM(dummySigner, rm, zlib.NoCompression)
			assert.NilErr(t, err)

			// This message should be sent without issues.
			msg1 := rpc.Message{
				Command: rpc.TaggedCmdRouteMessage,
				Tag:     1,
			}
			msg1Payload := rpc.RouteMessage{
				Rendezvous: ratchet.RVPoint{31: 0xff},
				Message:    compressed,
			}
			writeServerMsg(t, kx, msg1, msg1Payload)

			// Prepare a message larger than the max allowed.
			largeMsg := append(compressed, compressed[:]...)
			msg2 := rpc.Message{
				Command: rpc.TaggedCmdRouteMessage,
				Tag:     1,
			}
			msg2Payload := rpc.RouteMessage{
				Rendezvous: ratchet.RVPoint{31: 0xff},
				Message:    largeMsg,
			}

			// The server might error while the test is writing the
			// message or it may fail after the message has been
			// completely written (due to buffering), so attempt
			// writing twice to verify at least one of the times it
			// fails.
			for i := 0; i < 2; i++ {
				err := writeServerMsgMaybeErr(t, kx, msg2, msg2Payload)
				if err != nil {
					break
				}
				if i == 1 {
					t.Fatal("Second write did not fail.")
				}
				msg2.Tag += 1
				time.Sleep(10 * time.Millisecond)
			}
			assert.ChanNotWritten(t, errChan, time.Second)
		})
	}
}

// TestPushesToLastSub verifies the server pushes an RM to the session that
// subscribed last to it (and only to it).
func TestPushesToLastSub(t *testing.T) {
	tests := []struct {
		name       string
		disconnect bool
	}{{
		name:       "does not disconnect before push",
		disconnect: false,
	}, {
		name:       "disconnects before push",
		disconnect: true,
	}}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			svr := newTestServer(t)
			runTestServer(t, svr)
			addr := serverBoundAddr(t, svr)
			dialer := clientintf.NetDialer(addr, slog.Disabled)
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()

			// Open three sessions to the server (two to subscribe, one to push).
			conn1, _, err := dialer(ctx)
			assert.NilErr(t, err)
			kx1 := kxServerConn(t, conn1)
			conn2, _, err := dialer(ctx)
			assert.NilErr(t, err)
			kx2 := kxServerConn(t, conn2)
			conn3, _, err := dialer(ctx)
			assert.NilErr(t, err)
			kx3 := kxServerConn(t, conn3)

			// Sub message.
			rv := ratchet.RVPoint{1: 0xff}
			msgSub := rpc.Message{
				Command: rpc.TaggedCmdSubscribeRoutedMessages,
				Tag:     1,
			}
			sub := rpc.SubscribeRoutedMessages{
				AddRendezvous: []ratchet.RVPoint{rv},
			}

			// Subscribe in the first session.
			writeServerMsg(t, kx1, msgSub, sub)
			readNextServerMsg(t, kx1) // reply

			// Subscribe in the second session.
			writeServerMsg(t, kx2, msgSub, sub)
			readNextServerMsg(t, kx2) // reply

			// When the test case requires it, disconnect from
			// the first connection.
			if tc.disconnect {
				assert.NilErr(t, conn1.Close())
			}

			// Push in the third session.
			msgRM := rpc.Message{
				Command: rpc.TaggedCmdRouteMessage,
				Tag:     1,
			}
			rm := rpc.RouteMessage{
				Rendezvous: rv,
				Message:    []byte{0x01, 0x02, 0x03},
			}
			writeServerMsg(t, kx3, msgRM, rm)
			readNextServerMsg(t, kx3) // reply

			// The RM should be pushed in the second session.
			_, gotPayload := readNextServerMsg(t, kx2)
			pushedRM, ok := gotPayload.(*rpc.PushRoutedMessage)
			assert.DeepEqual(t, ok, true)
			assert.DeepEqual(t, pushedRM.RV, rv)
			assert.DeepEqual(t, pushedRM.Payload, rm.Message)

			// It should not be pushed to the first session.
			if !tc.disconnect {
				kx1Msgs := drainServerMsgs(t, kx1)
				assert.ChanNotWritten(t, kx1Msgs, time.Second)
			}
		})
	}
}

// TestReceivesStoredRM verifies the server pushes an RM to the session when
// that RM was already stored.
func TestReceivesStoredRM(t *testing.T) {
	svr := newTestServer(t)
	runTestServer(t, svr)
	addr := serverBoundAddr(t, svr)
	dialer := clientintf.NetDialer(addr, slog.Disabled)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	rv := ratchet.RVPoint{1: 0xee}

	// Open the conn and push the RM.
	conn1, _, err := dialer(ctx)
	assert.NilErr(t, err)
	kx1 := kxServerConn(t, conn1)
	msgRM := rpc.Message{
		Command: rpc.TaggedCmdRouteMessage,
		Tag:     1,
	}
	rm := rpc.RouteMessage{
		Rendezvous: rv,
		Message:    []byte{0x01, 0x02, 0x03},
	}
	writeServerMsg(t, kx1, msgRM, rm)
	readNextServerMsg(t, kx1) // reply

	// Open the second conn and subscribe.
	conn2, _, err := dialer(ctx)
	assert.NilErr(t, err)
	kx2 := kxServerConn(t, conn2)
	msgSub := rpc.Message{
		Command: rpc.TaggedCmdSubscribeRoutedMessages,
		Tag:     1,
	}
	sub := rpc.SubscribeRoutedMessages{
		AddRendezvous: []ratchet.RVPoint{rv},
	}

	writeServerMsg(t, kx2, msgSub, sub)
	readNextServerMsg(t, kx2) // reply

	// The RM should be pushed in the second session.
	_, gotPayload := readNextServerMsg(t, kx2)
	pushedRM, ok := gotPayload.(*rpc.PushRoutedMessage)
	assert.DeepEqual(t, ok, true)
	assert.DeepEqual(t, pushedRM.RV, rv)
	assert.DeepEqual(t, pushedRM.Payload, rm.Message)
}
