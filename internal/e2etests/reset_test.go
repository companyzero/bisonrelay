package e2etests

import (
	"testing"
	"time"

	"github.com/companyzero/bisonrelay/client"
	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/companyzero/bisonrelay/internal/assert"
	"github.com/companyzero/bisonrelay/rpc"
)

// TestDirectReset tests that resetting via reset RVs works.
func TestDirectReset(t *testing.T) {
	// Setup Alice and Bob and have them kx.
	tcfg := testScaffoldCfg{}
	ts := newTestScaffold(t, tcfg)
	alice := ts.newClient("alice")
	bob := ts.newClient("bob")

	aliceKXdChan, bobKXdChan := make(chan struct{}), make(chan struct{})
	alice.handle(client.OnKXCompleted(func(*clientintf.RawRVID, *client.RemoteUser) {
		aliceKXdChan <- struct{}{}
	}))
	bob.handle(client.OnKXCompleted(func(*clientintf.RawRVID, *client.RemoteUser) {
		bobKXdChan <- struct{}{}
	}))

	alicePMChan, bobPMChan := make(chan string, 1), make(chan string, 1)
	alice.handle(client.OnPMNtfn(func(ru *client.RemoteUser, pm rpc.RMPrivateMessage, ts time.Time) {
		alicePMChan <- pm.Message
	}))
	bob.handle(client.OnPMNtfn(func(ru *client.RemoteUser, pm rpc.RMPrivateMessage, ts time.Time) {
		bobPMChan <- pm.Message
	}))

	// Helper to consume the KXCompleted events.
	assertKXCompleted := func() {
		t.Helper()
		assert.ChanWritten(t, aliceKXdChan)
		assert.ChanWritten(t, bobKXdChan)
	}

	ts.kxUsers(alice, bob)
	assertKXCompleted()

	// Perform a reset from Alice to Bob.
	err := alice.ResetRatchet(bob.PublicID())
	assert.NilErr(t, err)

	// Ensure we got the new reset events.
	assertKXCompleted()

	// Reset on the other direction.
	err = bob.ResetRatchet(alice.PublicID())
	assert.NilErr(t, err)

	// Ensure we got the new reset events.
	assertKXCompleted()

	// Ensure Alice and Bob can message each other.
	aliceMsg, bobMsg := "i am alice", "i am bob"
	assert.NilErr(t, alice.PM(bob.PublicID(), aliceMsg))
	assert.NilErr(t, bob.PM(alice.PublicID(), bobMsg))
	assert.ChanWrittenWithVal(t, alicePMChan, bobMsg)
	assert.ChanWrittenWithVal(t, bobPMChan, aliceMsg)
}

// TestTransitiveReset tests that doing a transitive reset works.
func TestTransitiveReset(t *testing.T) {
	// Setup Alice, Bob and Charlie and have them kx.
	tcfg := testScaffoldCfg{}
	ts := newTestScaffold(t, tcfg)
	alice := ts.newClient("alice")
	bob := ts.newClient("bob")
	charlie := ts.newClient("charlie")

	aliceKXdChan, bobKXdChan := make(chan struct{}), make(chan struct{})
	alice.handle(client.OnKXCompleted(func(*clientintf.RawRVID, *client.RemoteUser) {
		aliceKXdChan <- struct{}{}
	}))
	bob.handle(client.OnKXCompleted(func(*clientintf.RawRVID, *client.RemoteUser) {
		bobKXdChan <- struct{}{}
	}))

	alicePMChan, bobPMChan := make(chan string, 1), make(chan string, 1)
	alice.handle(client.OnPMNtfn(func(ru *client.RemoteUser, pm rpc.RMPrivateMessage, ts time.Time) {
		alicePMChan <- pm.Message
	}))
	bob.handle(client.OnPMNtfn(func(ru *client.RemoteUser, pm rpc.RMPrivateMessage, ts time.Time) {
		bobPMChan <- pm.Message
	}))

	// Helper to consume the KXCompleted events.
	assertKXCompleted := func() {
		t.Helper()
		assert.ChanWritten(t, aliceKXdChan)
		assert.ChanWritten(t, bobKXdChan)
	}

	// Helper to verify the Alice -> Bob ratchet works.
	checkAliceBobRatchet := func() {
		aliceMsg, bobMsg := "i am alice", "i am bob"
		assert.NilErr(t, alice.PM(bob.PublicID(), aliceMsg))
		assert.NilErr(t, bob.PM(alice.PublicID(), bobMsg))
		assert.ChanWrittenWithVal(t, alicePMChan, bobMsg)
		assert.ChanWrittenWithVal(t, bobPMChan, aliceMsg)
	}

	// Complete KXs between Alice->Bob, Alice->Charlie and Bob->Charlie
	ts.kxUsers(alice, bob)
	ts.kxUsers(alice, charlie)
	ts.kxUsers(bob, charlie)

	// We should get two kx events on Alice and Bob.
	assertKXCompleted()
	assertKXCompleted()

	// Verify the Alice-Bob ratchet works.
	checkAliceBobRatchet()

	// Perform a transitive reset Alice -> Charlie -> Bob
	err := alice.RequestTransitiveReset(charlie.PublicID(), bob.PublicID())
	assert.NilErr(t, err)

	// We should get new KX completed events.
	assertKXCompleted()

	// Ensure Alice and Bob can message each other.
	checkAliceBobRatchet()
}
