package rtdtserver

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	randv2 "math/rand/v2"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/RoaringBitmap/roaring/v2"
	"github.com/companyzero/bisonrelay/internal/assert"
	"github.com/companyzero/bisonrelay/internal/testutils"
	"github.com/companyzero/bisonrelay/rpc"
	"github.com/companyzero/bisonrelay/zkidentity"
	"github.com/companyzero/sntrup4591761"
	"github.com/decred/slog"
	"golang.org/x/crypto/nacl/secretbox"
)

const (
	cookieDurationMultiplier = 2
)

// peerIDToRV uses the 16 MSB from the peer id as a session id encoded in
// an RV.
func peerIDToRV(id rpc.RTDTPeerID) zkidentity.ShortID {
	return zkidentity.ShortID{28: byte(id >> 24), 29: byte(id >> 16)}
}

type testClientCfg struct {
	skipHandshake bool
}

type testClientOption func(cfg *testClientCfg)

// testClient simulates a client that can exchange messages with a test server.
//
// The methods are not safe for concurrent access and should only be called from
// the main test goroutine.
type testClient struct {
	ts            *testScaffold
	c             *net.UDPConn
	sessKey       *sntrup4591761.SharedKey
	cipherSessKey *sntrup4591761.Ciphertext

	// buf is encrypted (both to send and receive) and bufPlain is
	// plaintext data.
	buf      []byte
	bufPlain []byte

	publisherKey *zkidentity.FixedSizeSymmetricKey
	sigKey       *zkidentity.FixedSizeEd25519PrivateKey
	sigKeyPub    *zkidentity.FixedSizeEd25519PublicKey

	pkt rpc.RTDTFramedPacket
}

func (tc *testClient) Fatalf(format string, args ...interface{}) {
	tc.ts.t.Helper()
	tc.ts.t.Fatalf(format, args...)
}

// handshakeSendKey performs the first stage of the handshake: sending the
// ciphertext remotely.
func (tc *testClient) handshakeSendKey() error {
	// Send the ciphertext to the remote server.
	_, err := tc.c.Write(tc.cipherSessKey[:])
	return err
}

// handshakeRecvReply performs the second stage of the handshake: waiting for
// the server to reply the ciphertext re-encrypted with the session key.
func (tc *testClient) handshakeRecvReply() error {
	tc.c.SetReadDeadline(time.Now().Add(time.Second))
	n, err := tc.c.Read(tc.buf)
	if err != nil {
		return err
	}
	tc.c.SetReadDeadline(time.Time{})

	const replyLen = sntrup4591761.CiphertextSize + 24 + secretbox.Overhead
	if n < replyLen {
		return fmt.Errorf("handshake received wrong number of bytes "+
			"in reply (got %d, want %d)", n, replyLen)
	}

	var nonce [24]byte
	copy(nonce[:], tc.buf) // First 24 bytes of the reply is the nonce.
	plain, ok := secretbox.Open(tc.bufPlain[:0], tc.buf[24:n], &nonce, tc.sessKey)
	if !ok {
		return errors.New("failed to decrypt handshake reply from server")
	}

	if !bytes.Equal(plain, tc.cipherSessKey[:]) {
		return fmt.Errorf("Received wrong ciphertext reply")
	}

	// Handshake completed.
	return nil
}

// handshake attempts to perform a new handshake. This must be the first set of
// messages exchanged.
func (tc *testClient) handshake() error {
	if err := tc.handshakeSendKey(); err != nil {
		return err
	}
	if err := tc.handshakeRecvReply(); err != nil {
		return err
	}
	return nil
}

// prepareWrite prepares a buffer for writing the message in outPlain to the
// server.  This reuses tc.buf.
func (tc *testClient) prepareWrite(outPlain []byte) []byte {
	if tc.sessKey == nil {
		return outPlain
	}

	// Client-to-server encryption enabled.
	out := tc.buf[:24]
	rand.Read(out[:]) // Read nonce
	return secretbox.Seal(out, outPlain, aliasNonce(out), tc.sessKey)
}

// write sends the given data to the server (either encrypted or not depending
// on how the connection was setup).
func (tc *testClient) write(outPlain []byte) {
	n, err := tc.c.Write(tc.prepareWrite(outPlain))
	if err != nil {
		tc.ts.t.Fatalf("unable to write pkt (wrote %d): %v", n, err)
	}
}

