package client

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/companyzero/bisonrelay/client/clientdb"
	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/companyzero/bisonrelay/client/internal/gcmcacher"
	"github.com/companyzero/bisonrelay/rpc"
	"github.com/companyzero/bisonrelay/zkidentity"
)

const (
	// {min,max}SupportedGCVersion tracks the mininum and maximum versions
	// the client code handles for GCs.
	minSupportedGCVersion = 0
	maxSupportedGCVersion = 0
)

// The group chat flow is:
//
//          Alice                                    Bob
//         -------                                  -----
//
//     NewGroupChat()
//     InviteToGroupChat()
//           \---------> RMGroupInvite -->
//
//                                             handleGCInvite()
//                                             AcceptGroupChatInvite()
//                      <-- RMGroupJoin -------------/
//
//     handleGCJoin()
//           \---------> RMGroupList -->
//                                             handleGroupList()
//

// setGCAlias sets the new group chat alias cache.
func (c *Client) setGCAlias(aliasMap map[string]zkidentity.ShortID) {
	c.gcAliasMtx.Lock()
	c.gcAliasMap = aliasMap
	c.gcAliasMtx.Unlock()
}

// AliasGC replaces the local alias of a GC for a new one.
func (c *Client) AliasGC(gcID zkidentity.ShortID, newAlias string) error {
	newAlias = strings.TrimSpace(newAlias)
	if newAlias == "" {
		return fmt.Errorf("new GC alias acnnot be empty")
	}

	return c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		// Ensure gc exists.
		if _, err := c.db.GetGC(tx, gcID); err != nil {
			return err
		}

		if aliasMap, err := c.db.SetGCAlias(tx, gcID, newAlias); err != nil {
			return err
		} else {
			c.setGCAlias(aliasMap)
		}

		return nil
	})
}

// GetGCAlias returns the local alias for the specified GC.
func (c *Client) GetGCAlias(gcID zkidentity.ShortID) (string, error) {
	var alias string
	c.gcAliasMtx.Lock()
	for v, id := range c.gcAliasMap {
		if id == gcID {
			alias = v
			break
		}
	}
	c.gcAliasMtx.Unlock()
	if alias == "" {
		return "", fmt.Errorf("gc %s not found", gcID)
	}
	return alias, nil
}

// GCIDByName returns the GC ID of the local GC with the given name. The name can
// be either a local GC alias or a full hex GC ID.
func (c *Client) GCIDByName(name string) (zkidentity.ShortID, error) {
	var id zkidentity.ShortID

	// Check if it's a full hex ID.
	if err := id.FromString(name); err == nil {
		return id, nil
	}

	// Check alias cache.
	c.gcAliasMtx.Lock()
	id, ok := c.gcAliasMap[name]
	c.gcAliasMtx.Unlock()

	if !ok {
		return id, fmt.Errorf("gc %q not found", name)
	}

	return id, nil
}

// NewGroupChat creates a new gc with the local user as admin.
func (c *Client) NewGroupChat(name string) (zkidentity.ShortID, error) {
	var id zkidentity.ShortID

	// Ensure we're not trying to duplicate the name.
	if _, err := c.GCIDByName(name); err == nil {
		return id, fmt.Errorf("gc named %q already exists", name)
	}

	if _, err := rand.Read(id[:]); err != nil {
		return id, err
	}
	err := c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		// Ensure it doesn't exist.
		_, err := c.db.GetGC(tx, id)
		if !errors.Is(err, clientdb.ErrNotFound) {
			if err == nil {
				err = fmt.Errorf("can't create gc %q (%s): %w",
					name, id.String(), errAlreadyExists)
			}
			return err
		}
		gc := rpc.RMGroupList{
			ID:         id,
			Name:       name,
			Generation: 1,
			Timestamp:  time.Now().Unix(),
			Members: []zkidentity.ShortID{
				c.PublicID(),
			},
		}
		if err = c.db.SaveGC(tx, gc); err != nil {
			return fmt.Errorf("can't save gc %q (%s): %v", name, id.String(), err)
		}
		if aliasMap, err := c.db.SetGCAlias(tx, id, name); err != nil {
			c.log.Errorf("can't set name %s for gc %s: %v", name, id.String(), err)
		} else {
			c.setGCAlias(aliasMap)
		}
		c.log.Infof("Created new gc %q (%s)", name, id.String())

		return nil
	})
	return id, err
}

