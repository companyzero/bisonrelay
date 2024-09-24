package client

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/companyzero/bisonrelay/client/clientdb"
	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/companyzero/bisonrelay/client/internal/lowlevel"
	"github.com/companyzero/bisonrelay/ratchet"
	"github.com/companyzero/bisonrelay/rpc"
	"github.com/companyzero/bisonrelay/sw"
	"github.com/companyzero/bisonrelay/zkidentity"
	"github.com/companyzero/sntrup4591761"
	"github.com/davecgh/go-spew/spew"
	"github.com/decred/slog"
	"golang.org/x/sync/errgroup"
	"golang.org/x/text/collate"
)

const (
	// The following are the priority values for various types of messages.
	priorityPM      = 0
	priorityUnacked = 0
	priorityGC      = 1
	priorityDefault = 2
	priorityUpload  = 4

	// remoteUserStopHandlerTimeout is the timeout that remote users use
	// to determine if a handler has timed out after the client was
	// commanded to stop.
	remoteUserStopHandlerTimeout = 10 * time.Second
)

type UserID = clientintf.UserID
type GCID = zkidentity.ShortID

// RemoteIDFromStr converts the given string to a UserID. Returns an empty
// uid if the string is not a valid UserID.
func UserIDFromStr(s string) UserID {
	var r UserID
	b, err := hex.DecodeString(s)
	if err != nil {
		return r
	}

	if len(b) != len(r) {
		return r
	}

	copy(r[:], b)
	return r
}

// RemoteUser tracks the state of a fully formed ratchet (that is, after kx
// completes) and offers services that can be performed on this remote user.
type RemoteUser struct {
	// The following fields should only be set during setup of this struct
	// and are not safe for concurrent modification.

	db            *clientdb.DB
	q             rmqIntf
	rmgr          rdzvManagerIntf
	log           slog.Logger
	logPayloads   slog.Logger
	id            zkidentity.ShortID
	sigKey        zkidentity.FixedSizeEd25519PublicKey
	localIDSigner rpc.MessageSigner
	stopped       chan struct{}
	compressLevel int
	myResetRV     clientdb.RawRVID
	theirResetRV  clientdb.RawRVID

	nick atomic.Pointer[string]

	// mtx protects the following fields.
	mtx     sync.Mutex
	ignored bool

	// rmHandler is called whenever we receive a RM from this user. This
	// runs in the same goroutine as the RV manager. If this returns a
	// non-nil channel, this means the handler has spawned a processing
	// goroutine to process the message and this channel will be closed
	// once the goroutine finishes executing.
	rmHandler func(ru *RemoteUser, h *rpc.RMHeader, c interface{}, ts time.Time) <-chan struct{}

	ntfns *NotificationManager

	// handlerSema tracks how many handlers may be active for this user.
	handlerSema chan struct{}

	// The following fields are protected by rLock.
	rLock       sync.Mutex
	r           *ratchet.Ratchet
	rError      error
	lastRecvRV  ratchet.RVPoint
	lastDrainRV ratchet.RVPoint

	// updateSubsChan is also protected by rLock and is non-nil when the
	// runUpdateSubs loop is running.
	updateSubsChan chan struct{}
}

func newRemoteUser(q rmqIntf, rmgr rdzvManagerIntf, db *clientdb.DB,
	remoteID *zkidentity.PublicIdentity, localIDSigner rpc.MessageSigner,
	r *ratchet.Ratchet) *RemoteUser {

	ru := &RemoteUser{
		q:             q,
		rmgr:          rmgr,
		r:             r,
		db:            db,
		log:           slog.Disabled,
		logPayloads:   slog.Disabled,
		id:            remoteID.Identity,
		sigKey:        remoteID.SigKey,
		localIDSigner: localIDSigner,
		stopped:       make(chan struct{}),
		handlerSema:   filledSema(50),
	}
	ru.setNick(remoteID.Nick)
	return ru
}

func (ru *RemoteUser) ID() UserID {
	return ru.id
}

