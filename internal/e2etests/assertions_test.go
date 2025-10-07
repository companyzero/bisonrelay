package e2etests

import (
	"context"
	cryptorand "crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"math/rand"
	"strconv"
	"testing"
	"time"

	"github.com/companyzero/bisonrelay/client"
	"github.com/companyzero/bisonrelay/client/clientdb"
	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/companyzero/bisonrelay/internal/assert"
	"github.com/companyzero/bisonrelay/rpc"
	rtdtclient "github.com/companyzero/bisonrelay/rtdt/client"
	"github.com/companyzero/bisonrelay/zkidentity"
)

func assertClientsKXd(t testing.TB, alice, bob *testClient) {
	t.Helper()
	var gotAlice, gotBob bool
	for i := 0; (!gotAlice || !gotBob) && i < 100; i++ {
		if alice.UserExists(bob.PublicID()) {
			gotAlice = true
		}
		if bob.UserExists(alice.PublicID()) {
			gotBob = true
		}
		time.Sleep(time.Millisecond * 100)
	}
	if !gotAlice || !gotBob {
		t.Fatalf("KX did not complete %v %v", gotAlice, gotBob)
	}
}

// assertJoinsGC asserts that the admin invites and the target accepts the
// invitation to join a GC.
func assertJoinsGC(t testing.TB, admin, target *testClient, gcID zkidentity.ShortID) {
	t.Helper()
	acceptChan := target.acceptNextGCInvite(gcID)
	assert.NilErr(t, admin.InviteToGroupChat(gcID, target.PublicID()))
	assert.NilErrFromChan(t, acceptChan)
}

// assertClientInGC asserts that `c` sees itself as a member of the GC.
func assertClientInGC(t testing.TB, c *testClient, gcID zkidentity.ShortID) {
	t.Helper()
	for i := 0; i < 100; i++ {
		_, err := c.GetGC(gcID)
		if err == nil {
			return
		}
		time.Sleep(time.Millisecond * 100)
	}
	t.Fatalf("Client did not join GC %s before timeout", gcID)
}

// assertClientSeesInGC asserts that `c` sees the target user as a member of the
// GC.
func assertClientSeesInGC(t testing.TB, c *testClient, gcID, target zkidentity.ShortID) {
	t.Helper()
	for i := 0; i < 100; i++ {
		gc, err := c.GetGC(gcID)
		if err != nil {
			continue
		}

		for _, uid := range gc.Members {
			if uid == target {
				return
			}
		}
		time.Sleep(time.Millisecond * 100)
	}
	t.Fatalf("Client does not see target %s as part of GC %s", target, gcID)
}

// assertClientUpToDate verifies the client has no pending updates to send
// to the server.
func assertClientUpToDate(t testing.TB, c *testClient) {
	t.Helper()
	var err error
	for i := 0; i < 200; i++ {
		err = nil
		if !c.RVsUpToDate() {
			err = fmt.Errorf("RVs are not up to date in the server")
		} else if q, s := c.RMQLen(); q+s != 0 {
			err = fmt.Errorf("RMQ is not empty")
		}
		if err != nil {
			time.Sleep(10 * time.Millisecond)
		}
	}
	if err != nil {
		t.Fatal(err)
	}
}

// assertClientHasNoRunningHandlers verifies the client does not have any
// running handlers.
func assertClientHasNoRunningHandlers(t testing.TB, c *testClient) {
	t.Helper()
	ti := c.testInterface()
	for i := 0; i < 200; i++ {
		if !ti.HasRunningUserHandlers() {
			return
		}
	}
	t.Fatalf("Client %s still has running handlers", c)
}

// assertClientsCanPM asserts that the clients can PM each other.
func assertClientsCanPM(t testing.TB, alice, bob *testClient) {
	t.Helper()
	aliceChan, bobChan := make(chan string, 1), make(chan string, 1)
	aliceReg := alice.NotificationManager().RegisterSync(client.OnPMNtfn(func(user *client.RemoteUser, msg rpc.RMPrivateMessage, ts time.Time) {
		aliceChan <- msg.Message
	}))
	bobReg := bob.NotificationManager().RegisterSync(client.OnPMNtfn(func(user *client.RemoteUser, msg rpc.RMPrivateMessage, ts time.Time) {
		bobChan <- msg.Message
	}))

	// Cleanup afterwards so we can do it multiple times.
	defer aliceReg.Unregister()
	defer bobReg.Unregister()

	aliceMsg, bobMsg := alice.name+"->"+bob.name, bob.name+"->"+alice.name
	alicePMSentChan, bobPMSentChan := make(chan error, 1), make(chan error, 1)
	go func() { alicePMSentChan <- alice.PM(bob.PublicID(), aliceMsg) }()
	go func() { bobPMSentChan <- bob.PM(alice.PublicID(), bobMsg) }()
	assert.NilErrFromChan(t, alicePMSentChan)
	assert.NilErrFromChan(t, bobPMSentChan)
	assert.ChanWrittenWithVal(t, aliceChan, bobMsg)
	assert.ChanWrittenWithVal(t, bobChan, aliceMsg)
}

