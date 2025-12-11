package client

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/companyzero/bisonrelay/client/clientdb"
	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/companyzero/bisonrelay/internal/strescape"
	"github.com/companyzero/bisonrelay/rpc"
	"github.com/companyzero/bisonrelay/zkidentity"
	"golang.org/x/exp/slices"
)

const (
	// {min,max}SupportedGCVersion tracks the mininum and maximum versions
	// the client code handles for GCs.
	minSupportedGCVersion = 0
	maxSupportedGCVersion = 1

	// newGCVersion is the version of newly created GCs.
	newGCVersion = 1
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

// AliasGC replaces the local alias of a GC for a new one.
func (c *Client) AliasGC(gcID zkidentity.ShortID, newAlias string) error {
	newAlias = strings.TrimSpace(newAlias)
	if newAlias == "" {
		return fmt.Errorf("new GC alias cannot be empty")
	}

	return c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		// Ensure gc exists.
		gc, err := c.db.GetGC(tx, gcID)
		if err != nil {
			return err
		}

		// Ensure no other GC has this alias.
		gcIDs := c.db.FindGCsWithPrefix(newAlias)
		for id, oldAlias := range gcIDs {
			if id == gcID {
				// Ignore target GC (allows setting the alias to
				// the same GC).
				continue
			}

			if oldAlias == newAlias {
				return fmt.Errorf("GC %s already has alias %q",
					id, newAlias)
			}
		}

		// Modify local GC alias.
		gc.Alias = newAlias
		return c.db.SaveGC(tx, gc)
	})
}

// GetGCAlias returns the local alias or the name for the specified GC.
func (c *Client) GetGCAlias(gcID zkidentity.ShortID) (string, error) {
	var res string
	err := c.dbView(func(tx clientdb.ReadTx) error {
		var err error
		res, err = c.db.GetGCName(tx, gcID)
		return err
	})
	return res, err
}

// gcIDByName returns the GC ID of the local GC with the given name. Expected
// to be run inside a transaction.
func (c *Client) gcIDByName(tx clientdb.ReadTx, byName string) (zkidentity.ShortID, error) {
	aliasMap := c.db.FindGCsWithPrefix(byName)
	for id, name := range aliasMap {
		if name == byName {
			return id, nil
		}
	}

	return zkidentity.ShortID{}, fmt.Errorf("gc %q not found", byName)

}

// GCIDByName returns the GC ID of the local GC with the given name. The name
// can be either a local GC alias or a full hex GC ID.
func (c *Client) GCIDByName(name string) (zkidentity.ShortID, error) {
	var id zkidentity.ShortID

	// Check if it's a full hex ID.
	if err := id.FromString(name); err == nil {
		return id, nil
	}

	// Check if any has this exact name/alias.
	var res zkidentity.ShortID
	err := c.dbView(func(tx clientdb.ReadTx) error {
		var err error
		res, err = c.gcIDByName(tx, name)
		return err
	})

	return res, err
}

// GCsWithPrefix returns a list of GC aliases that have the specified prefix.
func (c *Client) GCsWithPrefix(prefix string) []string {
	var res []string
	_ = c.dbView(func(tx clientdb.ReadTx) error {
		aliasMap := c.db.FindGCsWithPrefix(prefix)
		res = make([]string, 0, len(aliasMap))
		for _, name := range aliasMap {
			res = append(res, name)
		}
		return nil
	})
	return res
}

// GCsWithMember returns a list of GCs that have the specified UID as a member.
func (c *Client) GCsWithMember(uid UserID) ([]zkidentity.ShortID, error) {
	var res []zkidentity.ShortID
	err := c.dbView(func(tx clientdb.ReadTx) error {
		var err error
		res, err = c.db.ListGCsWithMember(tx, uid)
		return err
	})
	return res, err
}

// NewGroupChatVersion creates a new gc with the local user as admin and the
// specified version.
func (c *Client) NewGroupChatVersion(name string, version uint8) (zkidentity.ShortID, error) {
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
		gc := clientdb.GroupChat{Metadata: rpc.RMGroupList{
			ID:         id,
			Name:       name,
			Generation: 1,
			Version:    version,
			Timestamp:  time.Now().Unix(),
			Members: []zkidentity.ShortID{
				c.PublicID(),
			},
		}}
		if err = c.db.SaveGC(tx, gc); err != nil {
			return fmt.Errorf("can't save gc %q (%s): %v", name, id.String(), err)
		}
		c.log.Infof("Created new gc %q (%s)", name, id.String())

		return nil
	})
	return id, err
}

// NewGroupChat creates a group chat with the local client as admin.
func (c *Client) NewGroupChat(name string) (zkidentity.ShortID, error) {
	return c.NewGroupChatVersion(name, newGCVersion)
}

// uidHasGCPerm returns true whether the given UID has permission to modify the
// given GC. This takes into account the GC version.
func (c *Client) uidHasGCPerm(gc *rpc.RMGroupList, uid clientintf.UserID) error {
	if gc.Version == 0 {
		// Version 0 GCs only have admin as Members[0].
		if len(gc.Members) > 0 && gc.Members[0].ConstantTimeEq(&uid) {
			return nil
		}

		return fmt.Errorf("user %s not version 0 GC admin", uid)
	}

	if gc.Version == 1 {
		if len(gc.Members) > 0 && gc.Members[0].ConstantTimeEq(&uid) {
			// Update from admin. Accept.
			return nil
		}

		if slices.Contains(gc.ExtraAdmins, uid) {
			// Additional admin.
			return nil
		}

		return fmt.Errorf("user %s not version 1 GC admin", uid)
	}

	return fmt.Errorf("unsupported GC version %d", gc.Version)
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
		Expires: time.Now().Add(c.cfg.GCInviteExpiration).Unix(),
	}

	err = c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		// Ensure gc exists and we're the admin.
		gc, err := c.db.GetGC(tx, gcID)
		if err != nil {
			return err
		}

		if err := c.uidHasGCPerm(&gc.Metadata, c.PublicID()); err != nil {
			return fmt.Errorf("not permitted to send send invite: %v", err)
		}

		// If there's an RTDT session associated with this GC, ensure
		// there's still room in the session for new members.
		if gc.RTDTSessionRV != nil {
			sess, err := c.db.GetRTDTSession(tx, gc.RTDTSessionRV)
			if err != nil {
				return err
			}
			if !sess.LocalIsAdmin() {
				return fmt.Errorf("local client is not an admin "+
					"of associated RTDT session %s", gc.RTDTSessionRV)
			}
			if uint32(len(sess.Members)) >= sess.Metadata.Size {
				return ErrRTDTSessionFull{
					sessRV:  *gc.RTDTSessionRV,
					members: len(sess.Members),
					size:    int(sess.Metadata.Size),
				}
			}
		}

		invite.Name = gc.Metadata.Name
		invite.Version = gc.Metadata.Version

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

	if invite.Version < minSupportedGCVersion || invite.Version > maxSupportedGCVersion {
		return fmt.Errorf("invited to GC %s (%q) with unsupported version %d",
			invite.ID, invite.Name, invite.Version)
	}

	if invite.ID.IsEmpty() {
		return fmt.Errorf("GC id is empty")
	}

	expires := time.Unix(invite.Expires, 0)
	if expires.Before(time.Now()) {
		// This is not an error, just a sign that the local client was
		// offline for too long before receiving the invite.
		ru.log.Warnf("Received expired invitation to GC %q (%s)",
			invite.Name, invite.ID)
		return nil
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
	c.ntfns.notifyInvitedToGC(ru, iid, invite)
	return nil
}

