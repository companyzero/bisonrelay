package e2etests

import (
	"bytes"
	"crypto/rand"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/companyzero/bisonrelay/client"
	"github.com/companyzero/bisonrelay/client/clientdb"
	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/companyzero/bisonrelay/internal/assert"
	"github.com/companyzero/bisonrelay/rpc"
	rtdtclient "github.com/companyzero/bisonrelay/rtdt/client"
	"github.com/companyzero/bisonrelay/zkidentity"
	"golang.org/x/exp/slices"
)

// TestRTDTSession tests the basic workings of a C2C RTDT session.
func TestRTDTSession(t *testing.T) {
	t.Parallel()

	// Handler for inbound RTDT random data (needs to be setup on client
	// creation).
	gotAliceDataChan := make(chan []byte, 5)
	aliceRandomHandler := func(sess *rtdtclient.Session, enc *rpc.RTDTFramedPacket, plain *rpc.RTDTDataPacket) error {
		gotAliceDataChan <- slices.Clone(plain.Data)
		return nil
	}
	gotBobDataChan := make(chan []byte, 5)
	bobRandomHandler := func(sess *rtdtclient.Session, enc *rpc.RTDTFramedPacket, plain *rpc.RTDTDataPacket) error {
		gotBobDataChan <- slices.Clone(plain.Data)
		return nil
	}

	// Setup Alice and Bob and have them KX.
	tcfg := testScaffoldCfg{runRtdtServer: true}
	ts := newTestScaffold(t, tcfg)
	alice := ts.newClient("alice", withRTDTRandomStreamHandler(aliceRandomHandler))
	bob := ts.newClient("bob", withRTDTRandomStreamHandler(bobRandomHandler))
	ts.kxUsers(alice, bob)

	// Handlers.
	aliceAcceptedChan := make(chan bool, 5)
	alice.handle(client.OnRTDTSessionInviteAccepted(func(ru *client.RemoteUser, sessID zkidentity.ShortID, asPublisher bool) {
		if ru.ID() == bob.PublicID() {
			aliceAcceptedChan <- asPublisher
		}
	}))
	aliceUpdateChan := make(chan *client.RTDTSessionUpdateNtfn, 5)
	alice.handle(client.OnRTDTSesssionUpdated(func(ru *client.RemoteUser, update *client.RTDTSessionUpdateNtfn) {
		aliceUpdateChan <- update
	}))

	bobInviteChan := make(chan *rpc.RMRTDTSessionInvite, 2)
	bob.handle(client.OnInvitedToRTDTSession(func(ru *client.RemoteUser, invite *rpc.RMRTDTSessionInvite, ts time.Time) {
		bobInviteChan <- invite
	}))
	bobUpdateChan := make(chan *client.RTDTSessionUpdateNtfn, 5)
	bob.handle(client.OnRTDTSesssionUpdated(func(ru *client.RemoteUser, update *client.RTDTSessionUpdateNtfn) {
		bobUpdateChan <- update
	}))

	// Create the RTDT session.
	initialSess, err := alice.CreateRTDTSession(2, "test session")
	assert.NilErr(t, err)
	sessRV := initialSess.Metadata.RV
	assert.NotNil(t, initialSess.PublisherKey)
	assert.DeepEqual(t, initialSess.Metadata.Publishers[0].PublisherKey, *initialSess.PublisherKey)

	// Alice invites bob to session.
	err = alice.InviteToRTDTSession(sessRV, true, bob.PublicID())
	assert.NilErr(t, err)

	// Bob Will receive and accept the invite.
	gotBobInvite := assert.ChanWritten(t, bobInviteChan)
	assert.DeepEqual(t, gotBobInvite.RV, sessRV)
	assert.DeepEqual(t, gotBobInvite.AllowedAsPublisher, true)
	if gotBobInvite.PeerID == initialSess.LocalPeerID {
		t.Fatalf("Both users got the same peer id")
	}
	err = bob.AcceptRTDTSessionInvite(alice.PublicID(), gotBobInvite, true)
	assert.NilErr(t, err)
	assert.ChanWrittenWithVal(t, aliceAcceptedChan, true)
	gotAliceUpdate := assert.ChanWritten(t, aliceUpdateChan)
	assert.DeepEqual(t, len(gotAliceUpdate.NewPublishers), 1)
	assert.DeepEqual(t, gotAliceUpdate.NewPublishers[0].PeerID, gotBobInvite.PeerID)
	gotBobUpdate := assert.ChanWritten(t, bobUpdateChan)
	assert.DeepEqual(t, len(gotBobUpdate.NewPublishers), 2)
	assert.DeepEqual(t, gotBobUpdate.NewPublishers[0].PeerID, initialSess.LocalPeerID)
	assert.DeepEqual(t, gotBobUpdate.NewPublishers[0].PublisherKey, *initialSess.PublisherKey)
	assert.DeepEqual(t, gotBobUpdate.NewPublishers[1].PeerID, gotBobInvite.PeerID)
	assert.DeepEqual(t, gotBobUpdate.NewPublishers[1].PeerID, initialSess.LocalPeerID+1) // 2-way session

	// Double check the publisher key we got from bob actually is the one
	// he recorded himself.
	bobSess, err := bob.GetRTDTSession(&sessRV)
	assert.NilErr(t, err)
	assert.DeepEqual(t, gotBobUpdate.NewPublishers[1].PublisherKey, *bobSess.PublisherKey)

	// Now that the high level session is created C2C, join it on the rtdt
	// server.
	aliceJoinedChan := make(chan struct{}, 5)
	alice.handle(client.OnRTDTLiveSessionJoined(func(joinedSessRV zkidentity.ShortID) {
		if joinedSessRV == sessRV {
			aliceJoinedChan <- struct{}{}
		}
	}))
	bobJoinedChan := make(chan struct{}, 5)
	bob.handle(client.OnRTDTLiveSessionJoined(func(joinedSessRV zkidentity.ShortID) {
		if joinedSessRV == sessRV {
			bobJoinedChan <- struct{}{}
		}
	}))
	assert.NilErr(t, alice.JoinLiveRTDTSession(sessRV))
	assert.ChanWritten(t, aliceJoinedChan)
	assert.NilErr(t, bob.JoinLiveRTDTSession(sessRV))
	assert.ChanWritten(t, bobJoinedChan)

	// Finally, send some random data through RTDT from each peer. The
	// other one must receive it.
	rtSessAlice, rtSessBob := alice.GetLiveRTSession(&sessRV).RTSess, bob.GetLiveRTSession(&sessRV).RTSess
	assert.NotNil(t, rtSessAlice)
	assert.NotNil(t, rtSessBob)
	aliceData, bobData := []byte("from alice"), []byte("from bob")
	err = rtSessAlice.SendRandomData(ts.ctx, aliceData, 1000)
	assert.NilErr(t, err)
	assert.ChanWrittenWithVal(t, gotBobDataChan, aliceData)
	err = rtSessBob.SendRandomData(ts.ctx, bobData, 1000)
	assert.NilErr(t, err)
	assert.ChanWrittenWithVal(t, gotAliceDataChan, bobData)

	// Send so much that that it triggers a refresh of the send allowance.
	// The test allowance on the free payment scheme is 1 MB.
	aliceRefreshedAllowance := make(chan struct{}, 10)
	alice.handle(client.OnRTDTRefreshedSessionAllowance(func(refreshSess zkidentity.ShortID, addAllowance uint64) {
		if sessRV == refreshSess {
			aliceRefreshedAllowance <- struct{}{}
		}
	}))
	minRefreshes := 2
	buf := make([]byte, 10000)
	for sentN := 0; sentN < minRefreshes*rpc.PublishFreePayRTPublishAllowanceMB*1000000; sentN += len(buf) {
		rand.Read(buf)
		err := rtSessAlice.SendRandomData(ts.ctx, buf, 1000)
		assert.NilErr(t, err)
		assert.ChanWrittenWithVal(t, gotBobDataChan, buf)
	}

	// It should have refreshed at least minRefreshes times.
	for i := 0; i < minRefreshes; i++ {
		assert.ChanWritten(t, aliceRefreshedAllowance)
	}

	// Bob leaves the live session.
	assert.NilErr(t, bob.LeaveLiveRTSession(sessRV))

	// Bob should not receive data anymore.
	assert.NilErr(t, rtSessAlice.SendRandomData(ts.ctx, aliceData, 1000))
	assert.ChanNotWritten(t, gotBobDataChan, time.Second)
}