func (ru *RemoteUser) verifyMessage(msg []byte, sig *zkidentity.FixedSizeSignature) bool {
	return zkidentity.VerifyMessage(msg, sig, &ru.sigKey)
}

func (ru *RemoteUser) setNick(nick string) {
	ru.nick.Store(&nick)
}

func (ru *RemoteUser) Nick() string {
	return *ru.nick.Load()
}

// RatchetDebugInfo is debug information about a user's ratchet state.
type RatchetDebugInfo struct {
	SendRV       ratchet.RVPoint `json:"send_rv"`
	SendRVPlain  string          `json:"send_rv_plain"`
	RecvRV       ratchet.RVPoint `json:"recv_rv"`
	RecvRVPlain  string          `json:"recv_rv_plain"`
	DrainRV      ratchet.RVPoint `json:"drain_rv"`
	DrainRVPlain string          `json:"drain_rv_plain"`
	MyResetRV    string          `json:"my_reset_rv"`
	TheirResetRV string          `json:"their_reset_rv"`
	NbSavedKeys  int             `json:"nb_saved_keys"`
	WillRatchet  bool            `json:"will_ratchet"`
	LastEncTime  time.Time       `json:"last_enc_time"`
	LastDecTime  time.Time       `json:"last_dec_time"`
}

// RatchetDebugInfo returns debug information about this user's ratchet.
func (ru *RemoteUser) RatchetDebugInfo() RatchetDebugInfo {
	ru.rLock.Lock()
	recvPlain, drainPlain := ru.r.RecvRendezvousPlainText()
	recv, drain := ru.r.RecvRendezvous()
	encTime, decTime := ru.r.LastEncDecTimes()
	res := RatchetDebugInfo{
		SendRV:       ru.r.SendRendezvous(),
		SendRVPlain:  ru.r.SendRendezvousPlainText(),
		RecvRV:       recv,
		RecvRVPlain:  recvPlain,
		DrainRV:      drain,
		DrainRVPlain: drainPlain,
		MyResetRV:    ru.myResetRV.String(),
		TheirResetRV: ru.theirResetRV.String(),
		NbSavedKeys:  ru.r.NbSavedKeys(),
		WillRatchet:  ru.r.WillRatchet(),
		LastEncTime:  encTime,
		LastDecTime:  decTime,
	}
	ru.rLock.Unlock()
	return res
}

func (ru *RemoteUser) IsIgnored() bool {
	ru.mtx.Lock()
	res := ru.ignored
	ru.mtx.Unlock()
	return res
}

func (ru *RemoteUser) SetIgnored(ignored bool) {
	ru.mtx.Lock()
	ru.ignored = ignored
	ru.mtx.Unlock()
}

func (ru *RemoteUser) LastRatchetTimes() (time.Time, time.Time) {
	ru.rLock.Lock()
	enc, dec := ru.r.LastEncDecTimes()
	ru.rLock.Unlock()
	return enc, dec
}

func (ru *RemoteUser) String() string {
	return fmt.Sprintf("%s (%q)", ru.ID(), ru.Nick())
}

func (ru *RemoteUser) replaceRatchet(newR *ratchet.Ratchet) {
	ru.rLock.Lock()
	ru.r = newR
	ru.triggerRVUpdate()
	ru.rLock.Unlock()
}

// removeUnacked removes this user's unacked RM if it matches the specified rv.
func (ru *RemoteUser) removeUnacked(rv clientintf.RawRVID) {
	// Get a context that may be canceled once run is done.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		select {
		case <-ctx.Done():
		case <-ru.stopped:
			cancel()
		}
	}()
	var removed bool
	err := ru.db.Update(ctx, func(tx clientdb.ReadWriteTx) error {
		var err error
		removed, err = ru.db.RemoveUserUnackedRMWithRV(tx, ru.ID(), rv)
		return err
	})
	if err != nil {
		// Not removing the unacked RM is not a critical error. At most,
		// this causes a retransmit.
		ru.log.Warnf("Unable to delete unacked user RM: %v", err)
	} else if removed {
		ru.log.Debugf("Removed unacked RM with RV %s", rv)
	}
}

