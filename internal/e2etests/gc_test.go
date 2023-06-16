package e2etests

import (
	"fmt"
	"math/rand"
	"regexp"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/companyzero/bisonrelay/client"
	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/companyzero/bisonrelay/internal/assert"
	"github.com/companyzero/bisonrelay/rpc"
	"github.com/companyzero/bisonrelay/zkidentity"
)

// TestInviteGCOnKX performs tests for inviting new users to GC's
func TestInviteGCOnKX(t *testing.T) {
	t.Parallel()

	// Setup Alice and Bob.
	tcfg := testScaffoldCfg{}
	ts := newTestScaffold(t, tcfg)
	alice := ts.newClient("alice")
	bob := ts.newClient("bob")

	aliceAcceptedInvitesChan := make(chan clientintf.UserID, 1)
	alice.handle(client.OnGCInviteAcceptedNtfn(func(user *client.RemoteUser, _ rpc.RMGroupList) {
		aliceAcceptedInvitesChan <- user.ID()
	}))
	bobJoinedGCChan := make(chan struct{}, 1)
	bob.handle(client.OnJoinedGCNtfn(func(gc rpc.RMGroupList) {
		bobJoinedGCChan <- struct{}{}
	}))

	// Alice creates a GC.
	gcName := "testGC"
	gcID, err := alice.NewGroupChat(gcName)
	assert.NilErr(t, err)

	// Setup Bob to accept Alice's invite and send the invite.
	bobAcceptedChan := bob.acceptNextGCInvite(gcID)

	ts.kxUsersWithInvite(alice, bob, gcID)

	// Bob correctly accepted to join and Alice was notified Bob joined.
	assert.NilErrFromChan(t, bobAcceptedChan)
	assert.ChanWrittenWithVal(t, aliceAcceptedInvitesChan, bob.PublicID())
	assert.ChanWritten(t, bobJoinedGCChan)

	// Double check bob is in the GC.
	assertClientInGC(t, bob, gcID)
}

