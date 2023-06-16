package client

import (
	"errors"
	"fmt"
	"time"

	"github.com/companyzero/bisonrelay/client/clientdb"
	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/companyzero/bisonrelay/rpc"
)

// sendKXSearchQuery sends the specified kx search to the specified user.
func (c *Client) sendKXSearchQuery(target UserID, search rpc.RMKXSearch, to UserID) error {
	query := clientdb.KXSearchQuery{
		User:     to,
		DateSent: time.Now(),
	}

	err := c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		return c.db.AddKXSearchQuery(tx, target, search, query)
	})
	if err != nil {
		return err
	}

	ru, err := c.rul.byID(to)
	if err != nil {
		return err
	}
	ru.log.Infof("Querying user during KX search of %s", target)
	payEvent := fmt.Sprintf("kxsearch.%s", target)
	return c.sendWithSendQ(payEvent, search, to)
}

// KXSearchPostAuthor attempts to start a new kx search for the author of the
// specified post.
func (c *Client) KXSearchPostAuthor(postFrom UserID, post clientintf.PostID) error {

	var postAuthor UserID
	var kxs clientdb.KXSearch
	newSearch := false
	err := c.dbView(func(tx clientdb.ReadTx) error {
		// Verify post exists.
		post, err := c.db.ReadPost(tx, postFrom, post)
		if err != nil {
			return err
		}

		if err := postAuthor.FromString(post.Attributes[rpc.RMPStatusFrom]); err != nil {
			return fmt.Errorf("invalid author field in post: %v", err)
		}
		var emptyUID UserID
		if postAuthor == emptyUID {
			return fmt.Errorf("post author is empty in post")
		}

		kxs, err = c.db.GetKXSearch(tx, postAuthor)
		if errors.Is(err, clientdb.ErrNotFound) {
			newSearch = true
		}

		return nil
	})
	if err != nil {
		return err
	}

	if ru, err := c.rul.byID(postAuthor); err == nil {
		// Already know author. Just return.
		return fmt.Errorf("already know author %s", ru)
	}

	if postAuthor == c.PublicID() {
		return fmt.Errorf("cannot KX search self")
	}

	search := kxs.Search
	newRef := true
	if !newSearch {
		// Double check this post is not ref'd yet (otherwise this will
		// be a duplicate ref.
		postStr := post.String()
		for _, ref := range search.Refs {
			if ref.Type == rpc.KXSRTPostAuthor && ref.Ref == postStr {
				newRef = false
				break
			}
		}
	}

	// Check if this relayer was already contacted in a previous search
	// using this specific ref (no point in doing it again).
	alreadyQueried := false
	for _, query := range kxs.Queries {
		if query.User == postFrom {
			alreadyQueried = true
			break
		}
	}

	if !newRef && alreadyQueried {
		return fmt.Errorf("already searched for the author of the post %s "+
			"in relayer %s", post, postFrom)
	}
	c.log.Infof("Starting KX search for %s", postAuthor)

	search.Refs = append(search.Refs, rpc.RMKXSearchRef{
		Type: rpc.KXSRTPostAuthor,
		Ref:  post.String(),
	})

	// Store a post kx action to fetch the identified post from the user
	// after KX completes.
	if err := c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		action := clientdb.PostKXAction{
			Type:      clientdb.PKXActionFetchPost,
			DateAdded: time.Now(),
			Data:      post.String(),
		}
		return c.db.AddUniquePostKXAction(tx, postAuthor, action)
	}); err != nil {
		return err
	}

	return c.sendKXSearchQuery(postAuthor, search, postFrom)
}

func (c *Client) handleKXSearch(ru *RemoteUser, search rpc.RMKXSearch) error {

	// Based on the search queries, identify the target and potential
	// candidates for the search to continue.
	var targetID, emptyID UserID
	allRelayers := make(map[UserID]struct{})
	for _, ref := range search.Refs {
		switch ref.Type {
		case rpc.KXSRTPostAuthor:
			var pid clientdb.PostID
			if err := pid.FromString(ref.Ref); err != nil {
				ru.log.Warnf("Invalid post id sent in post author kx search: %v", err)
				continue
			}

			err := c.dbView(func(tx clientdb.ReadTx) error {
				relayers, err := c.db.ListPostRelayers(tx, pid)
				if err != nil {
					return err
				}
				if len(relayers) == 0 {
					return nil
				}
				for _, uid := range relayers {
					allRelayers[uid] = struct{}{}
				}
				if targetID != emptyID {
					return nil
				}

				// Figure out the target (author of these posts).
				post, err := c.db.ReadPost(tx, relayers[0], pid)
				if err != nil {
					return err
				}
				var uid UserID
				if err := uid.FromString(post.Attributes[rpc.RMPStatusFrom]); err == nil {
					targetID = uid
				}
				return nil
			})
			if err != nil {
				return err
			}

		default:
			ru.log.Debugf("Received unknown type of ref in kx search: %q", ref.Type)
		}
	}

	if targetID == emptyID {
		ru.log.Warnf("Received KX search for undetermined target")
		return nil
	}

	// See if we know the target directly.
	var ids []UserID
	targetRU, err := c.rul.byID(targetID)
	if err == nil && targetRU != nil {
		// We know! Reply with only the target.
		ru.log.Infof("Found KX search target %s", targetRU)
		ids = []UserID{targetID}
	} else {
		// We don't. Reply with potential list of people they can try to KX with
		// to continue their search.
		if len(allRelayers) == 0 {
			ru.log.Infof("Received KX search request for which we don't have any relayers")
		} else {
			ru.log.Infof("Replying to KX search for user %s with %d candidates",
				targetID, len(allRelayers))
		}

		ids = make([]UserID, 0, len(allRelayers))
		for uid := range allRelayers {
			ids = append(ids, uid)
		}
	}

	reply := rpc.RMKXSearchReply{TargetID: targetID, IDs: ids}
	payEvent := fmt.Sprintf("kxsearchreply.%s", targetID)
	return ru.sendRM(reply, payEvent)
}

