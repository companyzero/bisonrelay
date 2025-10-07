package rtdtclient

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/companyzero/bisonrelay/rpc"
	"github.com/companyzero/bisonrelay/zkidentity"
	"github.com/companyzero/sntrup4591761"
	"github.com/decred/slog"
	"github.com/puzpuzpuz/xsync/v3"
)

// AudioSpeechStream defines the interface for processing audio data received
// from a remote peer.
type AudioSpeechStream interface {
	// Input adds an audio input packet with the given data and timestamp
	// to processing.
	Input(data []byte, ts uint32)

	// MarkInputDone is used to signal that the stream will not receive any
	// more data.
	MarkInputDone(ctx context.Context)
}

// StreamHandler is the signature for callbacks to handle data received
// in a stream.
type StreamHandler func(sess *Session, enc *rpc.RTDTFramedPacket, plain *rpc.RTDTDataPacket) error

// NewPeerCallback is the signature for callbacks to handle new peers detected
// in a session. The callback should return the signature verification key and
// the key that should be used to decrypt remote peer data.
type NewPeerCallback func(id rpc.RTDTPeerID, sessRV *zkidentity.ShortID) (*zkidentity.FixedSizeEd25519PublicKey, *zkidentity.FixedSizeSymmetricKey)

// BytesWrittenCallback is the signature for callbacks to outbound data sent.
type BytesWrittenCallback func(sess *Session, n int)

// AudioStreamIniter is the signature for callbacks that initialize new audio
// streams.
type AudioStreamIniter func(sess *Session, peer rpc.RTDTPeerID) AudioSpeechStream

// SessionPeerListUpdated is the signature for callbacks to handle updates to
// the members list.
type SessionPeerListUpdated func(sess *Session)

// KickedFromSessionCallback is the signature for callbacks that are triggered
// when the local client is kicked from a session.
type KickedFromSessionCallback func(sess *Session, banDuration time.Duration)

// PingRTTCalculated is the signature for callbacks triggered when the RTT to
// the server is calculated.
type PingRTTCalculated func(addr net.UDPAddr, rtt time.Duration)

// PacketIOCallback is the signature for callbacks triggered when a packet is
// successfully sent or received by the client.
type PacketIOCallback func(n int)

// config holds client config data.
type config struct {
	log                    slog.Logger
	connCtx                context.Context
	randomStreamHandler    StreamHandler
	audioStreamHandler     StreamHandler
	chatStreamHandler      StreamHandler
	readRoutines           int
	newPeerCallback        NewPeerCallback
	bytesWrittenCallback   BytesWrittenCallback
	kickedCallback         KickedFromSessionCallback
	pingRTTCalculated      PingRTTCalculated
	handshakeRetryInterval time.Duration
	pingInterval           time.Duration
	readTimeout            time.Duration
	ignoreUnkeyedPeers     bool

	sessPeerListUpdatedCallback SessionPeerListUpdated

	// These control the max number of tries and max time to wait before
	// timing out an attempt to leave a session.
	maxLeaveTries     int
	leaveReplyTimeout time.Duration

	// These control the max number of tries and max time to wait before
	// timing out a kick attempt.
	maxKickTries     int
	kickReplyTimeout time.Duration

	// These control how publisher peers are determined to be stalled.
	peerStallInterval      time.Duration
	peerStallCheckInterval time.Duration

	// These control the admin request to rotate cookies.
	maxRotateTries      int
	rotateReplyInterval time.Duration

	// pingOnConnect is true if conns should send a ping immediately after
	// connecting.
	pingOnConnect bool

	// These are callbacks for packet IO.
	pktOutCallback PacketIOCallback
	pktInCallback  PacketIOCallback
}

// defaultConfig initializes the default config for a client.
func defaultConfig() config {
	// Aggressive ping for better disconnection detection.
	pingInterval := rpc.RTDTDefaultMinPingInterval * 2
	if pingInterval > rpc.RTDTMaxPingInterval {
		// Should never happen (protect against future modification
		// making these constants invalid).
		panic("invalid constant values definition")
	}

	// Read timeout short enough for 3 pings + reply.
	readTimeout := pingInterval*3 + 2*time.Second
	if readTimeout > rpc.RTDTMaxPingInterval {
		readTimeout = rpc.RTDTMaxPingInterval
	}

	return config{
		log:                    slog.Disabled,
		connCtx:                context.Background(),
		pingInterval:           pingInterval,
		readTimeout:            readTimeout,
		readRoutines:           1,
		handshakeRetryInterval: 5 * time.Second,
		maxLeaveTries:          3,
		leaveReplyTimeout:      5 * time.Second,
		maxKickTries:           3,
		kickReplyTimeout:       5 * time.Second,
		peerStallInterval:      5 * time.Second,
		peerStallCheckInterval: 1 * time.Second,
		maxRotateTries:         3,
		rotateReplyInterval:    5 * time.Second,
	}
}

// Option is a functional client config option.
type Option func(c *config)