// TestBasicGCFeatures performs tests for the basic GC features.
func TestBasicGCFeatures(t *testing.T) {
	t.Parallel()

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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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

// TestBlockIgnore ensures that the block and ignore features work as
// intended.
func TestBlockIgnore(t *testing.T) {
	t.Parallel()

	tcfg := testScaffoldCfg{}
	ts := newTestScaffold(t, tcfg)
	alice := ts.newClient("alice")
	bob := ts.newClient("bob")
	charlie := ts.newClient("charlie")

	// Hook needed handlers.
	bobBlockedChan := make(chan struct{}, 1)
	bob.handle(client.OnBlockNtfn(func(user *client.RemoteUser) {
		bobBlockedChan <- struct{}{}
	}))

	ts.kxUsers(alice, bob)
	ts.kxUsers(alice, charlie)
	ts.kxUsers(bob, charlie)

	gcName := "testGC"
	var gcID zkidentity.ShortID

	// Sleep until initial gc delay elapses.
	time.Sleep(alice.cfg.GCMQInitialDelay + alice.cfg.GCMQMaxLifetime*2)

	// Test base case of Alice and Bob chatting.
	assertClientsCanPM(t, alice, bob)

	// Alice ignores bob.
	assert.NilErr(t, alice.Ignore(bob.PublicID(), true))

	// Alice can msg Bob, Bob cannot msg Alice.
	assertClientsCanPMOneWay(t, alice, bob)
	assertClientsCannotPMOneWay(t, bob, alice)

	// Alice un-ignores Bob. Bob can now msg Alice again.
	assert.NilErr(t, alice.Ignore(bob.PublicID(), false))
	assertClientsCanPM(t, alice, bob)

	// Alice blocks Bob.
	assert.NilErr(t, alice.Block(bob.PublicID()))
	assert.ChanWritten(t, bobBlockedChan)

	// Attempting to send a message from either Bob or Alice errors.
	assert.NonNilErr(t, alice.PM(bob.PublicID(), ""))
	assert.NonNilErr(t, bob.PM(alice.PublicID(), ""))

	// Charlie knows Alice and Bob. Create a new GC, and invite both. It
	// should _not_ trigger a new autokx between Alice and Bob.
	bobKXdChan, aliceKXdChan := make(chan struct{}, 1), make(chan struct{}, 1)
	bob.handle(client.OnKXCompleted(func(*clientintf.RawRVID, *client.RemoteUser, bool) {
		bobKXdChan <- struct{}{}
	}))
	alice.handle(client.OnKXCompleted(func(*clientintf.RawRVID, *client.RemoteUser, bool) {
		aliceKXdChan <- struct{}{}
	}))
	gcID, err := charlie.NewGroupChat(gcName)
	assert.NilErr(t, err)

	// Invite Alice and Bob to it and ensure they joined.
	assertClientJoinsGC(t, gcID, charlie, alice)
	assertClientJoinsGC(t, gcID, charlie, bob)

	// Ensure they did not KX with each other.
	assert.ChanNotWritten(t, aliceKXdChan, 500*time.Millisecond)
	assert.ChanNotWritten(t, bobKXdChan, 500*time.Millisecond)

	// Ensure a GCM sent by charlie is received by both Alice and Bob.
	assertClientsCanSeeGCM(t, gcID, charlie, alice, bob)

	// Ensure a GCM sent by Alice is seen by Charlie, but not Bob.
	assertClientsCanSeeGCM(t, gcID, alice, charlie)
	assertClientCannotSeeGCM(t, gcID, alice, bob)

	// Ensure a GCM sent by Bob is seen by Charlie, but not Alice.
	assertClientsCanSeeGCM(t, gcID, bob, charlie)
	assertClientCannotSeeGCM(t, gcID, bob, alice)

	// Ensure Alice and Bob are not kx'd together.
	if alice.UserExists(bob.PublicID()) {
		t.Fatal("Alice knows Bob (but shouldn't)")
	}
	if bob.UserExists(alice.PublicID()) {
		t.Fatal("Bob knows Alice (but shouldn't)")
	}
}

// TestVersion1GCs tests version 1 GC features (extra admins).
func TestVersion1GCs(t *testing.T) {
	t.Parallel()

	tcfg := testScaffoldCfg{}
	ts := newTestScaffold(t, tcfg)
	alice := ts.newClient("alice")
	bob := ts.newClient("bob")
	charlie := ts.newClient("charlie")

	ts.kxUsers(alice, bob)
	ts.kxUsers(alice, charlie)
	ts.kxUsers(bob, charlie)

	gcID, err := alice.NewGroupChatVersion("test gc", 0)
	assert.NilErr(t, err)

	// Sanity check that the GC was originally created to be version 0.
	v0gc, err := alice.GetGC(gcID)
	assert.NilErr(t, err)
	assert.DeepEqual(t, v0gc.Version, 0)

	// Bob joins the GC.
	bob.acceptNextGCInvite(gcID)
	assert.NilErr(t, alice.InviteToGroupChat(gcID, bob.PublicID()))
	assertClientInGC(t, bob, gcID)

	// Bob cannot invite charlie.
	err = bob.InviteToGroupChat(gcID, charlie.PublicID())
	assert.NonNilErr(t, err)

	// Upgrade the GC. Bob sees the upgrade.
	bobUpgradedGCChan := make(chan error, 1)
	bob.handle(client.OnGCUpgradedNtfn(func(gc rpc.RMGroupList, oldVersion uint8) {
		if oldVersion != 0 {
			bobUpgradedGCChan <- fmt.Errorf("unexpected old version: "+
				"got %d, want %d", oldVersion, 0)
		} else if gc.Version != 1 {
			bobUpgradedGCChan <- fmt.Errorf("unexpected new version: "+
				"got %d, want %d", gc.Version, 1)
		} else {
			bobUpgradedGCChan <- nil
		}
	}))
	err = alice.UpgradeGC(gcID, 1)
	assert.NilErr(t, err)
	assert.NilErrFromChan(t, bobUpgradedGCChan)

	// Alice adds Bob as extra admin.
	bobAddedExtraAdminChan := make(chan error, 1)
	ntfnReg := bob.handle(client.OnGCAdminsChangedNtfn(func(_ *client.RemoteUser, gc rpc.RMGroupList, added, removed []zkidentity.ShortID) {
		if len(added) != 1 {
			bobAddedExtraAdminChan <- fmt.Errorf("unexpected nb of "+
				"added admins: got %d, want %d", len(added), 1)
		} else if added[0] != bob.PublicID() {
			bobAddedExtraAdminChan <- fmt.Errorf("unexpected id of "+
				"added admin: got %s, want %s", added[0],
				bob.PublicID())
		} else {
			bobAddedExtraAdminChan <- nil
		}

	}))
	err = alice.ModifyGCAdmins(gcID, []zkidentity.ShortID{bob.PublicID()}, "")
	assert.NilErr(t, err)
	assert.NilErrFromChan(t, bobAddedExtraAdminChan)
	ntfnReg.Unregister()

	// Bob can now invite Charlie to the GC. Alice should see charlie and
	// everyone should be GCM'ing.
	charlie.acceptNextGCInvite(gcID)
	assert.NilErr(t, bob.InviteToGroupChat(gcID, charlie.PublicID()))
	assertClientInGC(t, charlie, gcID)
	assertClientSeesInGC(t, alice, gcID, charlie.PublicID())
	assertClientsCanGCM(t, gcID, alice, bob, charlie)

	// Bob will remove itself and add charlie as admin.
	charlieAddedExtraAdminChan := make(chan error, 1)
	ntfnReg = charlie.handle(client.OnGCAdminsChangedNtfn(func(_ *client.RemoteUser, gc rpc.RMGroupList, added, removed []zkidentity.ShortID) {
		if len(added) != 1 {
			charlieAddedExtraAdminChan <- fmt.Errorf("unexpected nb of "+
				"added admins: got %d, want %d", len(added), 1)
		} else if len(removed) != 1 {
			charlieAddedExtraAdminChan <- fmt.Errorf("unexpected nb of "+
				"removed admins: got %d, want %d", len(removed), 1)
		} else if removed[0] != bob.PublicID() {
			charlieAddedExtraAdminChan <- fmt.Errorf("unexpected id of "+
				"added admin: got %s, want %s", added[0],
				bob.PublicID())
		} else if added[0] != charlie.PublicID() {
			charlieAddedExtraAdminChan <- fmt.Errorf("unexpected id of "+
				"added admin: got %s, want %s", added[0],
				charlie.PublicID())
		} else {
			charlieAddedExtraAdminChan <- nil
		}

	}))
	err = bob.ModifyGCAdmins(gcID, []zkidentity.ShortID{charlie.PublicID()}, "")
	assert.NilErr(t, err)
	assert.NilErrFromChan(t, charlieAddedExtraAdminChan)
	ntfnReg.Unregister()

	// Charlie will kick Bob from the GC now that he's an admin.
	bobKickedChan := make(chan error, 1)
	bob.handle(client.OnGCUserPartedNtfn(func(gcid client.GCID, uid client.UserID, reason string, kicked bool) {
		if uid != bob.PublicID() {
			bobKickedChan <- fmt.Errorf("unexpected kicked uid: "+
				"got %s, want %s", uid, bob.PublicID())
		} else if !kicked {
			bobKickedChan <- fmt.Errorf("unexpected kicked flag: "+
				"got %v, want %v", kicked, true)
		} else {
			bobKickedChan <- nil
		}
	}))
	err = charlie.GCKick(gcID, bob.PublicID(), "")
	assert.NilErr(t, err)
	assert.NilErrFromChan(t, bobKickedChan)
	assertClientsCanGCM(t, gcID, alice, charlie)
	assertClientCannotSeeGCM(t, gcID, alice, bob)

	// Charlie cannot kick Alice.
	err = charlie.GCKick(gcID, alice.PublicID(), "")
	assert.NonNilErr(t, err)
}

// TestGCCrossedMediatedKX tests a scenario that could cause broken ratchets
// when two users are added simultaneously to GCs where both will attempt
// to KX with each other.
//
// One of the users should skip attempting the KX in order not to cause the
// broken ratchet.
func TestGCCrossedMediatedKX(t *testing.T) {
	t.Parallel()

	// During this test Alice, Bob and Charlie are kept offline at various
	// stages in order to guarantee concurrent processing of the messages
	// by Charlie and Bob and cause both to launch parallel KX attempts.
	//
	// In order to verify that one of the KXs was skipped, track the log
	// lines for the pattern of the log indicating the skip.
	skipLogMsgPattern := `Skipping accepting invite for kx .* due to already ongoing kx [^\n]*\n$`
	tls := &testLogLineScanner{re: *regexp.MustCompile(skipLogMsgPattern)}

	// Test scaffold setup.
	tcfg := testScaffoldCfg{logScanner: tls}
	ts := newTestScaffold(t, tcfg)
	alice := ts.newClient("alice")
	bob := ts.newClient("bob")
	charlie := ts.newClient("charlie")

	// Alice will be the GC admin.
	aliceAcceptedInvitesChan := make(chan clientintf.UserID, 1)
	alice.handle(client.OnGCInviteAcceptedNtfn(func(user *client.RemoteUser, _ rpc.RMGroupList) {
		aliceAcceptedInvitesChan <- user.ID()
	}))

	// Bob and Charlie will join the GCs.
	bobJoinedChan, bobUpdatedChan := make(chan struct{}, 2), make(chan struct{}, 2)
	bob.handle(client.OnJoinedGCNtfn(func(_ rpc.RMGroupList) { bobJoinedChan <- struct{}{} }))
	bob.handle(client.OnAddedGCMembersNtfn(func(_ rpc.RMGroupList, _ []clientintf.UserID) { bobUpdatedChan <- struct{}{} }))
	charlieJoinedChan, charlieUpdatedChan := make(chan struct{}, 2), make(chan struct{}, 2)
	charlie.handle(client.OnJoinedGCNtfn(func(_ rpc.RMGroupList) { charlieJoinedChan <- struct{}{} }))
	charlie.handle(client.OnAddedGCMembersNtfn(func(_ rpc.RMGroupList, _ []clientintf.UserID) { charlieUpdatedChan <- struct{}{} }))

	// KX Alice to Bob and Charlie.
	ts.kxUsers(alice, bob)
	ts.kxUsers(alice, charlie)

	// Track the additional KX between Bob and Charlie.
	bobKXCompletedChan := make(chan struct{}, 10)
	bob.handle(client.OnKXCompleted(func(_ *clientintf.RawRVID, ru *client.RemoteUser, _ bool) {
		bobKXCompletedChan <- struct{}{}
	}))

	// Create the test GCs. Each user will first be added to one, then
	// to the other.
	gcID1, err := alice.NewGroupChat("gc01")
	assert.NilErr(t, err)
	gcID2, err := alice.NewGroupChat("gc02")
	assert.NilErr(t, err)

	// Invite Bob to GC1 and Charlie to GC2.
	bobAcceptedChan := bob.acceptNextGCInvite(gcID1)
	charlieAcceptedChan := charlie.acceptNextGCInvite(gcID2)
	assert.NilErr(t, alice.InviteToGroupChat(gcID1, bob.PublicID()))
	assert.NilErr(t, alice.InviteToGroupChat(gcID2, charlie.PublicID()))
	assert.NilErrFromChan(t, bobAcceptedChan)
	assert.NilErrFromChan(t, charlieAcceptedChan)
	for i := 0; i < 2; i++ {
		assert.ChanWritten(t, aliceAcceptedInvitesChan)
	}
	assert.ChanWritten(t, bobJoinedChan)
	assert.ChanWritten(t, charlieJoinedChan)
	assertEmptyRMQ(t, bob)
	assertEmptyRMQ(t, charlie)
	assertEmptyRMQ(t, alice)
	ts.log.Infof("Test setup done")

	// Bob and Charlie go offline.
	assertGoesOffline(t, bob)
	assertGoesOffline(t, charlie)

	// Invite Bob to GC2 and Charlie to GC1 (crossed invites).
	bobAcceptedChan = bob.acceptNextGCInvite(gcID2)
	charlieAcceptedChan = charlie.acceptNextGCInvite(gcID1)
	assert.NilErr(t, alice.InviteToGroupChat(gcID2, bob.PublicID()))
	assert.NilErr(t, alice.InviteToGroupChat(gcID1, charlie.PublicID()))
	assertEmptyRMQ(t, alice)
	ts.log.Infof("Bob and Charlie cross-invited")

	// Alice goes offline to prevent sending the GC lists just yet. Bob
	// and Charlie come online and accept the invite.
	assertGoesOffline(t, alice)
	assertGoesOnline(t, bob)
	assertGoesOnline(t, charlie)
	assert.NilErrFromChan(t, bobAcceptedChan)
	assert.NilErrFromChan(t, charlieAcceptedChan)
	assertEmptyRMQ(t, bob)
	assertEmptyRMQ(t, charlie)
	ts.log.Infof("Bob and Charlie accepted invites")

	// Bob and Charlie go offline to prevent the mediate kx to be requested
	// immediately. Alice comes online and sends the GC lists.
	assertGoesOffline(t, bob)
	assertGoesOffline(t, charlie)
	assertGoesOnline(t, alice)
	for i := 0; i < 2; i++ {
		assert.ChanWritten(t, aliceAcceptedInvitesChan)
	}
	time.Sleep(200 * time.Millisecond)
	assertEmptyRMQ(t, alice)
	ts.log.Infof("Alice sent GC lists")

	// Alice goes offline to prevent the mediate kx to be processed. Bob
	// and Charlie come online, receive the GC lists for the GC just joined
	// and an update for the GC they were already in. They send the request
	// to Alice for a transitive KX with each other (each one sends one
	// request to the other).
	assertGoesOffline(t, alice)
	assertGoesOnline(t, bob)
	assertGoesOnline(t, charlie)
	assert.ChanWritten(t, bobJoinedChan)
	assert.ChanWritten(t, charlieJoinedChan)
	assert.ChanWritten(t, bobUpdatedChan)
	assert.ChanWritten(t, charlieUpdatedChan)
	time.Sleep(200 * time.Millisecond)
	assertEmptyRMQ(t, bob)
	assertEmptyRMQ(t, charlie)
	ts.log.Infof("Bob and Charlie sent RMMediateIdentity")

	// Charlie and Bob go offline. Alice comes online, receives the
	// RMMediateIdentity, sends RMInvite towards the target.
	assertGoesOffline(t, bob)
	assertGoesOffline(t, charlie)
	assertGoesOnline(t, alice)
	time.Sleep(200 * time.Millisecond)
	assertEmptyRMQ(t, alice)
	ts.log.Infof("Alice sent RMInvites")

	// Alice goes offline. Charlie and Bob come online, receive the
	// RMInvite, send the RMTransitiveMessage to Alice.
	assertGoesOffline(t, alice)
	assertGoesOnline(t, bob)
	assertGoesOnline(t, charlie)
	time.Sleep(200 * time.Millisecond)
	assertEmptyRMQ(t, bob)
	assertEmptyRMQ(t, charlie)
	ts.log.Infof("Bob and Charlie sent RMTransitiveMessage")

	// Charlie and Bob go offline. Alice comes online, sends the
	// RMTransitiveMessageFwd messages.
	assertGoesOffline(t, bob)
	assertGoesOffline(t, charlie)
	assertGoesOnline(t, alice)
	time.Sleep(200 * time.Millisecond)
	assertEmptyRMQ(t, alice)
	ts.log.Infof("Alice sent RMTransitiveMessageFwds")

	// Alice goes offline. Charlie and Bob come online, receive the
	// forwarded invites. Both will attempt to accept the corresponding
	// invite, but one will skip it due to both also having created one
	// invite.
	assertGoesOffline(t, alice)
	go bob.GoOnline()
	go charlie.GoOnline()

	// Ensure transitive KX completes.
	assertClientsKXd(t, bob, charlie)
	assert.ChanWritten(t, bobKXCompletedChan)
	assertClientsCanPM(t, bob, charlie)

	// Ensure one of the KXs was skipped.
	assert.ChanNotWritten(t, bobKXCompletedChan, time.Second)
	assert.DeepEqual(t, tls.hasMatches(), true)
}

// TestGCUnkxMediateIDRetries asserts that sending a message on a GC with
// unkxd members causes a mediate ID request to be sent.
func TestGCUnkxMediateIDRetries(t *testing.T) {
	t.Parallel()

	tcfg := testScaffoldCfg{}
	ts := newTestScaffold(t, tcfg)
	alice := ts.newClient("alice")
	bob := ts.newClient("bob")
	charlie := ts.newClient("charlie")

	// KX Alice to Bob and Charlie.
	ts.kxUsers(alice, bob)
	ts.kxUsers(alice, charlie)

	// Create the GC and add Bob.
	gcID, err := alice.NewGroupChat("gc01")
	assert.NilErr(t, err)
	assertClientJoinsGC(t, gcID, alice, bob)

	// Setup handlers.
	aliceInviteAcceptedChan := make(chan clientintf.UserID, 3)
	alice.handle(client.OnGCInviteAcceptedNtfn(func(ru *client.RemoteUser, gc rpc.RMGroupList) {
		aliceInviteAcceptedChan <- ru.ID()
	}))
	bobNewMembersChan := make(chan clientintf.UserID, 3)
	bob.handle(client.OnAddedGCMembersNtfn(func(_ rpc.RMGroupList, uids []clientintf.UserID) {
		for _, uid := range uids {
			bobNewMembersChan <- uid
		}
	}))
	type gcUnkxdNtfnInfo struct {
		hasKX   bool
		hasMI   bool
		miCount uint32
		medID   clientintf.UserID
	}
	bobGCUnkxdMemberChan := make(chan gcUnkxdNtfnInfo, 10)
	bob.handle(client.OnGCWithUnkxdMemberNtfn(func(gc zkidentity.ShortID, uid clientintf.UserID,
		hasKX, hasMI bool, miCount uint32, startedMIMediator *clientintf.UserID) {
		if gc != gcID || uid != charlie.PublicID() {
			return
		}
		var medID clientintf.UserID
		if startedMIMediator != nil {
			medID = *startedMIMediator
		}
		bobGCUnkxdMemberChan <- gcUnkxdNtfnInfo{
			hasKX: hasKX, hasMI: hasMI, miCount: miCount, medID: medID,
		}
	}))
	bobKXCompletedChan := make(chan struct{}, 5)
	bob.handle(client.OnKXCompleted(func(*clientintf.RawRVID, *client.RemoteUser, bool) {
		bobKXCompletedChan <- struct{}{}
	}))

	// Alice invites Charlie, then goes offline to prevent the MI from
	// completing.
	assertGoesOffline(t, charlie)
	assert.NilErr(t, alice.InviteToGroupChat(gcID, charlie.PublicID()))
	assertGoesOffline(t, alice)

	// Charlie comes online, accepts the invite, then goes offline again to
	// prevent the MI from completing once Alice processes the join event.
	charlieAcceptedChan := charlie.acceptNextGCInvite(gcID)
	assertGoesOnline(t, charlie)
	assert.NilErrFromChan(t, charlieAcceptedChan)
	assertGoesOffline(t, charlie)

	// Alice comes online, processes the invite. Bob sees the new GC member.
	assertGoesOnline(t, alice)
	assert.ChanWrittenWithVal(t, aliceInviteAcceptedChan, charlie.PublicID())
	assert.ChanWrittenWithVal(t, bobNewMembersChan, charlie.PublicID())

	// Bob sends a message. We don't expect any notifications yet.
	assert.NilErr(t, bob.GCMessage(gcID, "msg01", 0, nil))
	assert.ChanNotWritten(t, bobGCUnkxdMemberChan, 100*time.Millisecond)

	// Wait for the time notifications are sent, then try again. This should
	// generate a ntfn without MI starting yet.
	time.Sleep(bob.cfg.UnkxdWarningTimeout)
	assert.NilErr(t, bob.GCMessage(gcID, "msg02", 0, nil))
	wantUnkx := gcUnkxdNtfnInfo{}
	assert.ChanWrittenWithVal(t, bobGCUnkxdMemberChan, wantUnkx)

	// Wait for the time until an MI will be attempted as a result of
	// an unkxd member. The mediator ID in the ntfn is set to the GC's
	// owner (Alice), but other fields are cleared.
	time.Sleep(bob.cfg.RecentMediateIDThreshold)
	assert.NilErr(t, bob.GCMessage(gcID, "msg03", 0, nil))
	wantUnkx = gcUnkxdNtfnInfo{medID: alice.PublicID()}
	assert.ChanWrittenWithVal(t, bobGCUnkxdMemberChan, wantUnkx)

	// Wait for half the time until the next mediate ID attempt. We should
	// not get a new attempt, but we should get an indication that there are
	// attempts.
	time.Sleep(bob.cfg.RecentMediateIDThreshold / 2)
	assert.NilErr(t, bob.GCMessage(gcID, "msg04", 0, nil))
	wantUnkx = gcUnkxdNtfnInfo{hasMI: true, miCount: 1}
	assert.ChanWrittenWithVal(t, bobGCUnkxdMemberChan, wantUnkx)

	// Wait another half time until the next mediate ID attempt. We should
	// get a new attempt.
	time.Sleep(bob.cfg.RecentMediateIDThreshold / 2)
	assert.NilErr(t, bob.GCMessage(gcID, "msg05", 0, nil))
	wantUnkx = gcUnkxdNtfnInfo{medID: alice.PublicID(), miCount: 1}
	assert.ChanWrittenWithVal(t, bobGCUnkxdMemberChan, wantUnkx)

	// Wait a third time for the next mediate ID attempt. This should be
	// the final MI attempt.
	time.Sleep(bob.cfg.RecentMediateIDThreshold)
	assert.NilErr(t, bob.GCMessage(gcID, "msg06", 0, nil))
	wantUnkx = gcUnkxdNtfnInfo{medID: alice.PublicID(), miCount: 2}
	assert.ChanWrittenWithVal(t, bobGCUnkxdMemberChan, wantUnkx)

	// Wait a fourth time for the next mediate ID attempt. This should
	// trigger a warning but the field that indicates the MI is being
	// attempted should be unset.
	time.Sleep(bob.cfg.RecentMediateIDThreshold)
	assert.NilErr(t, bob.GCMessage(gcID, "msg07", 0, nil))
	wantUnkx = gcUnkxdNtfnInfo{miCount: 3}
	assert.ChanWrittenWithVal(t, bobGCUnkxdMemberChan, wantUnkx)

	// Wait a fifth time for the next mediate ID attempt. This should no
	// longer trigger a notification.
	time.Sleep(bob.cfg.RecentMediateIDThreshold)
	assert.NilErr(t, bob.GCMessage(gcID, "msg07", 0, nil))
	assert.ChanNotWritten(t, bobGCUnkxdMemberChan, 100*time.Millisecond)

}
