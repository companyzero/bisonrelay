package lowlevel

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"math/rand"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/companyzero/bisonrelay/internal/testutils"
	"github.com/companyzero/bisonrelay/rpc"
	"github.com/companyzero/bisonrelay/session"
	"github.com/companyzero/bisonrelay/zkidentity"
)

func testRand(t testing.TB) *rand.Rand {
	seed := time.Now().UnixNano()
	rnd := rand.New(rand.NewSource(seed))
	t.Cleanup(func() {
		if t.Failed() {
			t.Logf("Seed: %d", seed)
		}
	})

	return rnd
}

func jsonAsHex(i interface{}) string {
	b, err := json.Marshal(i)
	if err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

func mustFromHex(h string) []byte {
	b, err := hex.DecodeString(h)
	if err != nil {
		panic(err)
	}
	return b
}

func mustPayloadForCmd(cmd string) interface{} {
	p, err := payloadForCmd(cmd)
	if err != nil {
		panic(err)
	}
	return p
}

func testTimeoutCtx(t testing.TB, timeout time.Duration) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	t.Cleanup(func() { cancel() })
	return ctx
}

func rvidFromStr(s string) RVID {
	var rv RVID
	copy(rv[:], []byte(s))
	return rv
}

func strFromRVID(rv RVID) string {
	return strings.TrimSpace(string(rv[:]))
}

func mockTLSConnState(certByte uint8) *tls.ConnectionState {
	return &tls.ConnectionState{
		PeerCertificates: []*x509.Certificate{{
			Raw: []byte{certByte},
		}},
	}
}

var (
	mockServerPRNG  = rand.New(rand.NewSource(0x1020304050))
	mockServerID, _ = zkidentity.NewWithRNG("mock server", "ms", mockServerPRNG)
)

func mockServerKX(conn clientintf.Conn) *session.KX {
	return &session.KX{
		Conn:           conn,
		MaxMessageSize: 1887437,
		OurPublicKey:   &mockServerID.Public.Key,
		OurPrivateKey:  &mockServerID.PrivateKey,
	}
}

// offlineConn is a connection that is always offline. All methods from it
// error.
type offlineConn struct{}

// static typecheck to ensure offlineConn is a valid Conn implementation.
var _ clientintf.Conn = offlineConn{}

var errOfflineConn = errors.New("offline conn is always offline")

func (oc offlineConn) Read(p []byte) (n int, err error) {
	return 0, errOfflineConn
}

func (oc offlineConn) Write(p []byte) (n int, err error) {
	return 0, errOfflineConn
}

func (oc offlineConn) Close() error {
	return errOfflineConn
}

func (oc offlineConn) RemoteAddr() net.Addr {
	return oc
}

func (oc offlineConn) Network() string {
	return "offlineConnNet"
}

func (oc offlineConn) String() string {
	return "offlineConn"
}

// spidConn is a connection that writes the specified server public id to the
// conn when needed.
type spidConn struct {
	b   []byte
	pid zkidentity.PublicIdentity
}

func newRandSpidConn() *spidConn {
	full, err := zkidentity.New("", "")
	if err != nil {
		panic(err)
	}
	spid := &spidConn{pid: full.Public}
	spid.reset()
	return spid
}

func newSpidConn() *spidConn {
	var pid zkidentity.PublicIdentity
	var buff bytes.Buffer
	err := json.NewEncoder(&buff).Encode(pid)
	if err != nil {
		panic(err)
	}

	return &spidConn{b: buff.Bytes(), pid: pid}
}

// static typecheck to ensure offlineConn is a valid Conn implementation.
var _ clientintf.Conn = offlineConn{}

func (sc *spidConn) reset() {
	var buff bytes.Buffer
	err := json.NewEncoder(&buff).Encode(sc.pid)
	if err != nil {
		panic(err)
	}

	sc.b = buff.Bytes()
}

func (sc *spidConn) Read(p []byte) (n int, err error) {
	n = copy(p, sc.b)
	sc.b = sc.b[n:]
	return n, nil
}

func (sc *spidConn) Write(p []byte) (n int, err error) {
	return len(p), nil
}

func (sc *spidConn) Close() error {
	return nil
}

func (sc *spidConn) RemoteAddr() net.Addr {
	return offlineConn{}
}

// pipedConn is a struct that implements Conn by reading from the reader end of
// a pipe and writing to the writer end of a pipe (usually not the same one).
type pipedConn struct {
	reader *io.PipeReader
	writer *io.PipeWriter
}

// static typecheck to ensure pipedConn is a valid Conn implementation.
var _ clientintf.Conn = (*pipedConn)(nil)

func (pc *pipedConn) Read(p []byte) (n int, err error) {
	return pc.reader.Read(p)
}

func (pc *pipedConn) Write(p []byte) (n int, err error) {
	return pc.writer.Write(p)
}