// InviteToGroupChat invites the given user to the given gc. The local user
// must be the admin of the group and the remote user must have been KX'd with.
func (c *Client) InviteToGroupChat(gcID zkidentity.ShortID, user UserID) error {
	ru, err := c.rul.byID(user)
	if err != nil {
		return err
	}

	invite := rpc.RMGroupInvite{
		ID:      gcID,
		Expires: time.Now().Add(time.Hour * 24).Unix(),
	}

	err = c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		// Ensure gc exists and we're the admin.
		gc, err := c.db.GetGC(tx, gcID)
		if err != nil {
			return err
		}

		if len(gc.Members) == 0 || gc.Members[0] != c.PublicID() {
			return fmt.Errorf("cannot create gc invite: not an admin of gc %s",
				gcID)
		}

		invite.Name = gc.Name

		// Generate an unused token.
		for {
			// The % 1000000 is to generate a shorter token and
			// maintain compat to old client.
			invite.Token = c.mustRandomUint64() % 1000000
			_, _, _, err := c.db.FindGCInvite(tx, gcID, invite.Token)
			if errors.Is(err, clientdb.ErrNotFound) {
				break
			} else if err != nil {
				return err
			}
		}

		// Add to db.
		_, err = c.db.AddGCInvite(tx, user, invite)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}

	// Send the invite.
	c.log.Infof("Inviting %s to gc %q (%s)", ru, invite.Name, gcID)
	payEvent := fmt.Sprintf("gc.%s.sendinvite", gcID.ShortLogID())
	return ru.sendRM(invite, payEvent)
}

// handleGCInvite handles a message where a remote user is inviting us to join
// a gc.
func (c *Client) handleGCInvite(ru *RemoteUser, invite rpc.RMGroupInvite) error {
	if invite.ID.IsEmpty() {
		return fmt.Errorf("cannot accept gc invite: gc id is empty")
	}

	invite.Name = strings.TrimSpace(invite.Name)
	if invite.Name == "" {
		invite.Name = hex.EncodeToString(invite.ID[:8])
	}

	// Add this invite to the DB.
	var iid uint64
	err := c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		var err error
		_, err = c.db.GetGC(tx, invite.ID)
		if !errors.Is(err, clientdb.ErrNotFound) {
			if err == nil {
				err = fmt.Errorf("can't accept gc invite: gc %q already exists",
					invite.ID.String())
			}
			return err
		}

		iid, err = c.db.AddGCInvite(tx, ru.ID(), invite)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return err
	}

	// Let user know about it.
	c.log.Infof("Received invitation to gc %q from user %s", invite.ID.String(), ru)
	if c.cfg.GCInviteHandler != nil {
		c.cfg.GCInviteHandler(ru, iid, invite)
	}

	return nil
}

// AcceptGroupChatInvite accepts the given invitation, previously received from
// some user.
func (c *Client) AcceptGroupChatInvite(iid uint64) error {
	var invite rpc.RMGroupInvite
	var uid UserID
	err := c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		var err error
		invite, uid, err = c.db.GetGCInvite(tx, iid)
		if err != nil {
			return err
		}

		if err := c.db.MarkGCInviteAccepted(tx, iid); err != nil {
			return err
		}
		return err
	})
	if err != nil {
		return err
	}

	ru, err := c.rul.byID(uid)
	if err != nil {
		return err
	}

	join := rpc.RMGroupJoin{
		ID:    invite.ID,
		Token: invite.Token,
	}
	c.log.Infof("Accepting invitation to gc %q (%s) from %s", invite.Name, invite.ID.String(), ru)
	payEvent := fmt.Sprintf("gc.%s.acceptinvite", invite.ID.ShortLogID())
	return ru.sendRM(join, payEvent)
}

// ListGCInvitesFor returns all GC invites received that were for the specified
// gc name.
func (c *Client) ListGCInvitesFor(gcName string) ([]*clientdb.GCInvite, error) {
	var invites []*clientdb.GCInvite
	err := c.dbView(func(tx clientdb.ReadTx) error {
		var err error
		invites, err = c.db.ListGCInvites(tx, gcName)
		return err
	})
	return invites, err
}

