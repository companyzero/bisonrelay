package client

import (
	"bytes"
	"context"
	"errors"
	"io"
	"math/rand"
	"testing"
	"time"

	"github.com/companyzero/bisonrelay/client/clientdb"
	"github.com/companyzero/bisonrelay/client/internal/lowlevel"
	"github.com/companyzero/bisonrelay/ratchet"
	"github.com/companyzero/bisonrelay/rpc"
	"github.com/companyzero/bisonrelay/zkidentity"
)

func newTestKXList(t testing.TB, svr *mockRMServer, rnd io.Reader, name string) *kxList {
	id := testID(t, rnd, name)
	q, r := svr.endpoints(id)
	db := testDB(t, id, nil)
	runTestDB(t, db)
	localID := localIdentityFromFull(id)
	kxl := newKXList(q, r, &localID, localID.public, db, context.Background())
	kxl.randReader = rnd
	//kxl.log = testutils.TestLoggerSys(t, name)
	return kxl
}

// TestKXSucceeds tests that a pair of guest/host lists can perform KX when
// they are in contact via a compliant server.
func TestKXSucceeds(t *testing.T) {
	t.Parallel()

	rnd := testRand(t)
	arnd := rand.New(rand.NewSource(rnd.Int63()))
	brnd := rand.New(rand.NewSource(rnd.Int63()))
	svr := newMockRMServer(t)

	// Create the test kx lists.
	alice := newTestKXList(t, svr, arnd, "alice")
	bob := newTestKXList(t, svr, brnd, "bob")

	// Ensure we're tracking the success of kx.
	aliceRChan, bobRChan := make(chan *ratchet.Ratchet), make(chan *ratchet.Ratchet)
	alice.kxCompleted = func(id *zkidentity.PublicIdentity, r *ratchet.Ratchet, irrv, mrrv, trrv clientdb.RawRVID) {
		aliceRChan <- r
	}
	bob.kxCompleted = func(id *zkidentity.PublicIdentity, r *ratchet.Ratchet, irrv, mrrv, trrv clientdb.RawRVID) {
		bobRChan <- r
	}

	// Create the invite in the host.
	buff := new(bytes.Buffer)
	_, err := alice.createInvite(buff, nil, nil, false, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Load the invite in the guest and kickstart the kx process.
	bobInvite, err := bob.decodeInvite(buff)
	if err != nil {
		t.Fatal(err)
	}
	err = bob.acceptInvite(bobInvite, false, false)
	if err != nil {
		t.Fatal(err)
	}

	var aliceR, bobR *ratchet.Ratchet
	for aliceRChan != nil || bobRChan != nil {
		select {
		case aliceR = <-aliceRChan:
			aliceRChan = nil
		case bobR = <-bobRChan:
			bobRChan = nil
		case <-time.After(30 * time.Second):
			t.Fatal("timeout")
		}
	}

	// Ensure the ratchets can in fact communicate.
	assertRatchetsSynced(t, aliceR, bobR)
}

// TestRepeatedResetActivation tests that the kx list correctly handles repeated
// activation of a reset RV.
func TestRepeatedResetActivation(t *testing.T) {
	t.Parallel()

	rnd := testRand(t)
	arnd := rand.New(rand.NewSource(rnd.Int63()))
	brnd := rand.New(rand.NewSource(rnd.Int63()))
	svr := newMockRMServer(t)

	// Create the test kx lists.
	alice := newTestKXList(t, svr, arnd, "alice")
	bob := newTestKXList(t, svr, brnd, "bob")

	// Create an invite for bob but change its initial RV point. This
	// ensures Bob won't complete the reset, allowing us to explicitly test
	// the scenario where Alice attempts to start the kx twice before one
	// completes.

	invite, err := bob.createInvite(nil, nil, nil, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	invite.InitialRendezvous[0] ^= 0xa5 // Prevent KX by changing the initial RV.
	packed, err := rpc.EncryptRMO(invite, alice.public(), 0)
	if err != nil {
		t.Fatal(err)
	}
	blob := lowlevel.RVBlob{Decoded: packed}

	bobPublic := bob.public()

	// Accepting the first reset should work.
	if err := alice.handleReset(&bobPublic, blob); err != nil {
		t.Fatalf("unexpected error on first handleReset: %v", err)
	}

	// The second call should error due to duplicated reset attempt.
	wantErr := clientdb.ErrAlreadyExists
	if err := alice.handleReset(&bobPublic, blob); !errors.Is(err, wantErr) {
		t.Fatalf("unexpected error on second handleReset: got %v, want %v",
			err, wantErr)
	}
}

func TestDropStaleKXs(t *testing.T) {
	t.Parallel()

	rnd := testRand(t)
	arnd := rand.New(rand.NewSource(rnd.Int63()))
	svr := newMockRMServer(t)
	alice := newTestKXList(t, svr, arnd, "alice")
	kxExpiryLimit := 24 * time.Hour

	// Manually create 2 kxs. One will be stale. Add them to Alice's DB.
	kxdOk := clientdb.KXData{
		Stage:     clientdb.KXStageStep2IDKX,
		InitialRV: ratchet.RVPoint{0: 0xff},
		Timestamp: time.Now().Add(-kxExpiryLimit / 2),
	}
	kxdStale := clientdb.KXData{
		Stage:     clientdb.KXStageStep2IDKX,
		InitialRV: ratchet.RVPoint{0: 0xee},
		Timestamp: time.Now().Add(-kxExpiryLimit - time.Hour),
	}
	err := alice.db.Update(context.Background(), func(tx clientdb.ReadWriteTx) error {
		if err := alice.db.SaveKX(tx, kxdOk); err != nil {
			return err
		}
		if err := alice.db.SaveKX(tx, kxdStale); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	// Listen to KXs. This should drop the stale kx.
	if err := alice.listenAllKXs(kxExpiryLimit); err != nil {
		t.Fatal(err)
	}

	// Double check via the mock RV manager: there should be a single
	// subscription, for the non-stale kx.
	rvmgr := alice.rmgr.(*mockRMServerRMgr)
	if rvmgr.hasSub(kxdStale.InitialRV) {
		t.Fatal("unexpected subscription for stale RV exists in RV manager")
	}
	if !rvmgr.hasSub(kxdOk.InitialRV) {
		t.Fatal("expected subscription for non-stale RV does not exist in RV manager")
	}
}