func (pc pipedConn) Close() error {
	if err := pc.reader.Close(); err != nil {
		pc.writer.Close()
		return err
	}
	return pc.writer.Close()
}

func (pc pipedConn) RemoteAddr() net.Addr {
	return nil
}

// clientServerPipedConn returns a pair of interconnected piped conns: the
// reading end of one will be the writing end of the other (and vice-versa).
// This simulates a bidirectional client-server network connection.
func clientServerPipedConn() (*pipedConn, *pipedConn) {
	r1, w1 := io.Pipe()
	r2, w2 := io.Pipe()
	return &pipedConn{reader: r1, writer: w2}, &pipedConn{reader: r2, writer: w1}
}

// mockKX allows mocking a KX object in a concurrent safe manner. Every call to
// Read() or Write() expects a corresponding call to one
// {read,write}{msg,err}chan.
type mockKX struct {
	readMsgChan  chan []byte
	readErrChan  chan error
	writeMsgChan chan []byte
	writeErrChan chan error
}

func newMockKX() *mockKX {
	return &mockKX{
		readMsgChan:  make(chan []byte),
		readErrChan:  make(chan error),
		writeMsgChan: make(chan []byte),
		writeErrChan: make(chan error),
	}
}

// Read fulfills the msgReadWriter interface.
func (mkx *mockKX) Read() ([]byte, error) {
	select {
	case m := <-mkx.readMsgChan:
		return m, nil
	case err := <-mkx.readErrChan:
		return nil, err
	}
}

// pushReadMsg pushes a message that will be read by the next call to Read().
// This blocks until the message is read.
func (mkx *mockKX) pushReadMsg(t testing.TB, msg *rpc.Message, payload interface{}) {
	t.Helper()
	var bb bytes.Buffer
	enc := json.NewEncoder(&bb)
	err := enc.Encode(msg)
	if err != nil {
		t.Fatalf("unable to marshal: %v", err)
	}
	err = enc.Encode(payload)
	if err != nil {
		t.Fatalf("unable to marshal payload: %v", err)
	}
	mkx.readMsgChan <- bb.Bytes()
}

// pushReadErr pushes an error that will be returned by the next call to Read().
func (mkx *mockKX) pushReadErr(t testing.TB, err error) {
	t.Helper()
	select {
	case mkx.readErrChan <- err:
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}
}

// Write fulfills the msgReadWriter interface.
func (mkx *mockKX) Write(b []byte) error {
	select {
	case mkx.writeMsgChan <- b:
		return nil
	case err := <-mkx.writeErrChan:
		return err
	}
}

// popWriteErr makes the next call to Write() fail with the specified error.
func (mkx *mockKX) popWriteErr(t testing.TB, err error) {
	t.Helper()
	select {
	case mkx.writeErrChan <- err:
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}
}

// popWrittenMsg decodes a message written by a Write() call.
func (mkx *mockKX) popWrittenMsg(t testing.TB) (rpc.Message, interface{}) {
	t.Helper()

	select {
	case b := <-mkx.writeMsgChan:
		var msg rpc.Message
		br := bytes.NewReader(b)
		dec := json.NewDecoder(br)
		err := dec.Decode(&msg)
		if err != nil {
			t.Fatalf("unable to unmarshal: %v", err)
		}

		payload, err := decodeRPCPayload(&msg, dec)
		if err != nil {
			t.Fatalf("unable to decode: %v", err)
		}

		return msg, payload
	case <-time.After(time.Second):
		t.Fatalf("timeout")
	}

	return rpc.Message{}, nil
}

var errMockServerSession = errors.New("mock server session errored")

type mockServerSession struct {
	sendErrChan chan wireMsg
	rpcChan     chan wireMsg
	policy      clientintf.ServerPolicy
	mpc         *testutils.MockPayClient
}

func newMockServerSession() *mockServerSession {
	return &mockServerSession{
		sendErrChan: make(chan wireMsg),
		rpcChan:     make(chan wireMsg),
		policy: clientintf.ServerPolicy{
			MaxPushInvoices:      1,
			PushPaymentLifetime:  time.Second,
			MaxMsgSizeVersion:    rpc.MaxMsgSizeV0,
			MaxMsgSize:           rpc.MaxMsgSizeForVersion(rpc.MaxMsgSizeV0),
			ExpirationDays:       30,
			PushPayRateMAtoms:    1000,
			PushPayRateBytes:     1,
			PushPayRateMinMAtoms: 1000,
			SubPayRate:           1000,
		},
		mpc: &testutils.MockPayClient{},
	}
}

func (m *mockServerSession) SendPRPC(msg rpc.Message, payload interface{}, reply chan<- interface{}) error {
	select {
	case m.rpcChan <- wireMsg{msg: msg, payload: payload, replyChan: reply}:
		return nil
	case m.sendErrChan <- wireMsg{msg: msg, payload: payload, replyChan: reply}:
		return errMockServerSession
	}
}

