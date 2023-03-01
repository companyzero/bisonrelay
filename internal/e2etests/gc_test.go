package e2etests

import (
	"math/rand"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/companyzero/bisonrelay/client"
	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/companyzero/bisonrelay/internal/assert"
	"github.com/companyzero/bisonrelay/rpc"
)

// TestBasicGCFeatures performs tests for the basic GC features.
func TestBasicGCFeatures(t *testing.T) {
	// Setup Alice and Bob.
	tcfg := testScaffoldCfg{}
	ts := newTestScaffold(t, tcfg)
	alice := ts.newClient("alice")
	bob := ts.newClient("bob")

	ts.kxUsers(alice, bob)

	aliceAcceptedInvitesChan := make(chan clientintf.UserID, 1)
	alice.handle(client.OnGCInviteAcceptedNtfn(func(user *client.RemoteUser, _ rpc.RMGroupList) {
		aliceAcceptedInvitesChan <- user.ID()
	}))
	bobJoinedGCChan := make(chan struct{}, 1)
	bob.handle(client.OnJoinedGCNtfn(func(gc rpc.RMGroupList) {
		bobJoinedGCChan <- struct{}{}
	}))
	bobUsersAddedChan := make(chan []clientintf.UserID, 2)
	bob.handle(client.OnAddedGCMembersNtfn(func(gc rpc.RMGroupList, uids []clientintf.UserID) {
		bobUsersAddedChan <- uids
	}))

	// Alice creates a GC.
	gcName := "testGC"
	gcID, err := alice.NewGroupChat(gcName)
	assert.NilErr(t, err)

	// Setup Bob to accept Alice's invite and send the invite.
	bobAcceptedChan := bob.acceptNextGCInvite(gcID)
	assert.NilErr(t, alice.InviteToGroupChat(gcID, bob.PublicID()))

	// Bob correctly accepted to join and Alice was notified Bob joined.
	assert.NilErrFromChan(t, bobAcceptedChan)
	assert.ChanWrittenWithVal(t, aliceAcceptedInvitesChan, bob.PublicID())
	assert.ChanWritten(t, bobJoinedGCChan)

	// Double check bob is in the GC.
	assertClientInGC(t, bob, gcID)

	// Send concurrent gc msgs between Alice and Bob.
	rnd := testRand(t)
	nbMsgs := 1
	maxMsgSize := 1024
	arnd := rand.New(rand.NewSource(rnd.Int63()))
	brnd := rand.New(rand.NewSource(rnd.Int63()))
	aliceSentMsgs := make([]string, nbMsgs)
	bobSentMsgs := make([]string, nbMsgs)
	aliceReceivedMsgs := make(map[string]struct{}, nbMsgs)
	bobReceivedMsgs := make(map[string]struct{}, nbMsgs)
	var aliceMtx, bobMtx sync.Mutex
	aliceDoneMsgs := make(chan struct{})
	bobDoneMsgs := make(chan struct{})
	alice.NotificationManager().RegisterSync(client.OnGCMNtfn(func(ru *client.RemoteUser, msg rpc.RMGroupMessage, ts time.Time) {
		aliceMtx.Lock()
		aliceReceivedMsgs[msg.Message] = struct{}{}
		if len(aliceReceivedMsgs) == nbMsgs {
			close(aliceDoneMsgs)
		}
		aliceMtx.Unlock()
	}))
	bob.NotificationManager().RegisterSync(client.OnGCMNtfn(func(ru *client.RemoteUser, msg rpc.RMGroupMessage, ts time.Time) {
		bobMtx.Lock()
		bobReceivedMsgs[msg.Message] = struct{}{}
		if len(bobReceivedMsgs) == nbMsgs {
			close(bobDoneMsgs)
		}
		bobMtx.Unlock()
	}))

	sendErrChan := make(chan error)
	go func() {
		for i := 0; i < nbMsgs; i++ {
			aliceSentMsgs[i] = strconv.Itoa(i) + randomHex(arnd, 1+arnd.Intn(maxMsgSize))
			err := alice.GCMessage(gcID, aliceSentMsgs[i], rpc.MessageModeNormal, nil)
			if err != nil {
				sendErrChan <- err
			}
			time.Sleep(time.Duration(100000+arnd.Intn(100000)) * time.Microsecond)
		}
	}()
	go func() {
		for i := 0; i < nbMsgs; i++ {
			bobSentMsgs[i] = strconv.Itoa(i) + randomHex(brnd, 1+brnd.Intn(maxMsgSize))
			err := bob.GCMessage(gcID, bobSentMsgs[i], rpc.MessageModeNormal, nil)
			if err != nil {
				sendErrChan <- err
			}
			time.Sleep(time.Duration(100000+brnd.Intn(100000)) * time.Microsecond)
		}
	}()

	// Ensure all messages were received.
	for aliceDoneMsgs != nil || bobDoneMsgs != nil {
		select {
		case <-aliceDoneMsgs:
			aliceDoneMsgs = nil
		case <-bobDoneMsgs:
			bobDoneMsgs = nil
		case err := <-sendErrChan:
			t.Fatal(err)
		case <-time.After(60 * time.Second):
			t.Fatal("timeout")
		}
	}

	// Ensure all msgs are in order. Alice receives Bob's messages (and
	// vice-versa).
	for i := 0; i < nbMsgs; i++ {
		if _, ok := aliceReceivedMsgs[bobSentMsgs[i]]; !ok {
			t.Fatalf("did not find alice msg %d", i)
		}
		if _, ok := bobReceivedMsgs[aliceSentMsgs[i]]; !ok {
			t.Fatalf("did not find bob msg %d", i)
		}
	}

	// Create Charlie. Create a GC with the same name as the one from
	// Alice. KX with alice and join gc. Autokx should start with Bob.
	charlie := ts.newClient("charlie")
	charlieJoinedGCChan := make(chan struct{}, 1)
	charlie.handle(client.OnJoinedGCNtfn(func(gc rpc.RMGroupList) {
		charlieJoinedGCChan <- struct{}{}
	}))
	_, err = charlie.NewGroupChat(gcName)
	assert.NilErr(t, err)
	ts.kxUsers(alice, charlie)
	charlieAcceptedChan := charlie.acceptNextGCInvite(gcID)
	assert.NilErr(t, alice.InviteToGroupChat(gcID, charlie.PublicID()))
	assert.NilErrFromChan(t, charlieAcceptedChan)
	assert.ChanWrittenWithVal(t, aliceAcceptedInvitesChan, charlie.PublicID())
	assert.ChanWritten(t, charlieJoinedGCChan)

	// Bob should receive a list update containing charlie.
	bobNewUsers := assert.ChanWritten(t, bobUsersAddedChan)
	assert.DeepEqual(t, len(bobNewUsers), 1)
	assert.DeepEqual(t, bobNewUsers[0], charlie.PublicID())

	// The alias of Alice's GC in charlie should be different than the
	// original one, due to Charlie already having a GC with the same name.
	aliasInCharlie, err := charlie.GetGCAlias(gcID)
	assert.NilErr(t, err)
	if aliasInCharlie == gcName {
		t.Fatalf("Unexpected gc alias in charlie: got %s", aliasInCharlie)
	}

	// Modifying the alias should work.
	newAlias := "bleh"
	err = charlie.AliasGC(gcID, newAlias)
	assert.NilErr(t, err)
	aliasInCharlie, err = charlie.GetGCAlias(gcID)
	assert.NilErr(t, err)
	if aliasInCharlie != newAlias {
		t.Fatalf("Unexpected gc alias in charlie: got %s, want %s",
			aliasInCharlie, newAlias)
	}

	// Bob and Charlie should haved kxd already and should be able to PM
	// each other.
	assertClientsKXd(t, bob, charlie)
	assertClientsCanPM(t, bob, charlie)
}

