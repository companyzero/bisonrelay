package e2etests

import (
	"testing"
	"time"

	"github.com/companyzero/bisonrelay/client"
	"github.com/companyzero/bisonrelay/client/clientdb"
	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/companyzero/bisonrelay/internal/assert"
	"github.com/companyzero/bisonrelay/rpc"
)

func TestBasicPostFeatures(t *testing.T) {
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
	err = alice.CommentPost(alice.PublicID(), alicePost.ID, wantComment, nil)
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
	err = charlie.CommentPost(bob.PublicID(), alicePost.ID, wantComment, nil)
	assert.ErrorIs(t, err, client.ErrKXSearchNeeded{})
}

// TestKXSearchFromPosts tests the KX search feature from posts.
//
// The test plan is the following: create a chain of 5 KXd users (A-E). Create
// a post and relay across the chain. The last user (Eve) attempts to search
// for the original post author (Alice).
func TestKXSearchFromPosts(t *testing.T) {
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
	err = alice.CommentPost(alice.PublicID(), alicePostID, aliceComment, nil)
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
	assert.NilErr(t, bob.CommentPost(alice.PublicID(), alicePostID, bobComment, nil))
	assert.ChanWrittenWithVal(t, eveRcvdStatus, bobComment)
}
