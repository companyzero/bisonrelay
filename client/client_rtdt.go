package client

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math"
	"slices"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/companyzero/bisonrelay/client/clientdb"
	"github.com/companyzero/bisonrelay/client/internal/lowlevel"
	"github.com/companyzero/bisonrelay/internal/audio"
	"github.com/companyzero/bisonrelay/internal/strescape"
	"github.com/companyzero/bisonrelay/rpc"
	rtdtclient "github.com/companyzero/bisonrelay/rtdt/client"
	"github.com/companyzero/bisonrelay/zkidentity"
)

// Client RTDT session management flow is:
//
//          Alice                                       Bob
//         -------                                     -----
// CreateRTDTSession()
// InviteToRTDTSession()
//       \-------- RMRTDTSessionInvite ----->
//
//                                                 handleRMRTDTSessionInvite()
//                   <--- RMRTDTSessionInviteAccept ---/
//
// handleRMRTDTSessionInviteAccept()
//        \------- RMRTDTSession ------>
//
//                                                handleRTDTSessionUpdate()
//
//                     (Proceed to RTDT comm flows)

// GetRTDTSession returns the RTDT session info for the given session id/RV.
func (c *Client) GetRTDTSession(rv *zkidentity.ShortID) (*clientdb.RTDTSession, error) {
	var sess *clientdb.RTDTSession
	err := c.dbView(func(tx clientdb.ReadTx) error {
		var err error
		sess, err = c.db.GetRTDTSession(tx, rv)
		return err
	})
	return sess, err
}

// GetRTDTSessionByPrefix returns the RTDT session info for the session which
// RV has the given prefix.
//
// This errors if there is more than one session with the same prefix.
func (c *Client) GetRTDTSessionByPrefix(prefix string) (*clientdb.RTDTSession, error) {
	var sess *clientdb.RTDTSession
	err := c.dbView(func(tx clientdb.ReadTx) error {
		var err error
		sess, err = c.db.GetRTDTSessionByPrefix(tx, prefix)
		return err
	})
	return sess, err
}

// ListRTDTSessions lists the known RTDT sessions by RV.
func (c *Client) ListRTDTSessions() []zkidentity.ShortID {
	var res []zkidentity.ShortID
	err := c.dbView(func(tx clientdb.ReadTx) error {
		res = c.db.ListRTDTSessions(tx)
		return nil
	})
	if err != nil {
		c.log.Errorf("Unable to list RTDT sessions: %v", err)
	}
	return res
}

// CreateRTDTSession creates a new RTDT session. This involves fetching a
// creation token from brserver.
//
// The local client does NOT automatically join the live session (but it has
// the ability to do so by calling JoinLiveRTDTSession).
//
// The size of the session determines how much payment is needed to create and
// send data ("publish") in the session. It cannot be modified after creation.
func (c *Client) CreateRTDTSession(size uint16, description string) (*clientdb.RTDTSession, error) {
	// Generate keys.
	publisherKey := zkidentity.NewFixedSizeSymmetricKey()

	// Create session in brserver.
	brsSess, err := c.rtmgr.CreateSession(size)
	if err != nil {
		return nil, fmt.Errorf("unable to create RTDT session in brserver: %v", err)
	}

	// The peer ID of the creator is the first one in the session.
	localPeerID := c.mustRandomUintRTDTPeerID(size)
	ownerSecret := zkidentity.RandomShortID()

	// Request an appointment cookie for the local peer.
	cookieReq := &rpc.GetRTDTAppointCookies{
		SessionCookie: brsSess.SessionCookie,
		OwnerSecret:   ownerSecret,
		Peers: []rpc.RTDTAppointmentCookiePeer{{
			ID:                 localPeerID,
			AllowedAsPublisher: true,
			IsAdmin:            true,
		}},
	}
	cookieRes, err := c.rtmgr.GetAppointCookies(cookieReq)
	if err != nil {
		return nil, err
	}
	appointCookie := cookieRes.AppointCookies[0]

	// Create the database entry.
	nowTs := time.Now().Unix()
	sess := &clientdb.RTDTSession{
		LocalPeerID:   localPeerID,
		NextPeerID:    localPeerID.Next(),
		PublisherKey:  publisherKey,
		OwnerSecret:   &ownerSecret,
		SessionCookie: brsSess.SessionCookie,
		AppointCookie: appointCookie,

		Metadata: rpc.RMRTDTSession{
			RV:          brsSess.SessionRV,
			Generation:  1,
			Size:        uint32(size),
			Description: description,

			// Local client is the owner of the session.
			Owner: c.PublicID(),

			// Local client will be a publisher by default.
			Publishers: []rpc.RMRTDTSessionPublisher{{
				PublisherID:  c.PublicID(),
				Alias:        c.LocalNick(),
				PeerID:       localPeerID,
				PublisherKey: *publisherKey,
			}},
		},

		Members: []clientdb.RTDTSessionMember{{
			// First member is the owner.
			UID:               c.PublicID(),
			PeerID:            localPeerID,
			AcceptedTimestamp: &nowTs,
			Publisher:         true,
		}},
	}

	err = c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		return c.db.UpdateRTDTSession(tx, sess)
	})
	if err != nil {
		return nil, err
	}

	c.log.Infof("Created RTDT session %s of size %d (peer ID %s)",
		sess.Metadata.RV, size, sess.LocalPeerID)

	return sess, nil
}

// inviteToRTDTSession invites an user to join an RTDT session. The local
// client must be the owner of the session.
func (c *Client) inviteToRTDTSession(session zkidentity.ShortID,
	allowedAsPublisher bool, inviteeIDs ...UserID) error {

	// Double check every user exists.
	for _, id := range inviteeIDs {
		_, err := c.UserByID(id)
		if err != nil {
			return err
		}
	}

	type invitee struct {
		id      UserID
		peerID  rpc.RTDTPeerID
		tag     uint64
		cookie  []byte
		isAdmin bool
	}

	// Track invitations in session.
	var sess *clientdb.RTDTSession
	var invitees []invitee
	err := c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		var err error
		sess, err = c.db.GetRTDTSession(tx, &session)
		if err != nil {
			return err
		}

		if !sess.LocalIsAdmin() {
			return errNotAdmin
		}
		if sess.OwnerSecret == nil {
			return errors.New("owner secret cannot be nil to invite to session")
		}

		// If the RTDT session has an associated GC, ensure the invitee
		// is already a member.
		if sess.GC != nil {
			gc, err := c.db.GetGC(tx, *sess.GC)
			if err != nil {
				return err
			}

			for _, inviteeID := range inviteeIDs {
				if !slices.Contains(gc.Metadata.Members, inviteeID) {
					return ErrRTDTInviteeNotGCMember{
						GC:      *sess.GC,
						Invitee: inviteeID,
					}
				}
			}
		}

		for _, inviteeID := range inviteeIDs {
			memberIdx, _ := sess.MemberIndices(&inviteeID)
			tag := c.mustRandomUint64() &^ (0xffff << 48) // Don't pick a large nb because JSON.
			inv := invitee{
				id:      inviteeID,
				tag:     tag,
				isAdmin: slices.Contains(sess.Metadata.Admins, inviteeID),
			}
			if memberIdx > -1 {
				// Resend invite if user was already invited.
				m := &sess.Members[memberIdx]
				m.SentTimestamp = time.Now().Unix()
				m.Publisher = allowedAsPublisher
				m.Tag = tag
				inv.peerID = m.PeerID
			} else {
				// New user being invited.
				if len(sess.Members) >= int(sess.Metadata.Size) {
					return errors.New("session already full")
				}

				inv.peerID = sess.NextPeerID
				sess.NextPeerID = inv.peerID.Next()
				sess.Members = append(sess.Members, clientdb.RTDTSessionMember{
					UID:           inviteeID,
					SentTimestamp: time.Now().Unix(),
					Publisher:     allowedAsPublisher,
					PeerID:        inv.peerID,
					Tag:           tag,
				})
			}

			invitees = append(invitees, inv)
		}

		return c.db.UpdateRTDTSession(tx, sess)
	})
	if err != nil {
		return err
	}

	// Request an appointment cookie from brserver for this session.
	cookieReq := &rpc.GetRTDTAppointCookies{
		SessionCookie: sess.SessionCookie,
		OwnerSecret:   *sess.OwnerSecret,
	}
	for _, inv := range invitees {
		cookieReq.Peers = append(cookieReq.Peers, rpc.RTDTAppointmentCookiePeer{
			ID:                 inv.peerID,
			AllowedAsPublisher: allowedAsPublisher,
			IsAdmin:            inv.isAdmin,
		})
	}
	cookieRes, err := c.rtmgr.GetAppointCookies(cookieReq)
	if err != nil {
		return err
	}
	for i := range cookieRes.AppointCookies {
		invitees[i].cookie = cookieRes.AppointCookies[i]
	}

	// With the received appointment cookies, prepare the send invitation
	// items to remote users.
	sendqItems := make([]*preparedSendqItem, 0, len(invitees)+len(sess.Metadata.Admins))
	payEvent := fmt.Sprintf("rtdt.invite.%s", session)

	for _, inv := range invitees {
		ru, err := c.UserByID(inv.id)
		if err != nil {
			continue
		}

		rm := rpc.RMRTDTSessionInvite{
			RV:                 sess.Metadata.RV,
			Size:               sess.Metadata.Size,
			Description:        sess.Metadata.Description,
			GC:                 sess.GC,
			AllowedAsPublisher: allowedAsPublisher,
			AppointCookie:      inv.cookie,
			PeerID:             inv.peerID,
			Tag:                inv.tag,
		}

		ru.log.Infof("Inviting to RTDT session %s with peer ID %s (asPublisher %v)",
			session, inv.peerID, allowedAsPublisher)

		sqi, err := c.prepareSendqItem(payEvent, rm, priorityGC,
			nil, inv.id)
		if err != nil {
			// Log, but keep going to next member.
			c.log.Warnf("Unable to add RMRTDTSessionInvite"+
				"to member %s: %v", inv.id, err)
			continue
		}
		sendqItems = append(sendqItems, sqi)
	}

	// Send an update to all admins about the new user.
	myID := c.PublicID()
	payEvent = fmt.Sprintf("rtdt.admincookies.%s", session)
	adminCookiesRM := rpc.RMRTDTAdminCookies{
		RV:         session,
		Members:    sess.RMMembersList(),
		NextPeerID: &sess.NextPeerID,
	}
	for _, admin := range sess.Metadata.AllAdmins() {
		if admin == myID {
			continue
		}

		sqi, err := c.prepareSendqItem(payEvent, adminCookiesRM, priorityGC,
			nil, admin)
		if err != nil {
			// Log, but keep going to next member.
			c.log.Warnf("Unable to add RMRTDTAdminCookies "+
				"to member %s: %v", admin, err)
			continue
		}
		sendqItems = append(sendqItems, sqi)
	}

	// Queue the RMs to remote users.
	for _, sqi := range sendqItems {
		err := c.sendPreparedSendqItem(sqi)
		if err != nil {
			c.log.Warnf("Unable to send RMRTDTSessionInvite "+
				"to member %s: %v", sqi.dests[0], err)

			// Keep going.
		}
	}

	return nil
}

