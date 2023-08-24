package e2etests

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/companyzero/bisonrelay/client"
	"github.com/companyzero/bisonrelay/client/clientdb"
	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/companyzero/bisonrelay/internal/assert"
	"github.com/companyzero/bisonrelay/rpc"
)

// TestResendsUnackedRM tests shutting down the client while there are
// unacknowledged RMs inflight works as expected and does not cause a busted
// ratchet.
func TestResendsUnackedRM(t *testing.T) {
	t.Parallel()

	// Setup Alice and Bob and have them KX.
	tcfg := testScaffoldCfg{}
	ts := newTestScaffold(t, tcfg)
	alice := ts.newClient("alice")
	bob := ts.newClient("bob")
	invite, err := alice.WriteNewInvite(io.Discard, nil)
	assert.NilErr(t, err)
	assert.NilErr(t, bob.AcceptInvite(invite))
	assertClientsKXd(t, alice, bob)

	// Hook into Alice's and Bob's onPM event.
	bobPMChan := make(chan string, 7)
	bob.NotificationManager().Register(client.OnPMNtfn(func(user *client.RemoteUser, msg rpc.RMPrivateMessage, ts time.Time) {
		bobPMChan <- msg.Message
	}))

	alicePMChan := make(chan string, 2)
	alice.NotificationManager().Register(client.OnPMNtfn(func(user *client.RemoteUser, msg rpc.RMPrivateMessage, ts time.Time) {
		alicePMChan <- msg.Message
	}))

	// Send an initial Alice->Bob and Bob->Alice message and assert they
	// were received.
	wantMsg := "test PM"
	err = alice.PM(bob.PublicID(), wantMsg)
	assert.NilErr(t, err)
	assert.DeepEqual(t, assert.ChanWritten(t, bobPMChan), wantMsg)
	assert.NilErr(t, bob.PM(alice.PublicID(), "bob msg"))
	assert.DeepEqual(t, assert.ChanWritten(t, alicePMChan), "bob msg")

	// Make sure the clients are fully synced before continuing test.
	assertClientUpToDate(t, alice)
	assertClientUpToDate(t, bob)

	// Setup Alice so that the next message she sends will cause the conn
	// to fail after the message is written but before the server ack is
	// processed.
	alice.preventFutureConns(fmt.Errorf("forced conn failure"))
	alice.conn.startFailing(fmt.Errorf("forced read failure"), nil)
	wantMsg2 := "test PM 2"
	aliceConnClosed := make(chan (struct{}))
	regServerChanged := alice.handleSync(client.OnServerSessionChangedNtfn(func(connected bool, _, _, _ uint64) {
		if !connected {
			select {
			case <-aliceConnClosed:
			default:
				close(aliceConnClosed)
			}
		}
	}))

	// Attempt to send the PM, which will cause an error. The error from
	// the alice.PM() call is only returned once Alice starts the shutdown
	// process.
	pmErrChan := make(chan error, 1)
	go func() { pmErrChan <- alice.PM(bob.PublicID(), wantMsg2) }()
	assert.ChanWritten(t, aliceConnClosed)

	// Still, since the error was _after_ writing the message, Bob
	// should've received it already.
	assert.DeepEqual(t, assert.ChanWritten(t, bobPMChan), wantMsg2)

	// Shutdown and recreate Alice. This should cause the previously written
	// (but unacked) message to be resent (in this case, duplicated).
	alice = ts.recreateClient(alice)
	assert.ChanWritten(t, pmErrChan)

	// Try to send a new message to Bob. Bob should receive it.
	wantMsg3 := "test PM 3"
	err = alice.PM(bob.PublicID(), wantMsg3)
	assert.NilErr(t, err)
	assert.DeepEqual(t, assert.ChanWritten(t, bobPMChan), wantMsg3)

	// Try to send another message to Bob. Bob should receive it. Sending
	// Two messages exercises an old failure scenario.
	wantMsg4 := "test PM 4"
	err = alice.PM(bob.PublicID(), wantMsg4)
	assert.NilErr(t, err)
	assert.DeepEqual(t, assert.ChanWritten(t, bobPMChan), wantMsg4)

	// Start the second stage of the test. We perform the same procedure,
	// but fail the conn just before writing the message to the server.
	//
	// Setup Alice to fail the conn.
	alice.preventFutureConns(fmt.Errorf("forced conn failure"))
	alice.conn.startFailing(nil, fmt.Errorf("forced write failure"))
	aliceConnClosed = make(chan (struct{}))
	regServerChanged.Unregister()
	alice.handleSync(client.OnServerSessionChangedNtfn(func(connected bool, _, _, _ uint64) {
		if !connected {
			select {
			case <-aliceConnClosed:
			default:
				close(aliceConnClosed)
			}
		}
	}))

	// Attempt to send the message and wait until Alice's conn is closed.
	wantMsg5 := "test PM 5"
	go func() { pmErrChan <- alice.PM(bob.PublicID(), wantMsg5) }()
	assert.ChanWritten(t, aliceConnClosed)

	// Shutdown and recreate Alice.
	alice = ts.recreateClient(alice)
	assert.ChanWritten(t, pmErrChan)

	// Bob should receive the message.
	assert.DeepEqual(t, assert.ChanWritten(t, bobPMChan), wantMsg5)

	// Send 2 new messages.
	wantMsg6 := "test PM 6"
	err = alice.PM(bob.PublicID(), wantMsg6)
	assert.NilErr(t, err)
	assert.DeepEqual(t, assert.ChanWritten(t, bobPMChan), wantMsg6)

	wantMsg7 := "test PM 7"
	err = alice.PM(bob.PublicID(), wantMsg7)
	assert.NilErr(t, err)
	assert.DeepEqual(t, assert.ChanWritten(t, bobPMChan), wantMsg7)

	// Finally send a Bob->Alice message.
	alice.NotificationManager().Register(client.OnPMNtfn(func(user *client.RemoteUser, msg rpc.RMPrivateMessage, ts time.Time) {
		alicePMChan <- msg.Message
	}))

	assert.NilErr(t, bob.PM(alice.PublicID(), "bob msg"))
	assert.DeepEqual(t, assert.ChanWritten(t, alicePMChan), "bob msg")
}

