package client

import (
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/companyzero/bisonrelay/client/clientdb"
	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/companyzero/bisonrelay/internal/strescape"
	"github.com/companyzero/bisonrelay/ratchet"
	"github.com/companyzero/bisonrelay/rpc"
	"github.com/companyzero/bisonrelay/zkidentity"
)

// Client KX flow is:
//
//          Alice                                    Bob
//         -------                                  -----
//
//     WriteNewInvite
//        \-----> OOBPublicIdentityInvite -->
//                  (out-of-band send)
//
//     kxlist.listenInvite()
//
//                                           ReadInvite()
//                                           AcceptInvite()
//                                           kxlist.acceptInvite()
//                               <-- RMOHalfKX ---/
//
//    kxlist.handleStep2IDKX()
//            \---- RMOFullKX -->
//    initRemoteUser()
//
//                                            kxlist.handleStep3IDKX()
//                                            initRemoteUser()
//

func (c *Client) takePostKXAction(ru *RemoteUser, act clientdb.PostKXAction) error {
	switch act.Type {
	case clientdb.PKXActionKXSearch:
		// Startup a KX search.
		var targetID UserID
		if err := targetID.FromString(act.Data); err != nil {
			return err
		}

		// See if we haven't found the target yet.
		if _, err := c.rul.byID(targetID); err == nil {
			// We have!
			return nil
		}

		// Not yet. Send the KX search query to them.
		var kxs clientdb.KXSearch
		if err := c.dbView(func(tx clientdb.ReadTx) error {
			var err error
			kxs, err = c.db.GetKXSearch(tx, targetID)
			return err
		}); err != nil {
			return err
		}

		return c.sendKXSearchQuery(kxs.Target, kxs.Search, ru.ID())

	case clientdb.PKXActionFetchPost:
		// Subscribe to posts, then fetch the specified post.
		var pid clientdb.PostID
		if err := pid.FromString(act.Data); err != nil {
			return err
		}

		return c.subscribeToPosts(ru.ID(), &pid, true)

	default:
		return fmt.Errorf("unknown post-kx action type")
	}
}

// takePostKXActions takes any post-kx actions needed after the user has been
// initialized.
func (c *Client) takePostKXActions(ru *RemoteUser, actions []clientdb.PostKXAction) {
	for _, act := range actions {
		act := act
		go func() {
			err := c.takePostKXAction(ru, act)
			if err != nil {
				ru.log.Errorf("Unable to take post-KX action %q: %v",
					act.Type, err)
			}
		}()
	}

	err := c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		return c.db.RemovePostKXActions(tx, ru.ID())
	})
	if err != nil {
		ru.log.Warnf("Unable to move post-KX actions: %v", err)
	}
}