// TestRTDTSessionNonPublisher tests a session where a remote peer is not a
// publisher.
func TestRTDTSessionNonPublisher(t *testing.T) {
	t.Parallel()

	// Handler for inbound RTDT random data (needs to be setup on client
	// creation).
	gotAliceDataChan := make(chan []byte, 5)
	aliceRandomHandler := func(sess *rtdtclient.Session, enc *rpc.RTDTFramedPacket, plain *rpc.RTDTDataPacket) error {
		gotAliceDataChan <- slices.Clone(plain.Data)
		return nil
	}
	gotBobDataChan := make(chan []byte, 5)
	bobRandomHandler := func(sess *rtdtclient.Session, enc *rpc.RTDTFramedPacket, plain *rpc.RTDTDataPacket) error {
		gotBobDataChan <- slices.Clone(plain.Data)
		return nil
	}

	// Setup Alice and Bob and have them KX.
	tcfg := testScaffoldCfg{runRtdtServer: true}
	ts := newTestScaffold(t, tcfg)
	alice := ts.newClient("alice", withRTDTRandomStreamHandler(aliceRandomHandler))
	bob := ts.newClient("bob", withRTDTRandomStreamHandler(bobRandomHandler))
	ts.kxUsers(alice, bob)

	// Handlers.
	aliceAcceptedChan := make(chan bool, 5)
	alice.handle(client.OnRTDTSessionInviteAccepted(func(ru *client.RemoteUser, sessID zkidentity.ShortID, asPublisher bool) {
		if ru.ID() == bob.PublicID() {
			aliceAcceptedChan <- asPublisher
		}
	}))
	aliceUpdateChan := make(chan *client.RTDTSessionUpdateNtfn, 5)
	alice.handle(client.OnRTDTSesssionUpdated(func(ru *client.RemoteUser, update *client.RTDTSessionUpdateNtfn) {
		aliceUpdateChan <- update
	}))

	bobInviteChan := make(chan *rpc.RMRTDTSessionInvite, 2)
	bob.handle(client.OnInvitedToRTDTSession(func(ru *client.RemoteUser, invite *rpc.RMRTDTSessionInvite, ts time.Time) {
		bobInviteChan <- invite
	}))
	bobUpdateChan := make(chan *client.RTDTSessionUpdateNtfn, 5)
	bob.handle(client.OnRTDTSesssionUpdated(func(ru *client.RemoteUser, update *client.RTDTSessionUpdateNtfn) {
		bobUpdateChan <- update
	}))

	// Create the RTDT session.
	initialSess, err := alice.CreateRTDTSession(2, "test session")
	assert.NilErr(t, err)
	sessRV := initialSess.Metadata.RV
	assert.NotNil(t, initialSess.PublisherKey)
	assert.DeepEqual(t, initialSess.Metadata.Publishers[0].PublisherKey, *initialSess.PublisherKey)

	// Alice invites bob to session.
	err = alice.InviteToRTDTSession(sessRV, false, bob.PublicID())
	assert.NilErr(t, err)

	// Bob Will receive and accept the invite.
	gotBobInvite := assert.ChanWritten(t, bobInviteChan)
	assert.DeepEqual(t, gotBobInvite.AllowedAsPublisher, false)
	err = bob.AcceptRTDTSessionInvite(alice.PublicID(), gotBobInvite, false)
	assert.NilErr(t, err)
	assert.ChanWrittenWithVal(t, aliceAcceptedChan, false)
	gotAliceUpdate := assert.ChanWritten(t, aliceUpdateChan)
	assert.DeepEqual(t, len(gotAliceUpdate.NewPublishers), 0)
	gotBobUpdate := assert.ChanWritten(t, bobUpdateChan)
	assert.DeepEqual(t, len(gotBobUpdate.NewPublishers), 1)
	assert.DeepEqual(t, gotBobUpdate.NewPublishers[0].PeerID, initialSess.LocalPeerID)
	assert.DeepEqual(t, gotBobUpdate.NewPublishers[0].PublisherKey, *initialSess.PublisherKey)

	// Now that the high level session is created C2C, join it on the rtdt
	// server.
	aliceJoinedChan := make(chan struct{}, 5)
	alice.handle(client.OnRTDTLiveSessionJoined(func(joinedSessRV zkidentity.ShortID) {
		if joinedSessRV == sessRV {
			aliceJoinedChan <- struct{}{}
		}
	}))
	bobJoinedChan := make(chan struct{}, 5)
	bob.handle(client.OnRTDTLiveSessionJoined(func(joinedSessRV zkidentity.ShortID) {
		if joinedSessRV == sessRV {
			bobJoinedChan <- struct{}{}
		}
	}))
	assert.NilErr(t, alice.JoinLiveRTDTSession(sessRV))
	assert.ChanWritten(t, aliceJoinedChan)
	assert.NilErr(t, bob.JoinLiveRTDTSession(sessRV))
	assert.ChanWritten(t, bobJoinedChan)

	// Finally, send some random data through RTDT from each peer. Bob
	// receives data, but cannot send (because he was not invited as a
	// publisher).
	rtSessAlice, rtSessBob := alice.GetLiveRTSession(&sessRV).RTSess, bob.GetLiveRTSession(&sessRV).RTSess
	assert.NotNil(t, rtSessAlice)
	assert.NotNil(t, rtSessBob)
	aliceData, bobData := []byte("from alice"), []byte("from bob")
	err = rtSessAlice.SendRandomData(ts.ctx, aliceData, 1000)
	assert.NilErr(t, err)
	assert.ChanWrittenWithVal(t, gotBobDataChan, aliceData)
	err = rtSessBob.SendRandomData(ts.ctx, bobData, 1000)
	assert.NilErr(t, err)
	assert.ChanNotWritten(t, gotAliceDataChan, time.Second)
}

