package client

import (
	"crypto/ed25519"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/companyzero/bisonrelay/client/clientdb"
	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/companyzero/bisonrelay/rpc"
	"github.com/companyzero/bisonrelay/zkidentity"
)

// subscribeToPosts subscribes to the given user posts, optionally fetching
// the given post as well.
func (c *Client) subscribeToPosts(uid UserID, pid *clientintf.PostID, includeStatus bool) error {
	ru, err := c.rul.byID(uid)
	if err != nil {
		return err
	}

	payEvent := "posts.subscribe"
	rm := rpc.RMPostsSubscribe{GetPost: pid, IncludeStatus: includeStatus}
	err = c.sendWithSendQ(payEvent, rm, uid)
	if err != nil {
		return err
	}
	ru.log.Infof("Subscribing to posts")
	return nil
}

// SubscribeToPosts attempts to subscribe to the posts of the given user.
func (c *Client) SubscribeToPosts(uid UserID) error {
	return c.subscribeToPosts(uid, nil, false)
}

// SubscribeToPostsAndFetch attempts to subscribe to the posts of the given user
// and also (if successful) asks the user to send the specified post.
func (c *Client) SubscribeToPostsAndFetch(uid UserID, pid clientintf.PostID) error {
	return c.subscribeToPosts(uid, &pid, true)
}

func (c *Client) handlePostsSubscribe(ru *RemoteUser, ps rpc.RMPostsSubscribe) error {
	var post rpc.PostMetadata
	var updates []rpc.PostMetadataStatus
	err := c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		err := c.db.SubscribeToPosts(tx, ru.ID())
		if err != nil {
			return err
		}

		if ps.GetPost == nil {
			return nil
		}

		if post, err = c.db.ReadPost(tx, c.PublicID(), *ps.GetPost); err != nil {
			return err
		}
		if ps.IncludeStatus {
			if updates, err = c.db.ListPostStatusUpdates(tx, c.PublicID(), *ps.GetPost); err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil && !errors.Is(err, clientdb.ErrAlreadySubscribed) {
		return err
	}

	var errMsg *string
	if err != nil {
		msg := err.Error()
		errMsg = &msg

		ru.log.Infof("Failed store remote user subscription: %v", err)
	} else {
		ru.log.Infof("Subscribed to our posts")

		c.ntfns.notifyPostsSubscriberUpdated(ru, true)
	}

	rm := rpc.RMPostsSubscribeReply{Error: errMsg}
	payEvent := "posts.subscribereply"
	if err := c.sendWithSendQ(payEvent, rm, ru.ID()); err != nil {
		return err
	}

	if errMsg != nil || ps.GetPost == nil {
		return nil
	}

	// Have post to send.
	return c.sendPostToUser(ru, *ps.GetPost, post, updates)
}

func (c *Client) handlePostsSubscribeReply(ru *RemoteUser, psr rpc.RMPostsSubscribeReply) error {
	if psr.Error != nil {
		subErr := strings.TrimSpace(*psr.Error)
		ru.log.Warnf("Received error reply when subscribing to posts: %q", subErr)
		c.ntfns.notifyOnRemoteSubErrored(ru, true, subErr)
	} else {
		uid := ru.ID()
		err := c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
			return c.db.StorePostSubscription(tx, uid)
		})
		if err != nil {
			return err
		}
		ru.log.Infof("Successfully subscribed to posts")

		c.ntfns.notifyOnRemoteSubChanged(ru, true)
	}

	return nil
}

// UnsubscribeToPosts unsubscribes the local user to the posts made by the
// given remote user.
//
// It returns when the remote user replies or when the client exits.
func (c *Client) UnsubscribeToPosts(uid UserID) error {
	ru, err := c.rul.byID(uid)
	if err != nil {
		return err
	}

	payEvent := "posts.unsubscribe"
	err = c.sendWithSendQ(payEvent, rpc.RMPostsUnsubscribe{}, uid)
	if err != nil {
		return err
	}

	// Ensure we store the unsubscription, in case we start receiving new
	// posts from the remote before it processes our unsubscribe request.
	err = c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		return c.db.StorePostUnsubscription(tx, uid)
	})
	if err != nil {
		return err
	}

	ru.log.Infof("Unsubscribing to posts")
	return nil
}

