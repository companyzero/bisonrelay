package lowlevel

import (
	"context"
	"errors"
	"fmt"
	"net"
	"slices"
	"sync/atomic"

	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/companyzero/bisonrelay/rpc"
	rtdtclient "github.com/companyzero/bisonrelay/rtdt/client"
	"github.com/companyzero/bisonrelay/zkidentity"
	"github.com/decred/slog"
)

// RTDTManagerHandlers are callbacks needed by the RTDT session manager.
type RTDTManagerHandlers interface {
	JoinedLiveSession(rtSess *rtdtclient.Session, rv zkidentity.ShortID)
	RefreshedAllowance(rv zkidentity.ShortID, addAllowance uint64)
}

type rtdtSession struct {
	rv               zkidentity.ShortID
	appoint          atomic.Pointer[rpc.AppointRTDTServer]
	publisherKey     *zkidentity.FixedSizeSymmetricKey
	rtSess           *rtdtclient.Session
	size             uint32
	bytesAllowance   int64
	mbytesPerRefresh uint32

	// Context used to cancel ongoing refreshes if the user commands to
	// stop.
	ctx    context.Context
	cancel func()
}

type createRtdtSessionCmd struct {
	size      uint16
	replyChan chan interface{}
}

type getAppointCookieCmd struct {
	req       *rpc.GetRTDTAppointCookies
	replyChan chan interface{}
}

type maintainRtdtSessionCmd struct {
	rv           zkidentity.ShortID
	localPeerID  rpc.RTDTPeerID
	size         uint32
	req          *rpc.AppointRTDTServer
	publisherKey *zkidentity.FixedSizeSymmetricKey
	replyChan    chan error

	// session context that may be canceled to preemptively cancel a
	// connection/joining attempt.
	joinCtx       context.Context
	cancelJoinCtx func()

	// rotatedAppointCookie is set if the appointment cookie is rotated
	// while the client was waiting for the first join cookie.
	rotatedAppointCookie atomic.Pointer[[]byte]
}

type firstAppointRes struct {
	newSess         *maintainRtdtSessionCmd
	rtSess          *rtdtclient.Session
	mbytesAllowance uint32
	err             error
}

type leaveRtdtSessCmd struct {
	rv        zkidentity.ShortID
	replyChan chan error
}

type leaveRtdtSessRes struct {
	cmd  leaveRtdtSessCmd
	sess *rtdtSession
	err  error
}

type refreshRtdtSessAllowanceRes struct {
	rv           zkidentity.ShortID
	addAllowance uint64
	err          error
}

type bytesWrittenCmd struct {
	rv *zkidentity.ShortID
	n  int
}

type rotateRtdtAppointCookieCmd struct {
	rv               zkidentity.ShortID
	newAppointCookie []byte
	replyChan        chan error
}

// RTDTSessionManager performs RTDT operations on a brserver. These include:
//   - Creating sessions
//   - Obtaining appointment cookies for administered sessions
//   - Obtaining and maintaining join cookies for live sessions.
type RTDTSessionManager struct {
	rtc      *rtdtclient.Client
	handlers RTDTManagerHandlers
	log      slog.Logger

	sessionChan             chan clientintf.ServerSessionIntf
	createSessChan          chan createRtdtSessionCmd
	leaveSessChan           chan leaveRtdtSessCmd
	leaveSessResChan        chan leaveRtdtSessRes
	forceUnmaintainChan     chan zkidentity.ShortID
	getCookieChan           chan getAppointCookieCmd
	maintainSessChan        chan *maintainRtdtSessionCmd
	firstAppointResChan     chan firstAppointRes
	refreshSessResChan      chan refreshRtdtSessAllowanceRes
	rotateAppointCookieChan chan rotateRtdtAppointCookieCmd
	bytesWrittenChan        chan bytesWrittenCmd
	runDone                 chan struct{}
}