// InviteToRTDTSession invites an user to join an RTDT session. The local
// client must be the owner of the session.
func (c *Client) InviteToRTDTSession(session zkidentity.ShortID,
	allowedAsPublisher bool, inviteeIDs ...UserID) error {
	return c.inviteToRTDTSession(session, allowedAsPublisher, inviteeIDs...)
}

// CreateRTDTSessionInGC creates a new RTDT session based on the given GC.
// extraSize is the additional size to add to the session (in addition to the
// size of the GC). If membersAsPublishers is true, every current member will
// receive an invitation to join the realtime session.
func (c *Client) CreateRTDTSessionInGC(gcID zkidentity.ShortID, extraSize uint16,
	membersAsPublishers bool) (*clientdb.RTDTSession, error) {

	// Sanity checks.
	gc, err := c.getGC(gcID)
	if err != nil {
		return nil, err
	}
	if gc.RTDTSessionRV != nil {
		return nil, ErrGCAlreadyHasRTDTSession
	}
	if gc.Metadata.Members[0] != c.PublicID() {
		return nil, fmt.Errorf("only GC owner can create an RTDT session " +
			"associated with GC")
	}

	size := uint32(extraSize) + uint32(len(gc.Metadata.Members))
	if size > math.MaxUint16 {
		return nil, fmt.Errorf("size is too large (%d)", size)
	}

	// Create the RTDT session.
	descr := fmt.Sprintf("GC %q (%s)", gc.Metadata.Name, gcID.ShortLogID())
	sess, err := c.CreateRTDTSession(uint16(size), descr)
	if err != nil {
		return nil, err
	}
	sessRV := sess.Metadata.RV

	// Update GC and session with their links to each other.
	err = c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		var err error
		gc, err = c.db.GetGC(tx, gcID)
		if err != nil {
			return err
		}

		sess, err = c.db.GetRTDTSession(tx, &sessRV)
		if err != nil {
			return err
		}
		if gc.RTDTSessionRV != nil {
			return errors.New("GC already updated with RTDT session")
		}
		gc.RTDTSessionRV = &sessRV
		sess.GC = &gcID

		// If there are extra admins in the GC, make them extra admins
		// here as well. NOTE: we assume the extra admins will be
		// invited to the RTDT session below and will accept it.
		if len(gc.Metadata.ExtraAdmins) > 0 {
			sess.Metadata.Admins = gc.Metadata.ExtraAdmins
		}

		if err := c.db.SaveGC(tx, gc); err != nil {
			return err
		}
		if err := c.db.UpdateRTDTSession(tx, sess); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return sess, err
	}

	c.log.Infof("Linked GC %s with RTDT session %s", gcID, sessRV)

	// If instructed to add the existing members as publishers, invite all
	// members as publishers.
	if membersAsPublishers {
		myID := c.PublicID()
		toAdd := slices.DeleteFunc(gc.Metadata.Members, func(id zkidentity.ShortID) bool {
			return id == myID
		})
		err = c.inviteToRTDTSession(sessRV, true, toAdd...)
		if err != nil {
			err = fmt.Errorf("unable to invite GC members to session: %v", err)
		}
	}

	return sess, err
}

// handleRMRTDTSessionInvite handles invitations received from remote users to
// join a new RTDT session.
func (c *Client) handleRMRTDTSessionInvite(ru *RemoteUser, invite rpc.RMRTDTSessionInvite) error {
	err := c.dbView(func(tx clientdb.ReadTx) error {
		// Double check we haven't joined the session yet.
		sess, err := c.db.GetRTDTSession(tx, &invite.RV)
		if err != nil && !errors.Is(err, clientdb.ErrNotFound) {
			return err
		}
		if sess != nil && sess.Metadata.Generation > 0 {
			return clientdb.ErrAlreadyExists
		}
		if sess != nil && !sess.Metadata.IsOwnerOrAdmin(ru.ID()) {
			return errors.New("new session invite not from original inviter")
		}
		return nil
	})
	if err != nil {
		return err
	}

	ru.log.Infof("Received invitation to RTDT session %s peerID %d (asPublisher %v)",
		invite.RV, invite.PeerID, invite.AllowedAsPublisher)
	c.ntfns.notifyInvitedToRTDTSession(ru, &invite)
	return nil
}

// AcceptRTDTSessionInvite accepts an invite to join an RTDT session. If
// acceptAsPublisher is false, then the user joins the session only receiving
// data.
func (c *Client) AcceptRTDTSessionInvite(inviter UserID, invite *rpc.RMRTDTSessionInvite, acceptAsPublisher bool) error {
	// Sanity checks.
	if acceptAsPublisher && !invite.AllowedAsPublisher {
		return errors.New("not invited as publisher")
	}
	ru, err := c.UserByID(inviter)
	if err != nil {
		return err
	}
	if invite.GC != nil {
		// Check associated GC exists.
		gc, err := c.getGC(*invite.GC)
		if err != nil {
			return fmt.Errorf("invite is for session associated "+
				"with GC %s but client is not a member of this GC",
				invite.GC)
		}

		// Check inviter is admin on that GC.
		if err := c.uidHasGCPerm(&gc.Metadata, inviter); err != nil {
			return fmt.Errorf("invite is for session associated "+
				"with GC %s but client is not an admin of this GC: %v",
				invite.GC, err)
		}

		// Check GC already has a RTDT session.
		if gc.RTDTSessionRV != nil && *gc.RTDTSessionRV != invite.RV {
			return fmt.Errorf("invite is for session associated "+
				"with GC %s but GC already has associated session %s",
				invite.GC, gc.RTDTSessionRV)
		}
	}

	rm := rpc.RMRTDTSessionInviteAccept{
		RV:  invite.RV,
		Tag: invite.Tag,
	}
	if acceptAsPublisher {
		rm.PublisherKey = zkidentity.NewFixedSizeSymmetricKey()
	}

	err = c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		sess, err := c.db.GetRTDTSession(tx, &invite.RV)
		if err != nil && !errors.Is(err, clientdb.ErrNotFound) {
			return err
		}

		// Ignore if session exists with generation == 0 because it is
		// an accepted invitation for which we have not received the
		// actual data yet.
		if sess != nil && sess.Metadata.Generation > 0 {
			return clientdb.ErrAlreadyExists
		}

		// Create initial version of the session data. This is used
		// to validate updates.
		sess = &clientdb.RTDTSession{
			PublisherKey: rm.PublisherKey,
			LocalPeerID:  invite.PeerID,
			GC:           invite.GC,

			AppointCookie: invite.AppointCookie,

			Metadata: rpc.RMRTDTSession{
				RV:          invite.RV,
				Size:        invite.Size,
				Description: invite.Description,
				Owner:       inviter,
			},
		}
		if err := c.db.UpdateRTDTSession(tx, sess); err != nil {
			return err
		}

		// If there's a GC associated with this session, link them.
		if invite.GC != nil {
			gc, err := c.db.GetGC(tx, *invite.GC)
			if err != nil {
				return err
			}

			gc.RTDTSessionRV = &invite.RV
			err = c.db.SaveGC(tx, gc)
			if err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return err
	}

	ru.log.Infof("Accepting invite to RTDT session %s as peer %s (asPublisher %v)",
		invite.RV, invite.PeerID, acceptAsPublisher)

	// Send reply accepting.
	payEvent := fmt.Sprintf("rtdt.accept.%s", invite.RV.String())
	return c.sendWithSendQ(payEvent, rm, inviter)
}

// sendRTDTSessionUpdate sends an update of the given session to the specified
// targets.
func (c *Client) sendRTDTSessionUpdate(metadata rpc.RMRTDTSession, targets []UserID) error {
	payEvent := fmt.Sprintf("rtdt.sessionmeta.%s", metadata.RV.String())
	return c.sendWithSendQ(payEvent, metadata, targets...)
}

