package e2etests

import (
	"testing"
	"time"

	"github.com/companyzero/bisonrelay/client"
	"github.com/companyzero/bisonrelay/client/clientdb"
	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/companyzero/bisonrelay/internal/assert"
	"github.com/companyzero/bisonrelay/rpc"
	"github.com/companyzero/bisonrelay/zkidentity"
)

func TestBasicPostFeatures(t *testing.T) {
	t.Parallel()

	// Setup Alice and Bob and have them KX.
	tcfg := testScaffoldCfg{}
	ts := newTestScaffold(t, tcfg)
	alice := ts.newClient("alice")
	bob := ts.newClient("bob")
	charlie := ts.newClient("charlie")

	// Setup handlers.
	bobRecvPosts := make(chan rpc.PostMetadata, 1)
	bob.handle(client.OnPostRcvdNtfn(func(ru *client.RemoteUser, summary clientdb.PostSummary, pm rpc.PostMetadata) {
		bobRecvPosts <- pm
	}))
	bobRecvComments := make(chan string)
	bob.handle(client.OnPostStatusRcvdNtfn(func(user *client.RemoteUser, pid clientintf.PostID,
		statusFrom client.UserID, status rpc.PostMetadataStatus) {
		bobRecvComments <- status.Attributes[rpc.RMPSComment]
	}))
	bobSubChanged := make(chan bool, 3)
	bob.handle(client.OnRemoteSubscriptionChangedNtfn(func(user *client.RemoteUser, subscribed bool) {
		bobSubChanged <- subscribed
	}))

	charlieRecvPosts := make(chan rpc.PostMetadata, 1)
	charlie.handle(client.OnPostRcvdNtfn(func(ru *client.RemoteUser, summary clientdb.PostSummary, pm rpc.PostMetadata) {
		charlieRecvPosts <- pm
	}))
	charlieSubChanged := make(chan bool, 3)
	charlie.handle(client.OnRemoteSubscriptionChangedNtfn(func(user *client.RemoteUser, subscribed bool) {
		charlieSubChanged <- subscribed
	}))

	ts.kxUsers(alice, bob)
	ts.kxUsers(bob, charlie)

	// Alice creates a post before any subscriptions.
	_, err := alice.CreatePost("first", "")
	assert.NilErr(t, err)

	// Bob should _not_ get a notification.
	assert.ChanNotWritten(t, bobRecvPosts, 50*time.Millisecond)

	// Bob subscribes to Alice's posts.
	err = bob.SubscribeToPosts(alice.PublicID())
	assert.NilErr(t, err)
	assert.ChanWrittenWithVal(t, bobSubChanged, true)

	// Charlie subscribes to Bob's posts.
	err = charlie.SubscribeToPosts(bob.PublicID())
	assert.NilErr(t, err)
	assert.ChanWrittenWithVal(t, charlieSubChanged, true)

	// Alice creates a post that bob will receive.
	alicePost, err := alice.CreatePost("second", "")
	assert.NilErr(t, err)
	pm := assert.ChanWritten(t, bobRecvPosts)
	assert.DeepEqual(t, pm.Attributes[rpc.RMPMain], "second")

	// Alice writes a comment. Bob should receive it.
	wantComment := "alice comment"
	_, err = alice.CommentPost(alice.PublicID(), alicePost.ID, wantComment, nil)
	assert.NilErr(t, err)
	gotComment := assert.ChanWritten(t, bobRecvComments)
	assert.DeepEqual(t, gotComment, wantComment)

	// Bob writes a comment. Bob should receive after it's replicated by Alice.
	wantComment = "bob comment"
	bob.CommentPost(alice.PublicID(), alicePost.ID, wantComment, nil)
	assert.NilErr(t, err)
	gotComment = assert.ChanWritten(t, bobRecvComments)
	assert.DeepEqual(t, gotComment, wantComment)

	// Bob unsubscribes to Alice's posts.
	err = bob.UnsubscribeToPosts(alice.PublicID())
	assert.NilErr(t, err)
	assert.ChanWrittenWithVal(t, bobSubChanged, false)

	// Alice creates a post that Bob shouldn't get anymore.
	_, err = alice.CreatePost("third", "")
	assert.NilErr(t, err)
	assert.ChanNotWritten(t, bobRecvPosts, 50*time.Millisecond)

	// Charlie shouldn't have received any posts.
	assert.ChanNotWritten(t, charlieRecvPosts, 50*time.Millisecond)

	// Bob relays Alice's post to Charlie.
	bob.RelayPost(alice.PublicID(), alicePost.ID, charlie.PublicID())

	// Charlie should get the relayed post.
	pm = assert.ChanWritten(t, charlieRecvPosts)
	assert.DeepEqual(t, pm.Hash(), alicePost.ID)

	// Charlie attempts to comment on the relayed post. It doesn't work
	// because Charlie isn't KXd with Alice.
	wantComment = "charlie comment"
	_, err = charlie.CommentPost(bob.PublicID(), alicePost.ID, wantComment, nil)
	assert.ErrorIs(t, err, client.ErrKXSearchNeeded{})
}

