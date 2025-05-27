package rtdtserver

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"slices"
	"sync"
	"time"

	"github.com/RoaringBitmap/roaring/v2"
	"github.com/companyzero/bisonrelay/rpc"
	"github.com/companyzero/bisonrelay/zkidentity"
	"lukechampine.com/blake3"
)

// sessionID fully identifies a session within the server.
type sessionID = zkidentity.ShortID

// sessionHasherKey is a key used to generate server-internal session IDs from
// public data. This ensures the internal server keys are not guessable from
// the outside, to prevent attempts to spoof and join sessions.
var sessionHasherKey []byte = initHasherKey()

func initHasherKey() (key []byte) {
	key = make([]byte, 32)
	rand.Read(key[:])
	return
}

// calcSessionID calculates the session ID based on the underlying parameters.
func calcSessionID(ownerSecret, serverSecret *zkidentity.ShortID, size uint32) sessionID {
	var szbuf [4]byte
	binary.BigEndian.PutUint32(szbuf[:], size)

	hasher := blake3.New(32, sessionHasherKey)
	hasher.Write(serverSecret[:])
	hasher.Write(ownerSecret[:])
	hasher.Write(szbuf[:])

	var res sessionID
	hasher.Sum(res[:0])
	return res
}

// sessionIDFromJoinCookie generates an internal server session ID from a join
// cookie.
func sessionIDFromJoinCookie(jc *rpc.RTDTJoinCookie) sessionID {
	return calcSessionID(&jc.OwnerSecret, &jc.ServerSecret, jc.Size)
}

// oldSessionFromRotCookie generates the old session id in the rotation cookie.
func oldSessionFromRotCookie(rc *rpc.RTDTRotateCookie) sessionID {
	return calcSessionID(&rc.OldOwnerSecret, &rc.ServerSecret, rc.Size)
}

// newSessionFromRotCookie generates the new session id from the rotation cookie.
func newSessionFromRotCookie(rc *rpc.RTDTRotateCookie) sessionID {
	return calcSessionID(&rc.NewOwnerSecret, &rc.ServerSecret, rc.Size)
}

// session defines the set of peers that relay messages between each other.
type session struct {
	mtx   sync.Mutex
	sid   sessionID
	peers []*peerSession

	// Bitmap and buffer for tracking and sending members listing.
	lastMemberListing time.Time
	bmp               *roaring.Bitmap
	bmpBuf            *bytes.Buffer

	// bans track any bans added when kicking a peer.
	bans map[rpc.RTDTPeerID]time.Time
}

func (s *session) id() sessionID {
	s.mtx.Lock()
	res := s.sid
	s.mtx.Unlock()
	return res
}

// appendDestPeers appends the list of current peers to the slice.
func (s *session) appendDestPeers(res []destPeer) []destPeer {
	s.mtx.Lock()
	for _, ps := range s.peers {
		res = append(res, destPeer{c: ps.c, targetID: ps.peerID})
	}
	s.mtx.Unlock()
	return res
}

// RemovePeer removes the given peer from the session. The session mutex MUST
// NOT be held.
func (s *session) RemovePeer(id rpc.RTDTPeerID) (empty bool, sid sessionID) {
	s.mtx.Lock()
	s.peers = slices.DeleteFunc(s.peers, func(ps *peerSession) bool { return ps.peerID == id })
	empty = len(s.peers) == 0
	s.bmp.Remove(uint32(id))
	s.bmpBuf.Reset()
	s.bmp.WriteTo(s.bmpBuf)
	sid = s.sid
	s.mtx.Unlock()
	return empty, sid
}

// PeerConn returns the conn for the peer with given id. The session mustex
// MUST NOT be held.
func (s *session) PeerConn(id rpc.RTDTPeerID) *conn {
	var c *conn
	s.mtx.Lock()
	idx := slices.IndexFunc(s.peers, func(ps *peerSession) bool { return ps.peerID == id })
	if idx > -1 {
		c = s.peers[idx].c
	}
	s.mtx.Unlock()
	return c
}

