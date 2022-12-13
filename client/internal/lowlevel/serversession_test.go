package lowlevel

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/companyzero/bisonrelay/internal/assert"
	"github.com/companyzero/bisonrelay/rpc"
	"github.com/davecgh/go-spew/spew"
)

// TestServerSessionClose tests the close() method for the server session works
// as expected.
func TestServerSessionClose(t *testing.T) {
	tests := []struct {
		name string
		sess *serverSession
	}{
		{name: "nil session", sess: nil},
		{name: "nil conn", sess: &serverSession{}},
		{name: "filled conn", sess: &serverSession{conn: offlineConn{}}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.sess.close() // Ensure it doesn't panic.
		})
	}
}

// TestSendPRPCSuccess tests the full successful flow for a SendPRPC call.
func TestSendPRPCSuccess(t *testing.T) {
	t.Parallel()

	replyChan := make(chan interface{}, 100)
	testMsg := rpc.Message{
		Command: rpc.TaggedCmdRouteMessage,
	}
	testPayload := &rpc.RouteMessage{
		Rendezvous: rvidFromStr("XXXXX"),
		Message:    []byte("YYYY"),
	}
	ctx, cancel := context.WithCancel(context.Background())

	kx := newMockKX()
	ss := newServerSession(offlineConn{}, kx, 10, nil)

	runErr := make(chan error)
	go func() { runErr <- ss.Run(ctx) }()
	time.Sleep(10 * time.Millisecond)

	// Send a message that will be successfully relayed.
	errChan := make(chan error)
	go func() { errChan <- ss.SendPRPC(testMsg, testPayload, replyChan) }()

	// Ensure the original call didn't return yet (due to missing write).
	select {
	case err := <-errChan:
		t.Fatalf("unexpected response %v", err)
	case <-time.After(10 * time.Millisecond):
	}

	// Get the msg on the server side.
	gotMsg, gotPayload := kx.popWrittenMsg(t)
	testMsg.TimeStamp = gotMsg.TimeStamp
	testMsg.Tag = gotMsg.Tag
	if !reflect.DeepEqual(gotMsg, testMsg) {
		t.Fatalf("unexpected msg: got %v, want %v", gotMsg, testMsg)
	}
	if !reflect.DeepEqual(gotPayload, testPayload) {
		t.Fatalf("unexpected payload: got %v, want %v", gotPayload, testPayload)
	}

	// Ensure SendPRPC() call completed.
	select {
	case err := <-errChan:
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}

	// Send a corresponding reply.
	replyMsg := rpc.Message{Command: rpc.TaggedCmdRouteMessageReply, Tag: gotMsg.Tag}
	replyPayload := &rpc.RouteMessageReply{Error: "booo"}
	kx.pushReadMsg(t, &replyMsg, replyPayload)

	// Expect replyChan to have been called with the reply payload.
	select {
	case gotPayload := <-replyChan:
		if !reflect.DeepEqual(gotPayload, replyPayload) {
			t.Fatalf("unexpected reply payload: got %s, want %s",
				spew.Sdump(gotPayload), replyPayload)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}

	// Check cancelling run works.
	cancel()
	select {
	case err := <-runErr:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("unexpected error: got %v, want %v", err, context.Canceled)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}
}

// TestWriteErrShutsDownSession tests that a writing error during a send
// shutsdown the session.
func TestWriteErrShutsDownSession(t *testing.T) {
	t.Parallel()

	testMsg := rpc.Message{Command: rpc.TaggedCmdRouteMessage}
	testPayload := &rpc.RouteMessage{}
	replyChan := make(chan interface{}, 100)

	kx := newMockKX()
	ss := newServerSession(offlineConn{}, kx, 10, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runErr := make(chan error)
	go func() { runErr <- ss.Run(ctx) }()
	time.Sleep(10 * time.Millisecond)

	// Send a message that will cause a write error.
	errChan := make(chan error)
	go func() { errChan <- ss.SendPRPC(testMsg, testPayload, replyChan) }()

	// Cause the wire error.
	errTest := errors.New("test error")
	kx.popWriteErr(t, errTest)

	// Ensure the original call returned with an error.
	select {
	case err := <-errChan:
		if !errors.Is(err, errTest) {
			t.Fatalf("unexpected error: got %v, want %v", err, errTest)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}

	// Check run terminated.
	select {
	case err := <-runErr:
		if !errors.Is(err, errTest) {
			t.Fatalf("unexpected error: got %v, want %v", err, errTest)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}
}

// TestSessionTagsInvariants tests the behavior of the tag stack management
// inside the run loop. In particular, it tests tags limit the number of
// un-acked msgs being sent and that stopping the send loop before tags are
// freed returns the correct errors.
func TestSessionTagsInvariants(t *testing.T) {
	t.Parallel()

	testMsg := rpc.Message{Command: rpc.TaggedCmdRouteMessage}
	testPayload := &rpc.RouteMessage{}
	replyChan := make(chan interface{}, 100)
	tagDepth := 10

	kx := newMockKX()
	ss := newServerSession(offlineConn{}, kx, int64(tagDepth), nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runErr := make(chan error)
	go func() { runErr <- ss.Run(ctx) }()
	time.Sleep(10 * time.Millisecond)

	// Start sending one more then the tag depth.
	errChan := make(chan error)
	for i := 0; i < tagDepth+1; i++ {
		go func() { errChan <- ss.SendPRPC(testMsg, testPayload, replyChan) }()
	}

	// Complete writing `tagDepth` messages to the wire.
	var gotTags int
	var lastMsg rpc.Message
	for i := 0; i < tagDepth; i++ {
		msg, _ := kx.popWrittenMsg(t)
		gotTags |= 1 << msg.Tag
		if msg.Tag == uint32(tagDepth-1) {
			lastMsg = msg
		}

		// Ensure SendPRPC() call completed.
		select {
		case err := <-errChan:
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		case <-time.After(time.Second):
			t.Fatal("timeout")
		}
	}

	wantTags := (1 << tagDepth) - 1
	if gotTags != wantTags {
		t.Fatalf("did not get all tags. got %b, want %b", gotTags, wantTags)
	}

	// We should still be waiting for the last send.
	select {
	case err := <-errChan:
		t.Fatalf("unexpected error: %v", err)
	case <-time.After(time.Millisecond * 50):
	}

	// Send the reply that corresponds to a tag.
	replyMsg := rpc.Message{Command: rpc.TaggedCmdRouteMessageReply, Tag: lastMsg.Tag}
	replyPayload := &rpc.RouteMessageReply{}
	kx.pushReadMsg(t, &replyMsg, replyPayload)

	// We can now complete the new write. The tag should be the same as the
	// one we just freed.
	msg, _ := kx.popWrittenMsg(t)
	if msg.Tag != replyMsg.Tag {
		t.Fatalf("unexpected free'd tag. got %d, want %d",
			msg.Tag, replyMsg.Tag)
	}

	// Ensure SendPRPC() call completed.
	select {
	case err := <-errChan:
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}

	// Start a new send that will block on tags again. We'll test
	// cancelling the session while this call is in flight.
	go func() { errChan <- ss.SendPRPC(testMsg, testPayload, replyChan) }()

	// We should still be waiting for one send.
	select {
	case err := <-errChan:
		t.Fatalf("unexpected error: %v", err)
	case <-time.After(time.Millisecond * 50):
	}

	// Cancel the session with a filled tagstack. The last send will be
	// result in an error.
	cancel()

	select { // caller gets result of SendPRPC call
	case err := <-errChan:
		if !errors.Is(err, errSendLoopExiting) {
			t.Fatalf("unexpected error. got %v, want %v", err,
				context.Canceled)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}

	// Expect the sendloop to have exited.
	select {
	case err := <-runErr:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("unexpected error: got %v, want %v", err,
				context.Canceled)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}
}

// TestReadAndDecodeMsg tests that the readAndDecodeMsg function works as
// intended.
func TestReadAndDecodeMsg(t *testing.T) {
	var errTest = errors.New("test error")
	tests := []struct {
		name        string
		cmd         string
		err         error
		hexData     string
		wantPayload interface{}
		wantErr     error
	}{{
		name:    "read error",
		err:     errTest,
		wantErr: errTest,
	}, {
		name:    "unmarshal error",
		hexData: "ff",
		wantErr: unmarshalError{},
	}, {
		name:    "decode error",
		hexData: jsonAsHex(rpc.Message{Command: "**boo"}),
		wantErr: errUnknownRPCCommand,
	}, {
		name:    "payload unmarshal error",
		hexData: jsonAsHex(rpc.Message{Command: rpc.TaggedCmdAcknowledge}),
		wantErr: unmarshalError{},
	}, {
		name: "payload decode success",
		hexData: jsonAsHex(rpc.Message{Command: rpc.TaggedCmdAcknowledge}) +
			jsonAsHex(rpc.Acknowledge{Error: "boo"}),
		wantPayload: &rpc.Acknowledge{Error: "boo"},
	}}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			kx := newMockKX()
			ss := newServerSession(offlineConn{}, kx, 0, nil)
			go func() {
				if tc.err != nil {
					kx.readErrChan <- tc.err
				} else {
					kx.readMsgChan <- mustFromHex(tc.hexData)
				}
			}()
			msg := rpc.Message{Command: tc.cmd}
			gotPayload, gotErr := ss.readAndDecodeMessage(&msg)
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("unexpected error: got %v, want %v",
					gotErr, tc.wantErr)
			}

			if !reflect.DeepEqual(gotPayload, tc.wantPayload) {
				t.Fatalf("unexpected payload: got %s, want %s",
					spew.Sdump(gotPayload), spew.Sdump(tc.wantPayload))
			}
		})
	}
}

// TestRecvLoopNoTag tests that receiving a message for which a tag was
// expected to be previously registered but which wasn't, causes an error.
func TestRecvLoopNoTag(t *testing.T) {
	t.Parallel()

	tests := []string{
		rpc.TaggedCmdAcknowledge,
		rpc.TaggedCmdRouteMessageReply,
		rpc.TaggedCmdSubscribeRoutedMessagesReply,
		rpc.TaggedCmdGetInvoiceReply,
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc, func(t *testing.T) {
			t.Parallel()

			kx := newMockKX()
			ss := newServerSession(offlineConn{}, kx, 10, nil)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			runErr := make(chan error)
			go func() { runErr <- ss.Run(ctx) }()
			time.Sleep(10 * time.Millisecond)

			// Send a payload with an invalid tag.
			msg := &rpc.Message{Command: tc, Tag: 999}
			testPayload := mustPayloadForCmd(tc)
			kx.pushReadMsg(t, msg, testPayload)

			// Expect the loop to have errored.
			select {
			case err := <-runErr:
				if !errors.Is(err, invalidRecvTagError{}) {
					t.Fatalf("unexpected error: got %v, want %v",
						err, invalidRecvTagError{})
				}
			case <-time.After(time.Second):
				t.Fatalf("timeout")
			}
		})
	}
}

// TestRecvLoopReadError tests that a read error causes the recvLoop to break
// with an error.
func TestRecvLoopReadError(t *testing.T) {
	t.Parallel()

	kx := newMockKX()
	ss := newServerSession(offlineConn{}, kx, 10, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runErr := make(chan error)
	go func() { runErr <- ss.Run(ctx) }()
	time.Sleep(10 * time.Millisecond)

	// Send a read error.
	errTest := errors.New("test error")
	kx.pushReadErr(t, errTest)

	// Ensure the loop finished with the error.
	select {
	case err := <-runErr:
		if !errors.Is(err, errTest) {
			t.Fatalf("unexpected error: got %v, want %v", err, errTest)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}
}

// TestRecvLoopPushedMsgs tests that pushed messages are routed to the session
// handler and that the correct ack is sent as needed.
func TestRecvLoopPushedMsgs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		res        error
		wantAck    *rpc.Acknowledge
		wantRunErr bool
	}{{
		name:       "pushed msg processed ok",
		res:        nil,
		wantAck:    &rpc.Acknowledge{},
		wantRunErr: false,
	}, {
		name: "handler returns generic error",
		res:  errors.New("test error"),
		wantAck: &rpc.Acknowledge{
			Error: "test error",
		},
		wantRunErr: true,
	}, {
		name: "handler returns fatal ack error",
		res:  AckError{"some error", 999, false},
		wantAck: &rpc.Acknowledge{
			Error:     "some error",
			ErrorCode: 999,
		},
		wantRunErr: true,
	}, {
		name: "handler returns non-fatal ack error",
		res:  AckError{"some error", 999, true},
		wantAck: &rpc.Acknowledge{
			Error:     "some error",
			ErrorCode: 999,
		},
		wantRunErr: false,
	}}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {

			pushedChan := make(chan *rpc.PushRoutedMessage)
			pushHandler := func(msg *rpc.PushRoutedMessage) error {
				pushedChan <- msg
				return tc.res
			}

			kx := newMockKX()
			ss := newServerSession(offlineConn{}, kx, 10, nil)
			ss.pushedRoutedMsgsHandler = pushHandler

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			runErr := make(chan error)
			go func() { runErr <- ss.Run(ctx) }()
			time.Sleep(10 * time.Millisecond)

			// Send a pushed message.
			wantTag := uint32(999)
			msg := &rpc.Message{Command: rpc.TaggedCmdPushRoutedMessage, Tag: wantTag}
			payload := &rpc.PushRoutedMessage{
				Payload: []byte("boo payload"),
				Error:   "boo",
			}
			kx.pushReadMsg(t, msg, payload)

			// Wait to get the pushed msg on the handler.
			select {
			case msg := <-pushedChan:
				if !reflect.DeepEqual(msg, payload) {
					t.Fatalf("unexpected pushed msg: got %s, want %s",
						spew.Sdump(msg), spew.Sdump(payload))
				}
			case <-time.After(time.Second):
				t.Fatal("timeout")
			}

			// We expect the session to ack the pushed msg.
			gotMsg, gotAck := kx.popWrittenMsg(t)
			if gotMsg.Tag != wantTag {
				t.Fatalf("unexpected tag: got %d, want %d",
					gotMsg.Tag, wantTag)
			}
			if !reflect.DeepEqual(gotAck, tc.wantAck) {
				t.Fatalf("unexpected ack: got %s, want %s",
					spew.Sdump(gotAck), spew.Sdump(tc.wantAck))
			}

			// If this was meant to be a fatal error, we expect the
			// run loop to have ended.
			if !tc.wantRunErr {
				return
			}

			select {
			case err := <-runErr:
				if !errors.Is(err, tc.res) {
					t.Fatalf("unexpected error: got %v, want %v",
						err, tc.res)
				}
			case <-time.After(time.Second):
				t.Fatalf("timeout")
			}
		})
	}
}

