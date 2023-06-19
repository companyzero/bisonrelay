package lowlevel

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/companyzero/bisonrelay/internal/assert"
	"github.com/companyzero/bisonrelay/rpc"
	"golang.org/x/exp/slices"
)

func assertSubAdded(t testing.TB, msg interface{}, id RVID) {
	subMsg := msg.(*rpc.SubscribeRoutedMessages)
	idx := slices.Index(subMsg.AddRendezvous, id)
	if idx == -1 {
		t.Fatalf("RV %d not included in added rendezvous", id)
	}
}

func assertSubDeleted(t testing.TB, msg interface{}, id RVID) {
	subMsg := msg.(*rpc.SubscribeRoutedMessages)
	idx := slices.Index(subMsg.DelRendezvous, id)
	if idx == -1 {
		t.Fatalf("RV %d not included in deleted rendezvous", id)
	}
}

// TestSuccessRendezvousManager asserts the rendezvous manager works when
// receiving messages about the subscription.
func TestSuccessRendezvousManager(t *testing.T) {
	t.Parallel()

	rnd := testRand(t)
	rmgr := NewRVManager(nil, &mockRvMgrDB{alwaysPaid: true}, nil, nil)
	delayChan := make(chan time.Time)
	rmgr.subsDelayer = func() <-chan time.Time { return delayChan }
	ctx, cancel := context.WithCancel(context.Background())
	runErr := make(chan error, 1)
	go func() { runErr <- rmgr.Run(ctx) }()

	sess := newMockServerSession()
	rmgr.BindToSession(sess)

	// Register concurrently.
	nb := 11
	blobs := make(map[RVID][]byte, nb)
	subDoneChan := make(chan error, nb)
	for i := 0; i < nb; i++ {
		blob := make([]byte, 32)
		rnd.Read(blob)
		id := rvidFromStr(fmt.Sprintf("rdzv-%d", i))
		blobs[id] = blob
		handler := func(gotBlob RVBlob) error {
			if !bytes.Equal(gotBlob.Decoded, blob) {
				return fmt.Errorf("unexpected blob: got %x, want %x",
					gotBlob.Decoded, blob)
			}

			return nil
		}
		go func() { subDoneChan <- rmgr.Sub(id, handler, nil) }()
	}

	// Trigger a resubscription.
	time.Sleep(time.Millisecond * 100)
	select {
	case delayChan <- time.Now():
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}
	payload := sess.replyNextPRPC(t, &rpc.SubscribeRoutedMessagesReply{})

	// Assert every client was added to the subscription.
	subsMsg, ok := payload.(*rpc.SubscribeRoutedMessages)
	if !ok {
		t.Fatalf("not the right type: %T", payload)
	}
	var gotFlags uint64
	for _, r := range subsMsg.AddRendezvous {
		var i uint64
		if _, err := fmt.Sscanf(strFromRVID(r), "rdzv-%d", &i); err != nil {
			t.Fatal(err)
		}
		mask := uint64(1 << i)
		if gotFlags&mask == mask {
			t.Fatalf("already got rendezvous %d", i)
		}
		gotFlags |= mask
	}
	wantFlags := uint64(1<<nb) - 1
	if gotFlags != wantFlags {
		t.Fatalf("missing rendezvous: got %b, want %b", gotFlags, wantFlags)
	}

	// Ensure every subscription was done.
	for i := 0; i < nb; i++ {
		select {
		case err := <-subDoneChan:
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		case <-time.After(time.Second):
			t.Fatal("timeout")
		}
	}

	// Trigger every subscription. They should all be handled correctly.
	for id, blob := range blobs {
		msg := &rpc.PushRoutedMessage{}
		msg.Payload = blob
		msg.RV = id
		delete(blobs, id)
		err := rmgr.HandlePushedRMs(msg)
		if err != nil {
			t.Fatal(err)
		}
	}

	// Cancel the context.
	cancel()

	// Ensure run was done.
	select {
	case err := <-runErr:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("unexpected error: got %v, want %v",
				err, context.Canceled)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}
}