// TestRTDTSessionKickAndBan tests scenarios for kicking and banning users from
// RTDT sessions.
func TestRTDTSessionKickAndBan(t *testing.T) {
	t.Parallel()

	// Handler for inbound RTDT random data (needs to be setup on client
	// creation).
	gotAliceDataChan := make(chan []byte, 5)
	aliceRandomHandler := func(sess *rtdtclient.Session, enc *rpc.RTDTFramedPacket, plain *rpc.RTDTDataPacket) error {
		gotAliceDataChan <- slices.Clone(plain.Data)
		return nil
	}
	gotBobDataChan := make(chan []byte, 5)
	bobRandomHandler := func(sess *rtdtclient.Session, enc *rpc.RTDTFramedPacket, plain *rpc.RTDTDataPacket) error {
		gotBobDataChan <- slices.Clone(plain.Data)
		return nil
	}
	gotCharlieDataChan := make(chan []byte, 5)
	charlieRandomHandler := func(sess *rtdtclient.Session, enc *rpc.RTDTFramedPacket, plain *rpc.RTDTDataPacket) error {
		gotCharlieDataChan <- slices.Clone(plain.Data)
		return nil
	}

	// Setup Alice, Bob and Charlie and have them KX.
	tcfg := testScaffoldCfg{runRtdtServer: true}
	ts := newTestScaffold(t, tcfg)
	alice := ts.newClient("alice", withRTDTRandomStreamHandler(aliceRandomHandler))
	bob := ts.newClient("bob", withRTDTRandomStreamHandler(bobRandomHandler))
	charlie := ts.newClient("charlie", withRTDTRandomStreamHandler(charlieRandomHandler))
	ts.kxUsers(alice, bob)
	ts.kxUsers(alice, charlie)

	bobRotCookie := make(chan struct{}, 2)
	bob.handle(client.OnRTDTRotatedCookie(func(ru *client.RemoteUser, sessRV zkidentity.ShortID) {
		bobRotCookie <- struct{}{}
	}))
	charlieRotCookie := make(chan struct{}, 2)
	charlie.handle(client.OnRTDTRotatedCookie(func(ru *client.RemoteUser, sessRV zkidentity.ShortID) {
		charlieRotCookie <- struct{}{}
	}))

	sess, err := alice.CreateRTDTSession(3, "test session")
	assert.NilErr(t, err)
	sessRV := sess.Metadata.RV

	// Bob and Charlie join the session.
	assertJoinsRTDTSession(t, alice, bob, sessRV)
	assertJoinsRTDTSession(t, alice, charlie, sessRV)

	aliceSess := assertJoinsLiveRTDTSession(t, alice, sessRV)
	bobSess := assertJoinsLiveRTDTSession(t, bob, sessRV)
	charlieSess := assertJoinsLiveRTDTSession(t, charlie, sessRV)

	// Everyone can send and receive messages.
	assertSendsRandomRTDTData(t, aliceSess, gotBobDataChan, gotCharlieDataChan)
	assertSendsRandomRTDTData(t, bobSess, gotAliceDataChan, gotCharlieDataChan)
	assertSendsRandomRTDTData(t, charlieSess, gotAliceDataChan, gotBobDataChan)

	// Alice temp kicks Bob (no temp ban).
	assertKicksFromLiveRTDTSession(t, alice, bob, &sessRV, 0)

	// Alice and Charlie still in the session.
	assertSendsRandomRTDTData(t, aliceSess, gotCharlieDataChan)
	assertSendsRandomRTDTData(t, charlieSess, gotAliceDataChan)
	assert.ChanNotWritten(t, gotBobDataChan, 100*time.Millisecond)

	// Bob rejoins and can exchange data.
	bobSess = assertJoinsLiveRTDTSession(t, bob, sessRV)
	assertSendsRandomRTDTData(t, aliceSess, gotBobDataChan, gotCharlieDataChan)
	assertSendsRandomRTDTData(t, bobSess, gotAliceDataChan, gotCharlieDataChan)
	assertSendsRandomRTDTData(t, charlieSess, gotAliceDataChan, gotBobDataChan)

	// Alice kicks Bob with a temp ban. Even though it looks like Bob can
	// rejoin, he can't exchange data.
	banDuration := time.Second
	assertKicksFromLiveRTDTSession(t, alice, bob, &sessRV, banDuration)
	assertJoinsLiveRTDTSession(t, bob, sessRV)
	assertSendsRandomRTDTData(t, aliceSess, gotCharlieDataChan)
	assertSendsRandomRTDTData(t, charlieSess, gotAliceDataChan)
	assert.ChanNotWritten(t, gotBobDataChan, 100*time.Millisecond)

	// Bob waits for ban to be lifted and rejoin.
	time.Sleep(banDuration)
	assert.NilErr(t, bob.LeaveLiveRTSession(sessRV))
	time.Sleep(10 * time.Millisecond)
	bobSess = assertJoinsLiveRTDTSession(t, bob, sessRV)
	assertSendsRandomRTDTData(t, aliceSess, gotBobDataChan, gotCharlieDataChan)
	assertSendsRandomRTDTData(t, bobSess, gotAliceDataChan, gotCharlieDataChan)
	assertSendsRandomRTDTData(t, charlieSess, gotAliceDataChan, gotBobDataChan)

	// Alice kicks bob again, with a long temp ban. But if everyone leaves
	// the live session, the temp ban is lifted.
	longBanDuration := time.Hour
	assertKicksFromLiveRTDTSession(t, alice, bob, &sessRV, longBanDuration)
	assert.NilErr(t, alice.LeaveLiveRTSession(sessRV))
	assert.NilErr(t, charlie.LeaveLiveRTSession(sessRV))
	time.Sleep(10 * time.Millisecond)
	aliceSess = assertJoinsLiveRTDTSession(t, alice, sessRV)
	bobSess = assertJoinsLiveRTDTSession(t, bob, sessRV)
	charlieSess = assertJoinsLiveRTDTSession(t, charlie, sessRV)
	assertSendsRandomRTDTData(t, aliceSess, gotBobDataChan, gotCharlieDataChan)
	assertSendsRandomRTDTData(t, bobSess, gotAliceDataChan, gotCharlieDataChan)
	assertSendsRandomRTDTData(t, charlieSess, gotAliceDataChan, gotBobDataChan)

	// Alice kicks Bob again, in order to rotate the appointment keys.
	assertKicksFromLiveRTDTSession(t, alice, bob, &sessRV, 0)

	// Rotate appointment keys, skipping Bob. This is only available through
	// the test interface, because the standard API rotates for all members.
	err = alice.testInterface().RotateRTDTAppointmentCookies(&sessRV, bob.PublicID())
	assert.NilErr(t, err)

	// Charlie gets the rotated cookie, Bob does not.
	assert.ChanWritten(t, charlieRotCookie)
	assert.ChanNotWritten(t, bobRotCookie, time.Second)

	// The session is still working.
	assertSendsRandomRTDTData(t, aliceSess, gotCharlieDataChan)
	assertSendsRandomRTDTData(t, charlieSess, gotAliceDataChan)
	assert.ChanNotWritten(t, gotBobDataChan, 100*time.Millisecond)

	// Bob attempts to join again, but ends up in a different (internal)
	// session, so he won't get data.
	assertJoinsLiveRTDTSession(t, bob, sessRV)
	assertSendsRandomRTDTData(t, aliceSess, gotCharlieDataChan)
	assertSendsRandomRTDTData(t, charlieSess, gotAliceDataChan)
	assert.ChanNotWritten(t, gotBobDataChan, 100*time.Millisecond)

	// Everybody leaves and joins again, same effect (Bob still in
	// different internal session).
	assert.NilErr(t, alice.LeaveLiveRTSession(sessRV))
	assert.NilErr(t, bob.LeaveLiveRTSession(sessRV))
	assert.NilErr(t, charlie.LeaveLiveRTSession(sessRV))
	time.Sleep(10 * time.Millisecond)
	aliceSess = assertJoinsLiveRTDTSession(t, alice, sessRV)
	assertJoinsLiveRTDTSession(t, bob, sessRV)
	charlieSess = assertJoinsLiveRTDTSession(t, charlie, sessRV)
	assertSendsRandomRTDTData(t, aliceSess, gotCharlieDataChan)
	assertSendsRandomRTDTData(t, charlieSess, gotAliceDataChan)
	assert.ChanNotWritten(t, gotBobDataChan, 100*time.Millisecond)

	// Alice rotates cookies again, but this time she includes Bob. Bob
	// should be able to rejoin the correct session. Test before leaving
	// and after everybody leaving and returning.
	err = alice.RotateRTDTAppointmentCookies(&sessRV)
	assert.NilErr(t, err)
	assert.ChanWritten(t, bobRotCookie)
	assert.ChanWritten(t, charlieRotCookie)
	assert.NilErr(t, bob.LeaveLiveRTSession(sessRV))
	bobSess = assertJoinsLiveRTDTSession(t, bob, sessRV)
	assertSendsRandomRTDTData(t, aliceSess, gotBobDataChan, gotCharlieDataChan)
	assertSendsRandomRTDTData(t, bobSess, gotAliceDataChan, gotCharlieDataChan)
	assertSendsRandomRTDTData(t, charlieSess, gotAliceDataChan, gotBobDataChan)
	assert.NilErr(t, alice.LeaveLiveRTSession(sessRV))
	assert.NilErr(t, bob.LeaveLiveRTSession(sessRV))
	assert.NilErr(t, charlie.LeaveLiveRTSession(sessRV))
	time.Sleep(10 * time.Millisecond)
	aliceSess = assertJoinsLiveRTDTSession(t, alice, sessRV)
	bobSess = assertJoinsLiveRTDTSession(t, bob, sessRV)
	charlieSess = assertJoinsLiveRTDTSession(t, charlie, sessRV)
	assertSendsRandomRTDTData(t, aliceSess, gotBobDataChan, gotCharlieDataChan)
	assertSendsRandomRTDTData(t, bobSess, gotAliceDataChan, gotCharlieDataChan)
	assertSendsRandomRTDTData(t, charlieSess, gotAliceDataChan, gotBobDataChan)

	// Alice permanently removes Bob from the session.
	bobKickedChan := make(chan struct{}, 3)
	bob.handle(client.OnRTDTKickedFromLiveSession(func(kickedSessRV zkidentity.ShortID,
		peerID rpc.RTDTPeerID, kickedBanDuration time.Duration) {
		bobKickedChan <- struct{}{}
	}))
	bobRemovedChan := make(chan struct{}, 3)
	bob.handle(client.OnRTDTRemovedFromSession(func(ru *client.RemoteUser, sessRV zkidentity.ShortID, reason string) {
		bobRemovedChan <- struct{}{}
	}))

	charlieUpdatedChan := make(chan error, 3)
	charlie.handle(client.OnRTDTSesssionUpdated(func(ru *client.RemoteUser, update *client.RTDTSessionUpdateNtfn) {
		if len(update.RemovedPublishers) == 1 {
			if update.RemovedPublishers[0].PublisherID == bob.PublicID() {
				charlieUpdatedChan <- nil
			} else {
				charlieUpdatedChan <- errors.New("not bob's publisher id")
			}
		} else {
			charlieUpdatedChan <- fmt.Errorf("wrong number of removed publishers (%d)",
				len(update.RemovedPublishers))
		}
	}))

	// Bob gets the removal event, charlie gets a metadata update with Bob
	// removed.
	bobUID := bob.PublicID()
	assert.NilErr(t, alice.RemoveRTDTMember(&sessRV, &bobUID, "test"))
	assert.ChanWritten(t, bobKickedChan)
	assert.ChanWritten(t, bobRemovedChan)
	assert.ChanWrittenWithVal(t, charlieUpdatedChan, error(nil))
	assertSendsRandomRTDTData(t, aliceSess, gotCharlieDataChan)
	assertSendsRandomRTDTData(t, charlieSess, gotAliceDataChan)
	assert.ChanNotWritten(t, gotBobDataChan, 100*time.Millisecond)
}

