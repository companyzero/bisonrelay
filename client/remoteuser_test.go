package client

import (
	"context"
	"math"
	"math/rand"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/companyzero/bisonrelay/client/clientintf"
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
	go ru1.updateRVs()
	go ru2.updateRVs()
	for _, ru := range []*RemoteUser{ru1, ru2} {
		ru := ru
		t.Cleanup(func() {
			select {
			case <-ru.stopped:
			default:
				ru.stop()
			}
		})
	}
	//ru1.log = testutils.TestLoggerSys(t, name1)
	//ru2.log = testutils.TestLoggerSys(t, name2)
	return ru1, ru2
}

// newTestRemoteUser is a minimal test user.
func newTestRemoteUser(t testing.TB, id clientintf.UserID, nick string) *RemoteUser {
	user := &RemoteUser{}
	user.setNick(nick)
	user.id = id
	return user
}

// TestRemoteUserPMs tests that users can send concurrent PMs to each other.
func TestRemoteUserPMs(t *testing.T) {
	t.Parallel()

	rnd := testRand(t)
	svr := newMockRMServer(t)

	aliceRemote, bobRemote := newRemoteUserTestPair(t, rnd, svr, "alice", "bob")

	nbMsgs := 100
	maxMsgSize := 10
	var gotAliceMtx, gotBobMtx sync.Mutex
	gotAliceMsgs, gotBobMsgs := make([]string, 0, nbMsgs), make([]string, 0, nbMsgs)
	bobRemote.rmHandler = func(_ *RemoteUser, h *rpc.RMHeader, p interface{}, ts time.Time) <-chan struct{} {
		// Bob receives Alice's msgs.
		pm := p.(rpc.RMPrivateMessage)
		gotAliceMtx.Lock()
		gotAliceMsgs = append(gotAliceMsgs, pm.Message)
		gotAliceMtx.Unlock()
		return nil
	}
	aliceRemote.rmHandler = func(_ *RemoteUser, h *rpc.RMHeader, p interface{}, ts time.Time) <-chan struct{} {
		// Alice receives Bob's msgs.
		pm := p.(rpc.RMPrivateMessage)
		gotBobMtx.Lock()
		gotBobMsgs = append(gotBobMsgs, pm.Message)
		gotBobMtx.Unlock()
		return nil
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
		case <-time.After(30 * time.Second):
			t.Fatal("timeout")
		}
	}

	// Ensure enough time to handle every message.
	time.Sleep(2000 * time.Millisecond)

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

	gotBobRM := make(chan struct{}, 1)
	bobRemote.rmHandler = func(_ *RemoteUser, h *rpc.RMHeader, p interface{}, ts time.Time) <-chan struct{} {
		gotBobRM <- struct{}{}
		return nil
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

// TestWaitsHandlerDone asserts that a remote user's waitHandlers() method does
// not return until all handlers finish processing.
func TestWaitsHandlerDone(t *testing.T) {
	t.Parallel()

	rnd := testRand(t)
	svr := newMockRMServer(t)

	aliceRemote, bobRemote := newRemoteUserTestPair(t, rnd, svr, "alice", "bob")
	aliceWaited, bobWaited := make(chan struct{}), make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		aliceRemote.waitHandlers(ctx.Done())
		close(aliceWaited)
	}()
	go func() {
		bobRemote.waitHandlers(ctx.Done())
		close(bobWaited)
	}()

	gotBobRM, returnFromBobHandler := make(chan struct{}, 1), make(chan struct{}, 1)
	bobRemote.rmHandler = func(_ *RemoteUser, h *rpc.RMHeader, p interface{}, ts time.Time) <-chan struct{} {
		doneChan := make(chan struct{})
		go func() {
			gotBobRM <- struct{}{}
			<-returnFromBobHandler
			close(doneChan)
		}()
		return doneChan
	}

	// Alice sends the PM.
	assert.NilErr(t, aliceRemote.sendPM("test"))

	// Bob receives it.
	assert.ChanWritten(t, gotBobRM)

	// Clients are asked to shut down.
	cancelTime := time.Now()
	aliceRemote.stop()
	bobRemote.stop()

	// Alice already finished, but Bob did not (up to 1/2 of the expected
	// timeout).
	assert.ChanWritten(t, aliceWaited)
	testDuration := remoteUserStopHandlerTimeout/2 - time.Since(cancelTime)
	assert.ChanNotWritten(t, bobWaited, testDuration)

	// Bob finishes processing.
	returnFromBobHandler <- struct{}{}

	// Finally, Bob's waitForHandlers() method returns.
	assert.ChanWritten(t, bobWaited)
}

// TestTimesoutSlowHandler asserts that a remote user's waitForHandler() method
// returns if the done chan is closed.
func TestTimesoutSlowHandler(t *testing.T) {
	t.Parallel()

	rnd := testRand(t)
	svr := newMockRMServer(t)

	aliceRemote, bobRemote := newRemoteUserTestPair(t, rnd, svr, "alice", "bob")
	aliceWaited, bobWaited := make(chan struct{}), make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		aliceRemote.waitHandlers(ctx.Done())
		close(aliceWaited)
	}()
	go func() {
		bobRemote.waitHandlers(ctx.Done())
		close(bobWaited)
	}()

	gotBobRM, returnFromBobHandler := make(chan struct{}, 1), make(chan struct{}, 1)
	bobRemote.rmHandler = func(_ *RemoteUser, h *rpc.RMHeader, p interface{}, ts time.Time) <-chan struct{} {
		doneChan := make(chan struct{})
		go func() {
			gotBobRM <- struct{}{}
			<-returnFromBobHandler
			close(doneChan)
		}()
		return doneChan
	}

	// Alice sends the PM.
	assert.NilErr(t, aliceRemote.sendPM("test"))

	// Bob receives it.
	assert.ChanWritten(t, gotBobRM)

	// Clients are asked to shut down.
	aliceRemote.stop()
	bobRemote.stop()

	// Alice already finished.
	assert.ChanWritten(t, aliceWaited)

	// Bob's waitForHandler() method only returns once the context is
	// canceled.
	assert.ChanNotWritten(t, bobWaited, time.Second)
	cancel()
	assert.ChanWritten(t, bobWaited)

	// Clean up handler goroutine.
	returnFromBobHandler <- struct{}{}
}