// removeUnackedRMDueToErr returns true if we should remove the unacked RM
// due to the specified error received from the sending loop.
func removeUnackedRMDueToErr(err error) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, clientintf.ErrSubsysExiting) {
		return false
	}

	return true
}

// queueRMPriority queues the given payload in the underlying RMQ and returns
// once it has been queued.
//
// replyChan is written to once the server acks the payload.
func (ru *RemoteUser) queueRMPriority(payload interface{}, priority uint,
	replyChan chan error, payEvent string) error {

	if priority > 4 {
		return fmt.Errorf("priority must be max 4")
	}

	if ru.isStopped() {
		return errRemoteUserExiting
	}

	me, err := rpc.ComposeCompressedRM(ru.localIDSigner, payload, ru.compressLevel)
	if err != nil {
		return err
	}

	estSize := rpc.EstimateRoutedRMWireSize(len(me))
	maxMsgSize := int(ru.q.MaxMsgSize())
	if estSize > maxMsgSize {
		return fmt.Errorf("message %T estimated as larger than "+
			"max message size %d > %d: %w", payload,
			estSize, maxMsgSize, errRMTooLarge)
	}

	if ru.logPayloads.Level() <= slog.LevelTrace {
		ru.logPayloads.Tracef("Queuing RM %s",
			strings.TrimSpace(spew.Sdump(payload)))
	} else {
		ru.logPayloads.Debugf("Queueing RM %T", payload)
	}

	orm := &remoteUserRM{
		pri:      priority,
		msg:      me,
		ru:       ru,
		payloadT: fmt.Sprintf("%T", payload),
		payEvent: payEvent,
	}

	// Inner channel that is written when the message was sent and ack'd by
	// the server.
	innerReplyChan := make(chan error)

	ru.log.Tracef("Queuing to RMQ %T", payload)
	if err := ru.q.QueueRM(orm, innerReplyChan); err != nil {
		return err
	}

	// Handle sending reply.
	go func() {
		var err error
		select {
		case err = <-innerReplyChan:
			ru.log.Debugf("Sent RM %T via RV %s (err: %v)", payload,
				orm.sendRV, err)

			if removeUnackedRMDueToErr(err) {
				ru.removeUnacked(orm.sendRV)
			}

			if err == nil && ru.ntfns != nil {
				ru.ntfns.notifyRMSent(ru, payload)
			}
		case <-ru.stopped:
			err = errRemoteUserExiting
		}

		if replyChan != nil {
			replyChan <- err
		}
	}()

	return nil
}

// sendRMPriority schedules the given RM in the user's rmq with the given
// priority number. It returns when the RM has been ack'd by the server .
func (ru *RemoteUser) sendRMPriority(payload interface{}, payEvent string, priority uint) error {
	replyChan := make(chan error)
	if err := ru.queueRMPriority(payload, priority, replyChan, payEvent); err != nil {
		return err
	}
	return <-replyChan
}

// sendRM schedules the given RM in the user's rmq. It returns when the RM has
// been ack'd by the server.
func (ru *RemoteUser) sendRM(payload interface{}, payEvent string) error {
	return ru.sendRMPriority(payload, payEvent, priorityDefault)
}

// sendTransitive sends the given payload as a transitive message encoded to
// the given ultimate receiver.
func (ru *RemoteUser) sendTransitive(payload interface{}, payType string,
	to *zkidentity.PublicIdentity, priority uint) error {

	// Create message blob
	composed, err := rpc.ComposeCompressedRM(ru.localIDSigner, payload, ru.compressLevel)
	if err != nil {
		return fmt.Errorf("compose encapsulated msg: %v", err)
	}

	// Encrypt payload
	cipherText, sharedKey, err := sntrup4591761.Encapsulate(rand.Reader,
		(*sntrup4591761.PublicKey)(&to.Key))
	if err != nil {
		return fmt.Errorf("encapsulate transitive msg: %v", err)
	}
	encrypted, err := sw.Seal(composed, sharedKey)
	if err != nil {
		return fmt.Errorf("encrypt reset: %v", err)
	}

	// Create message blob
	tm := rpc.RMTransitiveMessage{
		For:        to.Identity,
		Message:    encrypted,
		CipherText: *cipherText,
	}
	payEvent := fmt.Sprintf("sendTransitive.%s.%s", to.Identity, payType)
	return ru.sendRMPriority(tm, payEvent, priority)
}

