package waitingq

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestWaitingQueue(t *testing.T) {
	t.Parallel()

	// Sending a reply before someone is waiting for a response triggers an
	// error.
	ctxb := context.Background()
	q := new(WaitingReplyQueue)
	var v = 10
	err := q.ReplyLastSend(ctxb, v)
	if !errors.Is(err, errReplyBeforeRequest) {
		t.Fatalf("unexpected error. got %v, want %v", err, errReplyBeforeRequest)
	}

	// Attempt multiple concurrent sends.
	send1, send2 := make(chan chan interface{}), make(chan chan interface{})
	go func() { send1 <- q.WaitForReadyToSend(ctxb) }()
	time.Sleep(10 * time.Millisecond) // Ensure order.
	go func() { send2 <- q.WaitForReadyToSend(ctxb) }()

	// send1 should have received the reply chan.
	var replyChan1, replyChan2 chan interface{}
	select {
	case replyChan1 = <-send1:
	case <-time.After(10 * time.Second):
		t.Fatal("timeout")
	}

	// send2 should _not_ have received the reply chan.
	select {
	case <-send2:
		t.Fatal("send2 received unexpected value")
	case <-time.After(50 * time.Millisecond):
	}

	// Send first reply.
	v1 := 20
	err = q.ReplyLastSend(ctxb, v1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should receive first reply.
	select {
	case gotV1 := <-replyChan1:
		if gotV1 != v1 {
			t.Fatalf("unexpected v1: got %v, want %v", gotV1, v1)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("timeout")
	}

	// send2 should have received the reply chan.
	select {
	case replyChan2 = <-send2:
	case <-time.After(10 * time.Second):
		t.Fatal("timeout")
	}

	// Send second reply.
	v2 := 30
	err = q.ReplyLastSend(ctxb, v2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should receive second reply.
	select {
	case gotV2 := <-replyChan2:
		if gotV2 != v2 {
			t.Fatalf("unexpected v2: got %v, want %v", gotV2, v2)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("timeout")
	}
}
