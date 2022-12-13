package lowlevel

import (
	"context"
	"errors"
	"fmt"

	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/companyzero/bisonrelay/client/internal/multipriq"
	"github.com/companyzero/bisonrelay/rpc"
	"github.com/companyzero/bisonrelay/zkidentity"
	"github.com/davecgh/go-spew/spew"
	"github.com/decred/slog"
	"golang.org/x/sync/errgroup"
)

type rmmsg struct {
	orm       OutboundRM
	replyChan chan error
	rv        RVID
	encrypted []byte
}

func (r rmmsg) sendReply(err error) {
	if r.replyChan == nil {
		return
	}
	r.replyChan <- err
}

// RMQ is a queue for sending RoutedMessages (RMs) to the server. The rmq
// supports a flickering server connection: any unsent RMs are queued (FIFO
// style) until a new server session is bound via `bindToSession`.
//
// Sending an RM only fails when the rmq is shutting down or the rm failed to
// encrypt itself.
type RMQ struct {
	// The following fields should only be set during setup this struct and
	// are not safe for concurrent modification.

	sessionChan chan clientintf.ServerSessionIntf
	localID     *zkidentity.FullIdentity
	log         slog.Logger
	rmChan      chan rmmsg
	enqueueDone chan struct{}
	lenChan     chan chan int

	nextSendChan chan *rmmsg
	sendDoneChan chan struct{}

	// nextInvoice tracks the next invoice that needs to be paid to send the
	// next RM. It doesn't currently need a mutex to protect it because it's
	// only accessed inside sendLoop().
	nextInvoice string
}

func NewRMQ(log slog.Logger, payClient clientintf.PaymentClient, localID *zkidentity.FullIdentity) *RMQ {
	if log == nil {
		log = slog.Disabled
	}
	return &RMQ{
		sessionChan:  make(chan clientintf.ServerSessionIntf),
		localID:      localID,
		log:          log,
		rmChan:       make(chan rmmsg),
		enqueueDone:  make(chan struct{}),
		lenChan:      make(chan chan int),
		nextSendChan: make(chan *rmmsg),
		sendDoneChan: make(chan struct{}),
	}
}

// BindToSession binds the rmq to the specified server session. Queued and new
// messages will be sent via this server until it is removed or the rmq stops.
func (q *RMQ) BindToSession(sess clientintf.ServerSessionIntf) {
	select {
	case q.sessionChan <- sess:
	case <-q.enqueueDone:
	}
}

// QueueRM enqueues the given RM to be sent to the server as soon as possible.
// Returns when the rm has been queued to be sent.
//
// replyChan is written to when the RM has been received by server (which is
// determined when the RMQ receives the corresponding server ack) or if the rmq
// is stopping.
func (q *RMQ) QueueRM(orm OutboundRM, replyChan chan error) error {
	encLen := orm.EncryptedLen()
	if encLen > rpc.MaxMsgSize {
		return fmt.Errorf("%d > %d: %w", encLen, rpc.MaxMsgSize, errORMTooLarge)
	}

	rmm := rmmsg{
		orm:       orm,
		replyChan: replyChan,
	}
	select { // Enqueue.
	case q.rmChan <- rmm:
	case <-q.enqueueDone:
		return errRMQExiting
	}

	return nil
}

// SendRM sends the given routed message to the server whenever possible. It
// returns when the RM has been successfully written and acknowledged as received
// by the server.
func (q *RMQ) SendRM(orm OutboundRM) error {
	replyChan := make(chan error)
	if err := q.QueueRM(orm, replyChan); err != nil {
		return err
	}

	return <-replyChan
}