// assertClientsCanPMOneWay asserts that Alice can send a message that is seen
// by Bob.
func assertClientsCanPMOneWay(t testing.TB, alice, bob *testClient) {
	t.Helper()
	bobChan := make(chan string, 1)
	bobReg := bob.NotificationManager().RegisterSync(client.OnPMNtfn(func(user *client.RemoteUser, msg rpc.RMPrivateMessage, ts time.Time) {
		bobChan <- msg.Message
	}))

	// Cleanup afterwards so we can do it multiple times.
	defer bobReg.Unregister()

	aliceMsg := alice.name + "->" + bob.name
	assert.NilErr(t, alice.PM(bob.PublicID(), aliceMsg))
	assert.ChanWrittenWithVal(t, bobChan, aliceMsg)
}

// assertClientsCannotPMOneWay asserts that a message sent from Alice is NOT
// seen by Bob.
func assertClientsCannotPMOneWay(t testing.TB, alice, bob *testClient) {
	t.Helper()
	bobChan := make(chan string, 1)
	bobReg := bob.NotificationManager().RegisterSync(client.OnPMNtfn(func(user *client.RemoteUser, msg rpc.RMPrivateMessage, ts time.Time) {
		bobChan <- msg.Message
	}))

	// Cleanup afterwards so we can do it multiple times.
	defer bobReg.Unregister()

	aliceMsg := alice.name + "->" + bob.name
	assert.NilErr(t, alice.PM(bob.PublicID(), aliceMsg))
	assert.ChanNotWritten(t, bobChan, 100*time.Millisecond)
}

// assertClientJoinsGC asserts that the admin of the GC can invite and the
// invitee joins the GC.
func assertClientJoinsGC(t testing.TB, gcID zkidentity.ShortID, admin, invitee *testClient) {
	invitee.acceptNextGCInvite(gcID)
	assert.NilErr(t, admin.InviteToGroupChat(gcID, invitee.PublicID()))
	assertClientInGC(t, invitee, gcID)
}

// assertClientsCanGCM asserts that the clients can send GC messages to each
// other inside a GC.
func assertClientsCanGCM(t testing.TB, gcID zkidentity.ShortID, clients ...*testClient) {
	t.Helper()
	regs := make([]client.NotificationRegistration, len(clients))
	chans := make([]chan string, len(clients))

	// Setup all handlers.
	for i := range clients {
		i := i
		chans[i] = make(chan string, 1)
		regs[i] = clients[i].handle(client.OnGCMNtfn(func(_ *client.RemoteUser, msg rpc.RMGroupMessage, _ time.Time) {
			chans[i] <- msg.Message
		}))
	}

	// Send one message from each client and ensure the other ones receive
	// it.
	for i := range clients {
		wantMsg := fmt.Sprintf("msg from %d - %s", i, clients[i].name)
		err := clients[i].GCMessage(gcID, wantMsg, 0, nil)
		assert.NilErr(t, err)
		for j := range clients {
			if i == j {
				continue
			}

			assert.ChanWrittenWithVal(t, chans[j], wantMsg)
		}
	}

	// Teardown the handlers.
	for i := range clients {
		regs[i].Unregister()
	}
}

// assertClientsCanSeeGCM asserts that one client sends and all the other clients
// receive a GCM.
func assertClientsCanSeeGCM(t testing.TB, gcID zkidentity.ShortID, src *testClient, targets ...*testClient) {
	t.Helper()

	regs := make([]client.NotificationRegistration, len(targets))
	chans := make([]chan string, len(targets))

	// Setup all handlers.
	for i := range targets {
		i := i
		chans[i] = make(chan string, 1)
		regs[i] = targets[i].handle(client.OnGCMNtfn(func(_ *client.RemoteUser, msg rpc.RMGroupMessage, _ time.Time) {
			chans[i] <- msg.Message
		}))
	}

	// Send one message from src and see it on all targets.
	wantMsg := fmt.Sprintf("msg from %s", src.name)
	err := src.GCMessage(gcID, wantMsg, 0, nil)
	assert.NilErr(t, err)
	for i := range targets {
		assert.ChanWrittenWithVal(t, chans[i], wantMsg)
	}

	// Teardown the handlers.
	for i := range targets {
		regs[i].Unregister()
	}
}

