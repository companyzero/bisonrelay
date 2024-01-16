package lowlevel

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/companyzero/bisonrelay/rpc"
	"github.com/companyzero/bisonrelay/session"
	"github.com/companyzero/bisonrelay/zkidentity"
	"github.com/decred/slog"
)

type ConnKeeperCfg struct {
	PC             clientintf.PaymentClient
	Dialer         clientintf.Dialer
	CertConf       clientintf.CertConfirmer
	PingInterval   time.Duration
	ReconnectDelay time.Duration
	Log            slog.Logger
	LogPings       bool

	// Passed to created serverSession instances (see there for reference).
	PushedRoutedMsgsHandler func(msg *rpc.PushRoutedMessage) error
}

// ConnKeeper maintains an open connection to a server. Whenever the connection
// to the server closes, it attempts to re-connect. Only a single connection is
// kept online at any one time.
//
// Fully kx'd server sessions are emitted via NextSession().
type ConnKeeper struct {
	// The following fields should only be set during setup of this struct
	// and are not safe for concurrent modification.
	cfg ConnKeeperCfg

	sessionChan   chan clientintf.ServerSessionIntf
	log           slog.Logger
	skipPerformKX bool // Only set in some unit tests.

	certMtx sync.Mutex
	tlsCert []byte
	spid    zkidentity.PublicIdentity // server public id

	keepOnlineChan chan bool
}

func NewConnKeeper(cfg ConnKeeperCfg) *ConnKeeper {
	log := slog.Disabled
	if cfg.Log != nil {
		log = cfg.Log
	}
	return &ConnKeeper{
		cfg:            cfg,
		sessionChan:    make(chan clientintf.ServerSessionIntf),
		log:            log,
		keepOnlineChan: make(chan bool),
	}
}

// NextSession blocks until a session is available or the context is canceled.
// Note this returns nil in two situations: if the last session failed and is
// now offline or if the context is canceled.
func (ck *ConnKeeper) NextSession(ctx context.Context) clientintf.ServerSessionIntf {
	select {
	case sess := <-ck.sessionChan:
		return sess
	case <-ctx.Done():
		return nil
	}
}

// SetKnownServerID sets the known server certs as the passed ones. Whenever we
// connect to the server and the certs are different then these, we request
// confirmation from the user.
func (ck *ConnKeeper) SetKnownServerID(tlsCert []byte, spid zkidentity.PublicIdentity) {
	ck.certMtx.Lock()
	ck.tlsCert = tlsCert
	ck.spid = spid
	ck.certMtx.Unlock()
}

// RemainOffline asks the ConnKeeper to disconnect from the current session (if
// there is one) and to remain offline until GoOnline() is called.
func (ck *ConnKeeper) RemainOffline() {
	ck.keepOnlineChan <- false
}

// GoOnline instructs the ConnKeeper to keep attempting connections to the
// server.
func (ck *ConnKeeper) GoOnline() {
	ck.keepOnlineChan <- true
}

// fetchServerPublicID requests the server identity and waits for the server
// response in the given conn.
func (ck *ConnKeeper) fetchServerPublicID(conn clientintf.Conn) (zkidentity.PublicIdentity, error) {
	var pid zkidentity.PublicIdentity

	// tell remote we want its public identity
	err := json.NewEncoder(conn).Encode(rpc.InitialCmdIdentify)
	if err != nil {
		return pid, err
	}

	// get server identity
	err = json.NewDecoder(conn).Decode(&pid)
	if err != nil {
		return pid, err
	}

	ck.log.Debugf("Fetched server public ID %s", pid.Fingerprint())

	return pid, nil
}