// testClientSession wraps a client with its id within a specific session.
type testClientSession struct {
	tc      *testClient
	id      rpc.RTDTPeerID
	errCode errorCode
}

func (tcs *testClientSession) sendRandomData(data []byte, ts uint32) {
	tcs.tc.sendRandomData(tcs.id, data, ts)
}

func (tcs *testClientSession) assertErrCode(t *testing.T, want errorCode) {
	t.Helper()
	if tcs.errCode != want {
		t.Fatalf("unexpected error code: got %016x, want %016x",
			uint64(tcs.errCode), uint64(want))
	}
}

// deserializableCmd are rpc RTDT commands that can be deserialized.
type deserializableCmd interface {
	FromBytes(b []byte) error
}

// assertNextCmd verifies the next command received by the client is the
// expected one.
func (tc *testClient) assertNextCmd(t *testing.T, target rpc.RTDTPeerID, cmdType rpc.RTDTServerCmdType, cmd deserializableCmd) {
	t.Helper()
	inData, err := tc.readNext()
	assert.NilErr(t, err)

	var inPkt rpc.RTDTFramedPacket
	assert.NilErr(t, inPkt.FromBytes(inData))
	gotCmd, payload := inPkt.ServerCmd()
	if gotCmd != cmdType {
		t.Fatalf("Unexpected command: got %v, want %v", gotCmd, cmdType)
	}
	if inPkt.Target != target {
		t.Fatalf("Unexpected target: got %v, want %v", inPkt.Target, target)
	}
	err = cmd.FromBytes(payload)
	assert.NilErr(t, err)
}

// assertNextCmd verifies the next command received for the session is the
// expected one.
func (tcs *testClientSession) assertNextCmd(t *testing.T, cmdType rpc.RTDTServerCmdType, cmd deserializableCmd) {
	t.Helper()
	tcs.tc.assertNextCmd(t, tcs.id, cmdType, cmd)
}

// assertNextMembersList asserts the next command received by the client is a
// members list bitmap and returns the decoded bitmap.
func (tc *testClient) assertNextMembersList(t *testing.T, target rpc.RTDTPeerID) *roaring.Bitmap {
	t.Helper()
	var cmd rpc.RTDTServerCmdMembersBitmap
	tc.assertNextCmd(t, target, rpc.RTDTServerCmdTypeMembersBitmap, &cmd)
	bmp, err := cmd.ToBitmap()
	assert.NilErr(t, err)
	return bmp
}

// assertNextMembersList asserts the next command received by the client for
// this session is a list of members.
func (tcs *testClientSession) assertNextMembersList(t *testing.T) *roaring.Bitmap {
	t.Helper()
	return tcs.tc.assertNextMembersList(t, tcs.id)
}

// frameableCmd are RPC commands that can be sent framed.
type frameableCmd interface {
	AppendFramed(pkt *rpc.RTDTFramedPacket, b []byte) []byte
}

// sendCmd sends a command to the server.
func (tc *testClient) sendCmd(t *testing.T, source rpc.RTDTPeerID, cmd frameableCmd) {
	t.Helper()
	tc.pkt.Sequence++
	tc.pkt.Target = 0
	tc.pkt.Source = source
	tc.write(cmd.AppendFramed(&tc.pkt, nil))
}

// sendCmd sends a command to the server.
func (tcs *testClientSession) sendCmd(t *testing.T, cmd frameableCmd) {
	t.Helper()
	tcs.tc.sendCmd(t, tcs.id, cmd)
}

// joinSessionWithCookie attempts to join the session using the specified
// encoded session cookie.
func (tc *testClient) joinSessionWithCookie(id rpc.RTDTPeerID, joinCookie []byte) *testClientSession {
	tc.ts.t.Helper()
	joinCmd := rpc.RTDTServerCmdJoinSession{
		JoinCookie: joinCookie,
	}

	tc.pkt.Target = 0
	tc.pkt.Source = id
	tc.pkt.Sequence++
	outPlain := joinCmd.AppendFramed(&tc.pkt, tc.bufPlain[:0])
	tc.write(outPlain)

	// Receive, decode and process reply.
	replyBytes, err := tc.readNext()
	if err != nil {
		tc.ts.t.Fatalf("Unable to read server reply: %v", err)
	}
	var encPacket rpc.RTDTFramedPacket
	if err := encPacket.FromBytes(replyBytes); err != nil {
		tc.ts.t.Fatalf("Unable to decode packet: %v", err)
	}
	if encPacket.Target != id {
		tc.ts.t.Fatalf("Received packet for wrong target: got %d, want %d",
			encPacket.Target, id)
	}
	serverCmd, payload := encPacket.ServerCmd()
	if serverCmd != rpc.RTDTServerCmdTypeJoinSessionReply {
		tc.ts.t.Fatalf("Received unexpected server cmd: got %d, want %d",
			serverCmd, rpc.RTDTServerCmdTypeJoinSessionReply)
	}
	var reply rpc.RTDTServerCmdJoinSessionReply
	if err := reply.FromBytes(payload); err != nil {
		tc.ts.t.Fatalf("Unable to decode reply: %v", err)
	}

	return &testClientSession{tc: tc, id: id, errCode: errorCode(reply.ErrCode)}
}