// DeclineGroupChatInvite removes the given group chat invite from the list of
// outstanding invites.
func (c *Client) DeclineGroupChatInvite(iid uint64) error {
	var uid clientintf.UserID
	var inv rpc.RMGroupInvite
	err := c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		var err error
		inv, uid, err = c.db.GetGCInvite(tx, iid)
		if err != nil {
			return err
		}
		return c.db.DelGCInvite(tx, iid)
	})
	if err != nil {
		return err
	}

	ru, err := c.UserByID(uid)
	if err != nil {
		return err
	}

	ru.log.Infof("Declined invite from user to join GC %q (%s)", inv.Name,
		inv.ID)

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

	expires := time.Unix(invite.Expires, 0)
	if expires.Before(time.Now()) {
		return ErrGCInvitationExpired
	}

	join := rpc.RMGroupJoin{
		ID:    invite.ID,
		Token: invite.Token,
	}
	c.log.Infof("Accepting invitation to gc %q (%s) from %s", invite.Name, invite.ID.String(), ru)
	payEvent := fmt.Sprintf("gc.%s.acceptinvite", invite.ID.ShortLogID())
	return ru.sendRM(join, payEvent)
}

// ListGCInvitesFor returns all GC invites. If gcid is specified, only invites
// for the specified GC are returned.
func (c *Client) ListGCInvitesFor(gcid *zkidentity.ShortID) ([]*clientdb.GCInvite, error) {
	var invites []*clientdb.GCInvite
	err := c.dbView(func(tx clientdb.ReadTx) error {
		var err error
		invites, err = c.db.ListGCInvites(tx, gcid)
		return err
	})

	// Sort in a consistent way. By group ID, then inviter ID then
	// expiration.
	sort.Slice(invites, func(i, j int) bool {
		ii, ij := invites[i], invites[j]
		cgid := ii.Invite.ID.Compare(&ij.Invite.ID)
		if cgid < 0 {
			return true
		}
		cuid := ii.User.Compare(&ij.User)
		if cgid == 0 && cuid < 0 {
			return true
		}
		if cgid == 0 && cuid == 0 && ii.Invite.Expires < ij.Invite.Expires {
			return true
		}
		return false
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

	// missingKXInfo will be used to track information about members that
	// the local client hasn't KXd with yet.
	type missingKXInfo struct {
		uid      clientintf.UserID
		hasKX    bool
		hasMI    bool
		medID    *clientintf.UserID
		miCount  uint32
		skipWarn bool
		skipMI   bool
	}
	var missingKX []missingKXInfo

	// Add the set of outbound messages to the sendq.
	ids := make([]clientintf.UserID, 0, len(members)-1)
	for _, uid := range members {
		if uid == localID {
			continue
		}

		// If the user isn't KX'd with, and they haven't been warned
		// recently, add to the missingKX list to perform some actions
		// later on.
		_, err := c.rul.byID(uid)
		if err != nil {
			c.unkxdWarningsMtx.Lock()
			if t := c.unkxdWarnings[uid]; time.Since(t) > c.cfg.UnkxdWarningTimeout {
				missingKX = append(missingKX, missingKXInfo{uid: uid})
			}
			c.unkxdWarningsMtx.Unlock()
			continue
		}

		ids = append(ids, uid)
	}

	err := c.sendWithSendQPriority(payEvent, msg, priorityGC, progressChan, ids...)
	if err != nil {
		return fmt.Errorf("unable send GC msgs: %v", err)
	}

	// Early return if there are no members that are missing kx.
	if len(missingKX) == 0 {
		return nil
	}

	// Handle GC members for which we don't have KX. Determine if there
	// is a KX/MediateID attempt for them, start a new one if needed and
	// warn the UI about it.
	//
	// First: go through the DB to see if they are being KX'd with.
	err = c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		var localID, gcOwner clientintf.UserID
		var gotGCInfo bool
		for i := range missingKX {
			target := missingKX[i].uid

			// Check if already KXing.
			kxs, err := c.db.HasKXWithUser(tx, target)
			if err != nil {
				return err
			}
			missingKX[i].hasKX = len(kxs) > 0

			// Check if already has MediateID requests.
			hasRecent, err := c.db.HasAnyRecentMediateID(tx, target,
				c.cfg.RecentMediateIDThreshold)
			if err != nil {
				return err
			}
			missingKX[i].hasMI = hasRecent

			// Check if the attempts to KX with the target crossed
			// a max attempt count limit or if we're expecting a
			// KX request from them (because _they_ joined the
			// GC).
			if unkx, err := c.db.ReadUnxkdUserInfo(tx, target); err == nil {
				missingKX[i].miCount = unkx.MIRequests
				if unkx.MIRequests >= uint32(c.cfg.MaxAutoKXMediateIDRequests) {
					c.log.Debugf("Skipping autoKX with GC's %s member %s "+
						"due to MI requests %d >= max %d",
						gcID, target, unkx.MIRequests,
						c.cfg.MaxAutoKXMediateIDRequests)
					missingKX[i].skipMI = true
					missingKX[i].skipWarn = unkx.MIRequests > uint32(c.cfg.MaxAutoKXMediateIDRequests)
					if !missingKX[i].skipWarn {
						// Add one to MIRequests to avoid warning again.
						unkx.MIRequests += 1
						err := c.db.StoreUnkxdUserInfo(tx, unkx)
						if err != nil {
							return err
						}
					}
				} else if unkx.AddedToGCTime != nil && time.Since(*unkx.AddedToGCTime) < c.cfg.RecentMediateIDThreshold {
					c.log.Debugf("Skipping autoKX with GC's %s member %s "+
						"due to interval from GC add %s < recent "+
						"MI threshold %s", gcID, target,
						time.Since(*unkx.AddedToGCTime),
						c.cfg.RecentMediateIDThreshold)
					missingKX[i].skipMI = true
				}
				if unkx.AddedToGCTime != nil && time.Since(*unkx.AddedToGCTime) < c.cfg.UnkxdWarningTimeout {
					missingKX[i].skipWarn = true
				}
			}

			if missingKX[i].skipMI || missingKX[i].hasMI {
				continue
			}

			// Fetch the group list if needed.
			if !gotGCInfo {
				gcl, err := c.db.GetGC(tx, gcID)
				if err != nil {
					return err
				}
				localID = c.PublicID()
				gcOwner = gcl.Metadata.Members[0]
				gotGCInfo = true
			}

			// Determine if we can ask the GC's owner for a mediate
			// ID request.
			if gcOwner != localID {
				missingKX[i].medID = &gcOwner
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	// Next, log a warning and send a ntfn to the UI about each user's
	// situation.
	c.unkxdWarningsMtx.Lock()
	now := time.Now()
	for _, mkx := range missingKX {
		if mkx.skipWarn {
			continue
		}
		if t := c.unkxdWarnings[mkx.uid]; now.Sub(t) < c.cfg.UnkxdWarningTimeout {
			// Already warned.
			continue
		}
		c.unkxdWarnings[mkx.uid] = now
		c.log.Warnf("Unable to send %T to unKXd member %s in GC %s",
			msg, mkx.uid, gcID)
		c.ntfns.notifyGCWithUnxkdMember(gcID, mkx.uid, mkx.hasKX, mkx.hasMI,
			mkx.miCount, mkx.medID)

	}
	c.unkxdWarningsMtx.Unlock()

	// Next, start the mediate id requests that are needed.
	for _, mkx := range missingKX {
		if mkx.hasKX || mkx.hasMI || mkx.medID == nil || mkx.skipMI {
			continue
		}

		err := c.maybeRequestMediateID(*mkx.medID, mkx.uid)
		if err != nil && !errors.Is(err, clientintf.ErrSubsysExiting) {
			c.log.Errorf("Unable to request mediate ID of target %s "+
				"to mediator %s: %v", mkx.uid, mkx.medID, err)
		}
	}

	return nil
}

// maybeNotifyGCVersionWarning checks whether a notification for a GC version
// mismatch is needed for a received GC list, and triggers the notification.
func (c *Client) maybeNotifyGCVersionWarning(ru *RemoteUser, gcid zkidentity.ShortID, gcl rpc.RMGroupList) {
	notifyVersionWarning := (gcl.Version < minSupportedGCVersion || gcl.Version > maxSupportedGCVersion) && !c.gcWarnedVersions.Set(gcid)
	if notifyVersionWarning {
		c.log.Warnf("Received GCList for GC %s with version "+
			"%d which is not between the supported versions %d to %d",
			gcid, gcl.Version, minSupportedGCVersion, maxSupportedGCVersion)
		c.ntfns.notifyOnGCVersionWarning(ru, gcl, minSupportedGCVersion,
			maxSupportedGCVersion)
	}
}

// maybeUpdateGCFunc verifies that the given gcid exists, calls f() with the
// existing GC definition, then updates the DB with the modified value. It
// returns both the old and new GC definitions.
//
// Checks are performed to ensure the new GC definitions are sane and allowed
// by the given remote user. If ru is nil, then the update is assumed to be
// made by the local client.
//
// f is called within a DB tx.
func (c *Client) maybeUpdateGCFunc(ru *RemoteUser, gcid zkidentity.ShortID, f func(*clientdb.GroupChat) error) (oldGC, newGC clientdb.GroupChat, err error) {
	var checkVersionWarning bool
	var updaterID clientintf.UserID
	localID := c.PublicID()
	var senderNick string
	if ru != nil {
		updaterID = ru.ID()
		senderNick = strescape.Nick(ru.Nick())
	} else {
		updaterID = localID
		senderNick = "local client"
	}

	err = c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		// Fetch GC.
		var err error
		oldGC, err = c.db.GetGC(tx, gcid)
		if err != nil {
			return err
		}

		if len(oldGC.Metadata.Members) == 0 {
			return fmt.Errorf("old GC %s has zero members", gcid)
		}

		// Produce the new GC. Deep copy the old GC so f() can mutate
		// everything.
		newGC = oldGC.DeepCopy()
		if err := f(&newGC); err != nil {
			return err
		}

		// Ensure no backtrack on generation.
		oldMeta, newMeta := &oldGC.Metadata, &newGC.Metadata
		if newMeta.Generation < oldMeta.Generation {
			return fmt.Errorf("cannot backtrack GC generation on "+
				"GC %s (%d < %d)", gcid, oldMeta.Generation,
				newMeta.Generation)
		}

		// Ensure no downgrade in version.
		if newMeta.Version < oldMeta.Version {
			return fmt.Errorf("cannot downgrade GC version on "+
				"GC %s (%d < %d)", gcid, oldMeta.Generation,
				newMeta.Generation)
		}

		// Ensure no changing name.
		if newMeta.Name != oldMeta.Name {
			return fmt.Errorf("cannot change name of GC %s from "+
				"%q to %q", gcid, oldMeta.Name, newMeta.Name)
		}

		// Special case changing the owner: only the owner itself
		// can do it.
		if len(oldMeta.Members) == 0 || len(newMeta.Members) == 0 {
			return fmt.Errorf("gc has no members")
		}
		if oldMeta.Members[0] != newMeta.Members[0] && oldMeta.Members[0] != updaterID {
			return fmt.Errorf("only previous GC owner %s may change "+
				"GC's %s owner", oldMeta.Members[0], gcid)
		}

		// This check is done before checking for permission because a
		// future version might have different rules for checking
		// permission.
		checkVersionWarning = ru != nil

		if err := c.uidHasGCPerm(oldMeta, updaterID); err != nil {
			return err
		}

		// Handle case where the local client was removed from GC.
		stillMember := slices.Contains(newMeta.Members, c.PublicID())
		if !stillMember {
			c.db.LogGCMsg(tx, newGC.Name(), gcid, true, "",
				fmt.Sprintf("Admin %s removed us from GC", senderNick), time.Now())
			if err := c.db.DeleteGC(tx, oldMeta.ID); err != nil {
				return err
			}
			return nil
		}

		// This is an update, so any new members added to the GC that
		// we haven't KX'd with are expected to send a MI to the GC
		// owner/admin (because _they_ are the ones joining). So add
		// info to prevent us attempting a crossed MI with them for
		// some time.
		if ru != nil && stillMember {
			for _, uid := range newMeta.Members {
				if uid == localID {
					continue
				}
				if c.db.AddressBookEntryExists(tx, uid) {
					continue
				}

				unkx, err := c.db.ReadUnxkdUserInfo(tx, uid)
				if err != nil {
					if !errors.Is(err, clientdb.ErrNotFound) {
						return err
					}
					unkx.UID = uid
				}
				if unkx.AddedToGCTime != nil {
					continue
				}
				now := time.Now()
				unkx.AddedToGCTime = &now
				if err := c.db.StoreUnkxdUserInfo(tx, unkx); err != nil {
					return err
				}
			}
		}

		return c.db.SaveGC(tx, newGC)
	})

	if checkVersionWarning {
		c.maybeNotifyGCVersionWarning(ru, newGC.Metadata.ID, newGC.Metadata)
	}

	return
}

// maybeUpdateGC updates the given GC definitions for the specified one.
func (c *Client) maybeUpdateGC(ru *RemoteUser, newGCMeta rpc.RMGroupList) (oldGC clientdb.GroupChat, err error) {
	cb := func(ngc *clientdb.GroupChat) error {
		ngc.Metadata = newGCMeta
		return nil
	}
	oldGC, _, err = c.maybeUpdateGCFunc(ru, newGCMeta.ID, cb)
	return
}

// handleGCJoin handles a msg when a remote user is asking to join a GC we
// administer (that is, responding to an invite previously sent by us).
func (c *Client) handleGCJoin(ru *RemoteUser, invite rpc.RMGroupJoin) error {
	var gc clientdb.GroupChat
	updated := false
	err := c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		var err error
		sentInvite, uid, iid, err := c.db.FindGCInvite(tx,
			invite.ID, invite.Token)
		if err != nil {
			return err
		}

		// Ensure the invitation is still valid.
		expires := time.Unix(sentInvite.Expires, 0)
		if expires.Before(time.Now()) {
			return fmt.Errorf("%w %s", ErrGCInvitationExpired,
				expires.Format(time.RFC3339))
		}

		// Ensure we received this join from the same user we sent it
		// to.
		if uid != ru.ID() {
			return fmt.Errorf("received GCJoin from user %s when "+
				"it was sent to user %s", ru.ID(), uid)
		}

		// Ensure we have permission to add people to GC.
		gc, err = c.db.GetGC(tx, invite.ID)
		if err != nil {
			return err
		}
		if err := c.uidHasGCPerm(&gc.Metadata, c.PublicID()); err != nil {
			return fmt.Errorf("local user does not have permission "+
				"to add gc member: %v", err)
		}

		// This invitation is fulfilled.
		if err = c.db.DelGCInvite(tx, iid); err != nil {
			return err
		}

		// Ensure user is not on gc yet.
		if slices.Contains(gc.Metadata.Members, uid) {
			return fmt.Errorf("user %s already part of gc %q",
				uid, gc.Metadata.ID.String())
		}

		if invite.Error == "" {
			// Add the new member, increment generation, save the
			// new gc group.
			gc.Metadata.Members = append(gc.Metadata.Members, uid)
			gc.Metadata.Generation += 1
			gc.Metadata.Timestamp = time.Now().Unix()
			if err = c.db.SaveGC(tx, gc); err != nil {
				return err
			}
			updated = true

			c.db.LogGCMsg(tx, gc.Name(), gc.Metadata.ID, true, "",
				fmt.Sprintf("Local client added %s as member of the GC",
					strescape.Nick(ru.Nick())), time.Now())

		} else {
			c.log.Infof("User %s rejected invitation to %q: %q",
				ru, gc.Metadata.ID.String(), invite.Error)
		}

		return nil
	})

	if err != nil || !updated {
		return err
	}

	c.log.Infof("User %s joined gc %s (%q)", ru, gc.Metadata.ID, gc.Metadata.Name)

	// Join fulfilled. Send new group list to every member except admin
	// (us).
	err = c.sendToGCMembers(gc.Metadata.ID, gc.Metadata.Members, "sendlist",
		gc.Metadata, nil)
	if err != nil {
		return err
	}

	c.ntfns.notifyGCInviteAccepted(ru, gc.Metadata)

	// If the GC has an associated RTDT session, invite the user to the
	// session.
	if gc.RTDTSessionRV != nil {
		err := c.inviteToRTDTSession(*gc.RTDTSessionRV, true, ru.id)
		if err != nil {
			return fmt.Errorf("unable to invite to associated RTDT session: %v", err)
		}
	}
	return nil
}

