package client

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/companyzero/bisonrelay/client/clientdb"
	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/companyzero/bisonrelay/client/internal/lowlevel"
	"github.com/companyzero/bisonrelay/client/internal/waitingq"
	"github.com/companyzero/bisonrelay/ratchet"
	"github.com/companyzero/bisonrelay/rpc"
	"github.com/companyzero/bisonrelay/sw"
	"github.com/companyzero/bisonrelay/zkidentity"
	"github.com/companyzero/sntrup4591761"
	"github.com/davecgh/go-spew/spew"
	"github.com/decred/slog"
	"golang.org/x/sync/errgroup"
)

const (
	// The following are the priority values for various types of messages.
	priorityPM      = 0
	priorityUnacked = 0
	priorityGC      = 1
	priorityDefault = 2
	priorityUpload  = 4
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

	db              *clientdb.DB
	q               rmqIntf
	rmgr            rdzvManagerIntf
	log             slog.Logger
	logPayloads     slog.Logger
	id              *zkidentity.PublicIdentity
	localID         *zkidentity.FullIdentity
	runDone         chan struct{}
	stopChan        chan struct{}
	nextRMChan      chan *remoteUserRM
	ratchetChan     chan *ratchet.Ratchet
	decryptedRMChan chan error
	sentRMChan      chan error
	compressLevel   int
	myResetRV       clientdb.RawRVID
	theirResetRV    clientdb.RawRVID

	// mtx protects the following fields.
	mtx     sync.Mutex
	ignored bool

	// rmHandler is called whenever we receive a RM from this user. This is
	// called as a goroutine.
	rmHandler func(ru *RemoteUser, h *rpc.RMHeader, c interface{}, ts time.Time)

	// rmHandlerWG tracks calls to the rmHandler that need to complete
	// before run() returns.
	rmHandlerWG sync.WaitGroup

	// wq* keeps track of inflight calls to this user that need to be done
	// one at a time, because there's no way to demux replies.
	wqSubPosts *waitingq.WaitingReplyQueue

	rLock  sync.Mutex
	r      *ratchet.Ratchet
	rError error
}

func newRemoteUser(q rmqIntf, rmgr rdzvManagerIntf, db *clientdb.DB, remoteID *zkidentity.PublicIdentity, localID *zkidentity.FullIdentity, r *ratchet.Ratchet) *RemoteUser {
	return &RemoteUser{
		q:               q,
		rmgr:            rmgr,
		r:               r,
		db:              db,
		log:             slog.Disabled,
		logPayloads:     slog.Disabled,
		id:              remoteID,
		localID:         localID,
		runDone:         make(chan struct{}),
		stopChan:        make(chan struct{}),
		nextRMChan:      make(chan *remoteUserRM),
		wqSubPosts:      new(waitingq.WaitingReplyQueue),
		ratchetChan:     make(chan *ratchet.Ratchet),
		decryptedRMChan: make(chan error),
		sentRMChan:      make(chan error),
	}
}

func (ru *RemoteUser) ID() UserID {
	return ru.id.Identity
}

func (ru *RemoteUser) PublicIdentity() zkidentity.PublicIdentity {
	return *ru.id
}

func (ru *RemoteUser) Nick() string {
	return ru.id.Nick
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
	return fmt.Sprintf("%s (%q)", ru.id.Identity, ru.id.Nick)
}

func (ru *RemoteUser) replaceRatchet(newR *ratchet.Ratchet) {
	ru.ratchetChan <- newR
}