// joinSession attempts to join the session. This uses either a nil or a fully
// valid join cookie, depending on the test server config.
func (tc *testClient) joinSession(id rpc.RTDTPeerID) *testClientSession {
	tc.ts.t.Helper()
	var joinCookie []byte
	if tc.ts.cookieKey != nil {
		jc := tc.ts.validJoinCookie(id)
		joinCookie = jc.Encrypt(nil, tc.ts.cookieKey)
	}
	return tc.joinSessionWithCookie(id, joinCookie)
}

// leaveSession asserts the client leaves the session with the given id.
func (tc *testClient) leaveSession(id rpc.RTDTPeerID) {
	tc.ts.t.Helper()
	t := tc.ts.t

	leaveCmd := rpc.RTDTServerCmdLeaveSession{}
	tc.pkt.Sequence++
	tc.pkt.Target = 0
	tc.pkt.Source = id
	tc.write(leaveCmd.AppendFramed(&tc.pkt, nil))
	inData, err := tc.readNext()
	assert.NilErr(t, err)
	var inPkt rpc.RTDTFramedPacket
	assert.NilErr(t, inPkt.FromBytes(inData))
	assert.DeepEqual(t, inPkt.Target, id)
	gotCmd, _ := inPkt.ServerCmd()
	assert.DeepEqual(t, gotCmd, rpc.RTDTServerCmdTypeLeaveSessionReply)
}

// leaveSession leaves the session.
func (tcs *testClientSession) leaveSession() {
	tcs.tc.ts.t.Helper()
	tcs.tc.leaveSession(tcs.id)
}

// prepareDataPkt prepares an outbound data packet to be sent to the server,
// encrypted with the client's publisher and signature keys.  The output buffer
// reuses tc.bufPlain, so this must be copied if it will be stored.
func (tc *testClient) prepareDataPkt(id rpc.RTDTPeerID, stream rpc.RTDTStreamType, data []byte, ts uint32) []byte {
	pkt := rpc.RTDTDataPacket{
		Stream:    stream,
		Timestamp: ts,
		Data:      data,
	}
	tc.pkt.Target = id
	tc.pkt.Source = id
	tc.pkt.Sequence++
	return pkt.AppendEncrypted(&tc.pkt, tc.bufPlain[:0], tc.publisherKey, tc.sigKey)
}

// sendRandomData sends data in the random stream using the source id. The
// source peer id may or may not be for a session this test client has bound
// itself to.
func (tc *testClient) sendRandomData(id rpc.RTDTPeerID, data []byte, ts uint32) {
	outPlain := tc.prepareDataPkt(id, rpc.RTDTStreamRandom, data, ts)
	tc.write(outPlain)
}

func (tc *testClient) assertNextReadTimesOut(timeout time.Duration) {
	tc.ts.t.Helper()
	tc.c.SetReadDeadline(time.Now().Add(timeout))
	n, err := tc.c.Read(tc.buf)
	tc.c.SetReadDeadline(time.Time{})

	if !errors.Is(err, os.ErrDeadlineExceeded) {
		if n > 0 {
			tc.ts.t.Logf("Read %d bytes: %q", n, tc.buf[:n])
		}
		tc.ts.t.Fatalf("Unexpected error: got %v, want %v", err, os.ErrDeadlineExceeded)
	}
}

