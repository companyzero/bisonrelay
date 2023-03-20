//go:build e2elegacylntest
// +build e2elegacylntest

package client

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/companyzero/bisonrelay/client/clientdb"
	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/companyzero/bisonrelay/internal/assert"
	"github.com/companyzero/bisonrelay/internal/testutils"
	"github.com/companyzero/bisonrelay/rpc"
	"github.com/companyzero/bisonrelay/zkidentity"
	"github.com/davecgh/go-spew/spew"
	"github.com/decred/slog"
)

func newTestClient(t testing.TB, rnd io.Reader, name string) *Client {
	t.Helper()
	id := testID(t, rnd, name)
	logger := testutils.TestLoggerBackend(t, name)
	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	dcrlndDir := filepath.Join(homeDir, "dcrlndsimnetnodes", "dcrlnd2")
	dcrlndAddr := "localhost:20200"
	if name == "bob" {
		dcrlndDir = filepath.Join(homeDir, "dcrlndsimnetnodes", "dcrlnd1")
		dcrlndAddr = "localhost:20100"
	}

	pc, err := NewDcrlndPaymentClient(context.Background(), DcrlnPaymentClientCfg{
		TLSCertPath: filepath.Join(dcrlndDir, "tls.cert"),
		MacaroonPath: filepath.Join(dcrlndDir, "chain", "decred", "simnet",
			"admin.macaroon"),
		Address: dcrlndAddr,
		Log:     logger("LNPY"),
	})
	if err != nil {
		t.Fatal(err)
	}
	cfg := Config{
		Dialer:         clientintf.NetDialer("localhost:12345", slog.Disabled),
		CertConfirmer:  certConfirmerUnsafeAlwaysAccept,
		PayClient:      pc,
		Logger:         logger,
		LocalIDIniter:  fixedIDIniter(id),
		ReconnectDelay: 5 * time.Second,
		DB:             testDB(t, id, logger("FSDB")),
		Notifications:  NewNotificationManager(),
	}

	cli, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}

	return cli
}

func assertClientsKXd(t testing.TB, alice, bob *Client) {
	t.Helper()
	var gotAlice, gotBob bool
	for i := 0; (!gotAlice || !gotBob) && i < 100; i++ {
		if alice.UserExists(bob.PublicID()) {
			gotAlice = true
		}
		if bob.UserExists(alice.PublicID()) {
			gotBob = true
		}
		time.Sleep(time.Millisecond * 100)
	}
	if !gotAlice || !gotBob {
		t.Fatalf("KX did not complete %v %v", gotAlice, gotBob)
	}
}

func completeC2CKX(t testing.TB, alice, bob *Client) {
	t.Helper()

	// Alice generates the invite.
	br := bytes.NewBuffer(nil)
	_, err := alice.WriteNewInvite(br)
	if err != nil {
		t.Fatal(err)
	}

	// Bob reads the invite.
	bobInvite, err := bob.ReadInvite(br)
	if err != nil {
		t.Fatal(err)
	}

	// Bob accepts the invite.
	err = bob.AcceptInvite(bobInvite)
	if err != nil {
		t.Fatal(err)
	}

	// Ensure the KX completes.
	assertClientsKXd(t, alice, bob)
}