// notifyUpdatedGC determines what changed between two GC definitions and
// notifies the user about it.
func (c *Client) notifyUpdatedGC(ru *RemoteUser, oldGC, newGC rpc.RMGroupList, ts time.Time) {
	localID := c.PublicID()
	gcID := newGC.ID
	senderAlias := strescape.Nick(ru.Nick())
	if senderAlias == "" {
		senderAlias = ru.ID().String()
	}

	if oldGC.Version != newGC.Version {
		c.logGCEvent(gcID, ts, "User %s upgraded GC version from %d to %d",
			senderAlias, oldGC.Version, newGC.Version)
		c.ntfns.notifyOnGCUpgraded(newGC, oldGC.Version)
	}

	memberChanges := sliceDiff(oldGC.Members, newGC.Members)
	if len(memberChanges.added) > 0 {
		for _, uid := range memberChanges.added {
			nick := c.UserLogNick(uid)
			c.logGCEvent(gcID, ts, "Admin %s added new GC member %s",
				senderAlias, nick)
		}
		c.ntfns.notifyOnAddedGCMembers(newGC, memberChanges.added)
	}
	if len(memberChanges.removed) > 0 {
		// Log on GC log.
		for _, uid := range memberChanges.removed {
			if uid == localID {
				c.logGCEvent(gcID, ts, "Admin %s removed local client from GC",
					senderAlias)
			} else {
				nick := c.UserLogNick(uid)
				c.logGCEvent(gcID, ts, "Admin %s removed GC member %s",
					senderAlias, nick)
			}
		}

		c.ntfns.notifyOnRemovedGCMembers(newGC, memberChanges.removed)
	}

	adminChanges := sliceDiff(oldGC.ExtraAdmins, newGC.ExtraAdmins)
	ownerChanged := oldGC.Members[0] != newGC.Members[0]

	if ownerChanged || len(adminChanges.removed) > 0 || len(adminChanges.added) > 0 {
		c.logGCEvent(gcID, ts, "User %s modified GC admins:\n%s",
			senderAlias, c.gcAdminsChangeTxt(oldGC, newGC, adminChanges))

		// Also add owner (Members[0]) change.
		if ownerChanged {
			adminChanges.added = append(memberChanges.added, newGC.Members[0])
			adminChanges.removed = append(memberChanges.removed, oldGC.Members[0])
		}
		c.ntfns.notifyGCAdminsChanged(ru, newGC, adminChanges.added, adminChanges.removed)
	}
}

