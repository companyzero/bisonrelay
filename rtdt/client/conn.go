package rtdtclient

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"os"
	"slices"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/companyzero/bisonrelay/rpc"
	"github.com/companyzero/sntrup4591761"
	"github.com/decred/slog"
	"golang.org/x/crypto/nacl/secretbox"
	"golang.org/x/sync/errgroup"
)

type decryptedPacketHandler func(ctx context.Context, sess *Session,
	peer *sessionPeer, enc *rpc.RTDTFramedPacket,
	plain *rpc.RTDTDataPacket) error

type connSocket struct {
	c          *net.UDPConn
	sessionKey *[32]byte
}

// kickedFromConnSessionCB is the callback type for handling kick events from
// a conn.
type kickedFromConnSessionCB func(*conn, *Session, time.Duration)

// conn multiplexes session I/O through an UDP connection to a single server.
type conn struct {
	log slog.Logger

	runCtx    context.Context
	cancelRun func()

	sAddr net.UDPAddr
	cAddr string

	// pingOnConnect forces a ping immediately after connecting. This is
	// useful to get an RTT check right after connecting.
	pingOnConnect bool

	handshakeRetryInterval time.Duration
	serverKeyGen           ServerSharedKeyGenerator

	socket atomic.Pointer[connSocket]

	// handler is called after a data packet is decrypted.
	handler        decryptedPacketHandler
	newPeerCb      NewPeerCallback
	bytesWrittenCb BytesWrittenCallback
	membersUpdtCb  SessionPeerListUpdated
	kickedCb       kickedFromConnSessionCB
	rttCb          PingRTTCalculated
	pktInCb        PacketIOCallback
	pktOutCb       PacketIOCallback

	// ignoreUnkeyedPeers is set to true if the read loop should ignore
	// peers for which it has no publisher keys.
	ignoreUnkeyedPeers bool

	mtx          sync.Mutex
	pendingSess  map[rpc.RTDTPeerID]*pendingSession
	pendingLeave map[rpc.RTDTPeerID]chan error
	sessions     map[rpc.RTDTPeerID]*Session
	kickAttempts map[sourceTargetPairKey]chan error
	rotCookies   map[rpc.RTDTPeerID]chan error
	pkt          rpc.RTDTFramedPacket
	plainPkt     rpc.RTDTDataPacket
	sendBuf      []byte
	encSendBuf   []byte
	pingBuf      []byte
	pendingPings map[uint64]time.Time
}

func newConn(addr *net.UDPAddr, handler decryptedPacketHandler,
	newPeerCb NewPeerCallback,
	bytesWrittenCb BytesWrittenCallback, serverKeyGen ServerSharedKeyGenerator,
	handshakeRetryInterval time.Duration, ignoreUnkeyedPeers bool,
	membersUpdtCb SessionPeerListUpdated, kickedCb kickedFromConnSessionCB,
	rttCb PingRTTCalculated, log slog.Logger) *conn {

	// Presize the ping map to the max we expect (should be a low number).
	pingCountHint := rpc.RTDTMaxPingInterval/rpc.RTDTDefaultMinPingInterval + 1

	return &conn{
		sAddr:                  *addr,
		cAddr:                  addr.String(),
		handler:                handler,
		newPeerCb:              newPeerCb,
		bytesWrittenCb:         bytesWrittenCb,
		log:                    log,
		serverKeyGen:           serverKeyGen,
		handshakeRetryInterval: handshakeRetryInterval,
		ignoreUnkeyedPeers:     ignoreUnkeyedPeers,
		membersUpdtCb:          membersUpdtCb,
		kickedCb:               kickedCb,
		rttCb:                  rttCb,
		sessions:               make(map[rpc.RTDTPeerID]*Session),
		pendingSess:            make(map[rpc.RTDTPeerID]*pendingSession),
		pendingLeave:           make(map[rpc.RTDTPeerID]chan error),
		kickAttempts:           make(map[sourceTargetPairKey]chan error),
		rotCookies:             make(map[rpc.RTDTPeerID]chan error),
		encSendBuf:             make([]byte, rpc.RTDTMaxMessageSize),
		pendingPings:           make(map[uint64]time.Time, pingCountHint),
		pingBuf:                make([]byte, 8),
	}
}

// aliasNonce aliases a nonce into a byte slice. This avoids having to allocate
// and copy the nonce into a new array.
func aliasNonce(b []byte) *[24]byte {
	_ = b[23] // Bounds check.
	return (*[24]byte)(unsafe.Pointer(unsafe.SliceData(b)))
}

