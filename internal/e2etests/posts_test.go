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
	tcfg := testScaffoldCfg{showLog: true}
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

	// Bob gets a new post due to relay event.
	assert.ChanWritten(t, bobRecvPosts)

	// Charlie should get the relayed post.
	pm = assert.ChanWritten(t, charlieRecvPosts)
	assert.DeepEqual(t, pm.Hash(), alicePost.ID)

	// Charlie comments on the relayed post. Bob should get it.
	wantComment = "charlie comment"
	err = charlie.CommentPost(bob.PublicID(), alicePost.ID, wantComment, nil)
	assert.NilErr(t, err)
	gotComment = assert.ChanWritten(t, bobRecvComments)
	assert.DeepEqual(t, gotComment, wantComment)
}
