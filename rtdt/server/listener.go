package rtdtserver

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"net"
	"net/netip"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/companyzero/bisonrelay/rpc"
	"github.com/companyzero/bisonrelay/rtdt/internal/seqtracker"
	"github.com/companyzero/sntrup4591761"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/puzpuzpuz/xsync/v3"
	"golang.org/x/crypto/nacl/secretbox"
)

// pendingConn holds data from connections before the handshake is completed.
type pendingConn struct {
	ciphertext sntrup4591761.Ciphertext
}

// conn tracks data for a particular remote peer connection.
type conn struct {
	l        *listener
	addr     netip.AddrPort
	lastRead atomicTime

	// sessionKey is the key used to encrypt/decrypt messages from this
	// remote peer.
	sessionKey [32]byte

	// recvSeq tracks sequence numbers acceptable for receiving by the
	// server.
	recvSeq seqtracker.Tracker

	// sendSeq tracks the sequence number that should be used to send
	// packets to this client.
	sendSeq atomic.Uint32

	// lastRecvPingTime is the time the remote peer sent a ping message.
	lastRecvPingTime atomicTime

	// pending is filled when the conn is initially created and pending
	// the first non-handshake message.
	pending atomic.Pointer[pendingConn]

	// map of sessions this conn has bound itself to, along with the peer ID
	// it identifies itself with.
	sessions *xsync.MapOf[rpc.RTDTPeerID, *peerSession]

	// banScore tracks the ban score for this conn. If it exceeds some
	// given configured value, the conn will be closed.
	banScore atomic.Uintptr
}

func newConn(addr netip.AddrPort, recvTime time.Time, l *listener) *conn {
	const sessionsHint = 1

	c := &conn{
		addr:     addr,
		l:        l,
		sessions: xsync.NewMapOf[rpc.RTDTPeerID, *peerSession](xsync.WithPresize(sessionsHint)),
	}
	c.lastRead.Store(recvTime)
	return c
}

// LastReadTime returns the time when the last successful read was received
// from this conn.
func (c *conn) LastReadTime() time.Time {
	v, _ := c.lastRead.Load()
	return v
}

func (c *conn) String() string {
	return c.addr.String()
}

// nextSendSeq returns the next seq to use to send a message to this client.
func (c *conn) nextSendSeq() uint32 {
	return c.sendSeq.Add(1)
}

// unsessionedMsg is a message received from a peer that hasn't yet finished
// handshaking into a fully encrypted session.
type unsessionedMsg struct {
	in       []byte
	addr     netip.AddrPort
	recvTime time.Time
}

func (m *unsessionedMsg) clear() {
	clear(m.in)
	m.recvTime = time.Time{}
	m.addr = netip.AddrPort{}
}

// listener tracks data about a single bound listening socket.
type listener struct {
	l *net.UDPConn

	ksTracker kernelStatsTracker

	unsessMsgChan   chan *unsessionedMsg
	unsessQueueChan chan *unsessionedMsg

	// conns *xsync.MapOf[connID, *conn]
	conns *xsync.MapOf[netip.AddrPort, *conn]

	connsCount   atomic.Int64
	pendingCount atomic.Int64
}

// Addr returns the listener's network address.
func (l *listener) Addr() net.Addr {
	return l.l.LocalAddr()
}

func (l *listener) UDPStats() UDPProcStats {
	// Already checked for errors on Listen().
	stats, _ := l.ksTracker.stats()
	return stats
}