// TestGCKicks tests scenarios where users are leaving or being kicked from GC.
func TestGCKicks(t *testing.T) {
	tcfg := testScaffoldCfg{}
	ts := newTestScaffold(t, tcfg)
	alice := ts.newClient("alice")
	bob := ts.newClient("bob")
	charlie := ts.newClient("charlie")
	dave := ts.newClient("dave")

	// Alice will be the admin and charlie will be an user we'll use to
	// check the events.
	aliceAcceptedInvitesChan := make(chan clientintf.UserID, 1)
	alice.handle(client.OnGCInviteAcceptedNtfn(func(user *client.RemoteUser, _ rpc.RMGroupList) {
		aliceAcceptedInvitesChan <- user.ID()
	}))

	type partEvent struct {
		uid    clientintf.UserID
		reason string
		kicked bool
	}
	charliePartChan := make(chan partEvent)
	charlie.handle(client.OnGCUserPartedNtfn(func(gcid client.GCID, uid client.UserID, reason string, kicked bool) {
		charliePartChan <- partEvent{uid: uid, reason: reason, kicked: kicked}
	}))
	charlieKilledChan := make(chan struct{})
	charlie.handle(client.OnGCKilledNtfn(func(gcid client.GCID, reason string) {
		charlieKilledChan <- struct{}{}
	}))

	ts.kxUsers(alice, bob)
	ts.kxUsers(alice, charlie)
	ts.kxUsers(alice, dave)

	gcName := "test_gc"
	gcID, err := alice.NewGroupChat(gcName)
	assert.NilErr(t, err)

	// Invite everyone.
	bobAcceptedChan := bob.acceptNextGCInvite(gcID)
	charlieAcceptedChan := charlie.acceptNextGCInvite(gcID)
	daveAcceptedChan := dave.acceptNextGCInvite(gcID)
	assert.NilErr(t, alice.InviteToGroupChat(gcID, bob.PublicID()))
	assert.NilErr(t, alice.InviteToGroupChat(gcID, charlie.PublicID()))
	assert.NilErr(t, alice.InviteToGroupChat(gcID, dave.PublicID()))
	assert.NilErrFromChan(t, bobAcceptedChan)
	assert.NilErrFromChan(t, charlieAcceptedChan)
	assert.NilErrFromChan(t, daveAcceptedChan)
	for i := 0; i < 3; i++ {
		assert.ChanWritten(t, aliceAcceptedInvitesChan)
	}

	// Ensure all transitive KXs complete.
	assertClientsKXd(t, bob, charlie)
	assertClientsKXd(t, charlie, dave)
	assertClientsKXd(t, dave, bob)

	// Bob leaves. Charlie gets a part event.
	reason := "because reasons"
	assert.NilErr(t, bob.PartFromGC(gcID, reason))
	part := assert.ChanWritten(t, charliePartChan)
	if part.uid != bob.PublicID() {
		t.Fatalf("unexpected part uid. got %s, want %s",
			part.uid, bob.PublicID())
	}
	if part.reason != reason {
		t.Fatalf("unexpected reason. got %s, want %s", part.reason,
			reason)
	}
	if part.kicked {
		t.Fatalf("unexpected kicked. got %v, want false", part.kicked)
	}

	// Dave is kicked by alice. Charlie gets a kick event.
	reason = "because reasons 2"
	assert.NilErr(t, alice.GCKick(gcID, dave.PublicID(), reason))
	part = assert.ChanWritten(t, charliePartChan)
	if part.uid != dave.PublicID() {
		t.Fatalf("unexpected part uid. got %s, want %s",
			part.uid, dave.PublicID())
	}
	if part.reason != reason {
		t.Fatalf("unexpected reason. got %s, want %s", part.reason,
			reason)
	}
	if !part.kicked {
		t.Fatalf("unexpected kicked. got %v, want true", part.kicked)
	}

	// Alice dissolves the GC. Charlie gets a kill event.
	reason = "because reasons 3"
	assert.NilErr(t, alice.KillGroupChat(gcID, reason))
	assert.ChanWritten(t, charlieKilledChan)
}