// saveJoinedGC is called when the local client receives the first RMGroupList
// after requesting to join the GC with the GC admin.
//
// Returns the new GC name.
func (c *Client) saveJoinedGC(ru *RemoteUser, gl rpc.RMGroupList) (string, error) {
	var checkVersionWarning bool
	var gcName string
	err := c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		// Double check GC does not exist yet.
		_, err := c.db.GetGC(tx, gl.ID)
		if err == nil {
			return fmt.Errorf("GC %s already exists when attempting "+
				"to save after joining", gl.ID)
		}

		// This must have been an invite we accepted. Ensure
		// this came from the expected user.
		invite, _, err := c.db.FindAcceptedGCInvite(tx, gl.ID, ru.ID())
		if err != nil {
			return fmt.Errorf("unable to list gc invites: %v", err)
		}

		// This is set to true before the perm check because future
		// versions might change the permissions about who can send the
		// list.
		checkVersionWarning = true

		// Ensure we received this from someone that can add
		// members.
		if err := c.uidHasGCPerm(&gl, ru.ID()); err != nil {
			return err
		}

		// Remove all invites received to this GC.
		if err := c.db.DelAllInvitesToGC(tx, gl.ID); err != nil {
			return fmt.Errorf("unable to del gc invite: %v", err)
		}

		// Start preparing GroupChat structure.
		gc := clientdb.GroupChat{Metadata: gl}

		// Figure out the GC name (deduplicate name into alias).
		aliased := false
		_, err = c.gcIDByName(tx, gc.Metadata.Name)
		for i := 1; err == nil; i += 1 {
			gc.Alias = fmt.Sprintf("%s_%d", invite.Name, i)
			_, err = c.gcIDByName(tx, gc.Alias)
			aliased = true
		}

		if aliased {
			ru.log.Debugf("Aliasing GC %s as %q due to prior "+
				"GC names already existing", gl.ID, gc.Alias)
		}

		// All is well. Update the local gc data.
		if err := c.db.SaveGC(tx, gc); err != nil {
			return fmt.Errorf("unable to save gc: %v", err)
		}
		return nil
	})
	if checkVersionWarning {
		c.maybeNotifyGCVersionWarning(ru, gl.ID, gl)
	}
	return gcName, err
}

// handleGCList handles updates to a GC metadata. The sending user must have
// been the admin, otherwise this update is rejected.
func (c *Client) handleGCList(ru *RemoteUser, gl rpc.RMGroupList, ts time.Time) error {
	var gcName string

	// Check if GC exists to determine if it's the first GC list.
	_, err := c.GetGC(gl.ID)
	isNewGC := err != nil && errors.Is(err, clientdb.ErrNotFound)
	if err != nil && !isNewGC {
		return err
	}

	if !isNewGC {
		// Existing GC update. Do the update, then return.
		oldGC, err := c.maybeUpdateGC(ru, gl)
		if err != nil {
			return err
		}

		c.log.Infof("Received updated GC list %s (%q) from %s", gl.ID,
			oldGC.Name(), ru)
		c.notifyUpdatedGC(ru, oldGC.Metadata, gl, ts)
		return nil
	}

	// First GC list from a GC we just joined.
	gcName, err = c.saveJoinedGC(ru, gl)
	if err != nil {
		return err
	}
	c.log.Infof("Received first GC list of %s (%q) from %s", gl.ID, gcName, ru)
	c.logGCEvent(gl.ID, ts, "Admin %s added local client to GC", strescape.Nick(ru.Nick()))
	c.ntfns.notifyOnJoinedGC(gl)

	// Start kx with unknown members. They are relying on us performing
	// transitive KX via an admin.
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

		err = c.maybeRequestMediateID(ru.ID(), v)
		if err != nil && !errors.Is(err, clientintf.ErrSubsysExiting) {
			c.log.Errorf("Unable to autokx with %s via %s: %v",
				v, ru, err)
		}
	}

	return nil
}

