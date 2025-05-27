package rtdtserver

import (
	"testing"
	"time"

	"github.com/companyzero/bisonrelay/internal/assert"
	"github.com/companyzero/bisonrelay/rpc"
	"github.com/companyzero/bisonrelay/zkidentity"
)

// TestBindToSameSession tests that binding from two different addresses to
// the same session using the same peer ID works as expected (last bound client
// receives data).
func TestBindToSameSession(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)

	// Alice will send data in a session, Bob will receive it.
	alice, bob := ts.newClient(), ts.newClient()
	var aliceId, bobId rpc.RTDTPeerID = 1, 2
	alice.joinSession(aliceId)
	bob.joinSession(bobId)
	data1 := []byte("data 1 from alice")
	alice.sendRandomData(aliceId, data1, 0)
	bob.assertNextRandomData(alice, data1)

	// Charlie will join the same session using Bob's id (overriding Bob).
	// He should be the one to get the data now.
	charlie := ts.newClient()
	charlie.joinSession(bobId)
	data2 := []byte("data 2 from alice")
	alice.sendRandomData(aliceId, data2, 0)
	charlie.assertNextRandomData(alice, data2)

	// Bob should *NOT* get data (the next read will timeout).
	assertNoData(t, bob)

	// Bob goes back to the session. He should get the data now, but not
	// Charlie.
	bob.joinSession(bobId)
	data3 := []byte("data 3 from alice")
	alice.sendRandomData(aliceId, data3, 0)
	bob.assertNextRandomData(alice, data3)
	assertNoData(t, charlie)
}

// TestValidCookieKey tests that clients communicating with valid join cookies
// work.
func TestValidCookies(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t, withTestCookieKey())

	// Alice will send data in a session, Bob will receive it.
	alice, bob := ts.newClient(), ts.newClient()
	aliceSess := alice.joinSession(1)
	aliceSess.assertErrCode(t, errCodeNoError)
	bobSess := bob.joinSession(2)
	bobSess.assertErrCode(t, errCodeNoError)
	assertCanExchangeData(t, aliceSess, bobSess)
}

// TestExpiredCookie tests that trying to use a cookie that has expired does not
// work.
func TestExpiredCookie(t *testing.T) {
	t.Parallel()
	dropPaymentInterval := time.Second
	ts := newTestServer(t, withTestCookieKey(), withDropPaymentLoopInterval(dropPaymentInterval))

	var aliceId, bobId rpc.RTDTPeerID = 1, 2
	alice, bob := ts.newClient(), ts.newClient()
	aliceSess := alice.joinSession(aliceId)

	// Bob will generate a cookie but fail to use it until it's expired.
	bobJc := ts.validJoinCookie(bobId)
	bobCookie := bobJc.Encrypt(nil, ts.cookieKey)
	sleepDur := time.Until(time.Unix(bobJc.EndTimestamp, 0)) + dropPaymentInterval
	if sleepDur > 0 {
		time.Sleep(sleepDur)
	}
	bobSess := bob.joinSessionWithCookie(bobId, bobCookie)
	bobSess.assertErrCode(t, errCodeJoinCookieExpired)

	// Alice sends data but bob does not receive it.
	aliceSess.sendRandomData([]byte("data 1 from alice"), 0)
	assertNoData(t, bob)

	// Bob joins with an updated cookie.
	bobJc.EndTimestamp += 120
	bobSess = bob.joinSessionWithCookie(bobId, bobJc.Encrypt(nil, ts.cookieKey))
	bobSess.assertErrCode(t, errCodeNoError)
	assertCanExchangeData(t, aliceSess, bobSess)
}

// TestWrongPeerIDCookie tests that trying to use a cookie that has the wrong
// cookie id encoded does not work.
func TestWrongPeerIDCookie(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t, withTestCookieKey())

	var aliceId, bobId rpc.RTDTPeerID = 1, 2
	alice, bob := ts.newClient(), ts.newClient()
	aliceSess := alice.joinSession(aliceId)

	// Bob will generate a cookie but use a wrong peer id in it.
	bobJc := ts.validJoinCookie(bobId + 100)
	bobCookie := bobJc.Encrypt(nil, ts.cookieKey)
	bobSess := bob.joinSessionWithCookie(bobId, bobCookie)
	bobSess.assertErrCode(t, errCodeJoinCookieWrongPeerID)

	// Alice sends data but bob does not receive it.
	aliceSess.sendRandomData([]byte("data 1 from alice"), 0)
	assertNoData(t, bob)

	// Bob joins with an updated cookie.
	bobJc.PeerID = bobId
	bobSess = bob.joinSessionWithCookie(bobId, bobJc.Encrypt(nil, ts.cookieKey))
	bobSess.assertErrCode(t, errCodeNoError)
	assertCanExchangeData(t, aliceSess, bobSess)
}

