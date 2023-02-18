package lowlevel

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/companyzero/bisonrelay/rpc"
)

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
			} else {
				return nil
			}
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