// TestRTDTSessionExitDissolve tests exiting and dissolving an RTDT session.
func TestRTDTSessionExitDissolve(t *testing.T) {
	t.Parallel()

	// Handler for inbound RTDT random data (needs to be setup on client
	// creation).
	gotAliceDataChan := make(chan []byte, 5)
	aliceRandomHandler := func(sess *rtdtclient.Session, enc *rpc.RTDTFramedPacket, plain *rpc.RTDTDataPacket) error {
		gotAliceDataChan <- slices.Clone(plain.Data)
		return nil
	}
	gotBobDataChan := make(chan []byte, 5)
	bobRandomHandler := func(sess *rtdtclient.Session, enc *rpc.RTDTFramedPacket, plain *rpc.RTDTDataPacket) error {
		gotBobDataChan <- slices.Clone(plain.Data)
		return nil
	}
	gotCharlieDataChan := make(chan []byte, 5)
	charlieRandomHandler := func(sess *rtdtclient.Session, enc *rpc.RTDTFramedPacket, plain *rpc.RTDTDataPacket) error {
		gotCharlieDataChan <- slices.Clone(plain.Data)
		return nil
	}

	// Setup Alice, Bob and Charlie and have them KX.
	tcfg := testScaffoldCfg{runRtdtServer: true}
	ts := newTestScaffold(t, tcfg)
	alice := ts.newClient("alice", withRTDTRandomStreamHandler(aliceRandomHandler))
	bob := ts.newClient("bob", withRTDTRandomStreamHandler(bobRandomHandler))
	charlie := ts.newClient("charlie", withRTDTRandomStreamHandler(charlieRandomHandler))
	ts.kxUsers(alice, bob)
	ts.kxUsers(alice, charlie)

	// Create the RTDT session.
	sess, err := alice.CreateRTDTSession(3, "test session")
	assert.NilErr(t, err)
	sessRV := sess.Metadata.RV

	// Bob and Charlie join the session.
	assertJoinsRTDTSession(t, alice, bob, sessRV)
	assertJoinsRTDTSession(t, alice, charlie, sessRV)

	aliceSess := assertJoinsLiveRTDTSession(t, alice, sessRV)
	bobSess := assertJoinsLiveRTDTSession(t, bob, sessRV)
	charlieSess := assertJoinsLiveRTDTSession(t, charlie, sessRV)
	assertSendsRandomRTDTData(t, aliceSess, gotBobDataChan, gotCharlieDataChan)
	assertSendsRandomRTDTData(t, bobSess, gotAliceDataChan, gotCharlieDataChan)
	assertSendsRandomRTDTData(t, charlieSess, gotAliceDataChan, gotBobDataChan)

	aliceExitedChan := make(chan struct{}, 3)
	alice.handle(client.OnRTDTPeerExitedSession(func(ru *client.RemoteUser, sessRV zkidentity.ShortID, peerID rpc.RTDTPeerID) {
		if ru.ID() == bob.PublicID() {
			aliceExitedChan <- struct{}{}
		}
	}))

	charlieUpdatedChan := make(chan error, 3)
	charlie.handle(client.OnRTDTSesssionUpdated(func(ru *client.RemoteUser, update *client.RTDTSessionUpdateNtfn) {
		if len(update.RemovedPublishers) == 1 {
			if update.RemovedPublishers[0].PublisherID == bob.PublicID() {
				charlieUpdatedChan <- nil
			} else {
				charlieUpdatedChan <- errors.New("not bob's publisher id")
			}
		} else {
			charlieUpdatedChan <- fmt.Errorf("wrong number of removed publishers (%d)",
				len(update.RemovedPublishers))
		}
	}))

	// Bob leaves the channel. Alice gets an exit notification and Charlie
	// gets an update (from Alice).
	assert.NilErr(t, bob.ExitRTDTSession(&sessRV))
	assert.ChanWritten(t, aliceExitedChan)
	assert.ChanWritten(t, charlieUpdatedChan)

	// Only Alice and Charlie remain in the live session.
	assertSendsRandomRTDTData(t, aliceSess, gotCharlieDataChan)
	assertSendsRandomRTDTData(t, charlieSess, gotAliceDataChan)
	assert.ChanNotWritten(t, gotBobDataChan, 100*time.Millisecond)

	// Alice dissolves the session.
	charlieDissolvedChan := make(chan struct{}, 3)
	charlie.handle(client.OnRTDTSessionDissolved(func(ru *client.RemoteUser, sessRV zkidentity.ShortID, peerID rpc.RTDTPeerID) {
		charlieDissolvedChan <- struct{}{}
	}))
	assert.NilErr(t, alice.DissolveRTDTSession(&sessRV))
	assert.ChanWritten(t, charlieDissolvedChan)
}

// TestRTDTNoDupePeerID asserts that inviting, removing and then inviting again
// does not repeat the same peer id.
func TestRTDTNoDupePeerID(t *testing.T) {
	t.Parallel()

	// Setup Alice and Bob and have them KX.
	tcfg := testScaffoldCfg{runRtdtServer: true}
	ts := newTestScaffold(t, tcfg)
	alice := ts.newClient("alice")
	bob := ts.newClient("bob")
	ts.kxUsers(alice, bob)

	aliceExitedChan := make(chan rpc.RTDTPeerID, 5)
	alice.handle(client.OnRTDTPeerExitedSession(func(ru *client.RemoteUser, sessRV zkidentity.ShortID, peerID rpc.RTDTPeerID) {
		aliceExitedChan <- peerID
	}))

	// Create the RTDT session.
	sess, err := alice.CreateRTDTSession(3, "test session")
	assert.NilErr(t, err)
	sessRV := sess.Metadata.RV

	// Bob joins the session.
	assertJoinsRTDTSession(t, alice, bob, sessRV)

	// Remember Bob's peer id.
	bobSessMeta, err := bob.GetRTDTSession(&sessRV)
	assert.NilErr(t, err)
	bobPeerID1 := bobSessMeta.LocalPeerID

	// Bob leaves the session.
	assert.NilErr(t, bob.ExitRTDTSession(&sessRV))
	assert.ChanWrittenWithVal(t, aliceExitedChan, bobPeerID1)

	// Bob rejoins. This time, he will have a different id.
	assertJoinsRTDTSession(t, alice, bob, sessRV)
	bobSessMeta, err = bob.GetRTDTSession(&sessRV)
	assert.NilErr(t, err)
	bobPeerID2 := bobSessMeta.LocalPeerID
	if bobPeerID1 == bobPeerID2 {
		t.Fatalf("Bob peer ids should be different, they are the same (%s)",
			bobPeerID1)
	}
}

// TestRTDTMultipleInvites verifies the behavior when a session owner invites
// multiple times.
func TestRTDTMultipleInvites(t *testing.T) {
	t.Parallel()

	// Setup Alice and Bob and have them KX.
	tcfg := testScaffoldCfg{runRtdtServer: true}
	ts := newTestScaffold(t, tcfg)
	alice := ts.newClient("alice")
	bob := ts.newClient("bob")
	ts.kxUsers(alice, bob)

	// Handlers.
	aliceAcceptedChan := make(chan struct{}, 5)
	alice.handle(client.OnRTDTSessionInviteAccepted(func(ru *client.RemoteUser, sessID zkidentity.ShortID, acceptedAsPublisher bool) {
		aliceAcceptedChan <- struct{}{}
	}))
	bobInvitedChan := make(chan *rpc.RMRTDTSessionInvite, 5)
	bob.handle(client.OnInvitedToRTDTSession(func(ru *client.RemoteUser, invite *rpc.RMRTDTSessionInvite, ts time.Time) {
		bobInvitedChan <- invite
	}))
	bobJoinedChan := make(chan struct{}, 5)
	bob.handle(client.OnRTDTSesssionUpdated(func(ru *client.RemoteUser, update *client.RTDTSessionUpdateNtfn) {
		if update.InitialJoin {
			bobJoinedChan <- struct{}{}
		}
	}))

	// Create the RTDT session.
	sess, err := alice.CreateRTDTSession(3, "test session")
	assert.NilErr(t, err)
	sessRV := sess.Metadata.RV

	// Alice invites Bob twice.
	assert.NilErr(t, alice.InviteToRTDTSession(sessRV, true, bob.PublicID()))
	assert.NilErr(t, alice.InviteToRTDTSession(sessRV, true, bob.PublicID()))

	// Bob gets two invitations.
	invite1 := assert.ChanWritten(t, bobInvitedChan)
	invite2 := assert.ChanWritten(t, bobInvitedChan)
	assert.DeepEqual(t, invite1.PeerID, invite2.PeerID)
	if invite1.Tag == invite2.Tag {
		t.Fatalf("Tags are equal when they should be different")
	}
	if bytes.Equal(invite1.AppointCookie, invite2.AppointCookie) {
		t.Fatalf("Appointment cookies are equal when they should be different")
	}

	// First one can't be accepted (has been overridden by second one).
	assert.NilErr(t, bob.AcceptRTDTSessionInvite(alice.PublicID(), invite1, true))
	assert.ChanNotWritten(t, aliceAcceptedChan, time.Second)

	// Second one is accepted.
	assert.NilErr(t, bob.AcceptRTDTSessionInvite(alice.PublicID(), invite2, true))
	assert.ChanWritten(t, aliceAcceptedChan)
	assert.ChanWritten(t, bobJoinedChan)
}