// TestInvalidCookie tests that trying to use a cookie that does not correctly
// decrypt will fail.
func TestInvalidCookie(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t, withTestCookieKey())

	var aliceId, bobId rpc.RTDTPeerID = 1, 2
	alice, bob := ts.newClient(), ts.newClient()
	aliceSess := alice.joinSession(aliceId)

	// Bob will send invalid cookies. First one: broken validation tag.
	bobJc := ts.validJoinCookie(bobId)
	bobCookie := bobJc.Encrypt(nil, ts.cookieKey)
	bobCookie[len(bobCookie)-1] += 1
	bobSess := bob.joinSessionWithCookie(bobId, bobCookie)
	bobSess.assertErrCode(t, errCodeJoinCookieInvalid)

	// Broken nonce.
	bobCookie = bobJc.Encrypt(nil, ts.cookieKey)
	bobCookie[0] += 1
	bobSess = bob.joinSessionWithCookie(bobId, bobCookie)
	bobSess.assertErrCode(t, errCodeJoinCookieInvalid)

	// Encrypted with a different key.
	bobCookie = bobJc.Encrypt(nil, zkidentity.NewFixedSizeSymmetricKey())
	bobSess = bob.joinSessionWithCookie(bobId, bobCookie)
	bobSess.assertErrCode(t, errCodeJoinCookieInvalid)

	// Broken mid data.
	bobCookie = bobJc.Encrypt(nil, ts.cookieKey)
	bobCookie[len(bobCookie)/2] += 1
	bobSess = bob.joinSessionWithCookie(bobId, bobCookie)
	bobSess.assertErrCode(t, errCodeJoinCookieInvalid)

	// Extra data
	bobCookie = bobJc.Encrypt(nil, ts.cookieKey)
	bobCookie = append(bobCookie, 0)
	bobSess = bob.joinSessionWithCookie(bobId, bobCookie)
	bobSess.assertErrCode(t, errCodeJoinCookieInvalid)

	// Too little data.
	bobCookie = bobJc.Encrypt(nil, ts.cookieKey)
	bobCookie = bobCookie[:len(bobCookie)-1]
	bobSess = bob.joinSessionWithCookie(bobId, bobCookie)
	bobSess.assertErrCode(t, errCodeJoinCookieInvalid)

	// No data.
	bobSess = bob.joinSessionWithCookie(bobId, nil)
	bobSess.assertErrCode(t, errCodeJoinCookieInvalid)

	// Alice sends data but bob does not receive it.
	aliceSess.sendRandomData([]byte("data 1 from alice"), 0)
	assertNoData(t, bob)

	// Bob joins with the correct cookie.
	bobSess = bob.joinSessionWithCookie(bobId, bobJc.Encrypt(nil, ts.cookieKey))
	bobSess.assertErrCode(t, errCodeNoError)
	assertCanExchangeData(t, aliceSess, bobSess)
}

// TestOldCookieKey tests that the server can decode a cookie encrypted with an
// old key.
func TestOldCookieKey(t *testing.T) {
	t.Parallel()

	decodeKeys := []*zkidentity.FixedSizeSymmetricKey{{0x00: 0x0a}}

	ts := newTestServer(t, withTestCookieKey(), withDecodeCookieKeys(decodeKeys))

	var aliceId, bobId rpc.RTDTPeerID = 1, 2
	alice, bob := ts.newClient(), ts.newClient()
	aliceSess := alice.joinSession(aliceId)

	// Bob will join with a cookie encrypted with an old key.
	bobJc := ts.validJoinCookie(bobId)
	bobCookie := bobJc.Encrypt(nil, decodeKeys[0])
	bobSess := bob.joinSessionWithCookie(bobId, bobCookie)
	bobSess.assertErrCode(t, errCodeNoError)

	assertCanExchangeData(t, aliceSess, bobSess)
}

