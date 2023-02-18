package lowlevel

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/companyzero/bisonrelay/ratchet"
	"github.com/companyzero/bisonrelay/rpc"
	"github.com/decred/slog"
)

type RVID = ratchet.RVPoint

// RVBlob is a decoded blob received from the server at a specific RV point.
type RVBlob struct {
	Decoded  []byte
	ID       RVID
	ServerTS time.Time
}

type RVHandler func(blob RVBlob) error

// SubPaidHandler is a callback type for tracking payment for subscribing to an
// RV.
type SubPaidHandler func(amount, fees int64)

type rdzvSub struct {
	id          RVID
	handler     RVHandler
	subPaid     SubPaidHandler
	subDoneChan chan error
}

func (sub rdzvSub) replySubDone(err error, runDone chan struct{}) {
	select {
	case sub.subDoneChan <- err:
	case <-runDone:
	}
}

type rdzvUnsub struct {
	id            RVID
	unsubDoneChan chan error
}

func (unsub rdzvUnsub) replyUnsubDone(err error, runDone chan struct{}) {
	select {
	case unsub.unsubDoneChan <- err:
	case <-runDone:
	}
}

// recvdPRM is used during processing of received pushed messages.
type recvdPRM struct {
	prm       *rpc.PushRoutedMessage
	replyChan chan error
}

// RVManagerDB abstracts the necessary functions that the RV manager needs from
// the DB.
type RVManagerDB interface {
	// UnpaidRVs filters the list of RVs, returning the ones that haven't
	// been paid yet.
	UnpaidRVs(rvs []RVID, expirationDays int) ([]RVID, error)

	// SavePaidRVs saves the specified list of RVs as paid.
	SavePaidRVs(rvs []RVID) error

	// MarkRVUnpaid marks the specified RV as unpaid in the DB.
	MarkRVUnpaid(rv RVID) error
}

// RVManager keeps track of the various rendezvous points that should be
// registered on a remote server and what to do when RoutedMessages are
// received on the registered points.
//
// Values should not be reused once their run() method returns.
type RVManager struct {
	log         slog.Logger
	sessionChan chan clientintf.ServerSessionIntf
	subChan     chan rdzvSub
	unsubChan   chan rdzvUnsub
	handlerChan chan recvdPRM
	runDone     chan struct{}
	isUpToDate  chan chan bool
	db          RVManagerDB
	nextInvoice string
	subDoneCB   func()

	// subsDelayer is used to do some hysteresis around the full
	// subscription set and avoid sending multiple subscription requests to
	// the server in a very short time frame.
	subsDelayer func() <-chan time.Time
}

func NewRVManager(log slog.Logger, db RVManagerDB, subsDelayer func() <-chan time.Time, subDoneCB func()) *RVManager {
	// Default is to use a delayer that never delays (by returning a closed
	// chan it always writes the empty value immediately).
	closedTimeChan := make(chan time.Time)
	close(closedTimeChan)
	if subsDelayer == nil {
		subsDelayer = func() <-chan time.Time {
			return closedTimeChan
		}
	}

	if log == nil {
		log = slog.Disabled
	}

	return &RVManager{
		log:         log,
		db:          db,
		sessionChan: make(chan clientintf.ServerSessionIntf),
		subChan:     make(chan rdzvSub),
		unsubChan:   make(chan rdzvUnsub),
		handlerChan: make(chan recvdPRM),
		runDone:     make(chan struct{}),
		isUpToDate:  make(chan chan bool),
		subsDelayer: subsDelayer,
		subDoneCB:   subDoneCB,
	}
}

// Sub informs the manager to subscribe to the given rendezvous point and to
// call handler once a message is received in the given point.
//
// Note that handler might never be called if the manager is stopped and it
// might be called multiple times if the rendezvous is registered and pushed
// multiple times.
func (rmgr *RVManager) Sub(rdzv RVID, handler RVHandler, subPaid SubPaidHandler) error {
	sub := rdzvSub{
		id:          rdzv,
		handler:     handler,
		subPaid:     subPaid,
		subDoneChan: make(chan error),
	}
	select {
	case rmgr.subChan <- sub:
	case <-rmgr.runDone:
		return errRdvzMgrExiting
	}
	select {
	case err := <-sub.subDoneChan:
		return err
	case <-rmgr.runDone:
		return errRdvzMgrExiting
	}
}