// handleDelayedGCMessages is called by the gc message cacher when it's time
// to let external callers know about new messages.
func (c *Client) handleDelayedGCMessages(msg clientintf.ReceivedGCMsg) {
	user, err := c.UserByID(msg.UID)
	if err != nil {
		// Should only happen if we blocked the user
		// during the gcm cacher delay.
		c.log.Warnf("Delayed GC message with unknown user %s", msg.UID)
		return
	}

	// Log the message and remove the cached GCM from the db.
	err = c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		if err := c.db.RemoveCachedRGCM(tx, msg); err != nil {
			c.log.Warnf("Unable to remove cached RGCM: %v", err)
		}

		var err error
		msg.GCM.Message, err = c.db.LogGCMsg(tx, msg.GCAlias, msg.GCM.ID, false, user.Nick(),
			msg.GCM.Message, msg.TS)
		if err != nil {
			c.log.Warnf("Unable to log RGCM: %v", err)
		}

		return nil
	})
	if err != nil {
		// Not a fatal error, so just log a warning.
		c.log.Warnf("Unable to handle cached RGCM: %v", err)
	}

	c.ntfns.notifyOnGCM(user, msg.GCM, msg.GCAlias, msg.TS)
}

// SendProgress is sent to track progress of messages that are sent to multiple
// remote users (for example, GC messages that are sent to all members).
type SendProgress struct {
	Sent  int   `json:"sent"`
	Total int   `json:"total"`
	Err   error `json:"err,omitempty"`
}

// GCMessage sends a message to the given GC. If progressChan is not nil,
// events are sent to it as the sending process progresses. Writes to
// progressChan are serial, so it's important that it not block indefinitely.
func (c *Client) GCMessage(gcID zkidentity.ShortID, msg string, mode rpc.MessageMode,
	progressChan chan SendProgress) error {

	<-c.abLoaded
	var gc clientdb.GroupChat
	var gcBlockList clientdb.GCBlockList
	myNick := c.LocalNick()
	err := c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		var err error
		if gc, err = c.db.GetGC(tx, gcID); err != nil {
			return err
		}
		if gcBlockList, err = c.db.GetGCBlockList(tx, gcID); err != nil {
			return err
		}

		_, err = c.db.LogGCMsg(tx, gc.Name(), gcID, false, myNick, msg, time.Now())
		return err
	})
	if err != nil {
		return err
	}

	p := rpc.RMGroupMessage{
		ID:         gcID,
		Generation: gc.Metadata.Generation,
		Message:    msg,
		Mode:       mode,
	}
	members := gcBlockList.FilterMembers(gc.Metadata.Members)
	if len(members) == 0 {
		return nil
	}

	return c.sendToGCMembers(gcID, members, "msg", p, progressChan)
}

// handleGCMessage handles received GC messages.
//
// NOTE: this is called on the RV manager goroutine, so it should not block
// for long periods of time.
func (c *Client) handleGCMessage(ru *RemoteUser, gcm rpc.RMGroupMessage, ts time.Time) error {
	if ru.IsIgnored() {
		ru.log.Tracef("Ignoring received GC message")
		return nil
	}

	var gc clientdb.GroupChat
	var found, isBlocked bool

	// Create the local cached structure for a received GCM. The MsgID is
	// just a random id used for caching purposes.
	rgcm := clientintf.ReceivedGCMsg{
		UID: ru.ID(),
		GCM: gcm,
		TS:  ts,
	}
	_, _ = rand.Read(rgcm.MsgID[:])

	err := c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		// Ensure gc exists.
		var err error
		gc, err = c.db.GetGC(tx, gcm.ID)
		if err != nil {
			return err
		}
		for i := range gc.Metadata.Members {
			if ru.ID() == gc.Metadata.Members[i] {
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
		rgcm.GCAlias = gc.Name()

		return c.db.CacheReceivedGCM(tx, rgcm)
	})
	if errors.Is(err, clientdb.ErrNotFound) {
		// Remote user sent message on group chat we're no longer a
		// member of. Alert them not to resend messages in this GC to
		// us.
		ru.log.Warnf("Received message on unknown groupchat %q", gcm.ID)
		go func() {
			rmgp := rpc.RMGroupPart{
				ID:     gcm.ID,
				Reason: "I am not in that groupchat",
			}
			payEvent := fmt.Sprintf("gc.%s.preventiveGroupPart", gcm.ID.ShortLogID())
			err := c.sendWithSendQPriority(payEvent, rmgp, priorityGC, nil, ru.id)
			if err != nil && !errors.Is(err, clientintf.ErrSubsysExiting) {
				ru.log.Errorf("Unable to send preventive group part: %v", err)
			}
		}()
	}
	if err != nil {
		return err
	}

	if isBlocked {
		c.log.Warnf("Received message in GC %q from blocked member %s",
			rgcm.GCAlias, ru)
		return nil
	}

	if !found {
		// The sender is not in the GC list we have.
		c.log.Warnf("Received message in GC %q from non-member %s",
			rgcm.GCAlias, ru)
		return nil
	}

	if filter, _ := c.FilterGCM(ru.ID(), gc.Metadata.ID, gcm.Message); filter {
		return nil
	}

	ru.log.Debugf("Received message of len %d in GC %q (%s)", len(gcm.Message),
		rgcm.GCAlias, gc.Metadata.ID)

	c.gcmq.GCMessageReceived(rgcm)
	return nil
}

// getGC returns the full group chat data.
func (c *Client) getGC(gcID zkidentity.ShortID) (clientdb.GroupChat, error) {
	var gc clientdb.GroupChat
	err := c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		var err error
		gc, err = c.db.GetGC(tx, gcID)
		return err
	})
	return gc, err
}

// GetGC returns information about the given gc the local user participates in.
//
// Note: Prefer GetGCDB because it returns full data, not just GC metadata.
func (c *Client) GetGC(gcID zkidentity.ShortID) (rpc.RMGroupList, error) {
	gc, err := c.getGC(gcID)
	return gc.Metadata, err
}

// GetGCDB returns the full information about the given gc the local user
// participates in.
func (c *Client) GetGCDB(gcID zkidentity.ShortID) (clientdb.GroupChat, error) {
	gc, err := c.getGC(gcID)
	return gc, err
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

// GetGCTargetCount returns the number of members of the GC that would receive
// a message if the local client sent a message in this GC. This takes into
// account ignored/gc-blocked members and the local client itself.
//
// Returns zero if the passed id is not for a GC the client is a member of.
func (c *Client) GetGCDestCount(gcID zkidentity.ShortID) int {
	var gc clientdb.GroupChat
	var gcBlockList clientdb.GCBlockList
	err := c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		var err error
		if gc, err = c.db.GetGC(tx, gcID); err != nil {
			return err
		}
		if gcBlockList, err = c.db.GetGCBlockList(tx, gcID); err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return 0
	}

	myID := c.PublicID()
	members := gcBlockList.FilterMembers(gc.Metadata.Members)
	for i := 0; i < len(members); {
		if members[i] != myID {
			i += 1
		}
		if i < len(members)-1 {
			members[i] = members[len(members)-1]
		}
		members = members[:len(members)-1]
	}
	return len(members)
}

// ListGCs lists all local GCs the user is participating in.
func (c *Client) ListGCs() ([]clientdb.GroupChat, error) {
	var gcs []clientdb.GroupChat
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
	clientdb.GroupChat, error) {

	var gc clientdb.GroupChat
	var oldMembers []zkidentity.ShortID
	err := c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		// Ensure gc exists.
		var err error
		gc, err = c.db.GetGC(tx, gcID)
		if err != nil {
			return err
		}

		oldMembers = gc.Metadata.Members

		if localUserMustBeAdmin {
			if err := c.uidHasGCPerm(&gc.Metadata, c.PublicID()); err != nil {
				return fmt.Errorf("local user cannot remove from GC: %v", err)
			}
		}

		// Ensure the user is in the GC.
		var newMembers []zkidentity.ShortID
		for i, id := range gc.Metadata.Members {
			if id != uid {
				continue
			}
			if i == 0 {
				return fmt.Errorf("cannot remove members[0] from GC")
			}
			newMembers = make([]zkidentity.ShortID, 0, len(gc.Metadata.Members)-1)
			newMembers = append(newMembers, gc.Metadata.Members[:i]...)
			newMembers = append(newMembers, gc.Metadata.Members[i+1:]...)
			break
		}
		if len(newMembers) == 0 {
			return fmt.Errorf("user is not a member of the GC")
		}

		if idxAdmin := slices.Index(gc.Metadata.ExtraAdmins, uid); idxAdmin > -1 {
			gc.Metadata.ExtraAdmins = slices.Delete(gc.Metadata.ExtraAdmins, idxAdmin, idxAdmin+1)
		}

		gc.Metadata.Members = newMembers
		gc.Metadata.Timestamp = time.Now().Unix()
		if localUserMustBeAdmin {
			// Only bump generation when removing as an admin.
			gc.Metadata.Generation += 1
		}
		if err = c.db.SaveGC(tx, gc); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, clientdb.GroupChat{}, err
	}

	return oldMembers, gc, nil
}