// NewRTDTSessionManager creates a new RTDT session manager.
func NewRTDTSessionManager(rtc *rtdtclient.Client, handlers RTDTManagerHandlers, log slog.Logger) *RTDTSessionManager {
	return &RTDTSessionManager{
		log:                     log,
		handlers:                handlers,
		rtc:                     rtc,
		sessionChan:             make(chan clientintf.ServerSessionIntf),
		createSessChan:          make(chan createRtdtSessionCmd),
		getCookieChan:           make(chan getAppointCookieCmd),
		leaveSessChan:           make(chan leaveRtdtSessCmd),
		leaveSessResChan:        make(chan leaveRtdtSessRes),
		maintainSessChan:        make(chan *maintainRtdtSessionCmd),
		forceUnmaintainChan:     make(chan zkidentity.ShortID),
		firstAppointResChan:     make(chan firstAppointRes),
		refreshSessResChan:      make(chan refreshRtdtSessAllowanceRes),
		rotateAppointCookieChan: make(chan rotateRtdtAppointCookieCmd),
		bytesWrittenChan:        make(chan bytesWrittenCmd, 100), // Buffered to avoid blocking writing loops.
		runDone:                 make(chan struct{}),
	}
}

// BindToSession sets the remote brserver connection.
func (rtsm *RTDTSessionManager) BindToSession(sess clientintf.ServerSessionIntf) {
	select {
	case rtsm.sessionChan <- sess:
	case <-rtsm.runDone:
	}
}

// CreateRTDTSessionResult is the result of a CreateSession call.
type CreateRTDTSessionResult struct {
	// SessionRV is the final session RV.
	SessionRV zkidentity.ShortID

	// SessionCookie is an opaque cookie to send to manage the session.
	SessionCookie []byte
}

// CreateSession attempts to create a RTDT session in brserver.
func (rtsm *RTDTSessionManager) CreateSession(size uint16) (*CreateRTDTSessionResult, error) {
	if size < 2 {
		return nil, errors.New("cannot create session with less than 2 members")
	}

	cmd := createRtdtSessionCmd{
		size:      size,
		replyChan: make(chan interface{}, 1),
	}
	select {
	case rtsm.createSessChan <- cmd:
	case <-rtsm.runDone:
		return nil, errRtdtMgrExiting
	}

	var res interface{}
	select {
	case res = <-cmd.replyChan:
	case <-rtsm.runDone:
		return nil, errRtdtMgrExiting
	}

	switch res := res.(type) {
	case error:
		return nil, res
	case *CreateRTDTSessionResult:
		return res, nil
	default:
		return nil, fmt.Errorf("unknown return type %T", res)
	}
}

// GetAppointCookies obtains appointment cookies for a session.
func (rtsm *RTDTSessionManager) GetAppointCookies(req *rpc.GetRTDTAppointCookies) (*rpc.GetRTDTAppointCookiesReply, error) {
	cmd := getAppointCookieCmd{
		req:       req,
		replyChan: make(chan interface{}, 1),
	}
	select {
	case rtsm.getCookieChan <- cmd:
	case <-rtsm.runDone:
		return nil, errRtdtMgrExiting
	}

	var res interface{}
	select {
	case res = <-cmd.replyChan:
	case <-rtsm.runDone:
		return nil, errRtdtMgrExiting
	}

	switch res := res.(type) {
	case error:
		return nil, res
	case *rpc.GetRTDTAppointCookiesReply:
		return res, nil
	default:
		return nil, fmt.Errorf("unknown return type %T", res)
	}
}

// MaintainSession joins and maintains a live session. This includes obtaining
// new join cookies with updated publishing allowance.
func (rtsm *RTDTSessionManager) MaintainSession(sessRV zkidentity.ShortID, sess *rpc.AppointRTDTServer,
	publisherKey *zkidentity.FixedSizeSymmetricKey, size uint32, peerID rpc.RTDTPeerID) error {

	joinCtx, cancelJoin := context.WithCancel(context.Background())
	cmd := &maintainRtdtSessionCmd{
		rv:            sessRV,
		joinCtx:       joinCtx,
		cancelJoinCtx: cancelJoin,
		req:           sess,
		publisherKey:  publisherKey,
		size:          size,
		localPeerID:   peerID,
		replyChan:     make(chan error, 1),
	}
	select {
	case rtsm.maintainSessChan <- cmd:
	case <-rtsm.runDone:
		return errRtdtMgrExiting
	}

	select {
	case err := <-cmd.replyChan:
		return err
	case <-rtsm.runDone:
		return errRtdtMgrExiting
	}
}