// handleRMRTDTAcceptInvite handles remote clients accepting our invitation to
// join a RTDT session.
func (c *Client) handleRMRTDTAcceptInvite(ru *RemoteUser, accept rpc.RMRTDTSessionInviteAccept) error {
	var accepted, needsResend = false, false
	var sess *clientdb.RTDTSession
	var oldMeta rpc.RMRTDTSession
	var peerID rpc.RTDTPeerID
	err := c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		var err error
		sess, err = c.db.GetRTDTSession(tx, &accept.RV)
		if err != nil {
			return err
		}

		if !sess.LocalIsAdmin() {
			return errNotAdmin
		}

		oldMeta = sess.Metadata

		for i := range sess.Members {
			m := &sess.Members[i]
			if m.UID != ru.ID() {
				continue
			}
			if accept.Tag != m.Tag {
				return fmt.Errorf("wrong tag value (got %d, want %d)",
					accept.Tag, m.Tag)
			}
			if m.AcceptedTimestamp != nil {
				return errors.New("already accepted invite")
			}
			nowts := time.Now().Unix()
			m.AcceptedTimestamp = &nowts
			if !m.Publisher && accept.PublisherKey != nil {
				ru.log.Warnf("User sent publisher key " +
					"to RTDT session %s when it was not " +
					"invited as publisher. Ignoring key.")
				accept.PublisherKey = nil
			}

			peerID = m.PeerID

			if accept.PublisherKey != nil {
				sess.Metadata.Generation += 1
				sess.Metadata.Publishers = append(sess.Metadata.Publishers, rpc.RMRTDTSessionPublisher{
					PublisherID:  ru.ID(),
					PublisherKey: *accept.PublisherKey,
					Alias:        ru.Nick(),
					PeerID:       m.PeerID,
				})
				needsResend = true
			}

			accepted = true
			break
		}
		if !accepted {
			return errors.New("user not found in list of sent invites")
		}

		return c.db.UpdateRTDTSession(tx, sess)
	})
	if err != nil {
		return err
	}

	ru.log.Infof("Accepted invite to join RTDT session %s as peer %s (asPublisher %v)",
		accept.RV, peerID, accept.PublisherKey != nil)

	c.ntfns.notifyRTDTSessionInviteAccepted(ru, accept.RV, accept.PublisherKey != nil)
	ntfnUpdate := c.ntfns.buildRTDTSessionUpdateNtfn(&oldMeta, &sess.Metadata)
	c.ntfns.notifyRTDTSessionUpdated(ru, &ntfnUpdate)

	// Resend session metadata to all participants (includes new user as
	// publisher).
	if needsResend {
		err := c.sendRTDTSessionUpdate(sess.Metadata, sess.MemberUIDs(c.PublicID()))
		if err != nil {
			return err
		}
	} else {
		// Send only to client who accepted.
		err := c.sendRTDTSessionUpdate(sess.Metadata, []zkidentity.ShortID{ru.ID()})
		if err != nil {
			return err
		}
	}

	// If this user has been added as an admin, send it the necessary
	// metadata to admin the GC.
	if slices.Contains(sess.Metadata.Admins, ru.ID()) {
		rm := rpc.RMRTDTAdminCookies{
			RV:            accept.RV,
			SessionCookie: sess.SessionCookie,
			OwnerSecret:   sess.OwnerSecret,
			Members:       sess.RMMembersList(),
			NextPeerID:    &sess.NextPeerID,
		}
		payEvent := fmt.Sprintf("rtdt.rotatecookies.%s", accept.RV)
		err := c.sendWithSendQ(payEvent, rm, ru.ID())
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *Client) handleRTDTSessionUpdate(ru *RemoteUser, newMeta rpc.RMRTDTSession) error {
	var oldMeta rpc.RMRTDTSession
	var sess *clientdb.RTDTSession
	var joined bool
	err := c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		var err error
		sess, err = c.db.GetRTDTSession(tx, &newMeta.RV)
		if err != nil {
			return err
		}

		oldMeta = sess.Metadata

		if !sess.Metadata.IsOwnerOrAdmin(ru.ID()) {
			return errors.New("user is not allowed to update session")
		}

		if sess.Metadata.Generation > newMeta.Generation {
			return fmt.Errorf("received  old session generation (old is %d, current is %d)",
				newMeta.Generation, sess.Metadata.Generation)
		}

		if sess.Metadata.Size != newMeta.Size {
			return errors.New("cannot change size of session")
		}

		// Only owner can change the owner.
		if oldMeta.Owner != newMeta.Owner && ru.ID() != oldMeta.Owner {
			return errors.New("only session owner can change the owner")
		}

		joined = sess.Metadata.Generation == 0

		// TODO: Check if we were removed/muted.
		sess.Metadata = newMeta

		return c.db.UpdateRTDTSession(tx, sess)
	})

	if err != nil {
		return err
	}

	ntfnUpdate := c.ntfns.buildRTDTSessionUpdateNtfn(&oldMeta, &sess.Metadata)
	ntfnUpdate.InitialJoin = joined
	if joined {
		c.log.Infof("Joined RTDT session %s as peer %s (asPublisher %v)",
			newMeta.RV, sess.LocalPeerID, sess.PublisherKey != nil)
	} else {
		ru.log.Infof("Received RTDT session update for session %s (%d "+
			"new publishers, %d removed)", newMeta.RV,
			len(ntfnUpdate.NewPublishers), len(ntfnUpdate.RemovedPublishers))
	}
	c.ntfns.notifyRTDTSessionUpdated(ru, &ntfnUpdate)

	return nil
}

// rtdtMgrHandlersAdapter is an adapter structure to setup Client callbacks to
// the lowlevel RTDT session manager without exposing them in the public API.
type rtdtMgrHandlersAdapter struct {
	*Client
}

// JoinedLiveSession called by the RTDT manager when the local client has
// successfully joined a live session.
func (c *rtdtMgrHandlersAdapter) JoinedLiveSession(rtSess *rtdtclient.Session, rv zkidentity.ShortID) {
	liveSess := &liveRTDTSession{
		sessRV: rv,
		rtSess: rtSess,
		peers:  make(map[rpc.RTDTPeerID]*LiveRTDTPeer),
	}
	c.rtMtx.Lock()
	c.rtLiveSessions[rv] = liveSess
	c.rtMtx.Unlock()

	c.ntfns.notifyRTDTLiveSessionJoined(rv)
}

// RefreshedAllowance called by the RTDT manager when it has refreshed the
// allowance to publish data in a session.
func (c *rtdtMgrHandlersAdapter) RefreshedAllowance(rv zkidentity.ShortID, addAllowance uint64) {
	c.ntfns.notifyRTDTRefreshedSessionAllowance(rv, addAllowance)
}

// rtdtSessionMembersListUpdated is called when an updated list of members is
// received for a live RTDT session.
func (c *Client) rtdtSessionMembersListUpdated(sess *rtdtclient.Session) error {
	rv := *sess.RV()
	dbSess, err := c.GetRTDTSession(&rv)
	if err != nil {
		return err
	}

	c.rtMtx.Lock()
	liveSess := c.rtLiveSessions[rv]
	c.rtMtx.Unlock()

	if liveSess == nil {
		// This can happen if we receive a list before JoinedLiveSession
		// is called by the lowlevel rt manager.
		//
		// The next listing received will fix this.
		return nil
	}

	// Modify the live session, adding and removing live peers as needed.
	var delIds, newIds []rpc.RTDTPeerID
	liveSess.mtx.Lock()
	for livePeerId, peer := range liveSess.peers {
		if !sess.IsMemberLive(livePeerId) {
			if peer.ps != nil {
				go peer.ps.MarkInputDone(c.ctx)
			}
			peer.ps = nil
			delIds = append(delIds, livePeerId)
		}
	}
	for _, delId := range delIds {
		delete(liveSess.peers, delId)
	}
	sess.RangeMembersList(func(id rpc.RTDTPeerID) bool {
		if _, ok := liveSess.peers[id]; ok {
			// Already have this peer.
			return true
		}

		if !dbSess.IsPeerPublisher(id) {
			// Not a publisher.
			//
			// TODO: notify with new spectators?
			return true
		}

		if id == dbSess.LocalPeerID {
			// Ignore local peer id in list.
			return true
		}

		liveSess.peers[id] = &LiveRTDTPeer{}
		newIds = append(newIds, id)
		return true
	})
	liveSess.mtx.Unlock()

	// Send notifications.
	c.log.Debugf("New RTDT members list for session %s received (%d new, %d removed)",
		rv, len(newIds), len(delIds))
	for _, delId := range delIds {
		c.log.Debugf("RTDT session %s removed peer %s with list update",
			rv, delId)
		c.ntfns.notifyRTDTLivePeerStalled(rv, delId)
	}
	for _, newId := range newIds {
		c.ntfns.notifyRTDTLivePeerJoined(rv, newId)
	}

	return nil
}

// rtdtKickedFromSession is called when the local client is kicked from a live
// RTDT session.
func (c *Client) rtdtKickedFromSession(sess *rtdtclient.Session, banDuration time.Duration) {
	rv := *sess.RV()

	c.rtMtx.Lock()
	liveSess := c.rtLiveSessions[rv]
	if liveSess != nil && c.rtHotAudio == liveSess {
		c.removeFromHotAudio(rv)
	}
	c.rtdtRemoveLiveSess(rv)
	c.rtMtx.Unlock()

	_ = c.rtmgr.ForceUnmaintainSession(&rv) // Ok to ignore shutdown errors

	if liveSess == nil {
		return
	}

	c.ntfns.notifyRTDTKickedFromLiveSession(rv, sess.LocalID(), banDuration)
}

// LiveRTDTPeer tracks information about a remote RTDT peer that is live in the
// session.
type LiveRTDTPeer struct {
	// HasSoundStream is set to true if the peer is sending sound stream
	// data.
	HasSoundStream bool `json:"has_sound_stream"`

	// HasSound is true if this peer is currently outputting sound at a
	// detectable level.
	HasSound bool `json:"has_sound"`

	// Volume gain for this peer (in dB).
	VolumeGain float64 `json:"volume_gain"`

	// The following fields are accessed with the sessions mutex held.

	// ps is the playback stream associated to this peer.
	ps *audio.PlaybackStream

	// lastSoundTime tracks when the last sound packet was received.
	lastSoundTime time.Time
}

// RTDTChatMessage is an RTDT chat message received and tracked by the client.
type RTDTChatMessage struct {
	SourceID  rpc.RTDTPeerID `json:"source_id"`
	Message   string         `json:"message"`
	Timestamp int64          `json:"timestamp"`
}

// liveRTDTSession tracks data for each live RTDT session (session the local
// client has joined).
type liveRTDTSession struct {
	sessRV zkidentity.ShortID
	rtSess *rtdtclient.Session

	mtx   sync.Mutex
	peers map[rpc.RTDTPeerID]*LiveRTDTPeer
	msgs  []RTDTChatMessage
}