// TestUnsubbedRendezvous asserts that subbing then unsubbing from a rendezvous
// point does not cause the handler to be incorrectly called.
//
// Additionally, it tests the behavior of IsUpToDate.
func TestUnsubbedRendezvous(t *testing.T) {
	t.Parallel()

	// Create the manager with a delay so we can test IsUpToDate() in
	// intermediate sync states.
	sleepDuration := time.Millisecond * 10
	rmgr := NewRVManager(nil, &mockRvMgrDB{alwaysPaid: true}, nil, nil)
	rmgr.subsDelayer = func() <-chan time.Time { return time.After(sleepDuration * 2) }

	assertUpToDateIs := func(want bool) {
		t.Helper()
		var got bool
		for i := 0; i < 100; i++ {
			got = rmgr.IsUpToDate()
			if got == want {
				return
			}
			time.Sleep(sleepDuration)
		}
		t.Fatalf("Unexpected IsUpToDate value. got %v, want %v",
			got, want)
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	runErr := make(chan error, 1)
	go func() { runErr <- rmgr.Run(ctx) }()
	assertUpToDateIs(true)

	sess := newMockServerSession()
	rmgr.BindToSession(sess)
	assertUpToDateIs(true)

	// Register.
	id := rvidFromStr("test-id")
	subDoneChan := make(chan error, 1)
	handler := func(gotBlob RVBlob) error {
		return fmt.Errorf("handler should not have been called")
	}
	go func() { subDoneChan <- rmgr.Sub(id, handler, nil) }()
	assertUpToDateIs(false)
	time.Sleep(sleepDuration * 2)
	assertUpToDateIs(false)

	// Finish subscription.
	sess.replyNextPRPC(t, &rpc.SubscribeRoutedMessagesReply{})
	select {
	case err := <-subDoneChan:
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}
	assertUpToDateIs(true)

	// Unsubscribe.
	go func() { subDoneChan <- rmgr.Unsub(id) }()
	assertUpToDateIs(false)
	time.Sleep(sleepDuration * 2)
	assertUpToDateIs(false)

	// Finish unsubscription.
	sess.replyNextPRPC(t, &rpc.SubscribeRoutedMessagesReply{})
	select {
	case err := <-subDoneChan:
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}
	assertUpToDateIs(true)

	// Push the message.
	msg := &rpc.PushRoutedMessage{
		Payload: []byte("blob"),
		RV:      id,
	}
	err := rmgr.HandlePushedRMs(msg)
	if err != nil {
		t.Fatal(err)
	}

	// Shutdown manager.
	cancel()
	assertUpToDateIs(false)
}

// TestSubRendezvousFailingSession asserts that subscriptions are sent once a
// non-failing session is bound to the manager.
func TestSubRendezvousFailingSession(t *testing.T) {
	t.Parallel()

	rmgr := NewRVManager(nil, &mockRvMgrDB{alwaysPaid: true}, nil, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runErr := make(chan error, 1)
	go func() { runErr <- rmgr.Run(ctx) }()

	// Register.
	id := rvidFromStr("test-id")
	subDoneChan := make(chan error, 1)
	handlerCalled := make(chan struct{})
	handler := func(gotBlob RVBlob) error {
		close(handlerCalled)
		return nil
	}
	go func() { subDoneChan <- rmgr.Sub(id, handler, nil) }()

	// Attempt multiple session binds that end up failing due to an error
	// sending the subscription msg.
	sess := newMockServerSession()
	nb := 10
	for i := 0; i < nb; i++ {
		rmgr.BindToSession(sess)

		// Assert subscription is not done yet.
		sess.failNextPRPC(t)
		select {
		case err := <-subDoneChan:
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		case <-time.After(50 * time.Millisecond):
		}
	}

	// Finish subscription.
	rmgr.BindToSession(sess)
	sess.replyNextPRPC(t, &rpc.SubscribeRoutedMessagesReply{})
	select {
	case err := <-subDoneChan:
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}

	// Push the message.
	msg := &rpc.PushRoutedMessage{
		Payload: []byte("blob"),
		RV:      id,
	}
	err := rmgr.HandlePushedRMs(msg)
	if err != nil {
		t.Fatal(err)
	}

	// Ensure handler was called.
	select {
	case <-handlerCalled:
	case <-time.After(10 * time.Second):
		t.Fatal("timeout")
	}
}

// TestRendezvousActionsAfterCancel ensures that calls to API of the rendezvous
// manager fail after the manager is cancelled.
func TestRendezvousActionsAfterCancel(t *testing.T) {
	t.Parallel()

	rmgr := NewRVManager(nil, &mockRvMgrDB{alwaysPaid: true}, nil, nil)
	ctx, cancel := context.WithCancel(context.Background())
	runErr := make(chan error, 1)
	go func() { runErr <- rmgr.Run(ctx) }()

	id := rvidFromStr("test-id")
	handlerCalledChan := make(chan error, 1)
	handler := func(gotBlob RVBlob) error {
		go func() { handlerCalledChan <- fmt.Errorf("handler should not have been called") }()
		return nil
	}

	subDoneChan := make(chan error, 1)
	unsubDoneChan := make(chan error, 1)
	handlerChan := make(chan error, 1)
	go func() { subDoneChan <- rmgr.Sub(id, handler, nil) }()

	cancel()
	time.Sleep(10 * time.Millisecond)
	msg := &rpc.PushRoutedMessage{
		Payload: []byte("blob"),
		RV:      id,
	}
	go func() { handlerChan <- rmgr.HandlePushedRMs(msg) }()
	go func() { unsubDoneChan <- rmgr.Unsub(id) }()

	// Assert API calls errored with the appropriate error.
	for i, c := range []chan error{subDoneChan, unsubDoneChan, handlerChan} {
		select {
		case err := <-c:
			if !errors.Is(err, errRdvzMgrExiting) {
				t.Fatalf("unexpected error at %d: got %v, want %v",
					i, err, errRdvzMgrExiting)
			}
		case <-time.After(time.Second):
			t.Fatalf("timeout at %d", i)
		}
	}

	// Assert the handler was not called.
	select {
	case err := <-handlerCalledChan:
		t.Fatalf("handler was called: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
}

// TestRendezvousManagerNilSession asserts the rendezvous manager works when
// subbed and bound to a nil session.
func TestRendezvousManagerNilSession(t *testing.T) {
	t.Parallel()

	rmgr := NewRVManager(nil, &mockRvMgrDB{alwaysPaid: true}, nil, nil)
	//rmgr.log = testLogger(t)
	delayChan := make(chan time.Time)
	rmgr.subsDelayer = func() <-chan time.Time { return delayChan }
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runErr := make(chan error, 1)
	go func() { runErr <- rmgr.Run(ctx) }()

	sess := newMockServerSession()
	rmgr.BindToSession(sess)

	// Register.
	id := rvidFromStr("test-id")
	id2 := rvidFromStr("test-id2")
	errChan := make(chan error)
	subDoneChan := make(chan error, 1)
	handler := func(gotBlob RVBlob) error {
		errChan <- fmt.Errorf("handler should not have been called")
		return nil
	}
	go func() { subDoneChan <- rmgr.Sub(id, handler, nil) }()

	// Finish subscription.
	select {
	case delayChan <- time.Now():
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}
	sess.replyNextPRPC(t, &rpc.SubscribeRoutedMessagesReply{})
	select {
	case err := <-subDoneChan:
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}

	// Bind to a nil session.
	rmgr.BindToSession(nil)

	// Send a new subscription.
	go func() { subDoneChan <- rmgr.Sub(id2, handler, nil) }()
	select {
	case delayChan <- time.Now():
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}

	// Expect to not have subscribed yet and the manager to not have
	// failed.
	select {
	case err := <-subDoneChan:
		t.Fatal(err)
	case err := <-runErr:
		t.Fatal(err)
	case <-time.After(time.Millisecond * 50):
	}

	// Bind back to a valid session.
	rmgr.BindToSession(sess)

	// Registration should've been re-made and the second sub should've
	// been triggered.
	sess.replyNextPRPC(t, &rpc.SubscribeRoutedMessagesReply{})
	select {
	case err := <-subDoneChan:
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}
}

// TestRendezvousManagerPrepaysWorks asserts that prepaying for an RV
// does not cause the manager to subscribe to that RV, and (erroneously)
// receiving the message does not cause any errors.
func TestRendezvousManagerPrepaysWorks(t *testing.T) {
	t.Parallel()

	rmgr := NewRVManager(nil, &mockRvMgrDB{}, nil, nil)
	//rmgr.log = testutils.TestLoggerSys(t, "XXXX")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runErr := make(chan error, 1)
	go func() { runErr <- rmgr.Run(ctx) }()

	sess := newMockServerSession()
	rmgr.BindToSession(sess)

	// Hook into the mock pay client to catch any attempts at paying.
	gotPayInvoice := make(chan struct{}, 1)
	sess.mpc.HookPayInvoice(func(_ string) (int64, error) {
		gotPayInvoice <- struct{}{}
		return 0, nil
	})

	// Prepay.
	id := rvidFromStr("test-id")
	errChan := make(chan error, 1)
	go func() { errChan <- rmgr.PrepayRVSub(id, nil) }()

	// RV manager asks for invoice and attempts to pay it.
	sess.replyNextPRPC(t, &rpc.GetInvoiceReply{})
	assert.ChanWritten(t, gotPayInvoice)

	// Finish subscription.
	gotMsg := sess.replyNextPRPC(t, &rpc.SubscribeRoutedMessagesReply{})
	assert.NilErrFromChan(t, errChan)
	gotSubMsg := gotMsg.(*rpc.SubscribeRoutedMessages)
	if len(gotSubMsg.MarkPaid) != 1 {
		t.Fatalf("wrong nb of MarkPaid elements: got %d, want 1", len(gotSubMsg.MarkPaid))
	}
	if len(gotSubMsg.AddRendezvous) != 0 {
		t.Fatalf("wrong nb of AddRendezvous elements: got %d, want 0", len(gotSubMsg.AddRendezvous))
	}
	assert.DeepEqual(t, gotSubMsg.MarkPaid[0], id)

	// Push a routed message using the id. It should not error the manager.
	msg := &rpc.PushRoutedMessage{
		Payload: []byte("payload"),
		RV:      id,
	}
	assert.NilErr(t, rmgr.HandlePushedRMs(msg))
	assert.ChanNotWritten(t, runErr, 100*time.Millisecond)
}

// TestRendezvousManagerFetchPrepaidCancellable tests that attempting to fetch
// a prepaid RV is cancellable.
func TestRendezvousManagerFetchPrepaidCancellable(t *testing.T) {
	t.Parallel()

	ctxb := context.Background()
	rmgr := NewRVManager(nil, &mockRvMgrDB{alwaysPaid: true}, nil, nil)
	delayChan := make(chan time.Time)
	rmgr.subsDelayer = func() <-chan time.Time { return delayChan }
	//rmgr.log = testutils.TestLoggerSys(t, "XXXX")
	runCtx, cancelRun := context.WithCancel(ctxb)
	defer cancelRun()

	sess := newMockServerSession()

	// Helper to ask to fetch an id.
	var ctx context.Context
	var cancel func()
	id := rvidFromStr("test-id")
	errChan := make(chan error, 3)
	fetchPrepaidRV := func() {
		go func() {
			_, gotErr := rmgr.FetchPrepaidRV(ctx, id)
			errChan <- gotErr
		}()
	}

	// Ask to fetch before run() starts, but cancel it.
	ctx, cancel = context.WithCancel(ctxb)
	cancel()
	fetchPrepaidRV()
	assert.ChanWrittenWithVal(t, errChan, context.Canceled)

	// Run. No messages sent yet.
	runErr := make(chan error, 1)
	go func() { runErr <- rmgr.Run(runCtx) }()
	rmgr.BindToSession(sess)
	select {
	case delayChan <- time.Now():
		t.Fatal("messages were scheduled for sending")
	case <-time.After(100 * time.Millisecond):
	}
	sess.assertNoMessages(t, 100*time.Millisecond)

	// Ask to fetch but cancel before the sub is sent to server. The sub
	// is still sent but is canceled afterwards.
	ctx, cancel = context.WithCancel(ctxb)
	fetchPrepaidRV()
	assert.ChanNotWritten(t, errChan, 100*time.Millisecond)
	cancel()
	assert.ChanWrittenWithVal(t, errChan, context.Canceled)
	select {
	case delayChan <- time.Now():
	case <-time.After(100 * time.Millisecond):
		t.Fatal("message to sub was not scheduled")
	}
	gotMsg := sess.replyNextPRPC(t, &rpc.SubscribeRoutedMessagesReply{}) // Sub done
	assertSubAdded(t, gotMsg, id)
	select {
	case delayChan <- time.Now():
	case <-time.After(100 * time.Millisecond):
		t.Fatal("message to sub was not scheduled")
	}
	gotMsg = sess.replyNextPRPC(t, &rpc.SubscribeRoutedMessagesReply{}) // Unsub done
	assertSubDeleted(t, gotMsg, id)
	assert.ChanNotWritten(t, runErr, 100*time.Millisecond)
}

// TestRendezvousManagerFetchPrepaidWorks asserts that fetching a prepaid RV
// works as intended.
func TestRendezvousManagerFetchPrepaidWorks(t *testing.T) {
	t.Parallel()

	rmgr := NewRVManager(nil, &mockRvMgrDB{}, nil, nil)
	//rmgr.log = testutils.TestLoggerSys(t, "XXXX")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runErr := make(chan error, 1)
	go func() { runErr <- rmgr.Run(ctx) }()
	sess := newMockServerSession()
	rmgr.BindToSession(sess)

	// Hook into the mock pay client to catch any attempts at paying.
	gotPayInvoice := make(chan error, 1)
	sess.mpc.HookPayInvoice(func(_ string) (int64, error) {
		gotPayInvoice <- fmt.Errorf("got attempt at paying invoice")
		return 0, fmt.Errorf("not allowed")
	})

	id := rvidFromStr("test-id")
	errChan := make(chan error, 1)
	wantPayload := []byte("payload")
	go func() {
		gotBlob, gotErr := rmgr.FetchPrepaidRV(ctx, id)
		if gotErr != nil {
			errChan <- gotErr
		} else if !bytes.Equal(gotBlob.Decoded, wantPayload) {
			errChan <- fmt.Errorf("got incorrect blob payload")
		} else {
			errChan <- nil
		}
	}()

	// Complete the subscription.
	gotMsg := sess.replyNextPRPC(t, &rpc.SubscribeRoutedMessagesReply{}) // Sub done
	assertSubAdded(t, gotMsg, id)

	// No errors yet.
	assert.ChanNotWritten(t, gotPayInvoice, 100*time.Millisecond)
	assert.ChanNotWritten(t, errChan, 100*time.Millisecond)

	// Push the blob.
	msg := &rpc.PushRoutedMessage{
		Payload: wantPayload,
		RV:      id,
	}
	assert.NilErr(t, rmgr.HandlePushedRMs(msg))

	// Got the correct blob.
	assert.NilErrFromChan(t, errChan)
}