// TestE2EDcrlnPMs performs a full end-to-end test on client and server.
//
// TODO: this probably shouldn't be in the client package.
//
// FIXME: currently requires an already setup server configured to run with
// payments and the dcrlnd simnet tmux setup running.
func TestE2EDcrlnPMs(t *testing.T) {
	rnd := testRand(t)
	alice := newTestClient(t, rnd, "alice")
	bob := newTestClient(t, rnd, "bob")

	// Setup the callbacks for PMs to track the messages that will flow
	// between alice and bob.
	var gotAliceMtx, gotBobMtx sync.Mutex
	nbMsgs := 100 // MUST be less then 255
	gotAliceMsgs, gotBobMsgs := make([]string, nbMsgs), make([]string, nbMsgs)
	doneAliceMsgs, doneBobMsgs := make(chan error), make(chan error)
	var gotAliceCount, gotBobCount int
	bob.cfg.Notifications.Register(OnPMNtfn(func(ru *RemoteUser, pm rpc.RMPrivateMessage, ts time.Time) {
		// Bob receives Alice's msgs.
		gotAliceMtx.Lock()
		i, _ := strconv.ParseInt(pm.Message[2:4], 16, 64)
		gotAliceMsgs[i] = pm.Message
		gotAliceCount += 1
		if gotAliceCount == nbMsgs {
			close(doneAliceMsgs)
		}
		gotAliceMtx.Unlock()
	}))
	alice.cfg.Notifications.Register(OnPMNtfn(func(ru *RemoteUser, pm rpc.RMPrivateMessage, ts time.Time) {
		// Alice receives Bob's msgs.
		gotBobMtx.Lock()
		i, _ := strconv.ParseInt(pm.Message[2:4], 16, 64)
		gotBobMsgs[i] = pm.Message
		gotBobCount += 1
		if gotBobCount == nbMsgs {
			close(doneBobMsgs)
		}
		gotBobMtx.Unlock()
	}))

	// Start the clients. Return any run errors on errChan.
	ctx, cancel := context.WithCancel(context.Background())
	runErrChan := make(chan error)
	go func() { runErrChan <- alice.Run(ctx) }()
	go func() { runErrChan <- bob.Run(ctx) }()

	time.Sleep(100 * time.Millisecond) // Time for run() to start.
	completeC2CKX(t, alice, bob)

	// At this point, Alice and Bob are fully KX'd. Attempt to send
	// messages across the server.

	maxMsgSize := 1024
	wantAliceMsgs, wantBobMsgs := make([]string, nbMsgs), make([]string, nbMsgs)
	arnd := rand.New(rand.NewSource(rnd.Int63()))
	brnd := rand.New(rand.NewSource(rnd.Int63()))
	go func() {
		for i := 0; i < nbMsgs; i++ {
			wantAliceMsgs[i] = fmt.Sprintf("00%.2x00", i) + randomHex(arnd, 1+arnd.Intn(maxMsgSize))
			err := alice.PM(bob.PublicID(), wantAliceMsgs[i])
			if err != nil {
				doneAliceMsgs <- err
				return
			}
			time.Sleep(time.Duration(arnd.Intn(10000)) * time.Microsecond)
		}
	}()
	go func() {
		for i := 0; i < nbMsgs; i++ {
			wantBobMsgs[i] = fmt.Sprintf("00%.2x00", i) + randomHex(brnd, 1+brnd.Intn(maxMsgSize))
			err := bob.PM(alice.PublicID(), wantBobMsgs[i])
			if err != nil {
				doneBobMsgs <- err
				return
			}
			time.Sleep(time.Duration(brnd.Intn(10000)) * time.Microsecond)
		}
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
		case err := <-runErrChan:
			t.Fatal(err)
		case <-time.After(60 * time.Second):
			gotAliceMtx.Lock()
			gotBobMtx.Lock()
			t.Logf("Alice Msgs: %d, bob msgs: %d", len(gotAliceMsgs), len(gotBobMsgs))
			t.Fatal("timeout")
		}
	}

	gotAliceMtx.Lock()
	gotBobMtx.Lock()
	defer func() {
		gotAliceMtx.Unlock()
		gotBobMtx.Unlock()
	}()

	t.Logf("Alice msgs %d bob msgs %d", len(gotAliceMsgs), len(gotBobMsgs))
	if len(gotAliceMsgs) != nbMsgs {
		t.Fatalf("received only %d msgs from alice", len(gotAliceMsgs))
	}
	if len(gotBobMsgs) != nbMsgs {
		t.Fatalf("received only %d msgs from bob", len(gotBobMsgs))
	}

	// Ensure we got every message.
	for i := 0; i < nbMsgs; i++ {
		if wantAliceMsgs[i] != gotAliceMsgs[i] {
			t.Fatalf("unexpected alice msg %d: got %s, want %s",
				i, gotAliceMsgs[i], wantAliceMsgs[i])
		}
		if wantBobMsgs[i] != gotBobMsgs[i] {
			t.Fatalf("unexpected bob msg %d: got %s, want %s",
				i, gotBobMsgs[i], wantBobMsgs[i])
		}
	}

	cancel()

	for i := 0; i < 2; i++ {
		select {
		case err := <-runErrChan:
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("unexpected error: %v", err)
			}
		case <-time.After(time.Second):
			t.Fatal("timeout")
		}
	}
}