// assertClientCannotSeeGCM asserts that a GCM send by the source client is not
// received by the target client.
func assertClientCannotSeeGCM(t testing.TB, gcID zkidentity.ShortID, src, target *testClient) {
	c := make(chan string, 1)
	reg := target.handle(client.OnGCMNtfn(func(_ *client.RemoteUser, msg rpc.RMGroupMessage, _ time.Time) {
		c <- msg.Message
	}))

	msg := fmt.Sprintf("msg from %s not seen by %s", src.name, target.name)
	err := src.GCMessage(gcID, msg, 0, nil)
	assert.NilErr(t, err)
	assert.ChanNotWritten(t, c, time.Millisecond*500)
	reg.Unregister()
}

// assertIsGCOwner asserts that the client c sees owner as the owner of a GC.
func assertIsGCOwner(t testing.TB, gcID zkidentity.ShortID, c, owner *testClient) {
	t.Helper()
	gc, err := c.GetGC(gcID)
	assert.NilErr(t, err)
	if gc.Members[0] != owner.PublicID() {
		t.Fatalf("unexpected gc owner: got %s, want %s", gc.Members[0],
			owner.PublicID())
	}
}

// assertGCDoesNotExist asserts that the client c does not have the gcID as one
// of its GCs.
func assertGCDoesNotExist(t testing.TB, gcID zkidentity.ShortID, c *testClient) {
	t.Helper()
	gcs, err := c.ListGCs()
	assert.NilErr(t, err)
	for _, gc := range gcs {
		if gc.Metadata.ID == gcID {
			t.Fatalf("client %s has GC %s in list, when it should not",
				c.name, gcID)
		}
	}
}

// assertSubscribeToPosts attempts to make subscriber subscribe to target's
// posts.
func assertSubscribeToPosts(t testing.TB, target, subscriber *testClient) {
	t.Helper()
	subChan := make(chan error, 1)
	reg := subscriber.handle(client.OnRemoteSubscriptionChangedNtfn(func(ru *client.RemoteUser, subscribed bool) {
		if ru.ID() == target.PublicID() && subscribed {
			subChan <- nil
		}
	}))
	err := subscriber.SubscribeToPosts(target.PublicID())
	assert.NilErr(t, err)
	assert.NilErrFromChan(t, subChan)
	reg.Unregister()
}

// assertReceivesNewPost creates a new post in poster and asserts all passed
// subs receive a notification that the post was done. Returns the new post id.
//
// Any clients specified in excluded shoult NOT get the new post.
func assertReceivesNewPost(t testing.TB, poster *testClient, excluded []*testClient, targets ...*testClient) clientintf.PostID {
	t.Helper()

	regs := make([]client.NotificationRegistration, len(targets))
	chans := make([]chan struct{}, len(targets))
	regsExcluded := make([]client.NotificationRegistration, len(excluded))
	chansExcluded := make([]chan struct{}, len(excluded))

	postData := "test post **** " + strconv.FormatInt(rand.Int63(), 10)

	// Setup all handlers.
	for i := range targets {
		i := i
		chans[i] = make(chan struct{})
		regs[i] = targets[i].handle(client.OnPostRcvdNtfn(func(ru *client.RemoteUser, sum clientdb.PostSummary, _ rpc.PostMetadata) {
			if sum.Title == postData {
				close(chans[i])
			}
		}))
	}
	for i := range excluded {
		i := i
		chansExcluded[i] = make(chan struct{})
		regsExcluded[i] = excluded[i].handle(client.OnPostRcvdNtfn(func(ru *client.RemoteUser, sum clientdb.PostSummary, _ rpc.PostMetadata) {
			if sum.Title == postData {
				close(chansExcluded[i])
			}
		}))
	}

	// Create the post.
	post, err := poster.CreatePost(postData, "")
	assert.NilErr(t, err)

	// Check targets that should receive the post.
	for i := range targets {
		select {
		case <-chans[i]:
		case <-time.After(5 * time.Second):
			t.Fatalf("target %d (%s) did not receive post", i, targets[i].LocalNick())
		}
	}

	// Check targets that should NOT receive the post.
	assert.NoChansWritten(t, chansExcluded, time.Second)

	// Teardown the handlers.
	for i := range targets {
		regs[i].Unregister()
	}
	for i := range excluded {
		regsExcluded[i].Unregister()
	}

	return post.ID
}