func (m *mockServerSession) failNextPRPC(t testing.TB) {
	t.Helper()
	select {
	case <-m.sendErrChan:
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}
}

func (m *mockServerSession) replyNextPRPC(t testing.TB, reply interface{}) interface{} {
	t.Helper()
	select {
	case wm := <-m.rpcChan:
		select {
		case wm.replyChan <- reply:
		case <-time.After(time.Second):
			t.Fatal("timeout on reply")
		}
		return wm.payload
	case <-time.After(time.Second):
		t.Fatal("timeout on receive")
	}
	return nil
}

func (m *mockServerSession) assertNoMessages(t testing.TB, timeout time.Duration) {
	t.Helper()
	msg := rpc.Message{}
	payload := rpc.Pong{}
	reply := make(chan interface{}, 1)
	select {
	case m.rpcChan <- wireMsg{msg: msg, payload: payload, replyChan: reply}:
		t.Fatalf("write in RPCChan")
	case m.sendErrChan <- wireMsg{msg: msg, payload: payload, replyChan: reply}:
		t.Fatalf("write in sendErrChan")
	case <-time.After(timeout):
	}
}

func (m *mockServerSession) RequestClose(err error)              {}
func (m *mockServerSession) PayClient() clientintf.PaymentClient { return m.mpc }
func (m *mockServerSession) PaymentRates() (uint64, uint64)      { return 0, 0 }
func (m *mockServerSession) ExpirationDays() int                 { return 7 }
func (m *mockServerSession) Context() context.Context            { return context.Background() }
func (m *mockServerSession) Policy() clientintf.ServerPolicy     { return m.policy }

type mockRM string

func (rm mockRM) Priority() uint {
	return 0
}

func (rm mockRM) EncryptedLen() uint32 {
	return uint32(len(rm))
}

func (rm mockRM) EncryptedMsg() (RVID, []byte, error) {
	return rvidFromStr("rdzv_" + string(rm)), []byte(rm), nil
}

func (rm mockRM) PaidForRM(amount, fees int64) {}

var errMockRM = errors.New("mock RM error")

type mockFailedRM struct{}

func (rm mockFailedRM) Priority() uint {
	return 0
}

func (rm mockFailedRM) EncryptedLen() uint32 {
	return uint32(1)
}

func (rm mockFailedRM) EncryptedMsg() (RVID, []byte, error) {
	return rvidFromStr("mock_failed_rm_rdzv"), nil, errMockRM
}

func (rm mockFailedRM) PaidForRM(amount, fees int64) {}

type mockRvMgrDB struct {
	alwaysPaid bool
	paid       map[RVID]struct{}
}

func (db *mockRvMgrDB) UnpaidRVs(rvs []RVID, expirationDays int) ([]RVID, error) {
	if db.alwaysPaid {
		return nil, nil
	}

	if db.paid == nil {
		return rvs, nil
	}

	res := make([]RVID, 0, len(rvs))
	for _, rv := range rvs {
		if _, ok := db.paid[rv]; !ok {
			res = append(res, rv)
		}
	}
	return res, nil
}

func (db *mockRvMgrDB) SavePaidRVs(rvs []RVID) error {
	if db.paid == nil {
		db.paid = make(map[RVID]struct{})
	}

	for _, rv := range rvs {
		db.paid[rv] = struct{}{}
	}

	return nil
}

func (db *mockRvMgrDB) MarkRVUnpaid(rv RVID) error {
	delete(db.paid, rv)
	return nil
}

type mockRMQDBEntry struct {
	invoice string
	date    time.Time
}

type mockRMQDB struct {
	mtx   sync.Mutex
	store map[RVID]mockRMQDBEntry
}

func newMockRMQDB() *mockRMQDB {
	return &mockRMQDB{
		store: make(map[RVID]mockRMQDBEntry),
	}
}

func (m *mockRMQDB) RVHasPaymentAttempt(rv RVID) (string, time.Time, error) {
	m.mtx.Lock()
	defer m.mtx.Unlock()
	if e, ok := m.store[rv]; ok {
		return e.invoice, e.date, nil
	}
	return "", time.Time{}, nil
}

func (m *mockRMQDB) StoreRVPaymentAttempt(rv RVID, invoice string, date time.Time) error {
	m.mtx.Lock()
	defer m.mtx.Unlock()
	m.store[rv] = mockRMQDBEntry{invoice: invoice, date: date}
	return nil
}

func (m *mockRMQDB) DeleteRVPaymentAttempt(rv RVID) error {
	m.mtx.Lock()
	defer m.mtx.Unlock()
	delete(m.store, rv)
	return nil
}