// WithLogger sets up the client to use the logger. Logger MUST NOT be nil.
func WithLogger(l slog.Logger) Option {
	return func(c *config) {
		c.log = l
	}
}

// WithRandomStreamHandler sets the handler of data from the random stream.
func WithRandomStreamHandler(handler StreamHandler) Option {
	return func(c *config) {
		c.randomStreamHandler = handler
	}
}

// WithPerConnReadRoutines sets the number of goroutines to use to read data
// from remote connections.
func WithPerConnReadRoutines(i int) Option {
	if i < 1 {
		panic("number of read routines cannot be < 1")
	}
	return func(c *config) {
		c.readRoutines = i
	}
}

// WithNewPeerCallback sets the callback called when new peers are detected in
// a session.
func WithNewPeerCallback(cb NewPeerCallback) Option {
	return func(c *config) {
		c.newPeerCallback = cb
	}
}

// WithBytesWrittenCallback sets the callback called when data bytes are
// written to the remote connection.
func WithBytesWrittenCallback(cb BytesWrittenCallback) Option {
	return func(c *config) {
		c.bytesWrittenCallback = cb
	}
}

// WithAudioStreamHandler defines the handler of audio data.
func WithAudioStreamHandler(handler StreamHandler) Option {
	return func(c *config) {
		c.audioStreamHandler = handler
	}
}

// WithChatStreamHandler defines the handler of chat messages.
func WithChatStreamHandler(handler StreamHandler) Option {
	return func(c *config) {
		c.chatStreamHandler = handler
	}
}

// WithIgnoreUnkeyedPeers determines whether the client should ignore data from
// peers for which it has no encryption key.
func WithIgnoreUnkeyedPeers(ignore bool) Option {
	return func(c *config) {
		c.ignoreUnkeyedPeers = ignore
	}
}

// WithSessionPeerListUpdated defines a function to be called when updates to
// the list of members in a session are received.
func WithSessionPeerListUpdated(cb SessionPeerListUpdated) Option {
	return func(c *config) {
		c.sessPeerListUpdatedCallback = cb
	}
}

// WithConnContext defines the context to run conns in.
func WithConnContext(ctx context.Context) Option {
	return func(c *config) {
		c.connCtx = ctx
	}
}

// withReadTimeout sets the read timeout for the client. Only modified from
// default in some tests.
func withReadTimeout(d time.Duration) Option {
	return func(c *config) {
		c.readTimeout = d
	}
}

// WithKickedCallback sets the callback called when the local client is kicked
// from a session.
func WithKickedCallback(f KickedFromSessionCallback) Option {
	return func(c *config) {
		c.kickedCallback = f
	}
}

// WithPingRTTCalculated sets the callback called when the roundtrip time (RTT)
// to a server is determined.
func WithPingRTTCalculated(f PingRTTCalculated) Option {
	return func(c *config) {
		c.pingRTTCalculated = f
	}
}

// WithPingOnConnect sets the client to send a ping immediately after connecting
// to the server, in order to get a latency check done.
func WithPingOnConnect() Option {
	return func(c *config) {
		c.pingOnConnect = true
	}
}

// WithPacketIOCallbacks sets the callbacks for successful packet IO.
func WithPacketIOCallbacks(pktInCb, pktOutCb PacketIOCallback) Option {
	return func(c *config) {
		c.pktInCallback = pktInCb
		c.pktOutCallback = pktOutCb
	}
}

// withPingInterval sets the ping interval for the client. Only modified in
// some tests.
func withPingInterval(d time.Duration) Option {
	return func(c *config) {
		c.pingInterval = d
	}
}

// withLeaveAttemptOptions sets the options for leave session attempts.
func withLeaveAttemptOptions(maxTries int, replyTimeout time.Duration) Option {
	return func(c *config) {
		c.maxLeaveTries = maxTries
		c.leaveReplyTimeout = replyTimeout
	}
}

// withKickAttemptOptions sets the options for kick attempts.
func withKickAttemptOptions(maxTries int, replyTimeout time.Duration) Option {
	return func(c *config) {
		c.maxKickTries = maxTries
		c.kickReplyTimeout = replyTimeout
	}
}

// Client is an RTDT session multiplexer. It can be used to manage multiple
// sessions, in different servers.
type Client struct {
	cfg config
	log slog.Logger

	conns *xsync.MapOf[string, []*conn]
}

// newClient initializes a new client.
func newClient(cfg config) (*Client, error) {
	s := &Client{
		cfg:   cfg,
		log:   cfg.log,
		conns: xsync.NewMapOf[string, []*conn](),
	}
	return s, nil
}

// New creates a new RTDT client with the given options.
func New(opts ...Option) (*Client, error) {
	cfg := defaultConfig()
	for _, o := range opts {
		o(&cfg)
	}

	return newClient(cfg)
}

// ServerSharedKeyGenerator is the signature for the callback to return the
// server's client-server public key and a shared key for encrypting
// client-server comms.
type ServerSharedKeyGenerator func() (*sntrup4591761.Ciphertext, *sntrup4591761.SharedKey)