// TestRTDTSessionWithGC tests the features of an RTDT session that is
// associated with a GC.
func TestRTDTSessionWithGC(t *testing.T) {
	// Handler for inbound RTDT random data (needs to be setup on client
	// creation).
	gotAliceDataChan := make(chan []byte, 5)
	aliceRandomHandler := func(sess *rtdtclient.Session, enc *rpc.RTDTFramedPacket, plain *rpc.RTDTDataPacket) error {
		gotAliceDataChan <- slices.Clone(plain.Data)
		return nil
	}
	gotBobDataChan := make(chan []byte, 5)
	bobRandomHandler := func(sess *rtdtclient.Session, enc *rpc.RTDTFramedPacket, plain *rpc.RTDTDataPacket) error {
		gotBobDataChan <- slices.Clone(plain.Data)
		return nil
	}
	gotCharlieDataChan := make(chan []byte, 5)
	charlieRandomHandler := func(sess *rtdtclient.Session, enc *rpc.RTDTFramedPacket, plain *rpc.RTDTDataPacket) error {
		gotCharlieDataChan <- slices.Clone(plain.Data)
		return nil
	}

	// Setup Alice and Bob and have them KX.
	tcfg := testScaffoldCfg{runRtdtServer: true}
	ts := newTestScaffold(t, tcfg)
	alice := ts.newClient("alice", withRTDTRandomStreamHandler(aliceRandomHandler))
	bob := ts.newClient("bob", withRTDTRandomStreamHandler(bobRandomHandler))
	charlie := ts.newClient("charlie", withRTDTRandomStreamHandler(charlieRandomHandler))
	ts.kxUsers(alice, bob)
	ts.kxUsers(alice, charlie)
	ts.kxUsers(charlie, bob)

	// Create the GC and have Bob join it.
	gcID, err := alice.NewGroupChat("gc01")
	assert.NilErr(t, err)
	assertJoinsGC(t, alice, bob, gcID)
	assertClientInGC(t, bob, gcID)
	assertClientsCanGCM(t, gcID, alice, bob)

	// Handlers.
	aliceAcceptedChan := make(chan struct{}, 5)
	alice.handle(client.OnRTDTSessionInviteAccepted(func(ru *client.RemoteUser, sessID zkidentity.ShortID, acceptedAsPublisher bool) {
		aliceAcceptedChan <- struct{}{}
	}))
	bobInvitedChan := make(chan *rpc.RMRTDTSessionInvite, 5)
	bob.handle(client.OnInvitedToRTDTSession(func(ru *client.RemoteUser, invite *rpc.RMRTDTSessionInvite, ts time.Time) {
		bobInvitedChan <- invite
	}))
	bobJoinedChan := make(chan struct{}, 5)
	bob.handle(client.OnRTDTSesssionUpdated(func(ru *client.RemoteUser, update *client.RTDTSessionUpdateNtfn) {
		if update.InitialJoin {
			bobJoinedChan <- struct{}{}
		}
	}))
	charlieInvitedChan := make(chan *rpc.RMRTDTSessionInvite, 5)
	charlie.handle(client.OnInvitedToRTDTSession(func(ru *client.RemoteUser, invite *rpc.RMRTDTSessionInvite, ts time.Time) {
		charlieInvitedChan <- invite
	}))
	charlieJoinedChan := make(chan struct{}, 5)
	charlie.handle(client.OnRTDTSesssionUpdated(func(ru *client.RemoteUser, update *client.RTDTSessionUpdateNtfn) {
		if update.InitialJoin {
			charlieJoinedChan <- struct{}{}
		}
	}))

	// Alice creates an RTDT session associated with the GC. Bob should
	// automatically receive the invitation to join it.
	sess, err := alice.CreateRTDTSessionInGC(gcID, 1, true)
	assert.NilErr(t, err)
	sessRV := sess.Metadata.RV

	// Bob joins the session.
	bobInvite := assert.ChanWritten(t, bobInvitedChan)
	assert.NilErr(t, bob.AcceptRTDTSessionInvite(alice.PublicID(), bobInvite, true))
	assert.ChanWritten(t, aliceAcceptedChan)
	assert.ChanWritten(t, bobJoinedChan)
	bobRTDTSess, err := bob.GetRTDTSession(&sessRV)
	assert.NilErr(t, err)
	assert.DeepEqual(t, *bobRTDTSess.GC, gcID) // Assert GC and RTDT session are linked.
	aliceSess := assertJoinsLiveRTDTSession(t, alice, sessRV)
	bobSess := assertJoinsLiveRTDTSession(t, bob, sessRV)
	assertSendsRandomRTDTData(t, aliceSess, gotBobDataChan)
	assertSendsRandomRTDTData(t, bobSess, gotAliceDataChan)

	// Trying to invite charlie to the RTDT session should fail (charlie is
	// not in the GC yet).
	err = alice.InviteToRTDTSession(sessRV, true, charlie.PublicID())
	assert.ErrorIs(t, err, client.ErrRTDTInviteeNotGCMember{})

	// Charlie is invited to the GC.
	assertJoinsGC(t, alice, charlie, gcID)
	assertClientInGC(t, charlie, gcID)
	assertClientsCanGCM(t, gcID, alice, bob, charlie)

	// Charlie is automatically invited to the RTDT session.
	charlieInvite := assert.ChanWritten(t, charlieInvitedChan)
	assert.NilErr(t, charlie.AcceptRTDTSessionInvite(alice.PublicID(), charlieInvite, true))
	assert.ChanWritten(t, aliceAcceptedChan)
	assert.ChanWritten(t, charlieJoinedChan)
	charlieRTDTSess, err := charlie.GetRTDTSession(&sessRV)
	assert.NilErr(t, err)
	assert.DeepEqual(t, *charlieRTDTSess.GC, gcID) // Assert GC and RTDT session are linked.
	charlieSess := assertJoinsLiveRTDTSession(t, charlie, sessRV)
	assertSendsRandomRTDTData(t, aliceSess, gotBobDataChan, gotCharlieDataChan)
	assertSendsRandomRTDTData(t, bobSess, gotAliceDataChan, gotCharlieDataChan)
	assertSendsRandomRTDTData(t, charlieSess, gotAliceDataChan, gotBobDataChan)
}

