package lowlevel

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	genericlist "github.com/bahlo/generic-list-go"
	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/companyzero/bisonrelay/client/internal/multipriq"
	"github.com/companyzero/bisonrelay/client/timestats"
	"github.com/companyzero/bisonrelay/rpc"
	"github.com/decred/slog"
	"golang.org/x/sync/errgroup"
)

// rmmsg is the internal structure used to keep track of an outbound RM.
type rmmsg struct {
	orm       OutboundRM
	replyChan chan error
	rv        RVID
	encrypted []byte

	mtx      sync.Mutex
	paidHash []byte
}

func (r *rmmsg) sendReply(err error) {
	if r.replyChan == nil {
		return
	}
	r.replyChan <- err
}

// rmmsgReply is the server reply to sending an individual RM. This is sent
// from individual sending goroutines back to sendLoop.
type rmmsgReply struct {
	rmm         *rmmsg
	err         error
	nextInvoice string
}

type RMQDB interface {
	// StoreRVPaymentAttempt should store that an attempt to pay to push
	// to the given RV is being made with the given invoice.
	StoreRVPaymentAttempt(RVID, string, time.Time) error

	// RVHasPaymentAttempt should return the invoice and time that an
	// attempt to pay to push to the RV was made (i.e. it returns the
	// invoice and time saved on a prior call to StoreRVPaymentAttempt).
	RVHasPaymentAttempt(RVID) (string, time.Time, error)

	// DeleteRVPaymentAttempt removes the prior attempt to pay for the given
	// RV.
	DeleteRVPaymentAttempt(RVID) error
}

// RMQ is a queue for sending RoutedMessages (RMs) to the server. The rmq
// supports a flickering server connection: any unsent RMs are queued (FIFO
// style) until a new server session is bound via `bindToSession`.
//
// Sending an RM only fails when the rmq is shutting down or the rm failed to
// encrypt itself.
type RMQ struct {
	maxMsgSize atomic.Uint32

	// The following fields should only be set during setup this struct and
	// are not safe for concurrent modification.

	sessionChan    chan clientintf.ServerSessionIntf
	log            slog.Logger
	rmChan         chan *rmmsg
	enqueueDone    chan struct{}
	enqueueLenChan chan chan int
	sendLenChan    chan chan int
	timingStat     timestats.Tracker
	db             RMQDB

	nextSendChan chan *rmmsg
	sendDoneChan chan struct{}
}

func NewRMQ(log slog.Logger, db RMQDB) *RMQ {
	if log == nil {
		log = slog.Disabled
	}
	q := &RMQ{
		sessionChan:    make(chan clientintf.ServerSessionIntf),
		log:            log,
		db:             db,
		rmChan:         make(chan *rmmsg),
		enqueueDone:    make(chan struct{}),
		enqueueLenChan: make(chan chan int),
		sendLenChan:    make(chan chan int),
		nextSendChan:   make(chan *rmmsg),
		sendDoneChan:   make(chan struct{}),
		timingStat:     *timestats.NewTracker(250),
	}
	q.maxMsgSize.Store(uint32(rpc.MaxMsgSizeForVersion(rpc.MaxMsgSizeV0)))
	return q
}