// unsubRemoteFromLocalPosts unsubscribes a remote user from local posts.
func (c *Client) unsubRemoteFromLocalPosts(ru *RemoteUser, replyEvenIfErr bool) error {
	err := c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		return c.db.UnsubscribeToPosts(tx, ru.ID())
	})

	if err != nil && !errors.Is(err, clientdb.ErrNotSubscribed) {
		return err
	}

	if err == nil {
		ru.log.Infof("Unsubscribed to local posts")
		c.ntfns.notifyPostsSubscriberUpdated(ru, false)
	}

	if err == nil || replyEvenIfErr {
		rm := rpc.RMPostsUnsubscribeReply{}
		payEvent := "posts.ubsubscribereply"
		return c.sendWithSendQ(payEvent, rm, ru.ID())
	}

	return nil
}

func (c *Client) handlePostsUnsubscribe(ru *RemoteUser, pu rpc.RMPostsUnsubscribe) error {
	return c.unsubRemoteFromLocalPosts(ru, true)
}

func (c *Client) handlePostsUnsubscribeReply(ru *RemoteUser, psr rpc.RMPostsUnsubscribeReply) error {
	if psr.Error != nil {
		unsubErr := strings.TrimSpace(*psr.Error)
		ru.log.Warnf("Received error reply when unsubscribing to posts: %q", unsubErr)
		c.ntfns.notifyOnRemoteSubErrored(ru, false, unsubErr)
	} else {
		// Double check we unsubscribed.
		uid := ru.ID()
		err := c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
			return c.db.StorePostUnsubscription(tx, uid)
		})
		if err != nil {
			return err
		}
		ru.log.Infof("Successfully unsubscribed to posts")

		c.ntfns.notifyOnRemoteSubChanged(ru, false)
	}

	return nil
}

// HasPostSubscribers returns true if the local client has subscribers to our
// posts.
func (c *Client) HasPostSubscribers() (bool, error) {
	var hasSubs bool
	err := c.dbView(func(tx clientdb.ReadTx) error {
		subs, err := c.db.ListPostSubscribers(tx)
		hasSubs = len(subs) > 0
		return err
	})
	return hasSubs, err
}

// ListPostSubscribers lists the subscribers to the local client's posts.
func (c *Client) ListPostSubscribers() ([]clientintf.UserID, error) {
	var subs []clientintf.UserID
	err := c.dbView(func(tx clientdb.ReadTx) error {
		var err error
		subs, err = c.db.ListPostSubscribers(tx)
		return err
	})
	return subs, err
}

// ListPostSubscriptions lists remote users whose posts we are subscribed to.
func (c *Client) ListPostSubscriptions() ([]clientdb.PostSubscription, error) {
	var subs []clientdb.PostSubscription
	err := c.dbView(func(tx clientdb.ReadTx) error {
		var err error
		subs, err = c.db.ListPostSubscriptions(tx)
		return err
	})
	return subs, err
}

// ListUserPosts lists the posts made by the specified user.
func (c *Client) ListUserPosts(uid UserID) error {
	ru, err := c.rul.byID(uid)
	if err != nil {
		return err
	}

	ru.log.Infof("Listing user posts")

	return ru.sendRM(rpc.RMListPosts{}, "posts.list")
}

func (c *Client) handleListPosts(ru *RemoteUser, lp rpc.RMListPosts) error {
	var posts []rpc.PostMetadata
	err := c.dbView(func(tx clientdb.ReadTx) error {
		var err error
		posts, err = c.db.ListUserPosts(tx, c.PublicID())
		return err
	})
	if err != nil {
		return err
	}

	ru.log.Infof("Listing %d posts to user", len(posts))

	rm := rpc.RMListPostsReply{
		Posts: make([]rpc.PostListItem, 0, len(posts)),
	}
	for _, p := range posts {
		var id clientintf.PostID
		if err := id.FromString(p.Attributes[rpc.RMPIdentifier]); err != nil {
			continue
		}
		rm.Posts = append(rm.Posts, rpc.PostListItem{
			ID:    id,
			Title: clientintf.PostTitle(&p),
		})
	}

	return ru.sendRM(rm, "posts.listreply")
}

func (c *Client) handleListPostsReply(ru *RemoteUser, plr rpc.RMListPostsReply) error {
	ru.log.Infof("Received list of posts (total posts: %d)", len(plr.Posts))
	c.ntfns.notifyPostsListReceived(ru, plr)
	return nil
}