// attemptWelcome attempts to perform the welcome stage of server connection on
// a connected and KX'd server.
//
// If this succeeds, it returns the fully formed server session.
func (ck *ConnKeeper) attemptWelcome(conn clientintf.Conn, kx msgReaderWriter) (*serverSession, error) {
	var (
		command rpc.Message
		wmsg    rpc.Welcome
	)

	// Read command.
	b, err := kx.Read()
	if err != nil {
		return nil, fmt.Errorf("failed to read from kx reader: %v", err)
	}

	// Unmarshal header.
	br := bytes.NewReader(b)
	dec := json.NewDecoder(br)
	err = dec.Decode(&command)
	if err != nil {
		return nil, fmt.Errorf("unmarshal Welcome header failed: %w", err)
	}

	switch command.Command {
	case rpc.SessionCmdWelcome:
		// fallthrough
	default:
		return nil, fmt.Errorf("expected (un)welcome command")
	}

	// Unmarshal payload.
	err = dec.Decode(&wmsg)
	if err != nil {
		return nil, fmt.Errorf("unmarshal Welcome payload failed")
	}

	if wmsg.Version != rpc.ProtocolVersion {
		return nil, fmt.Errorf("protocol version mismatch: "+
			"got %v wanted %v",
			wmsg.Version,
			rpc.ProtocolVersion)
	}

	if ck.log.Level() <= slog.LevelDebug {
		ck.log.Debugf("Server welcome properties:")
		for _, v := range wmsg.Properties {
			ck.log.Debugf("%v = %v %v", v.Key, v.Value, v.Required)
		}
	}

	// Deal with server properties
	var (
		td     int64  = -1
		pt     int64  = -1
		ps     string = ""
		ppr    uint64 = 0
		spr    uint64 = 0
		lnNode string = ""

		// TODO: modify to zero once clients are updated to force
		// server to send an appropriate value.
		expd int64 = rpc.PropExpirationDaysDefault

		pushPaymentLifetime int64 = rpc.PropPushPaymentLifetimeDefault
		maxPushInvoices     int64 = rpc.PropMaxPushInvoicesDefault

		maxMsgSizeVersion rpc.MaxMsgSizeVersion = rpc.MaxMsgSizeV0
	)

	for _, v := range wmsg.Properties {
		switch v.Key {
		case rpc.PropTagDepth:
			td, err = strconv.ParseInt(v.Value, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid tag depth: %v",
					err)
			}

		case rpc.PropServerTime:
			pt, err = strconv.ParseInt(v.Value, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid server time: %v",
					err)
			}

		case rpc.PropPaymentScheme:
			ps = v.Value

		case rpc.PropPushPaymentRate:
			ppr, err = strconv.ParseUint(v.Value, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid payment rate: %v",
					err)
			}

		case rpc.PropSubPaymentRate:
			spr, err = strconv.ParseUint(v.Value, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid payment rate: %v",
					err)
			}

		case rpc.PropServerLNNode:
			lnNode = v.Value

		case rpc.PropExpirationDays:
			expd, err = strconv.ParseInt(v.Value, 10, 32)
			if err != nil {
				return nil, fmt.Errorf("invalid expiration days: %v", err)
			}

		case rpc.PropPushPaymentLifetime:
			pushPaymentLifetime, err = strconv.ParseInt(v.Value, 10, 32)
			if err != nil {
				return nil, fmt.Errorf("invalid push payment lifetime: %v", err)
			}

		case rpc.PropMaxPushInvoices:
			maxPushInvoices, err = strconv.ParseInt(v.Value, 10, 32)
			if err != nil {
				return nil, fmt.Errorf("invalid max push invoices: %v", err)
			}

		case rpc.PropMaxMsgSizeVersion:
			var mmv uint64
			mmv, err = strconv.ParseUint(v.Value, 10, 32)
			if err != nil {
				return nil, fmt.Errorf("invalid max msg size version: %v", err)
			}
			maxMsgSizeVersion = rpc.MaxMsgSizeVersion(mmv)

		default:
			if v.Required {
				errMsg := fmt.Sprintf("unhandled server property: %v", v.Key)
				return nil, makeUnwelcomeError(errMsg)
			}

			ck.log.Warnf("Received unknown optional server "+
				"property %q with value %q", v.Key, v.Value)
		}
	}

	// maxClientTagDepth is the maximum depth of the tagstack that a client
	// accepts (i.e. max number of inflight, un-acked messages).
	const maxClientTagDepth = 32

	// tag depth
	if td < 2 {
		return nil, fmt.Errorf("server did not provide tag depth")
	}
	if td > maxClientTagDepth {
		return nil, fmt.Errorf("tag depth higher then maximum: got %d, want %d", td,
			maxClientTagDepth)
	}

	// Max payment rate enforcement.
	const maxPushPaymentRate = uint64(rpc.PropPushPaymentRateDefault * 10)
	if ppr > maxPushPaymentRate {
		return nil, fmt.Errorf("push payment rate higher then maximum. got %d "+
			"want %d", ppr, maxPushPaymentRate)
	}
	const maxSubPaymentRate = uint64(rpc.PropSubPaymentRateDefault * 10)
	if spr > maxSubPaymentRate {
		return nil, fmt.Errorf("sub payment rate higher then maximum. got %d "+
			"want %d", spr, maxSubPaymentRate)
	}

	// Push policy enforcement.
	minPushPaymentLifetime := int64(15 * 60) // 15 minutes.
	if pushPaymentLifetime < minPushPaymentLifetime {
		return nil, fmt.Errorf("push payment lifetime is lower than minimum. got %d "+
			"want %d", pushPaymentLifetime, minPushPaymentLifetime)
	}
	if maxPushInvoices < 1 {
		return nil, fmt.Errorf("max push invoices %d < 1", maxPushInvoices)
	}

	// server time
	if pt == -1 {
		return nil, fmt.Errorf("server did not provide time")
	}
	ck.log.Debugf("Server provided time %v", time.Unix(pt, 0).Format(time.RFC3339))

	// message size
	maxMsgSize := rpc.MaxMsgSizeForVersion(maxMsgSizeVersion)
	if maxMsgSize == 0 {
		return nil, fmt.Errorf("server did not send a supported max msg size version")

	}
	if kx, ok := kx.(*session.KX); ok {
		kx.MaxMessageSize = maxMsgSize
	}

	// Expiration days.
	if expd < 1 {
		return nil, fmt.Errorf("server provided expiration days %d < 1", expd)
	}

	// Determine pay scheme w/ server (LN, on-chain, etc).
	var pc clientintf.PaymentClient
	switch ps {
	case rpc.PaySchemeFree:
		// Fallthrough and accept free pay scheme.
		pc = clientintf.FreePaymentClient{}
	default:
		// Only proceed if we're configured to use the same payment
		// scheme as server.
		if ps != ck.cfg.PC.PayScheme() {
			return nil, fmt.Errorf("mismatched payment scheme -- "+
				"client: %s, server: %s", ck.cfg.PC.PayScheme(), ps)
		}
		pc = ck.cfg.PC
	}

	// Reduce the tagstack depth by 1 to ensure the last tag is left for
	// sending pings.
	td -= 1

	// Return a new server session.
	sess := newServerSession(conn, kx, td, ck.log)
	sess.pc = pc
	sess.payScheme = ps
	sess.pushPayRate = ppr
	sess.subPayRate = spr
	sess.lnNode = lnNode
	sess.pingInterval = ck.cfg.PingInterval
	sess.pushedRoutedMsgsHandler = ck.cfg.PushedRoutedMsgsHandler
	sess.expirationDays = int(expd)
	sess.logPings = ck.cfg.LogPings
	sess.policy = clientintf.ServerPolicy{
		PushPaymentLifetime: time.Duration(pushPaymentLifetime) * time.Second,
		MaxPushInvoices:     int(maxPushInvoices),
		MaxMsgSizeVersion:   maxMsgSizeVersion,
		MaxMsgSize:          maxMsgSize,
	}

	ck.log.Infof("Connected to server %s",
		conn.RemoteAddr())

	return sess, nil
}