// TestLongOfflineClientResetsAllKX tests that if a client has been offline for
// longer than the server's message retention policy, the client attempts to
// reset KX with all its known users.
func TestLongOfflineClientResetsAllKX(t *testing.T) {
	t.Parallel()

	// Setup Alice and Bob and have them KX.
	tcfg := testScaffoldCfg{}
	ts := newTestScaffold(t, tcfg)
	alice := ts.newClient("alice")
	bob := ts.newClient("bob")
	invite, err := alice.WriteNewInvite(io.Discard, nil)
	assert.NilErr(t, err)
	assert.NilErr(t, bob.AcceptInvite(invite))
	assertClientsKXd(t, alice, bob)

	// Shutdown bob.
	bobUID := bob.PublicID()
	ts.stopClient(bob)

	// Modify Alice's DB directly marking its last conn time in the past.
	err = alice.db.Update(context.Background(), func(tx clientdb.ReadWriteTx) error {
		oldDate := time.Now().Add(-time.Hour * 24 * 365)
		_, err := alice.db.ReplaceLastConnDate(tx, oldDate)
		return err
	})
	assert.NilErr(t, err)

	// Recreate alice.
	alice = ts.recreateClient(alice)

	// It should end up with a new reset KX attempt.
	for i := 0; i < 20; i++ {
		kx, err := alice.ListKXs()
		assert.NilErr(t, err)
		if len(kx) == 0 {
			time.Sleep(time.Millisecond * 100)
			continue
		}

		if !kx[0].IsForReset {
			t.Fatalf("KX is not for reset")
		}
		if !kx[0].MediatorID.ConstantTimeEq(&bobUID) {
			t.Fatalf("KX is not for Bob")
		}
		return // Test done!
	}

	t.Fatalf("Timeout waiting for Bob's reset KX to appear")
}

// TestRemoteOfflineMsgs ensures that attempting to send messages while the
// remote peer is offline works.
func TestRemoteOfflineMsgs(t *testing.T) {
	t.Parallel()

	tcfg := testScaffoldCfg{}
	ts := newTestScaffold(t, tcfg)
	alice := ts.newClient("alice")
	bob := ts.newClient("bob")

	// Baseline test.
	ts.kxUsers(alice, bob)
	assertClientsCanPM(t, alice, bob)

	// Hook and helper to check on Alice connected state.
	aliceSess := make(chan bool, 3)
	alice.handle(client.OnServerSessionChangedNtfn(func(connected bool, _, _, _ uint64) {
		aliceSess <- connected
	}))
	assertAliceSess := func(wantSess bool) {
		t.Helper()
		assert.ChanWrittenWithVal(t, aliceSess, wantSess)
	}

	// Send a pm from alice. This advances the ratchet and is used to test
	// an old bug scenario where alice would listen on incorrect RV points.
	assert.NilErr(t, alice.PM(bob.PublicID(), ""))

	// Let alice go offline.
	alice.RemainOffline()
	assertAliceSess(false)

	// Hook into Alice's PM events for next set of tests.
	alicePMs := make(chan string, 10)
	alice.handle(client.OnPMNtfn(func(ru *client.RemoteUser, pm rpc.RMPrivateMessage, ts time.Time) {
		alicePMs <- pm.Message
	}))

	// Send PMs from bob.
	nbMsgs := 5
	testMsg := "bob msg while alice offline"
	for i := 0; i < nbMsgs; i++ {
		assert.NilErr(t, bob.PM(alice.PublicID(), testMsg))
	}

	// Alice does _not_ get them.
	assert.ChanNotWritten(t, alicePMs, time.Second)

	// Alice goes online.
	alice.GoOnline()
	assertAliceSess(true)

	// Alice gets the messages.
	for i := 0; i < nbMsgs; i++ {
		assert.ChanWrittenWithVal(t, alicePMs, testMsg)
	}
}