// sendPM sends a private message to this remote user.
func (ru *RemoteUser) sendPM(msg string) error {
	return ru.sendRMPriority(rpc.RMPrivateMessage{
		Mode:    rpc.RMPrivateMessageModeNormal,
		Message: msg,
	}, "pm", priorityPM)
}

// cancelableCtx returns a context that is cancelable and that is automatically
// canceled if the remote user is stopped.
func (ru *RemoteUser) cancelableCtx() (context.Context, func()) {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		select {
		case <-ctx.Done():
		case <-ru.stopped:
			cancel()
		}
	}()
	return ctx, cancel
}

// paidForRM is called whenever an outbound RM has been paid for.
func (ru *RemoteUser) paidForRM(event string, amount, fees int64) {
	// Amount is set to negative due to being an outbound payment.
	amount = -amount
	fees = -fees

	ctx, cancel := ru.cancelableCtx()
	defer cancel()
	err := ru.db.Update(ctx, func(tx clientdb.ReadWriteTx) error {
		return ru.db.RecordUserPayEvent(tx, ru.ID(), event, amount, fees)
	})
	if err != nil {
		ru.log.Warnf("Unable to store payment %d of event %q: %v", amount,
			event, err)
	}
}

// saveRatchet saves the current ratchet to the DB. This MUST be called with the
// ratchet lock held.
//
// If encrypted and rv are specified, they are saved as the current unsent user
// RM.
func (ru *RemoteUser) saveRatchet(encrypted []byte, rv *clientintf.RawRVID,
	payEvent string) error {

	// To ensure this save can't be canceled by almost any signal, we use a
	// special context here. We cancel this context prematurely only if
	// this RemoteUser.stopped() method is called and then _only_ if a
	// further 5 second timeout elapses, to ensure we've made all tries to
	// save the updated ratchet.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		select {
		case <-ctx.Done():
			// saveRatchet() returned already.
			return
		case <-ru.stopped:
		}

		// Wait for another 5 seconds after the client is stopped.
		select {
		case <-time.After(5 * time.Second):
			cancel()
		case <-ctx.Done():
		}
	}()

	// Persist the ratchet update and save the last unacked RM (if it was
	// provided).
	err := ru.db.Update(ctx, func(tx clientdb.ReadWriteTx) error {
		// Note that in the current DB implementation as of this
		// writing, this is NOT an atomic operation, therefore a busted
		// ratchet can happen if some critical error (power failure,
		// OOM, etc) happens between the two calls below.

		err := ru.db.UpdateRatchet(tx, ru.r, ru.id)

		if err == nil && encrypted != nil && rv != nil {
			err = ru.db.StoreUserUnackedRM(tx, ru.id, encrypted,
				*rv, payEvent)
		}

		return err
	})

	if errors.Is(err, context.Canceled) {
		// Only way to reach this is after the 5 second timeout after
		// the client is stopped.
		err = fmt.Errorf("premature cancellation of ratchet update in the DB")
	}

	if err != nil {
		ru.log.Errorf("Error while saving updated ratchet: %v", err)
	} else {
		ru.log.Debugf("Updated ratchet state in DB")
	}

	return err
}

