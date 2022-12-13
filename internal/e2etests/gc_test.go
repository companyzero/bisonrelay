package e2etests

import (
	"testing"
	"time"

	"github.com/companyzero/bisonrelay/client"
	"github.com/companyzero/bisonrelay/internal/assert"
	"github.com/companyzero/bisonrelay/rpc"
)

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
	bob.modifyHandlers(func() {
		bob.onGCMsg = func(user *client.RemoteUser, msg rpc.RMGroupMessage, ts time.Time) {
			bobGCMsgChan <- msg.Message
		}
	})
	charlie.modifyHandlers(func() {
		charlie.onGCMsg = func(user *client.RemoteUser, msg rpc.RMGroupMessage, ts time.Time) {
			charlieGCMsgChan <- msg.Message
		}
	})

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