// sendToGCMembers sends the given message to all GC members of the given slice
// (unless that is the local client).
func (c *Client) sendToGCMembers(gcID zkidentity.ShortID,
	members []zkidentity.ShortID, payType string, msg interface{},
	progressChan chan SendProgress) error {

	localID := c.PublicID()
	payEvent := fmt.Sprintf("gc.%s.%s", gcID.ShortLogID(), payType)

	ids := make([]clientintf.UserID, 0, len(members)-1)
	for _, uid := range members {
		if uid == localID {
			continue
		}

		ids = append(ids, uid)
	}
	sqid, err := c.addToSendQ(payEvent, msg, priorityGC, ids...)
	if err != nil {
		return fmt.Errorf("Unable to add gc msg to send queue: %v", err)
	}

	var progressMtx sync.Mutex
	var sent, total int

	for _, id := range members {
		if id == localID {
			continue
		}

		ru, err := c.rul.byID(id)
		if err != nil {
			// Warn about this error, but keep sending msgs to
			// other users. When an appropriate handler for this
			// event exists, log as a warning instead of error.
			if c.cfg.GCWithUnkxdMember != nil {
				c.log.Warnf("Error finding gc %q member %s in user list: %v",
					gcID.String(), id, err)
				go c.cfg.GCWithUnkxdMember(gcID, id)
			} else {
				c.log.Errorf("Error finding gc %q member %s in user list: %v",
					gcID.String(), id, err)
			}
			continue
		}

		progressMtx.Lock() // Unlikely, but could race with the result.
		total += 1
		progressMtx.Unlock()

		// Send as a goroutine to fulfill for all users concurrently.
		go func() {
			err := ru.sendRMPriority(msg, payEvent, priorityGC)
			if errors.Is(err, clientintf.ErrSubsysExiting) {
				return
			}

			// Remove from sendq independently of error.
			c.removeFromSendQ(sqid, ru.ID())
			if err != nil {
				c.log.Errorf("Unable to send %T on gc %q to user %s: %v",
					msg, gcID.String(), ru, err)
				return
			}

			if progressChan != nil {
				progressMtx.Lock()
				sent += 1
				progressChan <- SendProgress{
					Sent:  sent,
					Total: total,
					Err:   err,
				}
				progressMtx.Unlock()
			}
		}()
	}

	return nil
}

// handleGCJoin handles a msg when a remote user is asking to join a GC we
// administer (that is, responding to an invite previously sent by us).
func (c *Client) handleGCJoin(ru *RemoteUser, invite rpc.RMGroupJoin) error {
	var gc rpc.RMGroupList
	updated := false
	err := c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		var err error
		_, uid, iid, err := c.db.FindGCInvite(tx,
			invite.ID, invite.Token)
		if err != nil {
			return err
		}

		// Ensure we received this join from the same user we sent it
		// to.
		if uid != ru.ID() {
			return fmt.Errorf("received GCJoin from user %s when "+
				"it was sent to user %s", ru.ID(), uid)
		}

		// Ensure we are the admin of the group.
		gc, err = c.db.GetGC(tx, invite.ID)
		if err != nil {
			return err
		}
		if gc.Members[0] != c.PublicID() {
			return fmt.Errorf("cannot add gc member when not the gc admin")
		}

		// Ensure user is not on gc yet.
		for _, v := range gc.Members {
			if uid == v {
				return fmt.Errorf("user %s already part of gc %q",
					uid, gc.ID.String())
			}
		}

		if invite.Error == "" {
			// Add the new member, increment generation, save the
			// new gc group.
			gc.Members = append(gc.Members, uid)
			gc.Generation += 1
			gc.Timestamp = time.Now().Unix()
			if err = c.db.SaveGC(tx, gc); err != nil {
				return err
			}
			updated = true
		} else {
			c.log.Infof("User %s rejected invitation to %q: %q",
				ru, gc.ID.String(), invite.Error)
		}

		// This invitation is fulfilled.
		if err = c.db.DelGCInvite(tx, iid); err != nil {
			return err
		}

		return nil
	})

	if err != nil || !updated {
		return err
	}

	c.log.Infof("User %s joined gc %s (%q)", ru, gc.ID, gc.Name)

	// Join fulfilled. Send new group list to every member except admin
	// (us).
	err = c.sendToGCMembers(gc.ID, gc.Members, "sendlist", gc, nil)
	if err != nil {
		return err
	}

	if c.cfg.GCJoinHandler != nil {
		var entry clientdb.GCAddressBookEntry
		clientdb.RMGroupListToGCEntry(&gc, &entry)
		c.cfg.GCJoinHandler(ru, entry)
	}

	return nil
}