// processRMAck processes the given ack'd reply from a previously sent rm rpc
// message.
func (q *RMQ) processRMAck(reply interface{}) error {
	q.log.Tracef("Processing RMAck reply %T", reply)

	var err error
	switch reply := reply.(type) {
	case rpc.RouteMessageReply:
		if reply.Error != "" {
			err = routeMessageReplyError{errorStr: reply.Error}
		}
		if reply.NextInvoice != "" {
			q.nextInvoice = reply.NextInvoice
		}
	case *rpc.RouteMessageReply:
		if reply.Error != "" {
			err = routeMessageReplyError{errorStr: reply.Error}
		}
		if reply.NextInvoice != "" {
			q.nextInvoice = reply.NextInvoice
		}
	case error:
		err = reply
	default:
		err = fmt.Errorf("unknown reply of RMAck: %v", reply)
	}

	return err
}

func (q *RMQ) fetchNextInvoice(ctx context.Context, sess clientintf.ServerSessionIntf) error {
	msg := rpc.Message{Command: rpc.TaggedCmdGetInvoice}
	pc := sess.PayClient()
	payload := &rpc.GetInvoice{
		PaymentScheme: pc.PayScheme(),
		Action:        rpc.InvoiceActionPush,
	}

	q.log.Debugf("Requesting %s invoice for next RM", payload.PaymentScheme)

	replyChan := make(chan interface{})
	err := sess.SendPRPC(msg, payload, replyChan)
	if err != nil {
		return err
	}

	// Wait to get the invoice back.
	var reply interface{}
	select {
	case reply = <-replyChan:
	case <-ctx.Done():
		return ctx.Err()
	}

	switch reply := reply.(type) {
	case *rpc.GetInvoiceReply:
		q.nextInvoice = reply.Invoice
		q.log.Tracef("Received invoice reply: %q", q.nextInvoice)

		// Decode invoice and sanity check it.
		decoded, err := pc.DecodeInvoice(ctx, q.nextInvoice)
		if err != nil {
			return fmt.Errorf("unable to decode received invoice: %v", err)
		}
		if decoded.IsExpired(rpc.InvoiceExpiryAffordance) {
			return fmt.Errorf("server sent expired invoice")
		}
		if decoded.MAtoms != 0 {
			return fmt.Errorf("server sent invoice with amount instead of zero")
		}

		return nil
	case error:
		return reply
	default:
		return fmt.Errorf("unknown reply from server: %v", err)
	}
}

// payForRM pays for the given rm on the server.
func (q *RMQ) payForRM(ctx context.Context, orm OutboundRM, sess clientintf.ServerSessionIntf) error {

	// Fetch invoice if needed.
	pc := sess.PayClient()
	needsInvoice := false
	if q.nextInvoice == "" {
		needsInvoice = true
	} else {
		// Decode invoice, check if it's expired.
		decoded, err := pc.DecodeInvoice(ctx, q.nextInvoice)
		if err != nil {
			needsInvoice = true
		} else if decoded.IsExpired(rpc.InvoiceExpiryAffordance) {
			needsInvoice = true
		}
	}

	if needsInvoice {
		err := q.fetchNextInvoice(ctx, sess)
		if err != nil {
			return err
		}
	}

	// Determine payment amount.
	payloadSize := orm.EncryptedLen()
	pushPayRate, _ := sess.PaymentRates()
	amt := payloadSize * uint32(pushPayRate)

	// Enforce the minimum payment policy.
	if amt < uint32(rpc.MinRMPushPayment) {
		amt = uint32(rpc.MinRMPushPayment)
	}

	// Pay for it. Independently of payment result, clear the invoice to pay.
	q.log.Tracef("Attempting to pay %d MAtoms for next RM %s", amt, orm)
	ctx, cancel := multiCtx(ctx, sess.Context())
	fees, err := pc.PayInvoiceAmount(ctx, q.nextInvoice, int64(amt))
	cancel()
	q.nextInvoice = ""
	orm.PaidForRM(int64(amt), fees)
	return err
}

