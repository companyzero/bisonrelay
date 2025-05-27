package rtdtclient

import (
	"context"
	"net"
	"slices"
	"testing"
	"time"

	"github.com/companyzero/bisonrelay/internal/assert"
	"github.com/companyzero/bisonrelay/rpc"
	"github.com/companyzero/bisonrelay/zkidentity"
)

// TestMultipleHandshakeAttempts tests that multiple handshake attempts are
// made if the server fails to respond in time.
func TestMultipleHandshakeAttempts(t *testing.T) {
	t.Parallel()

	ts := newTestServer(t)
	tc := ts.newClient("alice")

	// Start goroutine to initiate connection.
	resChan := make(chan interface{}, 1)
	go func() {
		conn, _, err := tc.c.connToAddrAndId(ts.ctx, ts.addr,
			ts.serverPub.Encapsulate, 1)
		if err != nil {
			resChan <- err
		} else {
			resChan <- conn
		}
	}()

	// First ciphertext read. Check, but discard this attempt. No return
	// from connToAddr() yet.
	cipher1, addr := ts.nextServerBytesAddr()
	var sessKey zkidentity.FixedSizeSymmetricKey
	if !ts.serverPriv.Decapsulate(cipher1, (*[32]byte)(&sessKey)) {
		t.Fatalf("Decapsulation of ciphertext into session key failed")
	}
	assert.ChanNotWritten(t, resChan, 10*time.Millisecond)

	// Read ciphertext again (client resent). Reply with invalid (not
	// encrypted) data.
	cipher2, _ := ts.nextServerBytesAddr()
	assert.DeepEqual(t, cipher1, cipher2)
	assert.ChanNotWritten(t, resChan, 10*time.Millisecond)
	_, err := ts.svr.WriteToUDP(randomData(1087), addr)
	assert.NilErr(t, err)

	// Read a third time. Reply with encrypted data that does not complete
	// the handshake.
	cipher3, _ := ts.nextServerBytesAddr()
	assert.DeepEqual(t, cipher1, cipher3)
	assert.ChanNotWritten(t, resChan, 10*time.Millisecond)
	_, err = ts.svr.WriteToUDP(encrypt(randomData(1047), &sessKey), addr)
	assert.NilErr(t, err)
	assert.ChanNotWritten(t, resChan, 10*time.Millisecond)

	// Read a fourth time. Reply with correct response. This completes the
	// handshake.
	cipher4, _ := ts.nextServerBytesAddr()
	assert.DeepEqual(t, cipher1, cipher4)
	assert.ChanNotWritten(t, resChan, 10*time.Millisecond)
	_, err = ts.svr.WriteToUDP(encrypt(cipher1, &sessKey), addr)
	assert.NilErr(t, err)
	res := assert.ChanWritten(t, resChan)
	if err, ok := res.(error); ok {
		t.Fatalf("Unexpected error: %v", err)
	}
}

// TestGivesUpHandshake tests that the connection/handshake attempt is canceled
// once the context is canceled.
func TestGivesUpHandshake(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	tc := ts.newClient("alice")

	ctx, cancel := context.WithCancel(ts.ctx)

	// Start goroutine to initiate connection.
	resChan := make(chan error, 1)
	go func() {
		_, _, err := tc.c.connToAddrAndId(ctx, ts.addr,
			ts.serverPub.Encapsulate, 1)
		resChan <- err
	}()

	// Server does not reply, client will keep trying to connect.
	assert.ChanNotWritten(t, ctx.Done(), time.Second)

	// Cancel attempt. This should error the connection attempt.
	cancel()
	res := assert.ChanWritten(t, resChan)
	assert.ErrorIs(t, res, context.Canceled)

}