// handleGCList handles updates to a GC metadata. The sending user must have
// been the admin, otherwise this update is rejected.
func (c *Client) handleGCList(ru *RemoteUser, gl rpc.RMGroupList) error {
	// Helper to determine if the user needs a warning about the GC version.
	notifyVersionWarning := false
	checkNeedsVersionWarning := func() {
		notifyVersionWarning = (gl.Version < minSupportedGCVersion || gl.Version > maxSupportedGCVersion) && !c.gcWarnedVersions.Set(gl.ID)
	}

	newGC := false
	err := c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		oldGC, err := c.db.GetGC(tx, gl.ID)
		if errors.Is(err, clientdb.ErrNotFound) {
			newGC = true
		} else if err != nil {
			return err
		}

		var gcName string
		if newGC {
			// This must have been an invite we accepted. Ensure
			// this came from the expected user.
			invites, err := c.db.ListGCInvites(tx, gl.ID.String())
			if err != nil {
				return fmt.Errorf("unable to list gc invites: %v", err)
			}
			found := false
			for _, inv := range invites {
				if !inv.Accepted {
					continue
				}
				if inv.User == ru.ID() {
					found = true
					gcName = inv.Invite.Name
					break
				}
			}
			if !found {
				return fmt.Errorf("received unexpected group %q "+
					"list", gl.ID.String())
			}

			// Check for version warning. This is done before the
			// admin check because future versions may allow
			// receiving the GC list from non-admins.
			checkNeedsVersionWarning()

			// Ensure we received this from the admin.
			if gl.Members[0] != ru.ID() {
				return fmt.Errorf("received gc list %q from non-admin",
					gl.ID.String())
			}

			// Clear out the invites.
			for _, inv := range invites {
				if err := c.db.DelGCInvite(tx, inv.ID); err != nil {
					return fmt.Errorf("unable to del gc invite: %v", err)
				}
			}
		} else {
			// Check for version warning. This is done before the
			// admin check because future versions may allow
			// receiving the GC list from non-admins.
			checkNeedsVersionWarning()

			// Ensure we received this from the existing admin.
			if oldGC.Members[0] != ru.ID() {
				return fmt.Errorf("received gc list %q from non-admin",
					oldGC.ID.String())
			}

			// Ensure no backtrack on generation.
			if gl.Generation < oldGC.Generation {
				return fmt.Errorf("received gc list %q with wrong "+
					"generation (%d < %d)", oldGC.ID.String(), gl.Generation,
					oldGC.Generation)
			}
		}

		// All is well. Update the local gc data.
		if err = c.db.SaveGC(tx, gl); err != nil {
			return fmt.Errorf("unable to save gc: %v", err)
		}
		if gcName != "" {
			// Check if already have this alias.
			alias := gcName
			_, err := c.GCIDByName(alias)
			for i := 1; err == nil; i += 1 {
				alias = fmt.Sprintf("%s_%d", gcName, i)
				_, err = c.GCIDByName(alias)
			}

			if aliasMap, err := c.db.SetGCAlias(tx, gl.ID, alias); err != nil {
				c.log.Errorf("can't set name %s for gc %s: %v", alias, gl.ID.String(), err)
			} else {
				c.setGCAlias(aliasMap)
			}
		}

		return nil
	})
	if notifyVersionWarning {
		c.log.Warnf("Received GCList for GC %s with version "+
			"%d which is not between the supported versions %d to %d",
			gl.ID, gl.Version, minSupportedGCVersion, maxSupportedGCVersion)
		c.ntfns.notifyOnGCVersionWarning(ru, gl, minSupportedGCVersion,
			maxSupportedGCVersion)
	}
	if err != nil {
		return err
	}

	if newGC {
		c.log.Infof("Received first GC list of %q from %s", gl.ID.String(), ru)
	} else {
		c.log.Debugf("Received updated gc list of %q from %s", gl.ID.String(), ru)
	}

	if c.cfg.GCListUpdated != nil {
		var entry clientdb.GCAddressBookEntry
		clientdb.RMGroupListToGCEntry(&gl, &entry)
		c.cfg.GCListUpdated(entry)
	}

	// Start kx with unknown members if we just joined the chat.
	if !newGC {
		return nil
	}
	me := c.PublicID()
	for _, v := range gl.Members {
		v := v
		if v == me {
			continue
		}

		_, err := c.rul.byID(v)
		if !errors.Is(err, userNotFoundError{}) {
			continue
		}

		// TODO: check if there are no inflight kx attempts yet.
		go func() {
			err := c.RequestMediateIdentity(ru.ID(), v)
			if err != nil && !errors.Is(err, clientintf.ErrSubsysExiting) {
				c.log.Errorf("Unable to autokx with %s via %s: %v",
					v, ru, err)
			}
		}()
	}

	return nil
}