// LeaveSession stops maintaining the given session live. This does NOT make
// the client leave the actual RTDT sesssion, only stops obtaining new updated
// allowance for publishing.
func (rtsm *RTDTSessionManager) LeaveSession(sessRV *zkidentity.ShortID) error {
	cmd := leaveRtdtSessCmd{
		rv:        *sessRV,
		replyChan: make(chan error, 1),
	}

	select {
	case rtsm.leaveSessChan <- cmd:
	case <-rtsm.runDone:
		return errRtdtMgrExiting
	}

	select {
	case err := <-cmd.replyChan:
		return err
	case <-rtsm.runDone:
		return errRtdtMgrExiting
	}
}

// ForceUnmaintainSession forcibly stops maintaining this session alive. This
// usually happens because the client was kicked from the session or had some
// other connection error.
func (rtsm *RTDTSessionManager) ForceUnmaintainSession(sessRV *zkidentity.ShortID) error {
	select {
	case rtsm.forceUnmaintainChan <- *sessRV:
	case <-rtsm.runDone:
		return errRtdtMgrExiting
	}

	return nil
}

// RotateAppointCookie replaces the appointment cookie for the given session.
func (rtsm *RTDTSessionManager) RotateAppointCookie(sessRV *zkidentity.ShortID, newAppointCookie []byte) error {
	cmd := rotateRtdtAppointCookieCmd{
		rv:               *sessRV,
		newAppointCookie: newAppointCookie,
		replyChan:        make(chan error, 1),
	}
	select {
	case rtsm.rotateAppointCookieChan <- cmd:
	case <-rtsm.runDone:
		return errRtdtMgrExiting
	}

	select {
	case err := <-cmd.replyChan:
		return err
	case <-rtsm.runDone:
		return errRtdtMgrExiting
	}
}

// BytesWritten should be used as a callback on the rtdtclient.Client instance
// to track data sent remotely to the server.
func (rtsm *RTDTSessionManager) BytesWritten(sess *rtdtclient.Session, n int) {
	select {
	case rtsm.bytesWrittenChan <- bytesWrittenCmd{rv: sess.RV(), n: n}:
	case <-rtsm.runDone:
	}
}

func (rtsm *RTDTSessionManager) payInvoiceAmount(ctx context.Context,
	brs clientintf.ServerSessionIntf, action rpc.GetInvoiceAction, amount int64) error {
	pc := brs.PayClient()

	msg := rpc.Message{Command: rpc.TaggedCmdGetInvoice}
	payload := rpc.GetInvoice{
		PaymentScheme: pc.PayScheme(),
		Action:        action,
	}

	replyChan := make(chan interface{}, 1)
	err := brs.SendPRPC(msg, payload, replyChan)
	if err != nil {
		return err
	}

	var reply interface{}
	select {
	case reply = <-replyChan:
	case <-ctx.Done():
		return ctx.Err()
	}

	var invoice string
	switch reply := reply.(type) {
	case error:
		return reply
	case *rpc.GetInvoiceReply:
		invoice = reply.Invoice
	default:
		return fmt.Errorf("unexpected reply type %T", reply)
	}

	_, err = pc.PayInvoiceAmount(ctx, invoice, amount) // TODO: track fees somewhere?
	return err
}

func (rtsm *RTDTSessionManager) createSession(ctx context.Context,
	brs clientintf.ServerSessionIntf, cmd *createRtdtSessionCmd) error {

	// These hardcoded checks match the ones in the server. They are meant
	// as hard bounds during this stage of development.
	//
	// At some point these may be lifted after more testing.
	if cmd.size > 64 {
		return errors.New("cannot create sessions with more than 64 members")
	}

	// Request and pay invoice for creation.
	policy := brs.Policy()
	payAmount := policy.MilliAtomsPerRTSess +
		policy.MilliAtomsPerUserRTSess*uint64(cmd.size)
	err := rtsm.payInvoiceAmount(ctx, brs, rpc.InvoiceActionCreateRTSess, int64(payAmount))
	if err != nil {
		return err
	}

	msg := rpc.Message{Command: rpc.TaggedCmdCreateRTDTSession}
	payload := rpc.CreateRTDTSession{
		Size: uint32(cmd.size),
	}

	// Send brserver command to create session.
	replyChan := make(chan interface{}, 1)
	err = brs.SendPRPC(msg, payload, replyChan)
	if err != nil {
		return err
	}

	// Wait brserver reply.
	var reply interface{}
	select {
	case reply = <-replyChan:
	case <-ctx.Done():
		return ctx.Err()
	}

	// Process brserver reply.
	switch reply := reply.(type) {
	case error:
		return reply
	case *rpc.CreateRTDTSessionReply:
		// Send reply back to caller.
		res := &CreateRTDTSessionResult{
			SessionRV:     zkidentity.RandomShortID(),
			SessionCookie: reply.SessionCookie,
		}
		cmd.replyChan <- res
		return nil

	default:
		return fmt.Errorf("unexpected reply type %T", reply)
	}
}