// shareWithPostSubscribers sends the given post share message to the passed
// post subscribers.
func (c *Client) shareWithPostSubscribers(subs []clientintf.UserID,
	pid clientintf.PostID, rm rpc.RMPostShare, payType string) error {

	if len(subs) == 0 {
		c.log.Warnf("Attempting to share post without any subscribers")
		return nil
	}
	c.log.Debugf("Attempting to share post with %d subscribers", len(subs))

	// Store queue of msgs in db.
	payEvent := fmt.Sprintf("posts.%s.%s", pid.ShortLogID(), payType)
	sqid, err := c.addToSendQ(payEvent, rm, priorityDefault, subs...)
	if err != nil {
		return err
	}

	for _, uid := range subs {
		ru, err := c.rul.byID(uid)
		if errors.Is(err, userNotFoundError{}) {
			c.log.Warnf("unable to find subscriber to share post with: %v", err)
			continue
		}
		if err != nil {
			return err
		}

		// Send as a goroutine so all shares are concurrent.
		go func() {
			err := ru.sendRM(rm, payEvent)
			if err != nil {
				if !errors.Is(err, clientintf.ErrSubsysExiting) {
					ru.log.Errorf("unable to send RMPostShare: %v", err)
				}
				return
			}
			c.removeFromSendQ(sqid, ru.ID())
			ru.log.Debugf("Shared post %s with user", pid)
		}()
	}
	return nil
}

// CreatePost creates a new post and shares it with all current subscribers.
func (c *Client) CreatePost(post, descr string) (clientdb.PostSummary, error) {
	// Filename for embedded data is not currently used, so it's disabled at
	// the client API level.
	const fname = ""

	var pm rpc.PostMetadata
	var subs []clientdb.UserID
	var summ clientdb.PostSummary
	err := c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		var err error
		summ, pm, err = c.db.CreatePost(tx, post, descr, fname, nil, c.id)
		if err != nil {
			return err
		}

		subs, err = c.db.ListPostSubscribers(tx)
		return err
	})
	if err != nil {
		return summ, fmt.Errorf("unable to create local post: %w", err)
	}

	c.log.Infof("Created post %s", summ.ID)
	rm := rpc.RMPostShare(pm)
	if err := c.shareWithPostSubscribers(subs, summ.ID, rm, "sharecreated"); err != nil {
		return summ, err
	}

	return summ, nil
}

// verifyPostSignature returns an error if we fail to verify the signature
// in the post. Note that when the author of the post status is unknown, this _also_
// returns nil, as there's no way to globally verify the identity of the author.
func (c *Client) verifyPostSignature(p rpc.PostMetadata) error {
	failf := func(f string, args ...interface{}) error {
		return fmt.Errorf("cannot verify post signature: "+f,
			args...)
	}

	var from UserID
	if err := from.FromString(p.Attributes[rpc.RMPStatusFrom]); err != nil {
		return failf("unable to decode RMPStatusFrom: %v", err)
	}

	if from == c.PublicID() {
		return nil
	}

	// Select the public key to check (either the local client or from
	// a known user).
	var pubid zkidentity.PublicIdentity
	if from == c.PublicID() {
		pubid = c.id.Public
	} else {
		ru, err := c.rul.byID(from)
		if err != nil {
			c.log.Warnf("Unable to verify signature on post %x: "+
				"unknown author", p.Hash())
			return nil
		}
		pubid = *ru.id
	}

	var sig [ed25519.SignatureSize]byte
	sigStr := p.Attributes[rpc.RMPSignature]
	if len(sigStr) != len(sig)*2 {
		return failf("rpc.RMPSignature has wrong len (%d != %d)",
			len(sigStr), len(sig)*2)
	}

	if _, err := hex.Decode(sig[:], []byte(sigStr)); err != nil {
		return failf("unable to decode RMPSignature: %v", err)
	}

	msg := p.Hash()
	if !pubid.VerifyMessage(msg[:], sig) {
		return failf("signature failed verification")
	}

	return nil
}

