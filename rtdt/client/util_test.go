package rtdtclient

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"net"
	"net/netip"
	"os"
	"testing"
	"time"

	"github.com/companyzero/bisonrelay/internal/assert"
	"github.com/companyzero/bisonrelay/internal/testutils"
	"github.com/companyzero/bisonrelay/rpc"
	"github.com/companyzero/bisonrelay/zkidentity"
	"github.com/decred/slog"
	"golang.org/x/crypto/nacl/secretbox"
)

// inServerMsg is a message received by the server (from one of the test
// clients).
type inServerMsg struct {
	addr *net.UDPAddr
	msg  []byte
}

// testSession encapsulates an RTDT session to test it.
type testSession struct {
	t         testing.TB
	c         *testClient
	id        rpc.RTDTPeerID
	localAddr *net.UDPAddr
	sessKey   *zkidentity.FixedSizeSymmetricKey
	sess      *Session

	lastJoinCookie []byte

	serverPkt rpc.RTDTFramedPacket
}

func (tsess *testSession) Fatalf(format string, args ...interface{}) {
	tsess.t.Helper()
	tsess.t.Fatalf(format, args...)
}

// serverWrite sends a message from the server encoded with the key for this
// session.
func (tsess *testSession) serverWrite(msg []byte) {
	tsess.t.Helper()
	tsess.c.ts.serverWrite(tsess.localAddr, msg, tsess.sessKey)
}

// frameableCmd are RPC commands that can be sent framed.
type frameableCmd interface {
	AppendFramed(pkt *rpc.RTDTFramedPacket, b []byte) []byte
}

// serverSendCmd sends a command from the server, encoded with the key for
// this session.
func (tsess *testSession) serverSendCmd(c frameableCmd) {
	tsess.t.Helper()

	tsess.serverPkt.Sequence++
	tsess.serverPkt.Source = 0
	tsess.serverPkt.Target = tsess.id
	tsess.serverWrite(c.AppendFramed(&tsess.serverPkt, nil))
}

// serverSendData sends data from the server as if the given remote peer had
// sent it.
//
//nolint:unused
func (tsess *testSession) serverSendDataPkt(source rpc.RTDTPeerID, pkt *rpc.RTDTDataPacket) {
	tsess.serverPkt.Sequence++
	tsess.serverPkt.Source = source
	tsess.serverPkt.Target = tsess.id
	tsess.serverWrite(pkt.AppendFramed(&tsess.serverPkt, nil))
}

// sendRandomData sends random data from the client in this session and asserts
// that the data is correctly received in the server.
func (tsess *testSession) sendRandomData(data []byte, ts uint32) {
	tsess.c.ts.t.Helper()
	ctx, cancel := context.WithTimeout(tsess.c.ts.ctx, time.Second)
	defer cancel()
	err := tsess.sess.SendRandomData(ctx, data, ts)
	if err != nil {
		tsess.Fatalf("Unable to send data in session: %v", err)
	}

	// The server should receive this packet of data, properly encrypted and
	// signed (if these were setup).
	inBytes := tsess.nextServerBytes()

	var pkt rpc.RTDTFramedPacket
	if err := pkt.FromBytes(inBytes); err != nil {
		tsess.Fatalf("Error decoding server data into framed packet: %v", err)
	}

	if pkt.Source != pkt.Target {
		tsess.Fatalf("Received internal command when expected actual data")
	}

	if pkt.Target != tsess.id {
		tsess.Fatalf("Received packet with target %s when session id is %s",
			pkt.Target, tsess.id)
	}

	var dataPkt rpc.RTDTDataPacket
	if err := dataPkt.Decrypt(pkt, tsess.c.pubKey, tsess.c.sigPubKey); err != nil {
		tsess.Fatalf("Decryption of data packet failed: %v", err)
	}
	if dataPkt.Stream != rpc.RTDTStreamRandom {
		tsess.Fatalf("Unexpected stream: got %d, want %d", dataPkt.Stream, rpc.RTDTStreamRandom)
	}
	if dataPkt.Timestamp != ts {
		tsess.Fatalf("Unexpceted timestamp: got %d, want %d", dataPkt.Timestamp, ts)
	}
	if !bytes.Equal(dataPkt.Data, data) {
		tsess.Fatalf("Unexpected data: got %x, want %x", dataPkt.Data, data)
	}
}