// logGCEvent logs the given GC event. Must not be called from inside a DB
// transaction.
func (c *Client) logGCEvent(gcID zkidentity.ShortID, ts time.Time, msg string, args ...interface{}) {
	msg = fmt.Sprintf(msg, args...)

	err := c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		gcAlias, err := c.db.GetGCName(tx, gcID)
		if err != nil {
			return err
		}
		_, err = c.db.LogGCMsg(tx, gcAlias, gcID, true, "", msg, ts)
		return err
	})
	if err != nil {
		c.log.Warnf("Unable to add GC log msg: %v", err)
	}
}

// GCKick kicks the given user from the GC. This only works if we're the gc
// admin.
func (c *Client) GCKick(gcID zkidentity.ShortID, uid UserID, reason string) error {
	// Sanity check if this GC has an associated RTDT session, can only kick
	// if the local client is in the live session as well (to ensure
	// user is kicked from live session).
	gc, err := c.getGC(gcID)
	if err != nil {
		return err
	}
	if gc.RTDTSessionRV != nil {
		isLive, _ := c.IsLiveAndHotRTSession(gc.RTDTSessionRV)
		if !isLive {
			return errors.New("cannot kick from GC with associated " +
				"live RTDT session when disconnected from live session")
		}
	}

	oldMembers, gc, err := c.removeFromGC(gcID, uid, true)
	if err != nil {
		return err
	}

	rmgk := rpc.RMGroupKick{
		Member:       uid,
		Reason:       reason,
		Parted:       false,
		NewGroupList: gc.Metadata,
	}

	nick := c.UserLogNick(uid)
	gcAlias, _ := c.GetGCAlias(gcID)
	c.log.Infof("Kicking %s (%s) from GC %s (%s)", nick, uid.ShortLogID(),
		strescape.Nick(gcAlias), gcID)
	c.logGCEvent(gcID, time.Now(), "Local client kicked user %s. Reason: %s",
		nick, reason)

	// Notify user was kicked.
	c.ntfns.notifyGCUserParted(gcID, uid, reason, true)

	// Saved updated GC members list. Send kick event to list of old
	// members (which includes the kickee).
	if err := c.sendToGCMembers(gcID, oldMembers, "kick", rmgk, nil); err != nil {
		return err
	}

	// If the GC has an associated RTDT session, kick from there as well.
	if gc.RTDTSessionRV != nil {
		err := c.RemoveRTDTMember(gc.RTDTSessionRV, &uid, reason)
		if err != nil {
			return fmt.Errorf("unable to remove kicked GC member "+
				"from associated RTDT session: %v", err)
		}

		err = c.RotateRTDTAppointmentCookies(gc.RTDTSessionRV)
		if err != nil {
			return fmt.Errorf("unable to rotate cookies from "+
				"associated RTDT session: %v", err)
		}
	}

	return nil
}

func (c *Client) handleGCKick(ru *RemoteUser, rmgk rpc.RMGroupKick, ts time.Time) error {
	oldGC, err := c.maybeUpdateGC(ru, rmgk.NewGroupList)
	if err != nil {
		return err
	}

	// Log event.
	adminNick := c.UserLogNick(ru.ID())
	verb := "kicked"
	if rmgk.Parted {
		verb = "parted"
	}
	kickedUid := clientintf.UserID(rmgk.Member)
	kickedNick := c.UserLogNick(kickedUid)
	gcAlias, _ := c.GetGCAlias(rmgk.NewGroupList.ID)
	if kickedUid == c.PublicID() && gcAlias == "" {
		gcAlias = rmgk.NewGroupList.Name
	}
	ru.log.Infof("Admin %s %s user %s (%s) from GC %q (%s). Reason: %q",
		adminNick, verb, kickedNick, kickedUid.ShortLogID(),
		strescape.Nick(gcAlias), rmgk.NewGroupList.ID, rmgk.Reason)

	// Notify specific part and any other updates.
	c.ntfns.notifyGCUserParted(rmgk.NewGroupList.ID, rmgk.Member,
		rmgk.Reason, !rmgk.Parted)
	c.notifyUpdatedGC(ru, oldGC.Metadata, rmgk.NewGroupList, ts)

	return nil
}

// PartFromGC withdraws the local client from the given GC.
func (c *Client) PartFromGC(gcID zkidentity.ShortID, reason string) error {
	var gc clientdb.GroupChat

	err := c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		// Ensure gc exists.
		var err error
		gc, err = c.db.GetGC(tx, gcID)
		if err != nil {
			return err
		}

		// Ensure we're not leaving if we're admin.
		if len(gc.Metadata.Members) == 0 || gc.Metadata.Members[0] == c.PublicID() {
			return fmt.Errorf("cannot part from GC when we're the GC admin")
		}

		if _, err := c.db.LogGCMsg(tx, gc.Name(), gcID, true, "", "Parting from GC", time.Now()); err != nil {
			return err
		}

		if err := c.db.DeleteGC(tx, gcID); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}

	c.log.Infof("Parting from GC %q", gcID.String())

	// Send GroupPart msg to all members.
	rmgp := rpc.RMGroupPart{
		ID:     gcID,
		Reason: reason,
	}
	if err := c.sendToGCMembers(gcID, gc.Metadata.Members, "part", rmgp, nil); err != nil {
		return err
	}

	// Exit from realtime session as well.
	if gc.RTDTSessionRV != nil {
		err := c.ExitRTDTSession(gc.RTDTSessionRV)
		if err != nil {
			return fmt.Errorf("unable to leave associated RTDT esssion: %v", err)
		}
	}

	return nil
}

func (c *Client) handleGCPart(ru *RemoteUser, rmgp rpc.RMGroupPart, ts time.Time) error {
	// A part comes from the user himself (instead of admin) so it does
	// not use maybeUpdaGC().
	_, _, err := c.removeFromGC(rmgp.ID, ru.ID(), false)
	if err != nil {
		return err
	}

	c.log.Infof("User %s parting from GC %q. Reason: %q", ru, rmgp.ID.String(),
		rmgp.Reason)

	// Log on GC log.
	c.logGCEvent(rmgp.ID, ts, "User %q parting from GC. Reason: %q",
		strescape.Nick(ru.Nick()), rmgp.Reason)

	c.ntfns.notifyGCUserParted(rmgp.ID, ru.ID(), rmgp.Reason, false)
	return nil
}