// verifyPostStatusSignature returns an error if we fail to verify the signature
// in rmps. Note that when the author of the post status is unknown, this _also_
// returns nil, as there's no way to globally verify the identity of the author.
func (c *Client) verifyPostStatusSignature(pms rpc.PostMetadataStatus) error {
	failf := func(f string, args ...interface{}) error {
		return fmt.Errorf("cannot verify status update signature: "+f,
			args...)
	}

	var from UserID
	if err := from.FromString(pms.Attributes[rpc.RMPStatusFrom]); err != nil {
		return failf("unable to decode RMPStatusFrom: %v", err)
	}

	// Select the public key to check (either the local client or from
	// a known user).
	var pubid zkidentity.PublicIdentity
	if from == c.PublicID() {
		pubid = c.id.Public
	} else {
		ru, err := c.rul.byID(from)
		if err != nil {
			c.log.Warnf("Unable to verify signature on post status %x: "+
				"unknown author", pms.Hash())
			return nil
		}
		pubid = *ru.id
	}

	if pms.From == "" {
		pms.From = pms.Attributes[rpc.RMPStatusFrom]
	}

	var sig [ed25519.SignatureSize]byte
	sigStr := pms.Attributes[rpc.RMPSignature]
	if len(sigStr) != len(sig)*2 {
		return failf("rpc.RMPSignature has wrong len (%d != %d)",
			len(sigStr), len(sig)*2)
	}

	if _, err := hex.Decode(sig[:], []byte(sigStr)); err != nil {
		return failf("unable to decode RMPSignature: %v", err)
	}

	msg := pms.Hash()
	if !pubid.VerifyMessage(msg[:], sig) {
		return failf("signature failed verification")
	}
	return nil
}

func (c *Client) handlePostShare(ru *RemoteUser, ps rpc.RMPostShare) error {
	var p = rpc.PostMetadata(ps)
	var pid clientintf.PostID
	var statusFrom UserID
	var update rpc.PostMetadataStatus

	if s, ok := p.Attributes[rpc.RMPIdentifier]; !ok {
		return fmt.Errorf("post does not have an identifier")
	} else if err := pid.FromString(s); err != nil {
		return err
	}

	errStatusWithoutPost := errors.New("status without post")
	errHaveCopyFromAuthor := errors.New("have post copy from author")
	errFilter := errors.New("filtered post/comment")

	var summ clientdb.PostSummary
	var isUpdate bool
	from := ru.ID()
	err := c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		// Ensure this came from someone we're subscribed.
		if ok, err := c.db.IsPostSubscription(tx, from); err != nil {
			return err
		} else if !ok {
			return fmt.Errorf("received post from someone we're not subscribed")
		}

		var err error
		if exists, err := c.db.PostExists(tx, from, pid); err != nil {
			return err
		} else if !exists && rpc.IsPostStatus(p.Attributes) {
			return errStatusWithoutPost
		} else if exists {
			// Verify post status signature.
			pms := rpc.PostMetadataStatus{
				Version:    p.Version,
				Attributes: p.Attributes,
			}
			if err := c.verifyPostStatusSignature(pms); err != nil {
				return err
			}

			// Post already exists. Handle as a status update.
			isUpdate = true

			// Status updates must have a StatusFrom attribute.
			if s, ok := p.Attributes[rpc.RMPStatusFrom]; !ok {
				return fmt.Errorf("post status does not have a from field")
			} else if err := statusFrom.FromString(s); err != nil {
				return fmt.Errorf("invalid statusfrom field: %v", err)
			}

			// Check if content should be filtered (don't filter
			// the client's own contents).
			comment := p.Attributes[rpc.RMPSComment]
			if comment != "" && statusFrom != c.PublicID() {
				filter, _ := c.FilterPostComment(statusFrom, from, pid, comment)
				if filter {
					return errFilter
				}
			}

			_, update, err = c.db.AddPostStatusUpdate(tx, from, p)
			return err
		}

		// Sanity check post has correct id.
		hash := zkidentity.ShortID(p.Hash())
		if pid != p.Hash() {
			return fmt.Errorf("received post where id %s is different then hash %s",
				pid, hash)
		}

		// Verify post signature.
		if err := c.verifyPostSignature(p); err != nil {
			return err
		}

		// If we received this post from its author, remove the relayed
		// copies.
		var postAuthor UserID
		if id, ok := p.Attributes[rpc.RMPStatusFrom]; ok {
			_ = postAuthor.FromString(id) // Ok to ingore error
		}
		if !postAuthor.IsEmpty() && from == postAuthor {
			err := c.db.RemoveRelayedPostCopies(from, pid)
			if err != nil {
				return err
			}
		}
		if !postAuthor.IsEmpty() && from != postAuthor {
			if exists, _ := c.db.PostExists(tx, postAuthor, pid); exists {
				// Ignore relayed copy of the post in favor of
				// the one from the author.
				return errHaveCopyFromAuthor
			}
		}

		// Check if content is filtered.
		if filter, _ := c.FilterPost(ru.ID(), pid, p.Attributes[rpc.RMPMain]); filter {
			return errFilter
		}

		// Post does not exist. Save it.
		pid, summ, err = c.db.SaveReceivedPost(tx, from, p)
		return err
	})
	if err != nil {
		// Ignore duplicate post status for the purposes of error (we
		// could have requested the post multiple times).
		if errors.Is(err, clientdb.ErrDuplicatePostStatus) {
			return nil
		}

		// Log error when we receive post status for unknown post
		// differently, as it's likely this is a post from before we
		// subscribed to the user.
		if errors.Is(err, errStatusWithoutPost) {
			ru.log.Warnf("Received post status for unknown post %s",
				pid)
			return nil
		}

		// Log a debug message, but otherwise ignore as the post from
		// the author is the authoritative one.
		if errors.Is(err, errHaveCopyFromAuthor) {
			ru.log.Debugf("Received relayed copy of post %s which "+
				"we already have from author", pid)
			return nil
		}

		// Ignore when the post's or comment's content is filtered.
		if errors.Is(err, errFilter) {
			return nil
		}
		return err
	}

	if !isUpdate {
		ru.log.Infof("Received post %s", pid)
		c.ntfns.notifyOnPostRcvd(ru, summ, p)
	} else {
		ru.log.Infof("Received post update %s from %s", pid, statusFrom)
		c.ntfns.notifyOnPostStatusRcvd(ru, pid, statusFrom, update)
	}

	return nil
}