// nextServerBytes receives the next message from the server.
func (tsess *testSession) nextServerBytes() []byte {
	tsess.t.Helper()
	data, _ := tsess.c.ts.nextServerBytesAddr()
	return decrypt(tsess.t, data, tsess.sessKey)
}

// nextFramedPacket returns the next message received by the server as a framed
// packet.
func (tsess *testSession) nextFramedPacket() rpc.RTDTFramedPacket {
	tsess.t.Helper()
	data := tsess.nextServerBytes()
	var pkt rpc.RTDTFramedPacket
	if err := pkt.FromBytes(data); err != nil {
		tsess.t.Fatalf("Unable to decode framed packet in server: %v", err)
	}

	return pkt
}

// nextServerCmd returns the next message received by the server as an internal
// server command.
func (tsess *testSession) nextServerCmd() (rpc.RTDTPeerID, rpc.RTDTServerCmdType, []byte) {
	tsess.t.Helper()
	pkt := tsess.nextFramedPacket()
	if pkt.Source == pkt.Target {
		tsess.t.Fatalf("Next server message is not an internal command")
	}
	cmd, payload := pkt.ServerCmd()
	if cmd == 0 {
		tsess.t.Fatalf("Unable to identify next server command")
	}

	return pkt.Source, cmd, payload
}

// assertNextJoinCommand verifies the server receives a command to join a
// session with the given ID and join cookie.
func (tsess *testSession) assertNextJoinCommand(id rpc.RTDTPeerID, cookie []byte) {
	tsess.t.Helper()
	gotId, cmd, payload := tsess.nextServerCmd()
	if gotId != id {
		tsess.t.Fatalf("Unexpected source id: got %s, want %s", gotId, id)
	}
	if cmd != rpc.RTDTServerCmdTypeJoinSession {
		tsess.t.Fatalf("Unexpected join command: got %d, want %d", cmd, rpc.RTDTServerCmdTypeJoinSession)
	}

	var joinCmd rpc.RTDTServerCmdJoinSession
	if err := joinCmd.FromBytes(payload); err != nil {
		tsess.t.Fatalf("Unexpected error decoding join cmd: %v", err)
	}

	if !bytes.Equal(cookie, joinCmd.JoinCookie) {
		tsess.t.Fatalf("Unexpected join cookie: got %x, want %x", joinCmd.JoinCookie, cookie)
	}
}

// testClient encapsulates an RTDT client to test it.
type testClient struct {
	ts   *testScaffold
	c    *Client
	name string

	pubKey     *zkidentity.FixedSizeSymmetricKey
	sigPrivKey *zkidentity.FixedSizeEd25519PrivateKey
	sigPubKey  *zkidentity.FixedSizeEd25519PublicKey
}

// willReuseConnTo returns the conn that will be reused to connect to the given
// address and target session ID (if any will be reused). This is not safe from
// races in general and should only be used in tests.
func (c *Client) willReuseConnTo(addr *net.UDPAddr, targetId rpc.RTDTPeerID) *conn {
	var selConn *conn
	c.conns.Compute(addr.String(), func(conns []*conn, loaded bool) ([]*conn, bool) {
		for i := range conns {
			conns[i].mtx.Lock()
			_, contains := conns[i].sessions[targetId]
			conns[i].mtx.Unlock()

			if !contains {
				// Found one!
				selConn = conns[i]
				break
			}
		}

		return conns, false
	})

	return selConn
}