// TestReconnectsAfterTimeout verifies that the client attempts to reconnect and
// resend session join attempts after it times out.
func TestReconnectsAfterTimeout(t *testing.T) {
	t.Parallel()

	const readTimeout = time.Second

	var id rpc.RTDTPeerID = 1
	ts := newTestServer(t)
	tc := ts.newClient("alice", withReadTimeout(readTimeout))
	tsess := tc.newHandshakedSession(id)
	tsess.sendRandomData(randomData(100), 0)

	sessJoinedChan := make(chan struct{}, 5)
	tsess.sess.sessionJoinedCb = func() {
		sessJoinedChan <- struct{}{}
	}

	// Assert there are no more messages in the server queue and wait until
	// the client will deem it necessary to reconnect.
	ts.assertNoServerInMsgs(readTimeout / 2)
	time.Sleep(readTimeout / 2)

	// Server should get a new handshake attempt. Reply with the session key
	// and recomplete handshake.
	cipher, addr := ts.nextServerBytesAddr()
	var newSessKey [32]byte
	if !ts.serverPriv.Decapsulate(cipher, &newSessKey) {
		t.Fatalf("Decapsulation of ciphertext into session key failed")
	}
	tsess.sessKey = (*zkidentity.FixedSizeSymmetricKey)(&newSessKey)
	tsess.localAddr = addr
	ts.serverWrite(addr, cipher, tsess.sessKey)

	// Server should get the session join message again.
	tsess.assertNextJoinCommand(id, tsess.lastJoinCookie)
	assert.ChanNotWritten(t, sessJoinedChan, 10*time.Millisecond)

	// Server sends reply.
	var joinReply rpc.RTDTServerCmdJoinSessionReply
	tsess.serverPkt.Sequence++
	tsess.serverPkt.Source = 0
	tsess.serverPkt.Target = id
	tsess.serverWrite(joinReply.AppendFramed(&tsess.serverPkt, nil))
	assert.ChanWritten(t, sessJoinedChan)
}

// TestReconnectsAfterWriteError verifies that the client attempts to reconnect
// and resend session join attempts after a write error (e.g. server went
// monentarily offline).
func TestReconnectsAfterWriteError(t *testing.T) {
	t.Parallel()

	var id rpc.RTDTPeerID = 1
	ts := newTestServer(t)
	tc := ts.newClient("alice")
	tsess := tc.newHandshakedSession(id)
	tsess.sendRandomData(randomData(100), 0)

	sessJoinedChan := make(chan struct{}, 5)
	tsess.sess.sessionJoinedCb = func() {
		sessJoinedChan <- struct{}{}
	}

	// Assert there are no more messages in the server queue.
	ts.assertNoServerInMsgs(time.Second)

	// Reach into the conn and add a dummy write deadline which will trigger
	// a write error on the next packet sent.
	tsess.sess.conn.socket.Load().c.SetWriteDeadline(time.Now())
	time.Sleep(time.Millisecond)

	// Next write by the client will fail.
	err := tsess.sess.SendRandomData(ts.ctx, randomData(100), 0)
	if err == nil {
		t.Fatalf("Unexpected nil error when sending to closed server")
	}

	// Server should get a new handshake attempt. Reply with the session key
	// and recomplete handshake.
	cipher, addr := ts.nextServerBytesAddr()
	var newSessKey [32]byte
	if !ts.serverPriv.Decapsulate(cipher, &newSessKey) {
		t.Fatalf("Decapsulation of ciphertext into session key failed")
	}
	tsess.sessKey = (*zkidentity.FixedSizeSymmetricKey)(&newSessKey)
	tsess.localAddr = addr
	ts.serverWrite(addr, cipher, tsess.sessKey)

	// Server should get the session join message again.
	tsess.assertNextJoinCommand(id, tsess.lastJoinCookie)
	assert.ChanNotWritten(t, sessJoinedChan, 10*time.Millisecond)

	// Server sends reply.
	var joinReply rpc.RTDTServerCmdJoinSessionReply
	tsess.serverPkt.Sequence++
	tsess.serverPkt.Source = 0
	tsess.serverPkt.Target = id
	tsess.serverWrite(joinReply.AppendFramed(&tsess.serverPkt, nil))
	assert.ChanWritten(t, sessJoinedChan)

	// Double check data is going through.
	tsess.sendRandomData(randomData(100), 0)
}