// BanTarget adds a ban for a target peer for the given duration. The session
// mutex MUST NOT be held.
func (s *session) BanTarget(id rpc.RTDTPeerID, duration time.Duration) {
	s.mtx.Lock()
	if s.bans == nil {
		s.bans = make(map[rpc.RTDTPeerID]time.Time)
	}
	s.bans[id] = time.Now().Add(duration)
	s.mtx.Unlock()
}

// isBanned returns true if the given peer is currently banned. The session
// mutex MUST be held.
func (s *session) isBanned(id rpc.RTDTPeerID) (banned, removedBan bool) {
	if s.bans == nil {
		return
	}
	endBan, ok := s.bans[id]
	if !ok {
		return
	}
	if time.Now().After(endBan) {
		// Remove ban.
		removedBan = true
		delete(s.bans, id)
		if len(s.bans) == 0 {
			s.bans = nil
		}
		return
	}
	banned = true
	return
}

func newSession(id sessionID, size uint32) *session {
	// Pre-size the members bitmap buffer based on the most common session
	// sizes.
	var bmpBufSlice []byte
	switch {
	case size <= 2:
		bmpBufSlice = make([]byte, 0, 32)
	case size <= 4:
		bmpBufSlice = make([]byte, 0, 48)
	case size <= 8:
		bmpBufSlice = make([]byte, 0, 88)
	case size <= 16:
		bmpBufSlice = make([]byte, 0, 168)
	default:
		// Anything larger, start with a small buffer and let usage
		// dictate how large it will be.
		bmpBufSlice = make([]byte, 0, 48)
	}
	return &session{
		sid:    id,
		bmp:    roaring.New(),
		bmpBuf: bytes.NewBuffer(bmpBufSlice),
	}
}

// sessions holds a list of sessions.
type sessions struct {
	mtx      sync.Mutex
	sessions map[sessionID]*session
}

// len returns the number of sessions in the server.
func (s *sessions) len() int {
	s.mtx.Lock()
	res := len(s.sessions)
	s.mtx.Unlock()
	return res
}

// destPeer groups a peer id with the corresponding connection used to send
// data to it.
type destPeer struct {
	c        *conn
	targetID rpc.RTDTPeerID
}

// sourcePeerSess returns the peerSession of a peer identified by an ID that is
// bound from a specific conn.
func (s *sessions) sourcePeerSess(conn *conn, source rpc.RTDTPeerID) *peerSession {
	ps, _ := conn.sessions.Load(source)
	return ps
}

