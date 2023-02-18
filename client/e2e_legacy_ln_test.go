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

// TestE2EDcrlnGC tests E2E GCs.
func TestE2EDcrlnGC(t *testing.T) {
	rnd := testRand(t)
	alice := newTestClient(t, rnd, "alice")
	bob := newTestClient(t, rnd, "bob")
	gcName := "testgc"
	var gcID zkidentity.ShortID

	bobInviteChan := make(chan error, 1)
	bob.cfg.GCInviteHandler = func(user *RemoteUser, iid uint64, invite rpc.RMGroupInvite) {
		bobInviteChan <- bob.AcceptGroupChatInvite(iid)
	}
	aliceJoinChan := make(chan error, 3)
	alice.cfg.GCJoinHandler = func(user *RemoteUser, gc clientdb.GCAddressBookEntry) {
		if gc.ID != gcID {
			aliceJoinChan <- fmt.Errorf("not the correct name")
		} else {
			aliceJoinChan <- nil
		}
	}
	bobGCListChan := make(chan clientdb.GCAddressBookEntry, 3)
	bob.cfg.GCListUpdated = func(gc clientdb.GCAddressBookEntry) {
		bobGCListChan <- gc
	}

	// Start the clients. Return any run errors on errChan.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runErrChan := make(chan error)
	go func() { runErrChan <- alice.Run(ctx) }()
	go func() { runErrChan <- bob.Run(ctx) }()

	time.Sleep(100 * time.Millisecond) // Time for run() to start.
	completeC2CKX(t, alice, bob)

	// Create a new gc in Alice.
	var err error
	gcID, err = alice.NewGroupChat(gcName)
	assert.NilErr(t, err)

	// Invite bob to it.
	err = alice.InviteToGroupChat(gcID, bob.PublicID())
	assert.NilErr(t, err)

	// Bob should receive and accept the invitation.
	assert.NilErrFromChan(t, bobInviteChan)

	// Alice should receive the invite acceptance.
	assert.NilErrFromChan(t, aliceJoinChan)

	// Bob should receive the metadata of gc.
	select {
	case gc := <-bobGCListChan:
		if gc.ID != gcID {
			t.Fatal("wrong id")
		}
		if len(gc.Members) != 2 {
			t.Fatal("wrong nb of members")
		}
		if gc.Members[0] != alice.PublicID() || gc.Members[1] != bob.PublicID() {
			t.Fatal("wrong list of members")
		}
	case <-time.After(30 * time.Second):
		t.Fatal("timeout")
	}

	// Send concurrent gc msgs between Alice and Bob.
	nbMsgs := 1
	maxMsgSize := 1024
	arnd := rand.New(rand.NewSource(rnd.Int63()))
	brnd := rand.New(rand.NewSource(rnd.Int63()))
	aliceSentMsgs := make([]string, nbMsgs)
	bobSentMsgs := make([]string, nbMsgs)
	aliceReceivedMsgs := make(map[string]struct{}, nbMsgs)
	bobReceivedMsgs := make(map[string]struct{}, nbMsgs)
	var aliceMtx, bobMtx sync.Mutex
	aliceDoneMsgs := make(chan struct{})
	bobDoneMsgs := make(chan struct{})
	alice.cfg.Notifications.Register(OnGCMNtfn(func(ru *RemoteUser, msg rpc.RMGroupMessage, ts time.Time) {
		aliceMtx.Lock()
		aliceReceivedMsgs[msg.Message] = struct{}{}
		if len(aliceReceivedMsgs) == nbMsgs {
			close(aliceDoneMsgs)
		}
		aliceMtx.Unlock()
	}))
	bob.cfg.Notifications.Register(OnGCMNtfn(func(ru *RemoteUser, msg rpc.RMGroupMessage, ts time.Time) {
		bobMtx.Lock()
		bobReceivedMsgs[msg.Message] = struct{}{}
		if len(bobReceivedMsgs) == nbMsgs {
			close(bobDoneMsgs)
		}
		bobMtx.Unlock()
	}))

	sendErrChan := make(chan error)
	go func() {
		for i := 0; i < nbMsgs; i++ {
			aliceSentMsgs[i] = strconv.Itoa(i) + randomHex(arnd, 1+arnd.Intn(maxMsgSize))
			err := alice.GCMessage(gcID, aliceSentMsgs[i], rpc.MessageModeNormal, nil)
			if err != nil {
				sendErrChan <- err
			}
			time.Sleep(time.Duration(100000+arnd.Intn(100000)) * time.Microsecond)
		}
	}()
	go func() {
		for i := 0; i < nbMsgs; i++ {
			bobSentMsgs[i] = strconv.Itoa(i) + randomHex(brnd, 1+brnd.Intn(maxMsgSize))
			err := bob.GCMessage(gcID, bobSentMsgs[i], rpc.MessageModeNormal, nil)
			if err != nil {
				sendErrChan <- err
			}
			time.Sleep(time.Duration(100000+brnd.Intn(100000)) * time.Microsecond)
		}
	}()

	// Ensure all messages were received.
	for aliceDoneMsgs != nil || bobDoneMsgs != nil {
		select {
		case <-aliceDoneMsgs:
			aliceDoneMsgs = nil
		case <-bobDoneMsgs:
			bobDoneMsgs = nil
		case err := <-sendErrChan:
			t.Fatal(err)
		case <-time.After(60 * time.Second):
			t.Fatal("timeout")
		}
	}

	// Ensure all msgs are in order. Alice receives Bob's messages (and
	// vice-versa).
	for i := 0; i < nbMsgs; i++ {
		if _, ok := aliceReceivedMsgs[bobSentMsgs[i]]; !ok {
			t.Fatalf("did not find alice msg %d", i)
		}
		if _, ok := bobReceivedMsgs[aliceSentMsgs[i]]; !ok {
			t.Fatalf("did not find bob msg %d", i)
		}
	}

	// Create Charlie. Create a GC with the same name as the one from
	// Alice. KX with alice and join gc. Autokx should happen with Bob.
	charlie := newTestClient(t, rnd, "charlie")
	go func() { runErrChan <- charlie.Run(ctx) }()
	time.Sleep(100 * time.Millisecond)
	_, err = charlie.NewGroupChat(gcName)
	assert.NilErr(t, err)
	completeC2CKX(t, alice, charlie)
	charlieInviteChan := make(chan error, 1)
	charlie.cfg.GCInviteHandler = func(user *RemoteUser, iid uint64, invite rpc.RMGroupInvite) {
		charlieInviteChan <- charlie.AcceptGroupChatInvite(iid)
	}
	charlieGCListChan := make(chan clientdb.GCAddressBookEntry, 3)
	charlie.cfg.GCListUpdated = func(gc clientdb.GCAddressBookEntry) {
		charlieGCListChan <- gc
	}
	if err := alice.InviteToGroupChat(gcID, charlie.PublicID()); err != nil {
		t.Fatal(err)
	}
	assert.NilErrFromChan(t, charlieInviteChan)
	assert.ChanWritten(t, charlieGCListChan)

	// Bob should receive a list update containing charlie.
	select {
	case gc := <-bobGCListChan:
		if len(gc.Members) != 3 {
			t.Fatalf("wrong nb of members in update: %s", spew.Sdump(gc))
		}
		if gc.Members[2] != charlie.PublicID() {
			t.Fatalf("memers[2] is not charlie. got %s, want %s",
				gc.Members[2], charlie.PublicID())
		}
	case <-time.After(30 * time.Second):
		t.Fatal("timeout")
	}

	// The alias of Alice's GC in charlie should be different than the
	// original one, due to Charlie already having a GC with the same name.
	aliasInCharlie, err := charlie.GetGCAlias(gcID)
	assert.NilErr(t, err)
	if aliasInCharlie == gcName {
		t.Fatalf("Unexpected gc alias in charlie: got %s", aliasInCharlie)
	}

	// Modifying the alias should work.
	newAlias := "bleh"
	err = charlie.AliasGC(gcID, newAlias)
	assert.NilErr(t, err)
	aliasInCharlie, err = charlie.GetGCAlias(gcID)
	assert.NilErr(t, err)
	if aliasInCharlie != newAlias {
		t.Fatalf("Unexpected gc alias in charlie: got %s, want %s",
			aliasInCharlie, newAlias)
	}

	// Bob and Charlie should haved kxd already.
	assertClientsKXd(t, bob, charlie)

	// Bob and charlie should be able to PM each other.
	bobPMChan := make(chan string)
	bob.cfg.Notifications.Register(OnPMNtfn(func(user *RemoteUser, msg rpc.RMPrivateMessage, ts time.Time) {
		bobPMChan <- msg.Message
	}))
	charliePMChan := make(chan string)
	charlie.cfg.Notifications.Register(OnPMNtfn(func(user *RemoteUser, msg rpc.RMPrivateMessage, ts time.Time) {
		charliePMChan <- msg.Message
	}))
	testMsg := "booooo!!!! found you!!"
	err = charlie.PM(bob.PublicID(), testMsg)
	if err != nil {
		t.Fatal(err)
	}
	err = bob.PM(charlie.PublicID(), testMsg)
	if err != nil {
		t.Fatal(err)
	}

	var bobMsg, charlieMsg string
	for bobMsg == "" || charlieMsg == "" {
		select {
		case msg := <-bobPMChan:
			bobMsg = msg
		case msg := <-charliePMChan:
			charlieMsg = msg
		case <-time.After(30 * time.Second):
			t.Fatal("timeout")
		}
	}
	if bobMsg != charlieMsg {
		t.Fatalf("unexpected msgs: %q vs %q", bobMsg, charlieMsg)
	}
}

