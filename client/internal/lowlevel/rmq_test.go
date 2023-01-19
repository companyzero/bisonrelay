package lowlevel

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/companyzero/bisonrelay/rpc"
	"github.com/companyzero/bisonrelay/zkidentity"
)

// TestRMQSuccessRM asserts that the RMQ can successfully send messages to the
// server.
func TestRMQSuccessRM(t *testing.T) {
	t.Parallel()

	mockID := &zkidentity.FullIdentity{}
	q := NewRMQ(nil, clientintf.FreePaymentClient{}, mockID, newMockRMQDB())
	runErr := make(chan error)
	ctx, cancel := context.WithCancel(context.Background())
	go func() { runErr <- q.Run(ctx) }()

	// Bind to the server.
	sess := newMockServerSession()
	q.BindToSession(sess)

	// Send the RM.
	rm := mockRM("test")
	rmErrChan := make(chan error)
	go func() { rmErrChan <- q.SendRM(rm) }()

	// Reply to asking for a payload.
	sess.replyNextPRPC(t, &rpc.GetInvoiceReply{})

	// Expect the RM to be sent.
	sess.replyNextPRPC(t, &rpc.RouteMessageReply{})

	// Ensure no errors occurred.
	select {
	case err := <-runErr:
		t.Fatal(err)
	case err := <-rmErrChan:
		if err != nil {
			t.Fatalf("unexpected error %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}

	// Stop the rmq.
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

// TestRMQAckErrors asserts that when the RM was failed to be acknowledged
// by the server, it's attempted to be sent again.
func TestRMQAckErrors(t *testing.T) {
	t.Parallel()

	mockID := &zkidentity.FullIdentity{}
	q := NewRMQ(nil, clientintf.FreePaymentClient{}, mockID, newMockRMQDB())
	runErr := make(chan error)
	ctx, cancel := context.WithCancel(context.Background())
	go func() { runErr <- q.Run(ctx) }()

	// Bind to the server.
	sess := newMockServerSession()
	q.BindToSession(sess)

	// Send the RM.
	rm := mockRM("test")
	rmErrChan := make(chan error)
	go func() { rmErrChan <- q.SendRM(rm) }()

	// Reply to asking for a payload.
	sess.replyNextPRPC(t, &rpc.GetInvoiceReply{})

	// Reply the send attempt with a session exiting error.
	sess.replyNextPRPC(t, errRecvLoopExiting)

	// Ensure no errors occurred yet.
	select {
	case err := <-runErr:
		t.Fatal(err)
	case err := <-rmErrChan:
		t.Fatalf("unexpected error %v", err)
	case <-time.After(100 * time.Millisecond):
	}

	// Trigger a session change.
	q.BindToSession(nil)
	q.BindToSession(sess)

	// RMQ will try again. Respond with success.
	sess.replyNextPRPC(t, &rpc.GetInvoiceReply{})
	sess.replyNextPRPC(t, &rpc.RouteMessageReply{})

	// Ensure no errors occurred.
	select {
	case err := <-runErr:
		t.Fatal(err)
	case err := <-rmErrChan:
		if err != nil {
			t.Fatalf("unexpected error %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}

	// Stop the rmq.
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

// TestRMQMultipleRM asserts that the RMQ can successfully send multiple
// messages to the servers.
func TestRMQMultipleRM(t *testing.T) {
	t.Parallel()

	nb := 10
	mockID := &zkidentity.FullIdentity{}
	q := NewRMQ(nil, clientintf.FreePaymentClient{}, mockID, newMockRMQDB())
	runErr := make(chan error)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { runErr <- q.Run(ctx) }()

	// Bind to the server.
	sess := newMockServerSession()
	q.BindToSession(sess)

	// Send the RMs concurrently.
	rmErrChan := make(chan error)
	for i := 0; i < nb; i++ {
		rm := mockRM(fmt.Sprintf("test %d", i))
		go func() { rmErrChan <- q.SendRM(rm) }()
	}

	// RMs are sent sequentially, so check each one individually.
	var rmFlag uint64
	for i := 0; i < nb; i++ {
		// Reply to asking for a payload.
		sess.replyNextPRPC(t, &rpc.GetInvoiceReply{})

		select {
		case wm := <-sess.rpcChan:
			var rdvz, msg int
			gotRM, ok := wm.payload.(*rpc.RouteMessage)
			if !ok {
				t.Fatalf("did not receive an RM: got %v", wm.payload)
			}

			_, err := fmt.Sscanf(strFromRVID(gotRM.Rendezvous), "rdzv_test %d", &rdvz)
			if err != nil {
				t.Fatal(err)
			}

			_, err = fmt.Sscanf(hex.EncodeToString(gotRM.Message), "7465737420%x", &msg)
			if err != nil {
				t.Fatal(err)
			}
			msg = msg - 0x30 // ascii to int

			if rdvz != msg {
				t.Fatalf("inconsistency: %d != %d", rdvz, msg)
			}

			mask := uint64(1 << msg)
			if rmFlag&mask == mask {
				t.Fatalf("already got rm %d", msg)
			}
			rmFlag |= mask

			// Send reply.
			select {
			case wm.replyChan <- &rpc.RouteMessageReply{}:
			case <-time.After(time.Second):
				t.Fatal("timeout on reply")
			}

		case <-time.After(time.Second):
			t.Fatal("timeout on receive")
		}

		// Ensure no errors occurred.
		select {
		case err := <-runErr:
			t.Fatal(err)
		case err := <-rmErrChan:
			if err != nil {
				t.Fatalf("unexpected error %v", err)
			}
		case <-time.After(time.Second):
			t.Fatal("timeout")
		}
	}

	// Ensure we got all RMs (even if out of order).
	wantRmFlag := uint64(1<<nb - 1)
	if rmFlag != wantRmFlag {
		t.Fatalf("did not receive all flags: %b", rmFlag)
	}
}

// TestCanceledRMQErrorsRM asserts that stopping the RMQ errors out the sending
// rm.
func TestCanceledRMQErrorsRM(t *testing.T) {
	t.Parallel()

	mockID := &zkidentity.FullIdentity{}
	q := NewRMQ(nil, clientintf.FreePaymentClient{}, mockID, newMockRMQDB())
	runErr := make(chan error)
	ctx, cancel := context.WithCancel(context.Background())
	go func() { runErr <- q.Run(ctx) }()

	// Bind to the server.
	sess := newMockServerSession()
	q.BindToSession(sess)

	// Send the RM.
	rm := mockRM("test")
	rmErrChan := make(chan error)
	go func() { rmErrChan <- q.SendRM(rm) }()

	// Cancel the RMQ loop.
	cancel()

	// Ensure the original RM errored.
	select {
	case err := <-rmErrChan:
		if !errors.Is(err, errRMQExiting) {
			t.Fatalf("unexpected error: got %v, want %v", err,
				errRMQExiting)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}

	// Ensure the RMQ stopped.
	select {
	case err := <-runErr:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("unexpected error: got %v, want %v", err, context.Canceled)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}
}

// TestCanceledRMQAfterQueuedErrorsRM tests that cancelling the RMQ after an
// RM was queued correctly errors it. This tests an old buggy scenario.
func TestCanceledRMQAfterQueuedErrorsRM(t *testing.T) {
	t.Parallel()

	mockID := &zkidentity.FullIdentity{}
	q := NewRMQ(nil, clientintf.FreePaymentClient{}, mockID, newMockRMQDB())
	runErr := make(chan error)
	ctx, cancel := context.WithCancel(context.Background())
	go func() { runErr <- q.Run(ctx) }()

	// Bind to the server.
	sess := newMockServerSession()
	q.BindToSession(sess)

	// Send the RM.
	rm := mockRM("test")
	rmErrChan := make(chan error)
	err := q.QueueRM(rm, rmErrChan)
	if err != nil {
		t.Fatal(err)
	}

	// RM was queued. Cancel the RMQ loop.
	cancel()

	// If the RM got all the way to being sent, cancel sending.
	select {
	case <-sess.sendErrChan:
	case <-time.After(10 * time.Millisecond):
	}

	// Ensure the original RM errored.
	select {
	case err := <-rmErrChan:
		if !errors.Is(err, errRMQExiting) {
			t.Fatalf("unexpected error: got %v, want %v", err,
				errRMQExiting)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}

	// Ensure the RMQ stopped.
	select {
	case err := <-runErr:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("unexpected error: got %v, want %v", err, context.Canceled)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}
}

// TestEnqueUntilSessionSucceeds asserts that sending an RM succeeds even if it
// takes multiple session attempts.
func TestEnqueueRMBeforeSession(t *testing.T) {
	t.Parallel()

	mockID := &zkidentity.FullIdentity{}
	q := NewRMQ(nil, clientintf.FreePaymentClient{}, mockID, newMockRMQDB())
	runErr := make(chan error)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { runErr <- q.Run(ctx) }()

	// Enqueue multiple RMs.
	wantRMs := make([]mockRM, 10)
	rmErrChan := make(chan error)
	for i := range wantRMs {
		i := i
		wantRMs[i] = mockRM(fmt.Sprintf("test %d", i))
		go func() { rmErrChan <- q.SendRM(wantRMs[i]) }()
		time.Sleep(10 * time.Millisecond) // Ensure rms are enqueued in order
	}

	sess := newMockServerSession()

	// Attempt to send via multiple sessions.
	for i := 0; i < 10; i++ {
		// Bind to the new session.
		q.BindToSession(sess)

		// Simulate a failing session.
		sess.failNextPRPC(t)

		// Assert no errors.
		select {
		case err := <-runErr:
			t.Fatal(err)
		case err := <-rmErrChan:
			t.Fatal(err)
		case <-time.After(50 * time.Millisecond):
		}
	}

	// Test a successful session now.
	q.BindToSession(sess)

	// Accept all RMs.
	for range wantRMs {
		// Reply to asking for a payload.
		sess.replyNextPRPC(t, &rpc.GetInvoiceReply{})

		// Expect the RM to be sent.
		sess.replyNextPRPC(t, &rpc.RouteMessageReply{})

		// Ensure no errors occurred.
		select {
		case err := <-runErr:
			t.Fatal(err)
		case err := <-rmErrChan:
			if err != nil {
				t.Fatalf("unexpected error %v", err)
			}
		case <-time.After(time.Second):
			t.Fatal("timeout")
		}
	}
}

// TestRMQEncryptErrorFailsRM tests that a failure to encrypt an outbound RM
// causes the RM to fail but the RMQ to keep working.
func TestRMQEncryptErrorFailsRM(t *testing.T) {
	t.Parallel()

	mockID := &zkidentity.FullIdentity{}
	q := NewRMQ(nil, clientintf.FreePaymentClient{}, mockID, newMockRMQDB())
	runErr := make(chan error)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { runErr <- q.Run(ctx) }()

	// Bind to the server.
	sess := newMockServerSession()
	q.BindToSession(sess)

	// Send the failing RM.
	failRM := mockFailedRM{}
	rmErrChan := make(chan error)
	go func() { rmErrChan <- q.SendRM(failRM) }()

	// Reply to asking for a payload.
	sess.replyNextPRPC(t, &rpc.GetInvoiceReply{})

	// Expect the RM to fail to encrypt.
	select {
	case err := <-rmErrChan:
		if !errors.Is(err, mockRMError) {
			t.Fatalf("unexpected error: got %v, want %v", err, mockRMError)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}

	// Send an RM that will encrypt.
	rm := mockRM("test")
	go func() { rmErrChan <- q.SendRM(rm) }()

	// Reply to asking for a payload.
	sess.replyNextPRPC(t, &rpc.GetInvoiceReply{})

	// Expect the RM to be sent.
	sess.replyNextPRPC(t, &rpc.RouteMessageReply{})

	// Assert no errors occurred.
	select {
	case err := <-rmErrChan:
		if err != nil {
			t.Fatalf("unexpected error %v", err)
		}

	case <-time.After(time.Second):
		t.Fatal("timeout")
	}

	// Assert no run errors occurred.
	select {
	case err := <-runErr:
		t.Fatal(err)
	case <-time.After(50 * time.Millisecond):
	}
}

// TestRMQMaxMsgSizeErrors tests that attempting to queue a message larger then
// the max size fails.
func TestRMQMaxMsgSizeErrors(t *testing.T) {
	t.Parallel()
	const maxMsgSize = rpc.MaxMsgSize
	mockID := &zkidentity.FullIdentity{}
	q := NewRMQ(nil, clientintf.FreePaymentClient{}, mockID, newMockRMQDB())
	rm := mockRM(strings.Repeat(" ", maxMsgSize+1))
	err := q.SendRM(rm)
	if !errors.Is(err, errORMTooLarge) {
		t.Fatalf("Unexpected error: got %v, want %v", err,
			errORMTooLarge)
	}
}