// TestReconnectsPending verifies that the client attempts to reconnect to a
// previously pending connection if a conn error happens during the joining
// attempt.
func TestReconnectsPending(t *testing.T) {
	t.Parallel()

	const readTimeout = time.Second

	var id rpc.RTDTPeerID = 1
	ts := newTestServer(t)
	tc := ts.newClient("alice", withReadTimeout(readTimeout))

	// Start connection and join attempt.
	scfg := SessionConfig{
		ServerAddr:    ts.addr,
		LocalID:       id,
		SessionKeyGen: ts.serverPub.Encapsulate,
		PublisherKey:  tc.pubKey,
		SigKey:        tc.sigPrivKey,
		JoinCookie:    randomData(10), // Random cookie to test relaying.
	}
	newSessChan := make(chan error, 5)
	go func() {
		_, err := tc.c.NewSession(context.Background(), scfg)
		newSessChan <- err
	}()

	// Server reads ciphertext.
	cipher, addr := ts.nextServerBytesAddr()
	var sessKey zkidentity.FixedSizeSymmetricKey
	if !ts.serverPriv.Decapsulate(cipher, (*[32]byte)(&sessKey)) {
		ts.t.Fatalf("Decapsulation of ciphertext into session key failed")
	}

	tsess := &testSession{
		t:              tc.ts.t,
		c:              tc,
		id:             id,
		lastJoinCookie: scfg.JoinCookie,
		localAddr:      addr,
		sessKey:        &sessKey,
	}

	// Server replies with ciphertext encoded with session key.
	tc.ts.serverWrite(addr, cipher, &sessKey)

	// Client sends the server command to join.
	tsess.assertNextJoinCommand(id, scfg.JoinCookie)

	// Note at this point the client is handshaked to the server, but has
	// NOT actually received a reply to join the session.
	assert.ChanNotWritten(t, newSessChan, 10*time.Millisecond)

	// Assert there are no more messages in the server queue and wait until
	// the client will deem it necessary to reconnect.
	ts.assertNoServerInMsgs(readTimeout / 2)
	time.Sleep(readTimeout / 2)

	// Server should get a new handshake attempt. Reply with the session key
	// and recomplete handshake.
	cipher, addr = ts.nextServerBytesAddr()
	var newSessKey [32]byte
	if !ts.serverPriv.Decapsulate(cipher, &newSessKey) {
		t.Fatalf("Decapsulation of ciphertext into session key failed")
	}
	tsess.sessKey = (*zkidentity.FixedSizeSymmetricKey)(&newSessKey)
	tsess.localAddr = addr
	ts.serverWrite(addr, cipher, tsess.sessKey)

	// Server should get the session join message again.
	tsess.assertNextJoinCommand(id, scfg.JoinCookie)
	assert.ChanNotWritten(t, newSessChan, 10*time.Millisecond)

	// Server sends reply acking the join.
	var joinReply rpc.RTDTServerCmdJoinSessionReply
	tsess.serverPkt.Sequence++
	tsess.serverPkt.Source = 0
	tsess.serverPkt.Target = id
	tsess.serverWrite(joinReply.AppendFramed(&tsess.serverPkt, nil))

	// Client completed the join.
	assert.ChanWrittenWithVal(t, newSessChan, error(nil))
}

// TestSendsPing tests that the client sends ping commands to the server.
func TestSendsPing(t *testing.T) {
	t.Parallel()

	const pingInterval = time.Second

	rttChan := make(chan struct{}, 5)
	rttCb := func(_ net.UDPAddr, rtt time.Duration) {
		rttChan <- struct{}{}
	}

	var id rpc.RTDTPeerID = 1
	ts := newTestServer(t)
	tc := ts.newClient("alice", withPingInterval(pingInterval), WithPingRTTCalculated(rttCb))
	tsess := tc.newHandshakedSession(id)
	tsess.sendRandomData(randomData(100), 0)

	// After the ping interval, the client should send a new ping command.
	time.Sleep(pingInterval)
	gotId, cmd, cmdPayload := tsess.nextServerCmd()
	assert.DeepEqual(t, gotId, id)
	assert.DeepEqual(t, cmd, rpc.RTDTServerCmdTypePing)
	var cmdPing rpc.RTDTPingCmd
	assert.NilErr(t, cmdPing.FromBytes(cmdPayload))

	// Server replies.
	tsess.serverSendCmd(&rpc.RTDTPongCmd{Data: slices.Clone(cmdPing.Data)})
	assert.ChanWritten(t, rttChan)
}