// assertCommentsOnPost asserts that a commenter comments on a post, that the
// original post relayer receives that comment and that any passed subscribers
// get that status update. Returns the comment ID.
func assertCommentsOnPost(t testing.TB, relayer, commenter *testClient, pid clientintf.PostID, subs ...*testClient) clientintf.ID {
	t.Helper()

	commentReceived := make(chan clientintf.ID, 1)
	reg := relayer.handle(client.OnPostStatusRcvdNtfn(func(user *client.RemoteUser, pid clientintf.PostID,
		statusFrom client.UserID, status rpc.PostMetadataStatus) {
		commentReceived <- status.Hash()
	}))

	regs := make([]client.NotificationRegistration, len(subs))
	subChans := make([]chan clientintf.ID, len(subs))
	for i, sub := range subs {
		i := i
		subChans[i] = make(chan clientintf.ID, 1)
		regs[i] = sub.handle(client.OnPostStatusRcvdNtfn(func(user *client.RemoteUser, pid clientintf.PostID,
			statusFrom client.UserID, status rpc.PostMetadataStatus) {
			subChans[i] <- status.Hash()
		}))
	}

	commentID, err := commenter.CommentPost(relayer.PublicID(), pid, "test comment", nil)
	assert.NilErr(t, err)
	assert.ChanWrittenWithVal(t, commentReceived, commentID)
	for i := range subChans {
		assert.ChanWrittenWithVal(t, subChans[i], commentID)
		regs[i].Unregister()
	}

	reg.Unregister()
	return commentID
}

// assertRelaysPost attempts to relay a post from src to dst and verify that it
// was received in dst.
func assertRelaysPost(t testing.TB, src, dst *testClient, postFrom clientintf.UserID, pid clientintf.PostID) {
	t.Helper()
	relayChan := make(chan error, 1)
	reg := dst.handle(client.OnPostRcvdNtfn(func(ru *client.RemoteUser, summ clientdb.PostSummary, _ rpc.PostMetadata) {
		if ru.ID() == src.PublicID() && summ.ID == pid {
			relayChan <- nil
		}
	}))
	err := src.RelayPost(postFrom, pid, dst.PublicID())
	assert.NilErr(t, err)
	assert.NilErrFromChan(t, relayChan)
	reg.Unregister()
}

// assertRMQLenIs asserts the RMQ has the given length.
func assertRMQLenIs(t testing.TB, c *testClient, wantLen int) {
	// Wait until the queues are empty.
	t.Helper()
	maxCheck := 12000
	for i := 0; i < maxCheck; i++ {
		qlen, acklen := c.RMQLen()
		if qlen+acklen == wantLen {
			break
		}
		time.Sleep(10 * time.Millisecond)
		if i == maxCheck-1 {
			t.Fatalf("timeout waiting for wanted queue len %d (got %d + %d)",
				wantLen, qlen, acklen)
		}
	}
}

// assertEmptyRMQ asserts the RMQ of the passed client is or becomes empty.
func assertEmptyRMQ(t testing.TB, c *testClient) {
	t.Helper()
	assertRMQLenIs(t, c, 0)
}

// assertSendqDestsIs asserts the total number of messages in the sendq has the
// given length.
func assertSendqDestsIs(t testing.TB, c *testClient, wantLen int) {
	// Wait until the queues are empty.
	t.Helper()
	maxCheck := 12000
	for i := 0; i < maxCheck; i++ {
		_, dests := c.SendQueueLen()
		if dests == wantLen {
			break
		}
		time.Sleep(10 * time.Millisecond)
		if i == maxCheck-1 {
			t.Fatalf("timeout waiting for wanted sendqqueue dests %d (got %d)",
				wantLen, dests)
		}
	}
}

// assertGoesOffline verifies that the client switches to offline status.
func assertGoesOffline(t testing.TB, c *testClient) {
	t.Helper()
	ch := make(chan bool, 10)
	reg := c.handle(client.OnServerSessionChangedNtfn(func(connected bool, _ clientintf.ServerPolicy) {
		ch <- connected
	}))
	c.RemainOffline()
	assert.ChanWrittenWithVal(t, ch, false)
	reg.Unregister()
}