// bindToSession binds a peer (from a conn) to a session.
func (s *Server) bindToSession(c *conn, sourceID rpc.RTDTPeerID, pay *payment,
	sessID sessionID, size uint32, isAdmin bool) (*session, error) {

	// Create or load session.
	s.sessions.mtx.Lock()
	sess := s.sessions.sessions[sessID]
	if sess == nil {
		sess = newSession(sessID, size)
		s.sessions.sessions[sessID] = sess
		s.log.Infof("Created session %s", sessID)
		s.stats.sessions.Inc()
	}
	s.sessions.mtx.Unlock()

	// Determine what to do inside session.
	sess.mtx.Lock()

	// If peer is banned, prevent it from rejoining.
	if banned, removedBan := sess.isBanned(sourceID); banned {
		sess.mtx.Unlock()
		return nil, errCodeBanned
	} else if removedBan {
		s.log.Infof("Removed ban of user %s from session %s", sourceID, sessID)
	}

	// Ensure this conn is part of the session.
	psIdx := slices.IndexFunc(sess.peers, func(ps *peerSession) bool { return ps.peerID == sourceID })
	contains := psIdx > -1
	if !contains {
		// New peer binding to session.
		ps := &peerSession{
			sess:    sess,
			peerID:  sourceID,
			size:    int(size),
			c:       c,
			isAdmin: isAdmin,
		}
		sess.peers = append(sess.peers, ps)
		oldPs, loaded := c.sessions.LoadAndStore(sourceID, ps)
		if loaded && oldPs.sess != sess {
			// Peer is switching from one session to another using
			// the same id. Remove from old session.
			s.log.Infof("Peer %s with peer id %s switching from "+
				"session %s to %s allowance %d admin %v",
				c, sourceID, oldPs.sess.id(), sessID,
				pay.bytesAllowance.Load(), isAdmin)
			oldPs.sess.RemovePeer(sourceID)
		} else {
			s.stats.peers.Inc()
			s.log.Infof("Bound peer %s with peer id %s and allowance "+
				"%d for size %d in session %s admin %v",
				c, sourceID, pay.bytesAllowance.Load(), size,
				sessID, isAdmin)
		}
	} else {
		// Protect against trying to change the size of the peer
		// session within the same conn.
		//
		// This should not actually ever happen with the way the session
		// ID is calculated as a function of the size.
		ps := sess.peers[psIdx]
		if ps.size != int(size) {
			return nil, errors.New("cannot change peer session size")
		}

		if ps.c != c {
			// New conn replacing an old conn with the same peer id
			// within a session.
			newPs := &peerSession{
				sess:    sess,
				peerID:  sourceID,
				size:    int(size),
				c:       c,
				isAdmin: isAdmin,
			}
			sess.peers[psIdx] = newPs
			ps.c.sessions.Delete(sourceID)
			c.sessions.Store(sourceID, ps)
			s.log.Infof("Replacing old peer %s with %s peer id %s "+
				"and allowance %d for size %d in session %s admin %v",
				ps.c, c, sourceID, pay.bytesAllowance.Load(),
				size, sessID, isAdmin)
		} else if ps.sess != sess {
			// The peer is moving from one session ID to another
			// using the same peerID.
			c.sessions.Store(sourceID, ps)
			s.log.Infof("Replacing peer %s id %s from session %s "+
				"with allowance %d for size %d to session %s admin %v",
				c, sourceID, ps.sess.id(), pay.bytesAllowance.Load(),
				size, sessID, ps.isAdmin)
		}
	}

	// Add to the members bitmap.
	sess.bmp.Add(uint32(sourceID))
	sess.bmpBuf.Reset()
	bmpN, bmpErr := sess.bmp.WriteTo(sess.bmpBuf)
	if bmpN > rpc.RTDTMaxMessageSize {
		sess.bmpBuf.Reset()
	}
	sess.mtx.Unlock()

	if bmpErr != nil {
		s.log.Warnf("Unable to write members bitmap list for session "+
			"%s after peer %d joined: %v", sessID, sourceID, bmpErr)
	} else if bmpN > rpc.RTDTMaxMessageSize {
		s.log.Warnf("Members serialized bitmap for session %s larger "+
			"than max message size (%d bytes)", bmpN)
	}

	// Add allowance.
	ps, _ := c.sessions.Load(sourceID)
	ps.addPayment(pay)
	if contains {
		s.log.Debugf("Refreshed peer session %s with new allowance %d for size %d",
			sourceID, pay.bytesAllowance.Load(), size)
	}

	return sess, nil
}

// removePeerSession removes the peer from the session based on the peerSession
// object.
func (s *Server) removePeerSession(ps *peerSession, kicked bool) {
	kickedStr := ""
	if kicked {
		kickedStr = " (KICKED)"
	}
	empty, sessID := ps.sess.RemovePeer(ps.peerID)
	s.log.Debugf("Peer %s from %s left session %s%s", ps.peerID, ps.c, sessID,
		kickedStr)

	s.stats.peers.Dec()
	if empty {
		s.sessions.mtx.Lock()
		delete(s.sessions.sessions, sessID)
		s.sessions.mtx.Unlock()
		s.stats.sessions.Dec()
		s.log.Debugf("Removing empty session %s", sessID)
	} else {
		// Resend members list.
		s.forceSessListChan <- ps.sess
	}
}