// TestRTDTSessionWithGCExtraAdmin tests that having a GC with an extra admin,
// upon creating the RTDT esssion the extra admin is also made an admin of the
// realtime session and can invite people to both GC and session.
func TestRTDTSessionWithGCExtraAdmin(t *testing.T) {
	// Handler for inbound RTDT random data (needs to be setup on client
	// creation).
	gotAliceDataChan := make(chan []byte, 5)
	aliceRandomHandler := func(sess *rtdtclient.Session, enc *rpc.RTDTFramedPacket, plain *rpc.RTDTDataPacket) error {
		gotAliceDataChan <- slices.Clone(plain.Data)
		return nil
	}
	gotBobDataChan := make(chan []byte, 5)
	bobRandomHandler := func(sess *rtdtclient.Session, enc *rpc.RTDTFramedPacket, plain *rpc.RTDTDataPacket) error {
		gotBobDataChan <- slices.Clone(plain.Data)
		return nil
	}
	gotCharlieDataChan := make(chan []byte, 5)
	charlieRandomHandler := func(sess *rtdtclient.Session, enc *rpc.RTDTFramedPacket, plain *rpc.RTDTDataPacket) error {
		gotCharlieDataChan <- slices.Clone(plain.Data)
		return nil
	}
	gotDaveDataChan := make(chan []byte, 5)
	daveRandomHandler := func(sess *rtdtclient.Session, enc *rpc.RTDTFramedPacket, plain *rpc.RTDTDataPacket) error {
		gotDaveDataChan <- slices.Clone(plain.Data)
		return nil
	}

	// Setup Alice and Bob and have them KX.
	tcfg := testScaffoldCfg{runRtdtServer: true}
	ts := newTestScaffold(t, tcfg)
	alice := ts.newClient("alice", withRTDTRandomStreamHandler(aliceRandomHandler))
	bob := ts.newClient("bob", withRTDTRandomStreamHandler(bobRandomHandler))
	charlie := ts.newClient("charlie", withRTDTRandomStreamHandler(charlieRandomHandler))
	dave := ts.newClient("dave", withRTDTRandomStreamHandler(daveRandomHandler))
	ts.kxUsers(alice, bob)
	ts.kxUsers(alice, charlie)
	ts.kxUsers(alice, dave)
	ts.kxUsers(bob, charlie)
	ts.kxUsers(bob, dave)
	ts.kxUsers(charlie, dave)

	// Handlers.
	aliceAcceptedChan := make(chan struct{}, 5)
	alice.handle(client.OnRTDTSessionInviteAccepted(func(ru *client.RemoteUser, sessID zkidentity.ShortID, acceptedAsPublisher bool) {
		aliceAcceptedChan <- struct{}{}
	}))
	bobInvitedChan := make(chan *rpc.RMRTDTSessionInvite, 5)
	bob.handle(client.OnInvitedToRTDTSession(func(ru *client.RemoteUser, invite *rpc.RMRTDTSessionInvite, ts time.Time) {
		bobInvitedChan <- invite
	}))
	bobJoinedChan := make(chan struct{}, 5)
	bob.handle(client.OnRTDTSesssionUpdated(func(ru *client.RemoteUser, update *client.RTDTSessionUpdateNtfn) {
		if update.InitialJoin {
			bobJoinedChan <- struct{}{}
		}
	}))
	bobAcceptedChan := make(chan struct{}, 5)
	bob.handle(client.OnRTDTSessionInviteAccepted(func(ru *client.RemoteUser, sessID zkidentity.ShortID, acceptedAsPublisher bool) {
		bobAcceptedChan <- struct{}{}
	}))
	bobAdminCookiesChan := make(chan struct{}, 5)
	bob.handle(client.OnRTDTAdminCookiesReceived(func(ru *client.RemoteUser, sessRV zkidentity.ShortID) {
		bobAdminCookiesChan <- struct{}{}
	}))
	charlieInvitedChan := make(chan *rpc.RMRTDTSessionInvite, 5)
	charlie.handle(client.OnInvitedToRTDTSession(func(ru *client.RemoteUser, invite *rpc.RMRTDTSessionInvite, ts time.Time) {
		charlieInvitedChan <- invite
	}))
	charlieJoinedChan := make(chan struct{}, 5)
	charlie.handle(client.OnRTDTSesssionUpdated(func(ru *client.RemoteUser, update *client.RTDTSessionUpdateNtfn) {
		if update.InitialJoin {
			charlieJoinedChan <- struct{}{}
		}
	}))
	daveInvitedChan := make(chan *rpc.RMRTDTSessionInvite, 5)
	dave.handle(client.OnInvitedToRTDTSession(func(ru *client.RemoteUser, invite *rpc.RMRTDTSessionInvite, ts time.Time) {
		daveInvitedChan <- invite
	}))
	daveJoinedChan := make(chan struct{}, 5)
	dave.handle(client.OnRTDTSesssionUpdated(func(ru *client.RemoteUser, update *client.RTDTSessionUpdateNtfn) {
		if update.InitialJoin {
			daveJoinedChan <- struct{}{}
		}
	}))

	// Create the GC and have Bob join it.
	gcID, err := alice.NewGroupChat("gc01")
	assert.NilErr(t, err)
	assertJoinsGC(t, alice, bob, gcID)
	assertClientInGC(t, bob, gcID)
	assertClientsCanGCM(t, gcID, alice, bob)

	// Alice makes Bob an admin.
	bobAddedExtraAdminChan := make(chan error, 1)
	bob.handle(client.OnGCAdminsChangedNtfn(func(_ *client.RemoteUser, gc rpc.RMGroupList, added, removed []zkidentity.ShortID) {
		if len(added) == 1 && added[0] == bob.PublicID() {
			bobAddedExtraAdminChan <- nil
		}
	}))
	assert.NilErr(t, alice.ModifyGCAdmins(gcID, []zkidentity.ShortID{bob.PublicID()}, ""))
	assert.ChanWritten(t, bobAddedExtraAdminChan)

	// Alice creates an RTDT session associated with the given GC.
	sess, err := alice.CreateRTDTSessionInGC(gcID, 5, true)
	assert.NilErr(t, err)
	sessRV := sess.Metadata.RV

	// Bob accepts the invite to join the session.
	bobInvite := assert.ChanWritten(t, bobInvitedChan)
	assert.NilErr(t, bob.AcceptRTDTSessionInvite(alice.PublicID(), bobInvite, true))
	assert.ChanWritten(t, aliceAcceptedChan)
	assert.ChanWritten(t, bobJoinedChan)

	// Bob gets 2 admin cookie messages: first one was because of his
	// invitation, second one is because he was made an admin.
	assert.ChanWritten(t, bobAdminCookiesChan)
	assert.ChanWritten(t, bobAdminCookiesChan)

	// Bob joins the live session.
	aliceSess := assertJoinsLiveRTDTSession(t, alice, sessRV)
	bobSess := assertJoinsLiveRTDTSession(t, bob, sessRV)
	assertSendsRandomRTDTData(t, aliceSess, gotBobDataChan)
	assertSendsRandomRTDTData(t, bobSess, gotAliceDataChan)

	// Alice goes offline (to ensure she's not responding to any requests
	// related to either GC or RTDT session).
	assertEmptyRMQ(t, alice)
	assertGoesOffline(t, alice)
	assert.NilErr(t, alice.LeaveLiveRTSession(sessRV))

	// Bob can invite Charlie to the GC (implies he's an admin of the GC).
	// Charlie is invited to the GC.
	assertJoinsGC(t, bob, charlie, gcID)
	assertClientInGC(t, charlie, gcID)

	// Charlie accepts the invitation to join the RTDT session (implies Bob
	// is an admin and can add him to the RTDT session).
	charlieInvite := assert.ChanWritten(t, charlieInvitedChan)
	assert.NilErr(t, charlie.AcceptRTDTSessionInvite(bob.PublicID(), charlieInvite, true))
	assert.ChanWritten(t, bobAcceptedChan)
	assert.ChanWritten(t, charlieJoinedChan)
	charlieRTDTSess, err := charlie.GetRTDTSession(&sessRV)
	assert.NilErr(t, err)
	assert.DeepEqual(t, *charlieRTDTSess.GC, gcID) // Assert GC and RTDT session are linked.
	charlieSess := assertJoinsLiveRTDTSession(t, charlie, sessRV)
	assertSendsRandomRTDTData(t, bobSess, gotCharlieDataChan)
	assertSendsRandomRTDTData(t, charlieSess, gotBobDataChan)

	// Assert Bob can kick members (ensures he's an admin in the live
	// session). Charlie comes back to continue next tests.
	assertKicksFromLiveRTDTSession(t, bob, charlie, &sessRV, 0)
	charlieSess = assertJoinsLiveRTDTSession(t, charlie, sessRV)

	// Assert Bob can rotate cookies (ensures he can permanently kick).
	charlieRotCookie := make(chan struct{}, 2)
	charlie.handle(client.OnRTDTRotatedCookie(func(ru *client.RemoteUser, sessRV zkidentity.ShortID) {
		charlieRotCookie <- struct{}{}
	}))
	err = bob.RotateRTDTAppointmentCookies(&sessRV)
	assert.NilErr(t, err)
	assert.ChanWritten(t, charlieRotCookie)

	// Alice also gets a rotated cookie after she comes online (even though
	// she was the original owner of the session).
	aliceRotCookie := make(chan struct{}, 2)
	alice.handle(client.OnRTDTRotatedCookie(func(ru *client.RemoteUser, sessRV zkidentity.ShortID) {
		aliceRotCookie <- struct{}{}
	}))
	aliceAdminCookie := make(chan struct{}, 5)
	alice.handle(client.OnRTDTAdminCookiesReceived(func(ru *client.RemoteUser, sessRV zkidentity.ShortID) {
		aliceAdminCookie <- struct{}{}
	}))
	assertGoesOnline(t, alice)
	assert.ChanWritten(t, aliceRotCookie)

	// Alice also gets 2 admin cookie messages: 1 for the new member, 1 for
	// the rotated cookies.
	assert.ChanWritten(t, aliceAdminCookie)
	assert.ChanWritten(t, aliceAdminCookie)
	assertClientUpToDate(t, alice)

	// Alice returns to the live session. Everyone still in the same live
	// session.
	aliceSess = assertJoinsLiveRTDTSession(t, alice, sessRV)
	assertSendsRandomRTDTData(t, aliceSess, gotBobDataChan, gotCharlieDataChan)
	assertSendsRandomRTDTData(t, bobSess, gotAliceDataChan, gotCharlieDataChan)
	assertSendsRandomRTDTData(t, charlieSess, gotAliceDataChan, gotBobDataChan)

	// Alice tries to invite Dave into the session.
	assertJoinsGC(t, alice, dave, gcID)
	assertClientInGC(t, dave, gcID)
	assertClientsCanGCM(t, gcID, alice, bob, charlie, dave)

	// Dave accepts the invitation to join the RTDT session. This proves
	// Alice got the updated OwnerSecret after Bob rotated it.
	daveInvite := assert.ChanWritten(t, daveInvitedChan)
	assert.NilErr(t, dave.AcceptRTDTSessionInvite(alice.PublicID(), daveInvite, true))
	assert.ChanWritten(t, aliceAcceptedChan)
	assert.ChanWritten(t, daveJoinedChan)
	daveRTDTSess, err := dave.GetRTDTSession(&sessRV)
	assert.NilErr(t, err)
	assert.DeepEqual(t, *daveRTDTSess.GC, gcID) // Assert GC and RTDT session are linked.
	daveSess := assertJoinsLiveRTDTSession(t, dave, sessRV)
	assertSendsRandomRTDTData(t, aliceSess, gotBobDataChan, gotCharlieDataChan, gotDaveDataChan)
	assertSendsRandomRTDTData(t, bobSess, gotAliceDataChan, gotCharlieDataChan, gotDaveDataChan)
	assertSendsRandomRTDTData(t, charlieSess, gotAliceDataChan, gotBobDataChan, gotDaveDataChan)
	assertSendsRandomRTDTData(t, daveSess, gotAliceDataChan, gotBobDataChan, gotCharlieDataChan)

	// Bob kicks Dave from GC (and from associated RTDT session).
	aliceGCPartedChan := make(chan client.UserID, 5)
	alice.handle(client.OnGCUserPartedNtfn(func(gcid client.GCID, uid client.UserID, reason string, kicked bool) {
		aliceGCPartedChan <- uid
	}))
	daveKickedFromLiveChan := make(chan struct{}, 5)
	dave.handle(client.OnRTDTKickedFromLiveSession(func(kickedSessRV zkidentity.ShortID,
		peerID rpc.RTDTPeerID, kickedBanDuration time.Duration) {
		daveKickedFromLiveChan <- struct{}{}
	}))
	daveRemovedFromSessChan := make(chan struct{}, 5)
	dave.handle(client.OnRTDTRemovedFromSession(func(ru *client.RemoteUser, sessRV zkidentity.ShortID, reason string) {
		daveRemovedFromSessChan <- struct{}{}
	}))
	assert.NilErr(t, bob.GCKick(gcID, dave.PublicID(), ""))
	assert.ChanWrittenWithVal(t, aliceGCPartedChan, dave.PublicID())
	assert.ChanWritten(t, daveKickedFromLiveChan)
	assert.ChanWritten(t, daveRemovedFromSessChan)
	assert.ChanWritten(t, aliceRotCookie) // Bob rotated cookies after kick.

	// Dave does not receive any data anymore.
	assertSendsRandomRTDTData(t, aliceSess, gotBobDataChan, gotCharlieDataChan)
	assert.ChanNotWritten(t, gotDaveDataChan, time.Second)
	assertGCDoesNotExist(t, gcID, dave)

	// Charlie parts from GC (and from associated RTDT session).
	assert.NilErr(t, charlie.PartFromGC(gcID, ""))
	assert.ChanWrittenWithVal(t, aliceGCPartedChan, charlie.PublicID())

	// Charlie does not receive any data anymore.
	assertSendsRandomRTDTData(t, aliceSess, gotBobDataChan)
	assert.ChanNotWritten(t, gotCharlieDataChan, time.Second)
	assertGCDoesNotExist(t, gcID, charlie)

	// Alice dissolves the GC.
	bobSessDissolved := make(chan struct{}, 5)
	bob.handle(client.OnRTDTSessionDissolved(func(ru *client.RemoteUser, sessRV zkidentity.ShortID, peerID rpc.RTDTPeerID) {
		bobSessDissolved <- struct{}{}
	}))
	assert.NilErr(t, alice.KillGroupChat(gcID, ""))
	assert.ChanWritten(t, bobSessDissolved)
}