// handleDelayedGCMessages is called by the gc message cacher when it's time
// to let external callers know about new messages.
func (c *Client) handleDelayedGCMessages(msg gcmcacher.Msg) {
	user, err := c.UserByID(msg.UID)
	if err != nil {
		// Should only happen if we blocked the user
		// during the gcm cacher delay.
		c.log.Warnf("Delayed GC message with unknown user %s", msg.UID)
		return
	}

	c.ntfns.notifyOnGCM(user, msg.GCM, msg.TS)
}

// SendProgress is sent to track progress of messages that are sent to multiple
// remote users (for example, GC messages that are sent to all members).
type SendProgress struct {
	Sent  int
	Total int
	Err   error
}

// GCMessage sends a message to the given GC. If progressChan is not nil,
// events are sent to it as the sending process progresses. Writes to
// progressChan are serial, so it's important that it not block indefinitely.
func (c *Client) GCMessage(gcID zkidentity.ShortID, msg string, mode rpc.MessageMode,
	progressChan chan SendProgress) error {

	var gc rpc.RMGroupList
	var gcBlockList clientdb.GCBlockList
	err := c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		var err error
		if gc, err = c.db.GetGC(tx, gcID); err != nil {
			return err
		}
		if gcBlockList, err = c.db.GetGCBlockList(tx, gcID); err != nil {
			return err
		}

		gcAlias, err := c.GetGCAlias(gcID)
		if err != nil {
			gcAlias = gc.Name
		}

		return c.db.LogGCMsg(tx, gcAlias, gcID, false, c.id.Public.Nick, msg, time.Now())
	})
	if err != nil {
		return err
	}

	p := rpc.RMGroupMessage{
		ID:         gcID,
		Generation: gc.Generation,
		Message:    msg,
		Mode:       mode,
	}
	members := gcBlockList.FilterMembers(gc.Members)
	if len(members) == 0 {
		return nil
	}

	return c.sendToGCMembers(gcID, members, "msg", p, progressChan)
}

func (c *Client) handleGCMessage(ru *RemoteUser, gcm rpc.RMGroupMessage, ts time.Time) error {
	var gc rpc.RMGroupList
	var found, isBlocked bool
	var gcAlias string
	err := c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		// Ensure gc exists.
		var err error
		gc, err = c.db.GetGC(tx, gcm.ID)
		if err != nil {
			return err
		}
		for i := range gc.Members {
			if ru.ID() == gc.Members[i] {
				found = true
				break
			}
		}
		if !found {
			return nil
		}

		gcBlockList, err := c.db.GetGCBlockList(tx, gcm.ID)
		if err != nil {
			return err
		}
		isBlocked = gcBlockList.IsBlocked(ru.ID())
		if isBlocked {
			return nil
		}

		gcAlias, err = c.GetGCAlias(gcm.ID)
		if err != nil {
			gcAlias = gc.Name
		}
		return c.db.LogGCMsg(tx, gcAlias, gcm.ID, false, ru.Nick(), gcm.Message, ts)
	})
	if errors.Is(err, clientdb.ErrNotFound) {
		// Remote user sent message on group chat we're no longer a
		// member of. Alert them not to resend messages in this GC to
		// us.
		ru.log.Warnf("Received message on unknown groupchat %q", gcm.ID)
		rmgp := rpc.RMGroupPart{
			ID:     gcm.ID,
			Reason: "I am not in that groupchat",
		}
		payEvent := fmt.Sprintf("gc.%s.preventiveGroupPart", gcm.ID.ShortLogID())
		return ru.sendRMPriority(rmgp, payEvent, priorityGC)
	}
	if err != nil {
		return err
	}

	if isBlocked {
		c.log.Warnf("Received message in GC %q from blocked member %s",
			gcAlias, ru)
		return nil
	}

	if !found {
		// The sender is not in the GC list we have.
		c.log.Warnf("Received message in GC %q from non-member %s",
			gcAlias, ru)
		return nil
	}

	ru.log.Debugf("Received message of len %d in GC %q (%s)", len(gcm.Message),
		gcAlias, gc.ID)

	c.gcmq.GCMessageReceived(ru.ID(), gcm, ts)
	return nil
}