// removeFromSession removes the conn from the specified session.
func (s *Server) removeFromSession(c *conn, id rpc.RTDTPeerID, kicked bool) error {
	ps, loaded := c.sessions.LoadAndDelete(id)
	if !loaded {
		return errNotInSession
	}

	s.removePeerSession(ps, kicked)
	return nil
}

// removeFromAllSessions removes the passed conn from all its bound sessions.
func (s *Server) removeFromAllSessions(c *conn) {
	c.sessions.Range(func(id rpc.RTDTPeerID, ps *peerSession) bool {
		s.removePeerSession(ps, false)
		return true
	})
}

// kickFromSession processes the command to kick a target from a session,
// received from a member of the session.
//
// If successful, returns the conn of the peer that was kicked.
func (s *Server) kickFromSession(inConn *conn, source, kickTarget rpc.RTDTPeerID,
	banDurSeconds uint32) (*conn, error) {

	// Check if caller is admin of its session.
	ps, loaded := inConn.sessions.Load(source)
	if !loaded {
		return nil, errCodeSourcePeerNotInSession
	}
	if !ps.isAdmin {
		return nil, errCodeSourcePeerNotAdmin
	}

	// Determine target conn.
	targetConn := ps.sess.PeerConn(kickTarget)
	if targetConn == nil {
		return nil, errCodeTargetPeerNotInSession
	}

	// Add ban (to prevent rejoins).
	if banDurSeconds > 0 {
		banDuration := time.Duration(banDurSeconds) * time.Second
		s.log.Infof("Admin %s banning peer %s in session %s for %s",
			source, kickTarget, ps.sess.id(), banDuration)
		ps.sess.BanTarget(kickTarget, banDuration)
	} else {
		s.log.Infof("Admin %s kicking peer %s from session %s",
			source, kickTarget, ps.sess.id())
	}

	// Remove from session.
	err := s.removeFromSession(targetConn, kickTarget, true)
	if err != nil {
		return nil, err
	}

	return targetConn, nil
}

// rotateSessionCookies moves all peers inside a session to a new session (i.e.
// rotates the session id).
func (s *Server) rotateSessionCookies(inConn *conn, source rpc.RTDTPeerID, rotCookie []byte) error {
	// Check if caller is admin of its session.
	ps, loaded := inConn.sessions.Load(source)
	if !loaded {
		return errCodeSourcePeerNotInSession
	}
	if !ps.isAdmin {
		return errCodeSourcePeerNotAdmin
	}
	sess := ps.sess

	// Decrypt cookie.
	var rc rpc.RTDTRotateCookie
	err := rc.Decrypt(rotCookie, s.cfg.cookieKey, s.cfg.decodeCookieKeys)
	if err != nil {
		return errCodeInvalidRotCookie
	}

	// Rotate Cookie must not have expired.
	cookieTime := time.Unix(rc.Timestamp, 0)
	if time.Since(cookieTime) > s.cfg.rotateCookieLifetime {
		return errCodeExpiredRotCookie
	}
	cookieEndTime := cookieTime.Add(s.cfg.rotateCookieLifetime)

	// Rotate cookie must not have already been redeemed.
	s.rotPaymentsMtx.Lock()
	_, alreadyUsed := s.rotPayments[rc.PaymentTag]
	if !alreadyUsed {
		s.rotPayments[rc.PaymentTag] = cookieEndTime
	}
	s.rotPaymentsMtx.Unlock()
	if alreadyUsed {
		return errCodeAlreadyUsedRotCookie
	}

	// Old session id in rotate cookie must match the one the peer is on.
	oldSessId := oldSessionFromRotCookie(&rc)
	psOldSid := sess.id()
	if !oldSessId.ConstantTimeEq(&psOldSid) {
		return errCodeMismatchedOldSessId
	}

	newSessId := newSessionFromRotCookie(&rc)
	if oldSessId.ConstantTimeEq(&newSessId) {
		// Nothing else to do.
		return nil
	}

	// Actually change the id.
	sess.mtx.Lock()
	sess.sid = newSessId
	sess.mtx.Unlock()

	// There is a small chance of race here. Double check if this is not a
	// problem.

	// Change in the sessions map.
	s.sessions.mtx.Lock()
	s.sessions.sessions[newSessId] = sess
	delete(s.sessions.sessions, oldSessId)
	s.sessions.mtx.Unlock()

	s.log.Infof("Peer %s rotated session id from %s to %s", source,
		oldSessId, newSessId)

	return nil
}

