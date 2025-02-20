package e2etests

import (
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/companyzero/bisonrelay/client"
	"github.com/companyzero/bisonrelay/client/clientdb"
	"github.com/companyzero/bisonrelay/internal/assert"
	"github.com/companyzero/bisonrelay/internal/testutils"
	"github.com/companyzero/bisonrelay/ratchet"
	"github.com/companyzero/bisonrelay/rpc"
	"github.com/davecgh/go-spew/spew"
)

// TestFtDownloadFile verifies the behavior of downloading files from a remote
// user.
func TestFtDownloadFile(t *testing.T) {
	t.Parallel()

	// Setup Alice and Bob.
	tcfg := testScaffoldCfg{}
	ts := newTestScaffold(t, tcfg)
	alice := ts.newClient("alice")
	bob := ts.newClient("bob")
	ts.kxUsers(alice, bob)

	// Handlers.
	completedFileChan := make(chan string, 10)
	bob.handle(client.OnFileDownloadCompleted(func(user *client.RemoteUser, fm rpc.FileMetadata, diskPath string) {
		completedFileChan <- diskPath
	}))
	listedFilesChan := make(chan interface{}, 10)
	bob.handle(client.OnContentListReceived(func(user *client.RemoteUser, files []clientdb.RemoteFile, listErr error) {
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

	// Hooks to handle chunk payment.
	type hookedInvoice struct {
		amt int64
		cb  func(int64)
	}
	var invoicesMtx sync.Mutex
	var nbInvoices int
	invoices := map[string]hookedInvoice{}
	alice.mpc.HookGetInvoice(func(amt int64, cb func(int64)) (string, error) {
		invoicesMtx.Lock()
		id := fmt.Sprintf("hooked-inv-%03d", nbInvoices)
		nbInvoices++
		invoices[id] = hookedInvoice{amt: amt, cb: cb}
		invoicesMtx.Unlock()
		return id, nil
	})
	bob.mpc.HookPayInvoice(func(id string) (int64, error) {
		invoicesMtx.Lock()
		inv, ok := invoices[id]
		invoicesMtx.Unlock()
		if !ok {
			// Not a hooked invoice.
			return 0, nil
		}

		// Tell Alice that Bob paid the invoice.
		inv.cb(inv.amt)
		return inv.amt, nil
	})

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

	// Alice will share 2 files (one globally, one with Bob).
	fGlobal, fShared := testutils.RandomFile(t, defaultChunkSize*4), testutils.RandomFile(t, defaultChunkSize*4)
	sfGlobal, mdGlobal, err := alice.ShareFile(fGlobal, nil, 1, "global file")
	assert.NilErr(t, err)
	bobUID := bob.PublicID()
	_, mdShared, err := alice.ShareFile(fShared, &bobUID, 1, "user file")
	assert.NilErr(t, err)

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
	listResp := assert.ChanWritten(t, listedFilesChan)
	if _, ok := listResp.(error); !ok {
		t.Fatalf("unexpected result: %s", spew.Sdump(listResp))
	}

	// Bob asks for and receives Alice's global file.
	assert.NilErr(t, bob.GetUserContent(alice.PublicID(), sfGlobal.FID))
	completedPath1 := assert.ChanWritten(t, completedFileChan)
	assert.EqualFiles(t, fGlobal, completedPath1)
}

// TestFtSendFile tests that the send file feature works.
func TestFtSendFile(t *testing.T) {
	t.Parallel()

	// Setup Alice and Bob.
	tcfg := testScaffoldCfg{}
	ts := newTestScaffold(t, tcfg)
	alice := ts.newClient("alice")
	bob := ts.newClient("bob")
	ts.kxUsers(alice, bob)

	// Handlers.
	completedFileChan := make(chan string, 10)
	bob.handle(client.OnFileDownloadCompleted(func(user *client.RemoteUser, fm rpc.FileMetadata, diskPath string) {
		completedFileChan <- diskPath
	}))

	// Alice will send a file directly to bob.
	fSent := testutils.RandomFile(t, defaultChunkSize*4)
	assert.NilErr(t, alice.SendFile(bob.PublicID(), defaultChunkSize, fSent, nil))

	// Bob should receive it without having to do any payments.
	completedPath1 := assert.ChanWritten(t, completedFileChan)
	assert.EqualFiles(t, fSent, completedPath1)
}

// TestFtSendFileSenderRestarts tests restarting the sender of a file during
// the file sending process.
func TestFtSendFileSenderRestarts(t *testing.T) {
	t.Parallel()

	const nbTestChunks = 4

	// Setup Alice and Bob.
	tcfg := testScaffoldCfg{}
	ts := newTestScaffold(t, tcfg)
	alice := ts.newClient("alice")
	bob := ts.newClient("bob")
	ts.kxUsers(alice, bob)

	assertClientsCanPM(t, alice, bob)
	assertClientUpToDate(t, alice)
	assertClientUpToDate(t, bob)

	// Handlers.
	completedFileChan := make(chan string, 10)
	bob.handle(client.OnFileDownloadCompleted(func(user *client.RemoteUser, fm rpc.FileMetadata, diskPath string) {
		completedFileChan <- diskPath
	}))

	var testStage int
	testStageHitChan := make(chan struct{}, 5)
	testStageContChan := make(chan struct{}, 5)
	t.Cleanup(func() { close(testStageContChan) })
	alice.handleSync(client.OnRMSent(func(ru *client.RemoteUser, rv ratchet.RVPoint, p interface{}) {
		if _, ok := p.(rpc.RMFTGetChunkReply); testStage == 0 && ok {
			testStageHitChan <- struct{}{}
			<-testStageContChan
			testStage++
		}
	}))

	// Make the next message from Alice fail.
	err := fmt.Errorf("first fail")
	alice.preventFutureConns(err).startFailing(nil, err)

	// Alice will send a file directly to bob. This call will fail because
	// we'll stop it (by simulating a connection loss and client restart),
	// therefore run it in a goroutine.
	sendFileErrChan := make(chan error, 5)
	fSent := testutils.RandomFile(t, defaultChunkSize*nbTestChunks)
	go func() {
		sendFileErrChan <- alice.SendFile(bob.PublicID(), defaultChunkSize, fSent, nil)
	}()

	// Wait for chunks to be in the sendq and for the metadata to be in the
	// RMQ. We need this check to ensure we don't stop the client too soon,
	// before SendFile in the goroutine had a chance to start running.
	assertSendqDestsIs(t, alice, 4)
	assertRMQLenIs(t, alice, 1)
	ts.stopClient(alice)
	assert.ChanWritten(t, sendFileErrChan)

	// Setup the client config so that notifications are immediately hooked.
	alice.nccfg.ntfns = alice.NotificationManager()

	// Restart Alice. It will send the first chunk, but fail to send the
	// rest.
	alice = ts.recreateStoppedClient(alice)
	assert.ChanWritten(t, testStageHitChan)
	err = fmt.Errorf("second fail")
	alice.preventFutureConns(err).startFailing(nil, err)
	testStageContChan <- struct{}{}

	// Finally, restart Alice again. She will send the remaining chunks.
	alice = ts.recreateClient(alice)

	// Bob should receive it without having to do any payments.
	completedPath1 := assert.ChanWritten(t, completedFileChan)
	assert.EqualFiles(t, fSent, completedPath1)
}

// TestFtSendFileReceiverRestarts tests receiving a file through SendFile()
// while the receiver restarts its clients halfway through.
func TestFtSendFileReceiverRestarts(t *testing.T) {
	t.Parallel()

	const nbTestChunks = 4

	// Setup Alice and Bob.
	tcfg := testScaffoldCfg{}
	ts := newTestScaffold(t, tcfg)
	alice := ts.newClient("alice")
	bob := ts.newClient("bob")
	ts.kxUsers(alice, bob)

	// Handlers.
	completedFileChan := make(chan string, 10)
	bob.handle(client.OnFileDownloadCompleted(func(user *client.RemoteUser, fm rpc.FileMetadata, diskPath string) {
		completedFileChan <- diskPath
	}))

	var testStage int
	testStageHitChan := make(chan struct{}, 5)
	bob.handleSync(client.OnRMReceived(func(ru *client.RemoteUser, h *rpc.RMHeader, p interface{}, ts time.Time) {
		startFailing := false
		if _, ok := p.(rpc.RMFTSendFile); testStage == 0 && ok {
			// First fail after receiving RMFTSendFile.
			startFailing = true
		} else if _, ok := p.(rpc.RMFTGetChunkReply); testStage == 1 && ok {
			// Second fail after receiving the first chunk.
			startFailing = true
		}

		if startFailing {
			bob.log.Infof("Starting to fail test stage %d", testStage)
			err := fmt.Errorf("fail at stage %d", testStage)
			bob.preventFutureConns(err).startFailing(nil, err)
			testStage++
			testStageHitChan <- struct{}{}
		}
	}))

	// Alice sends the entire file successfully.
	fSent := testutils.RandomFile(t, defaultChunkSize*nbTestChunks)
	assert.NilErr(t, alice.SendFile(bob.PublicID(), defaultChunkSize, fSent, nil))

	// Bob stops after receiving file metadata.
	assert.ChanWritten(t, testStageHitChan)
	ts.stopClient(bob)
	assert.ChanNotWritten(t, completedFileChan, time.Second)

	// Setup the client config so that notifications are immediately hooked.
	bob.nccfg.ntfns = bob.NotificationManager()

	// Restart Bob. It will receive the first chunk, but no more.
	bob = ts.recreateStoppedClient(bob)
	assert.ChanWritten(t, testStageHitChan)

	// Finally, restart Bob again. He will receive all missing chunks.
	bob = ts.recreateClient(bob)
	completedPath1 := assert.ChanWritten(t, completedFileChan)
	assert.EqualFiles(t, fSent, completedPath1)
}