// writePkt writes what is in sendBuf to the remote server. Mtx MUST be held.
func (c *conn) writePkt() (int, error) {
	var n int
	var err error
	socket := c.socket.Load()
	if socket.sessionKey == nil {
		// No transport-level encryption.
		n, err = socket.c.Write(c.sendBuf)
	} else {
		// Client-to-server encryption enabled.
		out := c.encSendBuf[:24]
		rand.Read(out[:]) // Read nonce
		out = secretbox.Seal(out, c.sendBuf, aliasNonce(out), socket.sessionKey)
		n, err = socket.c.Write(out)
	}

	// If write() errored, close the conn to attempt a new handshake.
	if err != nil {
		// Ignore ErrClosed because it just means another goroutine
		// closed the conn during a write.
		if !errors.Is(err, net.ErrClosed) {
			c.log.Infof("Write to conn %s (local addr %s) failed due to %v",
				socket.c.RemoteAddr(), socket.c.LocalAddr(), err)
		}
		closeErr := socket.c.Close()
		if closeErr != nil && !errors.Is(err, net.ErrClosed) {
			c.log.Debugf("Close() errored after conn Write() error: %v", closeErr)
		}
	} else if c.pktOutCb != nil {
		// Write successful, alert callback.
		c.pktOutCb(n)
	}

	return n, err
}

func (c *conn) sendJoinCmd(sess *Session, addPending bool) (*pendingSession, error) {
	// Prepare and send the join command to server.
	sess.mtx.Lock()
	joinCmd := rpc.RTDTServerCmdJoinSession{
		JoinCookie: sess.lastJoinCookie,
	}
	sess.mtx.Unlock()

	c.mtx.Lock()
	c.pkt.Target = 0
	c.pkt.Source = sess.localID
	c.pkt.Sequence++
	c.sendBuf = joinCmd.AppendFramed(&c.pkt, c.sendBuf)
	_, err := c.writePkt()
	c.sendBuf = c.sendBuf[:0]

	// Track the pending session.
	var pending *pendingSession
	if addPending && err == nil {
		pending = &pendingSession{
			joinedChan: make(chan struct{}),
			sess:       sess,
		}
		c.pendingSess[sess.localID] = pending
	}
	c.mtx.Unlock()

	return pending, err
}

// joinSession sends a server command to join the specified session, using the
// given join cookie.
func (c *conn) joinSession(ctx context.Context, sess *Session, joinCookie []byte) error {
	// Track last join cookie in case a reconnection is needed.
	sess.mtx.Lock()
	sess.lastJoinCookie = joinCookie
	sess.mtx.Unlock()

	// Send the join command and track it as a pending session.
	pending, err := c.sendJoinCmd(sess, true)
	if err != nil {
		return err
	}

	// Wait until server replies with ack of join.
	select {
	case <-ctx.Done():
		// Stop expecting the reply.
		c.mtx.Lock()
		delete(c.pendingSess, sess.localID)
		c.mtx.Unlock()

		return ctx.Err()
	case <-pending.joinedChan:
		c.log.Infof("Received reply for joining session %s",
			sess.localID)

		// Change the status from pending to fully live.
		c.mtx.Lock()
		delete(c.pendingSess, sess.localID)
		c.sessions[sess.localID] = sess

		if sess.sessionJoinedCb != nil {
			sess.sessionJoinedCb()
		}
		c.mtx.Unlock()

		return nil
	}
}

// leaveSession sends a command to leave the session and waits for the reply.
func (c *conn) leaveSession(ctx context.Context, sess *Session) error {

	c.mtx.Lock()
	_, alreadyPending := c.pendingLeave[sess.localID]
	if alreadyPending {
		c.mtx.Unlock()
		return errors.New("already attempting to leave session")
	}
	chanPending := make(chan error, 1)
	c.pendingLeave[sess.localID] = chanPending

	cmd := rpc.RTDTServerCmdLeaveSession{}

	// Send the request to leave session.
	c.pkt.Target = 0
	c.pkt.Source = sess.localID
	c.pkt.Sequence++
	c.sendBuf = cmd.AppendFramed(&c.pkt, c.sendBuf)
	_, err := c.writePkt()
	c.sendBuf = c.sendBuf[:0]

	if err != nil {
		delete(c.pendingLeave, sess.localID)
		c.mtx.Unlock()
		return err
	}

	c.mtx.Unlock()

	// Wait for the reply.
	select {
	case err = <-chanPending:
		// The handler already removed from c.pendingLeave.
	case <-ctx.Done():
		err = ctx.Err()

		// Remove from pending leave.
		c.mtx.Lock()
		if c.pendingLeave[sess.localID] == chanPending {
			delete(c.pendingLeave, sess.localID)
		}
		c.mtx.Unlock()
	}

	return err
}