// ListReceivedPosts lists all posts created or received by the local client.
func (c *Client) ListPosts() ([]clientdb.PostSummary, error) {
	var res []clientdb.PostSummary
	err := c.dbView(func(tx clientdb.ReadTx) error {
		var err error
		res, err = c.db.ListPosts(tx)
		return err
	})
	return res, err
}

// ReadReceivedPost returns the post data for the given user/post.
func (c *Client) ReadPost(uid clientintf.UserID, pid clientintf.PostID) (rpc.PostMetadata, error) {
	var res rpc.PostMetadata
	err := c.dbView(func(tx clientdb.ReadTx) error {
		var err error
		res, err = c.db.ReadPost(tx, uid, pid)
		return err
	})
	return res, err
}

// addStatusToPost adds the given post status update to the post and shares an
// update message with all subscribers. The statusFrom argument refers to who
// sent the status update (which may be the local client).
//
// This should only be called to add status to posts created by the local
// client.
func (c *Client) addStatusToPost(statusFrom clientintf.UserID, pms *rpc.PostMetadataStatus) error {
	var pid clientintf.PostID
	var err error
	var subs []clientintf.UserID
	postFrom := c.PublicID()
	statusFromMe := statusFrom == c.PublicID()

	if err = pid.FromString(pms.Link); err != nil {
		return fmt.Errorf("specified link is not a post ID: %v", err)
	}

	// Verify if the status is filtered.
	if !statusFromMe && pms.Attributes[rpc.RMPSComment] != "" {
		comment := pms.Attributes[rpc.RMPSComment]
		filter, _ := c.FilterPostComment(statusFrom, postFrom, pid, comment)
		if filter {
			// Returning from this point means we don't store and
			// don't share the post status with subscribers.
			return nil
		}
	}

	// Add status to DB (this validates the status udpate against
	// the DB as well).
	err = c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		if err := c.db.AddPostStatus(tx, postFrom, statusFrom, pid, pms); err != nil {
			return err
		}
		var err error
		subs, err = c.db.ListPostSubscribers(tx)
		return err
	})
	if err != nil {
		// Internal error: don't send reply.
		return err
	}

	// Prepare post share msg.
	attr := pms.Attributes
	attr[rpc.RMPIdentifier] = pid.String()
	attr[rpc.RMPStatusFrom] = statusFrom.String()
	attr[rpc.RMPVersion] = strconv.Itoa(int(pms.Version))
	rm := rpc.RMPostShare{Version: pms.Version, Attributes: attr}

	// Log received status.
	fromStr := "me"
	if !statusFromMe {
		ru, _ := c.rul.byID(statusFrom)
		if ru != nil {
			fromStr = ru.String()
		} else {
			fromStr = statusFrom.String()
		}
	}
	statusType := "status update"
	if _, ok := attr[rpc.RMPSComment]; ok {
		statusType = "comment"
	} else if _, ok := attr[rpc.RMPSHeart]; ok {
		statusType = "heart"
	}
	c.log.Infof("New %s %x from %s on post %s", statusType, pms.Hash(), fromStr, pid)

	// Alert UI that we have a new post status.
	c.ntfns.notifyOnPostStatusRcvd(nil, pid, statusFrom, *pms)

	// Send status update to all subscribers.
	if err := c.shareWithPostSubscribers(subs, pid, rm, "statusupdate"); err != nil {
		return err
	}
	return nil
}