// encryptRM encrypts the given remoteUserRM just before it's sent.
func (ru *RemoteUser) encryptRM(rm *remoteUserRM) (lowlevel.RVID, []byte, error) {
	rm.ru.rLock.Lock()
	defer rm.ru.rLock.Unlock()

	if rm.encrypted != nil || ru.rError != nil {
		// If already encrypted, then re-send (this was likely a server
		// write error).
		return rm.sendRV, rm.encrypted, ru.rError
	}

	sendRV := ru.r.SendRendezvous()
	enc, err := ru.r.Encrypt(nil, rm.msg)

	// Save the ratchet as updated.
	if err == nil {
		err = ru.saveRatchet(enc, &sendRV, rm.payEvent)
	}

	if err != nil {
		// This means ratchet is busted. Stop any attempts to use it.
		//
		// This should only really error if something is _very_ wrong.
		ru.rError = err
		return rm.sendRV, rm.encrypted, err
	}

	rm.encrypted = enc
	rm.sendRV = sendRV
	wireEstSize := rpc.EstimateRoutedRMWireSize(len(rm.msg))
	ru.log.Debugf("Sending RM %s via RV %s (payload size %d, "+
		"encrypted size %d, wire est. size %d)", rm, rm.sendRV, len(rm.msg),
		len(enc), wireEstSize)

	return rm.sendRV, rm.encrypted, err
}

// handleReceivedEncrypted handles a received blob on one of this user's RV
// points. This is called by the RV Manager once we've registered this user's
// receive RV points.
//
// Note: This is called in the main RV manager goroutine, so it should not
// block for a significant amount of time.
//
// The received RV is only ack'd (and therefore removed from the server) once
// this function returns.
func (ru *RemoteUser) handleReceivedEncrypted(recvBlob lowlevel.RVBlob) error {
	// Handle received RM.
	ru.rLock.Lock()
	cleartext, decodeErr := ru.r.Decrypt(recvBlob.Decoded)

	if decodeErr == nil {
		err := ru.saveRatchet(nil, nil, "")
		if err != nil {
			ru.rLock.Unlock()
			ru.log.Errorf("Error saving ratchet after decryption: %v. " +
				"This might cause a busted ratchet")
			return err
		}

		// Decrypting worked, therefore the set of RVs we need to be
		// subscribed to has changed.
		ru.triggerRVUpdate()
	}
	ru.rLock.Unlock()

	if decodeErr != nil {
		// A decode error could be due to (e.g.) a network error that
		// created an invalid msg, a malicious server sending
		// gibberish, a malicious remote peer that was able to guess an
		// RV point or a de-sync error causing a busted ratchet. So we
		// log the error but otherwise don't completely stop trying to
		// use the ratchet.
		//
		// NOTE: we're assuming that if Decrypt() errors, the internal
		// ratchet state was _not_ updated and _not_ saved do the DB.
		ru.log.Warnf("Error decrypting ratchet msg at RV %s: %v",
			recvBlob.ID, decodeErr)
		return nil
	}

	h, c, err := rpc.DecomposeRM(ru.verifyMessage, cleartext, uint(ru.q.MaxMsgSize()))
	if err != nil {
		// Encryption is authenticated, so an error here means the
		// client encoded an unknown or otherwise invalid RM.
		return fmt.Errorf("could not decode remote command: %v", err)
	}

	if ru.logPayloads.Level() <= slog.LevelTrace {
		ru.logPayloads.Tracef("Received RM %q via RV %s: %s", h.Command,
			recvBlob.ID, spew.Sdump(c))
	} else {
		ru.logPayloads.Debugf("Received RM %q via RV %s", h.Command, recvBlob.ID)
	}

	if ru.rmHandler == nil {
		ru.log.Warnf("remoteUser rmHandler is nil")
		return nil
	}

	// Obtain a semaphore to run this handler.
	if err := ru.acquireHandlerSema(); err != nil {
		return err
	}

	handlerDoneChan := ru.rmHandler(ru, h, c, recvBlob.ServerTS)
	if handlerDoneChan == nil {
		// Msg handled, return semaphore directly.
		ru.returnHandlerSema()
		return nil
	}

	// Otherwise, return only after the processing goroutine is
	// done.
	go func() {
		select {
		case <-handlerDoneChan:
		case <-ru.stopped:
			// After commanded to stop, wait an additional
			// time to see if the handler will actually
			// terminate.
			select {
			case <-handlerDoneChan:
			case <-time.After(remoteUserStopHandlerTimeout):
				ru.log.Warnf("Handler for message %T "+
					"timed out after shutdown "+
					"requested", c)
			}
		}
		ru.returnHandlerSema()
	}()
	return nil
}

