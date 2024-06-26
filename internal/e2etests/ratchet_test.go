package e2etests

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/rand"
	"testing"
	"time"

	"github.com/companyzero/bisonrelay/client"
	"github.com/companyzero/bisonrelay/client/clientdb"
	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/companyzero/bisonrelay/internal/assert"
	"github.com/companyzero/bisonrelay/rpc"
	"github.com/companyzero/bisonrelay/zkidentity"
	"golang.org/x/text/collate"
	"golang.org/x/text/language"
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
	regServerChanged := alice.handleSync(client.OnServerSessionChangedNtfn(func(connected bool, _ clientintf.ServerPolicy) {
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
	alice.handleSync(client.OnServerSessionChangedNtfn(func(connected bool, _ clientintf.ServerPolicy) {
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
	alice.handle(client.OnServerSessionChangedNtfn(func(connected bool, _ clientintf.ServerPolicy) {
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
	alice.handle(client.OnServerSessionChangedNtfn(func(connected bool, _ clientintf.ServerPolicy) {
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

// TestSuggestKX tests that the suggest kx works.
func TestSuggestKX(t *testing.T) {
	t.Parallel()

	tcfg := testScaffoldCfg{}
	ts := newTestScaffold(t, tcfg)
	alice := ts.newClient("alice")
	bob := ts.newClient("bob")
	charlie := ts.newClient("charlie")

	ts.kxUsers(alice, bob)
	ts.kxUsers(alice, charlie)

	bobSuggestKxChan := make(chan clientintf.UserID, 5)
	bob.handle(client.OnKXSuggested(func(ru *client.RemoteUser, target zkidentity.PublicIdentity) {
		bobSuggestKxChan <- target.Identity
	}))

	// Suggest KX with unknown user.
	alice.SuggestKX(bob.PublicID(), charlie.PublicID())
	assert.ChanWrittenWithVal(t, bobSuggestKxChan, charlie.PublicID())

	// Suggest KX with known user.
	ts.kxUsers(bob, charlie)
	alice.SuggestKX(bob.PublicID(), charlie.PublicID())
	assert.ChanWrittenWithVal(t, bobSuggestKxChan, charlie.PublicID())
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

// TestDisabledAutoUnsubIdle asserts that idle clients are NOT unsubbed when
// the options to disable autounsub or autohandshake are set.
func TestDisabledAutoUnsubIdle(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		opts []newClientOpt
	}{{
		name: "no auto unsub",
		opts: []newClientOpt{withDisableAutoUnsubIdle()},
	}, {
		name: "no auto handshake",
		opts: []newClientOpt{withDisableAutoHandshake()},
	}}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tcfg := testScaffoldCfg{}
			ts := newTestScaffold(t, tcfg)
			alice := ts.newClient("alice", tc.opts...)
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

			// Wait for the interval of idle delay unsub to elapse.
			time.Sleep(defaultAutoUnsubIdleUserInterval + time.Second)

			// Flick Alice. Bob should not be unsubbed or kicked
			// from GC.
			assertGoesOffline(t, alice)
			assertGoesOnline(t, alice)
			assert.ChanNotWritten(t, aliceUnsubbing, time.Second)
			assert.ChanNotWritten(t, aliceKicked, time.Second)
		})
	}
}

// TestUnsubsIdleClientsAfterResetBug tests an old bug where an user was
// autounsubbed after a reset.
func TestUnsubsIdleClientsAfterResetBug(t *testing.T) {
	t.Parallel()

	tcfg := testScaffoldCfg{}
	ts := newTestScaffold(t, tcfg)
	alice := ts.newClient("alice")
	bob := ts.newClient("bob", withDisableAutoUnsubIdle(),
		withDisableAutoHandshake())

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
	aliceHandshake := make(chan string, 3)
	alice.handle(client.OnHandshakeStageNtfn(func(ru *client.RemoteUser, msgType string) {
		aliceHandshake <- msgType
	}))
	aliceKxCompleted := make(chan struct{}, 3)
	alice.handle(client.OnKXCompleted(func(_ *clientintf.RawRVID, ru *client.RemoteUser, isNew bool) {
		aliceKxCompleted <- struct{}{}
	}))
	aliceSYNSent := make(chan struct{}, 3)
	alice.handle(client.OnRMSent(func(ru *client.RemoteUser, p interface{}) {
		if _, ok := p.(rpc.RMHandshakeSYN); ok {
			aliceSYNSent <- struct{}{}
		}
	}))

	bobKicked := make(chan struct{}, 3)
	bob.handle(client.OnGCUserPartedNtfn(func(gcid client.GCID, uid client.UserID, reason string, kicked bool) {
		if uid != bob.PublicID() {
			return
		}
		bobKicked <- struct{}{}
	}))
	bobHandshake := make(chan string, 3)
	bob.handle(client.OnHandshakeStageNtfn(func(ru *client.RemoteUser, msgType string) {
		bobHandshake <- msgType
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
	assert.ChanWritten(t, aliceSYNSent)
	assert.ChanNotWritten(t, aliceUnsubbing, time.Second)

	// Bob comes back online and responds to the handshake. He also resets
	// the ratchet, which zeros the last decryption time.
	assertGoesOnline(t, bob)
	assert.ChanWrittenWithVal(t, bobHandshake, "SYN")
	assert.ChanWrittenWithVal(t, aliceHandshake, "SYNACK")
	assert.ChanWrittenWithVal(t, bobHandshake, "ACK")
	assert.NilErr(t, bob.ResetRatchet(alice.PublicID()))
	assert.ChanWritten(t, aliceKxCompleted)
	resetTime := time.Now()

	// Bob goes offline again.
	assertGoesOffline(t, bob)

	// The time from the last handshake sent by alice and now should not yet
	// cause the unsub to happen. This is because the reset should have
	// also reset the autohandshake inverval.
	time.Sleep(alice.cfg.AutoRemoveIdleUsersInterval - time.Since(start))
	assertGoesOffline(t, alice)
	assertGoesOnline(t, alice)
	assert.ChanNotWritten(t, aliceUnsubbing, time.Second)

	// Wait until alice should've sent the handshake attempt and flick her.
	time.Sleep(alice.cfg.AutoHandshakeInterval - time.Since(resetTime))
	assertGoesOffline(t, alice)
	assertGoesOnline(t, alice)
	assert.ChanWritten(t, aliceSYNSent)

	// Finally, wait until the autoremove _should_ be sent and verify all
	// unsubs were done.
	time.Sleep(alice.cfg.AutoRemoveIdleUsersInterval - time.Since(resetTime))
	assertGoesOffline(t, alice)
	assertGoesOnline(t, alice)
	assert.ChanWritten(t, aliceUnsubbing)
	assert.ChanWrittenWithVal(t, alicePostSub, false)
	assert.ChanWritten(t, aliceKicked)
}

// TestUnsubsIdleClientsWithHandshakeAttempt tests an old bug where an user was
// autounsubbed due to their recorded handshake attempt being older than
// the introduction of commit 15690ddfac057bd2ece38b110ba559d7277c2663.
func TestUnsubsIdleClientsWithHandshakeAttempt(t *testing.T) {
	t.Parallel()

	tcfg := testScaffoldCfg{}
	ts := newTestScaffold(t, tcfg)
	alice := ts.newClient("alice")
	bob := ts.newClient("bob", withDisableAutoUnsubIdle(),
		withDisableAutoHandshake())
	ts.kxUsers(alice, bob)

	aliceUnsubbing := make(chan struct{}, 3)
	alice.handle(client.OnUnsubscribingIdleRemoteClient(func(ru *client.RemoteUser, lastDecTime time.Time) {
		if ru.ID() != bob.PublicID() {
			return
		}
		aliceUnsubbing <- struct{}{}
	}))
	aliceSYNSent := make(chan struct{}, 3)
	alice.handle(client.OnRMSent(func(ru *client.RemoteUser, p interface{}) {
		if _, ok := p.(rpc.RMHandshakeSYN); ok {
			aliceSYNSent <- struct{}{}
		}
	}))

	// Bob subscribes to Alice's post.
	assertSubscribeToPosts(t, alice, bob)

	// Bob goes idle.
	assertGoesOffline(t, bob)

	// Manually change Alice's last recorded handshake attempt to Bob to
	// a date in the past.
	//
	// After this, the state within Alice will be that the last handshake
	// attempt is older than the last decryption time (which should not
	// happen anymore but can still happen for clients that were online
	// before 15690ddfa).
	err := alice.db.Update(context.Background(), func(tx clientdb.ReadWriteTx) error {
		entry, err := alice.db.GetAddressBookEntry(tx, bob.PublicID())
		if err != nil {
			return err
		}

		entry.LastHandshakeAttempt = time.Date(2024, 4, 18, 12, 00, 00, 00, time.UTC)
		return alice.db.UpdateAddressBookEntry(tx, entry)
	})
	assert.NilErr(t, err)

	// Wait until Alice should both send a handshake attempt and auto
	// unsub for already idle users.
	time.Sleep(alice.cfg.AutoRemoveIdleUsersInterval)

	// Flick alice. Before the bug was fixed, this would cause the
	// handshake to NOT be sent and the auto-unsub to happen. After the bug
	// is fixed, she sends the handshake but does not yet perform the
	// unsub.
	assertGoesOffline(t, alice)
	assertGoesOnline(t, alice)
	assert.ChanNotWritten(t, aliceUnsubbing, time.Second)
	assert.ChanWritten(t, aliceSYNSent)

	// Wait the additional handshake time. This should NOT cause an
	// additional handshake and SHOULD cause the autounsub.
	time.Sleep(alice.cfg.AutoHandshakeInterval)
	assertGoesOffline(t, alice)
	assertGoesOnline(t, alice)
	assert.ChanNotWritten(t, aliceSYNSent, time.Second)
	assert.ChanWritten(t, aliceUnsubbing)
}

// TestUserNickAlias performs tests around duplicate and aliased users.
func TestUserNickAlias(t *testing.T) {
	t.Parallel()

	// The Bobs initialized in the test need to be ordered by id. Use an
	// rng with a fixed seed to generate them, to ensure the test is not
	// stuck generating the Bobs.
	const seed = 0x10000
	rng := rand.New(rand.NewSource(seed))

	tcfg := testScaffoldCfg{}
	ts := newTestScaffold(t, tcfg)
	alice := ts.newClient("alice")

	// First bob.
	bob1 := ts.newClient("bob", withID(mustNewIDFromRNG("bob", rng)))
	t.Logf("bob1: %s", bob1.PublicID())
	ts.kxUsers(alice, bob1)
	assertUserNick(t, alice, bob1, "bob")

	// Second bob. Create in a loop until the ID for bob2 is < the id for
	// bob1.
	bob2 := ts.newClient("bob", withID(mustNewIDFromRNG("bob", rng)))
	for publicIDIsLess(bob1, bob2) {
		bob2 = ts.newClient("bob", withID(mustNewIDFromRNG("bob", rng)))
	}
	t.Logf("bob2: %s", bob2.PublicID())

	ts.kxUsers(alice, bob2)
	assertUserNick(t, alice, bob2, "bob_2")

	// Third bob. Create in a loop until the ID for bob3 is > the id for bob1.
	bob3 := ts.newClient("bob", withID(mustNewIDFromRNG("bob", rng)))
	for publicIDIsLess(bob3, bob1) {
		bob3 = ts.newClient("bob", withID(mustNewIDFromRNG("bob", rng)))
	}
	t.Logf("bob3: %s", bob3.PublicID())

	ts.kxUsers(alice, bob3)
	assertUserNick(t, alice, bob3, "bob_3")

	// Dupe alice. Given this is the same nick, the first Alice will see
	// it as alice_2.
	alice2 := ts.newClient("alice")
	ts.kxUsers(alice, alice2)
	assertUserNick(t, alice, alice2, "alice_2")

	// Create bob4 with a case sensitive change. The default collator
	// is case-sensitive.
	bob4 := ts.newClient("BOB", withID(mustNewIDFromRNG("BOB", rng)))
	for publicIDIsLess(bob4, bob3) {
		bob4 = ts.newClient("BOB", withID(mustNewIDFromRNG("BOB", rng)))
	}
	t.Logf("bob4: %s", bob4.PublicID())
	ts.kxUsers(alice, bob4)
	assertUserNick(t, alice, bob4, "BOB")

	bob5 := ts.newClient("böb", withID(mustNewIDFromRNG("böb", rng)))
	for publicIDIsLess(bob5, bob4) {
		bob5 = ts.newClient("böb", withID(mustNewIDFromRNG("böb", rng)))
	}
	t.Logf("bob5: %s", bob5.PublicID())
	ts.kxUsers(alice, bob5)
	assertUserNick(t, alice, bob5, "böb")

	// Restart alice. Bobs should be the same.
	alice = ts.recreateClient(alice)
	assertUserNick(t, alice, bob1, "bob")
	assertUserNick(t, alice, bob2, "bob_2")
	assertUserNick(t, alice, bob3, "bob_3")
	assertUserNick(t, alice, bob4, "BOB")
	assertUserNick(t, alice, bob5, "böb")

	// Rename bob_2 to bob2_renamed and assert.
	assert.NilErr(t, alice.RenameUser(bob2.PublicID(), "bob2_renamed"))
	assertUserNick(t, alice, bob1, "bob")
	assertUserNick(t, alice, bob2, "bob2_renamed")
	assertUserNick(t, alice, bob3, "bob_3")
	assertUserNick(t, alice, bob4, "BOB")
	assertUserNick(t, alice, bob5, "böb")
	alice = ts.recreateClient(alice)
	assertUserNick(t, alice, bob1, "bob")
	assertUserNick(t, alice, bob2, "bob2_renamed")
	assertUserNick(t, alice, bob3, "bob_3")
	assertUserNick(t, alice, bob4, "BOB")
	assertUserNick(t, alice, bob5, "böb")

	// Manually (locally) rename bob2 to bob2_localrename. This requires
	// recreating the client because the local identity is only loaded at
	// startup. Then reset KX so that the modified ID goes to alice.
	err := bob2.db.Update(context.Background(), func(tx clientdb.ReadWriteTx) error {
		id, err := bob2.db.LocalID(tx)
		if err != nil {
			return err
		}
		id.Public.Nick = "bob2_localrename"
		return bob2.db.UpdateLocalID(tx, id)
	})
	assert.NilErr(t, err)
	bob2 = ts.recreateClient(bob2)
	assert.DeepEqual(t, bob2.LocalNick(), "bob2_localrename")
	assertKXReset(t, alice, bob2)

	// Alice still sees the same nick.
	assertUserNick(t, alice, bob1, "bob")
	assertUserNick(t, alice, bob2, "bob2_renamed")
	assertUserNick(t, alice, bob3, "bob_3")
	assertUserNick(t, alice, bob4, "BOB")
	assertUserNick(t, alice, bob5, "böb")
	alice = ts.recreateClient(alice)
	assertUserNick(t, alice, bob1, "bob")
	assertUserNick(t, alice, bob2, "bob2_renamed")
	assertUserNick(t, alice, bob3, "bob_3")
	assertUserNick(t, alice, bob4, "BOB")
	assertUserNick(t, alice, bob5, "böb")

	// Manually (locally) rename bob1 to bob1_localrename. This requires
	// recreating the client because the local identity is only loaded at
	// startup. Then reset KX so that the modified ID goes to alice.
	err = bob1.db.Update(context.Background(), func(tx clientdb.ReadWriteTx) error {
		id, err := bob1.db.LocalID(tx)
		if err != nil {
			return err
		}
		id.Public.Nick = "bob1_localrename"
		return bob1.db.UpdateLocalID(tx, id)
	})
	assert.NilErr(t, err)
	bob1 = ts.recreateClient(bob1)
	assert.DeepEqual(t, bob1.LocalNick(), "bob1_localrename")
	assertKXReset(t, alice, bob1)

	// Alice still sees the same nick.
	assertUserNick(t, alice, bob1, "bob")
	assertUserNick(t, alice, bob2, "bob2_renamed")
	assertUserNick(t, alice, bob3, "bob_3")
	assertUserNick(t, alice, bob4, "BOB")
	assertUserNick(t, alice, bob5, "böb")
	alice = ts.recreateClient(alice)
	assertUserNick(t, alice, bob1, "bob")
	assertUserNick(t, alice, bob2, "bob2_renamed")
	assertUserNick(t, alice, bob3, "bob_3")
	assertUserNick(t, alice, bob4, "BOB")
	assertUserNick(t, alice, bob5, "böb")

	// Recreate Alice with a new loose (case-insensitive,
	// diacritic-insensitive) collator. bob4 ("BOB") will now be BOB_2,
	// böb will be böb_4.
	collator := collate.New(language.Afrikaans, collate.Loose)
	alice = ts.recreateClient(alice, withCollator(collator))
	assertUserNick(t, alice, bob1, "bob")
	assertUserNick(t, alice, bob2, "bob2_renamed")
	assertUserNick(t, alice, bob3, "bob_3")
	assertUserNick(t, alice, bob4, "BOB_2")
	assertUserNick(t, alice, bob5, "böb_4")
}

func TestUpdateProfileAvatar(t *testing.T) {
	t.Parallel()

	tcfg := testScaffoldCfg{}
	ts := newTestScaffold(t, tcfg)
	alice := ts.newClient("alice")

	// Initial avatar is empty.
	bob := ts.newClient("bob")
	ts.kxUsers(alice, bob)
	assertUserAvatar(t, bob, alice, nil)

	avatarUpdateChan := make(chan []byte, 5)
	bob.handle(client.OnProfileUpdated(func(ru *client.RemoteUser,
		ab *clientdb.AddressBookEntry, _ []client.ProfileUpdateField) {
		avatarUpdateChan <- ab.ID.Avatar
	}))

	avatar1 := bytes.Repeat([]byte{0xa1}, 16)
	assert.NilErr(t, alice.UpdateLocalAvatar(avatar1))
	assert.ChanWrittenWithVal(t, avatarUpdateChan, avatar1)
	assertUserAvatar(t, bob, alice, avatar1)

	// Ensure avatar was saved.
	bob = ts.recreateClient(bob)
	assertUserAvatar(t, bob, alice, avatar1)

	// Ensure Alice's avatar was saved.
	alice = ts.recreateClient(alice)
	gotAvatar := alice.Public().Avatar
	assert.DeepEqual(t, gotAvatar, avatar1)

	// Ensure avatar can be cleared.
	bob.handle(client.OnProfileUpdated(func(ru *client.RemoteUser,
		ab *clientdb.AddressBookEntry, _ []client.ProfileUpdateField) {
		avatarUpdateChan <- ab.ID.Avatar
	}))
	assert.NilErr(t, alice.UpdateLocalAvatar(nil))
	assert.ChanWrittenWithVal(t, avatarUpdateChan, nil)
	bob = ts.recreateClient(bob)
	assertUserAvatar(t, bob, alice, nil)
}