// TestKXSearchFromPosts tests the KX search feature from posts.
//
// The test plan is the following: create a chain of 5 KXd users (A-E). Create
// a post and relay across the chain. The last user (Eve) attempts to search
// for the original post author (Alice).
func TestKXSearchFromPosts(t *testing.T) {
	t.Parallel()

	tcfg := testScaffoldCfg{}
	ts := newTestScaffold(t, tcfg)
	alice := ts.newClient("alice")
	bob := ts.newClient("bob")
	charlie := ts.newClient("charlie")
	dave := ts.newClient("dave")
	eve := ts.newClient("eve")

	ts.kxUsers(alice, bob)
	ts.kxUsers(bob, charlie)
	ts.kxUsers(charlie, dave)
	ts.kxUsers(dave, eve)

	assertSubscribeToPosts(t, alice, bob)
	assertSubscribeToPosts(t, bob, charlie)
	assertSubscribeToPosts(t, charlie, dave)
	assertSubscribeToPosts(t, dave, eve)

	// Alice creates a post and comments on it.
	bobRecvPostChan := make(chan clientintf.PostID, 1)
	bob.handle(client.OnPostRcvdNtfn(func(ru *client.RemoteUser, summ clientdb.PostSummary, _ rpc.PostMetadata) {
		bobRecvPostChan <- summ.ID
	}))

	alicePost, err := alice.CreatePost("alice's post", "")
	alicePostID := alicePost.ID
	assert.NilErr(t, err)
	aliceComment := "alice's comment"
	_, err = alice.CommentPost(alice.PublicID(), alicePostID, aliceComment, nil)
	assert.NilErr(t, err)
	assert.ChanWrittenWithVal(t, bobRecvPostChan, alicePostID)

	// Each client will relay the post to the next one.
	assertRelaysPost(t, bob, charlie, alice.PublicID(), alicePostID)
	assertRelaysPost(t, charlie, dave, bob.PublicID(), alicePostID)
	assertRelaysPost(t, dave, eve, charlie.PublicID(), alicePostID)

	// Setup to track relevant events.
	eveKXdChan := make(chan clientintf.UserID, 4)
	eve.handle(client.OnKXCompleted(func(_ *clientintf.RawRVID, ru *client.RemoteUser, _ bool) {
		eveKXdChan <- ru.ID()
	}))
	eveKXSearched := make(chan clientintf.UserID, 1)
	eve.handle(client.OnKXSearchCompleted(func(ru *client.RemoteUser) {
		eveKXSearched <- ru.ID()
	}))
	eveRcvdStatus := make(chan string, 1)
	eve.handle(client.OnPostStatusRcvdNtfn(func(_ *client.RemoteUser, _ clientintf.PostID, _ client.UserID,
		status rpc.PostMetadataStatus) {
		eveRcvdStatus <- status.Attributes[rpc.RMPSComment]
	}))

	// Eve will search for Alice (the post author).
	err = eve.KXSearchPostAuthor(dave.PublicID(), alicePostID)
	assert.NilErr(t, err)

	// Eve will KX with everyone up to Alice and receive the original
	// comment.
	assert.ChanWrittenWithVal(t, eveKXdChan, charlie.PublicID())
	assert.ChanWrittenWithVal(t, eveKXdChan, bob.PublicID())
	assert.ChanWrittenWithVal(t, eveKXdChan, alice.PublicID())
	assert.ChanWrittenWithVal(t, eveKXSearched, alice.PublicID())
	assert.ChanWrittenWithVal(t, eveRcvdStatus, aliceComment)

	// Bob comments on Alice's post. Eve should receive it.
	bobComment := "bob comment"
	_, err = bob.CommentPost(alice.PublicID(), alicePostID, bobComment, nil)
	assert.NilErr(t, err)
	assert.ChanWrittenWithVal(t, eveRcvdStatus, bobComment)
}