// sendToSession sends the given rm to the given session. It can return either
// a client error (when the rm itself errored in some step) or a server error
// (for example, when failing to write the rm msg in the wire).
func (q *RMQ) sendToSession(ctx context.Context, rmm *rmmsg, sess clientintf.ServerSessionIntf,
	replyChan chan interface{}) (error, error) {

	// Pay for the next RM.
	if err := q.payForRM(ctx, rmm.orm, sess); err != nil {
		return nil, err
	}

	// Prepare the msg.
	var err error
	if rmm.encrypted == nil {
		rmm.rv, rmm.encrypted, err = rmm.orm.EncryptedMsg()
		if err != nil {
			return err, nil
		}
		q.log.Tracef("Generated encrypted %T with %d bytes at RV %s", rmm.orm,
			len(rmm.encrypted), rmm.rv)
	}

	msg := rpc.Message{Command: rpc.TaggedCmdRouteMessage}
	payload := &rpc.RouteMessage{
		Rendezvous: rmm.rv,
		Message:    rmm.encrypted,
	}

	// Send it!
	err = sess.SendPRPC(msg, payload, replyChan)
	if err != nil {
		q.log.Debugf("Error sending rm %s at RV %s: %v", rmm.orm, rmm.rv, err)
		return nil, err
	}

	q.log.Debugf("Success sending rm %s at RV %s", rmm.orm, rmm.rv)
	return nil, nil
}

// Len returns the current number of outstanding messages in the RMQ.
func (q *RMQ) Len() int {
	c := make(chan int)
	select {
	case q.lenChan <- c:
	case <-q.enqueueDone:
		return 0
	}

	select {
	case l := <-c:
		return l
	case <-q.enqueueDone:
		return 0
	}
}

// enqueueLoop is responsible for maintaining the prioritized outbound queue of
// routed messages. It attempts to build the queue as fast as possible for
// proper priorization of messages.
//
// It sends individual RMs to sendLoop whenever it needs an additional work item.
func (q *RMQ) enqueueLoop(ctx context.Context) error {
	// outq tracks messages that need to be sent out.
	outq := new(multipriq.MultiPriorityQueue)

	// waitingSendDone is set to q.sendDoneChan whenever we send a message
	// to sendLoop and are expecting the send to complete before dequeuing
	// and sending the next message.
	var waitingSendDone chan struct{}

	enqueue := func(rmm *rmmsg) {
		pri := rmm.orm.Priority()
		q.log.Tracef("Queueing rm %s with priority %d", rmm.orm, pri)
		outq.Push(rmm, pri)
	}

	// dequeue pops from outq and sends the popped value to sendLoop.
	dequeue := func() {
		e := outq.Pop()
		rmm := e.(*rmmsg)
		waitingSendDone = q.sendDoneChan
		go func() {
			select {
			case q.nextSendChan <- rmm:
			case <-ctx.Done():
				go rmm.sendReply(errRMQExiting)
			}
		}()
	}

	// The strategy for the enqueueLoop is to read as fast as possible from
	// rmChan and add items to outq for proper priorization. Whenever we
	// finish a send, we pop a value from the outq and send it to sendLoop,
	// which will make several attempts at sending the RM.
loop:
	for {
		select {
		case rmm := <-q.rmChan:
			enqueue(&rmm)
			if waitingSendDone == nil {
				// Queue was empty. Send to sendLoop directly.
				dequeue()
			}

		case <-waitingSendDone:
			// Send completed.
			if outq.Len() > 0 {
				// Send next.
				dequeue()
			} else {
				// No more items to send. We don't expect a
				// write to this channel anymore.
				waitingSendDone = nil
			}

		case c := <-q.lenChan:
			c <- outq.Len()

		case <-ctx.Done():
			break loop
		}
	}

	// Alert all outstanding elements in queue that we're exiting.
	close(q.enqueueDone)
	for outq.Len() > 0 {
		e := outq.Pop()
		rmm := e.(*rmmsg)
		go rmm.sendReply(errRMQExiting)
	}

	return ctx.Err()
}