// newHandshakedSession creates a new session and completes the handshake with
// the scaffold server.
func (tc *testClient) newHandshakedSession(id rpc.RTDTPeerID) *testSession {
	tc.ts.t.Helper()
	ts := tc.ts

	scfg := SessionConfig{
		ServerAddr:    tc.ts.addr,
		LocalID:       id,
		SessionKeyGen: tc.ts.serverPub.Encapsulate,
		PublisherKey:  tc.pubKey,
		SigKey:        tc.sigPrivKey,
		JoinCookie:    randomData(10), // Random cookie to test relaying.
	}

	oldConn := tc.c.willReuseConnTo(tc.ts.addr, id)

	resChan := make(chan interface{}, 1)
	go func() {
		sess, err := tc.c.NewSession(tc.ts.ctx, scfg)
		if err != nil {
			resChan <- err
		} else {
			resChan <- sess
		}
	}()

	tsess := &testSession{
		t:              tc.ts.t,
		c:              tc,
		id:             id,
		lastJoinCookie: scfg.JoinCookie,
	}

	// If the client did not have a conn to the server already, it will
	// perform a handshake.
	if oldConn == nil {
		// Server reads ciphertext.
		cipher, addr := ts.nextServerBytesAddr()
		var sessKey zkidentity.FixedSizeSymmetricKey
		if !ts.serverPriv.Decapsulate(cipher, (*[32]byte)(&sessKey)) {
			ts.t.Fatalf("Decapsulation of ciphertext into session key failed")
		}
		tsess.localAddr = addr

		// Server replies with ciphertext encoded with session key.
		tsess.sessKey = &sessKey
		tc.ts.serverWrite(addr, cipher, &sessKey)
	} else {
		// Reuse conn fields.
		oldSocket := oldConn.socket.Load()
		tsess.localAddr = net.UDPAddrFromAddrPort(netip.MustParseAddrPort(oldSocket.c.LocalAddr().String()))
		tsess.sessKey = (*zkidentity.FixedSizeSymmetricKey)(oldSocket.sessionKey)
	}

	// Client sends the server command to join.
	tsess.assertNextJoinCommand(id, scfg.JoinCookie)

	// Server sends reply acking the join.
	var joinReply rpc.RTDTServerCmdJoinSessionReply
	tsess.serverPkt.Sequence++
	tsess.serverPkt.Source = 0
	tsess.serverPkt.Target = id
	tsess.serverWrite(joinReply.AppendFramed(&tsess.serverPkt, nil))

	// This completes the handshake and returns the session.
	res := <-resChan
	if err, ok := res.(error); ok {
		ts.t.Fatal(err)
	}
	tsess.sess = res.(*Session)

	return tsess
}

// testScaffold is used to simulate a server for testing RTDT clients.
type testScaffold struct {
	t      testing.TB
	ctx    context.Context
	cancel func()
	log    slog.Logger
	tlb    *testutils.TestLogBackend
	svr    *net.UDPConn

	inServerMsgChan chan inServerMsg

	addr       *net.UDPAddr
	serverPriv *zkidentity.FixedSizeSntrupPrivateKey
	serverPub  *zkidentity.FixedSizeSntrupPublicKey
}

// newClient creates a new test client for this server.
func (ts *testScaffold) newClient(name string, opts ...Option) *testClient {
	ccfg := defaultConfig()
	for _, opt := range opts {
		opt(&ccfg)
	}
	ccfg.log = ts.tlb.NamedSubLogger(name, nil)("RTDT")
	ccfg.handshakeRetryInterval = time.Second

	c, err := newClient(ccfg)
	if err != nil {
		ts.t.Fatal(err)
	}

	sigPriv, sigPub := zkidentity.NewFixedSizeEd25519KeyPair()

	return &testClient{
		ts:         ts,
		c:          c,
		name:       name,
		pubKey:     zkidentity.NewFixedSizeSymmetricKey(),
		sigPrivKey: sigPriv,
		sigPubKey:  sigPub,
	}
}

// assertNoServerInMsgs assets no messages are received in the server.
func (ts *testScaffold) assertNoServerInMsgs(timeout time.Duration) {
	ts.t.Helper()
	select {
	case inMsg := <-ts.inServerMsgChan:
		ts.t.Fatalf("Server has inbound message from %s", inMsg.addr.String())
	case <-time.After(timeout):
	}
}

// nextServerBytesAddr reads the next message received by the server and returns
// it and the address that sent it.
func (ts *testScaffold) nextServerBytesAddr() ([]byte, *net.UDPAddr) {
	ts.t.Helper()

	select {
	case inMsg := <-ts.inServerMsgChan:
		return inMsg.msg, inMsg.addr
	case <-time.After(10 * time.Second):
		ts.t.Fatal("Timeout waiting next inbound server message")
		return nil, nil
	}
}

