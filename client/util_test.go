package client

import (
	"bytes"
	"context"
	crand "crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/companyzero/bisonrelay/client/clientdb"
	"github.com/companyzero/bisonrelay/client/internal/lowlevel"
	"github.com/companyzero/bisonrelay/ratchet"
	"github.com/companyzero/bisonrelay/rpc"
	"github.com/companyzero/bisonrelay/zkidentity"
	"github.com/decred/slog"
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

func testID(t testing.TB, rnd io.Reader, name string) *zkidentity.FullIdentity {
	t.Helper()
	id, err := zkidentity.NewWithRNG(name, name, rnd)
	if err != nil {
		t.Fatalf("unable to create test id %s: %v", name, err)
	}
	return id
}

func fixedIDIniter(id *zkidentity.FullIdentity) func(context.Context) (*zkidentity.FullIdentity, error) {
	return func(context.Context) (*zkidentity.FullIdentity, error) {
		c := new(zkidentity.FullIdentity)
		*c = *id
		return c, nil
	}
}

//nolint:golint,unused
func testRandomFile(t testing.TB) string {
	t.Helper()
	f, err := os.CreateTemp("", "test-random-file")
	if err != nil {
		t.Fatal(err)
	}
	var b [32]byte
	_, err = io.ReadFull(crand.Reader, b[:])
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.Write(b[:])
	if err != nil {
		t.Fatal(err)
	}
	err = f.Close()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Remove(f.Name()) })
	return f.Name()
}

func randomHex(rnd io.Reader, len int) string {
	b := make([]byte, len)
	_, err := rnd.Read(b)
	if err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

//nolint:golint,unused
func orFatal(t testing.TB, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func testDB(t testing.TB, id *zkidentity.FullIdentity, log slog.Logger) *clientdb.DB {
	//t.Helper()
	name := ""
	if id != nil && id.Public.Nick != "" {
		name = "-" + id.Public.Nick
	}
	tempDir, err := os.MkdirTemp("", "cr-client"+name+"-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if t.Failed() {
			t.Logf("DB Dir: %s", tempDir)
		} else {
			os.RemoveAll(tempDir)
		}
	})
	cfg := clientdb.Config{
		Root:          tempDir,
		DownloadsRoot: filepath.Join(tempDir, "downloads"),
		Logger:        log,
		ChunkSize:     8,
	}
	db, err := clientdb.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func runTestDB(t testing.TB, db *clientdb.DB) {
	ctx, cancel := context.WithCancel(context.Background())
	runErr := make(chan error)
	go func() { runErr <- db.Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-runErr:
			if !errors.Is(err, context.Canceled) {
				t.Logf("DB run error: %v", err)
			}
		case <-time.After(time.Second):
			t.Logf("timeout waiting for DB to finish running")
		}
	})
}

func pairedRatchet(t testing.TB, rnd io.Reader, ida, idb *zkidentity.FullIdentity) (*ratchet.Ratchet, *ratchet.Ratchet) {
	a := ratchet.New(rnd)
	a.MyPrivateKey = &ida.PrivateKey
	a.TheirPublicKey = &idb.Public.Key

	b := ratchet.New(rnd)
	b.MyPrivateKey = &idb.PrivateKey
	b.TheirPublicKey = &ida.Public.Key

	kxA, kxB := new(ratchet.KeyExchange), new(ratchet.KeyExchange)
	if err := a.FillKeyExchange(kxA); err != nil {
		t.Fatal(err)
	}
	if err := b.FillKeyExchange(kxB); err != nil {
		t.Fatal(err)
	}
	if err := a.CompleteKeyExchange(kxB, false); err != nil {
		t.Fatal(err)
	}
	if err := b.CompleteKeyExchange(kxA, true); err != nil {
		t.Fatal(err)
	}

	return a, b
}

// certConfirmerUnsafeAlwaysAccept is a function that fulfills the
// CertConfirmer prototype by always accepting the cert.
//
// It is NOT safe and only meant to be used in tests.
//
//nolint:golint,unused
func certConfirmerUnsafeAlwaysAccept(context.Context, *tls.ConnectionState,
	*zkidentity.PublicIdentity) error {
	return nil
}

// assertRatchetsSynced asserts the ratchets can communicate with one another
// (i.e. they are synced).
func assertRatchetsSynced(t testing.TB, a, b *ratchet.Ratchet) {
	t.Helper()

	msg := []byte(fmt.Sprintf("test message %d", time.Now().UnixNano()))
	encrypted, err := a.Encrypt(nil, msg)
	if err != nil {
		t.Fatal(err)
	}
	result, err := b.Decrypt(encrypted)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(msg, result) {
		t.Fatalf("a -> b result doesn't match: %x vs %x", msg, result)
	}

	encrypted, err = b.Encrypt(nil, msg)
	if err != nil {
		t.Fatal(err)
	}
	result, err = a.Decrypt(encrypted)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(msg, result) {
		t.Fatalf("b -> a result doesn't match: %x vs %x", msg, result)
	}
}