// kickFromSession sends a command to kick a target peer from the session and
// waits for the reply.
func (c *conn) kickFromSession(ctx context.Context, sess *Session,
	target rpc.RTDTPeerID, banDuration time.Duration) error {

	c.log.Debugf("Attempting to kick %s from session %s (banning for %s)",
		target, sess.localID, banDuration)

	kickKey := makeSourceTargetPairKey(sess.localID, target)

	c.mtx.Lock()
	_, alreadyKicking := c.kickAttempts[kickKey]
	if alreadyKicking {
		c.mtx.Unlock()
		return errors.New("already attempting to kick this target from session")
	}
	replyChan := make(chan error, 1)
	c.kickAttempts[kickKey] = replyChan

	cmd := rpc.RTDTServerCmdKickPeer{
		KickTarget:         target,
		BanDurationSeconds: uint32(banDuration.Seconds()),
	}

	// Send the request to kick target from session.
	c.pkt.Target = 0
	c.pkt.Source = sess.localID
	c.pkt.Sequence++
	c.sendBuf = cmd.AppendFramed(&c.pkt, c.sendBuf)
	_, err := c.writePkt()
	c.sendBuf = c.sendBuf[:0]

	if err != nil {
		delete(c.kickAttempts, kickKey)
		c.mtx.Unlock()
		return err
	}

	c.mtx.Unlock()

	// Wait for the reply.
	select {
	case err = <-replyChan:
		// The handler already removed from c.kickAttempts.
	case <-ctx.Done():
		err = ctx.Err()

		// Remove from pending kicks.
		c.mtx.Lock()
		if c.kickAttempts[kickKey] == replyChan {
			delete(c.kickAttempts, kickKey)
		}
		c.mtx.Unlock()
	}

	return err
}

// adminRotateSessionCookie sends the command to rotate session cookies and
// waits for the response.
func (c *conn) adminRotateSessionCookie(ctx context.Context, sess *Session, rotCookie []byte) error {
	c.log.Debugf("Attempting to rotate cookies of session %s peerID %s",
		sess.rv, sess.localID)
	c.mtx.Lock()
	_, alreadyRotating := c.rotCookies[sess.localID]
	if alreadyRotating {
		c.mtx.Unlock()
		return errors.New("already attempting to rotate this session cookies")
	}
	replyChan := make(chan error, 1)
	c.rotCookies[sess.localID] = replyChan

	cmd := rpc.RTDTServerCmdRotateSessionCookies{
		RotateCookie: rotCookie,
	}

	// Send the request to kick target from session.
	c.pkt.Target = 0
	c.pkt.Source = sess.localID
	c.pkt.Sequence++
	c.sendBuf = cmd.AppendFramed(&c.pkt, c.sendBuf)
	_, err := c.writePkt()
	c.sendBuf = c.sendBuf[:0]

	if err != nil {
		delete(c.rotCookies, sess.localID)
		c.mtx.Unlock()
		return err
	}

	c.mtx.Unlock()

	// Wait for the reply.
	select {
	case err = <-replyChan:
		// The handler already removed from c.rotCookies
	case <-ctx.Done():
		err = ctx.Err()

		// Remove from pending rotations.
		c.mtx.Lock()
		if c.rotCookies[sess.localID] == replyChan {
			delete(c.rotCookies, sess.localID)
		}
		c.mtx.Unlock()
	}

	return err
}

// sendPing sends a ping command to the server.
func (c *conn) sendPing() error {
	pingID := mustRandomUint64()
	cmd := rpc.RTDTPingCmd{}

	c.mtx.Lock()

	// Pick a session to ping. Any will do.
	for _, sess := range c.sessions {
		c.pkt.Source = sess.localID
		break
	}

	c.pendingPings[pingID] = time.Now()
	binary.LittleEndian.PutUint64(c.pingBuf, pingID)
	cmd.Data = c.pingBuf

	c.pkt.Target = 0
	c.pkt.Sequence++
	c.sendBuf = cmd.AppendFramed(&c.pkt, c.sendBuf)
	_, err := c.writePkt()
	c.sendBuf = c.sendBuf[:0]

	c.mtx.Unlock()
	return err
}