// updateRVs updates the RVs we listen on related to this user in the server.
func (ru *RemoteUser) updateRVs() {
	ru.rLock.Lock()
	rv, drainRV := ru.r.RecvRendezvous()
	rvDebug, drainDebug := ru.r.RecvRendezvousPlainText()
	lastRecvRV, lastDrainRV := ru.lastRecvRV, ru.lastDrainRV
	ru.rLock.Unlock()

	ru.log.Tracef("updateRVs: lastRcvRV %s lastDrainRV %s rv %s (%s) "+
		"drainRV %s (%s)", lastRecvRV, lastDrainRV, rv, rvDebug,
		drainRV, drainDebug)

	if (rv == lastRecvRV) && (drainRV == lastDrainRV) {
		return
	}

	handler := ru.handleReceivedEncrypted

	// Need to rotate receive RVs.
	//
	// We'll perform a bunch of modifications to our RV
	// subscriptions, so make sure we wait for all of them to
	// complete.
	var g errgroup.Group

	var emptyRV ratchet.RVPoint
	if lastDrainRV != emptyRV && lastDrainRV != drainRV && lastDrainRV != rv {
		// Unsub from lastDrainRV because it is not needed
		// anymore.
		ru.log.Tracef("Unsubscribing to lastDrainRV %s", lastDrainRV)
		g.Go(func() error { return ru.rmgr.Unsub(lastDrainRV) })
	}

	if lastRecvRV != emptyRV && lastRecvRV != drainRV && lastRecvRV != rv {
		// Unsub from lastRecvRV because it is not needed anymore.
		ru.log.Tracef("Unsubscribing to lastRecvRV %s", lastRecvRV)
		g.Go(func() error { return ru.rmgr.Unsub(lastRecvRV) })
	}

	if drainRV != emptyRV && drainRV != lastDrainRV && drainRV != lastRecvRV {
		// Sub to the drain RV when it's first needed.
		ru.log.Tracef("Subscribing to drainRV %s", drainRV)
		subPaidHandler := func(amount, fees int64) {
			payEvent := fmt.Sprintf("sub.%s", drainRV.ShortLogID())
			go ru.paidForRM(payEvent, amount, fees)
		}
		g.Go(func() error { return ru.rmgr.Sub(drainRV, handler, subPaidHandler) })
	}

	if rv != lastRecvRV {
		// Sub to the new RV.
		ru.log.Tracef("Subscribing to new RV %s", rv)
		subPaidHandler := func(amount, fees int64) {
			payEvent := fmt.Sprintf("sub.%s", rv.ShortLogID())
			go ru.paidForRM(payEvent, amount, fees)
		}
		g.Go(func() error { return ru.rmgr.Sub(rv, handler, subPaidHandler) })
	}

	// Wait until all changes are applied on the server.
	err := g.Wait()
	switch {
	case err == nil:
		// Keep going.
	case errors.Is(err, lowlevel.ErrRVAlreadySubscribed{}):
		// This ordinarily shouldn't happen if our assumptions
		// are correct, but isn't a fatal error, so log it as a
		// debug msg and keep going.
		ru.log.Debugf("Unexpected non-fatal error: %v", err)
	case errors.Is(err, clientintf.ErrSubsysExiting):
		// Ignore this error because it means the client is shutting
		// down.
	default:
		ru.log.Errorf("Unexpected error during RV subscription: %v", err)
		return
	}

	// Track the last RVs subscribed to.
	ru.rLock.Lock()
	ru.lastRecvRV = rv
	ru.lastDrainRV = drainRV
	ru.rLock.Unlock()
}

// stop requests this user to be stopped unilaterally. It must only be called
// once for an user.
func (ru *RemoteUser) stop() {
	ru.log.Tracef("Stopping remote user processing")
	close(ru.stopped)
}

// isStopped returns true if the user was commanded to stop processing.
func (ru *RemoteUser) isStopped() bool {
	select {
	case <-ru.stopped:
		return true
	default:
		return false
	}
}