// TestE2EDcrlnGCKicks tests scenarios where users are leaving or being kicked
// from GC.
func TestE2EDcrlnGCKicks(t *testing.T) {
	rnd := testRand(t)
	alice := newTestClient(t, rnd, "alice")
	bob := newTestClient(t, rnd, "bob")
	charlie := newTestClient(t, rnd, "charlie")
	dave := newTestClient(t, rnd, "dave")
	gcName := "testgc"
	var gcID zkidentity.ShortID

	// Alice will be the admin and charlie will be an user we'll use to
	// check the events.

	aliceJoinChan := make(chan error, 3)
	alice.cfg.GCJoinHandler = func(user *RemoteUser, gc clientdb.GCAddressBookEntry) {
		aliceJoinChan <- nil
	}

	type partEvent struct {
		uid    UserID
		reason string
		kicked bool
	}
	charliePartChan := make(chan partEvent)
	charlie.cfg.GCUserParted = func(gcid GCID, uid UserID, reason string, kicked bool) {
		charliePartChan <- partEvent{uid: uid, reason: reason, kicked: kicked}
	}
	charlieKilledChan := make(chan struct{})
	charlie.cfg.GCKilled = func(gcid GCID, reason string) {
		charlieKilledChan <- struct{}{}
	}

	bob.cfg.GCInviteHandler = func(user *RemoteUser, iid uint64, invite rpc.RMGroupInvite) {
		go func() { bob.AcceptGroupChatInvite(iid) }()
	}
	charlie.cfg.GCInviteHandler = func(user *RemoteUser, iid uint64, invite rpc.RMGroupInvite) {
		go func() { charlie.AcceptGroupChatInvite(iid) }()
	}
	dave.cfg.GCInviteHandler = func(user *RemoteUser, iid uint64, invite rpc.RMGroupInvite) {
		go func() { dave.AcceptGroupChatInvite(iid) }()
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runErrChan := make(chan error)
	go func() { runErrChan <- alice.Run(ctx) }()
	go func() { runErrChan <- bob.Run(ctx) }()
	go func() { runErrChan <- charlie.Run(ctx) }()
	go func() { runErrChan <- dave.Run(ctx) }()

	time.Sleep(100 * time.Millisecond) // Time for run() to start.
	completeC2CKX(t, alice, bob)
	completeC2CKX(t, alice, charlie)
	completeC2CKX(t, alice, dave)

	// Invite everyone.
	var err error
	if gcID, err = alice.NewGroupChat(gcName); err != nil {
		t.Fatal(err)
	}
	if err := alice.InviteToGroupChat(gcID, bob.PublicID()); err != nil {
		t.Fatal(err)
	}
	if err := alice.InviteToGroupChat(gcID, charlie.PublicID()); err != nil {
		t.Fatal(err)
	}
	if err := alice.InviteToGroupChat(gcID, dave.PublicID()); err != nil {
		t.Fatal(err)
	}

	// We should get 3 events in Alice's join handler.
	for i := 0; i < 3; i++ {
		select {
		case <-aliceJoinChan:
		case <-time.After(10 * time.Second):
			t.Fatal("timeout")
		}
	}

	// Ensure all transitive KXs complete.
	assertClientsKXd(t, charlie, bob)
	assertClientsKXd(t, charlie, dave)
	assertClientsKXd(t, dave, bob)

	// Bob leaves. Charlie gets a kick event.
	reason := "because reasons"
	if err := bob.PartFromGC(gcID, reason); err != nil {
		t.Fatal(err)
	}

	select {
	case part := <-charliePartChan:
		if part.uid != bob.PublicID() {
			t.Fatalf("unexpected part uid. got %s, want %s",
				part.uid, bob.PublicID())
		}
		if part.reason != reason {
			t.Fatalf("unexpected reason. got %s, want %s", part.reason,
				reason)
		}
		if part.kicked {
			t.Fatalf("unexpected kicked. got %v, want false", part.kicked)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("timeout")
	}

	// Dave is kicked by alice. Charlie gets a kick event.
	reason = "because reasons 2"
	if err := alice.GCKick(gcID, dave.PublicID(), reason); err != nil {
		t.Fatal(err)
	}

	select {
	case part := <-charliePartChan:
		if part.uid != dave.PublicID() {
			t.Fatalf("unexpected part uid. got %s, want %s",
				part.uid, dave.PublicID())
		}
		if part.reason != reason {
			t.Fatalf("unexpected reason. got %s, want %s", part.reason,
				reason)
		}
		if !part.kicked {
			t.Fatalf("unexpected kicked. got %v, want true", part.kicked)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("timeout")
	}

	// Alice dissolves the GC. Dave gets a kill event.
	reason = "because reasons 3"
	if err := alice.KillGroupChat(gcID, reason); err != nil {
		t.Fatal(err)
	}

	select {
	case <-charlieKilledChan:
	case <-time.After(10 * time.Second):
		t.Fatal("timeout")
	}
}

// TestE2EDcrlnGCBlockList tests scenarios where a user blocks another in a
// specific GC.
func TestE2EDcrlnGCBlockList(t *testing.T) {
	rnd := testRand(t)
	alice := newTestClient(t, rnd, "alice")
	bob := newTestClient(t, rnd, "bob")
	charlie := newTestClient(t, rnd, "charlie")
	gcName := "testgc"
	var gcID zkidentity.ShortID

	bob.cfg.GCInviteHandler = func(user *RemoteUser, iid uint64, invite rpc.RMGroupInvite) {
		go func() { bob.AcceptGroupChatInvite(iid) }()
	}
	charlie.cfg.GCInviteHandler = func(user *RemoteUser, iid uint64, invite rpc.RMGroupInvite) {
		go func() { charlie.AcceptGroupChatInvite(iid) }()
	}

	bobMsgChan, aliceMsgChan, charlieMsgChan := make(chan string, 1), make(chan string, 1), make(chan string, 1)
	alice.cfg.Notifications.Register(OnGCMNtfn(func(ru *RemoteUser, msg rpc.RMGroupMessage, ts time.Time) {
		aliceMsgChan <- msg.Message
	}))
	bob.cfg.Notifications.Register(OnGCMNtfn(func(ru *RemoteUser, msg rpc.RMGroupMessage, ts time.Time) {
		bobMsgChan <- msg.Message
	}))
	charlie.cfg.Notifications.Register(OnGCMNtfn(func(ru *RemoteUser, msg rpc.RMGroupMessage, ts time.Time) {
		charlieMsgChan <- msg.Message
	}))
	bobGCListChan := make(chan struct{}, 2)
	bob.cfg.GCListUpdated = func(gc clientdb.GCAddressBookEntry) {
		bobGCListChan <- struct{}{}
	}

	// Helper to verify a message was received.
	assertMsgReceived := func(c chan string, wantMsg string) {
		t.Helper()
		select {
		case gotMsg := <-c:
			if gotMsg != wantMsg {
				t.Fatalf("unexpected msg: got %q, want %q",
					gotMsg, wantMsg)
			}
		case <-time.After(40 * time.Second):
			t.Fatal("timeout")
		}
	}

	// Helper to verify a message was _not_ received within a test
	// timeframe.
	assertMsgNotReceived := func(c chan string) {
		t.Helper()
		select {
		case gotMsg := <-c:
			t.Fatalf("unexpected message: %q", gotMsg)
		case <-time.After(time.Second):
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runErrChan := make(chan error, 2)
	go func() { runErrChan <- alice.Run(ctx) }()
	go func() { runErrChan <- bob.Run(ctx) }()
	go func() { runErrChan <- charlie.Run(ctx) }()
	time.Sleep(100 * time.Millisecond) // Time for run() to start.
	completeC2CKX(t, alice, bob)
	completeC2CKX(t, alice, charlie)
	completeC2CKX(t, bob, charlie)

	// Invite everyone.
	var err error
	if gcID, err = alice.NewGroupChat(gcName); err != nil {
		t.Fatal(err)
	}
	if err := alice.InviteToGroupChat(gcID, bob.PublicID()); err != nil {
		t.Fatal(err)
	}
	if err := alice.InviteToGroupChat(gcID, charlie.PublicID()); err != nil {
		t.Fatal(err)
	}

	// Bob should get 2 GCList events (one for him, one for charlie).
	for i := 0; i < 2; i++ {
		select {
		case <-bobGCListChan:
		case <-time.After(10 * time.Second):
			t.Fatal("timeout")
		}
	}

	// Bob blocks Charlie in the GC.
	if err := bob.AddToGCBlockList(gcID, charlie.PublicID()); err != nil {
		t.Fatal(err)
	}

	// Bob sends a message, Alice gets it, but charlie does not.
	bobMsg := "message from bob"
	if err := bob.GCMessage(gcID, bobMsg, 0, nil); err != nil {
		t.Fatal(err)
	}
	assertMsgReceived(aliceMsgChan, bobMsg)
	assertMsgNotReceived(charlieMsgChan)

	// Alice sends a message that both received.
	aliceMsg := "message from alice"
	if err := alice.GCMessage(gcID, aliceMsg, 0, nil); err != nil {
		t.Fatal(err)
	}
	assertMsgReceived(bobMsgChan, aliceMsg)
	assertMsgReceived(charlieMsgChan, aliceMsg)

	// Charlie sends a message that Alice receives but Bob does not.
	charlieMsg := "message from charlie"
	if err := charlie.GCMessage(gcID, charlieMsg, 0, nil); err != nil {
		t.Fatal(err)
	}
	assertMsgReceived(aliceMsgChan, charlieMsg)
	assertMsgNotReceived(bobMsgChan)
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

	// Send a tip from Alice to Bob.
	errChan := make(chan error)
	go func() { errChan <- alice.TipUser(bob.PublicID(), 0.00001) }()
	select {
	case err := <-errChan:
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("timeout")
	}
}

func TestE2EDcrlnReset(t *testing.T) {
	rnd := testRand(t)
	alice := newTestClient(t, rnd, "alice")
	bob := newTestClient(t, rnd, "bob")

	aliceKXdChan, bobKXdChan := make(chan struct{}), make(chan struct{})
	alice.cfg.KXCompleted = func(*RemoteUser) { go func() { aliceKXdChan <- struct{}{} }() }
	bob.cfg.KXCompleted = func(*RemoteUser) { go func() { bobKXdChan <- struct{}{} }() }
	alicePMChan, bobPMChan := make(chan string, 1), make(chan string, 1)
	alice.cfg.Notifications.Register(OnPMNtfn(func(ru *RemoteUser, pm rpc.RMPrivateMessage, ts time.Time) {
		alicePMChan <- pm.Message
	}))

	bob.cfg.Notifications.Register(OnPMNtfn(func(ru *RemoteUser, pm rpc.RMPrivateMessage, ts time.Time) {
		bobPMChan <- pm.Message
	}))

	// Helper to consume the KXCompleted events.
	assertKXCompleted := func() {
		t.Helper()
		select {
		case <-aliceKXdChan:
		case <-time.After(30 * time.Second):
			t.Fatal("timeout")
		}
		select {
		case <-bobKXdChan:
		case <-time.After(30 * time.Second):
			t.Fatal("timeout")
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = alice.Run(ctx) }()
	go func() { _ = bob.Run(ctx) }()

	time.Sleep(100 * time.Millisecond) // Time for run() to start.
	completeC2CKX(t, alice, bob)

	// Consume the two KXCompleted Events.
	assertKXCompleted()

	alice.log.Infof("================ kx completed")
	bob.log.Infof("================ kx completed")

	// Perform a reset.
	err := alice.ResetRatchet(bob.PublicID())
	if err != nil {
		t.Fatal(err)
	}

	// Ensure we got the new reset events.
	assertKXCompleted()

	// Reset on the other direction.
	err = bob.ResetRatchet(alice.PublicID())
	if err != nil {
		t.Fatal(err)
	}

	// Ensure we got the new reset events.
	assertKXCompleted()

	// Ensure Alice and Bob can message each other.
	aliceMsg, bobMsg := "i am alice", "i am bob"
	if err := alice.PM(bob.PublicID(), aliceMsg); err != nil {
		t.Fatal(err)
	}
	if err := bob.PM(alice.PublicID(), bobMsg); err != nil {
		t.Fatal(err)
	}
	select {
	case gotMsg := <-alicePMChan:
		if gotMsg != bobMsg { // Alice gets Bob's msg
			t.Fatalf("unexpected alice msg: got %q, want %q",
				gotMsg, aliceMsg)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("timeout")
	}
	select {
	case gotMsg := <-bobPMChan:
		if gotMsg != aliceMsg { // Bob gets Alice's msg
			t.Fatalf("unexpected bob msg: got %q, want %q",
				gotMsg, bobMsg)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("timeout")
	}
}

func TestE2EDcrlnTransReset(t *testing.T) {
	rnd := testRand(t)
	alice := newTestClient(t, rnd, "alice")
	bob := newTestClient(t, rnd, "bob")
	charlie := newTestClient(t, rnd, "charlie")

	aliceKXdChan, bobKXdChan := make(chan struct{}), make(chan struct{})
	alice.cfg.KXCompleted = func(*RemoteUser) { go func() { aliceKXdChan <- struct{}{} }() }
	bob.cfg.KXCompleted = func(*RemoteUser) { go func() { bobKXdChan <- struct{}{} }() }

	alicePMs, bobPMs := make(chan string), make(chan string)
	alice.cfg.Notifications.Register(OnPMNtfn(func(ru *RemoteUser, pm rpc.RMPrivateMessage, ts time.Time) {
		go func() { alicePMs <- pm.Message }()
	}))
	bob.cfg.Notifications.Register(OnPMNtfn(func(ru *RemoteUser, pm rpc.RMPrivateMessage, ts time.Time) {
		go func() { bobPMs <- pm.Message }()
	}))

	// Helper to consume the KXCompleted events.
	assertKXCompleted := func() {
		t.Helper()
		select {
		case <-aliceKXdChan:
		case <-time.After(30 * time.Second):
			t.Fatal("timeout")
		}
		select {
		case <-bobKXdChan:
		case <-time.After(30 * time.Second):
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
	defer cancel()
	go func() { _ = alice.Run(ctx) }()
	go func() { _ = bob.Run(ctx) }()
	go func() { _ = charlie.Run(ctx) }()
	time.Sleep(100 * time.Millisecond) // Time for run() to start.

	// Complete KXs between Alice->Bob, Alice->Charlie and Bob->Charlie
	completeC2CKX(t, alice, bob)
	completeC2CKX(t, alice, charlie)
	completeC2CKX(t, bob, charlie)

	// We should get two kx events on Alice and Bob.
	assertKXCompleted()
	assertKXCompleted()

	// Verify the Alice-Bob ratchet works.
	checkAliceBobRatchet()

	// Perform a transitive reset Alice -> Charlie -> Bob
	err := alice.RequestTransitiveReset(charlie.PublicID(), bob.PublicID())
	if err != nil {
		t.Fatal(err)
	}

	// We should get new KX completed events.
	assertKXCompleted()

	// Ensure Alice and Bob can message each other.
	checkAliceBobRatchet()
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

	aliceInviteChan, bobInviteChan := make(chan error), make(chan error)
	alice.cfg.GCInviteHandler = func(user *RemoteUser, iid uint64, invite rpc.RMGroupInvite) {
		go func() {
			aliceInviteChan <- alice.AcceptGroupChatInvite(iid)
		}()
	}
	bob.cfg.GCInviteHandler = func(user *RemoteUser, iid uint64, invite rpc.RMGroupInvite) {
		go func() {
			bobInviteChan <- bob.AcceptGroupChatInvite(iid)
		}()
	}
	aliceGCListChan, bobGCListChan := make(chan struct{}), make(chan struct{})
	alice.cfg.GCListUpdated = func(gc clientdb.GCAddressBookEntry) {
		go func() { aliceGCListChan <- struct{}{} }()
	}
	bob.cfg.GCListUpdated = func(gc clientdb.GCAddressBookEntry) {
		go func() { bobGCListChan <- struct{}{} }()
	}
	aliceGCMChan, bobGCMChan, charlieGCMChan := make(chan string), make(chan string), make(chan string)
	alice.cfg.Notifications.Register(OnGCMNtfn(func(user *RemoteUser, msg rpc.RMGroupMessage, ts time.Time) {
		go func() { aliceGCMChan <- msg.Message }()
	}))
	bob.cfg.Notifications.Register(OnGCMNtfn(func(user *RemoteUser, msg rpc.RMGroupMessage, ts time.Time) {
		go func() { bobGCMChan <- msg.Message }()
	}))
	charlie.cfg.Notifications.Register(OnGCMNtfn(func(user *RemoteUser, msg rpc.RMGroupMessage, ts time.Time) {
		go func() { charlieGCMChan <- msg.Message }()
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
		case <-time.After(10 * time.Second):
			if wantGCM != "" {
				t.Fatal("timeout")
			}
		}
	}

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

// TestE2EDcrlnKXSearch tests the KX search feature.
//
// The test plan is the following: create a chain of 5 KXd users (A-E). Create
// a post and relay across the chain. The last user (Eve) attempts to search
// for the original post author (Alice).
func TestE2EDcrlnKXSearch(t *testing.T) {
	rnd := testRand(t)
	alice := newTestClient(t, rnd, "alice")
	bob := newTestClient(t, rnd, "bob")
	charlie := newTestClient(t, rnd, "charlie")
	dave := newTestClient(t, rnd, "dave")
	eve := newTestClient(t, rnd, "eve")

	bobRecvPosts := make(chan clientdb.PostSummary, 1)
	bob.cfg.Notifications.Register(OnPostRcvdNtfn(func(ru *RemoteUser, summary clientdb.PostSummary, pm rpc.PostMetadata) {
		bobRecvPosts <- summary
	}))
	bobSubChanged := make(chan bool, 1)
	bob.cfg.Notifications.Register(OnRemoteSubscriptionChangedNtfn(func(ru *RemoteUser, subscribed bool) {
		bobSubChanged <- subscribed
	}))
	charlieRecvPosts := make(chan clientdb.PostSummary, 1)
	charlie.cfg.Notifications.Register(OnPostRcvdNtfn(func(ru *RemoteUser, summary clientdb.PostSummary, pm rpc.PostMetadata) {
		charlieRecvPosts <- summary
	}))
	charlieSubChanged := make(chan bool, 1)
	charlie.cfg.Notifications.Register(OnRemoteSubscriptionChangedNtfn(func(ru *RemoteUser, subscribed bool) {
		charlieSubChanged <- subscribed
	}))
	daveRecvPosts := make(chan clientdb.PostSummary, 1)
	dave.cfg.Notifications.Register(OnPostRcvdNtfn(func(ru *RemoteUser, summary clientdb.PostSummary, pm rpc.PostMetadata) {
		daveRecvPosts <- summary
	}))
	daveSubChanged := make(chan bool, 1)
	dave.cfg.Notifications.Register(OnRemoteSubscriptionChangedNtfn(func(ru *RemoteUser, subscribed bool) {
		daveSubChanged <- subscribed
	}))
	eveRecvPosts := make(chan clientdb.PostSummary, 2)
	eve.cfg.Notifications.Register(OnPostRcvdNtfn(func(ru *RemoteUser, summary clientdb.PostSummary, pm rpc.PostMetadata) {
		eveRecvPosts <- summary
	}))
	eveSubChanged := make(chan bool, 1)
	eve.cfg.Notifications.Register(OnRemoteSubscriptionChangedNtfn(func(ru *RemoteUser, subscribed bool) {
		eveSubChanged <- subscribed
	}))

	eveKXdChan := make(chan *RemoteUser, 1)
	eve.cfg.KXCompleted = func(ru *RemoteUser) { eveKXdChan <- ru }
	assertKXd := func(c chan *RemoteUser, to UserID) {
		t.Helper()
		select {
		case ru := <-c:
			if ru.ID() != to {
				t.Fatalf("unexpected kx counterparty: got %s, want %s",
					ru.ID(), to)
			}
		case <-time.After(30 * time.Second):
			t.Fatal("timeout")
		}
	}

	eveRecvUpdate := make(chan rpc.PostMetadataStatus, 10)
	eve.cfg.Notifications.Register(OnPostStatusRcvdNtfn(func(user *RemoteUser, pid clientintf.PostID,
		statusFrom UserID, status rpc.PostMetadataStatus) {
		eveRecvUpdate <- status
	}))
	assertComment := func(wantComment string) {
		t.Helper()
		select {
		case update := <-eveRecvUpdate:
			gotComment := update.Attributes[rpc.RMPSComment]
			if gotComment != wantComment {
				t.Fatalf("unexpected comment: got %q, want %q",
					gotComment, wantComment)
			}
		case <-time.After(30 * time.Second):
			t.Fatal("timeout")
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	runErr := make(chan error)
	defer func() {
		cancel()
		for i := 0; i < 5; i++ {
			err := <-runErr
			if !errors.Is(err, context.Canceled) {
				t.Logf("Fatal run err: %v", err)
			}
		}
	}()
	go func() { runErr <- alice.Run(ctx) }()
	go func() { runErr <- bob.Run(ctx) }()
	go func() { runErr <- charlie.Run(ctx) }()
	go func() { runErr <- dave.Run(ctx) }()
	go func() { runErr <- eve.Run(ctx) }()
	time.Sleep(100 * time.Millisecond) // Time for run() to start.

	// Create the chain of KXd users.
	completeC2CKX(t, alice, bob)
	completeC2CKX(t, bob, charlie)
	completeC2CKX(t, charlie, dave)
	completeC2CKX(t, dave, eve)
	assertClientsKXd(t, alice, bob)
	assertClientsKXd(t, bob, charlie)
	assertClientsKXd(t, charlie, dave)
	assertClientsKXd(t, dave, eve)

	// Consume Dave->Eve KX.
	assertKXd(eveKXdChan, dave.PublicID())

	// Make each one in the chain subscribe to the previous one's posts.
	subToPosts := func(poster, subber *Client, subberChan chan bool) {
		t.Helper()
		if err := subber.SubscribeToPosts(poster.PublicID()); err != nil {
			t.Fatal(err)
		}
		if !assert.ChanWritten(t, subberChan) {
			t.Fatal("subscriber failed to subscribe to poster")
		}
	}
	subToPosts(alice, bob, bobSubChanged)
	subToPosts(bob, charlie, charlieSubChanged)
	subToPosts(charlie, dave, daveSubChanged)
	subToPosts(dave, eve, eveSubChanged)

	// Alice creates a post and comments on it.
	postSumm, err := alice.CreatePost("first", "")
	if err != nil {
		t.Fatal(err)
	}
	post := postSumm.ID
	aliceComment := "alice comment"
	if err := alice.CommentPost(alice.PublicID(), post, aliceComment, nil); err != nil {
		t.Fatal(err)
	}

	// Each client will relay the post to the next one.
	relayPost := func(relayer, from, to *Client) {
		t.Helper()
		err := relayer.RelayPost(from.PublicID(), post, to.PublicID())
		if err != nil {
			t.Fatal(err)
		}
	}
	assertPostReceived := func(c chan clientdb.PostSummary, from UserID) {
		t.Helper()
		select {
		case summ := <-c:
			if summ.From != from {
				t.Fatalf("received post from unexpected ID: got %s,"+
					"want %s", summ.From, from)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("timeout")
		}
	}
	assertPostReceived(bobRecvPosts, alice.PublicID())
	relayPost(bob, alice, charlie)
	assertPostReceived(charlieRecvPosts, bob.PublicID())
	relayPost(charlie, bob, dave)
	assertPostReceived(daveRecvPosts, charlie.PublicID())
	relayPost(dave, charlie, eve)
	assertPostReceived(eveRecvPosts, dave.PublicID())

	// Eve will search for Alice (the post author).
	if err := eve.KXSearchPostAuthor(dave.PublicID(), post); err != nil {
		t.Fatal(err)
	}

	// Eve will KX with everyone up to Alice.
	assertKXd(eveKXdChan, charlie.PublicID())
	assertKXd(eveKXdChan, bob.PublicID())
	assertKXd(eveKXdChan, alice.PublicID())

	// Eve receives Alice's original comment.
	assertComment(aliceComment)

	// Bob comments on Alice's post. Eve should receive it.
	bobComment := "bob comment"
	if err := bob.CommentPost(alice.PublicID(), post, bobComment, nil); err != nil {
		t.Fatal(err)
	}
	assertComment(bobComment)
}