// TestRTDTSessionWithGCReplaceSession tests that creating, dissolving, then
// creating an RTDT session associated with a GC works to increase the size of
// the RTDT session.
func TestRTDTSessionWithGCReplaceSession(t *testing.T) {
	// Handler for inbound RTDT random data (needs to be setup on client
	// creation).
	gotAliceDataChan := make(chan []byte, 5)
	aliceRandomHandler := func(sess *rtdtclient.Session, enc *rpc.RTDTFramedPacket, plain *rpc.RTDTDataPacket) error {
		gotAliceDataChan <- slices.Clone(plain.Data)
		return nil
	}
	gotBobDataChan := make(chan []byte, 5)
	bobRandomHandler := func(sess *rtdtclient.Session, enc *rpc.RTDTFramedPacket, plain *rpc.RTDTDataPacket) error {
		gotBobDataChan <- slices.Clone(plain.Data)
		return nil
	}
	gotCharlieDataChan := make(chan []byte, 5)
	charlieRandomHandler := func(sess *rtdtclient.Session, enc *rpc.RTDTFramedPacket, plain *rpc.RTDTDataPacket) error {
		gotCharlieDataChan <- slices.Clone(plain.Data)
		return nil
	}

	// Setup Alice and Bob and have them KX.
	tcfg := testScaffoldCfg{runRtdtServer: true}
	ts := newTestScaffold(t, tcfg)
	alice := ts.newClient("alice", withRTDTRandomStreamHandler(aliceRandomHandler))
	bob := ts.newClient("bob", withRTDTRandomStreamHandler(bobRandomHandler))
	charlie := ts.newClient("charlie", withRTDTRandomStreamHandler(charlieRandomHandler))
	ts.kxUsers(alice, bob)
	ts.kxUsers(alice, charlie)
	ts.kxUsers(bob, charlie)

	// Handlers.
	aliceAcceptedChan := make(chan struct{}, 5)
	alice.handle(client.OnRTDTSessionInviteAccepted(func(ru *client.RemoteUser, sessID zkidentity.ShortID, acceptedAsPublisher bool) {
		aliceAcceptedChan <- struct{}{}
	}))
	bobInvitedChan := make(chan *rpc.RMRTDTSessionInvite, 5)
	bob.handle(client.OnInvitedToRTDTSession(func(ru *client.RemoteUser, invite *rpc.RMRTDTSessionInvite, ts time.Time) {
		bobInvitedChan <- invite
	}))
	bobJoinedChan := make(chan struct{}, 5)
	bob.handle(client.OnRTDTSesssionUpdated(func(ru *client.RemoteUser, update *client.RTDTSessionUpdateNtfn) {
		if update.InitialJoin {
			bobJoinedChan <- struct{}{}
		}
	}))
	charlieInvitedChan := make(chan *rpc.RMRTDTSessionInvite, 5)
	charlie.handle(client.OnInvitedToRTDTSession(func(ru *client.RemoteUser, invite *rpc.RMRTDTSessionInvite, ts time.Time) {
		charlieInvitedChan <- invite
	}))
	charlieJoinedChan := make(chan struct{}, 5)
	charlie.handle(client.OnRTDTSesssionUpdated(func(ru *client.RemoteUser, update *client.RTDTSessionUpdateNtfn) {
		if update.InitialJoin {
			charlieJoinedChan <- struct{}{}
		}
	}))

	// Create the GC and have Bob join it.
	gcID, err := alice.NewGroupChat("gc01")
	assert.NilErr(t, err)
	assertJoinsGC(t, alice, bob, gcID)
	assertClientInGC(t, bob, gcID)
	assertClientsCanGCM(t, gcID, alice, bob)

	// Alice creates an RTDT session associated with the given GC.
	sess, err := alice.CreateRTDTSessionInGC(gcID, 0, true)
	assert.NilErr(t, err)
	sessRV := sess.Metadata.RV

	// Bob accepts the invite to join the session.
	bobInvite := assert.ChanWritten(t, bobInvitedChan)
	assert.NilErr(t, bob.AcceptRTDTSessionInvite(alice.PublicID(), bobInvite, true))
	assert.ChanWritten(t, aliceAcceptedChan)
	assert.ChanWritten(t, bobJoinedChan)

	// Bob joins the live session.
	aliceSess := assertJoinsLiveRTDTSession(t, alice, sessRV)
	bobSess := assertJoinsLiveRTDTSession(t, bob, sessRV)
	assertSendsRandomRTDTData(t, aliceSess, gotBobDataChan)
	assertSendsRandomRTDTData(t, bobSess, gotAliceDataChan)

	// Trying to invite Charlie to the GC should fail (because the
	// associated session is full).
	err = alice.InviteToGroupChat(gcID, charlie.PublicID())
	assert.ErrorIs(t, err, client.ErrRTDTSessionFull{})

	// Trying to create a second associated RTDT session should fail (only
	// one allowed per GC).
	_, err = alice.CreateRTDTSessionInGC(gcID, 0, true)
	assert.ErrorIs(t, err, client.ErrGCAlreadyHasRTDTSession)

	// Alice dissolves the RTDT session.
	bobSessDissolved := make(chan struct{}, 5)
	bob.handle(client.OnRTDTSessionDissolved(func(ru *client.RemoteUser, sessRV zkidentity.ShortID, peerID rpc.RTDTPeerID) {
		bobSessDissolved <- struct{}{}
	}))
	assert.NilErr(t, alice.DissolveRTDTSession(&sessRV))
	assert.ChanWritten(t, bobSessDissolved)

	// Alice can now create a new RTDT session with increased size.
	sess, err = alice.CreateRTDTSessionInGC(gcID, 1, true)
	assert.NilErr(t, err)
	oldRV, sessRV := sessRV, sess.Metadata.RV
	if oldRV == sessRV {
		t.Fatalf("Unexpected equal session RVs: %s, %s", oldRV, sessRV)
	}

	// Bob accepts the second invite and joins the live session.
	bobInvite = assert.ChanWritten(t, bobInvitedChan)
	assert.NilErr(t, bob.AcceptRTDTSessionInvite(alice.PublicID(), bobInvite, true))
	assert.ChanWritten(t, aliceAcceptedChan)
	assert.ChanWritten(t, bobJoinedChan)
	aliceSess = assertJoinsLiveRTDTSession(t, alice, sessRV)
	bobSess = assertJoinsLiveRTDTSession(t, bob, sessRV)
	assertSendsRandomRTDTData(t, aliceSess, gotBobDataChan)
	assertSendsRandomRTDTData(t, bobSess, gotAliceDataChan)

	// Charlie can be invited and join the live session now.
	assertJoinsGC(t, alice, charlie, gcID)
	charlieInvite := assert.ChanWritten(t, charlieInvitedChan)
	assert.NilErr(t, charlie.AcceptRTDTSessionInvite(alice.PublicID(), charlieInvite, true))
	assert.ChanWritten(t, aliceAcceptedChan)
	assert.ChanWritten(t, charlieJoinedChan)
	charlieSess := assertJoinsLiveRTDTSession(t, charlie, sessRV)
	assertSendsRandomRTDTData(t, aliceSess, gotBobDataChan, gotCharlieDataChan)
	assertSendsRandomRTDTData(t, bobSess, gotAliceDataChan, gotCharlieDataChan)
	assertSendsRandomRTDTData(t, charlieSess, gotAliceDataChan, gotBobDataChan)
}