func (c *Client) handleKXSearchReply(ru *RemoteUser, sr rpc.RMKXSearchReply) error {
	// Early check for no work to be done.
	if len(sr.IDs) == 0 {
		ru.log.Infof("Received empty list of KX search candidates "+
			"for target %s", sr.TargetID)
		return nil
	}

	// Track remote users from the reply we already queried or already
	// requested a mediate identity with.
	alreadyQueried := make(map[UserID]struct{})
	alreadyMIng := make(map[UserID]struct{})

	var kxs clientdb.KXSearch
	ruID := ru.ID()
	err := c.dbView(func(tx clientdb.ReadTx) error {
		// Find out if this kx search actually exists and that this
		// remote user was queried for it.
		var err error
		kxs, err = c.db.GetKXSearch(tx, sr.TargetID)
		if err != nil {
			return err
		}

		found := false
		for _, qry := range kxs.Queries {
			alreadyQueried[qry.User] = struct{}{}
			if qry.User == ruID {
				found = true
			}
		}
		if !found {
			return fmt.Errorf("did not send KX search query for %s "+
				"to this user", sr.TargetID)
		}

		// Figure out who from the reply we're already attempting to KX
		// with.
		for _, uid := range sr.IDs {
			hasMI, err := c.db.HasAnyRecentMediateID(tx, uid,
				c.cfg.RecentMediateIDThreshold)
			if err != nil {
				return err
			}
			if hasMI {
				alreadyMIng[uid] = struct{}{}
			}
		}

		return err
	})
	if err != nil {
		return err
	}

	// Ensure we haven't KX'd with the target yet.
	if targetRU, err := c.rul.byID(kxs.Target); err == nil {
		// We have!
		ru.log.Debugf("Ignoring KX search reply due to already knowing target %s",
			targetRU)
		return nil
	}

	// For every ID sent in the reply, decide what to do.
	me := c.PublicID()
	for _, uid := range sr.IDs {
		// I shared the post.
		if uid == me {
			c.log.Trace("Ignoring myself in set of KX search candidates")
			continue
		}

		// If already queried, no more to do.
		if _, ok := alreadyQueried[uid]; ok {
			c.log.Tracef("Already queried %s in set of KX search "+
				"candidates", uid)
			continue
		}

		// If we know this user (but haven't queried), try to send a
		// query to them.
		if _, err := c.rul.byID(uid); err == nil {
			ru.log.Infof("Attempting to send KX search query to %s "+
				"for target %s", uid, kxs.Target)
			if err := c.sendKXSearchQuery(kxs.Target, kxs.Search, uid); err != nil {
				return err
			}
			continue
		}

		// We don't know this user. If we're already attempting to MI
		// with them, continue.
		if _, ok := alreadyMIng[uid]; ok {
			ru.log.Tracef("Already attempting to MI with KX search "+
				"candidate %s", uid)
			continue
		}

		// Otherwise, attempt to MI. Register that we want to keep the
		// kx search procedure with this user after kx completes.
		if err := c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
			action := clientdb.PostKXAction{
				Type:      clientdb.PKXActionKXSearch,
				DateAdded: time.Now(),
				Data:      kxs.Target.String(),
			}
			return c.db.AddUniquePostKXAction(tx, uid, action)
		}); err != nil {
			return err
		}
		c.log.Infof("Found candidate relayer %s during kx search for %s",
			uid, kxs.Target)
		if err := c.RequestMediateIdentity(ru.ID(), uid); err != nil {
			return err
		}
		alreadyMIng[uid] = struct{}{}
	}

	return nil
}

// GetKXSearch returns the KX search status for the given target.
func (c *Client) GetKXSearch(targetID UserID) (clientdb.KXSearch, error) {
	var kxs clientdb.KXSearch
	err := c.dbView(func(tx clientdb.ReadTx) error {
		var err error
		kxs, err = c.db.GetKXSearch(tx, targetID)
		return err
	})
	return kxs, err
}

// ListKXSearches lists the IDs of all outstanding KX searches.
func (c *Client) ListKXSearches() ([]UserID, error) {
	var res []UserID
	err := c.dbView(func(tx clientdb.ReadTx) error {
		var err error
		res, err = c.db.ListKXSearches(tx)
		return err
	})
	return res, err
}