// listen starts an RTDT listener on the given listener conn.
func listen(inner *net.UDPConn, ignoreKernelTracker bool) (*listener, error) {
	var err error

	// Ensure we can read stats.
	var ksTracker kernelStatsTracker = nullKernelStatsTracker{}
	if !ignoreKernelTracker {
		ksTracker, err = initKernelStatsTracker(inner)
		if err != nil {
			return nil, err
		}
	}

	// How many unsessioned (possibly handshake) messages to queue to the
	// handshake goroutine. After this queue is filled, messages are
	// dropped.
	const unsessionedQueueSize = 5

	l := &listener{
		l:         inner,
		ksTracker: ksTracker,
		conns:     xsync.NewMapOf[netip.AddrPort, *conn](),

		unsessMsgChan:   make(chan *unsessionedMsg, unsessionedQueueSize),
		unsessQueueChan: make(chan *unsessionedMsg, unsessionedQueueSize),
	}

	// Fill the queue with pre-sized messages.
	for i := 0; i < cap(l.unsessMsgChan); i++ {
		l.unsessMsgChan <- &unsessionedMsg{
			in: make([]byte, sntrup4591761.CiphertextSize*2),
		}
	}

	return l, nil
}

// aliasNonce aliases a nonce into a byte slice. This avoids having to allocate
// and copy the nonce into a new array.
func aliasNonce(b []byte) *[24]byte {
	_ = b[23] // Bounds check.
	return (*[24]byte)(unsafe.Pointer(unsafe.SliceData(b)))
}

// listenerWrite writes the given raw message bytes to the remote conn. temp is
// used to encrypt the message if necessary.
func (s *Server) listenerWrite(c *conn, msg, temp []byte) (n int, err error) {

	if s.cfg.privKey == nil {
		// Write with encryption disabled.
		n, err = c.l.l.WriteToUDPAddrPort(msg, c.addr)
	} else {
		if len(msg) > 65536-24-16 {
			return 0, errors.New("message is too large to write encrypted")
		}

		// Write encrypted.
		temp = temp[:24]
		rand.Read(temp[:]) // Read random nonce.
		outBuf := secretbox.Seal(temp, msg, aliasNonce(temp), &c.sessionKey)
		n, err = c.l.l.WriteToUDPAddrPort(outBuf, c.addr)
	}

	if err == nil {
		if s.stats.promEnabled {
			s.stats.bytesWritten.Add(float64(n))
			s.stats.pktsWritten.Inc()
		}
		s.stats.bytesWrittenAtomic.Add(uint64(n))
		s.stats.pktsWrittenAtomic.Add(1)
	}

	return n, err
}

// listenerHandshakeLoop runs a loop that performs handshake operations with
// new incoming connections.
func (s *Server) listenerHandshakeLoop(ctx context.Context, l *listener) error {
	if s.cfg.privKey == nil {
		// Handshake is not needed
		s.log.Debugf("Early return from handshakeLoop because encryption is disabled")
		return nil
	}

	tempBuf := make([]byte, rpc.RTDTMaxMessageSize)

	for {
		var msg *unsessionedMsg
		select {
		case msg = <-l.unsessQueueChan:
		case <-ctx.Done():
			return ctx.Err()
		}

		conn, _ := l.conns.Compute(msg.addr, func(conn *conn, _ bool) (*conn, bool) {
			if conn != nil {
				// Some concurrent read already created the
				// conn object. Use it.
				return conn, false
			}

			// First message from this remote address and
			// it should be a session key. Decrypt it.
			var sessKey [32]byte
			if !s.cfg.privKey.Decapsulate(msg.in[:sntrup4591761.CiphertextSize], &sessKey) {
				// Not a valid ciphertext. Ignore.
				s.log.Debugf("Failed to decrypt session key from %s", msg.addr.String())
				return nil, true
			}

			conn = newConn(msg.addr, msg.recvTime, l)
			conn.sessionKey = sessKey
			pending := &pendingConn{}
			copy(pending.ciphertext[:], msg.in)
			conn.pending.Store(pending)
			s.stats.pendingConns.Inc()
			l.pendingCount.Add(1)

			// Send the ciphertext encrypted back to the
			// source.
			s.log.Debugf("Decrypted session key from %s", conn)
			return conn, false
		})

		var pending *pendingConn
		if conn != nil {
			pending = conn.pending.Load()
		}

		switch {
		case conn == nil:
			// Failed to decapsulate session key.

		case pending == nil:
			// Already setup conn. Ignore this message
			// (which should be a copy of the shared key).

		case !bytes.Equal(pending.ciphertext[:], msg.in):
			// Should not happen with a well-behaved client. This
			// means the remote client sent data other than
			// the ciphertext before we had a chance to
			// send the handshake reply.
			s.log.Warnf("Remote client sent data before handshake completed %s", conn)

		default:
			// in is a successfully decoded (or echoed back)
			// ciphertext for this address. Reply with handshake.
			s.log.Debugf("Echoing encrypted session key back to %s", conn)
			_, err := s.listenerWrite(conn, pending.ciphertext[:], tempBuf)
			if err != nil {
				return err
			}
		}

		// Return to queue to process another possible handshake.
		msg.clear()
		l.unsessMsgChan <- msg
	}
}