// readNext reads the next message sent by the server.
func (tc *testClient) readNext() ([]byte, error) {
	tc.c.SetReadDeadline(time.Now().Add(time.Second))
	n, err := tc.c.Read(tc.buf)
	tc.c.SetReadDeadline(time.Time{})
	if err != nil {
		return nil, fmt.Errorf("unable to read from client udp (read %d): %w", n, err)
	}

	if tc.sessKey != nil {
		if n < 24+16 {
			// Not enough bytes to decrypt.
			return nil, fmt.Errorf("not enought bytes to decrypt (read %d)", n)
		}

		in, ok := secretbox.Open(tc.bufPlain[:0], tc.buf[24:n], aliasNonce(tc.buf), tc.sessKey)
		if !ok {
			return nil, fmt.Errorf("decryption failed")
		}

		return in, nil
	}

	return tc.buf[:n], nil
}

// readNextPlainPacket reads the next message sent by the server and decodes it
// as a plain text packet.
func (tc *testClient) readNextPlainPacket(from *testClient) (*rpc.RTDTDataPacket, error) {
	in, err := tc.readNext()
	if err != nil {
		return nil, err
	}

	var encPacket rpc.RTDTFramedPacket
	var plainPacket rpc.RTDTDataPacket

	if err := encPacket.FromBytes(in); err != nil {
		return nil, fmt.Errorf("unable to decode packet: %v", err)
	}

	var encKey *zkidentity.FixedSizeSymmetricKey
	var sigKey *zkidentity.FixedSizeEd25519PublicKey

	if from != nil {
		encKey = from.publisherKey
		sigKey = from.sigKeyPub
	}

	if err := plainPacket.Decrypt(encPacket, encKey, sigKey); err != nil {
		return nil, fmt.Errorf("E2E decryption error: %v", err)
	}

	return &plainPacket, nil
}

// assertNextRandomData asserts that the next message from server is data sent
// on the random stream by some other test client and it equals the given
// expected data.
func (tc *testClient) assertNextRandomData(from *testClient, data []byte) {
	tc.ts.t.Helper()
	pkt, err := tc.readNextPlainPacket(from)
	if err != nil {
		tc.Fatalf("readNextPlainPacket errored: %v", err)
	}

	if pkt.Stream != rpc.RTDTStreamRandom {
		tc.Fatalf("unexpected stream: got %d, want %d", pkt.Stream, rpc.RTDTStreamRandom)
	}
	assert.DeepEqual(tc.ts.t, pkt.Data, data)
}

type testScaffoldCfg struct {
	addr                      string
	cookieKey                 *zkidentity.FixedSizeSymmetricKey
	dropPayLoopInterval       time.Duration
	maxPingInterval           time.Duration
	minPingInterval           time.Duration
	timeoutLoopTickerInterval time.Duration
	decodeCookieKeys          []*zkidentity.FixedSizeSymmetricKey
	diabledLogger             bool
	serverMinListInterval     time.Duration
	serverSessListInterval    time.Duration
	disableForceListing       bool
}

type testScaffoldOption func(tc *testScaffoldCfg)

// withTestCookieKey runs the server with a known, test cookie key.
func withTestCookieKey() testScaffoldOption {
	return func(tc *testScaffoldCfg) {
		// Generate a known, fixed, cookie key.
		tc.cookieKey = new(zkidentity.FixedSizeSymmetricKey)
		rng := randv2.NewChaCha8([32]byte{0: 0x01})
		rng.Read(tc.cookieKey[:])
	}
}

// withDropPaymentLoopInterval runs the server with the specified drop payment
// loop interval.
func withDropPaymentLoopInterval(d time.Duration) testScaffoldOption {
	return func(tc *testScaffoldCfg) {
		tc.dropPayLoopInterval = d
	}
}

// withPingInterval runs the server with the specified ping interval parameters.
func withPingInterval(maxInterval, minInterval time.Duration) testScaffoldOption {
	return func(tc *testScaffoldCfg) {
		tc.maxPingInterval = maxInterval
		tc.minPingInterval = minInterval
	}
}

// withTimeoutLoopTickerInterval runs the server with the specified timeout loop
// ticker interval.
func withTimeoutLoopTickerInterval(d time.Duration) testScaffoldOption {
	return func(tc *testScaffoldCfg) {
		tc.timeoutLoopTickerInterval = d
	}
}

// withDecodeCookieKeys runs the server with additional decode cookie keys.
func withDecodeCookieKeys(keys []*zkidentity.FixedSizeSymmetricKey) testScaffoldOption {
	return func(tc *testScaffoldCfg) {
		tc.decodeCookieKeys = keys
	}

}