// TestValidDuplicateJoin tests the case where the client sends the same cookie
// twice in a row. This may happen if the client fails to receive the reply.
func TestValidDuplicateJoin(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t, withTestCookieKey())

	var aliceId, bobId rpc.RTDTPeerID = 1, 2
	alice, bob := ts.newClient(), ts.newClient()
	aliceSess := alice.joinSession(aliceId)

	// First join: everything ok.
	bobJc := ts.validJoinCookie(bobId)
	bobCookie := ts.encryptJoinCookie(&bobJc)
	bobSess := bob.joinSessionWithCookie(bobId, bobCookie)
	bobSess.assertErrCode(t, errCodeNoError)
	assertCanExchangeData(t, aliceSess, bobSess)

	// Second join: keeps working.
	bobSess = bob.joinSessionWithCookie(bobId, bobCookie)
	bobSess.assertErrCode(t, errCodeNoError)
	assertCanExchangeData(t, aliceSess, bobSess)
}

// TestServerPong tests how the server replies to ping messages.
func TestServerPong(t *testing.T) {
	t.Parallel()

	minPingInterval := time.Second * 5
	ts := newTestServer(t, withPingInterval(minPingInterval*2, minPingInterval))
	data1 := randomData(rpc.RTDTMaxPingPayloadSize)

	// Send the first ping.
	outPkt := rpc.RTDTFramedPacket{Source: 1001, Sequence: 1}
	pingCmd := rpc.RTDTPingCmd{Data: data1}
	alice := ts.newClient()
	alice.write(pingCmd.AppendFramed(&outPkt, nil))

	// Receive the first pong reply.
	in, err := alice.readNext()
	assert.NilErr(t, err)
	var inPkt rpc.RTDTFramedPacket
	assert.NilErr(t, inPkt.FromBytes(in))
	assert.DeepEqual(t, inPkt.Target, outPkt.Source)
	assert.DeepEqual(t, inPkt.Source, outPkt.Target)
	inCmd, inPayload := inPkt.ServerCmd()
	assert.DeepEqual(t, inCmd, rpc.RTDTServerCmdTypePong)
	var inPong rpc.RTDTPongCmd
	assert.NilErr(t, inPong.FromBytes(inPayload))
	assert.DeepEqual(t, inPong.Data, data1)

	// A second ping, immediately after the first one does not generate a
	// reply.
	outPkt.Sequence++
	alice.write(pingCmd.AppendFramed(&outPkt, nil))
	alice.assertNextReadTimesOut(minPingInterval)

	// After waiting the mininum ping interval, the next ping should
	// generate a reply.
	data2 := randomData(rpc.RTDTMaxPingPayloadSize)
	pingCmd.Data = data2
	outPkt.Sequence++
	alice.write(pingCmd.AppendFramed(&outPkt, nil))
	in, err = alice.readNext()
	assert.NilErr(t, err)
	assert.NilErr(t, inPkt.FromBytes(in))
	assert.DeepEqual(t, inPkt.Target, outPkt.Source)
	assert.DeepEqual(t, inPkt.Source, outPkt.Target)
	inCmd, inPayload = inPkt.ServerCmd()
	assert.DeepEqual(t, inCmd, rpc.RTDTServerCmdTypePong)
	assert.NilErr(t, inPong.FromBytes(inPayload))
	assert.DeepEqual(t, inPong.Data, data2)
}

// TestPingBanScore tests sending too many pings causes a disconnection.
func TestPingBanScore(t *testing.T) {
	t.Parallel()

	loopTickerInterval := time.Second
	ts := newTestServer(t, withTimeoutLoopTickerInterval(loopTickerInterval))

	var aliceId, bobId rpc.RTDTPeerID = 1, 2
	alice, bob := ts.newClient(), ts.newClient()
	aliceSess := alice.joinSession(aliceId)
	bobSess := bob.joinSession(bobId)
	assertCanExchangeData(t, aliceSess, bobSess)

	// Send as many pings as needed to cause bob to be disconnected.
	bob.pkt.Source = 1001
	for i := 0; i <= int(ts.s.cfg.maxBanScore); i++ {
		bob.pkt.Sequence++
		pingCmd := rpc.RTDTPingCmd{}
		bob.write(pingCmd.AppendFramed(&bob.pkt, nil))
		time.Sleep(time.Microsecond)
	}

	// Receive the first pong reply.
	_, err := bob.readNext()
	assert.NilErr(t, err)

	// Wait for Bob to be removed.
	time.Sleep(loopTickerInterval * 2)

	// Bob can no longer receive data.
	alice.sendRandomData(aliceId, randomData(100), 0)
	assertNoData(t, bob)
}