// assertGoesOnline verifies that the client switches to onlne status.
func assertGoesOnline(t testing.TB, c *testClient) {
	t.Helper()
	ch := make(chan bool, 10)
	reg := c.handle(client.OnServerSessionChangedNtfn(func(connected bool, _ clientintf.ServerPolicy) {
		ch <- connected
	}))
	c.GoOnline()
	assert.ChanWrittenWithVal(t, ch, true)
	reg.Unregister()
}

// assertUserNick verifies that the client 'c' sees the nick of 'target' as
// 'nick'.
func assertUserNick(t testing.TB, c, target *testClient, nick string) {
	t.Helper()

	// Assert UserNick() returns the correct nick.
	gotNick, err := c.UserNick(target.PublicID())
	assert.NilErr(t, err)
	assert.DeepEqual(t, gotNick, nick)

	// Assert UserByNick() finds the correct user.
	ru, err := c.UserByNick(nick)
	assert.NilErr(t, err)
	assert.DeepEqual(t, ru.ID(), target.PublicID())

	// Assert ru.Nick() returns the correct nick.
	assert.DeepEqual(t, ru.Nick(), nick)
}

// assertKXReset verifies that client c can perform a ratchet reset with target.
func assertKXReset(t testing.TB, c, target *testClient) {
	t.Helper()
	targetKXChan := make(chan struct{}, 5)
	regTarget := target.handle(client.OnKXCompleted(func(_ *clientintf.RawRVID, ru *client.RemoteUser, isNew bool) {
		if ru.ID() == c.PublicID() && !isNew {
			targetKXChan <- struct{}{}
		}
	}))
	defer regTarget.Unregister()

	cKXChan := make(chan struct{}, 5)
	regC := c.handle(client.OnKXCompleted(func(_ *clientintf.RawRVID, ru *client.RemoteUser, isNew bool) {
		if ru.ID() == target.PublicID() && !isNew {
			cKXChan <- struct{}{}
		}
	}))
	defer regC.Unregister()

	assert.NilErr(t, c.ResetRatchet(target.PublicID()))
	assert.ChanWritten(t, targetKXChan)
	assert.ChanWritten(t, cKXChan)
}

// assertUserAvatar asserts that user c sees the avatar for the target user
// as want.
func assertUserAvatar(t testing.TB, c, target *testClient, want []byte) {
	t.Helper()
	ab, err := c.AddressBookEntry(target.PublicID())
	assert.NilErr(t, err)
	assert.DeepEqual(t, ab.ID.Avatar, want)
}

// assertPostSubscriber asserts whether client c contains the target user in
// its list of subscribers (target is/isn't subscribed to c's posts).
func assertPostSubscriber(t testing.TB, c, target *testClient, assertIsSubscribed bool) {
	t.Helper()
	subs, err := c.ListPostSubscribers()
	assert.NilErr(t, err)
	for _, sub := range subs {
		if sub != target.PublicID() {
			continue
		}
		if assertIsSubscribed {
			// Is subscribed and asserting it is subscribed.
			return
		}
		// Is subscribed and asserting is not subscribed.
		t.Fatalf("Target client %s is subscribed to %s posts", target, c)
	}
	if assertIsSubscribed {
		t.Fatalf("Target client %s is not subscribed to %s posts", target, c)
	}
}

// assertPostSubscription asserts whether client c is/isn't subscribed to the
// target user's posts. This is the reverse of assertContainsPostSubscriber.
func assertPostSubscription(t testing.TB, c, target *testClient, assertIsSubscribed bool) {
	t.Helper()
	subs, err := c.ListPostSubscriptions()
	assert.NilErr(t, err)
	for _, sub := range subs {
		if sub.To != target.PublicID() {
			continue
		}
		if assertIsSubscribed {
			// Is subscribed and asserting it is subscribed.
			return
		}

		// Is subscribed and asserting is not subscribed.
		t.Fatalf("Client %s is subscribed to %s posts", c, target)
	}
	if assertIsSubscribed {
		t.Fatalf("Client %s is not subscribed to %s posts", c, target)
	}
}