func (rtsm *RTDTSessionManager) getCookie(ctx context.Context,
	brs clientintf.ServerSessionIntf, cmd *getAppointCookieCmd) error {

	// Request and pay invoice for cookie generation.
	policy := brs.Policy()
	payAmount := policy.MilliAtomsGetCookie + policy.MilliAtomsPerUserCookie*uint64(len(cmd.req.Peers))
	err := rtsm.payInvoiceAmount(ctx, brs, rpc.InvoiceActionGetRTCookie, int64(payAmount))
	if err != nil {
		return fmt.Errorf("unable to complete payment to get RTDT appointment cookies: %v", err)
	}

	// Send request to generate appointment cookies.
	msg := rpc.Message{Command: rpc.TaggedCmdGetRTDTAppointCookie}

	replyChan := make(chan interface{}, 1)
	err = brs.SendPRPC(msg, cmd.req, replyChan)
	if err != nil {
		return err
	}

	// Process reply.
	var reply interface{}
	select {
	case reply = <-replyChan:
	case <-ctx.Done():
		return ctx.Err()
	}
	switch reply := reply.(type) {
	case error:
		return reply
	case *rpc.GetRTDTAppointCookiesReply:
		if len(reply.AppointCookies) != len(cmd.req.Peers) {
			return fmt.Errorf("server sent wrong number of cookies")
		}
		cmd.replyChan <- reply
		return nil
	default:
		return fmt.Errorf("unexpected reply type %T", reply)
	}
}

// firstServerAppointment handles the result of the first appointment to
// maintain a session.
func (rtsm *RTDTSessionManager) firstServerAppointment(ctx context.Context,
	cmd *maintainRtdtSessionCmd, res *rpc.AppointRTDTServerReply) (*rtdtclient.Session, error) {

	// brserver has replied with the corresponding rtdt server to use for
	// this session.
	serverAddr, err := net.ResolveUDPAddr("udp", res.ServerAddress)
	if err != nil {
		return nil, fmt.Errorf("Unable to resolve RTDT server addr %s: %v", res.ServerAddress, err)
	}
	rtsm.log.Infof("Resolved RTDT server address %q to %s", res.ServerAddress, serverAddr)

	// Join the live RTDT session.
	rtSessCfg := rtdtclient.SessionConfig{
		ServerAddr:   serverAddr,
		LocalID:      cmd.localPeerID,
		PublisherKey: cmd.publisherKey,
		RV:           &cmd.rv,
		JoinCookie:   res.JoinCookie,
	}
	if res.ServerPubKey != nil {
		rtSessCfg.SessionKeyGen = res.ServerPubKey.Encapsulate
	}
	rtSess, err := rtsm.rtc.NewSession(ctx, rtSessCfg)
	if err != nil {
		return nil, err
	}

	// Let user know we joined the live session.
	if rtsm.handlers != nil {
		rtsm.handlers.JoinedLiveSession(rtSess, cmd.rv)
	}
	return rtSess, nil
}

