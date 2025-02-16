package lowlevel

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/companyzero/bisonrelay/internal/assert"
	"github.com/companyzero/bisonrelay/rpc"
)

// TestRMQSuccessRM asserts that the RMQ can successfully send messages to the
// server.
func TestRMQSuccessRM(t *testing.T) {
	t.Parallel()

	q := NewRMQ(nil, newMockRMQDB())
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

	q := NewRMQ(nil, newMockRMQDB())
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

// TestRMQClearsInvoice asserts that when an appropriate error is returned from
// the server, a new attempt to fetch an invoice is made.
func TestRMQClearsInvoice(t *testing.T) {
	t.Parallel()

	q := NewRMQ(nil, newMockRMQDB())
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

	// Reply to asking for the first invoice.
	sess.replyNextPRPC(t, &rpc.GetInvoiceReply{})

	// Reply the send attempt with an RPC error that triggers a new
	// invoice fetch.
	sess.replyNextPRPC(t, rpc.RouteMessageReply{Error: rpc.ErrRMInvoicePayment.Error()})

	// Ensure no errors occurred yet.
	select {
	case err := <-runErr:
		t.Fatal(err)
	case err := <-rmErrChan:
		t.Fatalf("unexpected error %v", err)
	case <-time.After(100 * time.Millisecond):
	}

	// Reply to asking for the second invoice.
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

// TestRMQMultipleRM asserts that the RMQ can successfully send multiple
// messages to the servers.
func TestRMQMultipleRM(t *testing.T) {
	t.Parallel()

	nb := 10
	q := NewRMQ(nil, newMockRMQDB())
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

	q := NewRMQ(nil, newMockRMQDB())
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

	q := NewRMQ(nil, newMockRMQDB())
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

	q := NewRMQ(nil, newMockRMQDB())
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

	q := NewRMQ(nil, newMockRMQDB())
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
	q := NewRMQ(nil, newMockRMQDB())
	maxMsgSize := int(q.maxMsgSize.Load())
	rm := mockRM(strings.Repeat(" ", maxMsgSize+1))
	err := q.SendRM(rm)
	if !errors.Is(err, errORMTooLarge) {
		t.Fatalf("Unexpected error: got %v, want %v", err,
			errORMTooLarge)
	}
}

// TestReusesPaidRM tests that attempting to send a message for which payment
// had already been made reuses the same (already paid) invoice id.
func TestReusesPaidRM(t *testing.T) {
	t.Parallel()

	invoice := "paidinvoice"
	db := newMockRMQDB()
	rm := mockRM("test")
	rv, _, _ := rm.EncryptedMsg()
	db.StoreRVPaymentAttempt(rv, invoice, time.Now())

	q := NewRMQ(nil, db)
	runErr := make(chan error)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { runErr <- q.Run(ctx) }()

	// Bind to the server.
	sess := newMockServerSession()
	q.BindToSession(sess)

	// Send the RM.
	rmErrChan := make(chan error)
	go func() { rmErrChan <- q.SendRM(rm) }()

	// The RMQ will reuse the same invoice, therefore it won't request a new
	// one.

	// Expect the RM to be sent.
	reply := sess.replyNextPRPC(t, &rpc.RouteMessageReply{})
	sentRM, ok := reply.(*rpc.RouteMessage)
	if !ok {
		t.Fatalf("Unexpected message from RMQ. got %T, want %T",
			reply, rpc.RouteMessage{})
	}
	var wantInvoiceID [32]byte
	copy(wantInvoiceID[:], invoice)
	if !bytes.Equal(sentRM.PaidInvoiceID, wantInvoiceID[:]) {
		t.Fatalf("Unexpected paid invoice ID: got %x, want %x",
			sentRM.PaidInvoiceID, wantInvoiceID)
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

// TestRequestsInvoiceForOldInvoice tests that when a successful payment attempt
// was made too long ago (longer than the max acceptable), that a new invoice is
// requested from the server and that this new invoice is used to pay for an
// outbound RM.
func TestRequestsInvoiceForOldInvoice(t *testing.T) {
	t.Parallel()

	invoice := "paidinvoice"
	db := newMockRMQDB()
	rm := mockRM("test")
	rv, _, _ := rm.EncryptedMsg()
	db.StoreRVPaymentAttempt(rv, invoice, time.Now().Add(-time.Hour*24*2))

	q := NewRMQ(nil, db)
	runErr := make(chan error)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { runErr <- q.Run(ctx) }()

	// Bind to the server.
	sess := newMockServerSession()
	q.BindToSession(sess)

	// Send the RM.
	rmErrChan := make(chan error)
	go func() { rmErrChan <- q.SendRM(rm) }()

	// Reply to asking for a payload. Send a new invoice that was not
	// marked as paid.
	newInvoice := "newinvoice"
	sess.replyNextPRPC(t, &rpc.GetInvoiceReply{Invoice: newInvoice})

	// Expect the RM to be sent.
	reply := sess.replyNextPRPC(t, &rpc.RouteMessageReply{})
	sentRM, ok := reply.(*rpc.RouteMessage)
	if !ok {
		t.Fatalf("Unexpected message from RMQ. got %T, want %T",
			reply, rpc.RouteMessage{})
	}
	var wantInvoiceID [32]byte
	copy(wantInvoiceID[:], newInvoice)
	if !bytes.Equal(sentRM.PaidInvoiceID, wantInvoiceID[:]) {
		t.Fatalf("Unexpected paid invoice ID: got %x, want %x",
			sentRM.PaidInvoiceID, wantInvoiceID)
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

// TestConcurrentRMQSends tests that the RMQ can send multiple outbound RMs
// before any one is confirmed by the server.
func TestConcurrentRMQSends(t *testing.T) {
	t.Parallel()

	// Number of concurrent RMs to use in test (one more than this will be
	// sent).
	const nbRMs = 3

	q := NewRMQ(nil, newMockRMQDB())

	// Hook into the beforeFetchInvoiceHook event to reduce chance of test
	// failure.
	beforeFetchInvoiceChan := make(chan struct{}, nbRMs+1)
	q.beforeFetchInvoiceHook = func() {
		select {
		case <-beforeFetchInvoiceChan:
		case <-time.After(time.Second * 10):
		}
	}

	runErr := make(chan error)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { runErr <- q.Run(ctx) }()

	// Bind to the server.
	sess := newMockServerSession()
	sess.policy.MaxPushInvoices = nbRMs
	q.BindToSession(sess)

	// Hook into invoice payment to reduce chances of test flakiness.
	payInvoiceChan := make(chan struct{}, nbRMs+1)
	sess.mpc.HookPayInvoice(func(string) (int64, error) {
		select {
		case <-payInvoiceChan:
			return 0, nil
		case <-ctx.Done():
			return 0, ctx.Err()
		}
	})

	// Send the RM. Send one more than the max number of concurrent RMs to
	// test the additional RM is enqueued, but not sent.
	rmErrChan := make(chan error, nbRMs+1)
	for i := 0; i < (nbRMs + 1); i++ {
		rm := mockRM(fmt.Sprintf("test_%d", i))
		go func() { rmErrChan <- q.SendRM(rm) }()
	}

	// Wait for all invoices to be asked.
	time.Sleep(50 * time.Millisecond)

	// One RM should be queued.
	gotLen, _ := q.Len()
	if gotLen != 1 {
		t.Fatalf("Unexpected queue len: got %d, want 1", gotLen)
	}

	// Reply to asking for a payload.
	for i := 0; i < nbRMs; i++ {
		assert.WriteChan(t, beforeFetchInvoiceChan, struct{}{})
	}
	time.Sleep(50 * time.Millisecond)
	for i := 0; i < nbRMs; i++ {
		sess.replyNextPRPC(t, &rpc.GetInvoiceReply{})
	}
	time.Sleep(50 * time.Millisecond)

	// Complete the payments.
	for i := 0; i < nbRMs; i++ {
		assert.WriteChan(t, payInvoiceChan, struct{}{})
	}

	// Expect the RMs to be sent.
	for i := 0; i < nbRMs; i++ {
		sess.replyNextPRPC(t, &rpc.RouteMessageReply{})
	}

	// Ensure no errors occurred.
	for i := 0; i < nbRMs; i++ {
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

	// The final RM is not yet successfully sent.
	assert.ChanNotWritten(t, rmErrChan, 100*time.Millisecond)

	// Send the invoice and accept the last RM.
	assert.WriteChan(t, beforeFetchInvoiceChan, struct{}{})
	sess.replyNextPRPC(t, &rpc.GetInvoiceReply{})
	assert.WriteChan(t, payInvoiceChan, struct{}{})
	sess.replyNextPRPC(t, &rpc.RouteMessageReply{})

	// Ensure no errors occurred on the final RM.
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

	// Queue should be empty.
	gotQueue, gotSend := q.Len()
	gotLen = gotQueue + gotSend
	if gotLen != 0 {
		t.Fatalf("Unexpected queue len: got %d, want 0", gotLen)
	}
}