// TestPingBanScoreMaxPayload tests sending pings with too large payloads causes
// a disconnection.
func TestPingBanScoreMaxPayload(t *testing.T) {
	t.Parallel()

	minPingInterval := time.Millisecond
	loopTickerInterval := time.Second
	ts := newTestServer(t, withPingInterval(loopTickerInterval, minPingInterval))

	var aliceId, bobId rpc.RTDTPeerID = 1, 2
	alice, bob := ts.newClient(), ts.newClient()
	aliceSess := alice.joinSession(aliceId)
	bobSess := bob.joinSession(bobId)
	assertCanExchangeData(t, aliceSess, bobSess)

	// Send as many pings as needed to cause bob to be disconnected due to
	// payload size.
	bob.pkt.Source = 1001
	pingCmd := rpc.RTDTPingCmd{Data: randomData(rpc.RTDTMaxPingPayloadSize + 1)}
	for i := 0; i <= int(ts.s.cfg.maxBanScore); i++ {
		bob.pkt.Sequence++
		bob.write(pingCmd.AppendFramed(&bob.pkt, nil))
		time.Sleep(minPingInterval)
	}

	time.Sleep(loopTickerInterval)

	// Bob can no longer receive data.
	alice.sendRandomData(aliceId, randomData(100), 0)
	assertNoData(t, bob)
}

// TestLeaveSession tests a peer leaving a session.
func TestLeaveSession(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)

	// Join 2 different sessions.
	var aliceId, bobId rpc.RTDTPeerID = 1, 2
	alice, bob := ts.newClient(), ts.newClient()
	aliceSess := alice.joinSession(aliceId)
	bobSess := bob.joinSession(bobId)

	var aliceId2, bobId2 rpc.RTDTPeerID = 1<<17 + 1, 1<<17 + 2
	aliceSess2 := alice.joinSession(aliceId2)
	bobSess2 := bob.joinSession(bobId2)

	assertCanExchangeData(t, aliceSess, bobSess)
	assertCanExchangeData(t, aliceSess2, bobSess2)

	// Leave the Second session.
	leaveCmd := rpc.RTDTServerCmdLeaveSession{}
	bob.pkt.Sequence++
	bob.pkt.Target = 0
	bob.pkt.Source = bobId2
	bob.write(leaveCmd.AppendFramed(&bob.pkt, nil))
	inData, err := bob.readNext()
	assert.NilErr(t, err)
	var inPkt rpc.RTDTFramedPacket
	assert.NilErr(t, inPkt.FromBytes(inData))
	assert.DeepEqual(t, inPkt.Target, bobId2)
	gotCmd, _ := inPkt.ServerCmd()
	assert.DeepEqual(t, gotCmd, rpc.RTDTServerCmdTypeLeaveSessionReply)

	// The session which Bob left cannot receive new data, but the one where
	// he is still present still works.
	assertCanExchangeData(t, aliceSess, bobSess)
	aliceSess2.sendRandomData([]byte("alice data"), 0)
	assertNoData(t, bob)
}