// Peer returns the given peer from the session or nil.
//
// The session mtx MUST NOT be held.
func (liveSess *liveRTDTSession) Peer(id rpc.RTDTPeerID) *LiveRTDTPeer {
	if liveSess == nil {
		return nil
	}

	liveSess.mtx.Lock()
	res := liveSess.peers[id]
	liveSess.mtx.Unlock()
	return res
}

// LiveRTDTSession stores information about a live RTDT session.
type LiveRTDTSession struct {
	// HotAudio is true if this session currently has hot audio linked to
	// it.
	HotAudio bool `json:"hot_audio"`

	// Peers is the list of known live remote peers.
	Peers map[rpc.RTDTPeerID]LiveRTDTPeer `json:"peers"`

	// RTSess is the underlying RTDT client session.
	RTSess *rtdtclient.Session `json:"-"`
}

// IsPeerLive returns true if the peer is live. Can be used even if liveSess is
// nil.
func (liveSess *LiveRTDTSession) IsPeerLive(id rpc.RTDTPeerID) bool {
	if liveSess == nil {
		return false
	}

	_, ok := liveSess.Peers[id]
	return ok
}

// PeerHasSound returns true if the peer is live, has a sound stream and has
// sound detected.
func (liveSess *LiveRTDTSession) PeerHasSound(id rpc.RTDTPeerID) bool {
	if liveSess == nil {
		return false
	}

	peer, ok := liveSess.Peers[id]
	if !ok {
		return false
	}

	return peer.HasSoundStream && peer.HasSound
}

// JoinLiveRTDTSession joins the live RTDT session.
func (c *Client) JoinLiveRTDTSession(sessRV zkidentity.ShortID) error {
	var sess *clientdb.RTDTSession
	err := c.dbView(func(tx clientdb.ReadTx) error {
		var err error
		sess, err = c.db.GetRTDTSession(tx, &sessRV)
		return err
	})
	if err != nil {
		return err
	}

	req := &rpc.AppointRTDTServer{
		AppointCookie: sess.AppointCookie,
	}

	return c.rtmgr.MaintainSession(sessRV, req, sess.PublisherKey,
		sess.Metadata.Size, sess.LocalPeerID)
}

// newRTDTPeer is called when an rtdt session detects data from a new peer.
func (c *Client) newRTDTPeer(id rpc.RTDTPeerID, sessRV *zkidentity.ShortID) (
	*zkidentity.FixedSizeEd25519PublicKey, *zkidentity.FixedSizeSymmetricKey) {

	// Find the identity and publisher key for this peer in this session.
	var encKey *zkidentity.FixedSizeSymmetricKey
	var peerUserID *zkidentity.ShortID
	err := c.dbView(func(tx clientdb.ReadTx) error {
		sess, err := c.db.GetRTDTSession(tx, sessRV)
		if err != nil {
			return err
		}

		for _, pub := range sess.Metadata.Publishers {
			if pub.PeerID != id {
				continue
			}

			// Found it!
			encKey = new(zkidentity.FixedSizeSymmetricKey)
			copy(encKey[:], pub.PublisherKey[:])
			peerUserID = new(zkidentity.ShortID)
			copy(peerUserID[:], pub.PublisherID[:])
			break
		}

		return nil
	})

	if err != nil {
		c.log.Warnf("Failed to find new RTDT peer %s keys: %v", id, err)
		return nil, nil
	}
	if encKey == nil {
		c.log.Warnf("Received RTDT data from peer %s in session %s without known publisher key",
			id, sessRV)
	} else {
		// Track peer as online in live session.
		c.rtMtx.Lock()
		liveSess := c.rtLiveSessions[*sessRV]
		c.rtMtx.Unlock()
		if liveSess != nil {
			liveSess.mtx.Lock()
			c.log.Debugf("Initing live peer %s for session %s", id, sessRV.ShortLogID())
			if _, ok := liveSess.peers[id]; !ok {
				liveSess.peers[id] = &LiveRTDTPeer{}
			}
			liveSess.mtx.Unlock()
		}
	}

	// If we have KX'd with this user, find their signature key to validate
	// the origin of data.
	var sigKey *zkidentity.FixedSizeEd25519PublicKey
	if peerUserID != nil {
		if ru, _ := c.UserByID(*peerUserID); ru != nil {
			sigKey = ru.SignatureKey()
			c.log.Infof("Detected new publisher peer %s in RT session %s known user %s (%s)",
				id, sessRV.ShortLogID(), strescape.Nick(ru.Nick()), peerUserID)
		} else {
			c.log.Infof("Detected new publisher peer %s in RT session %s unkxd user %s",
				id, sessRV.ShortLogID(), peerUserID)
		}
	} else {
		c.log.Infof("Detected new publisher peer %s in RT session %s "+
			"without finding its publisher metadata (id %s)",
			id, sessRV.ShortLogID(), peerUserID)

		// TODO: request updated metadata from owner/admin?
	}

	// Notify UI the user has joined.
	c.ntfns.notifyRTDTLivePeerJoined(*sessRV, id)

	return sigKey, encKey
}

// initRTDTLivePeerPlaybackStream inits the playbackstream (ps) of a live peer.
// This does NOT modify the live peer ps field.
func (c *Client) initRTDTLivePeerPlaybackStream(sessRV zkidentity.ShortID, peerID rpc.RTDTPeerID) *audio.PlaybackStream {
	// Callback to notify when audio starts/ends.
	soundStateChanged := func(hasSound bool) {
		c.rtMtx.Lock()
		liveSess := c.rtLiveSessions[sessRV]
		c.rtMtx.Unlock()

		if liveSess == nil {
			return
		}

		notify := false
		liveSess.mtx.Lock()
		if peer := liveSess.peers[peerID]; peer != nil {
			peer.HasSound = hasSound
			notify = true
		}
		liveSess.mtx.Unlock()

		if notify {
			c.ntfns.notifyRTDTPeerSoundChanged(sessRV, peerID, true, hasSound)
		}
	}

	return c.noterec.PlaybackStream(c.ctx, soundStateChanged)
}
func (c *Client) rtdtAudioStreamHandler(sess *rtdtclient.Session,
	enc *rpc.RTDTFramedPacket, plain *rpc.RTDTDataPacket) error {

	sessRV := *sess.RV()
	peerID := enc.Source

	c.rtMtx.Lock()
	liveSess := c.rtLiveSessions[sessRV]
	c.rtMtx.Unlock()

	if liveSess == nil {
		// This can happen if we receive data before
		// JoinedLiveSession() is called by the lowlevel rt manager. In
		// that case, the first packet received after JoinedLiveSession
		// is called will init the playback stream.
		return nil
	}

	// Init audio playback stream if it does not exist.
	liveSess.mtx.Lock()
	lp := liveSess.peers[peerID]
	if lp == nil {
		lp = &LiveRTDTPeer{}
		liveSess.peers[peerID] = lp
	}
	if lp.ps == nil {
		lp.ps = c.initRTDTLivePeerPlaybackStream(sessRV, enc.Source)
	}
	lp.lastSoundTime = time.Now()
	if !lp.HasSoundStream {
		// Sound stream was off, force a notification that sound is back
		// on.
		c.ntfns.notifyRTDTPeerSoundChanged(sessRV, peerID, true, lp.HasSound)
		lp.HasSoundStream = true
	}
	liveSess.mtx.Unlock()

	// Send input data to stream (data gets copied by stream).
	lp.ps.Input(plain.Data, plain.Timestamp)
	return nil
}

// rtdtRemoveLiveSess removes the live session from the client. rtMtx MUST be
// held when calling this.
func (c *Client) rtdtRemoveLiveSess(sessRV zkidentity.ShortID) {
	sess := c.rtLiveSessions[sessRV]
	if sess == nil {
		return
	}

	delete(c.rtLiveSessions, sessRV)
	go func() {
		sess.mtx.Lock()
		c.log.Debugf("Marking %d streams for session %s as done",
			len(sess.peers), sessRV.ShortLogID())
		for _, peer := range sess.peers {
			if peer.ps != nil {
				peer.ps.MarkInputDone(c.ctx)
			}
		}
		sess.mtx.Unlock()
	}()
}

// remakeHotAudioAfterWriteErr runs a temporary loop waiting to see if the passed
// live session will stop failing writes. If it does (within a time interval),
// the session is re-made as having hot audio.
//
// This is called in response to write errors on the hot session, in situations
// where there are temporary network failures or the local network is changing.
func (c *Client) remakeHotAudioAfterWriteErr(liveSess *liveRTDTSession) {
	// Try once a second, for 5 seconds to see if the conn came back.
	c.log.Debugf("Remaking session %s as hot audio session if reconnection works",
		liveSess.sessRV)
	buf := make([]byte, 100)
	for i := 0; i < 5; i++ {
		select {
		case <-c.ctx.Done():
			return
		case <-time.After(time.Second):
		}

		// Double check session wasn't turned off.
		if liveSess.rtSess.Left() {
			c.log.Debugf("Session %s left while waiting to remake it hot",
				liveSess.sessRV)
			return
		}

		// Try sending data on the session. If it succeeds, the session
		// is back.
		rand.Read(buf)
		err := liveSess.rtSess.SendRandomData(c.ctx, buf, 0)
		if err != nil {
			c.log.Debugf("Still unable to remake session %s as hot audio: %v",
				liveSess.sessRV, err)
			continue
		}

		// Session is back. If user did not change to a different hot
		// audio session, then make this one hot.
		c.rtMtx.Lock()
		remade := c.rtReHotAudio == liveSess && c.rtHotAudio == nil
		if remade {
			c.log.Infof("Remaking session %s as the hot audio session after reconnection",
				liveSess.sessRV)
			c.makeAudioHot(liveSess.sessRV)
		} else {
			c.rtReHotAudio = nil
		}
		c.rtMtx.Unlock()

		if remade {
			c.ntfns.notifyRTDTRemadeLiveSessionHot(liveSess.sessRV)
		}
		return
	}

	c.log.Debugf("Giving up remaking session %s hot after reconnection",
		liveSess.sessRV)
}