// TestGCBlockList tests scenarios where a user blocks another in a specific
// GC.
func TestGCBlockList(t *testing.T) {
	tcfg := testScaffoldCfg{}
	ts := newTestScaffold(t, tcfg)
	alice := ts.newClient("alice")
	bob := ts.newClient("bob")
	charlie := ts.newClient("charlie")

	bobMsgChan, aliceMsgChan, charlieMsgChan := make(chan string, 1), make(chan string, 1), make(chan string, 1)
	alice.handle(client.OnGCMNtfn(func(ru *client.RemoteUser, msg rpc.RMGroupMessage, ts time.Time) {
		aliceMsgChan <- msg.Message
	}))
	bob.handle(client.OnGCMNtfn(func(ru *client.RemoteUser, msg rpc.RMGroupMessage, ts time.Time) {
		bobMsgChan <- msg.Message
	}))
	charlie.handle(client.OnGCMNtfn(func(ru *client.RemoteUser, msg rpc.RMGroupMessage, ts time.Time) {
		charlieMsgChan <- msg.Message
	}))

	ts.kxUsers(alice, bob)
	ts.kxUsers(alice, charlie)
	ts.kxUsers(bob, charlie)

	gcName := "test_gc"
	gcID, err := alice.NewGroupChat(gcName)
	assert.NilErr(t, err)

	// Invite everyone.
	bobAcceptedChan := bob.acceptNextGCInvite(gcID)
	charlieAcceptedChan := charlie.acceptNextGCInvite(gcID)
	assert.NilErr(t, alice.InviteToGroupChat(gcID, bob.PublicID()))
	assert.NilErr(t, alice.InviteToGroupChat(gcID, charlie.PublicID()))
	assert.NilErrFromChan(t, bobAcceptedChan)
	assert.NilErrFromChan(t, charlieAcceptedChan)
	assertClientInGC(t, bob, gcID)
	assertClientInGC(t, charlie, gcID)
	assertClientSeesInGC(t, bob, gcID, charlie.PublicID())

	// Bob blocks Charlie in the GC.
	assert.NilErr(t, bob.AddToGCBlockList(gcID, charlie.PublicID()))

	// Bob sends a message, Alice gets it, but charlie does not.
	bobMsg := "message 1 from bob"
	assert.NilErr(t, bob.GCMessage(gcID, bobMsg, 0, nil))
	assert.ChanWrittenWithVal(t, aliceMsgChan, bobMsg)
	assert.ChanNotWritten(t, charlieMsgChan, 250*time.Millisecond)

	// Alice sends a message that both received.
	aliceMsg := "message 1 from alice"
	assert.NilErr(t, alice.GCMessage(gcID, aliceMsg, 0, nil))
	assert.ChanWrittenWithVal(t, bobMsgChan, aliceMsg)
	assert.ChanWrittenWithVal(t, charlieMsgChan, aliceMsg)

	// Charlie sends a message that Alice receives but Bob does not.
	charlieMsg := "message 1 from charlie"
	assert.NilErr(t, charlie.GCMessage(gcID, charlieMsg, 0, nil))
	assert.ChanWrittenWithVal(t, aliceMsgChan, charlieMsg)
	assert.ChanNotWritten(t, bobMsgChan, 250*time.Millisecond)
}