// attemptServerKX switches the connection to a fully kx'd session.
func (ck *ConnKeeper) attemptServerKX(conn clientintf.Conn, spid *zkidentity.PublicIdentity) (*session.KX, error) {
	// Create the KX session w/ the server.
	err := json.NewEncoder(conn).Encode(rpc.InitialCmdSession)
	if err != nil {
		return nil, err
	}

	// Session with server and use a default msgSize.
	kx := &session.KX{
		Conn:           conn,
		MaxMessageSize: rpc.MaxMsgSizeForVersion(rpc.MaxMsgSizeV0),
		TheirPublicKey: &spid.Key,
	}
	err = kx.Initiate()
	if err != nil {
		return nil, makeKxError(err)
	}

	ck.log.Debug("Server KX stage performed")

	return kx, nil
}

// attemptConn attempts a single connection to the server.
func (ck *ConnKeeper) attemptConn(ctx context.Context) (*serverSession, error) {
	// Pre-Session Phase.

	// Attempting to dial should take at most 30 seconds.
	dialCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	ck.log.Tracef("Attempting to call dialer")
	conn, tlsState, err := ck.cfg.Dialer(dialCtx)
	if err != nil {
		return nil, err
	}

	if conn == nil {
		return nil, errors.New("invalid dialer returned nil conn")
	}

	// Helper to ensure we disconnect from server when returning with an
	// error.
	fail := func(err error) (*serverSession, error) {
		newServerSession(conn, nil, 0, ck.log).close()
		return nil, err
	}

	ck.certMtx.Lock()
	oldCert := ck.tlsCert
	oldSpid := ck.spid
	ck.certMtx.Unlock()

	// Verify the server has a TLS cert.
	if len(tlsState.PeerCertificates) < 1 {
		return fail(errNoPeerTLSCert)
	}
	newCert := tlsState.PeerCertificates[0].Raw

	// Request the inner server public identity if we don't have it yet.
	//
	// Maybe we should always request and compare to ensure a clearer error
	// in case of KX failure?
	ck.log.Debugf("Unknown server pid. Fetching it.")
	newSpid, err := ck.fetchServerPublicID(conn)
	if err != nil {
		return fail(err)
	}

	needsConfirm := !bytes.Equal(newCert, oldCert) || oldSpid != newSpid
	if needsConfirm {
		// Certs need confirmation. Ask it from user.
		if err := ck.cfg.CertConf(ctx, tlsState, &newSpid); err != nil {
			return fail(err)
		}

		// User confirmed certs. Store them, so reconnection attempts
		// to the same server don't require reconfirmations.
		ck.certMtx.Lock()
		ck.tlsCert = newCert
		ck.spid = newSpid
		ck.certMtx.Unlock()
	}

	// Session Phase.

	// Return early during some unit tests to avoid having to create a
	// valid kx server.
	if ck.skipPerformKX {
		ck.log.Warn("Skipping server KX setup stage")
		kx := blockingMsgReaderWriter{ctx: ctx}
		return newServerSession(conn, kx, 0, ck.log), nil
	}

	kx, err := ck.attemptServerKX(conn, &newSpid)
	if err != nil {
		return fail(err)
	}

	// Welcome Phase.
	sess, err := ck.attemptWelcome(conn, kx)
	if err != nil {
		return fail(err)
	}

	return sess, nil
}