// acquireHandlerSema acquires a semaphore value to process a handler for this
// user.
func (ru *RemoteUser) acquireHandlerSema() error {
	// Fast track.
	select {
	case <-ru.handlerSema:
		return nil
	case <-ru.stopped:
		return errRemoteUserExiting
	default:
	}

	// Slow track.
	for {
		select {
		case <-ru.handlerSema:
			return nil
		case <-ru.stopped:
			return errRemoteUserExiting
		case <-time.After(time.Second):
			ru.log.Warnf("User RM handling has drained semaphore")
		}
	}
}

// returnHandlerSema returns a semaphore value after processing a handler.
func (ru *RemoteUser) returnHandlerSema() {
	ru.handlerSema <- struct{}{}
}

// waitHandlers waits until all handlers have finished processing or the
// doneChan is closed. The wait only starts once the remote user has been
// asked to stop running. Returns true if the wait was successful.
func (ru *RemoteUser) waitHandlers(doneChan <-chan struct{}) bool {
	select {
	case <-ru.stopped:
	case <-doneChan:
		return false
	}
	for i := 0; i < cap(ru.handlerSema); i++ {
		select {
		case <-ru.handlerSema:
		case <-doneChan:
			return false
		}
	}
	return true
}

// runUpdateSubs runs a loop that updates the set of RVs this user is
// subscribed to whenever a new signal is sent on updateSubsChan.
func (ru *RemoteUser) runUpdateSubs(updateSubsChan chan struct{}) {

	// idleTimeout is when the loop should quit because there are no more
	// messages coming in (the RV manager keeps the last RVs subscribed
	// to).
	const idleTimeout = time.Minute
	timer := time.NewTimer(idleTimeout)

	ru.log.Tracef("Starting to update user's RV subscriptions")

	// needsFinalUpdate is setup to true after the loop if we should do a
	// final update on the RVs.
	needsFinalUpdate := true

loop:
	for {
		ru.updateRVs()
		if !timer.Stop() {
			<-timer.C
		}
		timer.Reset(idleTimeout)

		select {
		case <-updateSubsChan:
			// Loop and update the RVs.

		case <-ru.stopped:
			// No need for final update because client is shutting
			// down.
			needsFinalUpdate = false
			if !timer.Stop() {
				<-timer.C
			}
			break loop

		case <-timer.C:
			break loop
		}
	}

	// Drain the channel under the lock to ensure no more requests to
	// update the RVs came in.
	ru.rLock.Lock()
	for done := false; !done && needsFinalUpdate; {
		select {
		case <-updateSubsChan:
			needsFinalUpdate = true
		default:
			done = true
		}
	}
	ru.updateSubsChan = nil
	ru.rLock.Unlock()

	if needsFinalUpdate {
		ru.updateRVs()
	}

	ru.log.Tracef("Finished updating user's RV subscription")
}

// triggerRVUpdate triggers a new RV update to be started. This MUST be called
// with rLock locked.
func (ru *RemoteUser) triggerRVUpdate() {
	// An update loop is running.
	if ru.updateSubsChan != nil {
		select {
		case ru.updateSubsChan <- struct{}{}:
		case <-ru.stopped:
		}
		return
	}

	// An update loop is NOT running. start it. The chan is buffered to
	// avoid preempting on handleReceivedEncrypted when possible.
	ru.updateSubsChan = make(chan struct{}, 5)
	go ru.runUpdateSubs(ru.updateSubsChan)
}

// remoteUserList is a concurrent-safe list of active remote users.
type remoteUserList struct {
	sync.Mutex
	m       map[UserID]*RemoteUser
	stopped bool

	collator *collate.Collator
}

func newRemoteUserList(collator *collate.Collator) *remoteUserList {
	return &remoteUserList{
		m:        make(map[UserID]*RemoteUser),
		collator: collator,
	}
}