// MaxMsgSize returns the current max message size of the RMQ.
func (q *RMQ) MaxMsgSize() uint32 {
	return q.maxMsgSize.Load()
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
	maxMsgSize := q.maxMsgSize.Load()
	if encLen > maxMsgSize {
		return fmt.Errorf("%d > %d: %w", encLen, maxMsgSize, errORMTooLarge)
	}

	rmm := &rmmsg{
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

// TimingStats returns the latest timing stats for the RMQ.
func (q *RMQ) TimingStats() []timestats.Quantile {
	return q.timingStat.Quantiles()
}

// processRMAck processes the given ack'd reply from a previously sent rm rpc
// message. It returns a new server invoice, if the reply indicates success
// and there is a new invoice in it.
func (q *RMQ) processRMAck(reply interface{}) (string, error) {
	q.log.Tracef("Processing RMAck reply %T", reply)

	var err error
	var nextInvoice string
	switch reply := reply.(type) {
	case rpc.RouteMessageReply:
		if reply.Error != "" {
			if reply.Error == rpc.ErrRMInvoicePayment.Error() {
				err = rpc.ErrRMInvoicePayment
			} else {
				err = routeMessageReplyError{errorStr: reply.Error}
			}
		}
		if reply.NextInvoice != "" {
			nextInvoice = reply.NextInvoice
		}
	case *rpc.RouteMessageReply:
		if reply.Error != "" {
			if reply.Error == rpc.ErrRMInvoicePayment.Error() {
				err = rpc.ErrRMInvoicePayment
			} else {
				err = routeMessageReplyError{errorStr: reply.Error}
			}
		}
		if reply.NextInvoice != "" {
			nextInvoice = reply.NextInvoice
		}
	case error:
		err = reply
	default:
		err = fmt.Errorf("unknown reply of RMAck: %v", reply)
	}

	return nextInvoice, err
}

// fetchInvoice requests and returns an invoice for the server to pay for
// pushing an RM.
func (q *RMQ) fetchInvoice(ctx context.Context, sess clientintf.ServerSessionIntf) (string, error) {
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
		return "", err
	}

	// Wait to get the invoice back.
	var reply interface{}
	select {
	case reply = <-replyChan:
	case <-ctx.Done():
		return "", ctx.Err()
	}

	switch reply := reply.(type) {
	case *rpc.GetInvoiceReply:
		q.log.Tracef("Received invoice reply: %q", reply.Invoice)

		// Decode invoice and sanity check it.
		decoded, err := pc.DecodeInvoice(ctx, reply.Invoice)
		if err != nil {
			return "", fmt.Errorf("unable to decode received invoice: %v", err)
		}
		if decoded.IsExpired(rpc.InvoiceExpiryAffordance) {
			return "", fmt.Errorf("server sent expired invoice")
		}
		if decoded.MAtoms != 0 {
			return "", fmt.Errorf("server sent invoice with amount instead of zero")
		}

		return reply.Invoice, nil
	case error:
		return "", reply
	default:
		return "", fmt.Errorf("unknown reply from server: %v", err)
	}
}

// isRVInvoicePaid checks whether a previous payment attempt for the given RV
// and amount exists, and if it does, if the payment is still valid to be used
// as a payment token to the server.
//
// Returns the payment token, or nil if a new payment attempt is needed.
func (q *RMQ) isRVInvoicePaid(ctx context.Context, rv RVID, amt int64, pc clientintf.PaymentClient,
	sess clientintf.ServerSessionIntf) (int64, []byte) {

	// Check if there was a previous payment attempt for this rv.
	payInvoice, payDate, err := q.db.RVHasPaymentAttempt(rv)
	if err != nil || payInvoice == "" {
		q.log.Debugf("Push to RV %s has no stored payment attempt", rv)
		return 0, nil
	}

	// Check that the payment attempt isn't so old that the payment token
	// is no longer usable.
	ppLifetimeDuration := sess.Policy().PushPaymentLifetime
	payLifetimeLimit := time.Now().Add(-ppLifetimeDuration)
	if payDate.Before(payLifetimeLimit) {
		q.log.Warnf("Push payment attempt stored timed out: invoice %q "+
			"attempt time %s limit time %s amount %d milliatoms",
			payInvoice, payDate, payLifetimeLimit, amt)
		return 0, nil
	}

	// Create a special context to limit how long we wait for inflight
	// payments to complete. This handles the case where a payment takes
	// too long to complete and we might want to unblock the queue.
	//
	// NOTE: The payment might still be inflight, but it won't be reused in
	// that case, potentially causing a double payment for the same RV.
	//
	// The tradeoff is having to hang on to the payment attempt for
	// potentially a _long_ time (in the worse case scenario, an on-chain
	// settlement, hundreds of blocks in the future) and never being able
	// to send this message in a timely manner, causing broken ratchets.
	//
	// We assume this risk of double payment here, for the moment. In the
	// future, this should be exposed to the user somehow.
	const paymentTimeout = time.Minute
	ctx, cancel := context.WithTimeout(ctx, paymentTimeout)
	defer cancel()

	// Check invoice payment actually completed.
	fees, err := pc.IsPaymentCompleted(ctx, payInvoice)
	if err != nil {
		q.log.Warnf("Push payment attempt stored failed IsPaymentCompleted"+
			"check: %v", err)
		return 0, nil
	}

	// Paid for this RV and payment still valid. Extract payment id to
	// reuse it.
	decoded, err := pc.DecodeInvoice(ctx, payInvoice)
	if err != nil {
		q.log.Warnf("Push payment attempt stored invoice %s failed "+
			"to decode: %v", payInvoice, err)
		return 0, nil
	}

	q.log.Debugf("Reusing payment id for push to RV %s: %x", rv, decoded.ID)
	return fees, decoded.ID
}

// payForRM pays for the given rm on the server.
func (q *RMQ) payForRM(ctx context.Context, rmm *rmmsg, invoice string,
	sess clientintf.ServerSessionIntf) error {

	// Determine payment amount.
	pc := sess.PayClient()
	payloadSize := rmm.orm.EncryptedLen()
	pushPayRate := sess.Policy().PushPayRate
	amt := int64(payloadSize) * int64(pushPayRate)

	// Enforce the minimum payment policy.
	if amt < int64(rpc.MinRMPushPayment) {
		amt = int64(rpc.MinRMPushPayment)
	}

	// Check for a successful previous payment attempt.
	// TODO: track fees? What about duplicate checks?
	_, paidHash := q.isRVInvoicePaid(ctx, rmm.rv, amt, pc, sess)
	if paidHash != nil {
		rmm.mtx.Lock()
		rmm.paidHash = paidHash
		rmm.mtx.Unlock()
		return nil
	}

	// Fetch invoice if needed.
	var err error
	var decoded clientintf.DecodedInvoice
	needsInvoice := false
	if invoice == "" {
		needsInvoice = true
	} else {
		// Decode invoice, check if it's expired.
		decoded, err = pc.DecodeInvoice(ctx, invoice)
		if err != nil {
			needsInvoice = true
		} else if decoded.IsExpired(rpc.InvoiceExpiryAffordance) {
			needsInvoice = true
		}
	}

	if needsInvoice {
		invoice, err = q.fetchInvoice(ctx, sess)
		if err != nil {
			return err
		}
		if decoded, err = pc.DecodeInvoice(ctx, invoice); err != nil {
			return err
		}
	}

	// Save that there's a payment attempt outbound so that on restart
	// we double check if the payment completed.
	if err := q.db.StoreRVPaymentAttempt(rmm.rv, invoice, time.Now()); err != nil {
		return err
	}

	// Pay for it.
	q.log.Tracef("Attempting to pay %d MAtoms to push RM %s", amt, rmm.orm)
	ctx, cancel := multiCtx(ctx, sess.Context())
	fees, err := pc.PayInvoiceAmount(ctx, invoice, amt)
	cancel()
	if err == nil {
		q.log.Tracef("Payment to push RM %s to RV %s completed "+
			"successfully with ID %x", rmm.orm, rmm.rv, decoded.ID)
		rmm.mtx.Lock()
		rmm.paidHash = decoded.ID
		rmm.mtx.Unlock()
		rmm.orm.PaidForRM(amt, fees)
	}
	return err
}

// sendToSession sends the given rm to the given session. It sends the result
// of the send attempt in replyChan.
//
// Errors which are implied to cause a reconnection and resend attempt or are
// terminating (context.Canceled, ErrSubsysExiting, etc) don't cause a reply
// to be sent, as the sendLoop will attempt to send again.
func (q *RMQ) sendToSession(ctx context.Context, rmm *rmmsg, sess clientintf.ServerSessionIntf,
	invoice string, replyChan chan rmmsgReply) {

	// Pay for the RM.
	if err := q.payForRM(ctx, rmm, invoice, sess); err != nil {
		q.log.Debugf("Unable to pay for RM %s: %v", rmm.orm, err)

		// Request connection close so that we reconnect and try to
		// pay again.
		sess.RequestClose(err)
		return
	}

	msg := rpc.Message{Command: rpc.TaggedCmdRouteMessage}
	payload := &rpc.RouteMessage{
		PaidInvoiceID: rmm.paidHash,
		Rendezvous:    rmm.rv,
		Message:       rmm.encrypted,
	}

	// Send it!
	ackChan := make(chan interface{})
	err := sess.SendPRPC(msg, payload, ackChan)
	sendTime := time.Now()
	if err != nil {
		// Connection will be dropped, try again with next connection.
		q.log.Debugf("Error sending rm %s at RV %s: %v", rmm.orm, rmm.rv, err)
		return
	}

	q.log.Debugf("Success sending rm %s at RV %s", rmm.orm, rmm.rv)

	// Wait for server ack.
	var ackReply interface{}
	select {
	case ackReply = <-ackChan:
	case <-ctx.Done():
		// RMQ is quitting.
		return
	}

	// Ack received from server. Process it.
	nextInvoice, err := q.processRMAck(ackReply)

	// Ignore ErrSubsysExiting. This error happens when (a) the session was
	// closed or (b) the user is quitting the client.  Either way, the
	// sendloop will attempt to send again once the next connection is
	// available (or after the client restarts).
	if errors.Is(err, clientintf.ErrSubsysExiting) {
		return
	}

	// When we receive back an invoice payment error, clear the invoice
	// used for payment and try again with a fresh invoice.
	if errors.Is(err, rpc.ErrRMInvoicePayment) {
		rmm.mtx.Lock()
		oldHash := rmm.paidHash
		rmm.paidHash = nil
		rmm.mtx.Unlock()

		q.log.Warnf("Received ErrRMInvoicePayment when attempting to "+
			"push to RV %s with old payment hash %x. Attempting again "+
			"with new invoice.", rmm.rv, oldHash)

		if err := q.db.DeleteRVPaymentAttempt(rmm.rv); err != nil {
			q.log.Warnf("Unable to delete payment to push RV %s: %v",
				rmm.rv, err)
		}

		q.sendToSession(ctx, rmm, sess, "", replyChan)
		return
	}

	// Track how long it took to get the ack.
	q.timingStat.Add(time.Since(sendTime))

	// Today, an ack error is some processing error on the server side. So
	// disconnect and hope sending through the next connection works.
	//
	// FIXME: This is known to fail in some circumstances, such as a
	// message larger than the server accepts causing a reconnection loop.
	// The server needs to return a proper error in that situation.
	if err != nil {
		sess.RequestClose(fmt.Errorf("RM push ack error: %v", err))
		return
	}

	// At this point, err == nil (RM was sent and acknowledged by server).

	// Send reply to original caller.
	go rmm.sendReply(err)

	// Mark payment as used.
	if err := q.db.DeleteRVPaymentAttempt(rmm.rv); err != nil {
		q.log.Warnf("Unable to delete payment to push RV %s: %v",
			rmm.rv, err)
	}

	// Reply sendLoop that the ack for this was received.
	select {
	case replyChan <- rmmsgReply{rmm: rmm, err: err, nextInvoice: nextInvoice}:
	case <-ctx.Done():
	}
}

// Len returns the current number of outstanding messages in the RMQs enqueue
// loop and send loop.
func (q *RMQ) Len() (int, int) {
	// Send the request for len.
	cq, cs := make(chan int, 1), make(chan int, 1)
	select {
	case q.enqueueLenChan <- cq:
	case <-q.enqueueDone:
		return 0, 0
	}
	select {
	case q.sendLenChan <- cs:
	case <-q.enqueueDone:
		return 0, 0
	}

	// Read the replies.
	var lq, ls int
	select {
	case lq = <-cq:
	case <-q.enqueueDone:
		return 0, 0
	}
	select {
	case ls = <-cs:
	case <-q.enqueueDone:
		return 0, 0
	}

	return lq, ls
}

// enqueueLoop is responsible for maintaining the prioritized outbound queue of
// routed messages. It attempts to build the queue as fast as possible for
// proper priorization of messages.
//
// It sends individual RMs to sendLoop whenever it needs an additional work item.
func (q *RMQ) enqueueLoop(ctx context.Context) error {
	// outq tracks messages that need to be sent out.
	outq := new(multipriq.MultiPriorityQueue)

	emptyQueue := func() bool {
		return outq.Len() == 0
	}

	// nextRMM to send (last dequeued value).
	var nextRMM *rmmsg

	// sendChan is set to either q.nextSendChan (when we have items to send)
	// or nil (when we have no items to send).
	var sendChan chan *rmmsg

	// enqueue pushes an rmm into the outq priority queue.
	enqueue := func(rmm *rmmsg) {
		pri := rmm.orm.Priority()
		q.log.Tracef("Queueing rm %s with priority %d", rmm.orm, pri)
		outq.Push(rmm, pri)
	}

	// dequeue pops from outq.
	dequeue := func() *rmmsg {
		e := outq.Pop()
		return e.(*rmmsg)
	}

	// The strategy for the enqueueLoop is to read as fast as possible from
	// rmChan and add items to outq for proper priorization. Whenever there
	// are items in outq, we fill sendChan and nextRMM with the next item
	// that needs to go to the send loop. We pop from outq as soon as the
	// send loop receives nextRMM.

loop:
	for {
		select {
		case rmm := <-q.rmChan:
			enqueue(rmm)
			if nextRMM == nil {
				sendChan = q.nextSendChan
				nextRMM = dequeue()
			}

		case c := <-q.enqueueLenChan:
			l := outq.Len()
			if nextRMM != nil {
				l += 1
			}
			c <- l

		case sendChan <- nextRMM:
			if emptyQueue() {
				sendChan = nil
				nextRMM = nil
			} else {
				nextRMM = dequeue()
			}

		case <-ctx.Done():
			break loop
		}
	}

	// Alert all outstanding elements in queue that we're exiting.
	close(q.enqueueDone)
	if nextRMM != nil {
		go nextRMM.sendReply(errRMQExiting)
	}
	for !emptyQueue() {
		rmm := dequeue()
		go rmm.sendReply(errRMQExiting)
	}

	return ctx.Err()
}

// sendLoop attempts to send individual RMs to the server and waits until
// they are acked before attempting to send the next one. It receives items
// from enqueueLoop whenever needed.
//
// The sendLoop keeps track of as many messages as the server will accept
// concurrently.
func (q *RMQ) sendLoop(ctx context.Context) error {

	// clientMaxPendingRMMs is the max number of pending RMMs the client
	// will enforce independently of the server provided setting.
	const clientMaxPendingRMMs = 256

	// maxPendingRMMs is the max number of pending RMMs/invoices on the
	// server.
	var maxPendingRMMs int

	// sess is the current server session to send RMs to.
	var sess clientintf.ServerSessionIntf

	// replyChan is the channel where ack replies are written.
	replyChan := make(chan rmmsgReply)

	// rmm tracks the current RM we're attempting to send.
	rmms := make(map[*rmmsg]struct{}, 0)

	// sendChan is set to q.nextSendChan when we have less pending RMs
	// than maxPendingRMMs and set to nil when we need to wait until an
	// RM is ack'd before we can send another.
	var sendChan chan *rmmsg

	// invoices is the list of available invoices returned by a push message
	// and that can be used to pay for the next one.
	invoices := &genericlist.List[string]{}

loop:
	for {
		select {
		case sess = <-q.sessionChan:
			q.log.Debugf("Using new server session %v", sess)
			if sess == nil {
				// Lost the server connection, so stop fetching
				// new items to send.
				invoices = &genericlist.List[string]{}
				sendChan = nil
				continue loop
			}

			maxMsgSize := uint32(sess.Policy().MaxMsgSize)
			if q.maxMsgSize.Swap(maxMsgSize) != maxMsgSize {
				q.log.Infof("Max message size changed to %d bytes",
					maxMsgSize)
			}

			// We received a new session, resend outstanding items.
			// The list of available invoices is empty because we
			// just reconnected to the server, therefore will
			// require new invoices.
			q.log.Tracef("Starting to resend %d messages to server in sendloop",
				len(rmms))
			for rmm := range rmms {
				rmm := rmm
				go q.sendToSession(ctx, rmm, sess, "", replyChan)
			}

			// Figure out the max number of outstanding RMs we'll
			// use.
			newMaxPendingRMMs := sess.Policy().MaxPushInvoices
			if newMaxPendingRMMs > clientMaxPendingRMMs {
				newMaxPendingRMMs = clientMaxPendingRMMs
			}
			if newMaxPendingRMMs < maxPendingRMMs {
				// We currently don't correctly handle the case
				// where the max number of pending RMs is
				// reduced across server reconnections, so warn
				// and hope the server will accept our already
				// inflight messages.
				//
				// In the future, this case could be handled by
				// returning the rmm to the enqueue loop and
				// prioritizing it over other items.
				q.log.Errorf("Server operator reduced max number "+
					"of RMMs from %d to %d", maxPendingRMMs, newMaxPendingRMMs)
			}
			maxPendingRMMs = newMaxPendingRMMs
			if len(rmms) < maxPendingRMMs {
				// Start accepting new items to send.
				q.log.Tracef("Starting to accept new messages in sendloop")
				sendChan = q.nextSendChan
			}

		case rmm := <-sendChan:
			// Prepare the msg. This is done synchronously so that
			// RMs sent to the same user are sent in ratchet
			// sendcount order.
			if rmm.encrypted == nil {
				var err error
				rmm.rv, rmm.encrypted, err = rmm.orm.EncryptedMsg()
				if err != nil {
					q.log.Debugf("Error encrypting RM %s: %v",
						rmm.orm, err)
					// This is a fatal error for this RM.
					// We cannot send it anymore, so inform
					// original caller of the error.
					go rmm.sendReply(err)
					continue loop
				}
				q.log.Tracef("Generated encrypted %T with %d bytes at RV %s", rmm.orm,
					len(rmm.encrypted), rmm.rv)
			}

			// New item to send.
			rmms[rmm] = struct{}{}
			if len(rmms) >= maxPendingRMMs {
				// Stop accepting new items to send while we
				// have too many pending for confirmation.
				sendChan = nil
				q.log.Tracef("Pausing acceptance of new messages "+
					"in sendloop due to %d >= %d",
					len(rmms), maxPendingRMMs)
			}

			// Use an available invoice if we have one.
			var invoice string
			if invoices.Len() > 0 {
				e := invoices.Front()
				invoice = e.Value
				invoices.Remove(e)
			}

			// Attempt send.
			go q.sendToSession(ctx, rmm, sess, invoice, replyChan)

		case reply := <-replyChan:
			// Whatever we receive as reply, we consider this RM
			// as sent.
			delete(rmms, reply.rmm)

			if len(rmms) < maxPendingRMMs && sess != nil && sendChan == nil {
				// We just sent an item and still have a
				// session bound, so we can accept more items
				// to send.
				sendChan = q.nextSendChan
				q.log.Tracef("Restarting acceptance of new messages "+
					"in sendloop due to %d < %d", len(rmms),
					maxPendingRMMs)
			}

			// Add the invoice in the reply to the list of available
			// invoices.
			if reply.nextInvoice != "" {
				invoices.PushBack(reply.nextInvoice)
			}

		case c := <-q.sendLenChan:
			c <- len(rmms)

		case <-ctx.Done():
			break loop
		}
	}

	// Send reply to the outstanding queue items that we're quitting.
	for rmm := range rmms {
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