// sendHotMicData is called as a callback from c.rtCapStream when new mic data
// is ready to be sent remotely.
func (c *Client) sendHotMicData(ctx context.Context, opusPacket []byte, timestamp uint32) error {
	c.rtMtx.Lock()
	if c.rtHotAudio != nil {
		err := c.rtHotAudio.rtSess.SendSpeechPacket(ctx, opusPacket, timestamp)
		if err != nil {
			// Error sending to this session. Remove it from hot
			// audio.
			rv := c.rtHotAudio.sessRV
			oldHot := c.rtHotAudio

			c.log.Errorf("Errored sending speech packet: %v", err)
			c.removeFromHotAudio(c.rtHotAudio.sessRV)
			c.rtReHotAudio = oldHot
			c.ntfns.notifyRTDTLiveSendErrored(rv, err)
			go c.remakeHotAudioAfterWriteErr(oldHot)
		}
	}
	c.rtMtx.Unlock()

	return nil
}

// makeAudioHot makes the given session as "audio hot". MUST be called with
// the rtMtx held.
func (c *Client) makeAudioHot(sessRV zkidentity.ShortID) error {
	liveSess := c.rtLiveSessions[sessRV]
	if liveSess == nil {
		return fmt.Errorf("session %s is not live", sessRV)
	}

	// Prevent re-attaching a previously hot session.
	c.rtReHotAudio = nil

	// Avoid duplicate entries as hot.
	if liveSess == c.rtHotAudio {
		// Already hot.
		return nil
	}

	// If the capture stream was in the shutdown stage, cancel the shutdown
	// and resume with the same stream (to avoid stalling remote peers when
	// muting/unmuting in a short interval).
	if c.rtCloseCapChan != nil {
		c.rtCloseCapChan <- struct{}{}
		c.rtCloseCapChan = nil
	}

	// Start capture stream (capturing from mic) if not yet running.
	if c.rtCapStream == nil {
		c.log.Debugf("Starting audio capture stream")
		cs, err := c.noterec.CaptureStream(c.ctx, c.sendHotMicData)
		if err != nil {
			return fmt.Errorf("unable to start capture stream: %v", err)
		}

		c.rtCapStream = cs
	}

	// All good. Add to list of sessions that have a hot mic.
	c.rtHotAudio = liveSess
	return nil
}

// removeFromHotAudio removes the given session from the hot audio sessions.
//
// rtMtx MUST be held when calling this function.
func (c *Client) removeFromHotAudio(sessRV zkidentity.ShortID) {
	if c.rtHotAudio != nil && c.rtHotAudio.sessRV == sessRV {
		c.rtHotAudio = nil
	}
	if c.rtHotAudio == nil && c.rtCapStream != nil {
		// Last hot mic audio session was removed, stop capturing.
		c.log.Debugf("Removing capture stream from last audio hot session")

		if c.rtCloseCapChan == nil {
			// Only close the capture stream after a few seconds,
			// to ensure if we come back very fast the timestamp
			// will keep ticking and not stall remote peers.
			closeCapChan := make(chan interface{}, 2)
			c.rtCloseCapChan = closeCapChan
			go func() {
				select {
				case <-time.After(10 * time.Second):
					c.rtMtx.Lock()
					if c.rtCloseCapChan == closeCapChan {
						c.log.Debugf("Closing audio capture stream")
						if c.rtCapStream != nil {
							c.rtCapStream.Stop()
							c.rtCapStream = nil
						}
						c.rtCloseCapChan = nil
					}
					c.rtMtx.Unlock()

				case <-closeCapChan:
					// Someone restarted capturing.
					c.log.Debugf("Cancelling closing audio capture stream")
				}
			}()
		}
	}
}

// ChangePlaybackDeviceID changes the playback device id of all current and
// future streams to be the given one.
func (c *Client) ChangePlaybackDeviceID(devID audio.DeviceID) error {
	err := c.noterec.SetPlaybackDevice(devID)
	if err != nil {
		return err
	}

	c.rtMtx.Lock()
	allSess := make([]*liveRTDTSession, 0, len(c.rtLiveSessions))
	for _, sess := range c.rtLiveSessions {
		allSess = append(allSess, sess)
	}
	c.rtMtx.Unlock()

	for _, sess := range allSess {
		sess.mtx.Lock()
		for _, peer := range sess.peers {
			if peer.ps != nil {
				peer.ps.ChangePlaybackDevice(devID)
			}
		}
		sess.mtx.Unlock()
	}

	return nil
}

// SwitchHotAudio disables hot audio from all sessions and sets it to the
// session with the specified session RV.
//
// If an empty RV is passed, then this disables hot audio mic.
func (c *Client) SwitchHotAudio(sessRV zkidentity.ShortID) error {
	c.rtMtx.Lock()
	var err error
	if !sessRV.IsEmpty() {
		err = c.makeAudioHot(sessRV)
	} else if c.rtHotAudio != nil {
		c.removeFromHotAudio(c.rtHotAudio.sessRV)
	}
	c.rtMtx.Unlock()
	return err
}

// HasHotAudioRTDT returns true if there is a live RTDT session with hot audio.
func (c *Client) HasHotAudioRTDT() bool {
	c.rtMtx.Lock()
	res := c.rtHotAudio != nil
	c.rtMtx.Unlock()
	return res
}

// ListLiveRTSessions returns a list of live sessions. It is a map by RV and
// a bool to tell if that session has hot mic.
func (c *Client) ListLiveRTSessions() map[zkidentity.ShortID]bool {
	c.rtMtx.Lock()
	res := make(map[zkidentity.ShortID]bool, len(c.rtLiveSessions))
	for _, sess := range c.rtLiveSessions {
		res[sess.sessRV] = sess == c.rtHotAudio
	}
	c.rtMtx.Unlock()
	return res
}

// GetLiveRTSession returns information about a live RTDT session. If the
// session is not live, this returns nil.
func (c *Client) GetLiveRTSession(sessRV *zkidentity.ShortID) *LiveRTDTSession {
	c.rtMtx.Lock()
	liveSess := c.rtLiveSessions[*sessRV]
	hasHotAudio := liveSess != nil && c.rtHotAudio == liveSess
	c.rtMtx.Unlock()

	if liveSess == nil {
		return nil
	}

	// Create copy of live session.
	liveSess.mtx.Lock()
	res := &LiveRTDTSession{
		RTSess:   liveSess.rtSess,
		HotAudio: hasHotAudio,
		Peers:    make(map[rpc.RTDTPeerID]LiveRTDTPeer, len(liveSess.peers)),
	}
	for k, v := range liveSess.peers {
		res.Peers[k] = *v
	}
	liveSess.mtx.Unlock()

	return res
}

// IsLiveAndHotRTSession returns whether the given session RV is live and if
// it has hot audio.
func (c *Client) IsLiveAndHotRTSession(sessRV *zkidentity.ShortID) (isLive, isHotAudio bool) {
	c.rtMtx.Lock()
	_, isLive = c.rtLiveSessions[*sessRV]
	isHotAudio = c.rtHotAudio != nil && c.rtHotAudio.sessRV == *sessRV
	c.rtMtx.Unlock()
	return
}

// LeaveLiveRTSession leaves the given live session.
func (c *Client) LeaveLiveRTSession(sessRV zkidentity.ShortID) error {
	c.rtMtx.Lock()
	if c.rtHotAudio != nil && c.rtHotAudio.sessRV == sessRV {
		c.removeFromHotAudio(sessRV)
	}
	c.rtMtx.Unlock()

	// Send command to leave session to rtmgr even if we don't have a live
	// session recorded yet to ensure joining attempts are canceled.
	err := c.rtmgr.LeaveSession(&sessRV)
	if err == nil {
		c.rtMtx.Lock()
		c.rtdtRemoveLiveSess(sessRV)
		c.rtMtx.Unlock()
	}

	return err
}

// ModifyRTDTLivePeerVolumeGain modifies the playback audio gain for the given
// peer. It adds or subtracts from the current gain (in dB).
func (c *Client) ModifyRTDTLivePeerVolumeGain(sessRV *zkidentity.ShortID,
	peerID rpc.RTDTPeerID, delta float64) float64 {

	c.rtMtx.Lock()
	liveSess := c.rtLiveSessions[*sessRV]
	c.rtMtx.Unlock()
	if liveSess == nil {
		return 0
	}

	var newGain float64
	liveSess.mtx.Lock()
	if livePeer := liveSess.peers[peerID]; livePeer != nil {
		newGain = livePeer.VolumeGain + delta
		livePeer.VolumeGain = newGain
		if livePeer.ps != nil {
			livePeer.ps.SetVolumeGain(newGain)
		}
	}
	liveSess.mtx.Unlock()

	return newGain
}

// KickFromLiveRTDTSession kicks a peer from a live RTDT session.
func (c *Client) KickFromLiveRTDTSession(sessRV *zkidentity.ShortID, target rpc.RTDTPeerID, banDuration time.Duration) error {
	liveSess := c.GetLiveRTSession(sessRV)
	if liveSess == nil {
		return errors.New("live session not found")
	}

	return c.rtc.KickMember(c.ctx, liveSess.RTSess, target, banDuration)
}