// uniqueNick returns a nick that is unique within the list of all users. It
// does so by trying to append "_n" for increasing n to the passed nick.
//
// This is called by add() and for correct operation, the list must be locked.
func (rul *remoteUserList) uniqueNick(nick string, uid UserID, myNick string) string {
	origNick := nick

	for i := 2; ; i++ {
		// Ensure no one has the same nick.
		isDupe := rul.collator.CompareString(nick, myNick) == 0
		for ruid, ru := range rul.m {
			if isDupe {
				break
			}
			if ruid == uid {
				continue
			}
			isDupe = isDupe ||
				rul.collator.CompareString(ru.Nick(), nick) == 0 ||
				strings.HasPrefix(ruid.String(), nick)
		}

		if isDupe {
			nick = fmt.Sprintf("%s_%d", origNick, i)
		} else {
			// All good.
			break
		}
	}

	return nick
}

func (rul *remoteUserList) add(ru *RemoteUser, myNick string) (*RemoteUser, error) {
	rul.Lock()
	if rul.stopped {
		return nil, errRemoteUserExiting
	}
	if oldRU, ok := rul.m[ru.id]; ok {
		rul.Unlock()
		return oldRU, alreadyHaveUserError{ru.id}
	}

	ru.setNick(rul.uniqueNick(ru.Nick(), ru.id, myNick))
	rul.m[ru.id] = ru
	rul.Unlock()
	return nil, nil
}

func (rul *remoteUserList) del(ru *RemoteUser) {
	rul.Lock()
	delete(rul.m, ru.id)
	rul.Unlock()
}

func (rul *remoteUserList) byID(uid UserID) (*RemoteUser, error) {
	rul.Lock()
	ru := rul.m[uid]
	rul.Unlock()

	if ru == nil {
		return nil, userNotFoundError{id: uid.String()}
	}

	return ru, nil
}

// byNick returns the user with the given nick. The nick can be a string
// representation of the ID or a prefix of the actual nick.
func (rul *remoteUserList) byNick(nick string) (*RemoteUser, error) {
	rul.Lock()
	var res *RemoteUser
	for uid, ru := range rul.m {
		if rul.collator.CompareString(ru.Nick(), nick) == 0 || len(nick) > 4 && strings.HasPrefix(uid.String(), nick) {
			res = ru
			break
		}
	}
	rul.Unlock()

	if res == nil {
		return nil, userNotFoundError{id: nick}
	}
	return res, nil
}

// userList returns the list of all user IDs.
func (rul *remoteUserList) userList(onlyNotIgnored bool) []UserID {
	rul.Lock()
	res := make([]UserID, 0, len(rul.m))
	for id, ru := range rul.m {
		if onlyNotIgnored && ru.IsIgnored() {
			continue
		}
		res = append(res, id)
	}
	rul.Unlock()
	return res
}

func (rul *remoteUserList) nicksWithPrefix(prefix string) []string {
	rul.Lock()
	var res []string
	for _, ru := range rul.m {
		nick := ru.Nick()
		if len(nick) < len(prefix) {
			continue
		}

		// Not a perfect prefix search.
		if rul.collator.CompareString(nick[:len(prefix)], prefix) == 0 {
			res = append(res, nick)
		}
	}
	rul.Unlock()
	return res
}

func (rul *remoteUserList) modifyUserNick(ru *RemoteUser, newNick string) {
	rul.Lock()
	ru.setNick(newNick)
	rul.Unlock()
}

func (rul *remoteUserList) allUsers() []*RemoteUser {
	rul.Lock()
	res := make([]*RemoteUser, len(rul.m))
	var i int
	for _, ru := range rul.m {
		res[i] = ru
		i++
	}
	rul.Unlock()
	return res
}

// stopAndWait stops and waits all remote users to be done or the context to
// be closed.
func (rul *remoteUserList) stopAndWait(ctx context.Context) bool {
	ok := true
	rul.Lock()
	rul.stopped = true
	toStop := rul.m
	rul.m = map[UserID]*RemoteUser{}
	rul.Unlock()

	for _, ru := range toStop {
		ru.stop()
	}
	for _, ru := range toStop {
		ok = ok && ru.waitHandlers(ctx.Done())
	}
	return ok
}