// TestRTDTInstantSession tests how instant/ephemeral RTDT ssesions work.
func TestRTDTInstantSession(t *testing.T) {
	t.Parallel()

	// Handler for inbound RTDT random data (needs to be setup on client
	// creation).
	gotAliceDataChan := make(chan []byte, 5)
	aliceRandomHandler := func(sess *rtdtclient.Session, enc *rpc.RTDTFramedPacket, plain *rpc.RTDTDataPacket) error {
		gotAliceDataChan <- slices.Clone(plain.Data)
		return nil
	}
	gotBobDataChan := make(chan []byte, 5)
	bobRandomHandler := func(sess *rtdtclient.Session, enc *rpc.RTDTFramedPacket, plain *rpc.RTDTDataPacket) error {
		gotBobDataChan <- slices.Clone(plain.Data)
		return nil
	}
	gotCharlieDataChan := make(chan []byte, 5)
	charlieRandomHandler := func(sess *rtdtclient.Session, enc *rpc.RTDTFramedPacket, plain *rpc.RTDTDataPacket) error {
		gotCharlieDataChan <- slices.Clone(plain.Data)
		return nil
	}

	// Setup Alice and Bob and have them KX.
	tcfg := testScaffoldCfg{runRtdtServer: true}
	ts := newTestScaffold(t, tcfg)
	alice := ts.newClient("alice", withRTDTRandomStreamHandler(aliceRandomHandler))
	bob := ts.newClient("bob", withRTDTRandomStreamHandler(bobRandomHandler))
	charlie := ts.newClient("charlie", withRTDTRandomStreamHandler(charlieRandomHandler))
	ts.kxUsers(alice, bob)
	ts.kxUsers(alice, charlie)
	ts.kxUsers(bob, charlie)

	// Handlers.
	aliceJoinedChan := make(chan struct{}, 5)
	alice.handle(client.OnRTDTJoinedInstantCall(func(sessRV zkidentity.ShortID) {
		aliceJoinedChan <- struct{}{}
	}))
	aliceJoinedLiveChan := make(chan struct{}, 5)
	alice.handle(client.OnRTDTLiveSessionJoined(func(sessRV zkidentity.ShortID) {
		aliceJoinedLiveChan <- struct{}{}
	}))

	bobInvitedChan := make(chan *rpc.RMRTDTSessionInvite, 5)
	bob.handle(client.OnInvitedToRTDTSession(func(ru *client.RemoteUser, invite *rpc.RMRTDTSessionInvite, ts time.Time) {
		bobInvitedChan <- invite
	}))
	bobJoinedChan := make(chan struct{}, 5)
	bob.handle(client.OnRTDTJoinedInstantCall(func(sessRV zkidentity.ShortID) {
		bobJoinedChan <- struct{}{}
	}))
	bobJoinedLiveChan := make(chan struct{}, 5)
	bob.handle(client.OnRTDTLiveSessionJoined(func(sessRV zkidentity.ShortID) {
		bobJoinedLiveChan <- struct{}{}
	}))

	charlieInvitedChan := make(chan *rpc.RMRTDTSessionInvite, 5)
	charlie.handle(client.OnInvitedToRTDTSession(func(ru *client.RemoteUser, invite *rpc.RMRTDTSessionInvite, ts time.Time) {
		charlieInvitedChan <- invite
	}))
	charlieJoinedChan := make(chan struct{}, 5)
	charlie.handle(client.OnRTDTJoinedInstantCall(func(sessRV zkidentity.ShortID) {
		charlieJoinedChan <- struct{}{}
	}))
	charlieJoinedLiveChan := make(chan struct{}, 5)
	charlie.handle(client.OnRTDTLiveSessionJoined(func(sessRV zkidentity.ShortID) {
		charlieJoinedLiveChan <- struct{}{}
	}))

	// Create the instant RTDT session.
	invitees := []clientintf.UserID{bob.PublicID(), charlie.PublicID()}
	initialSess, err := alice.CreateInstantRTDTSession(invitees)
	assert.NilErr(t, err)
	sessRV := initialSess.Metadata.RV

	// Bob and Charlie receive and accept the invite.
	gotBobInvite := assert.ChanWritten(t, bobInvitedChan)
	assert.NilErr(t, bob.AcceptRTDTSessionInvite(alice.PublicID(), gotBobInvite, true))
	gotCharlieInvite := assert.ChanWritten(t, charlieInvitedChan)
	assert.NilErr(t, charlie.AcceptRTDTSessionInvite(alice.PublicID(), gotCharlieInvite, true))

	// Alice, Bob and Charlie automatically join the live session.
	assert.ChanWritten(t, aliceJoinedChan)
	assert.ChanWritten(t, bobJoinedChan)
	assert.ChanWritten(t, charlieJoinedChan)
	assert.ChanWritten(t, aliceJoinedLiveChan)
	assert.ChanWritten(t, bobJoinedLiveChan)
	assert.ChanWritten(t, charlieJoinedLiveChan)

	// Ensure all updates have been sent between the clients.
	assertEmptyRMQ(t, alice)
	assertClientUpToDate(t, bob)
	assertClientUpToDate(t, charlie)
	assertClientHasNoRunningHandlers(t, alice)
	assertClientHasNoRunningHandlers(t, bob)
	assertClientHasNoRunningHandlers(t, charlie)

	// They exchange data.
	aliceSess := alice.GetLiveRTSession(&sessRV)
	bobSess := bob.GetLiveRTSession(&sessRV)
	charlieSess := charlie.GetLiveRTSession(&sessRV)
	assert.NotNil(t, aliceSess)
	assert.NotNil(t, bobSess)
	assert.NotNil(t, charlieSess)
	assertSendsRandomRTDTData(t, aliceSess.RTSess, gotBobDataChan, gotCharlieDataChan)
	assertSendsRandomRTDTData(t, bobSess.RTSess, gotAliceDataChan, gotCharlieDataChan)
	assertSendsRandomRTDTData(t, charlieSess.RTSess, gotAliceDataChan, gotBobDataChan)

	// Alice leaves the session. This automatically removes the session from
	// her list of sessions.
	assert.NilErr(t, alice.LeaveLiveRTSession(sessRV))
	_, err = alice.GetRTDTSession(&sessRV)
	assert.ErrorIs(t, err, clientdb.ErrNotFound)
	assert.Nil(t, alice.GetLiveRTSession(&sessRV))

	// Bob and Charlie still in the session.
	assertSendsRandomRTDTData(t, bobSess.RTSess, gotCharlieDataChan)
	assertSendsRandomRTDTData(t, charlieSess.RTSess, gotBobDataChan)

	// Bob exits the session. Same result.
	assert.NilErr(t, bob.LeaveLiveRTSession(sessRV))
	_, err = bob.GetRTDTSession(&sessRV)
	assert.ErrorIs(t, err, clientdb.ErrNotFound)
	assert.Nil(t, bob.GetLiveRTSession(&sessRV))

	// Charlie is shutdown. When he returns, he no longer has the session.
	charlie = ts.recreateClient(charlie)
	_, err = charlie.GetRTDTSession(&sessRV)
	assert.ErrorIs(t, err, clientdb.ErrNotFound)
}
