package waitingq

import (
	"container/list"
	"context"
	"errors"
	"sync"
)

var errReplyBeforeRequest = errors.New("received reply before corresponding request")

// waitingReplyQueue abstracts a queue where requests on multiple goroutines
// must be made one at a time to a backing service because replies must be
// received in order.
//
// A new initialized value is ready for use and must not be copied. Note that
// passing different contexts to the different functions may result in memory
// leaks.
type WaitingReplyQueue struct {
	sync.Mutex
	waitingToSend *list.List
	nextReplyChan chan interface{}
}

// waitForReadyToSend returns nil if the context is canceled or a chan where
// the next reply of this waiting queue will be received.
func (wq *WaitingReplyQueue) WaitForReadyToSend(ctx context.Context) chan interface{} {
	for {
		wq.Lock()
		if wq.nextReplyChan == nil {
			// Ready to send.
			replyChan := make(chan interface{})
			wq.nextReplyChan = replyChan
			wq.Unlock()
			return replyChan
		}

		myTurnChan := make(chan chan interface{})
		if wq.waitingToSend == nil {
			wq.waitingToSend = list.New()
		}
		wq.waitingToSend.PushBack(myTurnChan)
		wq.Unlock()

		select {
		case replyChan := <-myTurnChan:
			return replyChan
		case <-ctx.Done():
			return nil
		}
	}
}

// ReplyLastSend pushes the reply to the last unblocked send. It returns an
// error if there are no callers waiting for a reply.
func (wq *WaitingReplyQueue) ReplyLastSend(ctx context.Context, v interface{}) error {
	wq.Lock()
	// Store the chan where we need to send the reply.
	replyChan := wq.nextReplyChan

	// Figure out if there's someone waiting for their turn to send, and
	// alert them to wake up.
	var nextReplyChan chan interface{}
	var nextTurnChan *list.Element
	if wq.waitingToSend != nil {
		nextTurnChan = wq.waitingToSend.Front()
	}
	if nextTurnChan == nil {
		// No one else waiting to send.
		wq.nextReplyChan = nil
	} else {
		// Someone waiting to send.
		nextReplyChan = make(chan interface{})
		wq.nextReplyChan = nextReplyChan
		wq.waitingToSend.Remove(nextTurnChan)
	}
	wq.Unlock()

	if nextTurnChan != nil {
		// Alert the caller to wake up.
		go func() {
			select {
			case nextTurnChan.Value.(chan chan interface{}) <- nextReplyChan:
			case <-ctx.Done():
			}
		}()
	}

	if replyChan != nil {
		go func() {
			select {
			case replyChan <- v:
			case <-ctx.Done():
			}
		}()
		return nil
	}

	return errReplyBeforeRequest
}