// TestPostReceiveReceipts tests that post and post status received receipts
// work.
func TestPostReceiveReceipts(t *testing.T) {
	t.Parallel()

	tcfg := testScaffoldCfg{}
	ts := newTestScaffold(t, tcfg)
	alice := ts.newClient("alice")
	bob := ts.newClient("bob", withSendRecvReceipts())
	charlie := ts.newClient("charlie", withSendRecvReceipts())
	eve := ts.newClient("eve")

	ts.kxUsers(alice, bob)
	ts.kxUsers(alice, charlie)
	ts.kxUsers(alice, eve)
	assertSubscribeToPosts(t, alice, bob)
	assertSubscribeToPosts(t, alice, charlie)
	assertSubscribeToPosts(t, alice, eve)

	// Setup handlers.
	rrFromBob := make(chan rpc.RMReceiveReceipt, 10)
	rrFromCharlie := make(chan rpc.RMReceiveReceipt, 10)
	rrFromEve := make(chan rpc.RMReceiveReceipt, 10)
	alice.handle(client.OnReceiveReceipt(func(ru *client.RemoteUser, rr rpc.RMReceiveReceipt, _ time.Time) {
		if ru.ID() == bob.PublicID() {
			rrFromBob <- rr
		} else if ru.ID() == charlie.PublicID() {
			rrFromCharlie <- rr
		} else if ru.ID() == eve.PublicID() {
			rrFromEve <- rr
		}
	}))

	assertRR := func(wantDomain rpc.RMReceiptDomain, wantID, wantSubID *zkidentity.ShortID, ch chan rpc.RMReceiveReceipt) {
		t.Helper()
		got := assert.ChanWritten(t, ch)
		assert.DeepEqual(t, got.Domain, wantDomain)
		assert.DeepEqual(t, got.ID, wantID)
		assert.DeepEqual(t, got.SubID, wantSubID)
	}

	// Alice will create a post. Bob and Charlie will ack it, Eve will NOT
	// ack it.
	post1 := assertReceivesNewPost(t, alice, bob, charlie, eve)
	assertRR(rpc.ReceiptDomainPosts, &post1, nil, rrFromBob)
	assertRR(rpc.ReceiptDomainPosts, &post1, nil, rrFromCharlie)
	assert.ChanNotWritten(t, rrFromEve, 500*time.Millisecond)

	// Bob will write a comment. Alice will receive and relay it. Bob, and
	// Charlie ack it, Eve does NOT.
	comment1 := assertCommentsOnPost(t, alice, bob, post1, bob, charlie, eve)
	assertRR(rpc.ReceiptDomainPostComments, &post1, &comment1, rrFromBob)
	assertRR(rpc.ReceiptDomainPostComments, &post1, &comment1, rrFromCharlie)
	assert.ChanNotWritten(t, rrFromEve, 500*time.Millisecond)

	// Second comment, from Eve.
	comment2 := assertCommentsOnPost(t, alice, eve, post1, bob, charlie, eve)
	assertRR(rpc.ReceiptDomainPostComments, &post1, &comment2, rrFromBob)
	assertRR(rpc.ReceiptDomainPostComments, &post1, &comment2, rrFromCharlie)
	assert.ChanNotWritten(t, rrFromEve, 500*time.Millisecond)

	// Third comment, from Alice.
	comment3 := assertCommentsOnPost(t, alice, alice, post1, bob, charlie, eve)
	assertRR(rpc.ReceiptDomainPostComments, &post1, &comment3, rrFromBob)
	assertRR(rpc.ReceiptDomainPostComments, &post1, &comment3, rrFromCharlie)
	assert.ChanNotWritten(t, rrFromEve, 500*time.Millisecond)
}

// TestAutoSubToPosts tests that when the autosubscribe to posts flag is set, it
// works.
func TestAutoSubToPosts(t *testing.T) {
	t.Parallel()

	tcfg := testScaffoldCfg{}
	ts := newTestScaffold(t, tcfg)
	alice := ts.newClient("alice")
	bob := ts.newClient("bob", withAutoSubToPosts())
	charlie := ts.newClient("charlie", withAutoSubToPosts())

	aliceSubChan := make(chan clientintf.UserID, 5)
	alice.handle(client.OnPostSubscriberUpdated(func(ru *client.RemoteUser, subscribed bool) {
		if subscribed {
			aliceSubChan <- ru.ID()
		}
	}))
	bobSubChan := make(chan clientintf.UserID, 5)
	bob.handle(client.OnPostSubscriberUpdated(func(ru *client.RemoteUser, subscribed bool) {
		if subscribed {
			bobSubChan <- ru.ID()
		}
	}))
	charlieSubChan := make(chan clientintf.UserID, 5)
	charlie.handle(client.OnPostSubscriberUpdated(func(ru *client.RemoteUser, subscribed bool) {
		if subscribed {
			charlieSubChan <- ru.ID()
		}
	}))

	// Invite based subscription.
	ts.kxUsers(alice, bob)
	assert.ChanWrittenWithVal(t, aliceSubChan, bob.PublicID())
	ts.kxUsers(alice, charlie)
	assert.ChanWrittenWithVal(t, aliceSubChan, charlie.PublicID())

	// GC-based subscription.
	gcID, err := alice.NewGroupChat("gc01")
	assert.NilErr(t, err)
	assertClientJoinsGC(t, gcID, alice, bob)

	assert.ChanNotWritten(t, aliceSubChan, 150*time.Millisecond)
	assert.ChanNotWritten(t, bobSubChan, 150*time.Millisecond)
	assert.ChanNotWritten(t, charlieSubChan, 150*time.Millisecond)

	assertClientJoinsGC(t, gcID, alice, charlie)
	assert.ChanWrittenWithVal(t, bobSubChan, charlie.PublicID())
	assert.ChanWrittenWithVal(t, charlieSubChan, bob.PublicID())
	assert.ChanNotWritten(t, aliceSubChan, 150*time.Millisecond)
}