// readMessages reads messages on the server sent by test clients.
func (ts *testScaffold) readMessages(ctx context.Context) {
	for {
		msg := make([]byte, rpc.RTDTMaxMessageSize)
		n, addr, err := ts.svr.ReadFromUDP(msg)

		if err != nil {
			return
		}

		ts.log.Tracef("Test server got %d inbound bytes from %s", n, addr)

		select {
		case ts.inServerMsgChan <- inServerMsg{msg: msg[:n], addr: addr}:
		case <-ctx.Done():
			return
		}
	}
}

// serverWrite writes a message (with optional encryption) from the server side.
func (ts *testScaffold) serverWrite(addr *net.UDPAddr, msg []byte, key *zkidentity.FixedSizeSymmetricKey) {
	ts.t.Helper()
	if key != nil {
		msg = encrypt(msg, key)
	}
	_, err := ts.svr.WriteToUDP(msg, addr)
	if err != nil {
		ts.t.Fatalf("Unable to serverWrite message: %v", err)
	}
}

// newTestServer creates a new test server.
func newTestServer(t testing.TB) *testScaffold {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	logEnv := os.Getenv("BR_E2E_LOG")
	showLog := logEnv == "1" || logEnv == t.Name()
	tlb := testutils.NewTestLogBackend(t, testutils.WithShowLog(showLog))

	listener, err := net.ListenUDP("udp", net.UDPAddrFromAddrPort(netip.MustParseAddrPort("127.0.0.1:0")))
	if err != nil {
		t.Fatalf("Unable to listen on UDP: %v", err)
	}
	assert.NilErr(t, listener.SetReadBuffer(1<<18))
	t.Cleanup(func() { listener.Close() })

	serverPriv, serverPub := zkidentity.NewFixedSizeSntrupKeyPair()
	addr := net.UDPAddrFromAddrPort(netip.MustParseAddrPort(listener.LocalAddr().String()))

	ts := &testScaffold{
		t:      t,
		ctx:    ctx,
		cancel: cancel,
		log:    tlb.NamedSubLogger("rtsvr", nil)("RTDT"),
		addr:   addr,
		tlb:    tlb,
		svr:    listener,

		inServerMsgChan: make(chan inServerMsg, 100),

		serverPriv: serverPriv,
		serverPub:  serverPub,
	}

	ts.log.Infof("Mock server listening on %s", listener.LocalAddr().String())

	go ts.readMessages(ctx)

	return ts
}

// encrypt encrypts data with key and a random nonce and returns a new slice
// with the nonce prepended.
//
// This should only be used in tests because it forces allocations.
func encrypt(data []byte, key *zkidentity.FixedSizeSymmetricKey) []byte {
	var nonce [24]byte
	rand.Read(nonce[:]) // Read random nonce.
	out := make([]byte, 0, 24+len(data)+16)
	out = append(out, nonce[:]...)
	return secretbox.Seal(out, data, &nonce, (*[32]byte)(key))
}

// decrypt decrypts the given input message. Assumes the nonce is prepended
// in the input byte slice.
func decrypt(t testing.TB, in []byte, key *zkidentity.FixedSizeSymmetricKey) []byte {
	t.Helper()
	var nonce [24]byte
	copy(nonce[:], in)

	msg := make([]byte, 0, len(in)-24-16)
	out, ok := secretbox.Open(msg, in[24:], &nonce, (*[32]byte)(key))
	if !ok {
		t.Fatal("Unable to decrypt message")
	}
	return out
}

// randomData generates a random slice of data.
func randomData(size int) []byte {
	b := make([]byte, size)
	rand.Read(b)
	return b
}

func TestSourceTargetPairKey(t *testing.T) {
	tests := []struct {
		source rpc.RTDTPeerID
		target rpc.RTDTPeerID
		want   sourceTargetPairKey
	}{{
		source: 0,
		target: 0,
		want:   0,
	}, {
		source: 0x70000001,
		target: 0x70000001,
		want:   0x7000000170000001,
	}, {
		source: 0x70000001,
		target: 0xffffffff,
		want:   0x70000001ffffffff,
	}, {
		source: 0xffffffff,
		target: 0x70000001,
		want:   0xffffffff70000001,
	}}

	for _, tc := range tests {
		name := fmt.Sprintf("%s/%s", tc.source, tc.target)
		t.Run(name, func(t *testing.T) {
			got := makeSourceTargetPairKey(tc.source, tc.target)
			assert.DeepEqual(t, got, tc.want)
		})
	}
}