// KillGroupChat completely dissolves the group chat.
func (c *Client) KillGroupChat(gcID zkidentity.ShortID, reason string) error {
	var oldMembers []zkidentity.ShortID
	var gc clientdb.GroupChat
	err := c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		// Ensure gc exists and we're the admin.
		var err error
		gc, err = c.db.GetGC(tx, gcID)
		if err != nil {
			return err
		}

		if len(gc.Metadata.Members) == 0 || gc.Metadata.Members[0] != c.PublicID() {
			return fmt.Errorf("cannot kill GC: not the owner of gc %q",
				gcID.String())
		}

		oldMembers = gc.Metadata.Members

		if err := c.db.DeleteGC(tx, gc.Metadata.ID); err != nil {
			return err
		}

		_, err = c.db.LogGCMsg(tx, gc.Name(), gcID, true, "",
			fmt.Sprintf("Local user killed GC. Reason: %q", reason),
			time.Now())
		return err
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
	if err := c.sendToGCMembers(gcID, oldMembers, "kill", rmgk, nil); err != nil {
		return err
	}

	if gc.RTDTSessionRV != nil {
		err := c.DissolveRTDTSession(gc.RTDTSessionRV)
		if err != nil {
			return fmt.Errorf("unable to dissolve associated RTDT session: %v", err)
		}
	}

	return nil
}

func (c *Client) handleGCKill(ru *RemoteUser, rmgk rpc.RMGroupKill, ts time.Time) error {
	gcAlias, _ := c.GetGCAlias(rmgk.ID)
	err := c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		// Ensure gc exists.
		gc, err := c.db.GetGC(tx, rmgk.ID)
		if err != nil {
			return err
		}

		// Ensure we received this from the existing owner.
		if len(gc.Metadata.Members) == 0 || gc.Metadata.Members[0] != ru.ID() {
			return fmt.Errorf("received gc kill %q from non-owner",
				gc.Metadata.ID.String())
		}
		if err := c.db.DeleteGC(tx, gc.Metadata.ID); err != nil {
			return err
		}

		_, err = c.db.LogGCMsg(tx, gc.Name(), rmgk.ID, true, "",
			fmt.Sprintf("User %s killed GC. Reason: %q", strescape.Nick(ru.Nick()), rmgk.Reason),
			ts)
		return err
	})
	if err != nil {
		return err
	}

	adminKick, _ := c.UserNick(ru.ID())
	ru.log.Infof("Admin %s killed GC %s (%s). Reason: %q", adminKick,
		strescape.Nick(gcAlias), rmgk.ID.ShortLogID(), rmgk.Reason)

	c.ntfns.notifyOnGCKilled(ru, rmgk.ID, rmgk.Reason)
	return nil
}

// AddToGCBlockList adds the user to the block list of the specified GC. This
// user will no longer be sent messages from the local client in the given GC
// and messages from this user will not generate GCMessage events.
func (c *Client) AddToGCBlockList(gcid zkidentity.ShortID, uid UserID) error {
	ru, err := c.UserByID(uid)
	if err != nil {
		return err
	}

	return c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		// Ensure GC exists.
		gc, err := c.db.GetGC(tx, gcid)
		if err != nil {
			return err
		}

		// Ensure specified uid is a member.
		found := false
		for i := range gc.Metadata.Members {
			if uid == gc.Metadata.Members[i] {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("user %s is not part of the GC", uid)
		}

		// Block user in GC.
		err = c.db.AddToGCBlockList(tx, gcid, uid)
		if err != nil {
			return err
		}

		ru.log.Infof("Added user %q to GC %s (%s) block list", ru.Nick(),
			gc.Name(), gcid)

		_, err = c.db.LogGCMsg(tx, gc.Name(), gcid, true, "",
			fmt.Sprintf("Added user %s to GC block list", strescape.Nick(ru.Nick())),
			time.Now())
		return err
	})
}

// AddToGCBlockList removes the user from the block list of the specified GC.
func (c *Client) RemoveFromGCBlockList(gcid zkidentity.ShortID, uid UserID) error {
	ru, err := c.UserByID(uid)
	if err != nil {
		return err
	}

	return c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		// Ensure GC exists.
		gc, err := c.db.GetGC(tx, gcid)
		if err != nil {
			return err
		}

		// Ensure specified uid is a member.
		found := false
		for i := range gc.Metadata.Members {
			if uid == gc.Metadata.Members[i] {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("user %s is not part of the GC", uid)
		}

		// Block user in GC.
		err = c.db.RemoveFromGCBlockList(tx, gcid, uid)
		if err != nil {
			return err
		}

		ru.log.Infof("Removed user %q from GC %s (%s) block list", ru.Nick(),
			gc.Name(), gcid)

		_, err = c.db.LogGCMsg(tx, gc.Name(), gcid, true, "",
			fmt.Sprintf("Removed user %s from GC block list", strescape.Nick(ru.Nick())),
			time.Now())
		return err
	})
}

// ResendGCList resends the GC list to a user. We must be the admin of the GC
// for this to be accepted by the remote user.
//
// When the UID is not specified, the list is resent to all members.
func (c *Client) ResendGCList(gcid zkidentity.ShortID, uid *UserID) error {
	allMembers := uid == nil

	var ru *RemoteUser
	if !allMembers {
		// Verify user exists.
		var err error
		ru, err = c.UserByID(*uid)
		if err != nil {
			return err
		}
	}

	var gc clientdb.GroupChat
	err := c.dbView(func(tx clientdb.ReadTx) error {
		// Fetch GC.
		var err error
		gc, err = c.db.GetGC(tx, gcid)
		if err != nil {
			return err
		}

		// Ensure we're the GC admin.
		if err := c.uidHasGCPerm(&gc.Metadata, c.PublicID()); err != nil {
			return fmt.Errorf("cannot send GC list to user when "+
				"local client is not a GC admin: %v", err)
		}

		// Ensure specified uid is a member.
		if !allMembers && !slices.Contains(gc.Metadata.Members, *uid) {
			return fmt.Errorf("user %s is not part of the GC", uid)
		}

		return nil
	})
	if err != nil {
		return err
	}

	payType := "resendGCList"
	if allMembers {
		c.log.Infof("Resending GC %s list to all members", gcid)
		c.sendToGCMembers(gcid, gc.Metadata.Members, payType, gc, nil)
		return nil
	}
	ru.log.Infof("Resending GC %s list to user", gcid)
	payEvent := fmt.Sprintf("gc.%s.%s", gcid.ShortLogID(), payType)
	return c.sendWithSendQ(payEvent, gc, *uid)
}

// UpgradeGC upgrades the version of the GC to the specified one. The local
// user must have permission to upgrade the GC.
func (c *Client) UpgradeGC(gcid zkidentity.ShortID, newVersion uint8) error {
	if newVersion < minSupportedGCVersion || newVersion > maxSupportedGCVersion {
		return fmt.Errorf("unsupported GC version %d not between %d and %d",
			newVersion, minSupportedGCVersion, maxSupportedGCVersion)
	}

	cb := func(gc *clientdb.GroupChat) error {
		if gc.Metadata.Version >= newVersion {
			return fmt.Errorf("cannot downgrade GC %s from version %d to %d",
				gcid, gc.Metadata.Version, newVersion)

		}

		gc.Metadata.Version = newVersion
		gc.Metadata.Timestamp = time.Now().Unix()
		gc.Metadata.Generation += 1
		return nil
	}

	oldGC, newGC, err := c.maybeUpdateGCFunc(nil, gcid, cb)
	if err != nil {
		return err
	}
	c.log.Infof("Upgraded GC %s version from %d to %d",
		gcid, oldGC.Metadata.Version, newVersion)
	c.logGCEvent(gcid, time.Now(), "Local client upgraded GC version from %d to %d",
		oldGC.Metadata.Version, newVersion)

	rm := rpc.RMGroupUpgradeVersion{
		NewGroupList: newGC.Metadata,
	}
	return c.sendToGCMembers(gcid, newGC.Metadata.Members, "upgradeGC", rm, nil)
}

func (c *Client) handleGCUpgradeVersion(ru *RemoteUser, gcuv rpc.RMGroupUpgradeVersion,
	ts time.Time) error {

	oldGC, err := c.maybeUpdateGC(ru, gcuv.NewGroupList)
	if err != nil {
		return err
	}
	ru.log.Infof("Received GC %s Version Upgrade from %d to %d",
		gcuv.NewGroupList.ID, oldGC.Metadata.Version, gcuv.NewGroupList.Version)
	c.notifyUpdatedGC(ru, oldGC.Metadata, gcuv.NewGroupList, ts)
	return err
}