// Unsub unsubscribes from the given rendezvous point.
func (rmgr *RVManager) Unsub(rdzv RVID) error {
	unsub := rdzvUnsub{
		id:            rdzv,
		unsubDoneChan: make(chan error),
	}
	select {
	case rmgr.unsubChan <- unsub:
	case <-rmgr.runDone:
		return errRdvzMgrExiting
	}
	select {
	case err := <-unsub.unsubDoneChan:
		return err
	case <-rmgr.runDone:
		return errRdvzMgrExiting
	}
}

// BindToSession binds the rendezvous manager to the specified server session.
//
// Note: the rendezvous manager assumes the given session has been setup such
// that its `pushedRoutedMsgsHandler` calls the manager's `handlePushedRMs`.
func (rmgr *RVManager) BindToSession(sess clientintf.ServerSessionIntf) {
	select {
	case rmgr.sessionChan <- sess:
	case <-rmgr.runDone:
	}
}

// HandlePushedRMs is called via a bound session's `pushedRoutedMsgsHandler`
// whenever routed messages are pushed from server to client.
func (rmgr *RVManager) HandlePushedRMs(prm *rpc.PushRoutedMessage) error {
	if prm == nil {
		return fmt.Errorf("prm cannot be nil in HandlePushedRMs")
	}

	if prm.Error != "" {
		return AckError{ErrorStr: prm.Error}
	}

	if len(prm.Payload) == 0 {
		rmgr.log.Tracef("Received empty pushed RM")
		return nil
	}

	rprm := recvdPRM{
		prm:       prm,
		replyChan: make(chan error),
	}

	// Schedule it in Run().
	select {
	case rmgr.handlerChan <- rprm:
	case <-rmgr.runDone:
		return errRdvzMgrExiting
	}

	// Get processing reply.
	select {
	case err := <-rprm.replyChan:
		return err
	case <-rmgr.runDone:
		return errRdvzMgrExiting
	}
}

// IsUpToDate returns true if the the manager has sent all updates to the
// remote server and the server has ack'd them.
func (rmgr *RVManager) IsUpToDate() bool {
	c := make(chan bool, 1)
	select {
	case rmgr.isUpToDate <- c:
	case <-rmgr.runDone:
		return false
	}

	select {
	case res := <-c:
		return res
	case <-rmgr.runDone:
		return false
	}
}