// removeUnacked removes this user's unacked RM if it matches the specified rv.
func (ru *RemoteUser) removeUnacked(rv clientintf.RawRVID) {
	// Get a context that may be canceled once run is done.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		select {
		case <-ctx.Done():
		case <-ru.runDone:
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

	me, err := rpc.ComposeCompressedRM(ru.localID, payload, ru.compressLevel)
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

	// This inner channel is needed in order to alert run() of completed
	// sends.
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

			// Alert run() of result of send.
			select {
			case ru.sentRMChan <- err:
			case <-ru.runDone:
			}
		case <-ru.runDone:
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
	to zkidentity.PublicIdentity, priority uint) error {

	// Create message blob
	composed, err := rpc.ComposeCompressedRM(ru.localID, payload, ru.compressLevel)
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
// canceled if the run() method terminates.
func (ru *RemoteUser) cancelableCtx() (context.Context, func()) {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		select {
		case <-ctx.Done():
		case <-ru.runDone:
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
	// this RemoteUser.run() method returns and then _only_ if a further 5
	// second timeout elapses, to ensure we've made all tries to save the
	// updated ratchet.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		select {
		case <-ctx.Done():
			// saveRatchet() returned already.
			return
		case <-ru.runDone:
		}

		// Wait for another 5 seconds after run() ends.
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

		err := ru.db.UpdateRatchet(tx, ru.r, ru.id.Identity)

		if err == nil && encrypted != nil && rv != nil {
			err = ru.db.StoreUserUnackedRM(tx, ru.ID(), encrypted,
				*rv, payEvent)
		}

		return err
	})

	if errors.Is(err, context.Canceled) {
		// Only way to reach this is after the 5 second timeout after
		// run() ends.
		err = fmt.Errorf("premature cancellation of ratchet update in the DB")
	}

	if err != nil {
		ru.log.Warnf("Error while saving updated ratchet: %v", err)
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
// points. This is called asynchronously by the RV Manager once we've registered
// this user's receive RV points.
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

	// Successfully decrypted using the ratchet. Let Run() know.
	select {
	case ru.decryptedRMChan <- nil:
	case <-ru.runDone:
		return errRemoteUserExiting
	}

	h, c, err := rpc.DecomposeRM(ru.id, cleartext, uint(ru.q.MaxMsgSize()))
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

	// Handle every received msg in a different goroutine.
	if ru.rmHandler != nil {
		ru.rmHandlerWG.Add(1)
		go func() {
			ru.rmHandler(ru, h, c, recvBlob.ServerTS)
			ru.rmHandlerWG.Done()
		}()
	}

	return nil
}

// maybeUpdateRVs updates the RVs we listen on related to this user in the
// server.
func (ru *RemoteUser) maybeUpdateRVs(lastRecvRV, lastDrainRV ratchet.RVPoint, handler lowlevel.RVHandler) (ratchet.RVPoint, ratchet.RVPoint, error) {
	ru.rLock.Lock()
	rv, drainRV := ru.r.RecvRendezvous()
	rvDebug, drainDebug := ru.r.RecvRendezvousPlainText()
	ru.rLock.Unlock()

	ru.log.Tracef("maybeUpdateRVs: lastRcvRV %s lastDrainRV %s rv %s (%s) "+
		"drainRV %s (%s)", lastRecvRV, lastDrainRV, rv, rvDebug,
		drainRV, drainDebug)

	if (rv == lastRecvRV) && (drainRV == lastDrainRV) {
		return lastRecvRV, lastDrainRV, nil
	}

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
	default:
		// Other errors are fatal.
		return emptyRV, emptyRV, err
	}

	return rv, drainRV, nil
}

// stop requests this user to be stopped unilaterally.
func (ru *RemoteUser) stop() {
	select {
	case ru.stopChan <- struct{}{}:
	case <-ru.runDone:
	}
}