// sendData sends data to the remote server.
func (c *conn) sendData(sess *Session, stream rpc.RTDTStreamType, ts uint32, data []byte) error {

	c.mtx.Lock()
	c.pkt.Target = sess.localID
	c.pkt.Source = sess.localID
	c.pkt.Sequence++
	c.plainPkt.Stream = stream
	c.plainPkt.Timestamp = ts
	c.plainPkt.Data = data
	c.sendBuf = c.plainPkt.AppendEncrypted(&c.pkt, c.sendBuf, sess.publisherKey, sess.sigKey)
	n, err := c.writePkt()

	c.sendBuf = c.sendBuf[:0]
	c.mtx.Unlock()

	if c.bytesWrittenCb != nil {
		c.bytesWrittenCb(sess, n)
	}

	return err
}

// handshake performs the encryption handshake with the remote server. This
// MUST be run before run() is called.
func (c *conn) handshake(ctx context.Context) error {
	var udpConn *net.UDPConn
	udpConn, err := net.DialUDP("udp", nil, &c.sAddr)
	if err != nil {
		return err
	}

	if c.serverKeyGen == nil {
		// This is allowed for testing purposes.
		c.log.Warnf("Running connection to server %s WITHOUT ENCRYPTION", c.cAddr)
		c.socket.Store(&connSocket{c: udpConn})
		return nil
	}

	type msg struct {
		err error
		n   int
	}
	readChan := make(chan msg, 1)

	const replyLen = sntrup4591761.CiphertextSize + 24 + secretbox.Overhead
	buf := make([]byte, replyLen)
	rcvCiphertext := make([]byte, sntrup4591761.CiphertextSize)
	var nonce [24]byte

	// Generate a new session key.
	ciphertext, sessionKey := c.serverKeyGen()

	const maxTries = 5

nextTry:
	for i := 0; ; i++ {
		c.log.Debugf("Attempting handshake with %s (attempt #%d, local addr %s)",
			c.cAddr, i+1, udpConn.LocalAddr().String())

		// Send the ciphertext to the remote server.
		n, err := udpConn.Write(ciphertext[:])
		if err != nil {
			return err
		} else if c.pktOutCb != nil {
			c.pktOutCb(n)
		}

		// Wait until we read back the response.
		go func() {
			clear(buf)
			udpConn.SetReadDeadline(time.Now().Add(c.handshakeRetryInterval))
			n, err := udpConn.Read(buf)
			udpConn.SetReadDeadline(time.Time{})
			readChan <- msg{err: err, n: n}
		}()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case msg := <-readChan:
			if errors.Is(msg.err, os.ErrDeadlineExceeded) {
				// Server took too long to respond.
				if i >= maxTries {
					return fmt.Errorf("exceeded max handshake tries (%d)", maxTries)
				}
				continue nextTry
			}

			if msg.err != nil {
				return fmt.Errorf("conn read() errored during handshake: %v", msg.err)
			}

			if c.pktInCb != nil {
				c.pktInCb(n)
			}

			if msg.n < replyLen {
				// Try again.
				if i >= maxTries {
					return fmt.Errorf("exceeded max handshake tries (%d)", maxTries)
				}

				c.log.Debugf("Handshake received wrong number of bytes "+
					"in reply (got %d, want %d)", msg.n, replyLen)
				continue nextTry
			}
		}

		// The response must be the ciphertext, encrypted back with the
		// shared key.
		copy(nonce[:], buf) // First 24 bytes of the reply is the nonce.
		_, ok := secretbox.Open(rcvCiphertext[:0], buf[24:], &nonce, sessionKey)
		if !ok {
			// Try again.
			c.log.Warnf("Failed to decrypt handshake reply from server")
			continue
		}

		if !bytes.Equal(rcvCiphertext, ciphertext[:]) {
			// Try again.
			c.log.Warnf("Received wrong ciphertext reply")
			continue
		}

		// Session key is established.
		c.socket.Store(&connSocket{c: udpConn, sessionKey: sessionKey})
		break
	}
	c.log.Debugf("Completed handshake with server %s (local addr %s)",
		c.cAddr, udpConn.LocalAddr())
	return nil
}