func (rmgr *RVManager) fetchNextInvoice(ctx context.Context, sess clientintf.ServerSessionIntf) error {
	msg := rpc.Message{Command: rpc.TaggedCmdGetInvoice}
	pc := sess.PayClient()
	payload := &rpc.GetInvoice{
		PaymentScheme: pc.PayScheme(),
		Action:        rpc.InvoiceActionSub,
	}

	rmgr.log.Debugf("Requesting %s invoice for next subscriptions", payload.PaymentScheme)

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
		rmgr.nextInvoice = reply.Invoice
		rmgr.log.Tracef("Received invoice reply: %q", rmgr.nextInvoice)

		// Decode invoice and sanity check it.
		decoded, err := pc.DecodeInvoice(ctx, rmgr.nextInvoice)
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

// payForSubs pays for any unpaid RVs contained in the passed list. Returns the
// list of (previously) unpaid RVs.
func (rmgr *RVManager) payForSubs(ctx context.Context, rlist []ratchet.RVPoint,
	subs map[RVID]rdzvSub, sess clientintf.ServerSessionIntf) ([]ratchet.RVPoint, error) {

	// Determine payment amount. The amount to pay depends on how many
	// unpaid for RVs we have.
	unpaidRVs, err := rmgr.db.UnpaidRVs(rlist, sess.ExpirationDays())
	if err != nil {
		return nil, err
	}

	// Fetch invoice if needed.
	pc := sess.PayClient()
	needsInvoice := false
	if len(unpaidRVs) == 0 {
		// No need to pay.
		return nil, nil
	} else if rmgr.nextInvoice == "" {
		needsInvoice = true
	} else {
		// Decode invoice, check if it's expired.
		decoded, err := pc.DecodeInvoice(ctx, rmgr.nextInvoice)
		if err != nil {
			needsInvoice = true
		} else if decoded.IsExpired(rpc.InvoiceExpiryAffordance) {
			needsInvoice = true
		} else {
			rmgr.log.Tracef("Paying with existing invoice")
		}
	}

	if needsInvoice {
		err := rmgr.fetchNextInvoice(ctx, sess)
		if err != nil {
			return nil, err
		}
	}

	_, subPayRate := sess.PaymentRates()
	amt := len(unpaidRVs) * int(subPayRate)

	// Pay for it. Independently of payment result, clear the invoice to pay.
	ctx, cancel := multiCtx(ctx, sess.Context())
	rmgr.log.Debugf("Attempting to pay %d MAtoms for new subs %s", amt,
		joinRVList(unpaidRVs))
	totalFees, err := pc.PayInvoiceAmount(ctx, rmgr.nextInvoice, int64(amt))
	cancel()
	rmgr.nextInvoice = ""

	// If the payment completed, track the stats for the previously unpaid
	// subs.
	if err == nil {
		for i, id := range unpaidRVs {
			sub, ok := subs[id]
			if !ok {
				// Should not happen.
				return nil, fmt.Errorf("unpaid RV not in subs map: %s", id)
			}

			if sub.subPaid == nil {
				continue
			}

			subFees := totalFees / int64(len(unpaidRVs))
			if i == 0 {
				// Add rest of fee to the first one.
				subFees += totalFees % int64(len(unpaidRVs))
			}
			sub.subPaid(int64(subPayRate), subFees)
		}
	}

	return unpaidRVs, err
}

// updatePayloadSubscriptions (re-)subscribes to all rendezvous points in subs on
// the given server session.
func (rmgr *RVManager) updatePayloadSubscriptions(ctx context.Context,
	add, del []ratchet.RVPoint, subs map[RVID]rdzvSub, sess clientintf.ServerSessionIntf) error {

	// Pay for the subs we haven't paid yet.
	unpaidRVs, err := rmgr.payForSubs(ctx, add, subs, sess)
	if err != nil {
		return err
	}

	rmgr.log.Debugf("Updating server subscription with +%d-%d RVs", len(add),
		len(del))

	msg := rpc.Message{Command: rpc.TaggedCmdSubscribeRoutedMessages}
	payload := &rpc.SubscribeRoutedMessages{
		AddRendezvous: add,
		DelRendezvous: del,
	}

	replyChan := make(chan interface{})
	err = sess.SendPRPC(msg, payload, replyChan)
	if err != nil {
		return err
	}

	// Wait for the reply.
	var reply interface{}
	select {
	case reply = <-replyChan:
	case <-ctx.Done():
		return ctx.Err()
	}

	// Resolve the subscription reply.
	switch reply := reply.(type) {
	case *rpc.SubscribeRoutedMessagesReply:
		if reply.Error != "" {
			// Handle the "unpaid subscription" error specially,
			// in order to clear the paid flag from the local DB.
			errUnpaid := rpc.ParseErrUnpaidSubscriptionRV(reply.Error)
			if errUnpaid != nil {
				return errUnpaid
			}
			return AckError{ErrorStr: reply.Error}
		}

		// Store invoice for next sub.
		if reply.NextInvoice != "" {
			rmgr.nextInvoice = reply.NextInvoice
		}
	case error:
		return reply
	default:
		return fmt.Errorf("unknown reply from server: %v", err)
	}

	// Mark the unpaid RVs as paid (since server ack'd).
	if sess.PayClient().PayScheme() != rpc.PaySchemeFree {
		if err := rmgr.db.SavePaidRVs(unpaidRVs); err != nil {
			rmgr.log.Warnf("Unable to save paid RVs: %v", err)
		}
	}

	if rmgr.log.Level() <= slog.LevelTrace {
		rmgr.log.Tracef("RV subcriptions changed +%d [%s] -%d [%s]", len(add),
			joinRVList(add), len(del), joinRVList(del))
	} else {
		rmgr.log.Debugf("RV subscriptions changed +%d -%d", len(add), len(del))
	}

	return nil
}

// handleInSubs calls the handler for the given subscription, passing as
// argument the specified pushed RM.
func (rmgr *RVManager) handleInSub(rprm recvdPRM, sub rdzvSub, ok bool) {
	var err error
	if !ok {
		rmgr.log.Warnf("Received pushed RM for unknown rendezvous %q",
			rprm.prm.RV)
	} else if sub.handler != nil {
		var serverTS time.Time
		switch rprm.prm.Timestamp {
		case 0:
			// Fill TS when empty: this handles old server versions
			// from before rpc.PushRoutedMessage.Timestamp existed.
			// This can be removed once the servers are updated.
			serverTS = time.Now()
		default:
			serverTS = time.Unix(rprm.prm.Timestamp, 0)
		}

		blob := RVBlob{
			Decoded:  rprm.prm.Payload,
			ID:       rprm.prm.RV,
			ServerTS: serverTS,
		}

		rmgr.log.Tracef("Received %d bytes at RV %q with ts %s",
			len(blob.Decoded), blob.ID, blob.ServerTS)
		err = sub.handler(blob)
	} else {
		rmgr.log.Warnf("Subscription without handler at RV %q", rprm.prm.RV)
	}

	// Send reply of the processing.
	select {
	case rprm.replyChan <- err:
	case <-rmgr.runDone:
	}
}

// Run runs the rendezvous manager services. A given RVManager instance should
// not be reused once its run method returns.
func (rmgr *RVManager) Run(ctx context.Context) error {

	subs := make(map[RVID]rdzvSub)
	var toAdd, toDel []RVID
	var unsubs, requestedUnsubs []rdzvUnsub
	var sess clientintf.ServerSessionIntf
	var err error
	var needsUpdate bool

	// updateResChan gets the result of the async call to
	// updatePayloadSubscriptions().
	updateResChan := make(chan error)

	// lastUpdateDone tracks whether the last update of the subscriptions
	// to the server was done. This is needed to ensure there's ever only
	// one in-flight request.
	lastUpdateDone := true

	// lastUpdateSuccess tracks whether the last update attempt was
	// successful. We assume an empty manager is updated.
	lastUpdateSuccess := true

	// delayChan keeps track of whether we need to delay to send a new set
	// of subscriptions. This helps avoid sending multiple subscriptions
	// when a bunch of changes happen in the manager in a short time frame.
	var delayChan <-chan time.Time

loop:
	for err == nil {
		select {
		case <-delayChan:
			delayChan = nil

		case newSess := <-rmgr.sessionChan:
			rmgr.log.Debugf("Using new server session %v", newSess)
			sess = newSess
			rmgr.nextInvoice = ""

			// We're about to send the full set, so clear
			// out delayChan to avoid duplicate
			// registration attempts.
			delayChan = nil
			lastUpdateDone = true

			// Re-send all unsubscriptions and all additions.
			unsubs = append(unsubs, requestedUnsubs...)
			requestedUnsubs = nil
			for _, unsub := range requestedUnsubs {
				toDel = append(toDel, unsub.id)
			}
			toAdd = rvMapKeys(subs)

		case sub := <-rmgr.subChan:
			if _, ok := subs[sub.id]; ok {
				// Already have this sub. Fail the subscribe
				// call.
				subErr := makeErrRVAlreadySubscribed(sub.id.String())
				go sub.replySubDone(subErr, rmgr.runDone)
				continue loop
			}

			rmgr.log.Tracef("New subscription for RV %s", sub.id)

			toAdd = append(toAdd, sub.id)
			subs[sub.id] = sub
			if delayChan == nil {
				delayChan = rmgr.subsDelayer()
			}
			needsUpdate = true

			continue loop

		case unsub := <-rmgr.unsubChan:
			if _, ok := subs[unsub.id]; !ok {
				// Do not have this sub. Fail the unsubscribe.
				subErr := makeRdvzAlreadyUnsubscribedError(unsub.id.String())
				go unsub.replyUnsubDone(subErr, rmgr.runDone)
				continue loop
			}

			rmgr.log.Tracef("Unsubscribe from RV %s", unsub.id)

			delete(subs, unsub.id)
			unsubs = append(unsubs, unsub)
			toDel = append(toDel, unsub.id)
			if delayChan == nil {
				delayChan = rmgr.subsDelayer()
			}
			needsUpdate = true

			continue loop

		case rprm := <-rmgr.handlerChan:
			// Handle received pushed RM. The handleInSub call will
			// ack the result of processing the RV.
			sub, ok := subs[rprm.prm.RV]
			go rmgr.handleInSub(rprm, sub, ok)

			continue loop

		case updateErr := <-updateResChan:
			lastUpdateDone = true
			lastUpdateSuccess = updateErr == nil
			if updateErr != nil {
				// Dissociate from server due to send error.
				var errUnpaid rpc.ErrUnpaidSubscriptionRV
				if errors.As(updateErr, &errUnpaid) {
					rv := RVID(errUnpaid)
					rmgr.log.Warnf("Received unpaid RV error "+
						"for RV %s. Marking as unpaid.", rv)
					errDB := rmgr.db.MarkRVUnpaid(rv)
					if errDB != nil {
						rmgr.log.Warnf("Unable to mark RV %s unpaid: %v",
							rv, errDB)
					}
				} else {
					rmgr.log.Debugf("Error updating rendezvous subs: %v", updateErr)

					// Force the server connection to be dropped.
					if sess != nil {
						sess.RequestClose(fmt.Errorf("Unable to "+
							"update RVs in server: %v", updateErr))
					}
				}
				sess = nil
				unsubs = append(unsubs, requestedUnsubs...)
				requestedUnsubs = nil
				continue loop
			}

			// Alert outstanding callers that the initial subscription has
			// been done.
			for id, sub := range subs {
				if sub.subDoneChan != nil {
					go sub.replySubDone(nil, rmgr.runDone)
					sub.subDoneChan = nil
					subs[id] = sub
				}
			}

			// Alert any outstanding unsub that is was done.
			for _, unsub := range requestedUnsubs {
				go unsub.replyUnsubDone(nil, rmgr.runDone)
			}

			if needsUpdate && delayChan == nil {
				delayChan = rmgr.subsDelayer()
			}

			// Call global sub done CB.
			if rmgr.subDoneCB != nil {
				rmgr.subDoneCB()
			}

			continue loop

		case replyC := <-rmgr.isUpToDate:
			replyC <- !needsUpdate && lastUpdateSuccess
			continue loop

		case <-ctx.Done():
			err = ctx.Err()
			break loop
		}

		if sess == nil || (len(subs) == 0 && len(unsubs) == 0) || !lastUpdateDone {
			continue loop
		}

		// Start updating the latest subscriptions in a goroutine.
		lastUpdateDone = false
		lastUpdateSuccess = false
		requestedUnsubs = unsubs
		unsubs = nil
		delayChan = nil
		needsUpdate = false
		go func(add, del []ratchet.RVPoint, sess clientintf.ServerSessionIntf) {
			select {
			case updateResChan <- rmgr.updatePayloadSubscriptions(ctx, add, del, subs, sess):
			case <-ctx.Done():
			}
		}(toAdd, toDel, sess)
		toAdd = nil
		toDel = nil
	}

	close(rmgr.runDone)

	return err
}