// TestKickFromSession tests kicking an user from a session.
func TestKickFromSession(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t, withTestCookieKey())

	var aliceId, bobId rpc.RTDTPeerID = 1, 2
	alice, bob := ts.newClient(), ts.newClient()

	// Alice cannot kick Bob yet (she is not in session).
	aliceKickCmd1 := &rpc.RTDTServerCmdKickPeer{KickTarget: bobId}
	alice.sendCmd(t, aliceId, aliceKickCmd1)
	var aliceKickReply1 rpc.RTDTServerCmdKickPeerReply
	alice.assertNextCmd(t, aliceId, rpc.RTDTServerCmdTypeKickPeerReply, &aliceKickReply1)
	assert.DeepEqual(t, aliceKickReply1.ErrCode, uint64(errCodeSourcePeerNotInSession))

	// Alice joins session as admin.
	aliceJc := alice.ts.validJoinCookie(aliceId)
	aliceJc.IsAdmin = true
	aliceCookie := ts.encryptJoinCookie(&aliceJc)
	aliceSess := alice.joinSessionWithCookie(aliceId, aliceCookie)
	aliceSess.assertErrCode(t, errCodeNoError)

	// Alice cannot kick Bob yet (he's not in session).
	aliceKickCmd2 := &rpc.RTDTServerCmdKickPeer{KickTarget: bobId}
	aliceSess.sendCmd(t, aliceKickCmd2)
	var aliceKickReply2 rpc.RTDTServerCmdKickPeerReply
	aliceSess.assertNextCmd(t, rpc.RTDTServerCmdTypeKickPeerReply, &aliceKickReply2)
	assert.DeepEqual(t, aliceKickReply2.ErrCode, uint64(errCodeTargetPeerNotInSession))

	// Bob joins session.
	bobSess := bob.joinSession(bobId)
	bobSess.assertErrCode(t, errCodeNoError)
	assertCanExchangeData(t, aliceSess, bobSess)

	// Bob cannot kick Alice (he's not an admin).
	bobKickCmd := rpc.RTDTServerCmdKickPeer{KickTarget: aliceId}
	bob.pkt.Sequence++
	bob.pkt.Target = 0
	bob.pkt.Source = bobId
	bob.write(bobKickCmd.AppendFramed(&bob.pkt, nil))
	var bobKickReply rpc.RTDTServerCmdKickPeerReply
	bobSess.assertNextCmd(t, rpc.RTDTServerCmdTypeKickPeerReply, &bobKickReply)
	assert.DeepEqual(t, bobKickReply.ErrCode, uint64(errCodeSourcePeerNotAdmin))

	// Alice kicks bob.
	aliceKickCmd3 := &rpc.RTDTServerCmdKickPeer{KickTarget: bobId, BanDurationSeconds: 0}
	aliceSess.sendCmd(t, aliceKickCmd3)
	var aliceKickReply3 rpc.RTDTServerCmdKickPeerReply
	aliceSess.assertNextCmd(t, rpc.RTDTServerCmdTypeKickPeerReply, &aliceKickReply3)
	assert.DeepEqual(t, aliceKickReply3.ErrCode, uint64(errCodeNoError))
	var bobKickedReport rpc.RTDTServerCmdKickPeer
	bobSess.assertNextCmd(t, rpc.RTDTServerCmdTypeKickPeer, &bobKickedReport)

	// Messages are not exchanged anymore.
	assertCannotExchangeData(t, aliceSess, bobSess)

	// Bob rejoins session.
	bobSess2 := bob.joinSession(bobId)
	bobSess2.assertErrCode(t, errCodeNoError)
	assertCanExchangeData(t, aliceSess, bobSess2)

	// Alice kicks and bans bob.
	banSeconds := 4
	aliceKickCmd4 := &rpc.RTDTServerCmdKickPeer{KickTarget: bobId, BanDurationSeconds: uint32(banSeconds)}
	aliceSess.sendCmd(t, aliceKickCmd4)
	banStart := time.Now()
	var aliceKickReply4 rpc.RTDTServerCmdKickPeerReply
	aliceSess.assertNextCmd(t, rpc.RTDTServerCmdTypeKickPeerReply, &aliceKickReply4)
	assert.DeepEqual(t, aliceKickReply4.ErrCode, uint64(errCodeNoError))
	bobSess.assertNextCmd(t, rpc.RTDTServerCmdTypeKickPeer, &bobKickedReport)
	assertCannotExchangeData(t, aliceSess, bobSess)

	// Bob cannot join.
	bobSess3 := bob.joinSession(bobId)
	bobSess3.assertErrCode(t, errCodeBanned)
	assertCannotExchangeData(t, aliceSess, bobSess)

	// Wait until ban times out.
	time.Sleep(time.Second*time.Duration(banSeconds) - time.Since(banStart))

	// Bob can rejoin after ban is lifted.
	bobSess4 := bob.joinSession(bobId)
	bobSess4.assertErrCode(t, errCodeNoError)
	assertCanExchangeData(t, aliceSess, bobSess4)
}