// handleCmd handles remote commands and replies from the server.
func (c *conn) handleCmd(pkt rpc.RTDTFramedPacket) error {
	cmdType, cmdPayload := pkt.ServerCmd()

	switch cmdType {
	case rpc.RTDTServerCmdTypeJoinSessionReply:
		// Note: non-debug server does not fill the error code field,
		// so that's not checked here.
		c.mtx.Lock()
		pendingSess, ok := c.pendingSess[pkt.Target]
		if ok {
			// Let caller know we joined the session.
			select {
			case <-pendingSess.joinedChan:
			default:
				close(pendingSess.joinedChan)
			}
		} else if sess := c.sessions[pkt.Target]; sess != nil {
			// This can legitimately happen after a reconnection.
			c.log.Debugf("Received join session reply for already established session %s", pkt.Target)
			if sess.sessionJoinedCb != nil {
				sess.sessionJoinedCb()
			}
			ok = true
		}
		c.mtx.Unlock()

		if !ok {
			return fmt.Errorf("session %s was not pending", pkt.Target)
		}

		return nil

	case rpc.RTDTServerCmdTypeLeaveSessionReply:
		// Note: non-debug server does not fill the error code field,
		// so that's not checked here.

		c.mtx.Lock()
		c.log.Debugf("Received reply to leave session %s", pkt.Target)
		if sess, hasSess := c.sessions[pkt.Target]; hasSess {
			delete(c.sessions, pkt.Target)
			sess.left.Store(true)
		}

		pendingLeave, ok := c.pendingLeave[pkt.Target]
		if ok && pendingLeave != nil {
			pendingLeave <- nil
			delete(c.pendingLeave, pkt.Target)
		}
		c.mtx.Unlock()

		if !ok {
			return fmt.Errorf("session %s was not pending to leave", pkt.Target)
		}

		return nil

	case rpc.RTDTServerCmdTypePong:
		// Nothing to do with pong.
		var cmd rpc.RTDTPongCmd
		if err := cmd.FromBytes(cmdPayload); err != nil {
			return err
		}
		if len(cmd.Data) < 8 {
			return fmt.Errorf("server did not echo pong data")
		}
		pingID := binary.LittleEndian.Uint64(cmd.Data)

		c.mtx.Lock()
		var rtt time.Duration
		if outTime, ok := c.pendingPings[pingID]; ok {
			rtt = time.Since(outTime)
			delete(c.pendingPings, pingID)
		}
		c.mtx.Unlock()
		c.log.Tracef("Received pong from %s (RTT %s)", c.cAddr, rtt)
		if rtt > 0 && c.rttCb != nil {
			c.rttCb(c.sAddr, rtt)
		}
		return nil

	case rpc.RTDTServerCmdTypeKickPeer:
		// Clients receive this when they were kicked from the session.
		var cmd rpc.RTDTServerCmdKickPeer
		if err := cmd.FromBytes(cmdPayload); err != nil {
			return err
		}

		banDuration := time.Duration(cmd.BanDurationSeconds) * time.Second

		c.mtx.Lock()
		sess, hasSess := c.sessions[pkt.Target]
		if hasSess {
			c.log.Warnf("Kicked from session %s (ban duration %s)",
				pkt.Target, banDuration)
			delete(c.sessions, pkt.Target)
			sess.left.Store(true)
		}
		c.mtx.Unlock()
		if !hasSess {
			return fmt.Errorf("received kicked report for non-session %s",
				pkt.Target)
		}

		if c.kickedCb != nil {
			c.kickedCb(c, sess, banDuration)
		}

		return nil

	case rpc.RTDTServerCmdTypeKickPeerReply:
		// Note: non-debug server does not fill the error code field,
		// so that's not checked here.
		var cmd rpc.RTDTServerCmdKickPeerReply
		if err := cmd.FromBytes(cmdPayload); err != nil {
			return err
		}

		kickKey := makeSourceTargetPairKey(pkt.Target, cmd.KickTarget)

		c.mtx.Lock()
		replyChan, ok := c.kickAttempts[kickKey]
		if ok {
			delete(c.kickAttempts, kickKey)
		}
		c.mtx.Unlock()

		if !ok {
			return fmt.Errorf("target %s in session %s was not "+
				"a kick attempt", cmd.KickTarget, pkt.Target)
		} else {
			c.log.Infof("Received reply to kick %s from session %s",
				cmd.KickTarget, pkt.Target)
			replyChan <- nil
		}

		return nil

	case rpc.RTDTServerCmdTypeMembersBitmap:
		c.mtx.Lock()
		sess := c.sessions[pkt.Target]
		c.mtx.Unlock()

		if sess == nil {
			return fmt.Errorf("not bound to session %s while "+
				"decoding members list", pkt.Target)
		}

		var cmd rpc.RTDTServerCmdMembersBitmap
		if err := cmd.FromBytes(cmdPayload); err != nil {
			return err
		}

		sess.mtx.Lock()
		sess.bmpBuf.Reset()
		sess.bmpBuf.Write(cmd.Bitmap)
		sess.bmp.Clear() // Needed before ReadFrom?
		_, readErr := sess.bmp.ReadFrom(sess.bmpBuf)
		validateErr := sess.bmp.Validate()
		if readErr != nil || validateErr != nil {
			sess.bmp.Clear()
		}
		sess.mtx.Unlock()

		if readErr != nil {
			return fmt.Errorf("unable to decode members bitmap: %v", readErr)
		}
		if validateErr != nil {
			return fmt.Errorf("validation failed after decoding members bitmap: %v", validateErr)
		}

		if c.membersUpdtCb != nil {
			c.membersUpdtCb(sess)
		}

		return nil

	case rpc.RTDTServerCmdTypeRotateCookiesReply:
		// Note: non-debug server does not fill the error code field,
		// so that's not checked here.
		var cmd rpc.RTDTServerCmdRotateSessionCookiesReply
		if err := cmd.FromBytes(cmdPayload); err != nil {
			return err
		}

		c.mtx.Lock()
		replyChan, ok := c.rotCookies[pkt.Target]
		if ok {
			delete(c.rotCookies, pkt.Target)
		}
		c.mtx.Unlock()

		if replyChan != nil {
			replyChan <- nil
		} else {
			c.log.Warnf("Received reply to rotating cookies of "+
				"session %s when not part of that session",
				pkt.Target)
		}

		return nil

	default:
		return fmt.Errorf("unknown server command %d", cmdType)
	}
}