// GetGC returns information about the given gc the local user participates in.
func (c *Client) GetGC(gcID zkidentity.ShortID) (clientdb.GCAddressBookEntry, error) {
	var gc rpc.RMGroupList
	err := c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		var err error
		gc, err = c.db.GetGC(tx, gcID)
		return err
	})

	var entry clientdb.GCAddressBookEntry
	clientdb.RMGroupListToGCEntry(&gc, &entry)
	return entry, err
}

// GetGCBlockList returns the blocklist of the given GC.
func (c *Client) GetGCBlockList(gcID zkidentity.ShortID) (clientdb.GCBlockList, error) {
	var gcbl clientdb.GCBlockList
	err := c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		var err error
		gcbl, err = c.db.GetGCBlockList(tx, gcID)
		return err
	})

	return gcbl, err
}

// ListGCs lists all local GCs the user is participating in.
func (c *Client) ListGCs() ([]clientdb.GCAddressBookEntry, error) {
	var gcs []clientdb.GCAddressBookEntry
	err := c.dbView(func(tx clientdb.ReadTx) error {
		var err error
		gcs, err = c.db.ListGCs(tx)
		return err
	})
	return gcs, err
}

// removeFromGC removes the given user from the GC.
//
// Returns the old members of the gc and the new gc list.
func (c *Client) removeFromGC(gcID zkidentity.ShortID, uid UserID,
	localUserMustBeAdmin bool) ([]zkidentity.ShortID,
	rpc.RMGroupList, error) {

	var gc rpc.RMGroupList
	var oldMembers []zkidentity.ShortID
	err := c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		// Ensure gc exists.
		var err error
		gc, err = c.db.GetGC(tx, gcID)
		if err != nil {
			return err
		}

		oldMembers = gc.Members

		if localUserMustBeAdmin && (len(oldMembers) == 0 || oldMembers[0] != c.PublicID()) {
			return fmt.Errorf("local user is not the admin of the GC")
		}

		// Ensure the user is in the GC.
		var newMembers []zkidentity.ShortID
		for i, id := range gc.Members {
			if id != uid {
				continue
			}
			newMembers = make([]zkidentity.ShortID, 0, len(gc.Members)-1)
			newMembers = append(newMembers, gc.Members[:i]...)
			newMembers = append(newMembers, gc.Members[i+1:]...)
			break
		}
		if len(newMembers) == 0 {
			return fmt.Errorf("user is not a member of the GC")
		}

		gc.Members = newMembers
		gc.Timestamp = time.Now().Unix()
		if err = c.db.SaveGC(tx, gc); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, rpc.RMGroupList{}, err
	}

	return oldMembers, gc, nil
}

// GCKick kicks the given user from the GC. This only works if we're the gc
// admin.
func (c *Client) GCKick(gcID zkidentity.ShortID, uid UserID, reason string) error {
	oldMembers, gc, err := c.removeFromGC(gcID, uid, true)
	if err != nil {
		return err
	}

	rmgk := rpc.RMGroupKick{
		Member:       uid,
		Reason:       reason,
		Parted:       false,
		NewGroupList: gc,
	}

	us := uid.String()
	if ru, err := c.rul.byID(uid); err == nil {
		us = ru.String()
	}
	c.log.Infof("Kicking %s from GC %q", us, gcID.String())

	// Saved updated GC members list. Send kick event to list of old
	// members (which includes the kickee).
	return c.sendToGCMembers(gcID, oldMembers, "kick", rmgk, nil)
}