// Requests and pays for a join cookie from brserver, using the passed
// appointment cookie.
func (rtsm *RTDTSessionManager) appointRTDTServer(ctx context.Context,
	brs clientintf.ServerSessionIntf, req *rpc.AppointRTDTServer, size uint32,
	mbytes uint32) (*rpc.AppointRTDTServerReply, error) {

	// Pay for publishing allowance.
	policy := brs.Policy()
	payAmount, err := policy.CalcRTPushMAtoms(size, mbytes)
	if err != nil {
		return nil, err
	}
	rtsm.log.Debugf("Attempting to pay %d MAtoms for allowance of %dMB (session size %d)",
		payAmount, mbytes, size)
	err = rtsm.payInvoiceAmount(ctx, brs, rpc.InvoiceActionPublishInRTSess, payAmount)
	if err != nil {
		return nil, fmt.Errorf("unable to complete payment to create server RTDT session: %v", err)
	}

	// Ask brserver for a join cookie.
	msg := rpc.Message{Command: rpc.TaggedCmdAppointRTDTServer}
	replyChan := make(chan interface{}, 1)
	err = brs.SendPRPC(msg, req, replyChan)
	if err != nil {
		return nil, err
	}

	var reply interface{}
	select {
	case reply = <-replyChan:
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	// Request sent to brserver, wait for its reply.
	switch reply := reply.(type) {
	case error:
		return nil, reply
	case *rpc.AppointRTDTServerReply:
		return reply, nil
	default:
		return nil, fmt.Errorf("unexpected reply type %T", reply)
	}
}

// refreshSessAllowance refresh the session allowance for the given session.
func (rtsm *RTDTSessionManager) refreshSessAllowance(ctx context.Context,
	brs clientintf.ServerSessionIntf, sess *rtdtSession) (uint64, error) {

	rtsm.log.Debugf("Starting to refresh session %s with %d MB", sess.rv,
		sess.mbytesPerRefresh)
	res, err := rtsm.appointRTDTServer(ctx, brs, sess.appoint.Load(), sess.size,
		sess.mbytesPerRefresh)
	if err != nil {
		return 0, err
	}

	if err := sess.rtSess.RefreshSession(ctx, res.JoinCookie); err != nil {
		return 0, err
	}
	addAllowance := uint64(sess.mbytesPerRefresh) * 1000000
	if rtsm.handlers != nil {
		rtsm.handlers.RefreshedAllowance(sess.rv, addAllowance)
	}

	return addAllowance, nil
}

// attemptRefreshSessAllowance is a helper to attempt to refresh a session and
// let Run() know of the result.
func (rtsm *RTDTSessionManager) attemptRefreshSessAllowance(ctx context.Context,
	brs clientintf.ServerSessionIntf, sess *rtdtSession) {

	addAllowance, err := rtsm.refreshSessAllowance(ctx, brs, sess)

	// Let Run() know of result.
	res := refreshRtdtSessAllowanceRes{
		rv:           sess.rv,
		addAllowance: addAllowance,
		err:          err,
	}
	select {
	case rtsm.refreshSessResChan <- res:
	case <-rtsm.runDone:
	}
}

func (rtsm *RTDTSessionManager) Run(ctx context.Context) error {
	// TODO: parametrize based on which streams (audio/video) we
	// intend to publish?
	//
	// This should be ~2 minutes of audio between refreshes.
	const mbytesPublisher = 1

	// When to start refresh procedures for a session (in bytes).
	const refreshNeededLoMark = 500000

	var brs clientintf.ServerSessionIntf

	var pendingAppoint = make(map[zkidentity.ShortID]*maintainRtdtSessionCmd)
	var pendingRefresh = make(map[zkidentity.ShortID]bool)
	var sessions = make(map[zkidentity.ShortID]*rtdtSession)
	var pendingLeave = make(map[zkidentity.ShortID]struct{})

loop:
	for {
		select {
		case brs = <-rtsm.sessionChan:
			rtsm.log.Debugf("Using new server session %v", brs)

			if brs == nil {
				// TODO Move every session back to pending
				// appointment?
			} else {
				// Refresh every session that is pending refresh
				// but brserver was offline.
				for rv, refreshing := range pendingRefresh {
					if refreshing {
						continue
					}
					sess := sessions[rv]
					if sess == nil {
						continue
					}

					pendingRefresh[rv] = true
					go rtsm.attemptRefreshSessAllowance(ctx, brs, sess)
				}
			}

		case newSessCmd := <-rtsm.createSessChan:
			// Create session in brserver.
			if brs == nil {
				newSessCmd.replyChan <- errors.New("no server connection to create session")
			} else {
				go func() {
					err := rtsm.createSession(ctx, brs, &newSessCmd)
					if err != nil {
						rtsm.log.Errorf("Unable to create RTDT session in brserver: %v", err)
					}
				}()
			}

		case newGetCookie := <-rtsm.getCookieChan:
			// Obtain appointment cookies from brserver.
			if brs == nil {
				newGetCookie.replyChan <- errors.New("no server connection to create session")
			} else {
				go func() {
					err := rtsm.getCookie(ctx, brs, &newGetCookie)
					if err != nil {
						rtsm.log.Errorf("Unable to create RTDT session in brserver: %v", err)
					}
				}()
			}

		case newSess := <-rtsm.maintainSessChan:
			// Join and maintain live session.
			_, alreadyPending := pendingAppoint[newSess.rv]
			_, alreadyMaintaining := sessions[newSess.rv]
			switch {
			case alreadyMaintaining:
				newSess.replyChan <- errors.New("already maintaining this session")
			case alreadyPending:
				newSess.replyChan <- errors.New("already pending to maintain this session")
			case brs == nil:
				newSess.replyChan <- errors.New("cannot maintain session without brserver connection")
			default:
				mbytes := uint32(mbytesPublisher)
				if newSess.publisherKey == nil {
					// Do not add allowance if not sending data in
					// the session.
					mbytes = 0
					rtsm.log.Debug("Skipping obtaining publish allowance due to nil publisher key")
				}

				pendingAppoint[newSess.rv] = newSess

				// Try to make first server appointment.
				go func() {
					apReply, err := rtsm.appointRTDTServer(newSess.joinCtx, brs,
						newSess.req, newSess.size, mbytes)
					var rtSess *rtdtclient.Session
					if err == nil {
						rtSess, err = rtsm.firstServerAppointment(newSess.joinCtx, newSess, apReply)
					}

					// Let Run() know the result.
					res := firstAppointRes{
						newSess:         newSess,
						rtSess:          rtSess,
						mbytesAllowance: mbytes,
						err:             err,
					}
					select {
					case rtsm.firstAppointResChan <- res:
						// Let caller know the result.
						newSess.replyChan <- err

					case <-rtsm.runDone:
					}
				}()
			}

		case maintainRes := <-rtsm.firstAppointResChan:
			// Process reply of first appointment to maintain session.
			delete(pendingAppoint, maintainRes.newSess.rv)

			if maintainRes.err == nil {
				newSess := maintainRes.newSess
				mbytes := maintainRes.mbytesAllowance

				// Track and maintain this as an active session.
				sess := &rtdtSession{
					rv:               newSess.rv,
					size:             newSess.size,
					publisherKey:     newSess.publisherKey,
					rtSess:           maintainRes.rtSess,
					bytesAllowance:   int64(mbytes * 1000000),
					mbytesPerRefresh: mbytes,
					ctx:              newSess.joinCtx,
					cancel:           newSess.cancelJoinCtx,
				}

				// If the appointment cookie was rotated while
				// waiting for the reply, use that one.
				reqCopy := *newSess.req
				if rotatedCookie := newSess.rotatedAppointCookie.Load(); rotatedCookie != nil {
					reqCopy.AppointCookie = *rotatedCookie
				}
				sess.appoint.Store(&reqCopy)

				sessions[newSess.rv] = sess
			}

		case leaveCmd := <-rtsm.leaveSessChan:
			// Caller requested to leave session.
			if _, ok := pendingLeave[leaveCmd.rv]; ok {
				// Already attempting to leave.
				leaveCmd.replyChan <- errors.New("already pending to leave session")
				continue loop
			}

			pendingSess := pendingAppoint[leaveCmd.rv]
			sess := sessions[leaveCmd.rv]
			if pendingSess != nil {
				rtsm.log.Debugf("Cancelling attempting to join pending live session %s", leaveCmd.rv)
				pendingSess.cancelJoinCtx()
				delete(pendingAppoint, leaveCmd.rv)
			}
			delete(pendingRefresh, leaveCmd.rv)
			if sess != nil {
				rtsm.log.Debugf("Stopping maintaining session %s live", leaveCmd.rv)

				// Command the rt client to leave session and
				// reply user when that is done.
				pendingLeave[leaveCmd.rv] = struct{}{}
				go func() {
					err := rtsm.rtc.LeaveSession(ctx, sess.rtSess)
					rtsm.leaveSessResChan <- leaveRtdtSessRes{
						cmd:  leaveCmd,
						sess: sess,
						err:  err,
					}
				}()
			} else {
				// No rt client instance found, reply user
				// directly.
				leaveCmd.replyChan <- nil
			}

		case leaveRes := <-rtsm.leaveSessResChan:
			// Cancel ongoing refreshes.
			leaveRes.sess.cancel()
			delete(pendingLeave, leaveRes.cmd.rv)

			// Only remove from list of kept sessions if leaving
			// did not error (ensures we don't lose track of
			// sessions that are still kept in rtc).
			if leaveRes.err == nil {
				delete(sessions, leaveRes.cmd.rv)
			}

			// Send reply to user.
			leaveRes.cmd.replyChan <- leaveRes.err

		case bytesWritten := <-rtsm.bytesWrittenChan:
			// Bytes were sent through an rtdt session.
			sess := sessions[*bytesWritten.rv]
			if sess != nil {
				_, alreadyPending := pendingRefresh[sess.rv]

				// Ignore possible refresh if not sending data
				// in the session.
				oldAllowance := sess.bytesAllowance
				sess.bytesAllowance -= int64(bytesWritten.n)
				if !alreadyPending && sess.bytesAllowance < refreshNeededLoMark && sess.publisherKey != nil {
					rtsm.log.Debugf("Session %s requires "+
						"allowance refresh (%d bytes)",
						bytesWritten.rv, sess.bytesAllowance)

					// Start refresh procedure.
					pendingRefresh[sess.rv] = brs != nil
					if brs != nil {
						go rtsm.attemptRefreshSessAllowance(ctx, brs, sess)
					}
				} else if oldAllowance%100000 > 50000 && sess.bytesAllowance%100000 < 50000 {
					rtsm.log.Tracef("Session %s has allowance %d (after deducting %d)",
						bytesWritten.rv, sess.bytesAllowance, bytesWritten.n)
				}
			} else {
				rtsm.log.Warnf("Got bytes written for unknown "+
					"session %s (%d bytes)", bytesWritten.rv,
					bytesWritten.n)
			}

		case refreshRes := <-rtsm.refreshSessResChan:
			// TODO: what to do on refresh errors? Leave session?
			// Close brserver connection? Alert user?
			delete(pendingRefresh, refreshRes.rv)
			if refreshRes.err != nil {
				rtsm.log.Errorf("Unable to refresh session %s: %v",
					refreshRes.rv, refreshRes.err)
			} else if sess := sessions[refreshRes.rv]; sess != nil {
				sess.bytesAllowance += int64(refreshRes.addAllowance)
			}

		case forceUnmaintainRV := <-rtsm.forceUnmaintainChan:
			delete(pendingRefresh, forceUnmaintainRV)
			delete(pendingAppoint, forceUnmaintainRV)
			delete(sessions, forceUnmaintainRV)
			rtsm.log.Debugf("Stopping to maintain session %s", forceUnmaintainRV)

		case rotateCookieCmd := <-rtsm.rotateAppointCookieChan:
			var rotateErr error
			if sess := sessions[rotateCookieCmd.rv]; sess != nil {
				rtsm.log.Debugf("Rotating appointment cookie for sess %s",
					rotateCookieCmd.rv)
				newAppoint := *(sess.appoint.Load()) // Make copy.
				newAppoint.AppointCookie = rotateCookieCmd.newAppointCookie
				sess.appoint.Store(&newAppoint)

				// What to do with the following? Drop request?
				// Replace request?
				if isPendingRefresh := pendingRefresh[rotateCookieCmd.rv]; isPendingRefresh {
					rtsm.log.Warnf("Pending refresh request while "+
						"rotating cookie for session %s", rotateCookieCmd.rv)
				}

			} else if pending := pendingAppoint[rotateCookieCmd.rv]; pending != nil {
				// What to do? Force refresh? Rely on refresh
				// by admin? Force user to quit and re-join?
				rtsm.log.Warnf("Pending first appointment while "+
					"rotating cookie for session %s", rotateCookieCmd.rv)
				cookie := slices.Clone(rotateCookieCmd.newAppointCookie)
				pending.rotatedAppointCookie.Store(&cookie)
			} else {
				rotateErr = ErrNoRtdtSessToRotateCookie
			}

			// Send reply to caller.
			rotateCookieCmd.replyChan <- rotateErr

		case <-ctx.Done():
			break loop
		}
	}

	close(rtsm.runDone)
	return ctx.Err()
}