func TestE2EDcrlnContent(t *testing.T) {
	rnd := testRand(t)
	alice := newTestClient(t, rnd, "alice")
	bob := newTestClient(t, rnd, "bob")

	completedFileChan := make(chan string)
	bob.cfg.FileDownloadCompleted = func(user *RemoteUser, fm rpc.FileMetadata, diskPath string) {
		completedFileChan <- diskPath
	}
	listedFilesChan := make(chan interface{})
	bob.cfg.ContentListReceived = func(user *RemoteUser, files []clientdb.RemoteFile, listErr error) {
		if listErr != nil {
			listedFilesChan <- listErr
			return
		}
		mds := make([]rpc.FileMetadata, len(files))
		for i, rf := range files {
			mds[i] = rf.Metadata
		}
		listedFilesChan <- mds
	}

	// Start the clients. Return any run errors on errChan.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runErrChan := make(chan error)
	go func() { runErrChan <- alice.Run(ctx) }()
	go func() { runErrChan <- bob.Run(ctx) }()

	time.Sleep(100 * time.Millisecond) // Time for run() to start.
	completeC2CKX(t, alice, bob)

	// Alice will share 2 files (one globally, one with Bob).
	fGlobal, fShared := testRandomFile(t), testRandomFile(t)
	sfGlobal, mdGlobal, err := alice.ShareFile(fGlobal, nil, 1, false, "global file")
	if err != nil {
		t.Fatal(err)
	}
	bobUID := bob.PublicID()
	sfShared, mdShared, err := alice.ShareFile(fShared, &bobUID, 1, false, "user file")
	if err != nil {
		t.Fatal(err)
	}
	_ = sfShared

	// Helpers to assert listing works.
	lsAlice := func(dirs []string) {
		t.Helper()
		err := bob.ListUserContent(alice.PublicID(), dirs, "")
		assert.NilErr(t, err)
	}
	assertNextRes := func(wantFiles []rpc.FileMetadata) {
		t.Helper()
		select {
		case v := <-listedFilesChan:
			switch v := v.(type) {
			case error:
				t.Fatal(v)
			case []rpc.FileMetadata:
				if !reflect.DeepEqual(wantFiles, v) {
					t.Fatalf("unexpected files. got %s, want %s",
						spew.Sdump(v), spew.Sdump(wantFiles))
				}
			default:
				t.Fatalf("unexpected result: %s", spew.Sdump(v))
			}
		case <-time.After(10 * time.Second):
			t.Fatal("timeout")
		}
	}

	// First one should be of the global file.
	lsAlice([]string{rpc.RMFTDGlobal})
	assertNextRes([]rpc.FileMetadata{mdGlobal})

	// Second one should be the user shared file.
	lsAlice([]string{rpc.RMFTDShared})
	assertNextRes([]rpc.FileMetadata{mdShared})

	// Third one should be both.
	lsAlice([]string{rpc.RMFTDGlobal, rpc.RMFTDShared})
	assertNextRes([]rpc.FileMetadata{mdGlobal, mdShared})

	// Last call should error.
	lsAlice([]string{"*dir that doesn't exist"})
	select {
	case v := <-listedFilesChan:
		switch v := v.(type) {
		case error:
			// expected error.
		default:
			t.Fatalf("unexpected result: %s", spew.Sdump(v))
		}
	case <-time.After(10 * time.Second):
		t.Fatal("timeout")
	}

	// Bob starts fetching the global file from Alice.
	if err := bob.GetUserContent(alice.PublicID(), sfGlobal.FID); err != nil {
		t.Fatal(err)
	}

	// Ensure the file is received.
	select {
	case completedPath := <-completedFileChan:
		// Assert the global file is correct.
		orig, err := os.ReadFile(fGlobal)
		orFatal(t, err)
		fetched, err := os.ReadFile(completedPath)
		orFatal(t, err)
		if !bytes.Equal(orig, fetched) {
			t.Fatal("Unequal original and fetched files")
		}

	case <-time.After(30 * time.Second):
		t.Fatal("timeout")
	}

	// Alice will send a file directly to bob.
	fSent := testRandomFile(t)
	err = alice.SendFile(bob.PublicID(), fSent)
	assert.NilErr(t, err)

	// Bob should receive it.
	select {
	case completedPath := <-completedFileChan:
		// Assert the sent file is correct.
		orig, err := os.ReadFile(fSent)
		orFatal(t, err)
		fetched, err := os.ReadFile(completedPath)
		orFatal(t, err)
		if !bytes.Equal(orig, fetched) {
			t.Fatal("Unequal original and fetched files")
		}

	case <-time.After(30 * time.Second):
		t.Fatal("timeout")
	}
}