// TestLeavesSession tests that the client can leave a session.
func TestLeavesSession(t *testing.T) {
	t.Parallel()

	var id rpc.RTDTPeerID = 1
	ts := newTestServer(t)
	tc := ts.newClient("alice")

	tsess := tc.newHandshakedSession(id)
	tsess.sendRandomData(randomData(100), 0)

	// Start to leave.
	leaveErrChan := make(chan error, 1)
	go func() {
		leaveErrChan <- tc.c.LeaveSession(context.Background(), tsess.sess)
	}()

	// Server gets a command to leave.
	assert.ChanNotWritten(t, leaveErrChan, time.Second)
	_, cmd, _ := tsess.nextServerCmd()
	assert.DeepEqual(t, id, tsess.id)
	assert.DeepEqual(t, cmd, rpc.RTDTServerCmdTypeLeaveSession)

	// Server replies.
	tsess.serverSendCmd(&rpc.RTDTServerCmdLeaveSessionReply{})
	assert.ChanWrittenWithVal(t, leaveErrChan, nil)

	// Attempting to send data now fails.
	err := tsess.sess.SendRandomData(context.Background(), randomData(10), 0)
	assert.ErrorIs(t, err, errLeftSession)
}

// TestLeavesSessionMultipleTries tests that the client tries multiple times to
// leave the session before giving up.
func TestLeavesSessionMultipleTries(t *testing.T) {
	t.Parallel()

	const maxTries = 3
	const replyTimeout = time.Second + 500*time.Millisecond

	var id rpc.RTDTPeerID = 1
	ts := newTestServer(t)
	tc := ts.newClient("alice", withLeaveAttemptOptions(maxTries, replyTimeout))

	tsess := tc.newHandshakedSession(id)
	tsess.sendRandomData(randomData(100), 0)

	// Start to leave.
	leaveErrChan := make(chan error, 1)
	go func() {
		leaveErrChan <- tc.c.LeaveSession(context.Background(), tsess.sess)
	}()

	// Server gets multiple commands to leave.
	for i := 0; i < maxTries; i++ {
		assert.ChanNotWritten(t, leaveErrChan, 500*time.Millisecond)
		_, cmd, _ := tsess.nextServerCmd()
		assert.DeepEqual(t, id, tsess.id)
		assert.DeepEqual(t, cmd, rpc.RTDTServerCmdTypeLeaveSession)
	}

	// Caller got an error describing the failed attempts.
	err := assert.ChanWritten(t, leaveErrChan)
	assert.ErrorIs(t, err, errLeaveSessNoReply)

	// Attempting to send data does not fail (because client did not
	// receive a reply).
	tsess.sendRandomData(randomData(100), 0)
}

// TestKickFromSession tests that the client tries multiple times to
// kick a target from a session and receives a reply.
func TestKickFromSession(t *testing.T) {
	t.Parallel()

	const maxTries = 3
	const replyTimeout = time.Second + 500*time.Millisecond

	var id rpc.RTDTPeerID = 1
	ts := newTestServer(t)
	tc := ts.newClient("alice", withKickAttemptOptions(maxTries, replyTimeout))

	tsess := tc.newHandshakedSession(id)
	tsess.sendRandomData(randomData(100), 0)

	// Start to kick.
	kickErrChan := make(chan error, 1)
	kickTarget, kickDuration := rpc.RTDTPeerID(0x70000001), time.Hour
	go func() {
		kickErrChan <- tc.c.KickMember(context.Background(), tsess.sess,
			kickTarget, kickDuration)
	}()

	// Server gets multiple commands to kick.
	for i := 0; i < maxTries-1; i++ {
		assert.ChanNotWritten(t, kickErrChan, 500*time.Millisecond)
		_, cmdType, cmdPayload := tsess.nextServerCmd()
		assert.DeepEqual(t, id, tsess.id)
		assert.DeepEqual(t, cmdType, rpc.RTDTServerCmdTypeKickPeer)
		var cmd rpc.RTDTServerCmdKickPeer
		assert.NilErr(t, cmd.FromBytes(cmdPayload))
		assert.DeepEqual(t, cmd.BanDurationSeconds, uint32(kickDuration.Seconds()))
		assert.DeepEqual(t, cmd.KickTarget, kickTarget)
	}

	// Server replies.
	tsess.serverSendCmd(&rpc.RTDTServerCmdKickPeerReply{KickTarget: kickTarget})
	assert.ChanWrittenWithVal(t, kickErrChan, nil)
}