// sendPostStatus sends the given list of attributes as a status update on a
// post.
func (c *Client) sendPostStatus(postFrom clientintf.UserID,
	pid clientintf.PostID, attr map[string]string) error {

	// Ensure post exists.
	var kxSearchTarget *UserID
	err := c.dbView(func(tx clientdb.ReadTx) error {
		post, err := c.db.ReadPost(tx, postFrom, pid)
		if err != nil {
			return err
		}

		// If this is a relayed post (postAuthor != postFrom), we won't
		// add the status in the relayed post but instead attempt to
		// KX search the author.
		var postAuthor UserID
		if id, ok := post.Attributes[rpc.RMPStatusFrom]; ok {
			_ = postAuthor.FromString(id) // Ok to ingore error
		}

		if postAuthor == postFrom {
			return nil
		}

		// Double check we don't have the original post.
		_, err = c.db.ReadPost(tx, postAuthor, pid)
		if err == nil {
			// We do. Modify postFrom to be for the original post.
			postFrom = postAuthor
		} else {
			kxSearchTarget = &postAuthor
		}

		return nil
	})
	if err != nil {
		return err
	}

	if kxSearchTarget != nil {
		return ErrKXSearchNeeded{Author: *kxSearchTarget}
	}

	// Status is coming from the local client.
	statusFrom := c.PublicID()

	attr[rpc.RMPVersion] = strconv.Itoa(rpc.PostMetadataStatusVersion)
	attr[rpc.RMPIdentifier] = pid.String()
	attr[rpc.RMPStatusFrom] = statusFrom.String()
	attr[rpc.RMPNonce] = strconv.FormatUint(c.mustRandomUint64(), 16)
	pms := rpc.PostMetadataStatus{
		Version:    rpc.PostMetadataStatusVersion,
		From:       statusFrom.String(),
		Link:       pid.String(),
		Attributes: attr,
	}

	// Sign the status.
	pmHash := pms.Hash()
	signature := c.id.SignMessage(pmHash[:])
	attr[rpc.RMPSignature] = hex.EncodeToString(signature[:])

	// Set the timestamp for the status update
	attr[rpc.RMPTimestamp] = strconv.FormatInt(time.Now().Unix(), 16)

	// If it's a post from us, share the status update directly. Otherwise,
	// we'll send the status update to the post author.
	if postFrom == statusFrom {
		pms.Attributes[rpc.RMPFromNick] = c.LocalNick()
		return c.addStatusToPost(statusFrom, &pms)
	}

	// Ensure we know author.
	ru, err := c.rul.byID(postFrom)
	if err != nil {
		return err
	}

	// Store in case we go offline.
	payEvent := fmt.Sprintf("posts.%s.sendstatus", pid.ShortLogID())
	rm := rpc.RMPostStatus{
		Link:       pid.String(),
		Attributes: attr,
	}
	sqid, err := c.addToSendQ(payEvent, rm, priorityDefault, postFrom)
	if err != nil {
		return err
	}

	ru.log.Debugf("Sending post status update %x about post %s", pms.Hash(), pid)

	// Ask the author to share the status update.
	err = ru.sendRM(rm, payEvent)
	if err == nil {
		c.removeFromSendQ(sqid, postFrom)
	}
	return err
}