// withDisabledLogger uses a disabled logger (instead of a regular test logger).
// Useful for benchmarks.
func withDisabledLogger() testScaffoldOption {
	return func(tc *testScaffoldCfg) {
		tc.diabledLogger = true
	}
}

// withServerMembersListInterval runs the server with the specified parameters
// for sending member listings.
func withServerMembersListInterval(sessListInterval, minSessListingInterval time.Duration) testScaffoldOption {
	return func(tc *testScaffoldCfg) {
		tc.serverSessListInterval = sessListInterval
		tc.serverMinListInterval = minSessListingInterval
	}
}

// withEnabledForceSendMembersList re-enables sending of member listings after
// joins take place.
func withEnabledForceSendMembersList() testScaffoldOption {
	return func(tc *testScaffoldCfg) {
		tc.disableForceListing = false
	}
}

// testScaffold is used as a scaffold to run tests on a RTDT server.
type testScaffold struct {
	t         testing.TB
	ctx       context.Context
	runDone   chan struct{}
	runErr    error
	cancel    func()
	s         *Server
	addr      string
	cookieKey *zkidentity.FixedSizeSymmetricKey
	serverPub *zkidentity.FixedSizeSntrupPublicKey
}

func (ts *testScaffold) assertNoAllowanceData(want uint64) {
	ts.t.Helper()
	got := ts.s.stats.noAllowanceBytesAtomic.Load()
	if got != want {
		ts.t.Fatalf("Unexpected number of non allowance bytes: got %d, want %d",
			got, want)
	}
}

// validJoinCookie returns a valid join cookie that can be used by a peer with
// the given ID to join a session.
func (ts *testScaffold) validJoinCookie(id rpc.RTDTPeerID) rpc.RTDTJoinCookie {
	endTimestamp := time.Now().Add(ts.s.cfg.dropPaymentLoopInterval * cookieDurationMultiplier).Unix()
	return rpc.RTDTJoinCookie{
		OwnerSecret:      peerIDToRV(id),
		PeerID:           id,
		EndTimestamp:     endTimestamp,
		Size:             1 << 16,
		PublishAllowance: 10000000000, // 10GB
		PaymentTag:       randv2.Uint64(),
	}
}

// encryptJoinCookie encrypts the given join cookie if there is a cookie key.
func (ts *testScaffold) encryptJoinCookie(jc *rpc.RTDTJoinCookie) []byte {
	if ts.cookieKey == nil {
		return nil
	}

	return jc.Encrypt(nil, ts.cookieKey)
}

// newClient creates a new client that can communicate with the server. This
// client will automatically perform handshake by default.
func (ts *testScaffold) newClient(opts ...testClientOption) *testClient {
	ts.t.Helper()
	cfg := &testClientCfg{}
	for _, opt := range opts {
		opt(cfg)
	}

	addr := net.UDPAddrFromAddrPort(netip.MustParseAddrPort(ts.addr))
	udpConn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		return nil
	}

	tc := &testClient{
		c:        udpConn,
		ts:       ts,
		buf:      make([]byte, 1<<16),
		bufPlain: make([]byte, 1<<16),
	}

	if cfg.skipHandshake || ts.serverPub == nil {
		return tc
	}

	// Perform handshake automatically.
	tc.cipherSessKey, tc.sessKey = ts.serverPub.Encapsulate()
	if err := tc.handshake(); err != nil {
		ts.t.Fatalf("Unable to complete handshake: %v", err)
	}

	return tc
}

