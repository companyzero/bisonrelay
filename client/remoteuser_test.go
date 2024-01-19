package client

import (
	"context"
	"errors"
	"math"
	"math/rand"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/companyzero/bisonrelay/internal/assert"
	"github.com/companyzero/bisonrelay/rpc"
	"github.com/companyzero/bisonrelay/zkidentity"
)

func newRemoteUserTestPair(t testing.TB, rnd *rand.Rand, svr *mockRMServer, name1, name2 string) (*RemoteUser, *RemoteUser) {
	id1 := testID(t, rnd, name1)
	id2 := testID(t, rnd, name2)
	r1, r2 := pairedRatchet(t, rnd, id1, id2)
	q1, rm1 := svr.endpoints(id1)
	q2, rm2 := svr.endpoints(id2)
	db1 := testDB(t, id1, nil)
	db2 := testDB(t, id2, nil)
	runTestDB(t, db1)
	runTestDB(t, db2)
	ru1 := newRemoteUser(q1, rm1, db1, &id2.Public, id1.SignMessage, r1)
	ru2 := newRemoteUser(q2, rm2, db2, &id1.Public, id2.SignMessage, r2)
	//ru1.log = testutils.TestLoggerSys(t, name1)
	//ru2.log = testutils.TestLoggerSys(t, name2)
	return ru1, ru2
}