func (c *Client) handleGCKick(ru *RemoteUser, rmgk rpc.RMGroupKick) error {
	meKicked := rmgk.Member == c.PublicID()
	err := c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		// Ensure gc exists.
		gc, err := c.db.GetGC(tx, rmgk.NewGroupList.ID)
		if err != nil {
			return err
		}

		// Ensure we received this from the existing admin.
		if len(gc.Members) == 0 || gc.Members[0] != ru.ID() {
			return fmt.Errorf("received gc kick %q from non-admin",
				gc.ID.String())
		}

		// Ensure no backtrack on generation.
		if rmgk.NewGroupList.Generation < gc.Generation {
			return fmt.Errorf("received gc list %q with wrong "+
				"generation (%d < %d)", gc.ID.String(), rmgk.NewGroupList.Generation,
				gc.Generation)
		}

		// If we were kicked, remove gc from DB.
		if meKicked {
			if err := c.db.DeleteGC(tx, gc.ID); err != nil {
				return err
			}
			if aliasMap, err := c.db.SetGCAlias(tx, gc.ID, ""); err != nil {
				return err
			} else {
				c.setGCAlias(aliasMap)
			}
			return nil
		}

		// All is well. Update the local gc data.
		if err = c.db.SaveGC(tx, rmgk.NewGroupList); err != nil {
			return fmt.Errorf("unable to save gc: %v", err)
		}
		return nil
	})
	if err != nil {
		return err
	}

	us := UserID(rmgk.Member).String()
	if ru, err := c.rul.byID(rmgk.Member); err == nil {
		us = ru.String()
	}
	verb := "kicked"
	if rmgk.Parted {
		verb = "parted"
	}
	c.log.Infof("User %s %s from GC %q. Reason: %q", us, verb,
		rmgk.NewGroupList.ID.String(), rmgk.Reason)

	if c.cfg.GCUserParted != nil {
		c.cfg.GCUserParted(rmgk.NewGroupList.ID, rmgk.Member,
			rmgk.Reason, !rmgk.Parted)
	}

	return nil
}

// PartFromGC withdraws the local client from the given GC.
func (c *Client) PartFromGC(gcID zkidentity.ShortID, reason string) error {
	var gc rpc.RMGroupList

	err := c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		// Ensure gc exists.
		var err error
		gc, err = c.db.GetGC(tx, gcID)
		if err != nil {
			return err
		}

		// Ensure we're not leaving if we're admin.
		if len(gc.Members) == 0 || gc.Members[0] == c.PublicID() {
			return fmt.Errorf("cannot part from GC when we're the GC admin")
		}

		return nil
	})
	if err != nil {
		return err
	}

	c.log.Infof("Parting from GC %q", gcID.String())

	err = c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		if err := c.db.DeleteGC(tx, gcID); err != nil {
			return err
		}
		if aliasMap, err := c.db.SetGCAlias(tx, gc.ID, ""); err != nil {
			return err
		} else {
			c.setGCAlias(aliasMap)
		}
		return nil
	})
	if err != nil {
		return err
	}

	// Send GroupPart msg to all members.
	rmgp := rpc.RMGroupPart{
		ID:     gcID,
		Reason: reason,
	}
	return c.sendToGCMembers(gcID, gc.Members, "part", rmgp, nil)
}

func (c *Client) handleGCPart(ru *RemoteUser, rmgp rpc.RMGroupPart) error {
	_, _, err := c.removeFromGC(rmgp.ID, ru.ID(), false)
	if err != nil {
		return err
	}

	c.log.Infof("User %s parting from GC %q. Reason: %q", ru, rmgp.ID.String(),
		rmgp.Reason)

	if c.cfg.GCUserParted != nil {
		c.cfg.GCUserParted(rmgp.ID, ru.ID(),
			rmgp.Reason, false)
	}

	return nil
}

// KillGroupChat completely dissolves the group chat.
func (c *Client) KillGroupChat(gcID zkidentity.ShortID, reason string) error {
	var oldMembers []zkidentity.ShortID
	err := c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		// Ensure gc exists and we're the admin.
		var err error
		gc, err := c.db.GetGC(tx, gcID)
		if err != nil {
			return err
		}

		if len(gc.Members) == 0 || gc.Members[0] != c.PublicID() {
			return fmt.Errorf("cannot kill GC: not an admin of gc %q",
				gcID.String())
		}

		oldMembers = gc.Members

		if err := c.db.DeleteGC(tx, gc.ID); err != nil {
			return err
		}
		if aliasMap, err := c.db.SetGCAlias(tx, gc.ID, ""); err != nil {
			return err
		} else {
			c.setGCAlias(aliasMap)
		}
		return nil

	})
	if err != nil {
		return err
	}

	c.log.Infof("Killed GC %s. Reason: %q", gcID.String(), reason)

	rmgk := rpc.RMGroupKill{
		ID:     gcID,
		Reason: reason,
	}

	// Saved updated GC members list. Send kick event to list of old members (which
	// includes the kickee).
	return c.sendToGCMembers(gcID, oldMembers, "kill", rmgk, nil)
}