// gcAdminsChangeTxt returns a string to log as the list of changed admins
// of the GC.
func (c *Client) gcAdminsChangeTxt(oldGC, newGC rpc.RMGroupList, changes sliceChanges[zkidentity.ShortID]) string {
	var adminsTxt string
	if oldGC.Members[0] != newGC.Members[0] {
		oldNick := c.UserLogNick(oldGC.Members[0])
		newNick := c.UserLogNick(newGC.Members[0])
		adminsTxt += fmt.Sprintf("  - Changed GC owner from %s to %s\n",
			strescape.Nick(oldNick), strescape.Nick(newNick))
	}

	for _, uid := range changes.added {
		nick := c.UserLogNick(uid)
		adminsTxt += fmt.Sprintf("  - Added %s as admin\n", strescape.Nick(nick))
	}
	for _, uid := range changes.removed {
		nick := c.UserLogNick(uid)
		adminsTxt += fmt.Sprintf("  - Removed %s as admin\n", strescape.Nick(nick))
	}
	return strings.TrimRightFunc(adminsTxt, unicode.IsSpace)
}

// ModifyGCAdmins modifies the admins of the GC.
func (c *Client) ModifyGCAdmins(gcid zkidentity.ShortID, extraAdmins []zkidentity.ShortID, reason string) error {
	cb := func(gc *clientdb.GroupChat) error {
		if gc.Metadata.Version < 1 {
			return fmt.Errorf("cannot modify extra admins for GC with version < 1")
		}
		for _, uid := range extraAdmins {
			if !slices.Contains(gc.Metadata.Members, uid) {
				return fmt.Errorf("cannot make non-member an admin")
			}
			if _, err := c.UserByID(uid); err != nil {
				return fmt.Errorf("cannot make unknown user an admin")
			}
		}

		gc.Metadata.Timestamp = time.Now().Unix()
		gc.Metadata.Generation += 1
		gc.Metadata.ExtraAdmins = extraAdmins
		return nil
	}

	oldGC, newGC, err := c.maybeUpdateGCFunc(nil, gcid, cb)
	if err != nil {
		return err
	}

	c.log.Infof("Changed list of GC admins for GC %s to %v",
		gcid, extraAdmins)

	c.logGCEvent(gcid, time.Now(), "Local client modified GC admins:\n%s",
		c.gcAdminsChangeTxt(oldGC.Metadata, newGC.Metadata, sliceDiff(oldGC.Metadata.ExtraAdmins, newGC.Metadata.ExtraAdmins)))

	rm := rpc.RMGroupUpdateAdmins{
		Reason:       reason,
		NewGroupList: newGC.Metadata,
	}
	err = c.sendToGCMembers(gcid, newGC.Metadata.Members, "modifyAdmins", rm, nil)
	if err != nil {
		return err
	}

	// If there's an associated RTDT session, ensure the same admins of the
	// GC can admin the session.
	if newGC.RTDTSessionRV != nil {
		return c.ModifyRTDTSessionAdmins(*newGC.RTDTSessionRV, extraAdmins)
	}

	return nil
}

// ModifyGCOwner changes the owner of a GC. The old owner still remains as
// a member of the GC (but not as admin).
func (c *Client) ModifyGCOwner(gcid zkidentity.ShortID, newOwner clientintf.UserID,
	reason string) error {

	cb := func(gc *clientdb.GroupChat) error {
		if gc.Metadata.Version < 1 {
			return fmt.Errorf("cannot modify extra admins for GC with version < 1")
		}
		newOwnerIdx := slices.Index(gc.Metadata.Members, newOwner)
		if newOwnerIdx < 0 {
			return fmt.Errorf("new owner is not a member of the GC")
		}
		if newOwnerIdx == 0 {
			return fmt.Errorf("new owner is already owner of the GC")
		}
		gc.Metadata.Timestamp = time.Now().Unix()
		gc.Metadata.Generation += 1
		gc.Metadata.Members[0], gc.Metadata.Members[newOwnerIdx] = gc.Metadata.Members[newOwnerIdx], gc.Metadata.Members[0]

		// Remove new owner from list of extra admins if it's there.
		newOwnerAdminIdx := slices.Index(gc.Metadata.ExtraAdmins, newOwner)
		if newOwnerAdminIdx > -1 {
			gc.Metadata.ExtraAdmins = slices.Delete(gc.Metadata.ExtraAdmins, newOwnerAdminIdx, newOwnerAdminIdx+1)
		}
		return nil
	}

	oldGC, newGC, err := c.maybeUpdateGCFunc(nil, gcid, cb)
	if err != nil {
		return err
	}

	newOwnerNick, _ := c.UserNick(newOwner)
	c.log.Infof("Changed list GC owner of GC %q (%s) to %q (%v)",
		newGC.Name(), gcid, newOwnerNick, newOwner)
	c.logGCEvent(gcid, time.Now(), "Local client modified GC admins:\n%s",
		c.gcAdminsChangeTxt(oldGC.Metadata, newGC.Metadata, sliceDiff(oldGC.Metadata.ExtraAdmins, newGC.Metadata.ExtraAdmins)))

	rm := rpc.RMGroupUpdateAdmins{
		Reason:       reason,
		NewGroupList: newGC.Metadata,
	}
	return c.sendToGCMembers(gcid, newGC.Metadata.Members, "modifyOwner", rm, nil)
}

func (c *Client) handleGCUpdateAdmins(ru *RemoteUser, gcup rpc.RMGroupUpdateAdmins, ts time.Time) error {
	oldGC, err := c.maybeUpdateGC(ru, gcup.NewGroupList)
	if err != nil {
		return err
	}

	if gcup.NewGroupList.Members[0] != oldGC.Metadata.Members[0] {
		newOwnerNick, _ := c.UserNick(gcup.NewGroupList.Members[0])
		ru.log.Infof("Changed owner of GC %q (%s) to %q (%v)",
			oldGC.Name(), gcup.NewGroupList.ID, newOwnerNick, gcup.NewGroupList.Members[0])
	}

	if !slices.Equal(oldGC.Metadata.ExtraAdmins, gcup.NewGroupList.ExtraAdmins) {
		ru.log.Infof("Updated list of GC admins for GC %s to %v",
			gcup.NewGroupList.ID, gcup.NewGroupList.ExtraAdmins)
	}

	c.notifyUpdatedGC(ru, oldGC.Metadata, gcup.NewGroupList, ts)
	return err
}

// loadCachedRGCMs reloads previously persisted cached RGCMs that have not
// been emitted yet.
func (c *Client) loadCachedRGCMs(ctx context.Context) error {
	var rgcms []clientintf.ReceivedGCMsg
	err := c.db.View(ctx, func(tx clientdb.ReadTx) error {
		var err error
		rgcms, err = c.db.ListCachedRGCMs(tx)
		return err
	})

	if err != nil {
		return err
	}

	c.gcmq.ReloadCachedMessages(rgcms)
	return nil
}

// removeExpiredGCInvites removes any GC invites that have expired for too long.
func (c *Client) removeExpiredGCInvites(ctx context.Context) error {
	return c.db.Update(ctx, func(tx clientdb.ReadWriteTx) error {
		invites, err := c.db.ListGCInvites(tx, nil)
		if err != nil {
			return err
		}

		now := time.Now()
		for _, invite := range invites {
			expires := time.Unix(invite.Invite.Expires, 0)
			if expires.After(now) {
				continue
			}

			c.log.Debugf("Removing expired GC invite %d", invite.ID)
			if err := c.db.DelGCInvite(tx, invite.ID); err != nil {
				return err
			}
		}
		return err
	})
}