// TestExitingRunErrorsReplyChan tests that exiting the run loop sends an
// appropriate error to outstanding SendPRPC calls that are waiting for a
// reply.
func TestExitingRunErrorsReplyChan(t *testing.T) {
	t.Parallel()

	replyChan := make(chan interface{})
	testMsg := rpc.Message{Command: rpc.TaggedCmdRouteMessage}
	testPayload := &rpc.RouteMessage{}
	ctx, cancel := context.WithCancel(context.Background())

	kx := newMockKX()
	ss := newServerSession(offlineConn{}, kx, 10, nil)

	runErr := make(chan error)
	go func() { runErr <- ss.Run(ctx) }()
	time.Sleep(10 * time.Millisecond)

	// Send a message that will be successfully relayed.
	errChan := make(chan error)
	go func() { errChan <- ss.SendPRPC(testMsg, testPayload, replyChan) }()
	kx.popWrittenMsg(t)

	// Ensure the reply hasn't been sent yet.
	select {
	case r := <-replyChan:
		t.Fatalf("unexpected reply: %v", r)
	case <-time.After(50 * time.Millisecond):
	}

	// Cancel the run loop.
	cancel()

	// Expect the replyChan to be given an error
	select {
	case r := <-replyChan:
		err, ok := r.(error)
		if !ok {
			t.Fatalf("did not receive an error: got %#v", err)
		}
		if !errors.Is(err, errRecvLoopExiting) {
			t.Fatalf("unexpected error: got %v, want %v", err,
				errRecvLoopExiting)
		}

	case <-time.After(time.Second):
		t.Fatal("timeout")
	}
}