// sendSessionListing sends the list of session members to every member of the
// session. The session mutex MUST be held when calling this function.
func (s *Server) sendSessionListing(sess *session, pkt *framedPktBuffer, tmp []byte) {
	if sess.bmpBuf.Len() == 0 {
		s.log.Debugf("Skipping forced sending of "+
			"members list for session %s because "+
			"session has no members bitmap", sess.sid)
	}
	if sess.bmpBuf.Len() > rpc.RTDTMaxMessageSize {
		s.log.Debugf("Skipping forced sending of "+
			"members list for session %s because "+
			"message to send members list is too large (%d)",
			sess.bmpBuf.Len())
	}

	// TODO: split into multiple goroutines if there are too many peers?
	pkt.setSource(0)
	pkt.setCmdPayload(rpc.RTDTServerCmdTypeMembersBitmap, sess.bmpBuf.Bytes())
	outMsg := pkt.outBuffer()
	for _, peer := range sess.peers {
		pkt.setTarget(peer.peerID)
		pkt.setSequence(peer.c.nextSendSeq())
		_, err := s.listenerWrite(peer.c, outMsg, tmp)
		if err != nil {
			s.log.Warnf("Unable to write session %s listing to %s: %v",
				sess.sid, peer.c, err)
		}
	}
	sess.lastMemberListing = time.Now()
	s.log.Debugf("Finished sending listing for session %s for %d peers",
		sess.sid, len(sess.peers))
}

// runSessionListingLoop runs a loop to send timed reports about members of
// sessions.
func (s *Server) runSessionListingLoop(ctx context.Context, sessListInterval,
	minListInterval time.Duration) error {

	// +1 ms to ensure time.Since() > sessListInterval always returns true.
	listTicker := time.NewTicker(sessListInterval + time.Millisecond)
	pkt := newFramedPktBuffer()
	tmp := make([]byte, rpc.RTDTMaxMessageSize)
	listSessions := make([]*session, 0, 256)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case forceSess := <-s.forceSessListChan:
			if s.cfg.disableForceListing {
				s.log.Warnf("Skipping force send members list "+
					"of session %s due to server config",
					forceSess.id())
				continue
			}

			// Ensure any reply is going out before this list.
			time.Sleep(time.Millisecond / 2)

			forceSess.mtx.Lock()
			if time.Since(forceSess.lastMemberListing) < minListInterval {
				// Prevent a peer flicking from generating too
				// many E2E packets.
				s.log.Debugf("Skipping forced sending of "+
					"members list for session %s because "+
					"time since last listing is smaller "+
					"than min interval", forceSess.sid)
			} else {
				s.sendSessionListing(forceSess, pkt, tmp)
			}
			forceSess.mtx.Unlock()

		case <-listTicker.C:
			// Go through every session to send it.
			s.sessions.mtx.Lock()
			for _, sess := range s.sessions.sessions {
				sess.mtx.Lock()
				if time.Since(sess.lastMemberListing) > sessListInterval {
					listSessions = append(listSessions, sess)
				}
				sess.mtx.Unlock()
			}
			s.sessions.mtx.Unlock()

			// Send any that are needed.
			for i, sess := range listSessions {
				sess.mtx.Lock()
				s.sendSessionListing(sess, pkt, tmp)
				sess.mtx.Unlock()
				listSessions[i] = nil
			}
			listSessions = listSessions[:0]
		}
	}
}