// ExitRTDTSession permanently removes the local client from the given RTDT
// session. If the client is in the live session, this also makes it leave the
// session.
func (c *Client) ExitRTDTSession(sessRV *zkidentity.ShortID) error {
	// Leave live session first.
	liveSess := c.GetLiveRTSession(sessRV)
	if liveSess != nil {
		err := c.LeaveLiveRTSession(*sessRV)
		if err != nil {
			return err
		}
	}

	// Remove from DB.
	var sess *clientdb.RTDTSession
	err := c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		var err error
		sess, err = c.db.GetRTDTSession(tx, sessRV)
		if err != nil {
			return err
		}

		if err := c.db.RemoveRTDTSession(tx, sessRV); err != nil {
			return err
		}

		// Detach from GC.
		if sess.GC != nil {
			gc, _ := c.db.GetGC(tx, *sess.GC)
			if gc.RTDTSessionRV != nil && *gc.RTDTSessionRV == *sess.GC {
				gc.RTDTSessionRV = nil
				c.db.SaveGC(tx, gc)
			}
		}

		return nil
	})
	if err != nil {
		return err
	}
	c.log.Infof("Exited from RTDT session %s", sessRV)

	if sess.Metadata.Owner == c.PublicID() {
		// All done (we were the owner).
		return nil
	}

	// Send RM to owner.
	payEvent := fmt.Sprintf("rtdt.exit.%s", sessRV)
	rm := rpc.RMRTDTExitSession{RV: *sessRV}
	return c.sendWithSendQ(payEvent, rm, sess.Metadata.Owner)
}

// removeFromRTDTSession removes a member from an RTDT session. The local client
// must be a session admin for this to work.
func (c *Client) removeFromRTDTSession(sessRV *zkidentity.ShortID, memberID *UserID) (sess *clientdb.RTDTSession, peerID rpc.RTDTPeerID, wasPublisher bool, err error) {
	err = c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		var err error
		sess, err = c.db.GetRTDTSession(tx, sessRV)
		if err != nil {
			return err
		}

		if !sess.Metadata.IsOwnerOrAdmin(c.PublicID()) {
			return errNotAdmin
		}

		idxMember, idxPublisher := sess.MemberIndices(memberID)
		if idxMember < 0 {
			return errNotAMember
		}
		peerID = sess.Members[idxMember].PeerID
		sess.Members = slices.Delete(sess.Members, idxMember, idxMember+1)
		if idxPublisher > -1 {
			sess.Metadata.Publishers = slices.Delete(sess.Metadata.Publishers, idxPublisher, idxPublisher+1)
			sess.Metadata.Generation += 1
			wasPublisher = true
		}

		return c.db.UpdateRTDTSession(tx, sess)
	})
	return sess, peerID, wasPublisher, err
}

// handleRMRTDTExitSession handles the message for a client that wishes to exit
// an RTDT session.
func (c *Client) handleRMRTDTExitSession(ru *RemoteUser, rmes rpc.RMRTDTExitSession) error {
	memberID := ru.ID()
	sess, peerID, wasPublisher, err := c.removeFromRTDTSession(&rmes.RV, &memberID)
	if errors.Is(err, errNotAdmin) {
		ru.log.Warnf("User sent request to leave RTDT session %s "+
			"when local client is not an admin", rmes.RV)
		return nil
	}
	if errors.Is(err, errNotAMember) {
		ru.log.Warnf("User sent request to leave RTDT session %s "+
			"when they were not a member", rmes.RV)
		return nil
	}
	if err != nil {
		return err
	}

	ru.log.Infof("Removed user with peer id %s from RTDT session %s",
		peerID, rmes.RV)
	c.ntfns.notifyRTDTPeerExitedSession(ru, rmes.RV, peerID)

	// Send update to all existing members if this generated a metadata
	// change.
	if wasPublisher {
		return c.sendRTDTSessionUpdate(sess.Metadata, sess.MemberUIDs(c.PublicID()))
	}

	return nil
}

// rtdtHandleChatMsgReceived is called by the RTDT client when a chat message is
// received.
func (c *Client) rtdtHandleChatMsgReceived(sess *rtdtclient.Session, enc *rpc.RTDTFramedPacket, plain *rpc.RTDTDataPacket) error {
	sessRV := sess.RV()
	dbSess, err := c.GetRTDTSession(sessRV)
	if err != nil {
		return err
	}

	var pub *rpc.RMRTDTSessionPublisher
	for i := range dbSess.Metadata.Publishers {
		if dbSess.Metadata.Publishers[i].PeerID == enc.Source {
			pub = &dbSess.Metadata.Publishers[i]
			break
		}
	}

	if pub == nil {
		// Should not happen.
		return errors.New("publisher not found in session metadata")
	}

	if !utf8.Valid(plain.Data) {
		return errors.New("publisher sent invalid UTF-8 data")
	}
	msg := string(plain.Data)

	// If we were configured to track received messages directly, do so
	// now.
	if c.cfg.TrackRTDTChatMessages {
		c.rtMtx.Lock()
		liveSess := c.rtLiveSessions[*sessRV]
		c.rtMtx.Unlock()
		if liveSess != nil {
			rtMsg := RTDTChatMessage{
				SourceID:  pub.PeerID,
				Message:   msg,
				Timestamp: time.Now().Unix(),
			}
			liveSess.mtx.Lock()
			liveSess.msgs = append(liveSess.msgs, rtMsg)
			liveSess.mtx.Unlock()
		}
	}

	c.ntfns.notifyRTDTChatMsgReceived(*sessRV, *pub, msg, plain.Timestamp)
	return nil
}

// DissolveRTDTSession completely dissolves the given RTDT session to all
// members.
func (c *Client) DissolveRTDTSession(sessRV *zkidentity.ShortID) error {
	// Leave live session first.
	liveSess := c.GetLiveRTSession(sessRV)
	if liveSess != nil {
		err := c.LeaveLiveRTSession(*sessRV)
		if err != nil {
			return err
		}
	}

	// Remove from DB.
	var sess *clientdb.RTDTSession
	err := c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		var err error
		sess, err = c.db.GetRTDTSession(tx, sessRV)
		if err != nil {
			return err
		}

		if !sess.Metadata.IsOwnerOrAdmin(c.PublicID()) {
			return errNotAdmin
		}

		err = c.db.RemoveRTDTSession(tx, sessRV)
		if err != nil {
			return err
		}

		// Detach from GC.
		if sess.GC != nil {
			gc, err := c.db.GetGC(tx, *sess.GC)
			if err != nil {
				// This can happen if the GC is killed before
				// the RTDT session.
				c.log.Warnf("Unable to get associated GC %s "+
					"when dissolving RTDT session: %v",
					sess.GC, err)
			}
			if gc.RTDTSessionRV != nil && *gc.RTDTSessionRV == *sessRV {
				gc.RTDTSessionRV = nil
				err := c.db.SaveGC(tx, gc)
				if err != nil {
					c.log.Errorf("Unable to save GC: %v", err)
				} else {
					c.log.Infof("Detached RTDT session %s from GC %s",
						sessRV, sess.GC)
				}
			} else if gc.RTDTSessionRV != nil {
				c.log.Warnf("GC %s has associated session %s "+
					"when expected was dissolved session %s",
					sess.GC, gc.RTDTSessionRV, sessRV)
			}
		}

		return nil
	})
	if err != nil {
		return err
	}
	c.log.Infof("Dissolved RTDT session %s", sessRV)

	payEvent := fmt.Sprintf("rtdt.dissolve.%s", sessRV)
	rm := rpc.RMRTDTDissolveSession{RV: *sessRV}
	return c.sendWithSendQ(payEvent, rm, sess.MemberUIDs(c.PublicID())...)
}

// handleRMRTDTDissolveSession handles the RM to dissolve an RTDTSession.
func (c *Client) handleRMRTDTDissolveSession(ru *RemoteUser, rmds rpc.RMRTDTDissolveSession) error {
	var peerID rpc.RTDTPeerID
	err := c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		var err error
		sess, err := c.db.GetRTDTSession(tx, &rmds.RV)
		if err != nil {
			return err
		}

		if !sess.Metadata.IsOwnerOrAdmin(ru.ID()) {
			return errors.New("user is not allowed to dissolve session")
		}

		peerID = sess.LocalPeerID

		if err := c.db.RemoveRTDTSession(tx, &rmds.RV); err != nil {
			return err
		}

		// Detach from GC.
		if sess.GC != nil {
			gc, err := c.db.GetGC(tx, *sess.GC)
			if err != nil {
				// This can happen when the GC was dissolved
				// first.
				c.log.Warnf("Unable to fetch associated GC %s "+
					"during RTDT session dissolve: %v",
					sess.GC, err)
			}
			if gc.RTDTSessionRV != nil && *gc.RTDTSessionRV == rmds.RV {
				gc.RTDTSessionRV = nil
				err := c.db.SaveGC(tx, gc)
				if err != nil {
					c.log.Errorf("Unable to save GC %s without "+
						"associated RTDT session %s: %v",
						sess.GC, rmds.RV, err)
				}
			} else if gc.RTDTSessionRV != nil {
				c.log.Warnf("GC %s has associated session %s "+
					"when expected was dissolved session %s",
					sess.GC, gc.RTDTSessionRV, rmds.RV)
			}
		}

		return nil
	})
	if err != nil {
		return err
	}

	ru.log.Infof("RTDT session %s was dissolved", rmds.RV)

	// Session was dissolved, leave it if it is live.
	liveSess := c.GetLiveRTSession(&rmds.RV)
	if liveSess != nil {
		err := c.LeaveLiveRTSession(rmds.RV)
		if err != nil {
			return err
		}
	}

	c.ntfns.notifyRTDTSessionDissolved(ru, rmds.RV, peerID)
	return nil
}