// TestLocalOfflineMsgs ensures that attempting to send messages while local
// client is offline works.
func TestLocalOfflineMsgs(t *testing.T) {
	t.Parallel()

	tcfg := testScaffoldCfg{}
	ts := newTestScaffold(t, tcfg)
	alice := ts.newClient("alice")
	bob := ts.newClient("bob")

	// Baseline test.
	ts.kxUsers(alice, bob)
	assertClientsCanPM(t, alice, bob)

	// Hook and helper to check on Alice connected state.
	aliceSess := make(chan bool, 3)
	alice.handle(client.OnServerSessionChangedNtfn(func(connected bool, _, _, _ uint64) {
		aliceSess <- connected
	}))
	assertAliceSess := func(wantSess bool) {
		t.Helper()
		assert.ChanWrittenWithVal(t, aliceSess, wantSess)
	}

	// Let Alice go offline.
	alice.RemainOffline()
	assertAliceSess(false)

	// Hook into Bob's PM events for next set of tests.
	bobPMs := make(chan string, 10)
	bob.handle(client.OnPMNtfn(func(ru *client.RemoteUser, pm rpc.RMPrivateMessage, ts time.Time) {
		bobPMs <- pm.Message
	}))

	// Send PMs from Alice.
	nbMsgs := 5
	testMsg := "alice msg while alice offline"
	errChan := make(chan error, nbMsgs+1)
	for i := 0; i < nbMsgs; i++ {
		go func() {
			errChan <- alice.PM(bob.PublicID(), testMsg)
		}()
	}

	// Messages are not sent yet.
	assert.ChanNotWritten(t, errChan, time.Second)

	// Bob does _not_ get them.
	assert.ChanNotWritten(t, bobPMs, time.Second)

	// Alice goes online.
	alice.GoOnline()
	assertAliceSess(true)

	// Alice sends the messages and Bob gets them.
	for i := 0; i < nbMsgs; i++ {
		assert.NilErrFromChan(t, errChan)
		assert.ChanWrittenWithVal(t, bobPMs, testMsg)
	}
}

// TestPrepaidInvites asserts that using prepaid invites works.
func TestPrepaidInvites(t *testing.T) {
	t.Parallel()

	tcfg := testScaffoldCfg{}
	ts := newTestScaffold(t, tcfg)
	alice := ts.newClient("alice")
	bob := ts.newClient("bob")

	// Create the paid invite on Alice.
	srcInvite := bytes.NewBuffer(nil)
	_, srcPik, err := alice.CreatePrepaidInvite(srcInvite, nil)
	assert.NilErr(t, err)

	// Assert going from->to the string encoding of the paid invite key
	// works.
	srcPikEncoded, err := srcPik.Encode()
	assert.NilErr(t, err)
	dstPik, err := clientintf.DecodePaidInviteKey(srcPikEncoded)
	assert.NilErr(t, err)

	// Fetch the paid invite on Bob.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	dstInvite := bytes.NewBuffer(nil)
	decodedInvite, err := bob.FetchPrepaidInvite(ctx, dstPik, dstInvite)
	assert.NilErr(t, err)

	// Assert both are equal.
	if !bytes.Equal(srcInvite.Bytes(), dstInvite.Bytes()) {
		t.Fatal("source and dest invites are not equal")
	}

	// Attempt to KX using the fetched invite.
	assert.NilErr(t, bob.AcceptInvite(decodedInvite))
	assertClientsKXd(t, alice, bob)
}