// SessionConfig defines the config of a new session.
type SessionConfig struct {
	// ServerAddr is the address of this session's server.
	ServerAddr *net.UDPAddr

	// SessionKeyGen is the callback to generate the client-server
	// transport encryption key.
	SessionKeyGen ServerSharedKeyGenerator

	// LocalID is the peer id of the local client in the session.
	LocalID rpc.RTDTPeerID

	// PublisherKey is the local client's publisher key to E2E encrypt
	// data in this session. If nil, data is sent in plaintext.
	PublisherKey *zkidentity.FixedSizeSymmetricKey

	// SigKey is the local client's private key to use to sign data. If nil,
	// data sent in this session won't be authenticated.
	SigKey *zkidentity.FixedSizeEd25519PrivateKey

	// RV is the full session RV identifier.
	RV *zkidentity.ShortID

	// JoinCookie is the cookie to send to the server when joining the
	// sesson.
	JoinCookie []byte
}

// NewSession connects to and creates a new RTDT session based on the config.
// The session (and any other sessions made to the same server) run until the
// context is canceled or LeaveSession() is called.
func (c *Client) NewSession(ctx context.Context, sessCfg SessionConfig) (*Session, error) {
	conn, _, err := c.connToAddrAndId(ctx, sessCfg.ServerAddr,
		sessCfg.SessionKeyGen, sessCfg.LocalID)
	if err != nil {
		return nil, err
	}

	c.log.Infof("Creating session %s to server %s", sessCfg.LocalID, sessCfg.ServerAddr)
	sess := newSession(conn, sessCfg.LocalID, sessCfg.SigKey,
		sessCfg.PublisherKey, sessCfg.RV)
	err = conn.joinSession(ctx, sess, sessCfg.JoinCookie)
	if err != nil {
		return nil, err
	}
	return sess, nil
}

// LeaveSession makes the client leave the given session.
func (c *Client) LeaveSession(ctx context.Context, sess *Session) error {
	// Try multiple times to leave the session.
	for i := 0; i < c.cfg.maxLeaveTries; i++ {
		tryCtx, cancel := context.WithTimeout(ctx, c.cfg.leaveReplyTimeout)
		c.log.Debugf("Attempting to leave session %s (attempt #%d)",
			sess.localID, i)
		err := sess.conn.leaveSession(tryCtx, sess)
		cancel()
		if err == nil {
			// Success. remove conn if there are no more sessions.
			sessCount, pendingCount := c.removeConnIfEmpty(sess.conn)
			c.log.Debugf("Remaining in conn %s: sessions %d, pending %d",
				sess.conn.cAddr, sessCount, pendingCount)
			c.log.Infof("Left session %s from conn %s",
				sess.localID, sess.conn.cAddr)
			return nil
		} else {
			c.log.Tracef("Unable to leave session %s (attempt #%d): %v",
				sess.localID, i, err)
		}
		if ctx.Err() != nil {
			// Parent context canceled.
			return err
		}

		// Try again.
	}

	return fmt.Errorf("%w %s received after %d tries", errLeaveSessNoReply,
		sess.localID, c.cfg.maxLeaveTries)
}

// KickMember attempts to kick a target member from the session. This will only
// work if the local peer is an admin of the session.
//
// The ban only applies while there are other peers in the session (if the
// session empties out, the ban is lifted).
func (c *Client) KickMember(ctx context.Context, sess *Session,
	target rpc.RTDTPeerID, banDuration time.Duration) error {

	// Try multiple times before giving up.
	for i := 0; i < c.cfg.maxKickTries; i++ {
		tryCtx, cancel := context.WithTimeout(ctx, c.cfg.kickReplyTimeout)
		err := sess.conn.kickFromSession(tryCtx, sess, target, banDuration)

		cancel()
		if err == nil {
			// Success.
			return err
		}
		if ctx.Err() != nil {
			// Parent context canceled.
			return err
		}

		// Try again.
	}

	return fmt.Errorf("%w %s received after %d tries", errKickFromSessNoReply,
		target, c.cfg.maxKickTries)
}

// AdminRotateSessionCookie forces the server to change all members to a new
// session id. This is usually called after kicking and banning one or more
// users, to prevent them ever joining the session again.
//
// The local client must be an admin of the session for this to work.
func (c *Client) AdminRotateSessionCookie(ctx context.Context, sess *Session, rotCookie []byte) error {
	// Try multiple times before giving up.
	for i := 0; i < c.cfg.maxRotateTries; i++ {
		tryCtx, cancel := context.WithTimeout(ctx, c.cfg.rotateReplyInterval)
		err := sess.conn.adminRotateSessionCookie(tryCtx, sess, rotCookie)

		cancel()
		if err == nil {
			// Success.
			return err
		}
		if ctx.Err() != nil {
			// Parent context canceled.
			return err
		}

		// Try again.
	}

	return fmt.Errorf("%w %s received after %d tries", errAdminRotateCookiesNoReply,
		sess.rv, c.cfg.maxRotateTries)

}