// sendLoop attempts to send individual RMs to the server and waits until
// they are acked before attempting to send the next one. It receives items
// from enqueueLoop whenever needed.
func (q *RMQ) sendLoop(ctx context.Context) error {
	// sess is the current server session to send RMs to.
	var sess clientintf.ServerSessionIntf

	// ackChan is where we receive acks from the server when we send RMs.
	ackChan := make(chan interface{})

	// rmm tracks the current RM we're attempting to send.
	var rmm *rmmsg

	// alsertSendDone alerts enqueueLoop we need a new work item. It must
	// be called as a goroutine.
	alertSendDone := func() {
		select {
		case q.sendDoneChan <- struct{}{}:
		case <-ctx.Done():
		}
	}

loop:
	for {
		// Keep waiting until we have _both_ an RM to send and
		// a session to send it through.
		for rmm == nil || sess == nil {
			select {
			case nextRMM := <-q.nextSendChan:
				if rmm != nil {
					// This _really_ shouldn't happen.
					return fmt.Errorf("logic error: sendLoop received " +
						"nextRMM before last one was sent")
				}
				rmm = nextRMM
				q.log.Tracef("Sendloop received RM %s", rmm.orm)

			case sess = <-q.sessionChan:
				q.log.Debugf("Using new server session %v", sess)

				// Whenever the sesion changes, clear the
				// invoice since it may not be valid with the
				// server anymore.
				q.nextInvoice = ""

			case <-ctx.Done():
				break loop
			}
		}

		// Attempt to send.
		rmErr, svrErr := q.sendToSession(ctx, rmm, sess, ackChan)

		// Server errors are not fatal to the rmq: we'll dissociate
		// from the session and wait for a new one to try again. This
		// maintains the current rmm and clears the session.
		if svrErr != nil {
			if canceled(ctx) {
				break loop
			}
			if q.log.Level() <= slog.LevelTrace {
				q.log.Tracef("Server error sending rm "+
					"%T: %v. Payload: %s", rmm.orm,
					svrErr, spew.Sdump(rmm.orm))
			} else {
				q.log.Debugf("Server error sending "+
					"rm %T: %v", rmm.orm, svrErr)
			}
			sess.RequestClose(svrErr)
			sess = nil
			continue loop
		}

		// If some RM-specific error occurred, we don't expect
		// a reply from the server (because the RM was not in
		// fact sent). Alert enqueue loop to send the next work item.
		if rmErr != nil {
			go rmm.sendReply(rmErr)
			rmm = nil
			go alertSendDone()
			continue loop
		}

		// RM was in fact sent. Wait for an ack from the server.
		var ackReply interface{}
		select {
		case ackReply = <-ackChan:
		case <-ctx.Done():
			break loop
		}

		// Received ack result. Process it and determine what to do.
		ackErr := q.processRMAck(ackReply)
		if errors.Is(ackErr, clientintf.ErrSubsysExiting) {
			// This error happens when (a) the session was closed
			// by the remote end or (b) the user is quitting the
			// client. Either way, we'll wait for a new connection
			// before trying to send again or for our own context
			// to be cancelled.
			q.log.Debugf("Stopped waiting for ack due to %v", ackErr)
			sess = nil
			continue loop
		}

		// Send ack reply to SendRM() so that it returns the result to
		// caller.
		go rmm.sendReply(ackErr)
		if ackErr != nil {
			// Today, an ack error is some processing error on the
			// server side. So disconnect and hope the next
			// connection works.
			q.log.Errorf("Server sent an RM reply error: %v", ackErr)
			if sess != nil {
				sess.RequestClose(ackErr)
			}
			sess = nil
		}

		// Either way, we won't attempt to send this RM again. Alert
		// enqueue loop to send next work item.
		rmm = nil
		go alertSendDone()
	}

	// Send reply to the outstanding queue item that we're quitting.
	if rmm != nil {
		go rmm.sendReply(errRMQExiting)
	}

	return ctx.Err()
}

// Run the services of this rmq. Must only be called once.
func (q *RMQ) Run(ctx context.Context) error {
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error { return q.enqueueLoop(gctx) })
	g.Go(func() error { return q.sendLoop(gctx) })
	return g.Wait()
}