// TestRemoteUserPMs tests that users can send concurrent PMs to each other.
func TestRemoteUserPMs(t *testing.T) {
	t.Parallel()

	rnd := testRand(t)
	svr := newMockRMServer(t)

	aliceRemote, bobRemote := newRemoteUserTestPair(t, rnd, svr, "alice", "bob")
	aliceRun, bobRun := make(chan error), make(chan error)
	ctx, cancel := context.WithCancel(context.Background())
	go func() { aliceRun <- aliceRemote.run(ctx) }()
	go func() { bobRun <- bobRemote.run(ctx) }()

	nbMsgs := 100
	maxMsgSize := 10
	var gotAliceMtx, gotBobMtx sync.Mutex
	gotAliceMsgs, gotBobMsgs := make([]string, 0, nbMsgs), make([]string, 0, nbMsgs)
	bobRemote.rmHandler = func(_ *RemoteUser, h *rpc.RMHeader, p interface{}, ts time.Time) {
		// Bob receives Alice's msgs.
		pm := p.(rpc.RMPrivateMessage)
		gotAliceMtx.Lock()
		gotAliceMsgs = append(gotAliceMsgs, pm.Message)
		gotAliceMtx.Unlock()
	}
	aliceRemote.rmHandler = func(_ *RemoteUser, h *rpc.RMHeader, p interface{}, ts time.Time) {
		// Alice receives Bob's msgs.
		pm := p.(rpc.RMPrivateMessage)
		gotBobMtx.Lock()
		gotBobMsgs = append(gotBobMsgs, pm.Message)
		gotBobMtx.Unlock()
	}

	wantAliceMsgs, wantBobMsgs := make([]string, nbMsgs), make([]string, nbMsgs)
	doneAliceMsgs, doneBobMsgs := make(chan error), make(chan error)
	arnd := rand.New(rand.NewSource(rnd.Int63()))
	brnd := rand.New(rand.NewSource(rnd.Int63()))
	go func() {
		for i := 0; i < nbMsgs; i++ {
			wantAliceMsgs[i] = randomHex(arnd, 1+arnd.Intn(maxMsgSize))
			err := aliceRemote.sendPM(wantAliceMsgs[i])
			if err != nil {
				doneAliceMsgs <- err
				return
			}
			time.Sleep(time.Duration(arnd.Intn(10000)) * time.Microsecond)
		}
		close(doneAliceMsgs)
	}()
	go func() {
		for i := 0; i < nbMsgs; i++ {
			wantBobMsgs[i] = randomHex(brnd, 1+brnd.Intn(maxMsgSize))
			err := bobRemote.sendPM(wantBobMsgs[i])
			if err != nil {
				doneBobMsgs <- err
				return
			}
			time.Sleep(time.Duration(brnd.Intn(10000)) * time.Microsecond)
		}
		close(doneBobMsgs)
	}()

	// Wait until all messages are done.
	for doneAliceMsgs != nil || doneBobMsgs != nil {
		select {
		case err := <-doneAliceMsgs:
			if err != nil {
				t.Fatalf("alice error: %v", err)
			}
			doneAliceMsgs = nil
		case err := <-doneBobMsgs:
			if err != nil {
				t.Fatalf("bob error: %v", err)
			}
			doneBobMsgs = nil
		case err := <-aliceRun:
			t.Fatal(err)
		case err := <-bobRun:
			t.Fatal(err)
		case <-time.After(10 * time.Second):
			t.Fatal("timeout")
		}
	}

	// Ensure enough time to handle every message.
	time.Sleep(2000 * time.Millisecond)

	cancel()
	select {
	case err := <-aliceRun:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("unexpected error: got %v, want %v", err,
				context.Canceled)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}
	select {
	case err := <-bobRun:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("unexpected error: got %v, want %v", err,
				context.Canceled)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}

	gotAliceMtx.Lock()
	gotBobMtx.Lock()
	defer func() {
		gotAliceMtx.Unlock()
		gotBobMtx.Unlock()
	}()

	if len(wantAliceMsgs) != len(gotAliceMsgs) {
		t.Fatalf("Unexpected nb of alice messages: got %d, want %d",
			len(gotAliceMsgs), len(wantAliceMsgs))
	}
	if len(wantBobMsgs) != len(gotBobMsgs) {
		t.Fatalf("Unexpected nb of bob messages: got %d, want %d",
			len(gotBobMsgs), len(wantBobMsgs))
	}
	if len(wantAliceMsgs) != nbMsgs {
		t.Fatalf("Did not receive all alice messages: %d != %d",
			len(wantAliceMsgs), nbMsgs)
	}
	if len(wantBobMsgs) != nbMsgs {
		t.Fatalf("Did not receive all bob messages: %d != %d",
			len(wantBobMsgs), nbMsgs)
	}

	for _, want := range wantAliceMsgs {
		var found bool
		for _, got := range gotAliceMsgs {
			if want == got {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("Alice did not receive message %v", want)
		}
	}
	for _, want := range wantBobMsgs {
		var found bool
		for _, got := range gotBobMsgs {
			if want == got {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("Bob did not receive message %v", want)
		}
	}
}

// TestAttemptMaxRMSize tests an attempt to send an RM larger than the max
// acceptable size.
func TestAttemptMaxRMSize(t *testing.T) {
	t.Parallel()

	rnd := testRand(t)
	svr := newMockRMServer(t)

	aliceRemote, bobRemote := newRemoteUserTestPair(t, rnd, svr, "alice", "bob")
	aliceRun, bobRun := make(chan error), make(chan error)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { aliceRun <- aliceRemote.run(ctx) }()
	go func() { bobRun <- bobRemote.run(ctx) }()

	gotBobRM := make(chan struct{}, 1)
	bobRemote.rmHandler = func(_ *RemoteUser, h *rpc.RMHeader, p interface{}, ts time.Time) {
		gotBobRM <- struct{}{}
	}

	rm := rpc.RMFTGetChunkReply{
		FileID: zkidentity.ShortID{}.String(),
		Index:  math.MaxInt,
		Tag:    math.MaxUint32,
		Chunk:  make([]byte, 1024*1024),
	}

	// Send a valid file chunk with the max size.
	err := aliceRemote.sendRM(rm, "")
	assert.NilErr(t, err)
	assert.ChanWritten(t, gotBobRM)

	// Send a file slightly larger than the max size.
	rm.Chunk = make([]byte, (1024+15)*1024)
	err = aliceRemote.sendRM(rm, "")
	assert.ErrorIs(t, err, errRMTooLarge)

	// Send a very large PM.
	pm := rpc.RMPrivateMessage{
		Message: strings.Repeat(" ", (1024+357)*1024+703),
	}
	err = aliceRemote.sendRM(pm, "")
	assert.ErrorIs(t, err, errRMTooLarge)
}