// readLoop is the loop that processes incoming messages from this conn.
func (c *conn) readLoop(ctx context.Context, readTimeout time.Duration) error {
	readBuf := make([]byte, rpc.RTDTMaxMessageSize)
	decryptedBuf := make([]byte, rpc.RTDTMaxMessageSize)

	var nonce [24]byte
	var encPacket rpc.RTDTFramedPacket
	var plainPacket rpc.RTDTDataPacket

	var decryptErrCount int64

	socket := c.socket.Load()

	for {
		socket.c.SetReadDeadline(time.Now().Add(readTimeout))
		n, err := socket.c.Read(readBuf)
		socket.c.SetReadDeadline(time.Time{})
		if err != nil {
			return err
		} else if c.pktInCb != nil {
			c.pktInCb(n)
		}

		// Decrypt transport-level (client-to-server) packet.
		msgBuf := readBuf[:n]
		if socket.sessionKey != nil {
			if n < 24+16 {
				// Not enough bytes to decrypt.
				continue
			}

			copy(nonce[:], msgBuf)
			var ok bool
			msgBuf, ok = secretbox.Open(decryptedBuf[:0], msgBuf[24:], &nonce, socket.sessionKey)
			if !ok {
				// Decryption failed.
				decryptErrCount++
				if decryptErrCount%10000 == 1 {
					c.log.Warnf("Decryption failed for %d packets", decryptErrCount)
				}
				continue
			}
		}

		if err := encPacket.FromBytes(msgBuf); err != nil {
			c.log.Warnf("Error decoding incoming packet: %v", err)
			continue
		}

		// Handle internal server commands.
		if encPacket.Source == 0 {
			err := c.handleCmd(encPacket)
			if err != nil {
				c.log.Warnf("Error processing internal server command: %v", err)
			}
			continue
		}

		c.mtx.Lock()
		sess, ok := c.sessions[encPacket.Target]
		c.mtx.Unlock()
		if !ok {
			c.log.Warnf("Received packet for unknown session %s on conn %s",
				encPacket.Target, c.cAddr)
			continue
		}

		// Determine the source peer.
		sess.mtx.Lock()
		peer, isNew := sess.getPeer(encPacket.Source)
		if isNew && c.newPeerCb != nil {
			// First time seeing this remote peer. Fetch its sig and
			// decryption keys.
			peer.sigKey, peer.publisherKey = c.newPeerCb(encPacket.Source, sess.rv)
		}
		encKey, sigKey := peer.publisherKey, peer.sigKey
		sess.mtx.Unlock()

		// Ignore data from this peer if we don't have keys for them and
		// have been configured to do so.
		if encKey == nil && c.ignoreUnkeyedPeers {
			continue
		}

		// E2E decrypt packet. Decrypt() only performs actions based on
		// the non-nil keys.
		if err := plainPacket.Decrypt(encPacket, encKey, sigKey); err != nil {
			c.log.Warnf("E2E decryption error for peer %s on conn %s: %v",
				encPacket.Source, c.cAddr, err)
			continue
		}

		// Handle packet payload.
		if err := c.handler(ctx, sess, peer, &encPacket, &plainPacket); err != nil {
			c.log.Warnf("Error processing encrypted packet: %v", err)
			continue
		}
	}
}