// listenerReadConn determines the conn for this next input message.
func (s *Server) listenerReadConn(in []byte, addr netip.AddrPort,
	l *listener, recvTime time.Time) (*conn, *pendingConn) {

	// Do an initial load, which takes care of the fast path of already
	// established connections without causing additional heap allocations.
	if c, _ := l.conns.Load(addr); c != nil {
		pending := c.pending.Load()
		if pending == nil {
			// Already setup conn. This is the most common case.
			return c, nil
		}

		if !bytes.Equal(pending.ciphertext[:], in) {
			// First message received that is not a copy of
			// the ciphertext. We expect all messages to
			// now be encrypted.
			c.pending.Store(nil)
			s.stats.pendingConns.Dec()
			l.pendingCount.Add(-1)
			s.stats.conns.Inc()
			l.connsCount.Add(1)
			s.log.Debugf("Completed handshake with %s", c)
			return c, nil
		}

		// This is a copy of the ciphertext, which means the source did
		// not receive the server's reply establishing the session key.
		// Resend it.
		s.log.Debugf("Resending ciphertext to %s due to received copy", c)
		c.banScore.Add(1) // Needed to avoid infinite resend.
		return c, pending
	}

	if s.cfg.privKey == nil {
		// No handshake needed because server is running without
		// encryption.
		conn, _ := l.conns.LoadOrCompute(addr, func() *conn {
			s.stats.conns.Inc()
			l.connsCount.Add(1)
			conn := newConn(addr, recvTime, l)
			s.log.Debugf("New unencrypted conn to %s", conn)
			return conn
		})
		return conn, nil
	}

	if len(in) < sntrup4591761.CiphertextSize {
		// No conn and not enough bytes to init the
		// shared key. Discard this connection attempt.
		s.log.Debugf("Received too few bytes in new conn from %s", addr.String())
		return nil, nil
	}

	// No prior conn for this source address, handshake needed and this
	// message is possibly a shared key. Send it to be processed on the
	// handshake goroutine.
	select {
	case msg := <-l.unsessMsgChan:
		msg.in = append(msg.in[:0], in...)
		msg.addr = addr
		msg.recvTime = recvTime
		l.unsessQueueChan <- msg
	default:
		// Stall: too many unsessioned messages being processed.
		// Increase the stats, but otherwise drop this message.
		s.log.Warnf("Dropping unsessioned msg from %s due to stall",
			addr.String())
		s.stats.handshakeStall.WithLabelValues(l.Addr().String()).Inc()
	}

	// In case this message is to be replied, the reply will be sent by the
	// handshake goroutine.
	return nil, nil
}