// newTestServer creates a new testScaffold for testing the server.
func newTestServer(t testing.TB, opts ...testScaffoldOption) *testScaffold {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	logEnv := os.Getenv("BR_E2E_LOG")
	showLog := logEnv == "1" || logEnv == t.Name()
	tlb := testutils.NewTestLogBackend(t, testutils.WithShowLog(showLog))

	rootDir, err := os.MkdirTemp("", "br-rtdttest-*")
	assert.NilErr(t, err)
	t.Cleanup(func() {
		if t.Failed() {
			t.Logf("Root test dir: %s", rootDir)
		} else {
			err := os.RemoveAll(rootDir)
			if err != nil {
				t.Logf("Error removing test dir: %v", err)
			}
		}
	})
	logFile, err := os.Create(filepath.Join(rootDir, "rtdtserver.log"))
	assert.NilErr(t, err)
	t.Cleanup(func() { logFile.Close() })

	cfg := &testScaffoldCfg{
		addr:                   "127.0.0.1:0",
		dropPayLoopInterval:    time.Hour,
		maxPingInterval:        rpc.RTDTMaxPingInterval,
		minPingInterval:        rpc.RTDTDefaultMinPingInterval,
		serverMinListInterval:  time.Minute,
		serverSessListInterval: time.Minute,
		disableForceListing:    true,
	}
	for _, opt := range opts {
		opt(cfg)
	}

	ts := &testScaffold{
		t:       t,
		ctx:     ctx,
		cancel:  cancel,
		runDone: make(chan struct{}),
	}
	listener, err := net.ListenUDP("udp", net.UDPAddrFromAddrPort(netip.MustParseAddrPort(cfg.addr)))
	if err != nil {
		t.Fatalf("Unable to listen on UDP: %v", err)
	}
	ts.addr = listener.LocalAddr().String()
	ts.cookieKey = cfg.cookieKey

	serverPriv, serverPub := zkidentity.NewFixedSizeSntrupKeyPair()
	ts.serverPub = serverPub
	logger := tlb.NamedSubLogger("rtsvr", logFile)("RTDT")
	if cfg.diabledLogger {
		logger = slog.Disabled
	}
	svrCfg := fillConfig(
		WithLogger(logger),
		WithPrivateKey(serverPriv),
		WithListeners(listener),
		WithCookieKey(ts.cookieKey, cfg.decodeCookieKeys),
	)
	svrCfg.dropPaymentLoopInterval = cfg.dropPayLoopInterval
	svrCfg.maxPingInterval = cfg.maxPingInterval
	svrCfg.minPingInterval = cfg.minPingInterval
	svrCfg.replyErrorCodes = true
	svrCfg.logReadLoopErrors = true
	svrCfg.ignoreKernelStats = true
	svrCfg.disableForceListing = cfg.disableForceListing
	if cfg.timeoutLoopTickerInterval > 0 {
		svrCfg.timeoutLoopTickerInterval = cfg.timeoutLoopTickerInterval
	}
	svrCfg.minSessListingInterval = cfg.serverMinListInterval
	svrCfg.sessListingInterval = cfg.serverSessListInterval
	ts.s, err = newServer(&svrCfg)
	if err != nil {
		t.Fatalf("Unable to create RTDTServer: %v", err)
	}
	go func() {
		ts.runErr = ts.s.Run(ts.ctx)
		close(ts.runDone)
	}()
	time.Sleep(10 * time.Millisecond) // Give Run() a chance to run.

	// Ensure at end of test the server has not failed yet.
	t.Cleanup(func() {
		if t.Failed() {
			return
		}

		select {
		case <-ts.runDone:
			if ts.runErr != nil {
				t.Fatalf("Server Run() failed with error %v", ts.runErr)
			}
		default:
		}
	})

	return ts
}

// assertCanExchangeData asserts that every client within the passed list can
// send and receive messages from every other client.
func assertCanExchangeData(t testing.TB, clients ...*testClientSession) {
	t.Helper()
	data := make([]byte, 32)
	rng := randv2.NewChaCha8(zkidentity.RandomShortID())
	for i := range clients {
		rng.Read(data)
		clients[i].sendRandomData(data, 0)
		for j := range clients {
			if i == j {
				continue
			}

			clients[j].tc.assertNextRandomData(clients[i].tc, data)
		}
	}
}

// assertNoData asserts that none of the specified clients reads any data from
// the server.
func assertNoData(t testing.TB, clients ...*testClient) {
	t.Helper()
	for _, tc := range clients {
		tc.assertNextReadTimesOut(time.Second)
	}
}

// assertCannotExchangeData asserts that no clients can exchange data with
// one another.
func assertCannotExchangeData(t testing.TB, clients ...*testClientSession) {
	t.Helper()
	data := make([]byte, 32)
	rng := randv2.NewChaCha8(zkidentity.RandomShortID())
	for i := range clients {
		rng.Read(data)
		clients[i].sendRandomData(data, 0)
		for j := range clients {
			if i == j {
				continue
			}

			clients[j].tc.assertNextReadTimesOut(time.Second)
		}
	}

}

// randomData generates a random slice of data.
func randomData(size int) []byte {
	b := make([]byte, size)
	rand.Read(b)
	return b
}