func (c *Client) handleGCKill(ru *RemoteUser, rmgk rpc.RMGroupKill) error {
	err := c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		// Ensure gc exists.
		gc, err := c.db.GetGC(tx, rmgk.ID)
		if err != nil {
			return err
		}

		// Ensure we received this from the existing admin.
		if len(gc.Members) == 0 || gc.Members[0] != ru.ID() {
			return fmt.Errorf("received gc kill %q from non-admin",
				gc.ID.String())
		}
		if err := c.db.DeleteGC(tx, gc.ID); err != nil {
			return err
		}
		if aliasMap, err := c.db.SetGCAlias(tx, gc.ID, ""); err != nil {
			return err
		} else {
			c.setGCAlias(aliasMap)
		}
		return nil
	})
	if err != nil {
		return err
	}

	c.log.Infof("User %s killed GC %q. Reason: %q", ru, rmgk.ID.String(), rmgk.Reason)

	if c.cfg.GCKilled != nil {
		c.cfg.GCKilled(rmgk.ID, rmgk.Reason)
	}
	return nil
}

// AddToGCBlockList adds the user to the block list of the specified GC. This
// user will no longer be sent messages from the local client in the given GC
// and messages from this user will not generate GCMessage events.
func (c *Client) AddToGCBlockList(gcid zkidentity.ShortID, uid UserID) error {
	return c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		// Ensure GC exists.
		gc, err := c.db.GetGC(tx, gcid)
		if err != nil {
			return err
		}

		// Ensure specified uid is a member.
		found := false
		for i := range gc.Members {
			if uid == gc.Members[i] {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("user %s is not part of the GC", uid)
		}

		// Block user in GC.
		return c.db.AddToGCBlockList(tx, gcid, uid)
	})
}

// AddToGCBlockList removes the user from the block list of the specified GC.
func (c *Client) RemoveFromGCBlockList(gcid zkidentity.ShortID, uid UserID) error {
	return c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		// Ensure GC exists.
		gc, err := c.db.GetGC(tx, gcid)
		if err != nil {
			return err
		}

		// Ensure specified uid is a member.
		found := false
		for i := range gc.Members {
			if uid == gc.Members[i] {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("user %s is not part of the GC", uid)
		}

		// Block user in GC.
		return c.db.RemoveFromGCBlockList(tx, gcid, uid)
	})
}

// ResendGCList resends the GC list to a user. We must be the admin of the GC
// for this to be accepted by the remote user.
//
// When the UID is not specified, the list is resent to all members.
func (c *Client) ResendGCList(gcid zkidentity.ShortID, uid *UserID) error {
	allMembers := uid == nil
	if !allMembers {
		// Verify user exists.
		_, err := c.UserByID(*uid)
		if err != nil {
			return err
		}
	}

	var gc rpc.RMGroupList
	err := c.dbView(func(tx clientdb.ReadTx) error {
		// Fetch GC.
		var err error
		gc, err = c.db.GetGC(tx, gcid)
		if err != nil {
			return err
		}

		// Ensure we're the GC admin.
		if gc.Members[0] != c.PublicID() {
			return fmt.Errorf("cannot send GC list to user when local client is not the GC admin")
		}

		// Ensure specified uid is a member.
		if !allMembers {
			found := false
			for i := range gc.Members {
				if *uid == gc.Members[i] {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("user %s is not part of the GC", uid)
			}
		}

		return nil
	})
	if err != nil {
		return err
	}

	payType := "resendGCList"
	if allMembers {
		c.sendToGCMembers(gcid, gc.Members, payType, gc, nil)
		return nil
	}
	payEvent := fmt.Sprintf("gc.%s.%s", gcid.ShortLogID(), payType)
	return c.sendWithSendQ(payEvent, gc, *uid)
}