// TestGCKickBlockedUser tests that the gc admin can kick an user from a gc,
// even after this user was blocked by the admin.
func TestGCKickBlockedUser(t *testing.T) {
	// Setup Alice, Bob and Charlie have them KX with Alice.
	tcfg := testScaffoldCfg{}
	ts := newTestScaffold(t, tcfg)
	alice := ts.newClient("alice")
	bob := ts.newClient("bob")
	charlie := ts.newClient("charlie")

	ts.kxUsers(alice, bob)
	ts.kxUsers(alice, charlie)

	// Alice creates a GC and invites bob and charlie.
	gcID, err := alice.NewGroupChat("test gc")
	assert.NilErr(t, err)
	bobAcceptedChan := bob.acceptNextGCInvite(gcID)
	assert.NilErr(t, alice.InviteToGroupChat(gcID, bob.PublicID()))
	assert.NilErrFromChan(t, bobAcceptedChan)
	assertClientInGC(t, bob, gcID)
	charlieAcceptedChan := charlie.acceptNextGCInvite(gcID)
	assert.NilErr(t, alice.InviteToGroupChat(gcID, charlie.PublicID()))
	assert.NilErrFromChan(t, charlieAcceptedChan)
	assertClientInGC(t, charlie, gcID)

	// Bob and Charlie KXd due to autokx.
	assertClientsKXd(t, bob, charlie)

	// Setup handlers for GC messages.
	bobGCMsgChan, charlieGCMsgChan := make(chan string, 1), make(chan string, 1)
	bob.NotificationManager().Register(client.OnGCMNtfn(func(user *client.RemoteUser, msg rpc.RMGroupMessage, ts time.Time) {
		bobGCMsgChan <- msg.Message
	}))
	charlie.NotificationManager().Register(client.OnGCMNtfn(func(user *client.RemoteUser, msg rpc.RMGroupMessage, ts time.Time) {
		charlieGCMsgChan <- msg.Message
	}))

	// Alice sends a GC message. Both Bob and Charlie should receive it.
	testMsg1 := "test gc message 1"
	alice.GCMessage(gcID, testMsg1, rpc.MessageModeNormal, nil)
	gotBobMsg := assert.ChanWritten(t, bobGCMsgChan)
	assert.DeepEqual(t, gotBobMsg, testMsg1)
	gotCharlieMsg := assert.ChanWritten(t, charlieGCMsgChan)
	assert.DeepEqual(t, gotCharlieMsg, testMsg1)

	// Alice blocks bob.
	assert.NilErr(t, alice.Block(bob.PublicID()))

	// Alice ends a new GC message. Only charlie receives it.
	testMsg2 := "test gc message 2"
	alice.GCMessage(gcID, testMsg2, rpc.MessageModeNormal, nil)
	gotCharlieMsg = assert.ChanWritten(t, charlieGCMsgChan)
	assert.DeepEqual(t, gotCharlieMsg, testMsg2)
	assert.ChanNotWritten(t, bobGCMsgChan, 250*time.Millisecond)

	// Hook into charlie's user parted chan.
	charlieUserPartedChan := charlie.nextGCUserPartedIs(gcID, bob.PublicID(), true)

	// Alice removes bob from GC. Charlie receives this update.
	assert.NilErr(t, alice.GCKick(gcID, bob.PublicID(), "no reason"))
	assert.NilErrFromChan(t, charlieUserPartedChan)
}

