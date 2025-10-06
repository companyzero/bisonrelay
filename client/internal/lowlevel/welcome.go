package lowlevel

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
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

	// OnUnwelcomeError is called when a connection attempt is rejected
	// due to a protocol negotiation error. This usually means the client
	// needs to be upgraded. This is called concurrently to the connection
	// attempts, therefore it should not block for long.
	OnUnwelcomeError func(err error)
}

// serverCertPairs tracks individual pairs of outer and inner server
// certificates.
type serverCertPair struct {
	outerTlsCert   []byte
	innerServerPub zkidentity.PublicIdentity
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

	certMtx    sync.Mutex
	knownCerts []serverCertPair

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

// AddKnownServerCerts adds a set of servers as already known. Whenever the
// ConnKeeper connects to an unknown server, it will ask for user confirmation.
func (ck *ConnKeeper) AddKnownServerCerts(tlsCert []byte, spid zkidentity.PublicIdentity) {
	ck.certMtx.Lock()
	ck.knownCerts = append(ck.knownCerts, serverCertPair{outerTlsCert: tlsCert, innerServerPub: spid})
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
		err := makeUnwelcomeError(fmt.Sprintf("protocol version mismatch: "+
			"got %v wanted %v", wmsg.Version, rpc.ProtocolVersion))
		if ck.cfg.OnUnwelcomeError != nil {
			ck.cfg.OnUnwelcomeError(err)
		}
		return nil, err
	}

	if ck.log.Level() <= slog.LevelDebug {
		ck.log.Debugf("Server welcome properties:")
		for _, v := range wmsg.Properties {
			ck.log.Debugf("%v = %v %v", v.Key, v.Value, v.Required)
		}
	}

	// Deal with server properties
	policy := clientintf.ServerPolicy{
		// These default values may be removed once the corresponding
		// properties are made required.

		ExpirationDays:      rpc.PropExpirationDaysDefault,
		PushPaymentLifetime: rpc.PropPushPaymentLifetimeDefault,
		MaxPushInvoices:     rpc.PropMaxPushInvoicesDefault,

		// These two must be set at the same time.
		MaxMsgSizeVersion: rpc.PropMaxMsgSizeVersionDefault,
		MaxMsgSize:        rpc.MaxMsgSizeForVersion(rpc.PropMaxMsgSizeVersionDefault),

		PushPayRateMinMAtoms: rpc.PropPushPaymentRateMinMAtomsDefault,
		PushPayRateBytes:     rpc.PropPushPaymentRateBytesDefault,

		MilliAtomsPerRTSess:     1000,
		MilliAtomsPerUserRTSess: 1000,
		MilliAtomsGetCookie:     1000,
		MilliAtomsPerUserCookie: 100,
		MilliAtomsRTJoin:        1000,
		MilliAtomsRTPushRate:    100,
		RTPushRateMBytes:        1,
	}
	var (
		tagDepth   int64 = -1
		serverTime int64 = -1
		payScheme        = ""
		lnNode           = ""
	)

	puint := func(s string) (uint64, error) {
		return strconv.ParseUint(s, 10, 64)
	}

	for _, v := range wmsg.Properties {
		switch v.Key {
		case rpc.PropTagDepth:
			tagDepth, err = strconv.ParseInt(v.Value, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid tag depth: %v",
					err)
			}

		case rpc.PropServerTime:
			serverTime, err = strconv.ParseInt(v.Value, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid server time: %v",
					err)
			}

		case rpc.PropPaymentScheme:
			payScheme = v.Value

		case rpc.PropPushPaymentRate:
			ppr, err := strconv.ParseUint(v.Value, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid payment rate: %v",
					err)
			}
			policy.PushPayRateMAtoms = ppr

		case rpc.PropPushPaymentRateBytes:
			ppb, err := strconv.ParseUint(v.Value, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid payment rate bytes: %v", err)
			}
			policy.PushPayRateBytes = ppb

		case rpc.PropPushPaymentRateMinMAtoms:
			ppmma, err := strconv.ParseUint(v.Value, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid payment rate min matoms: %v", err)
			}
			policy.PushPayRateMinMAtoms = ppmma

		case rpc.PropSubPaymentRate:
			spr, err := strconv.ParseUint(v.Value, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid payment rate: %v",
					err)
			}
			policy.SubPayRate = spr

		case rpc.PropServerLNNode:
			lnNode = v.Value

		case rpc.PropExpirationDays:
			expd, err := strconv.ParseInt(v.Value, 10, 32)
			if err != nil {
				return nil, fmt.Errorf("invalid expiration days: %v", err)
			}
			policy.ExpirationDays = int(expd)

		case rpc.PropPushPaymentLifetime:
			ppl, err := strconv.ParseInt(v.Value, 10, 32)
			if err != nil {
				return nil, fmt.Errorf("invalid push payment lifetime: %v", err)
			}

			policy.PushPaymentLifetime = time.Duration(ppl) * time.Second

		case rpc.PropMaxPushInvoices:
			maxPushInvoices, err := strconv.ParseInt(v.Value, 10, 32)
			if err != nil {
				return nil, fmt.Errorf("invalid max push invoices: %v", err)
			}
			policy.MaxPushInvoices = int(maxPushInvoices)

		case rpc.PropMaxMsgSizeVersion:
			mmv, err := strconv.ParseUint(v.Value, 10, 32)
			if err != nil {
				return nil, fmt.Errorf("invalid max msg size version: %v", err)
			}
			policy.MaxMsgSizeVersion = rpc.MaxMsgSizeVersion(mmv)
			policy.MaxMsgSize = rpc.MaxMsgSizeForVersion(policy.MaxMsgSizeVersion)

		case rpc.PropPingLimit:
			pl, err := strconv.ParseInt(v.Value, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid ping limit: %v", err)
			}
			policy.PingLimit = time.Duration(pl) * time.Second

		case rpc.PropSuggestClientVersions:
			policy.ClientVersions = rpc.SplitSuggestedClientVersions(v.Value)

		case rpc.PropRTMAtomsPerSess:
			policy.MilliAtomsPerRTSess, err = puint(v.Value)
			if err != nil {
				return nil, fmt.Errorf("invalid RTMAtomsPerSess: %v", err)
			}

		case rpc.PropRTMAtomsPerUserSess:
			policy.MilliAtomsPerUserRTSess, err = puint(v.Value)
			if err != nil {
				return nil, fmt.Errorf("invalid RTMAtomsPerUserSess: %v", err)
			}

		case rpc.PropRTMAtomsGetCookie:
			policy.MilliAtomsGetCookie, err = puint(v.Value)
			if err != nil {
				return nil, fmt.Errorf("invalid RTMAtomsGetCookie: %v", err)
			}

		case rpc.PropRTMAtomsPerUserGetCookie:
			policy.MilliAtomsPerUserCookie, err = puint(v.Value)
			if err != nil {
				return nil, fmt.Errorf("invalid RTMAtomsPerUserGetCookie: %v", err)
			}

		case rpc.PropRTMAtomsJoin:
			policy.MilliAtomsRTJoin, err = puint(v.Value)
			if err != nil {
				return nil, fmt.Errorf("invalid RTMAtomsJoin: %v", err)
			}

		case rpc.PropRTMAtomsPushRate:
			policy.MilliAtomsRTPushRate, err = puint(v.Value)
			if err != nil {
				return nil, fmt.Errorf("invalid RTMAtomsPubPerUserMB: %v", err)
			}

		case rpc.PropRTPushRateMBytes:
			policy.RTPushRateMBytes, err = puint(v.Value)
			if err != nil {
				return nil, fmt.Errorf("invalid RTMAtomsPubPerUserMB: %v", err)
			}

		default:
			if v.Required {
				err := makeUnwelcomeError(fmt.Sprintf("unhandled server property: %v", v.Key))
				if ck.cfg.OnUnwelcomeError != nil {
					ck.cfg.OnUnwelcomeError(err)
				}
				return nil, err
			}

			ck.log.Warnf("Received unknown optional server "+
				"property %q with value %q", v.Key, v.Value)
		}
	}

	// Validate tag depth (max number of inflight, un-acked messages).
	const maxClientTagDepth = 32
	if tagDepth < 2 {
		return nil, fmt.Errorf("server did not provide tag depth")
	}
	if tagDepth > maxClientTagDepth {
		return nil, fmt.Errorf("tag depth higher then maximum: got %d, want %d", tagDepth,
			maxClientTagDepth)
	}

	// Max payment rate enforcement.
	const maxPushPaymentRate = uint64(rpc.PropPushPaymentRateDefault * 10)
	if policy.PushPayRateMAtoms > maxPushPaymentRate {
		return nil, fmt.Errorf("push payment rate higher then maximum. got %d "+
			"want %d", policy.PushPayRateMAtoms, maxPushPaymentRate)
	}
	const maxMinPayRateMAtoms = rpc.PropPushPaymentRateMinMAtomsDefault * 100
	if policy.PushPayRateMinMAtoms > maxMinPayRateMAtoms {
		return nil, fmt.Errorf("push payment rate min MAtoms higher then maximum. got %d "+
			"want %d", policy.PushPayRateMinMAtoms, maxMinPayRateMAtoms)
	}
	const maxSubPaymentRate = uint64(rpc.PropSubPaymentRateDefault * 10)
	if policy.SubPayRate > maxSubPaymentRate {
		return nil, fmt.Errorf("sub payment rate higher then maximum. got %d "+
			"want %d", policy.SubPayRate, maxSubPaymentRate)
	}

	// Push policy enforcement.
	const minPushPaymentLifetime = 15 * time.Minute
	if policy.PushPaymentLifetime < minPushPaymentLifetime {
		return nil, fmt.Errorf("push payment lifetime is lower than minimum. got %s "+
			"want %s", policy.PushPaymentLifetime, minPushPaymentLifetime)
	}
	if policy.MaxPushInvoices < 1 {
		return nil, fmt.Errorf("max push invoices %d < 1", policy.MaxPushInvoices)
	}

	// Realtime rates enforcement.
	const (
		maxRTPushRate       = 1000_000_000 // 0.01 DCR / MB
		maxRTJoinRate       = 1000_000_000 // 0.01 DCR
		maxRTGetCookieRate  = 1000_000_000 // 0.01 DCR
		maxRTNewSessionRate = 1000_000_000 // 0.01 DCR
	)
	if policy.MilliAtomsRTPushRate > maxRTPushRate {
		return nil, fmt.Errorf("realtime payment rate higher then maximum. got %d "+
			"want %d", policy.MilliAtomsRTPushRate, maxRTPushRate)
	}
	if policy.MilliAtomsRTJoin > maxRTJoinRate {
		return nil, fmt.Errorf("realtime join rate higher then maximum. got %d "+
			"want %d", policy.MilliAtomsRTJoin, maxRTJoinRate)
	}
	if policy.MilliAtomsGetCookie > maxRTGetCookieRate {
		return nil, fmt.Errorf("realtime get cookie rate higher then maximum. got %d "+
			"want %d", policy.MilliAtomsGetCookie, maxRTGetCookieRate)
	}
	if policy.MilliAtomsPerUserCookie > maxRTGetCookieRate {
		return nil, fmt.Errorf("realtime per user get cookie rate higher then maximum. got %d "+
			"want %d", policy.MilliAtomsPerUserCookie, maxRTGetCookieRate)
	}
	if policy.MilliAtomsPerRTSess > maxRTNewSessionRate {
		return nil, fmt.Errorf("realtime new session rate higher then maximum. got %d "+
			"want %d", policy.MilliAtomsPerRTSess, maxRTNewSessionRate)
	}
	if policy.MilliAtomsPerUserRTSess > maxRTNewSessionRate {
		return nil, fmt.Errorf("realtime per user rate higher then maximum. got %d "+
			"want %d", policy.MilliAtomsPerUserRTSess, maxRTNewSessionRate)
	}

	// server time
	if serverTime == -1 {
		return nil, fmt.Errorf("server did not provide time")
	}
	ck.log.Debugf("Server provided time %v", time.Unix(serverTime, 0).Format(time.RFC3339))

	// message size
	if policy.MaxMsgSize == 0 {
		err := makeUnwelcomeError("server did not send a supported max msg size version")
		if ck.cfg.OnUnwelcomeError != nil {
			ck.cfg.OnUnwelcomeError(err)
		}
		return nil, err

	}
	if kx, ok := kx.(*session.KX); ok {
		kx.MaxMessageSize = policy.MaxMsgSize
	}

	// Double check a message of the max valid size is payable.
	_, err = policy.CalcPushCostMAtoms(int(policy.MaxMsgSize))
	if err != nil {
		return nil, fmt.Errorf("invalid combination of push pay rates and max msg size: %v", err)
	}

	// Expiration days.
	if policy.ExpirationDays < 1 {
		return nil, fmt.Errorf("server provided expiration days %d < 1",
			policy.ExpirationDays)
	}

	// Determine pay scheme w/ server (LN, on-chain, etc).
	var pc clientintf.PaymentClient
	switch payScheme {
	case rpc.PaySchemeFree:
		// Fallthrough and accept free pay scheme.
		pc = clientintf.FreePaymentClient{}
	default:
		// Only proceed if we're configured to use the same payment
		// scheme as server.
		if payScheme != ck.cfg.PC.PayScheme() {
			return nil, fmt.Errorf("mismatched payment scheme -- "+
				"client: %s, server: %s", ck.cfg.PC.PayScheme(),
				payScheme)
		}
		pc = ck.cfg.PC
	}

	// If the server specified a ping limit and the user has enabled
	// pinging, determine if we need to adjust or error out compared to the
	// user's expected ping interval.
	pingInterval := ck.cfg.PingInterval
	userExpectedPingLimit := pingInterval + pingInterval/4
	if policy.PingLimit > time.Second && policy.PingLimit < userExpectedPingLimit && pingInterval > 0 {
		// When the server provided ping interval is less than 30
		// seconds, error out unless our own ping interval is lower.
		// This prevents the server from requesting pings too often.
		if policy.PingLimit < time.Second*30 && pingInterval > policy.PingLimit {
			return nil, fmt.Errorf("%w: server specified a ping limit of %s "+
				"which is too short given our ping interval of %s",
				errShortPingLimit, policy.PingLimit, pingInterval)
		}

		// Otherwise, use a ping interval that should ensure we don't
		// get disconnected too often.
		pingInterval = policy.PingLimit * 3 / 4
		ck.log.Warnf("Reducing ping interval to %s", pingInterval)
	} else if policy.PingLimit < time.Second {
		policy.PingLimit = rpc.PropPingLimitDefault
	}

	// Reduce the tagstack depth by 1 to ensure the last tag is left for
	// sending pings.
	tagDepth -= 1

	// Return a new server session.
	sess := newServerSession(conn, kx, tagDepth, ck.log)
	sess.pc = pc
	sess.payScheme = payScheme
	sess.lnNode = lnNode
	sess.pingInterval = pingInterval
	sess.pushedRoutedMsgsHandler = ck.cfg.PushedRoutedMsgsHandler
	sess.logPings = ck.cfg.LogPings
	sess.policy = policy

	ck.log.Infof("Connected to server %s", conn.RemoteAddr())

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

	// Verify the server has a TLS cert.
	if len(tlsState.PeerCertificates) < 1 {
		return fail(errNoPeerTLSCert)
	}
	newCert := tlsState.PeerCertificates[0].Raw

	// Request the inner server public identity.
	ck.log.Debugf("Fetching inner server public ID")
	newSpid, err := ck.fetchServerPublicID(conn)
	if err != nil {
		return fail(err)
	}

	// Determine if the server is already known.
	needsConfirm := true
	ck.certMtx.Lock()
	for _, certPair := range ck.knownCerts {
		isKnown := bytes.Equal(certPair.outerTlsCert, newCert) &&
			reflect.DeepEqual(certPair.innerServerPub, newSpid)
		if isKnown {
			needsConfirm = false
			break
		}
	}
	ck.certMtx.Unlock()

	if needsConfirm {
		ck.log.Debugf("Requiring certificate confirmation for server connection")

		// Certs need confirmation. Ask it from user.
		if err := ck.cfg.CertConf(ctx, tlsState, &newSpid); err != nil {
			return fail(err)
		}

		// User confirmed certs. Store them, so reconnection attempts
		// to the same server don't require reconfirmations.
		ck.log.Debugf("Server connection confirmed")
		ck.certMtx.Lock()
		ck.knownCerts = append(ck.knownCerts, serverCertPair{outerTlsCert: newCert, innerServerPub: newSpid})
		ck.certMtx.Unlock()
	} else {
		ck.log.Tracef("Already known server inner and outer certs")
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