// runSession runs the given session. Any errors are sent to sessErrChan.
func (ck *ConnKeeper) runSession(ctx context.Context, sess *serverSession, sessErrChan chan error) {
	// Alert callers of the new session.
	go func() {
		time.Sleep(5 * time.Millisecond) // Time for the sess to run.
		ck.sessionChan <- sess
	}()

	// Run the session until if fails or the context is canceled.
	err := sess.Run(ctx)

	// Alert the session is offline.
	go func() { ck.sessionChan <- nil }()

	// Send error.
	select {
	case sessErrChan <- err:
	case <-ctx.Done():
	}
}

// Run runs the services of this conn keeper.
func (ck *ConnKeeper) Run(ctx context.Context) error {
	sessErrChan := make(chan error)
	var sess *serverSession
	var err error
	keepOnline := true
	var delayChan <-chan time.Time
	delayNextAttempt := func() {
		delayChan = time.After(ck.cfg.ReconnectDelay)
		ck.log.Debugf("Delaying reconnect attempt by %s", ck.cfg.ReconnectDelay)
	}
	firstConn := make(chan struct{}, 1)
	firstConn <- struct{}{}

	ck.log.Trace("Starting conn keeper")
nextAction:
	for {
		select {
		case <-firstConn:
			// First connection attempt.
			firstConn = nil

		case err := <-sessErrChan:
			// Current session errored.
			if !errors.Is(err, errSessRequestedClose) {
				ck.log.Errorf("Connection to server failed due to %v", err)
				delayNextAttempt()
			} else {
				ck.log.Infof("Disconnected from server as requested")
			}
			sess = nil
			continue nextAction

		case <-delayChan:
			// Time to try again.
			delayChan = nil

		case keepOnline = <-ck.keepOnlineChan:
			// External command to change connection status.
			if (keepOnline && sess != nil) || (!keepOnline && sess == nil) {
				// Nothing to do.
				continue nextAction
			}

			if !keepOnline {
				// Request to go offline.
				ck.log.Debugf("Requested to remain offline")
				go sess.RequestClose(errSessRequestedClose)
			} else {
				ck.log.Debugf("Requested to go online")
			}

			// Continue executing, we'll either attempt a
			// connection or not.

		case <-ctx.Done():
			break nextAction
		}

		if !keepOnline {
			continue nextAction
		}

		if sess != nil {
			// Disconnect if already connected somewhere.
			go sess.RequestClose(errSessRequestedClose)
		}

		// Attempt connection.
		sess, err = ck.attemptConn(ctx)
		if errors.Is(err, context.Canceled) && canceled(ctx) {
			// Context canceled, return from here.
			break nextAction
		} else if err != nil {
			ck.log.Errorf("Error connecting to server: %v", err)
			delayNextAttempt()
			continue nextAction
		}

		go ck.runSession(ctx, sess, sessErrChan)
	}

	// Context is done.
	ck.log.Debug("Shutting down connection to server")
	return ctx.Err()
}