// TestInviteToTwoGCsConcurrentAccept tests an old buggy scenario where the
// client could not accept two GC invites at the same time.
func TestInviteToTwoGCsConcurrentAccept(t *testing.T) {
	tcfg := testScaffoldCfg{}
	ts := newTestScaffold(t, tcfg)
	alice := ts.newClient("alice")
	bob := ts.newClient("bob")

	ts.kxUsers(alice, bob)

	gcName1, gcName2 := "test gc 1", "test gc 2"
	gcID1, err := alice.NewGroupChat(gcName1)
	assert.NilErr(t, err)
	gcID2, err := alice.NewGroupChat(gcName2)
	assert.NilErr(t, err)

	testDone := make(chan struct{})
	defer close(testDone)
	invite1Recvd := make(chan struct{})
	invite2Recvd := make(chan struct{})
	acceptErrChan := make(chan error, 2)

	// This handler is called async, once for each invite.
	bob.handle(client.OnInvitedToGCNtfn(func(ru *client.RemoteUser, iid uint64, invite rpc.RMGroupInvite) {
		if invite.ID == gcID1 {
			// Wait until invite 2 is received, then accept invite.
			close(invite1Recvd)
			select {
			case <-invite2Recvd:
			case <-testDone:
			}
			acceptErrChan <- bob.AcceptGroupChatInvite(iid)
		} else if invite.ID == gcID2 {
			// Singal invite 2 received, wait until invite 1
			// accepted, then accept this invite.
			close(invite2Recvd)
			select {
			case <-invite1Recvd:
			case <-testDone:
			}
			acceptErrChan <- bob.AcceptGroupChatInvite(iid)
		}
	}))

	// Alice invites to the 2 GCs.
	assert.NilErr(t, alice.InviteToGroupChat(gcID1, bob.PublicID()))
	assert.NilErr(t, alice.InviteToGroupChat(gcID2, bob.PublicID()))

	// Expect 2 GC acceptances.
	for i := 0; i < 2; i++ {
		assert.ChanWrittenWithVal(t, acceptErrChan, nil)
	}

	// Expect Bob to be in the two GCs.
	assertClientInGC(t, bob, gcID1)
	assertClientInGC(t, bob, gcID2)
}