// TestHandshakesIdleClients asserts that if a client goes too long without
// being reached, an automatic handshake attempt is made.
func TestHandshakesIdleClients(t *testing.T) {
	t.Parallel()

	tcfg := testScaffoldCfg{}
	ts := newTestScaffold(t, tcfg)
	alice := ts.newClient("alice")
	bob := ts.newClient("bob")

	ts.kxUsers(alice, bob)

	aliceHandshaked := make(chan struct{}, 3)
	alice.handle(client.OnHandshakeStageNtfn(func(ru *client.RemoteUser, msgType string) {
		if ru.ID() != bob.PublicID() {
			return
		}
		if msgType == "SYNACK" {
			aliceHandshaked <- struct{}{}
		}
	}))
	bobHandshaked := make(chan struct{}, 3)
	bob.handle(client.OnHandshakeStageNtfn(func(ru *client.RemoteUser, msgType string) {
		if ru.ID() != alice.PublicID() {
			return
		}
		if msgType == "ACK" {
			bobHandshaked <- struct{}{}
		}
	}))

	// Wait halfway through the time for an autohandshake interval. It should
	// NOT trigger an automatic handshake.
	time.Sleep(alice.cfg.AutoHandshakeInterval / 2)
	assertGoesOffline(t, alice)
	assertGoesOnline(t, alice)
	assert.ChanNotWritten(t, aliceHandshaked, time.Second)

	// Wait until the timeout for sending a handshake on startup elapses.
	time.Sleep(alice.cfg.AutoHandshakeInterval / 2)

	// Flick Alice's connection. This should trigger an automatic handshake
	// attempt.
	assertGoesOffline(t, alice)
	assertGoesOnline(t, alice)
	assert.ChanWritten(t, aliceHandshaked)
	assert.ChanWritten(t, bobHandshaked)
}

// TestUnsubsIdleClients asserts that idle clients are unsubscribed and removed
// from GCs after they become idle.
func TestUnsubsIdleClients(t *testing.T) {
	t.Parallel()

	tcfg := testScaffoldCfg{}
	ts := newTestScaffold(t, tcfg)
	alice := ts.newClient("alice")
	bob := ts.newClient("bob")

	ts.kxUsers(alice, bob)

	alicePostSub := make(chan bool, 3)
	alice.handle(client.OnPostSubscriberUpdated(func(ru *client.RemoteUser, subscribed bool) {
		if ru.ID() != bob.PublicID() {
			return
		}
		alicePostSub <- subscribed
	}))

	aliceUnsubbing := make(chan struct{}, 3)
	alice.handle(client.OnUnsubscribingIdleRemoteClient(func(ru *client.RemoteUser, lastDecTime time.Time) {
		if ru.ID() != bob.PublicID() {
			return
		}
		aliceUnsubbing <- struct{}{}
	}))

	bobPostSub := make(chan bool, 3)
	bob.handle(client.OnRemoteSubscriptionChangedNtfn(func(ru *client.RemoteUser, subscribed bool) {
		if ru.ID() != alice.PublicID() {
			return
		}
		bobPostSub <- subscribed
	}))

	aliceKicked := make(chan struct{}, 3)
	alice.handle(client.OnGCUserPartedNtfn(func(gcid client.GCID, uid client.UserID, reason string, kicked bool) {
		if uid != bob.PublicID() {
			return
		}
		aliceKicked <- struct{}{}
	}))

	bobKicked := make(chan struct{}, 3)
	bob.handle(client.OnGCUserPartedNtfn(func(gcid client.GCID, uid client.UserID, reason string, kicked bool) {
		if uid != bob.PublicID() {
			return
		}
		bobKicked <- struct{}{}
	}))

	// Bob subscribes to Alice's post.
	assert.NilErr(t, bob.SubscribeToPosts(alice.PublicID()))
	assert.ChanWrittenWithVal(t, alicePostSub, true)
	assert.ChanWrittenWithVal(t, bobPostSub, true)

	// Bob joins Alice's GC.
	gcid, err := alice.NewGroupChat("testgc")
	assert.NilErr(t, err)
	assertClientJoinsGC(t, gcid, alice, bob)

	// Bob goes idle.
	assertGoesOffline(t, bob)

	// Wait until the alice will send a handshake attempt.
	start := time.Now()
	time.Sleep(alice.cfg.AutoHandshakeInterval)

	// Flick alice. She sends the handshake but does not yet perform the
	// unsub.
	assertGoesOffline(t, alice)
	assertGoesOnline(t, alice)
	assert.ChanNotWritten(t, aliceUnsubbing, time.Second)

	// Wait for the interval of idle delay unsub to elapse.
	time.Sleep(alice.cfg.AutoRemoveIdleUsersInterval - time.Since(start))

	// Flick Alice. Bob should be unsubbed and kicked from GC.
	assertGoesOffline(t, alice)
	assertGoesOnline(t, alice)
	assert.ChanWritten(t, aliceUnsubbing)
	assert.ChanWrittenWithVal(t, alicePostSub, false)
	assert.ChanWritten(t, aliceKicked)

	// Bob comes online. It should have been unsubbed and kicked from GC.
	assertGoesOnline(t, bob)
	assert.ChanWrittenWithVal(t, bobPostSub, false)
	assert.ChanWritten(t, bobKicked)
}