// TestSessionPingPongs ensures the session keeps ping/ponging the server.
func TestSessionPingPongs(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	kx := newMockKX()
	td := 10
	pingInterval := 50 * time.Millisecond
	ss := newServerSession(offlineConn{}, kx, int64(td), nil)
	ss.logPings = true
	ss.pingInterval = pingInterval

	runErr := make(chan error)
	go func() { runErr <- ss.Run(ctx) }()
	time.Sleep(10 * time.Millisecond)

	// Expect to get a ping.
	gotMsg, _ := kx.popWrittenMsg(t)
	ss.log.Debugf("MockKX: Popped msg %s", gotMsg.Command)
	if gotMsg.Command != rpc.TaggedCmdPing {
		t.Fatalf("unexpected command: got %s, want %s", gotMsg.Command,
			rpc.TaggedCmdPing)
	}
	if gotMsg.Tag != uint32(td) {
		t.Fatalf("unexpected tag: got %d, want: %d", gotMsg.Tag, td)
	}

	// Write a pong.
	kx.pushReadMsg(t, &rpc.Message{Command: rpc.TaggedCmdPong}, rpc.Pong{})
	ss.log.Debugf("MockKX: Wrote Pong")

	// Wait 3/4 of the time until the next ping, and write a dummy message.
	// Expect the next ping message to be delayed until the pingInterval
	// has elapsed from the dummy message.
	go kx.popWrittenMsg(t)
	time.Sleep(pingInterval * 3 / 4)
	ss.SendPRPC(rpc.Message{Command: rpc.TaggedCmdRouteMessage}, rpc.RouteMessage{}, nil)
	ss.log.Debugf("MockKX: Wrote dummy msg")
	lastMsgTime := time.Now()

	// Expect a second ping.
	gotMsg, _ = kx.popWrittenMsg(t)
	nextPingTime := time.Now()
	ss.log.Debugf("MockKX: Popped msg %s", gotMsg.Command)
	if gotMsg.Command != rpc.TaggedCmdPing {
		t.Fatalf("unexpected command: got %s, want %s", gotMsg.Command,
			rpc.TaggedCmdPing)
	}
	if gotMsg.Tag != uint32(td) {
		t.Fatalf("unexpected tag: got %d, want: %d", gotMsg.Tag, td)
	}
	gotPingInterval := nextPingTime.Sub(lastMsgTime)
	if gotPingInterval < pingInterval {
		t.Fatalf("unexpected interval to next ping: got %s, want %s",
			gotPingInterval, pingInterval)
	}

	// Fail to respond with a pong.
	time.Sleep(pingInterval + (10 * time.Millisecond))
	ss.log.Debugf("MockKX: Slept")

	// Expect the session to have been closed due to pong timeout error.
	select {
	case err := <-runErr:
		if !errors.Is(err, errPongTimeout) {
			t.Fatalf("unexpected error: got %v, want %v", err,
				errPongTimeout)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout")
	}
}

// TestRequestCloseAlreadyFinished tests that requesting to close a session that
// was already closed does not block.
func TestRequestCloseAlreadyFinished(t *testing.T) {
	// Canceled context so Run() returns immediately.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	kx := newMockKX()
	ss := newServerSession(offlineConn{}, kx, 0, nil)

	errChan := make(chan error, 1)
	go func() { errChan <- ss.Run(ctx) }()
	gotErr := assert.ChanWritten(t, errChan)
	assert.ErrorIs(t, gotErr, context.Canceled)

	// Attempt to request server session to close again. This must not
	// block.
	errDummy := fmt.Errorf("dummy error")
	assert.DoesNotBlock(t, func() { ss.RequestClose(errDummy) })
}

// TestServerRunCancelsContext asserts that once Run() ends, the server session
// context is canceled.
func TestServerRunCancelsContext(t *testing.T) {
	// Canceled context so Run() returns immediately.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	kx := newMockKX()
	ss := newServerSession(offlineConn{}, kx, 0, nil)

	errChan := make(chan error, 1)
	go func() { errChan <- ss.Run(ctx) }()
	gotErr := assert.ChanWritten(t, errChan)
	assert.ErrorIs(t, gotErr, context.Canceled)

	assert.ContextDone(t, ss.Context())
	assert.ErrorIs(t, ss.Context().Err(), context.Canceled)
}
