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

		TipUserRestartDelay: time.Millisecond,
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
	_, err := alice.WriteNewInvite(br, nil)
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
	bob.cfg.Notifications.Register(OnFileDownloadCompleted(func(user *RemoteUser, fm rpc.FileMetadata, diskPath string) {
		completedFileChan <- diskPath
	}))
	listedFilesChan := make(chan interface{})
	bob.cfg.Notifications.Register(OnContentListReceived(func(user *RemoteUser, files []clientdb.RemoteFile, listErr error) {
		if listErr != nil {
			listedFilesChan <- listErr
			return
		}
		mds := make([]rpc.FileMetadata, len(files))
		for i, rf := range files {
			mds[i] = rf.Metadata
		}
		listedFilesChan <- mds
	}))

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
	sfGlobal, mdGlobal, err := alice.ShareFile(fGlobal, nil, 1, "global file")
	if err != nil {
		t.Fatal(err)
	}
	bobUID := bob.PublicID()
	sfShared, mdShared, err := alice.ShareFile(fShared, &bobUID, 1, "user file")
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