// HeartPost sends a status update, either adding or removing the current
// user's heart on the given received post.
func (c *Client) HeartPost(from clientintf.UserID, pid clientintf.PostID, heart bool) error {
	mode := rpc.RMPSHeartYes
	if !heart {
		mode = rpc.RMPSHeartNo
	}

	attr := map[string]string{
		rpc.RMPSHeart: mode,
	}
	return c.sendPostStatus(from, pid, attr)
}

// CommentPost sends a comment status update on the received post.
func (c *Client) CommentPost(postFrom clientintf.UserID, pid clientintf.PostID,
	comment string, parent *clientintf.ID) error {

	attr := map[string]string{
		rpc.RMPSComment: comment,
	}
	if parent != nil {
		attr[rpc.RMPParent] = parent.String()
	}
	return c.sendPostStatus(postFrom, pid, attr)
}

func (c *Client) handlePostStatus(ru *RemoteUser, rmps rpc.RMPostStatus) error {
	ru.log.Infof("Received status update on post %q", rmps.Link)

	// Create PMS and assert attributes are sane.
	var pms rpc.PostMetadataStatus
	var err error
	pms.Version, err = strconv.ParseUint(rmps.Attributes[rpc.RMPVersion], 10, 64)
	pms.From = ru.ID().String()
	pms.Link = rmps.Link
	pms.Attributes = rmps.Attributes
	pms.Attributes[rpc.RMPFromNick] = ru.Nick()
	if err == nil && rmps.Attributes[rpc.RMPStatusFrom] != ru.ID().String() {
		err = fmt.Errorf("unexpected statusfrom attr. want %q, got %q",
			ru.ID().String(), rmps.Attributes[rpc.RMPStatusFrom])
	}
	if err == nil && rmps.Attributes[rpc.RMPIdentifier] != rmps.Link {
		err = fmt.Errorf("unexpected identifier attr. want %q, got %q",
			rmps.Link, rmps.Attributes[rpc.RMPIdentifier])
	}
	if err == nil {
		err = c.verifyPostStatusSignature(pms)
	}
	if err == nil {
		err = c.addStatusToPost(ru.ID(), &pms)
	}

	// Send reply to status sender.
	var reply rpc.RMPostStatusReply
	if err != nil {
		errMsg := err.Error()
		reply.Error = &errMsg
	}
	go func() {
		payEvent := fmt.Sprintf("posts.%s.statusreply", rmps.Link[:16])
		err := ru.sendRM(reply, payEvent)
		if err != nil {
			ru.log.Errorf("Unable to send post status reply: %v", err)
		}
	}()
	return err
}

func (c *Client) handlePostStatusReply(ru *RemoteUser, reply rpc.RMPostStatusReply) error {
	if reply.Error != nil && *reply.Error != "" {
		return errors.New(*reply.Error)
	}
	return nil
}

// ListPostStatusUpdates lists the status updates of the specified post.
func (c *Client) ListPostStatusUpdates(from UserID, pid clientintf.PostID) ([]rpc.PostMetadataStatus, error) {
	var res []rpc.PostMetadataStatus
	err := c.dbView(func(tx clientdb.ReadTx) error {
		var err error
		res, err = c.db.ListPostStatusUpdates(tx, from, pid)
		return err
	})
	return res, err
}

// GetUserPost attempts to fetch the given post from the specified user. The
// post will be supplied in the PostReceived event of the client.
//
// The includeStatus flag dictates whether to also request the associated
// status updates (comments, etc).
func (c *Client) GetUserPost(from UserID, pid clientintf.PostID, includeStatus bool) error {
	ru, err := c.rul.byID(from)
	if err != nil {
		return err
	}

	// See if we're subscribed to the posts, otherwise this will fail.
	err = c.dbView(func(tx clientdb.ReadTx) error {
		subs, err := c.db.ListPostSubscriptions(tx)
		if err != nil {
			return err
		}
		isSubbed := false
		for _, sub := range subs {
			if sub.To == from {
				isSubbed = true
				break
			}
		}

		if !isSubbed {
			return fmt.Errorf("not subscribed to user posts")
		}
		return nil
	})
	if err != nil {
		return err
	}

	ru.log.Infof("Fetching post %s from user", pid)

	rm := rpc.RMGetPost{ID: pid, IncludeStatus: includeStatus}
	payEvent := fmt.Sprintf("posts.%s.get", pid.ShortLogID())
	return ru.sendRM(rm, payEvent)
}