// listenerReadNext reads the next message into out and returns the associated
// connection. tempBuf is used to decrypt the message.
func (s *Server) listenerReadNext(inPkt *framedPktBuffer, tempBuf []byte, l *listener) (c *conn, n int, recvTime time.Time, err error) {

	for {
		var addr netip.AddrPort
		var pending *pendingConn
		var in []byte

		// Process new incoming message.
		n, addr, err = l.l.ReadFromUDPAddrPort(tempBuf)
		recvTime = time.Now()
		if err != nil {
			return nil, 0, time.Time{}, err
		}
		in = tempBuf[:n]

		if s.stats.promEnabled {
			s.stats.bytesRead.Add(float64(n))
			s.stats.pktsRead.Inc()
		}
		s.stats.bytesReadAtomic.Add(uint64(n))
		s.stats.pktsReadAtomic.Add(1)

		// Determine the conn associated to this address.
		c, pending = s.listenerReadConn(in, addr, l, recvTime)

		switch {
		case c == nil:
			// This inbound message should be ignored.
			continue

		case pending != nil:
			// New conn echoing handshake, send encrypted
			// ciphertext again.
			c.lastRead.Store(recvTime)

			// New conn performing handshake, send encrypted
			// ciphertext.
			_, err := s.listenerWrite(c, pending.ciphertext[:], tempBuf)
			if err != nil {
				return nil, 0, time.Time{}, err
			}

			// Ignore this message for reading purposes until the
			// handshake completes.
			s.log.Debugf("Echoing encrypted session key back to %s", c)
			continue

		case len(inPkt.b) < n-24-16:
			// Output buffer is too short to decrypt message. Should
			// never happen because the input buffer is sized to
			// the maximum message size (which is the maximum UDP
			// datagram size).
			err = errors.New("short read buffer")
			return

		case s.cfg.privKey == nil:
			// Encryption disabled. Read as-is.
			inPkt.setFullData(tempBuf[:n])
			c.lastRead.Store(recvTime)
			return

		case n < 24+16:
			// Not enough bytes to decrypt message.
			continue
		}

		// Decrypt message.
		ok := inPkt.decryptFrom(in, &c.sessionKey)
		if !ok {
			// Ignore message that could not be decrypted.
			s.stats.decryptFails.WithLabelValues(l.Addr().String()).Inc()
			c.banScore.Add(1)
			continue
		}

		// Successfully decrypted message.
		n = n - 24 - 16
		c.lastRead.Store(recvTime)
		return
	}
}