// pingLoop sends periodic pings to the remote server.
func (c *conn) pingLoop(ctx context.Context, pingInterval time.Duration) error {
	if c.pingOnConnect {
		c.log.Debugf("Sending initial ping after connecting to conn %s", c.cAddr)
		if err := c.sendPing(); err != nil {
			return err
		}
	}

	for {
		select {
		case <-time.After(pingInterval):
		case <-ctx.Done():
			return ctx.Err()
		}

		c.log.Tracef("Sending ping to conn %s", c.cAddr)
		if err := c.sendPing(); err != nil {
			return err
		}

		// Remove stale pings.
		c.mtx.Lock()
		for id, t := range c.pendingPings {
			if time.Since(t) > rpc.RTDTMaxPingInterval {
				delete(c.pendingPings, id)
			}
		}
		c.mtx.Unlock()
	}
}

// runConn a conn until it errors.
func (c *Client) runConn(ctx context.Context, conn *conn) error {

	g, ctx := errgroup.WithContext(ctx)
	c.log.Infof("Running conn with %s using %d read routines", conn.cAddr,
		c.cfg.readRoutines)
	g.Go(func() error { return conn.pingLoop(ctx, c.cfg.pingInterval) })
	g.Go(func() error {
		// Close conn if the context is closed to force Read() to error
		// out earlier than the readTimeout.
		<-ctx.Done()
		err := conn.socket.Load().c.Close()
		if errors.Is(err, net.ErrClosed) {
			// Ignore ErrClosed because it just means somewhere
			// else already closed the conn.
			err = nil
		}
		return err
	})

	for i := 0; i < c.cfg.readRoutines; i++ {
		g.Go(func() error { return conn.readLoop(ctx, c.cfg.readTimeout) })
	}
	return g.Wait()
}

// attemptReconnect attempts to re-connect and rejoin all sessions.
func (c *conn) attemptReconnect(ctx context.Context) error {
	// Retry handshake.
	err := c.handshake(ctx)
	if err != nil {
		return err
	}

	// Attempt to rejoin sessions. Deduplicate c.sessions and c.pendingSess
	// because multiple reconnections attempts may leave the same session in
	// both.
	c.mtx.Lock()
	sessions := make(map[*Session]struct{}, len(c.sessions)+len(c.pendingSess))
	for _, sess := range c.sessions {
		sessions[sess] = struct{}{}
	}
	for _, pending := range c.pendingSess {
		sessions[pending.sess] = struct{}{}
	}
	c.mtx.Unlock()

	for sess := range sessions {
		sess := sess
		go func() {
			// Resend join command but do not setup as a pending
			// conn because the original caller may be waiting for
			// it still.
			c.log.Debugf("Attempt to re-join session %s after reconnection", sess.localID)
			_, err := c.sendJoinCmd(sess, false)
			if err != nil && !errors.Is(err, context.Canceled) {
				c.log.Errorf("Unable to re-join session during reconnection: %v", err)
			}
		}()
	}

	return nil
}

// removeConn removes the conn from the client. MUST NOT be called from within
// c.conns mutex.
func (c *Client) removeConn(delConn *conn) {
	c.conns.Compute(delConn.cAddr, func(conns []*conn, _ bool) ([]*conn, bool) {
		conns = slices.DeleteFunc(conns, func(cc *conn) bool { return cc == delConn })
		return conns, len(conns) == 0
	})
}

// removeConnIfEmpty removes the conn from the client if the conn is otherwise
// empty (no sessions). MUST NOT be called from within c.conns mutex.
func (c *Client) removeConnIfEmpty(delConn *conn) (sessCount int, pendingCount int) {
	c.conns.Compute(delConn.cAddr, func(conns []*conn, _ bool) ([]*conn, bool) {
		delConn.mtx.Lock()
		sessCount, pendingCount = len(delConn.sessions), len(delConn.pendingSess)
		if sessCount == 0 && pendingCount == 0 {
			conns = slices.DeleteFunc(conns, func(cc *conn) bool { return cc == delConn })
			delConn.cancelRun()
		}
		delConn.mtx.Unlock()
		return conns, len(conns) == 0
	})
	return
}