// mockRMServer is a mock server that perfectly relays RMs back and forth
// between multiple users.
type mockRMServer struct {
	sync.Mutex
	t      testing.TB
	done   chan struct{}
	single map[lowlevel.RVID]lowlevel.RVHandler
	subs   map[lowlevel.RVID]lowlevel.RVHandler
	rms    map[lowlevel.RVID]lowlevel.RVBlob
}

func newMockRMServer(t testing.TB) *mockRMServer {
	ms := &mockRMServer{
		t:      t,
		done:   make(chan struct{}),
		single: make(map[lowlevel.RVID]lowlevel.RVHandler),
		subs:   make(map[lowlevel.RVID]lowlevel.RVHandler),
		rms:    make(map[lowlevel.RVID]lowlevel.RVBlob),
	}

	t.Cleanup(func() { close(ms.done) })

	return ms
}

type mockRMServerRMQ struct {
	id zkidentity.FullIdentity
	ms *mockRMServer
}

func (q *mockRMServerRMQ) QueueRM(orm lowlevel.OutboundRM, replyChan chan error) error {
	// Queuing itself never fails.

	go func() {
		// This goroutine simulates the sending process. The locking
		// here makes sure only one process is sending or registering
		// sends at one time.
		q.ms.Lock()
		defer q.ms.Unlock()

		rv, encrypted, err := orm.EncryptedMsg()
		if err != nil {
			replyChan <- err
			return
		}

		blob := lowlevel.RVBlob{
			Decoded: encrypted,
			ID:      rv,
		}

		// Sending is done.
		replyChan <- nil

		// This emulates the current server behavior of sending if the
		// prefix to the RV matched.
		//
		// TODO: this leaks metadata and needs to be fixed on the
		// protocol.
		for prv, single := range q.ms.single {
			if strings.HasPrefix(rv.String(), prv.String()) {
				delete(q.ms.single, prv)
				go single(blob)
				return
			}
		}

		for prv, sub := range q.ms.subs {
			if strings.HasPrefix(rv.String(), prv.String()) {
				go sub(blob)
				return
			}
		}

		q.ms.rms[rv] = blob
	}()

	return nil
}

func (q *mockRMServerRMQ) SendRM(orm lowlevel.OutboundRM) error {
	replyChan := make(chan error)
	_ = q.QueueRM(orm, replyChan)
	return <-replyChan
}

func (q *mockRMServerRMQ) MaxMsgSize() uint32 {
	return uint32(rpc.MaxMsgSizeForVersion(rpc.MaxMsgSizeV0))

}

type mockRMServerRMgr struct {
	ms *mockRMServer
}

func (rmgr *mockRMServerRMgr) Sub(rdzv lowlevel.RVID, handler lowlevel.RVHandler,
	payHandler lowlevel.SubPaidHandler) error {

	ms := rmgr.ms
	ms.Lock()
	defer ms.Unlock()

	if _, ok := ms.subs[rdzv]; ok {
		return fmt.Errorf("already have rv %s", rdzv)
	} else if _, ok := ms.single[rdzv]; ok {
		return fmt.Errorf("already have single rv %s", rdzv)
	}
	ms.subs[rdzv] = handler

	for rmrv, blob := range ms.rms {
		if strings.HasPrefix(rdzv.String(), rmrv.String()) {
			delete(ms.rms, rmrv)
			go handler(blob)
		}
	}
	return nil
}

func (rmgr *mockRMServerRMgr) Unsub(rdzv lowlevel.RVID) error {
	ms := rmgr.ms
	ms.Lock()
	defer ms.Unlock()

	delete(ms.subs, rdzv)
	delete(ms.single, rdzv)
	return nil
}

func (rmgr *mockRMServerRMgr) PrepayRVSub(rdzv lowlevel.RVID, subPaid lowlevel.SubPaidHandler) error {
	return nil
}

func (rmgr *mockRMServerRMgr) FetchPrepaidRV(ctx context.Context, rdzv lowlevel.RVID) (lowlevel.RVBlob, error) {
	panic("not working")
}

func (rmgr *mockRMServerRMgr) hasSub(rdzv lowlevel.RVID) bool {
	ms := rmgr.ms
	ms.Lock()
	defer ms.Unlock()
	if _, ok := ms.subs[rdzv]; ok {
		return true
	} else if _, ok := ms.single[rdzv]; ok {
		return true
	}
	return false
}

func (ms *mockRMServer) endpoints(id *zkidentity.FullIdentity) (*mockRMServerRMQ, *mockRMServerRMgr) {
	q := &mockRMServerRMQ{id: *id, ms: ms}
	r := &mockRMServerRMgr{ms: ms}
	return q, r
}
