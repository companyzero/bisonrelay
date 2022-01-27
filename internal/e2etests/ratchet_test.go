package e2etests

import (
	"context"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/companyzero/bisonrelay/client"
	"github.com/companyzero/bisonrelay/client/clientdb"
	"github.com/companyzero/bisonrelay/internal/assert"
	"github.com/companyzero/bisonrelay/rpc"
)

// TestResendsUnackedRM tests shutting down the client while there are
// unacknowledged RMs inflight works as expected and does not cause a busted
// ratchet.
func TestResendsUnackedRM(t *testing.T) {
	// Setup Alice and Bob and have them KX.
	tcfg := testScaffoldCfg{}
	ts := newTestScaffold(t, tcfg)
	alice := ts.newClient("alice")
	bob := ts.newClient("bob")
	invite, err := alice.WriteNewInvite(io.Discard)
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
	alice.modifyHandlers(func() {
		alice.onConnChanged = func(connected bool, pushRate, subRate uint64) {
			if !connected {
				select {
				case <-aliceConnClosed:
				default:
					close(aliceConnClosed)
				}
			}
		}
	})

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
	alice.modifyHandlers(func() {
		alice.onConnChanged = func(connected bool, pushRate, subRate uint64) {
			if !connected {
				select {
				case <-aliceConnClosed:
				default:
					close(aliceConnClosed)
				}
			}
		}
	})

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
	// Setup Alice and Bob and have them KX.
	tcfg := testScaffoldCfg{}
	ts := newTestScaffold(t, tcfg)
	alice := ts.newClient("alice")
	bob := ts.newClient("bob")
	invite, err := alice.WriteNewInvite(io.Discard)
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