// initRemoteUser inserts the given ratchet as a new remote user.
func (c *Client) initRemoteUser(id *zkidentity.PublicIdentity, r *ratchet.Ratchet,
	updateAB bool, myResetRV, theirResetRV clientdb.RawRVID, ignored bool) (*RemoteUser, error) {

	var postKXActions []clientdb.PostKXAction

	// Track the new user.
	ru := newRemoteUser(c.q, c.rmgr, c.db, id, c.id, r)
	ru.ignored = ignored
	ru.compressLevel = c.cfg.CompressLevel
	ru.log = c.cfg.logger(fmt.Sprintf("RUSR %x", id.Identity[:8]))
	ru.logPayloads = c.cfg.logger(fmt.Sprintf("RMPL %x", id.Identity[:8]))
	ru.rmHandler = c.handleUserRM

	oldRU, err := c.rul.add(ru)
	oldUser := false
	if errors.Is(err, alreadyHaveUserError{}) && oldRU != nil {
		oldRU.log.Tracef("Reusing old remote user and replacing ratchet")

		// Already have this user running. Replace the ratchet with the
		// new one.
		ru = oldRU
		go ru.replaceRatchet(r)
		oldUser = true
	} else if err != nil {
		return nil, err
	} else {
		ru.log.Debugf("Initializing remote user")
	}

	// Save the newly formed address book entry to the DB.
	var oldEntry *clientdb.AddressBookEntry
	hadKXSearch := false
	err = c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		var err error
		oldEntry, err = c.db.GetAddressBookEntry(tx, id.Identity, c.id)
		if err != nil && !errors.Is(err, clientdb.ErrNotFound) {
			return err
		}

		if err := c.db.UpdateRatchet(tx, r, id.Identity); err != nil {
			return err
		}
		var ignored bool
		if oldEntry != nil {
			ignored = oldEntry.Ignored
		}
		if updateAB {
			if err := c.db.UpdateAddressBookEntry(tx, id, myResetRV,
				theirResetRV, ignored); err != nil {
				return err
			}

			// Log in the user chat that kx completed.
			if oldEntry == nil {
				c.db.LogPM(tx, id.Identity, true, "", "Completed KX", time.Now())
			} else {
				c.db.LogPM(tx, id.Identity, true, "", "Re-done KX", time.Now())
			}
		}

		// See if there are any actions to be taken after completing KX.
		postKXActions, err = c.db.ListPostKXActions(tx, id.Identity)
		if err != nil {
			return err
		}

		// Remove KX search if it exists.
		if _, err := c.db.GetKXSearch(tx, id.Identity); err == nil {
			hadKXSearch = true
		}
		if err := c.db.RemoveKXSearch(tx, id.Identity); err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	// Change the reset listening state on a goroutine so we don't block on
	// it.
	go func() {
		// Unsubscribe from the old reset RV point.
		if oldEntry != nil {
			c.kxl.unlistenReset(oldEntry.MyResetRV)
		}

		// Subscribe to the reset RV point.
		if err := c.kxl.listenReset(myResetRV, id); err != nil {
			ru.log.Warnf("unable to listen to reset: %v", err)
		}
	}()

	// Run the new user.
	if !oldUser {
		select {
		case c.newUsersChan <- ru:
		case <-c.runDone:
			return nil, errClientExiting
		}
	}

	// If there are any post-kx actions to be taken, start them up.
	if len(postKXActions) > 0 {
		go c.takePostKXActions(ru, postKXActions)
	}

	// If this target was the subject of a KX search, trigger event.
	if hadKXSearch && c.cfg.KXSearchCompleted != nil {
		c.cfg.KXSearchCompleted(ru)
	}

	return ru, nil
}

func (c *Client) kxCompleted(public *zkidentity.PublicIdentity, r *ratchet.Ratchet,
	myResetRV, theirResetRV clientdb.RawRVID) {

	ru, err := c.initRemoteUser(public, r, true, myResetRV, theirResetRV, false)
	if err != nil && !errors.Is(err, clientintf.ErrSubsysExiting) {
		c.log.Errorf("unable to init user for completed kx: %v", err)
	}

	if c.cfg.KXCompleted != nil {
		c.cfg.KXCompleted(ru)
	}
}

// WriteNewInvite creates a new invite and writes it to the given writer.
func (c *Client) WriteNewInvite(w io.Writer) (rpc.OOBPublicIdentityInvite, error) {
	return c.kxl.createInvite(w, nil, nil, false)
}

// ReadInvite decodes an invite from the given reader. Note the invite is not
// acted upon until AcceptInvite is called.
func (c *Client) ReadInvite(r io.Reader) (rpc.OOBPublicIdentityInvite, error) {
	return c.kxl.decodeInvite(r)
}

// AcceptInvite blocks until the remote party reponds with us accepting the
// remote party's invitation. The invite should've been created by ReadInvite.
func (c *Client) AcceptInvite(invite rpc.OOBPublicIdentityInvite) error {
	return c.kxl.acceptInvite(invite, false)
}

// ResetRatchet requests a ratchet reset with the given user.
func (c *Client) ResetRatchet(uid UserID) error {
	ru, err := c.rul.byID(uid)
	if err != nil {
		return err
	}

	var resetRV clientdb.RawRVID
	err = c.dbView(func(tx clientdb.ReadTx) error {
		ab, err := c.db.GetAddressBookEntry(tx, uid, c.id)
		if err != nil {
			return err
		}
		resetRV = ab.TheirResetRV
		return nil
	})
	if err != nil {
		return err
	}

	ru.log.Infof("Initiating reset via RV %s", resetRV)

	return c.kxl.requestReset(resetRV, ru.id)
}

func (c *Client) ListKXs() ([]clientdb.KXData, error) {
	var kxs []clientdb.KXData
	err := c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		var err error
		kxs, err = c.db.ListKXs(tx)
		return err
	})

	return kxs, err
}