func (ru *RemoteUser) run(ctx context.Context) error {
	// Perform initial subscription to the ratchet receive RVs.
	var emptyRV ratchet.RVPoint
	handler := ru.handleReceivedEncrypted
	lastRecvRV, lastDrainRV, err := ru.maybeUpdateRVs(emptyRV, emptyRV, handler)
	if err != nil {
		return err
	}

nextAction:
	for err == nil {
		select {
		case <-ru.stopChan:
			// nil marks this run as exiting gracefully.
			err = nil
			break nextAction

		case err = <-ru.sentRMChan:
			// Completed a send. We only get errors if encryption
			// failed on the RM or if the rmq is exiting.
			if err != nil && !errors.Is(err, clientintf.ErrSubsysExiting) {
				ru.log.Errorf("Stopping remote user due to "+
					"send error: %v", err)
			} else {
				ru.log.Tracef("Got send err: %v", err)
			}

		case decodeErr := <-ru.decryptedRMChan:
			err = decodeErr

		case newR := <-ru.ratchetChan:
			// Ratchet updated. Save in DB and update listen RVs.
			ru.log.Debugf("Received new ratchet to replace existing")
			ru.rLock.Lock()
			ru.r = newR
			ru.rError = nil
			err = ru.saveRatchet(nil, nil, "")
			ru.rLock.Unlock()

		case <-ctx.Done():
			err = ctx.Err()
		}

		if err != nil {
			break nextAction
		}

		// Rotate receive RVs in the rv manager if needed.
		lastRecvRV, lastDrainRV, err = ru.maybeUpdateRVs(lastRecvRV,
			lastDrainRV, handler)
	}

	// Prevent new stuff coming in.
	close(ru.runDone)

	// Unsub from the last subscribed RVs. Safe to ignore errors here, as
	// we're quitting anyway.
	if lastDrainRV != emptyRV {
		go ru.rmgr.Unsub(lastDrainRV)
	}
	if lastRecvRV != emptyRV {
		go ru.rmgr.Unsub(lastRecvRV)
	}

	// Wait until all handlers complete or at most 10 seconds. In general,
	// handlers aren't slow, so this protects against deadlocks preventing
	// shutdown.
	handlersDone := make(chan struct{})
	go func() {
		ru.rmHandlerWG.Wait()
		close(handlersDone)
	}()
	select {
	case <-handlersDone:
	case <-time.After(10 * time.Second):
		ru.log.Warnf("Timeout waiting for some handlers to finish")
	}

	ru.log.Tracef("Finished running user. err=%v", err)
	return err
}

// remoteUserList is a concurrent-safe list of active remote users.
type remoteUserList struct {
	sync.Mutex
	m map[UserID]*RemoteUser
}

func newRemoteUserList() *remoteUserList {
	return &remoteUserList{
		m: make(map[UserID]*RemoteUser),
	}
}

// uniqueNick returns a nick that is unique within the list of all users. It
// does so by trying to append "_n" for increasing n to the passed nick.
//
// This is called by add() and for correct operation, the list must be locked.
func (rul *remoteUserList) uniqueNick(nick string, uid UserID) string {
	origNick := nick

nextTry:
	for i := 2; ; i++ {
		// Ensure no one has the same nick.
		for ruid, ru := range rul.m {
			if ruid == uid {
				continue
			}
			if ru.id.Nick == nick || strings.HasPrefix(ruid.String(), nick) {
				nick = fmt.Sprintf("%s_%d", origNick, i)
				continue nextTry
			}
		}
		// All good.
		break
	}

	return nick
}

func (rul *remoteUserList) add(ru *RemoteUser) (*RemoteUser, error) {
	rul.Lock()
	if oldRU, ok := rul.m[ru.id.Identity]; ok {
		rul.Unlock()
		return oldRU, alreadyHaveUserError{ru.id.Identity}
	}

	ru.id.Nick = rul.uniqueNick(ru.id.Nick, ru.id.Identity)
	rul.m[ru.id.Identity] = ru
	rul.Unlock()
	return nil, nil
}

func (rul *remoteUserList) del(ru *RemoteUser) {
	rul.Lock()
	delete(rul.m, ru.id.Identity)
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
		if strings.EqualFold(ru.id.Nick, nick) || len(nick) > 4 && strings.HasPrefix(uid.String(), nick) {
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
func (rul *remoteUserList) userList() []UserID {
	rul.Lock()
	res := make([]UserID, len(rul.m))
	var i int
	for id := range rul.m {
		res[i] = id
		i += 1
	}
	rul.Unlock()
	return res
}

func (rul *remoteUserList) nicksWithPrefix(prefix string) []string {
	rul.Lock()
	var res []string
	for _, ru := range rul.m {
		nick := ru.Nick()
		if strings.HasPrefix(nick, prefix) {
			res = append(res, nick)
		}
	}
	rul.Unlock()
	return res
}

func (rul *remoteUserList) modifyUserNick(ru *RemoteUser, newNick string) {
	rul.Lock()
	ru.id.Nick = newNick
	rul.Unlock()
}