// RemoveRTDTMember removes a member from an RTDT session. If the local client
// is in the live session, It Automatically kicks and temporarily bans the
// member.
//
// It does NOT automatically rotate secret keys to join the new live session.
func (c *Client) RemoveRTDTMember(sessRV *zkidentity.ShortID, memberID *UserID,
	reason string) error {

	// Remove from DB.
	sess, peerID, wasPublisher, err := c.removeFromRTDTSession(sessRV, memberID)
	if err != nil {
		return err
	}
	c.log.Infof("Removed member %s from RTDT session %s (reason %q)",
		memberID, sessRV, reason)

	// Kick member from live session if we are in it. A ban of 2 hours will
	// cause the appointment cookie to expire.
	liveSess := c.GetLiveRTSession(sessRV)
	if liveSess != nil {
		err := c.KickFromLiveRTDTSession(sessRV, peerID, 2*time.Hour)
		if err != nil {
			// Do not return error directly, but keep going to send
			// updated lists to members.
			c.log.Errorf("Unable to kick removed peer %s from RTDT "+
				"session %s: %v", peerID, sessRV, err)
		}
	} else {
		c.log.Warnf("Skipping kicking and temp banning peer %s (uid %s) "+
			"from RTDT session %s because local client is not in live session",
			peerID, memberID, sessRV)
	}

	// Send alert to removed member.
	payEvent := fmt.Sprintf("rtdt.removedmember.%s", sessRV)
	rm := rpc.RMRTDTRemovedFromSession{RV: *sessRV, Reason: reason}
	if err := c.sendWithSendQ(payEvent, rm, *memberID); err != nil {
		return err
	}

	// Send updated list of members to other admins.
	payEvent = fmt.Sprintf("rtdt.admincookies.%s", sessRV)
	adminCookiesRM := rpc.RMRTDTAdminCookies{
		RV:      *sessRV,
		Members: sess.RMMembersList(),
	}
	if err := c.sendWithSendQ(payEvent, adminCookiesRM, sess.Metadata.AllAdmins()...); err != nil {
		// Not critical (keep going to ensure session update goes to
		// all others).
		c.log.Errorf("Unable to send admin cookies of RTDT session %s after removing user: %v",
			sessRV, err)
	}

	// Send update to all members (only if it caused a metadata update).
	if wasPublisher {
		return c.sendRTDTSessionUpdate(sess.Metadata, sess.MemberUIDs(c.PublicID()))
	}

	return nil
}

// handleRMRTDTRemovedFromSession handles the RM when the local client was
// removed from an RTDT session by its admin.
func (c *Client) handleRMRTDTRemovedFromSession(ru *RemoteUser, rmrs rpc.RMRTDTRemovedFromSession) error {
	err := c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		sess, err := c.db.GetRTDTSession(tx, &rmrs.RV)
		if err != nil {
			return err
		}

		if !sess.Metadata.IsOwnerOrAdmin(ru.ID()) {
			return errNotAdmin
		}

		return c.db.RemoveRTDTSession(tx, &rmrs.RV)
	})
	if err != nil {
		return err
	}

	ru.log.Warnf("User removed local client from RTDT session %s (reason %q)",
		rmrs.RV, rmrs.Reason)

	// We were removed from the session. If we're still live, in it, leave
	// it.
	liveSess := c.GetLiveRTSession(&rmrs.RV)
	if liveSess != nil {
		err := c.LeaveLiveRTSession(rmrs.RV)
		if err != nil {
			return err
		}
	}

	c.ntfns.notifyRTDTRemovedFromSession(ru, rmrs.RV, rmrs.Reason)
	return nil
}

// rotateRTDTCookies rotates the appointment cookies for all members of the
// given sessions, including generating a new owner secret to change the server
// session.
//
// Members specified in skipMembers won't receive the new appointment cookies
// (and will likely be unable to join the live session again).
func (c *Client) rotateRTDTAppointmentCookies(sessRV *zkidentity.ShortID, skipMembers ...zkidentity.ShortID) error {
	// Sanity checks.
	sess, err := c.GetRTDTSession(sessRV)
	if err != nil {
		return err
	}
	if !sess.Metadata.IsOwnerOrAdmin(c.PublicID()) {
		return errors.New("cannot rotate cookies when not a RTDT session admin")
	}
	if isLive, _ := c.IsLiveAndHotRTSession(sessRV); !isLive {
		return errors.New("cannot rotate cookies when not in live RTDT session")
	}

	c.log.Infof("Starting to rotate cookies on RTDT session %s", sessRV)

	// Create request for new appointment cookies for every member.
	newOwnerSecret := zkidentity.RandomShortID()
	cookieReq := &rpc.GetRTDTAppointCookies{
		SessionCookie:  sess.SessionCookie,
		OwnerSecret:    newOwnerSecret,
		OldOwnerSecret: sess.OwnerSecret,
		Peers:          make([]rpc.RTDTAppointmentCookiePeer, 0, len(sess.Members)),
	}
	for _, member := range sess.Members {
		isAdmin := member.PeerID == sess.LocalPeerID

		if slices.Contains(skipMembers, member.UID) {
			// Prevent wrong usage (rotating cookie and leaving out
			// admin).
			if isAdmin {
				return errors.New("cannot skip rotating cookie for admin")
			}

			// Do not rotate for this member.
			continue
		}

		cookieReq.Peers = append(cookieReq.Peers, rpc.RTDTAppointmentCookiePeer{
			ID:                 member.PeerID,
			AllowedAsPublisher: member.Publisher,
			IsAdmin:            isAdmin,
		})
	}

	// Request updated cookies.
	cookieRes, err := c.rtmgr.GetAppointCookies(cookieReq)
	if err != nil {
		return err
	}

	if len(cookieRes.RotateCookie) == 0 {
		return errors.New("server did not return rotate cookie with reply")
	}

	if len(cookieRes.AppointCookies) != len(cookieReq.Peers) {
		return fmt.Errorf("server sent wrong number of appointment cookies (got %d, want %d)",
			len(cookieRes.AppointCookies), len(cookieReq.Peers))
	}

	// Aux map from peer id to index in sess.Members.
	var membersMap map[rpc.RTDTPeerID]*clientdb.RTDTSessionMember

	// Save session with new, updated cookies and secret.
	err = c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		// Reload sess because it could've been modified while waiting
		// for the cookies.
		var err error
		sess, err = c.db.GetRTDTSession(tx, sessRV)
		if err != nil {
			return err
		}

		// Update owner secret.
		sess.OwnerSecret = &newOwnerSecret

		// Update the member's and local client's cookies.
		membersMap = sess.MembersMap()
		for i, reqPeer := range cookieReq.Peers {
			if reqPeer.ID == sess.LocalPeerID {
				sess.AppointCookie = cookieRes.AppointCookies[i]
			}

			member, ok := membersMap[reqPeer.ID]
			if !ok {
				// This peer was removed while we were waiting
				// for the cookies.
				c.log.Infof("Peer %s was removed from session "+
					"%s while waiting for reply of appointment cookies",
					reqPeer.ID, sessRV)
				continue
			}
			member.AppointCookie = cookieRes.AppointCookies[i]
		}

		return c.db.UpdateRTDTSession(tx, sess)
	})
	if err != nil {
		return err
	}

	// Let every peer know of their new appointment cookie.
	//
	// First, add all messages to the sendq so that the client will send
	// them if it shuts down later.
	sendqItems := make([]*preparedSendqItem, 0, len(sess.Members)-1+len(sess.Metadata.Admins))
	payEvent := fmt.Sprintf("rtdt.rotatecookies.%s", sessRV)
	for _, memberReq := range cookieReq.Peers {
		if memberReq.ID == sess.LocalPeerID {
			// Skip self.
			continue
		}

		member, ok := membersMap[memberReq.ID]
		if !ok {
			// Not a member anymore.
			continue
		}

		rm := rpc.RMRTDTRotateAppointCookie{
			RV:            *sessRV,
			AppointCookie: member.AppointCookie,
		}
		sqi, err := c.prepareSendqItem(payEvent, rm, priorityGC,
			nil, member.UID)
		if err != nil {
			// Log, but keep going to next member.
			c.log.Warnf("Unable to add RMRTDTRotateAppointCookie "+
				"to member %s: %v", member.UID, err)
			continue
		}
		sendqItems = append(sendqItems, sqi)
	}

	// Ensure every admin will receive the new OwnerSecret in order for them
	// to continue working.
	myID := c.PublicID()
	payEvent = fmt.Sprintf("rtdt.admincookies.%s", sessRV)
	adminCookiesRM := rpc.RMRTDTAdminCookies{
		RV:          *sessRV,
		OwnerSecret: sess.OwnerSecret,
	}
	for _, admin := range sess.Metadata.AllAdmins() {
		if admin == myID {
			continue
		}

		sqi, err := c.prepareSendqItem(payEvent, adminCookiesRM, priorityGC,
			nil, admin)
		if err != nil {
			// Log, but keep going to next member.
			c.log.Warnf("Unable to add RMRTDTAdminCookies "+
				"to member %s: %v", admin, err)
			continue
		}
		sendqItems = append(sendqItems, sqi)
	}

	// Queue the RMs to remote users.
	for _, sqi := range sendqItems {
		err := c.sendPreparedSendqItem(sqi)
		if err != nil {
			c.log.Warnf("Unable to send RMRTDTRotateAppointCookie "+
				"to member %s: %v", sqi.dests[0], err)

			// Keep going.
		}
	}

	// Rotate the live session. This will move every peer currently live
	// into the new session id.
	liveSess := c.GetLiveRTSession(sessRV)
	if liveSess == nil {
		// Can only happen if getting the appointment cookies takes too
		// long and user commands to leave live session.
		return fmt.Errorf("not in live RTDT session %s anymore to rotate",
			sessRV)
	}
	err = c.rtc.AdminRotateSessionCookie(c.ctx, liveSess.RTSess, cookieRes.RotateCookie)
	if err != nil {
		return err
	}

	// Rotate our own appointment cookie on rtmanager.
	err = c.rtmgr.RotateAppointCookie(sessRV, sess.AppointCookie)
	if err != nil {
		return err
	}

	// All done.
	c.log.Infof("Finished rotating cookies in RTDT session %s", sessRV)
	return nil
}

// RotateRTDTCookies rotates the appointment cookies for all members of the
// given sessions, including generating a new owner secret to change the server
// session.
func (c *Client) RotateRTDTAppointmentCookies(sessRV *zkidentity.ShortID) error {
	return c.rotateRTDTAppointmentCookies(sessRV)
}