// IsIgnored indicates whether the given client has the ignored flag set.
func (c *Client) IsIgnored(uid clientintf.UserID) (bool, error) {
	ru, err := c.rul.byID(uid)
	if err != nil {
		return false, err
	}
	return ru.IsIgnored(), nil
}

// Ignore changes the setting of the local ignore flag of the specified user.
func (c *Client) Ignore(uid UserID, ignore bool) error {
	ru, err := c.rul.byID(uid)
	if err != nil {
		return err
	}
	isIgnored := ru.IsIgnored()
	if ignore {
		if isIgnored {
			return fmt.Errorf("user is already ignored")
		}
		ru.SetIgnored(true)
		c.log.Infof("Ignoring user %s", ru)
	} else {
		if !isIgnored {
			return fmt.Errorf("user was not ignored")
		}
		ru.SetIgnored(false)
		c.log.Infof("Un-ignoring user %s", ru)
	}

	return c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		ab, err := c.db.GetAddressBookEntry(tx, ru.ID(), c.id)
		if err != nil {
			return err
		}

		return c.db.UpdateAddressBookEntry(tx, ru.id, ab.MyResetRV,
			ab.TheirResetRV, ru.IsIgnored())
	})
}

// Block blocks a uid.
func (c *Client) Block(uid UserID) error {
	<-c.abLoaded

	ru, err := c.rul.byID(uid)
	if err != nil {
		return err
	}
	c.log.Infof("Blocking user %s", ru)

	payEvent := "blockUser"
	err = ru.sendRMPriority(rpc.RMBlock{}, payEvent, priorityPM)
	if err != nil {
		return err
	}

	// Delete user
	c.rul.del(ru)
	ru.stop()
	return c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		return c.db.RemoveUser(tx, ru.ID(), true)
	})
}

// handleRMBlock handles an incoming block message.
func (c *Client) handleRMBlock(ru *RemoteUser, bl rpc.RMBlock) error {
	c.log.Infof("Blocking user due to received request: %s", ru)

	// Delete user
	c.rul.del(ru)
	ru.stop()
	err := c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		return c.db.RemoveUser(tx, ru.ID(), true)
	})
	if err != nil {
		return err
	}

	if c.cfg.UserBlocked != nil {
		c.cfg.UserBlocked(ru)
	}
	return nil
}

// RenameUser modifies the nick for the specified user.
func (c *Client) RenameUser(uid UserID, newNick string) error {
	<-c.abLoaded

	ru, err := c.rul.byID(uid)
	if err != nil {
		return err
	}

	_, err = c.UserByNick(newNick)
	if err == nil {
		return fmt.Errorf("user with nick %q already exists", newNick)
	}

	ru.log.Infof("Renaming user to %q", newNick)
	c.rul.modifyUserNick(ru, newNick)

	return c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		ab, err := c.db.GetAddressBookEntry(tx, ru.ID(), c.id)
		if err != nil {
			return err
		}

		return c.db.UpdateAddressBookEntry(tx, ru.id, ab.MyResetRV,
			ab.TheirResetRV, ru.IsIgnored())
	})
}

// SuggestKX sends a message to invitee suggesting they KX with target (through
// the local client).
func (c *Client) SuggestKX(invitee, target UserID) error {
	_, err := c.rul.byID(invitee)
	if err != nil {
		return err
	}

	ruTarget, err := c.rul.byID(target)
	if err != nil {
		return err
	}

	rm := rpc.RMKXSuggestion{Target: ruTarget.PublicIdentity()}
	payEvent := "kxsuggest." + target.String()
	return c.sendWithSendQ(payEvent, rm, invitee)
}

func (c *Client) handleKXSuggestion(ru *RemoteUser, kxsg rpc.RMKXSuggestion) error {
	known := "known"
	_, err := c.rul.byID(kxsg.Target.Identity)
	if err != nil {
		known = "unknown"
	}

	ru.log.Infof("Received suggestion to KX with %s %s (%q)", known,
		kxsg.Target.Identity, strescape.Nick(kxsg.Target.Nick))

	if c.cfg.KXSuggestion != nil {
		c.cfg.KXSuggestion(ru, kxsg.Target)
	}
	return nil
}