// kickedFromConnSession is called from a conn when the client was kicked.
func (c *Client) kickedFromConnSession(conn *conn, sess *Session, banDuration time.Duration) {
	c.removeConnIfEmpty(conn)
	if c.cfg.kickedCallback != nil {
		c.cfg.kickedCallback(sess, banDuration)
	}
}

// keepConnRunning runs the conn. If the conn errors, it attempts a new
// handshake with the server.
func (c *Client) keepConnRunning(conn *conn) {
	defer func() {
		c.removeConn(conn)
		c.log.Debugf("Finished keeping conn alive to %s", conn.cAddr)
	}()

	ctx := conn.runCtx
	for ctx.Err() == nil {
		err := c.runConn(ctx, conn)
		if errors.Is(err, context.Canceled) {
			return
		}
		if err != nil {
			c.log.Debugf("Error running conn to %s: %v", conn.cAddr, err)
		}

		closeErr := conn.socket.Load().c.Close()
		if closeErr != nil && !errors.Is(err, net.ErrClosed) {
			// Ignore ErrClosed because it just means somewhere
			// else already closed the conn.
			c.log.Debugf("Socket close error for %s: %v",
				conn.cAddr, err)
		}

		// When the conn doesn't have any more sessions, there's no
		// need to keep it running.
		conn.mtx.Lock()
		noSessions := len(conn.sessions) == 0 && len(conn.pendingSess) == 0
		if noSessions {
			c.removeConn(conn) // Avoid race scenario.
		}
		conn.mtx.Unlock()
		if noSessions {
			return
		}

		// Attempt new handshake.
		var attempt int
		for ctx.Err() == nil {
			// Wait a short amount before attempting a new dial and
			// handshake to account for changing network.
			select {
			case <-time.After(time.Second):
			case <-ctx.Done():
				return
			}

			attempt++
			c.log.Debugf("Attempt %d at reconnecting to %s after failure",
				attempt, conn.cAddr)
			err := conn.attemptReconnect(ctx)
			if err == nil || errors.Is(err, context.Canceled) {
				// err == nil will run the conn,
				// context.Canceled will exit this function.
				break
			}
		}
	}
}

// connToAddrAndId returns a conn to use to contact the given server address
// for a specific session id. It may create a new connection if needed.
//
// If the conn is loaded (as opposed to created), the bool return value will be
// true.
func (c *Client) connToAddrAndId(ctx context.Context, addr *net.UDPAddr,
	serverKeyGen ServerSharedKeyGenerator, targetId rpc.RTDTPeerID) (*conn, bool, error) {

	strAddr := addr.String()
	var err error

	var selConn *conn
	var loaded bool

	c.conns.Compute(strAddr, func(conns []*conn, _ bool) ([]*conn, bool) {
		// If there is a conn that does _not_ have a session with the
		// given id, we can use it.
		for i := range conns {
			conns[i].mtx.Lock()
			_, contains := conns[i].sessions[targetId]
			conns[i].mtx.Unlock()

			if !contains {
				// Found one!
				loaded = true
				selConn = conns[i]
				return conns, false
			}
		}

		// None of the existing conns (if there are any) is acceptable.
		// Create a new conn.
		conn := newConn(addr, c.handleDecryptedPacket,
			c.cfg.newPeerCallback,
			c.cfg.bytesWrittenCallback,
			serverKeyGen, c.cfg.handshakeRetryInterval,
			c.cfg.ignoreUnkeyedPeers, c.cfg.sessPeerListUpdatedCallback,
			c.kickedFromConnSession, c.cfg.pingRTTCalculated, c.log)
		conn.pingOnConnect = c.cfg.pingOnConnect
		conn.pktInCb = c.cfg.pktInCallback
		conn.pktOutCb = c.cfg.pktOutCallback

		// Perform the first handshake attempt. Fail outright if it
		// never completes.
		if err = conn.handshake(ctx); err != nil {
			err = fmt.Errorf("unable to complete handshake: %w", err)
			return conns, false
		}

		conn.runCtx, conn.cancelRun = context.WithCancel(c.cfg.connCtx)

		selConn = conn
		conns = append(conns, conn)
		return conns, false
	})
	if err != nil {
		return nil, false, err
	}

	if !loaded {
		// Keep running the new conn. If it fails after this, it will
		// keep re-trying to handshake and re-join sessions, until the
		// context is closed.
		go c.keepConnRunning(selConn)
	}

	return selConn, loaded, nil
}
