package rtdtclient

import (
	"bytes"
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/RoaringBitmap/roaring/v2"
	"github.com/companyzero/bisonrelay/rpc"
	"github.com/companyzero/bisonrelay/zkidentity"
)

// pendingSession holds sessions that have not yet completed join.
type pendingSession struct {
	joinedChan chan struct{}
	sess       *Session
}

// sessionPeer represents an individual remote peer in a session.
type sessionPeer struct {
	// These must be accessed with the session mutex held.
	publisherKey *zkidentity.FixedSizeSymmetricKey
	sigKey       *zkidentity.FixedSizeEd25519PublicKey
}

// Session is a live RTDT session.
type Session struct {
	conn         *conn
	startTime    time.Time
	localID      rpc.RTDTPeerID
	publisherKey *zkidentity.FixedSizeSymmetricKey
	sigKey       *zkidentity.FixedSizeEd25519PrivateKey
	rv           *zkidentity.ShortID

	// Only used in tests.
	sessionJoinedCb func()

	mtx            sync.Mutex
	lastJoinCookie []byte
	peers          map[rpc.RTDTPeerID]*sessionPeer

	bmp    *roaring.Bitmap
	bmpBuf *bytes.Buffer

	left atomic.Bool
}

// newSession creates a new session.
func newSession(conn *conn, id rpc.RTDTPeerID, sigKey *zkidentity.FixedSizeEd25519PrivateKey,
	publisherKey *zkidentity.FixedSizeSymmetricKey, sessRV *zkidentity.ShortID) *Session {

	return &Session{
		conn:         conn,
		localID:      id,
		startTime:    time.Now(),
		peers:        make(map[rpc.RTDTPeerID]*sessionPeer),
		sigKey:       sigKey,
		publisherKey: publisherKey,
		rv:           sessRV,
		bmp:          roaring.New(),
		bmpBuf:       bytes.NewBuffer(make([]byte, 0, 168)), // Create with a common size as hint.
	}
}

// Left returns true if the client has left the session.
func (sess *Session) Left() bool {
	return sess.left.Load()
}

// LocalID returns the local id used in this session.
func (sess *Session) LocalID() rpc.RTDTPeerID {
	return sess.localID
}

// RV returns the RV of this session.
func (sess *Session) RV() *zkidentity.ShortID {
	return sess.rv
}

// getPeer returns or creates a new peer. The session mtx MUST be held to call.
func (sess *Session) getPeer(id rpc.RTDTPeerID) (*sessionPeer, bool) {
	var peer *sessionPeer
	isNew := false
	if peer = sess.peers[id]; peer == nil {
		peer = &sessionPeer{}
		sess.peers[id] = peer
		isNew = true
	}
	return peer, isNew
}

// SendSpeechPacket sends data in the speech stream.
func (sess *Session) SendSpeechPacket(ctx context.Context, opusPacket []byte, timestamp uint32) error {
	if sess.Left() {
		return errLeftSession
	}

	return sess.conn.sendData(sess, rpc.RTDTStreamSpeech, timestamp, opusPacket)
}

// SendRandomData sends data in the random stream. This is only used during
// tests.
func (sess *Session) SendRandomData(ctx context.Context, data []byte, ts uint32) error {
	if sess.Left() {
		return errLeftSession
	}
	return sess.conn.sendData(sess, rpc.RTDTStreamRandom, ts, data)
}

// SendTextMessage sends data in the text message stream.
func (sess *Session) SendTextMessage(ctx context.Context, data string) error {
	if sess.Left() {
		return errLeftSession
	}

	ts := time.Since(sess.startTime).Milliseconds()
	return sess.conn.sendData(sess, rpc.RTDTStreamChat, uint32(ts), []byte(data))
}

// RefreshSession updates the session by sending a new join cookie to the remote
// server.
func (sess *Session) RefreshSession(ctx context.Context, joinCookie []byte) error {
	if sess.Left() {
		return errLeftSession
	}

	// Uses the same message as the one for joining the session.
	return sess.conn.joinSession(ctx, sess, joinCookie)
}

// IsMemberLive returns true if the id is in the last received bitmap of
// members.
func (sess *Session) IsMemberLive(id rpc.RTDTPeerID) bool {
	sess.mtx.Lock()
	res := sess.bmp.Contains(uint32(id))
	sess.mtx.Unlock()
	return res
}

// RangeMembersList iterates through every member of the last received bitmap
// of members.
//
// Return false in the callback to stop iterating.
func (sess *Session) RangeMembersList(f func(rpc.RTDTPeerID) bool) {
	sess.mtx.Lock()
	sess.bmp.Iterate(func(id uint32) bool {
		return f(rpc.RTDTPeerID(id))
	})
	sess.mtx.Unlock()
}

// handleDecryptedPacket handles data packets that have been successfully E2E
// decrypted from remote peers.
func (c *Client) handleDecryptedPacket(ctx context.Context, sess *Session, peer *sessionPeer,
	encPacket *rpc.RTDTFramedPacket, plainPacket *rpc.RTDTDataPacket) error {

	switch plainPacket.Stream {
	case rpc.RTDTStreamRandom:
		if c.cfg.randomStreamHandler != nil {
			return c.cfg.randomStreamHandler(sess, encPacket, plainPacket)
		}

	case rpc.RTDTStreamSpeech:
		if c.cfg.audioStreamHandler != nil {
			return c.cfg.audioStreamHandler(sess, encPacket, plainPacket)
		}

	case rpc.RTDTStreamChat:
		if c.cfg.chatStreamHandler != nil {
			return c.cfg.chatStreamHandler(sess, encPacket, plainPacket)
		}

	default:
		return fmt.Errorf("unknown stream %d", plainPacket.Stream)
	}

	return nil
}