// sendPostToUser sends the given post to the user.
func (c *Client) sendPostToUser(ru *RemoteUser, pid clientintf.PostID, post rpc.PostMetadata, updates []rpc.PostMetadataStatus) error {

	ru.log.Infof("Sending requested post %s (IncludeStatus=%v)", pid,
		updates != nil)
	payEvent := fmt.Sprintf("posts.%s.getreply", pid.ShortLogID())
	rm := rpc.RMPostShare(post)
	if err := c.sendWithSendQ(payEvent, rm, ru.ID()); err != nil {
		return err
	}
	if len(updates) > 0 {
		payEvent := fmt.Sprintf("posts.%s.getreplystatusupdate",
			pid.ShortLogID())
		for _, update := range updates {
			rm := rpc.RMPostShare{
				Version:    update.Version,
				Attributes: update.Attributes,
			}
			if err := c.sendWithSendQ(payEvent, rm, ru.ID()); err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *Client) handleGetPost(ru *RemoteUser, gp rpc.RMGetPost) error {
	// Check if user is subscriber.
	errNotSubscriber := errors.New("not a subscriber")
	var post rpc.PostMetadata
	var updates []rpc.PostMetadataStatus
	err := c.dbView(func(tx clientdb.ReadTx) error {
		isSub, err := c.db.IsPostSubscriber(tx, ru.ID())
		if err != nil {
			return err
		}
		if !isSub {
			return errNotSubscriber
		}

		if post, err = c.db.ReadPost(tx, c.PublicID(), gp.ID); err != nil {
			return err
		}
		if gp.IncludeStatus {
			if updates, err = c.db.ListPostStatusUpdates(tx, c.PublicID(), gp.ID); err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		if errors.Is(err, errNotSubscriber) {
			ru.log.Warnf("Attempted to fetch post %s while not a subscriber",
				gp.ID)
			return nil
		}
		if errors.Is(err, clientdb.ErrNotFound) {
			ru.log.Warnf("Attempted to fetch unknown post %s",
				gp.ID)
			return nil
		}
		return err
	}

	return c.sendPostToUser(ru, gp.ID, post, updates)
}

// relayPost relays the post to the specified users.
func (c *Client) relayPost(postFrom clientintf.UserID, pid clientintf.PostID,
	users ...clientintf.UserID) error {

	var post rpc.PostMetadata
	var updates []rpc.PostMetadataStatus
	err := c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		var err error
		post, err = c.db.ReadPost(tx, postFrom, pid)
		return err
	})
	if err != nil {
		return err
	}

	// Log the event.
	var from interface{}
	if from, err = c.rul.byID(postFrom); err != nil {
		from = postFrom
	}
	c.log.Infof("Relaying post %s from %s to %d subscribers",
		pid, len(updates), from, len(users))

	// Relay post.
	rm := rpc.RMPostShare(post)
	if err := c.shareWithPostSubscribers(users, pid, rm, "relaypost"); err != nil {
		return err
	}

	return nil
}

// RelayPost sends the given post to the specified user.
func (c *Client) RelayPost(postFrom clientintf.UserID, pid clientintf.PostID,
	toUser clientintf.UserID) error {

	errNotSubscriber := errors.New("destination not a subscriber")
	err := c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		// Ensure dest is subscriber, otherwise relaying will fail.
		if isSub, err := c.db.IsPostSubscriber(tx, toUser); err != nil {
			return err
		} else if !isSub {
			return errNotSubscriber
		}
		return nil
	})
	if err != nil {
		return err
	}

	return c.relayPost(postFrom, pid, toUser)
}

// RelayPostToSubscribers relays the specified post to all current post
// subscribers.
func (c *Client) RelayPostToSubscribers(postFrom clientintf.UserID, pid clientintf.PostID) error {
	var subs []clientintf.UserID
	err := c.dbView(func(tx clientdb.ReadTx) error {
		var err error
		subs, err = c.db.ListPostSubscribers(tx)
		return err
	})
	if err != nil {
		return err
	}

	return c.relayPost(postFrom, pid, subs...)
}