// TestInviteToTwoGCsConcurrentAccept tests an old buggy scenario where the
// client could not accept a GC after it had joined a second one.
func TestInviteToTwoGCsAcceptAfterJoin(t *testing.T) {
	tcfg := testScaffoldCfg{}
	ts := newTestScaffold(t, tcfg)
	alice := ts.newClient("alice")
	bob := ts.newClient("bob")

	ts.kxUsers(alice, bob)

	gcName1, gcName2 := "test gc 1", "test gc 2"
	gcID1, err := alice.NewGroupChat(gcName1)
	assert.NilErr(t, err)
	gcID2, err := alice.NewGroupChat(gcName2)
	assert.NilErr(t, err)

	testDone := make(chan struct{})
	defer close(testDone)
	invite2Recvd := make(chan struct{})
	acceptErrChan := make(chan error, 2)
	joinedGC1 := make(chan struct{})

	// This handler is called async, once for each invite.
	bob.handle(client.OnInvitedToGCNtfn(func(ru *client.RemoteUser, iid uint64, invite rpc.RMGroupInvite) {
		if invite.ID == gcID1 {
			// Wait until invite 2 is received, then accept invite.
			select {
			case <-invite2Recvd:
			case <-testDone:
			}
			acceptErrChan <- bob.AcceptGroupChatInvite(iid)
		} else if invite.ID == gcID2 {
			// Singal invite 2 received, wait until fully joined
			// GC1, then accept this invite.
			close(invite2Recvd)
			select {
			case <-joinedGC1:
			case <-testDone:
			}
			acceptErrChan <- bob.AcceptGroupChatInvite(iid)
		}
	}))
	bob.handle(client.OnJoinedGCNtfn(func(gc rpc.RMGroupList) {
		if gc.ID == gcID1 {
			close(joinedGC1)
		}
	}))

	// Alice invites to the 2 GCs.
	assert.NilErr(t, alice.InviteToGroupChat(gcID1, bob.PublicID()))
	assert.NilErr(t, alice.InviteToGroupChat(gcID2, bob.PublicID()))

	// Expect 2 GC acceptances.
	for i := 0; i < 2; i++ {
		assert.ChanWrittenWithVal(t, acceptErrChan, nil)
	}

	// Expect Bob to be in the two GCs.
	assertClientInGC(t, bob, gcID1)
	assertClientInGC(t, bob, gcID2)

	gotGCName1, err := bob.GetGCAlias(gcID1)
	assert.NilErr(t, err)
	assert.DeepEqual(t, gotGCName1, gcName1)

	gotGCName2, err := bob.GetGCAlias(gcID2)
	assert.NilErr(t, err)
	assert.DeepEqual(t, gotGCName2, gcName2)

}