// assertJoinsRTDTSession asserts a client asks a target client to join an RTDT
// session and they accept.
func assertJoinsRTDTSession(t testing.TB, c, target *testClient, sessRV zkidentity.ShortID) {
	t.Helper()
	inviteChan := make(chan *rpc.RMRTDTSessionInvite, 2)
	invitedHandler := target.handle(client.OnInvitedToRTDTSession(func(ru *client.RemoteUser, invite *rpc.RMRTDTSessionInvite, ts time.Time) {
		inviteChan <- invite
	}))

	updateChan := make(chan *client.RTDTSessionUpdateNtfn, 5)
	updateHandler := target.handle(client.OnRTDTSesssionUpdated(func(ru *client.RemoteUser, update *client.RTDTSessionUpdateNtfn) {
		updateChan <- update
	}))

	// C invites Target to session.
	err := c.InviteToRTDTSession(sessRV, true, target.PublicID())
	if err != nil {
		t.Fatalf("Inviting to RTDT session failed: %v", err)
	}

	// Target will receive and accept the invitation.
	gotInvite := assert.ChanWritten(t, inviteChan)
	err = target.AcceptRTDTSessionInvite(c.PublicID(), gotInvite, true)
	assert.NilErr(t, err)
	_ = assert.ChanWritten(t, updateChan)

	invitedHandler.Unregister()
	updateHandler.Unregister()
}

// assertJoinsLiveRTDTSession asserts the client can join the specified live
// session.
func assertJoinsLiveRTDTSession(t testing.TB, c *testClient, sessRV zkidentity.ShortID) *rtdtclient.Session {
	t.Helper()
	joinedChan := make(chan struct{}, 5)
	handler := c.handle(client.OnRTDTLiveSessionJoined(func(joinedSessRV zkidentity.ShortID) {
		if joinedSessRV == sessRV {
			joinedChan <- struct{}{}
		}
	}))

	assert.NilErr(t, c.JoinLiveRTDTSession(sessRV))
	assert.ChanWritten(t, joinedChan)
	rtSess := c.GetLiveRTSession(&sessRV).RTSess
	handler.Unregister()
	return rtSess
}

// assertSendsRandomRTDTData asserts that src sends and the targets receive
// random data in an RTDT session.
func assertSendsRandomRTDTData(t testing.TB, src *rtdtclient.Session, dests ...chan []byte) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	data := randomData(100)
	err := src.SendRandomData(ctx, data, 1000)
	assert.NilErr(t, err)

	for _, dst := range dests {
		gotData := assert.ChanWritten(t, dst)
		assert.DeepEqual(t, gotData, data)
	}
}

// assertKicksFromLiveRTDTSession asserts src can kick target from a live RTDT
// session.
func assertKicksFromLiveRTDTSession(t testing.TB, src *testClient,
	target *testClient, sessRV *zkidentity.ShortID, banDuration time.Duration) {
	t.Helper()

	kickedChan := make(chan struct{}, 5)
	handler := target.handle(client.OnRTDTKickedFromLiveSession(func(kickedSessRV zkidentity.ShortID,
		peerID rpc.RTDTPeerID, kickedBanDuration time.Duration) {
		if *sessRV == kickedSessRV && kickedBanDuration == banDuration {
			kickedChan <- struct{}{}
		}
	}))

	// Src kicks target.
	targetSess := target.GetLiveRTSession(sessRV)
	if targetSess == nil {
		t.Fatalf("Target does not have live session")
		return
	}
	targetPeerID := targetSess.RTSess.LocalID()
	assert.NilErr(t, src.KickFromLiveRTDTSession(sessRV, targetPeerID, banDuration))

	// Target gets notification.
	assert.ChanWritten(t, kickedChan)
	handler.Unregister()
}

func testRand(t testing.TB) *rand.Rand {
	seed := time.Now().UnixNano()
	rnd := rand.New(rand.NewSource(seed))
	t.Cleanup(func() {
		if t.Failed() {
			t.Logf("Seed: %d", seed)
		}
	})

	return rnd
}

func randomHex(rnd io.Reader, len int) string {
	b := make([]byte, len)
	_, err := rnd.Read(b)
	if err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

func randomData(size int) []byte {
	data := make([]byte, size)
	cryptorand.Read(data)
	return data
}

func publicIDIsLess(c1, c2 *testClient) bool {
	c1pub := c1.PublicID()
	c2pub := c2.PublicID()
	return c1pub.Less(&c2pub)
}

func mustNewIDFromRNG(nick string, rng *rand.Rand) *zkidentity.FullIdentity {
	id, err := zkidentity.NewWithRNG(nick, nick, rng)
	if err != nil {
		panic(err)
	}
	return id
}