// handleRMRTDTRotateAppointCookie handles the message to rotate the appointment
// cookie for an RTDT session.
func (c *Client) handleRMRTDTRotateAppointCookie(ru *RemoteUser, rmrac rpc.RMRTDTRotateAppointCookie) error {
	// Update the database with new appointment cookie.
	err := c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		sess, err := c.db.GetRTDTSession(tx, &rmrac.RV)
		if err != nil {
			return err
		}

		if !sess.Metadata.IsOwnerOrAdmin(ru.ID()) {
			return errNotAdmin
		}

		sess.AppointCookie = rmrac.AppointCookie
		return c.db.UpdateRTDTSession(tx, sess)
	})
	if err != nil {
		return err
	}

	// Let the RTDT manager know we have an updated appointment cookie.
	err = c.rtmgr.RotateAppointCookie(&rmrac.RV, rmrac.AppointCookie)
	if errors.Is(err, lowlevel.ErrNoRtdtSessToRotateCookie) {
		// Ok to ignore this error (means this session is not live).
		err = nil
	}
	if err != nil {
		return err
	}

	ru.log.Infof("Received new appointment cookie to rotate RTDT session %s",
		rmrac.RV)
	c.ntfns.notifyRTDTRotatedCookie(ru, rmrac.RV)
	return nil
}

// detectStalledRTDTPeers runs a loop to detect RTDTPeers that have stalled.
func (c *Client) detectStalledRTDTPeers(ctx context.Context) error {
	const stallInterval = time.Second / 2
	const checkInterval = time.Second / 4

	// Channel to get notification of new live sessions. This is used to
	// avoid checking peer stalls until there is at least one live session.
	sessionJoinedChan := make(chan struct{}, 1)
	c.ntfns.Register(OnRTDTLiveSessionJoined(func(sessRV zkidentity.ShortID) {
		select {
		case sessionJoinedChan <- struct{}{}:
		case <-ctx.Done():
		}
	}))

	ticker := time.NewTicker(checkInterval)
	for {
		// Block until there are realtime chat sessions running.
		select {
		case <-sessionJoinedChan:
		case <-ctx.Done():
			return ctx.Err()
		}

		// Run the stall loop until there are no more sessions.
		hasSessions := true
		for hasSessions {
			var now time.Time
			select {
			case <-ctx.Done():
				return ctx.Err()
			case now = <-ticker.C:
			}

			c.rtMtx.Lock()
			for _, liveSess := range c.rtLiveSessions {
				liveSess.mtx.Lock()
				for peerID, peer := range liveSess.peers {
					if peer.HasSoundStream && now.Sub(peer.lastSoundTime) > stallInterval {
						peer.HasSoundStream = false
						c.log.Debugf("Setting peer %s from RTDT session %s as stalled",
							peerID, liveSess.sessRV)
						c.ntfns.notifyRTDTPeerSoundChanged(liveSess.sessRV, peerID,
							false, false)
					}
				}
				liveSess.mtx.Unlock()
			}
			hasSessions = len(c.rtLiveSessions) > 0
			c.rtMtx.Unlock()
		}
	}
}

// SendRTDTChatMsg sends a chat message in a live RTDT session.
func (c *Client) SendRTDTChatMsg(sessRV zkidentity.ShortID, msg string) error {
	c.rtMtx.Lock()
	liveSess := c.rtLiveSessions[sessRV]
	c.rtMtx.Unlock()

	if liveSess == nil {
		return errors.New("RTDT session is not live")
	}

	if c.cfg.TrackRTDTChatMessages {
		rtMsg := RTDTChatMessage{
			SourceID:  liveSess.rtSess.LocalID(),
			Message:   msg,
			Timestamp: time.Now().Unix(),
		}
		liveSess.mtx.Lock()
		liveSess.msgs = append(liveSess.msgs, rtMsg)
		liveSess.mtx.Unlock()
	}

	return liveSess.rtSess.SendTextMessage(c.ctx, msg)
}

// GetRTDTMessages returns the list of tracked RTDT messages. This will only
// return results if the session is live and if the client was configured to
// track RTDT messages.
func (c *Client) GetRTDTMessages(sessRV zkidentity.ShortID) []RTDTChatMessage {
	if !c.cfg.TrackRTDTChatMessages {
		return nil
	}

	c.rtMtx.Lock()
	liveSess := c.rtLiveSessions[sessRV]
	c.rtMtx.Unlock()

	if liveSess == nil {
		return nil
	}

	liveSess.mtx.Lock()
	res := slices.Clone(liveSess.msgs)
	liveSess.mtx.Unlock()

	return res
}

// ModifyRTDTSessionAdmins modifies the list of admins of the given RTDT esssion.
func (c *Client) ModifyRTDTSessionAdmins(sessRV zkidentity.ShortID, newAdmins []zkidentity.ShortID) error {
	// Sanity checks.
	for _, id := range newAdmins {
		_, err := c.UserByID(id)
		if err != nil {
			return err
		}
		if id == c.PublicID() {
			return errors.New("cannot add local client as admin")
		}
	}

	var sess *clientdb.RTDTSession
	var adminsToUpdate []zkidentity.ShortID
	err := c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		var err error
		sess, err = c.GetRTDTSession(&sessRV)
		if err != nil {
			return err
		}
		if !sess.Metadata.IsOwnerOrAdmin(c.PublicID()) {
			return errors.New("cannot change admins when local client is not already an admin")
		}

		var gc clientdb.GroupChat
		if sess.GC != nil {
			gc, err = c.db.GetGC(tx, *sess.GC)
			if err != nil {
				return err
			}
		}

		// Admins should be in the session.
		for _, newID := range newAdmins {
			memberID, _ := sess.MemberIndices(&newID)
			if memberID == -1 {
				return fmt.Errorf("new admin %s is not a session member", newID)
			}

			// If there's an associated GC with this session, then admins
			// should also be admins/owner of the GC.
			if sess.GC != nil {
				if !slices.Contains(gc.Metadata.ExtraAdmins, newID) {
					return fmt.Errorf("cannot add %s as session "+
						"admin when they are not a GC "+
						"admin for the associated GC %s",
						newID, sess.GC)
				}
			}

		}

		// Create list of new admins that need to receive the session
		// cookie and existing appointment cookies.
		adminsToUpdate = slices.Clone(newAdmins)
		for _, id := range sess.Metadata.Admins {
			adminsToUpdate = slices.DeleteFunc(adminsToUpdate, func(oid zkidentity.ShortID) bool { return oid == id })
		}

		sess.Metadata.Admins = newAdmins
		return c.db.UpdateRTDTSession(tx, sess)
	})
	if err != nil {
		return err
	}

	// Send cookies to new admins.
	sendqItems := make([]*preparedSendqItem, 0, len(newAdmins))
	payEvent := fmt.Sprintf("rtdt.admincookies.%s", sessRV)
	adminCookiesRM := rpc.RMRTDTAdminCookies{
		RV:            sessRV,
		SessionCookie: sess.SessionCookie,
		OwnerSecret:   sess.OwnerSecret,
		Members:       sess.RMMembersList(),
		NextPeerID:    &sess.NextPeerID,
	}
	for _, newAdminID := range newAdmins {
		sqi, err := c.prepareSendqItem(payEvent, adminCookiesRM, priorityGC,
			nil, newAdminID)
		if err != nil {
			// Log, but keep going to next member.
			c.log.Warnf("Unable to add RMRTDTAdminCookies "+
				"to member %s: %v", newAdminID, err)
			continue
		}
		sendqItems = append(sendqItems, sqi)
	}

	// Queue the RMs to remote users.
	for _, sqi := range sendqItems {
		err := c.sendPreparedSendqItem(sqi)
		if err != nil {
			c.log.Warnf("Unable to send RMRTDTRotateAppointCookie "+
				"to member %s: %v", sqi.dests[0], err)

			// Keep going.
		}
	}

	// Send update to every member.
	return c.sendRTDTSessionUpdate(sess.Metadata, sess.MemberUIDs(c.PublicID()))
}

// handleRMRTDTAdminCookies handles updates from other session admins about
// admin'd users.
func (c *Client) handleRMRTDTAdminCookies(ru *RemoteUser, rmac rpc.RMRTDTAdminCookies) error {
	var newMembers, delMembers int
	err := c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		sess, err := c.db.GetRTDTSession(tx, &rmac.RV)
		if err != nil {
			return err
		}

		if !sess.Metadata.IsOwnerOrAdmin(ru.ID()) {
			return errNotAdmin
		}

		if rmac.Members != nil {
			memberIDs := make(map[zkidentity.ShortID]struct{}, len(rmac.Members))
			for _, m := range rmac.Members {
				idx, _ := sess.MemberIndices(&m.UID)
				if idx == -1 {
					sess.Members = append(sess.Members, clientdb.RTDTSessionMember{
						UID:    m.UID,
						Tag:    c.mustRandomUint64(),
						PeerID: m.PeerID,
					})
					idx = len(sess.Members) - 1
					newMembers++
				}

				sess.Members[idx].Publisher = m.AllowedAsPublisher
				sess.Members[idx].AppointCookie = m.AppointCookie
				memberIDs[m.UID] = struct{}{}
			}
			for i := 0; i < len(sess.Members); {
				_, ok := memberIDs[sess.Members[i].UID]
				if ok {
					// Still a member.
					i++
					continue
				}

				// Not a member anymore.
				sess.Members = slices.Delete(sess.Members, i, i+1)
				delMembers++
			}
		}

		if rmac.OwnerSecret != nil {
			sess.OwnerSecret = rmac.OwnerSecret
		}
		if rmac.NextPeerID != nil {
			sess.NextPeerID = *rmac.NextPeerID
		}
		if rmac.SessionCookie != nil {
			sess.SessionCookie = rmac.SessionCookie
		}

		return c.db.UpdateRTDTSession(tx, sess)
	})

	if err != nil {
		return err
	}

	ru.log.Infof("Received RTDT admin update (%d new, %d removed members)",
		newMembers, delMembers)
	c.ntfns.notifyRTDTAdminCookiesReceived(ru, rmac.RV)
	return nil
}