// listenerReadLoop is the main reading loop for a listener. It receives data
// from a remote conn, determines if this is an internal command or a data
// packet and acts accordingly. A single listener may have multiple concurrent
// goroutines reading from it.
func (s *Server) listenerReadLoop(ctx context.Context, l *listener) error {

	var writePeers []destPeer
	tempBuf := make([]byte, rpc.RTDTMaxMessageSize)
	inPkt := newFramedPktBuffer()
	// readBuf := make([]byte, rpc.RTDTMaxMessageSize)

	statSkippedSeqPackets := s.stats.skippedSeqPackets.With(
		prometheus.Labels{"addr": l.Addr().String()})

	for {
		conn, n, startTs, err := s.listenerReadNext(inPkt, tempBuf, l)
		if err != nil {
			// We assume this is caused by the context being closed,
			// so the existing conns will also start erroring.
			return err
		}

		// Sanity checks.
		if !inPkt.hasValidSize() {
			conn.banScore.Add(1)
			continue
		}
		if !conn.recvSeq.MayAccept(inPkt.getSequence()) {
			statSkippedSeqPackets.Inc()
			s.log.Debugf("Ignoring packet from %s with unnaceptable seq number %d",
				conn, inPkt.getSequence())
			continue
		}

		// Process internal server commands.
		if inPkt.getSource() != inPkt.getTarget() {
			err := s.connHandleInternalPkt(ctx, conn, inPkt, startTs, tempBuf)
			if err != nil {
				newBS := conn.banScore.Add(1)
				if s.cfg.logReadLoopErrors {
					s.log.Warnf("Error processing internal cmd from %s: %v (banscore %d)",
						conn, err, newBS)
				}
			}
			continue
		}

		// Validate source is in the session.
		srcPS := s.sessions.sourcePeerSess(conn, inPkt.getSource())
		if srcPS == nil {
			if s.cfg.logReadLoopErrors {
				s.log.Warnf("Peer %s sent data on session %s when its not bound to it",
					conn, inPkt.getSource())
			}
			conn.banScore.Add(1)
			continue
		}

		// Validate the peer has enough allowance to relay this message.
		if !srcPS.deductAllowance(int64(n)) {
			if s.cfg.logReadLoopErrors {
				s.log.Warnf("Peer %s trying to send data when "+
					"allowance was already depleted", inPkt.getSource())
			}
			s.stats.noAllowanceBytes.Add(float64(n))
			s.stats.noAllowanceBytesAtomic.Add(uint64(n))
			conn.banScore.Add(1)
			continue
		}

		// Forward message to peers.
		//
		// TODO: send concurrently over multiple goroutines? Schedule
		// on goroutines based on # of writePeers?
		writePeers = srcPS.sess.appendDestPeers(writePeers[:0])
		if len(writePeers) > srcPS.size {
			// Limit max number of write peers to whatever max the
			// last payment was for.
			writePeers = writePeers[:srcPS.size]
			if s.cfg.logReadLoopErrors {
				s.log.Warnf("Reducing number of write peers from conn "+
					"%s to sess %s to %d", conn, inPkt.getSource(),
					srcPS.size)
			}
			conn.banScore.Add(1)
		}
		for _, wp := range writePeers {
			// Never echo back data to sender.
			if wp.c == conn {
				continue
			}

			// Write the target id and sequence number on the buffer.
			inPkt.setTarget(wp.targetID)
			inPkt.setSequence(wp.c.nextSendSeq())

			n, err := s.listenerWrite(wp.c, inPkt.outBuffer(), tempBuf)
			if err != nil && s.cfg.logReadLoopErrors {
				s.log.Warnf("Unable to write %d bytes to %s (wrote %d): %v",
					inPkt.n, wp.c, n, err)
			}

		}

		// Gather forwarding stats.
		fwdDelay := time.Since(startTs)
		fwdDelayMicro := fwdDelay / time.Microsecond
		s.stats.fwdDelay.Observe(float64(fwdDelayMicro))
	}
}

// timeoutStaleConns times out connections which have not had traffic within
// the server's ping window.
func (s *Server) timeoutStaleConns(ctx context.Context, l *listener) error {

	readTimeout := s.cfg.maxPingInterval
	tickerInterval := s.cfg.timeoutLoopTickerInterval
	if readTimeout < tickerInterval {
		tickerInterval = readTimeout
	}
	ticker := time.NewTicker(tickerInterval)

	type removedConn struct {
		k   netip.AddrPort
		c   *conn
		err error
	}

	for {
		select {
		case now := <-ticker.C:
			s.log.Tracef("Timeout stale conns tick time")
			var removedConns []removedConn
			l.conns.Range(func(k netip.AddrPort, v *conn) bool {
				d := now.Sub(v.LastReadTime())
				if d > readTimeout {
					removedConns = append(removedConns,
						removedConn{k: k, c: v, err: errConnTimedOut})
				} else if v.banScore.Load() >= s.cfg.maxBanScore {
					removedConns = append(removedConns,
						removedConn{k: k, c: v, err: errBanScoreReached})
				}
				return true
			})

			for _, c := range removedConns {
				wasPending := c.c.pending.Load() != nil
				pendingStr := ""
				if wasPending {
					pendingStr = "pending "
				}
				s.log.Infof("Removing %s%s from all sessions (reason: %v)",
					pendingStr, c.c, c.err)

				s.removeFromAllSessions(c.c)
				l.conns.Delete(c.k)

				if !wasPending {
					s.stats.conns.Dec()
					l.connsCount.Add(-1)
				} else {
					s.stats.pendingConns.Dec()
					l.pendingCount.Add(-1)
				}
			}

		case <-ctx.Done():
			err := l.l.Close()
			if err != nil {
				return err
			}

			return ctx.Err()
		}
	}
}