func TestE2EDcrlnTipUser(t *testing.T) {
	rnd := testRand(t)
	alice := newTestClient(t, rnd, "alice")
	bob := newTestClient(t, rnd, "bob")

	// Start the clients. Return any run errors on errChan.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = alice.Run(ctx) }()
	go func() { _ = bob.Run(ctx) }()

	time.Sleep(100 * time.Millisecond) // Time for run() to start.
	completeC2CKX(t, alice, bob)

	progressErrChan := make(chan error, 1)
	alice.cfg.Notifications.Register(OnTipAttemptProgressNtfn(func(ru *RemoteUser, amtMAtoms int64, completed bool, attempt int, attemptErr error, willRetry bool) {
		progressErrChan <- attemptErr
	}))

	// Send a tip from Alice to Bob. Sending should succeed and we should
	// get a completed progress event.
	errChan := make(chan error)
	const maxAttempts = 1
	go func() { errChan <- alice.TipUser(bob.PublicID(), 0.00001, maxAttempts) }()
	assert.NilErrFromChan(t, errChan)
	assert.NilErrFromChan(t, progressErrChan)
}

// TestE2EDcrlnOfflineMsgs ensures that attempting to send messages while the
// counterparty is offline works.
func TestE2EDcrlnOfflineMsgs(t *testing.T) {
	rnd := testRand(t)
	alice := newTestClient(t, rnd, "alice")
	bob := newTestClient(t, rnd, "bob")

	alicePMs, bobPMs := make(chan string), make(chan string)
	alice.cfg.Notifications.Register(OnPMNtfn(func(ru *RemoteUser, pm rpc.RMPrivateMessage, ts time.Time) {
		go func() { alicePMs <- pm.Message }()
	}))
	bob.cfg.Notifications.Register(OnPMNtfn(func(ru *RemoteUser, pm rpc.RMPrivateMessage, ts time.Time) {
		go func() { bobPMs <- pm.Message }()
	}))
	aliceSess, bobSess := make(chan bool), make(chan bool)
	alice.cfg.ServerSessionChanged = func(connected bool, pushRate, subRate, expDays uint64) {
		go func() { aliceSess <- connected }()
	}
	bob.cfg.ServerSessionChanged = func(connected bool, pushRate, subRate, expDays uint64) {
		go func() { bobSess <- connected }()
	}
	assertSess := func(c chan bool, wantSess bool) {
		t.Helper()
		select {
		case gotSess := <-c:
			if wantSess != gotSess {
				t.Fatalf("unexpected session. got %v, want %v",
					gotSess, wantSess)
			}
		case <-time.After(10 * time.Second):
			t.Fatal("timeout")
		}
	}

	// Helper to verify the Alice -> Bob ratchet works.
	checkAliceBobRatchet := func() {
		t.Helper()

		testMsg := "test message " + strconv.Itoa(rand.Int())
		if err := alice.PM(bob.PublicID(), testMsg); err != nil {
			t.Fatalf("Alice PM() error: %v", err)
		}
		if err := bob.PM(alice.PublicID(), testMsg); err != nil {
			t.Fatalf("Bob PM() error: %v", err)
		}
		select {
		case gotPM := <-alicePMs:
			if gotPM != testMsg {
				t.Fatalf("unexpected msg: got %v, want %v", gotPM, testMsg)
			}
		case <-time.After(10 * time.Second):
			t.Fatalf("timeout waiting for bob msg")
		}
		select {
		case gotPM := <-bobPMs:
			if gotPM != testMsg {
				t.Fatalf("unexpected msg: got %v, want %v", gotPM, testMsg)
			}
		case <-time.After(10 * time.Second):
			t.Fatalf("timeout waiting for alice msg")
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	runErr := make(chan error)
	defer func() {
		err := <-runErr
		if !errors.Is(err, context.Canceled) {
			t.Logf("Fatal run err: %v", err)
		}
		err = <-runErr
		if !errors.Is(err, context.Canceled) {
			t.Logf("Fatal run err: %v", err)
		}
	}()
	defer cancel()
	go func() { runErr <- alice.Run(ctx) }()
	go func() { runErr <- bob.Run(ctx) }()
	time.Sleep(100 * time.Millisecond) // Time for run() to start.
	completeC2CKX(t, alice, bob)
	checkAliceBobRatchet()

	// Send a pm from alice. This advances the ratchet and is used to test
	// an old bug scenario where alice would listen on incorrect RV points.
	if err := alice.PM(bob.PublicID(), "boo"); err != nil {
		t.Fatalf("alice PM() error: %v", err)
	}

	// Consume connection events.
	assertSess(aliceSess, true)
	assertSess(bobSess, true)

	// Let alice go offline.
	alice.RemainOffline()
	assertSess(aliceSess, false)

	// Send PMs from bob.
	nbMsgs := 5
	testMsg := "bob msg while alice offline"
	for i := 0; i < nbMsgs; i++ {
		if err := bob.PM(alice.PublicID(), testMsg); err != nil {
			t.Fatalf("Bob PM() error: %v", err)
		}
	}

	// Alice does _not_ get it.
	select {
	case gotPM := <-alicePMs:
		t.Fatalf("Unexpected pm %v", gotPM)
	case <-time.After(time.Second):
		// Expected.
	}

	// Alice goes online.
	alice.GoOnline()
	assertSess(aliceSess, true)

	// Alice gets the messages.
	for i := 0; i < nbMsgs; i++ {
		select {
		case gotPM := <-alicePMs:
			if gotPM != testMsg {
				t.Fatalf("unexpected msg: got %v, want %v", gotPM, testMsg)
			}

		case <-time.After(10 * time.Second):
			t.Fatal("timeout")
		}
	}
}

// TestE2EDcrlnSendOfflineMsgs ensures that attempting to send messages while
// local client is offline works.
func TestE2EDcrlnSendOfflineMsgs(t *testing.T) {
	rnd := testRand(t)
	alice := newTestClient(t, rnd, "alice")
	bob := newTestClient(t, rnd, "bob")

	alicePMs, bobPMs := make(chan string), make(chan string)
	alice.cfg.Notifications.Register(OnPMNtfn(func(ru *RemoteUser, pm rpc.RMPrivateMessage, ts time.Time) {
		go func() { alicePMs <- pm.Message }()
	}))
	bob.cfg.Notifications.Register(OnPMNtfn(func(ru *RemoteUser, pm rpc.RMPrivateMessage, ts time.Time) {
		go func() { bobPMs <- pm.Message }()
	}))
	aliceSess, bobSess := make(chan bool), make(chan bool)
	alice.cfg.ServerSessionChanged = func(connected bool, pushRate, subRate, expDays uint64) {
		go func() { aliceSess <- connected }()
	}
	bob.cfg.ServerSessionChanged = func(connected bool, pushRate, subRate, expDays uint64) {
		go func() { bobSess <- connected }()
	}
	assertSess := func(c chan bool, wantSess bool) {
		t.Helper()
		select {
		case gotSess := <-c:
			if wantSess != gotSess {
				t.Fatalf("unexpected session. got %v, want %v",
					gotSess, wantSess)
			}
		case <-time.After(10 * time.Second):
			t.Fatal("timeout")
		}
	}

	// Helper to verify the Alice -> Bob ratchet works.
	checkAliceBobRatchet := func() {
		t.Helper()

		testMsg := "test message " + strconv.Itoa(rand.Int())
		if err := alice.PM(bob.PublicID(), testMsg); err != nil {
			t.Fatalf("Alice PM() error: %v", err)
		}
		if err := bob.PM(alice.PublicID(), testMsg); err != nil {
			t.Fatalf("Bob PM() error: %v", err)
		}
		select {
		case gotPM := <-alicePMs:
			if gotPM != testMsg {
				t.Fatalf("unexpected msg: got %v, want %v", gotPM, testMsg)
			}
		case <-time.After(10 * time.Second):
			t.Fatalf("timeout waiting for bob msg")
		}
		select {
		case gotPM := <-bobPMs:
			if gotPM != testMsg {
				t.Fatalf("unexpected msg: got %v, want %v", gotPM, testMsg)
			}
		case <-time.After(10 * time.Second):
			t.Fatalf("timeout waiting for alice msg")
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	runErr := make(chan error)
	defer func() {
		err := <-runErr
		if !errors.Is(err, context.Canceled) {
			t.Logf("Fatal run err: %v", err)
		}
		err = <-runErr
		if !errors.Is(err, context.Canceled) {
			t.Logf("Fatal run err: %v", err)
		}
	}()
	defer cancel()
	go func() { runErr <- alice.Run(ctx) }()
	go func() { runErr <- bob.Run(ctx) }()
	time.Sleep(100 * time.Millisecond) // Time for run() to start.
	completeC2CKX(t, alice, bob)
	checkAliceBobRatchet()

	// Consume connection events.
	assertSess(aliceSess, true)
	assertSess(bobSess, true)

	// Let alice go offline.
	alice.RemainOffline()
	assertSess(aliceSess, false)

	// Send PMs from alice
	nbMsgs := 5
	testMsg := "alice msg while bob offline"
	for i := 0; i < nbMsgs; i++ {
		go func() {
			if err := alice.PM(bob.PublicID(), testMsg); err != nil {
				t.Fatalf("alice PM() error: %v", err)
			}
		}()
	}

	// Bob does _not_ get it.
	select {
	case gotPM := <-bobPMs:
		t.Fatalf("Unexpected pm %v", gotPM)
	case <-time.After(time.Second):
		// Expected.
	}

	// Alice goes online.
	t.Logf("=================== Alice going online")
	alice.GoOnline()
	assertSess(aliceSess, true)

	// Bob gets the messages.
	for i := 0; i < nbMsgs; i++ {
		select {
		case gotPM := <-bobPMs:
			if gotPM != testMsg {
				t.Fatalf("unexpected msg: got %v, want %v", gotPM, testMsg)
			}

		case <-time.After(10 * time.Second):
			t.Fatal("timeout")
		}
	}

	// Ensure we haven't gotten any run errors.
	select {
	case err := <-runErr:
		t.Fatalf("Run err: %v", err)
	case <-time.After(time.Second):
	}
}

// TestE2EDcrlnBlockIgnore ensures that the block and ignore features work as
// intended.
func TestE2EDcrlnBlockIgnore(t *testing.T) {
	rnd := testRand(t)
	alice := newTestClient(t, rnd, "alice")
	bob := newTestClient(t, rnd, "bob")
	charlie := newTestClient(t, rnd, "charlie")

	alicePMs, bobPMs := make(chan string), make(chan string)
	alice.cfg.Notifications.Register(OnPMNtfn(func(ru *RemoteUser, pm rpc.RMPrivateMessage, ts time.Time) {
		go func() { alicePMs <- pm.Message }()
	}))
	bob.cfg.Notifications.Register(OnPMNtfn(func(ru *RemoteUser, pm rpc.RMPrivateMessage, ts time.Time) {
		go func() { bobPMs <- pm.Message }()
	}))

	bobBlockedChan := make(chan struct{})
	bob.cfg.UserBlocked = func(ru *RemoteUser) {
		go func() { bobBlockedChan <- struct{}{} }()
	}

	aliceInviteChan, bobInviteChan := make(chan error, 1), make(chan error, 1)
	alice.cfg.Notifications.Register(OnInvitedToGCNtfn(func(user *RemoteUser, iid uint64, invite rpc.RMGroupInvite) {
		aliceInviteChan <- alice.AcceptGroupChatInvite(iid)
	}))
	bob.cfg.Notifications.Register(OnInvitedToGCNtfn(func(user *RemoteUser, iid uint64, invite rpc.RMGroupInvite) {
		bobInviteChan <- bob.AcceptGroupChatInvite(iid)
	}))
	aliceGCListChan, bobGCListChan := make(chan struct{}, 1), make(chan struct{}, 1)
	alice.cfg.Notifications.Register(OnJoinedGCNtfn(func(gc rpc.RMGroupList) {
		aliceGCListChan <- struct{}{}
	}))
	bob.cfg.Notifications.Register(OnJoinedGCNtfn(func(gc rpc.RMGroupList) {
		bobGCListChan <- struct{}{}
	}))
	aliceGCMChan, bobGCMChan, charlieGCMChan := make(chan string), make(chan string), make(chan string)
	alice.cfg.Notifications.Register(OnGCMNtfn(func(user *RemoteUser, msg rpc.RMGroupMessage, ts time.Time) {
		aliceGCMChan <- msg.Message
	}))
	bob.cfg.Notifications.Register(OnGCMNtfn(func(user *RemoteUser, msg rpc.RMGroupMessage, ts time.Time) {
		bobGCMChan <- msg.Message
	}))
	charlie.cfg.Notifications.Register(OnGCMNtfn(func(user *RemoteUser, msg rpc.RMGroupMessage, ts time.Time) {
		charlieGCMChan <- msg.Message
	}))

	ctx, cancel := context.WithCancel(context.Background())
	runErr := make(chan error)
	defer func() {
		err := <-runErr
		if !errors.Is(err, context.Canceled) {
			t.Logf("Fatal run err: %v", err)
		}
		err = <-runErr
		if !errors.Is(err, context.Canceled) {
			t.Logf("Fatal run err: %v", err)
		}
		err = <-runErr
		if !errors.Is(err, context.Canceled) {
			t.Logf("Fatal run err: %v", err)
		}
	}()
	defer cancel()
	go func() { runErr <- alice.Run(ctx) }()
	go func() { runErr <- bob.Run(ctx) }()
	go func() { runErr <- charlie.Run(ctx) }()
	time.Sleep(100 * time.Millisecond) // Time for run() to start.
	completeC2CKX(t, alice, bob)
	completeC2CKX(t, alice, charlie)
	completeC2CKX(t, bob, charlie)

	// Helper to check if one of the clients can send a PM that is received
	// by the other.
	assertCanPM := func(src, dst *Client, dstChan chan string, wantPM bool) {
		t.Helper()
		testMsg := "test message " + strconv.Itoa(rand.Int())
		if err := src.PM(dst.PublicID(), testMsg); err != nil {
			t.Fatalf("PM error: %v", err)
		}
		select {
		case gotMsg := <-dstChan:
			if !wantPM {
				t.Fatalf("unexpected received PM %q", gotMsg)
			}
			if gotMsg != testMsg {
				t.Fatalf("unexpected msg: got %q, want %q",
					gotMsg, testMsg)
			}
		case <-time.After(time.Second):
			if wantPM {
				t.Fatal("timeout")
			}
		}
	}

	gcName := "testGC"
	var gcID zkidentity.ShortID
	assertRecvGCM := func(dstChan chan string, wantGCM string) {
		t.Helper()
		select {
		case gotMsg := <-dstChan:
			if gotMsg != wantGCM {
				t.Fatalf("unexpected msg: got %q, want %q",
					gotMsg, wantGCM)
			}
		case <-time.After(40 * time.Second):
			if wantGCM != "" {
				t.Fatal("timeout")
			}
		}
	}

	// Sleep until initial gc delay elapses.
	//
	// TODO: parametrize in client so this doesn't have to exist.
	time.Sleep(time.Second * 30)

	// Alice and Bob should be able to PM each other.
	assertCanPM(alice, bob, bobPMs, true)
	assertCanPM(bob, alice, alicePMs, true)

	// Alice ignores bob.
	if err := alice.Ignore(bob.PublicID(), true); err != nil {
		t.Fatal(err)
	}

	// Alice can msg Bob, Bob cannot msg Alice.
	assertCanPM(alice, bob, bobPMs, true)
	assertCanPM(bob, alice, alicePMs, false)

	// Alice un-ignores Bob. Bob can now msg Alice again.
	if err := alice.Ignore(bob.PublicID(), false); err != nil {
		t.Fatal(err)
	}
	assertCanPM(bob, alice, alicePMs, true)

	// Alice blocks Bob.
	if err := alice.Block(bob.PublicID()); err != nil {
		t.Fatal(err)
	}
	select {
	case <-bobBlockedChan:
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}

	// Attempting to send a message from either Bob or Alice errors.
	if err := alice.PM(bob.PublicID(), "test"); !errors.Is(err, userNotFoundError{}) {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := bob.PM(alice.PublicID(), "test"); !errors.Is(err, userNotFoundError{}) {
		t.Fatalf("unexpected error: %v", err)
	}

	// Charlie knows Alice and Bob. Create a new GC, and invite both. It
	// should _not_ trigger a new autokx between Alice and Bob.
	var err error
	if gcID, err = charlie.NewGroupChat(gcName); err != nil {
		t.Fatal(err)
	}

	// Invite Alice and Bob to it and ensure they joined.
	if err := charlie.InviteToGroupChat(gcID, alice.PublicID()); err != nil {
		t.Fatal(err)
	}
	if err := charlie.InviteToGroupChat(gcID, bob.PublicID()); err != nil {
		t.Fatal(err)
	}
	select {
	case <-aliceGCListChan:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout")
	}
	select {
	case <-bobGCListChan:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout")
	}

	// Give some time to see if autokx completes.
	time.Sleep(time.Second)

	// Ensure a GCM sent by charlie is received by both Alice and Bob.
	testMsg := "test message " + strconv.Itoa(rand.Int())
	if err := charlie.GCMessage(gcID, testMsg, 0, nil); err != nil {
		t.Fatalf("GCM error: %v", err)
	}
	assertRecvGCM(aliceGCMChan, testMsg)
	assertRecvGCM(bobGCMChan, testMsg)

	// Ensure a GCM sent by Alice is seen by Charlie, but not Bob.
	testMsg = "test message " + strconv.Itoa(rand.Int())
	if err := alice.GCMessage(gcID, testMsg, 0, nil); err != nil {
		t.Fatalf("GCM error: %v", err)
	}
	assertRecvGCM(charlieGCMChan, testMsg)
	assertRecvGCM(bobGCMChan, "")

	// Ensure a GCM sent by Bob is seen by Charlie, but not Alice.
	testMsg = "test message " + strconv.Itoa(rand.Int())
	if err := bob.GCMessage(gcID, testMsg, 0, nil); err != nil {
		t.Fatalf("GCM error: %v", err)
	}
	assertRecvGCM(charlieGCMChan, testMsg)
	assertRecvGCM(aliceGCMChan, "")

	// Ensure Alice and Bob are not kx'd together.
	if alice.UserExists(bob.PublicID()) {
		t.Fatal("Alice knows Bob (but shouldn't)")
	}
	if bob.UserExists(alice.PublicID()) {
		t.Fatal("Bob knows Alice (but shouldn't)")
	}
}
