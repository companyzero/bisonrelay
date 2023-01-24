package e2etests

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/companyzero/bisonrelay/client"
	"github.com/companyzero/bisonrelay/client/clientdb"
	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/companyzero/bisonrelay/internal/assert"
	"github.com/companyzero/bisonrelay/internal/testutils"
	"github.com/companyzero/bisonrelay/rpc"
	"github.com/companyzero/bisonrelay/server"
	"github.com/companyzero/bisonrelay/server/settings"
	"github.com/companyzero/bisonrelay/zkidentity"
	"github.com/decred/slog"
)

type testScaffoldCfg struct {
	showLog bool
}

type testConn struct {
	sync.Mutex
	netConn   clientintf.Conn
	failRead  error
	failWrite error
}

// startFailing starts failing Read() and Write() calls with the specified
// errors.
func (tc *testConn) startFailing(readErr, writeErr error) {
	tc.Lock()
	tc.failRead = readErr
	tc.failWrite = writeErr
	tc.Unlock()
}

func (tc *testConn) Read(p []byte) (int, error) {
	tc.Lock()
	err := tc.failRead
	tc.Unlock()
	if err != nil {
		return 0, err
	}
	return tc.netConn.Read(p)
}

func (tc *testConn) Write(p []byte) (int, error) {
	tc.Lock()
	err := tc.failWrite
	tc.Unlock()
	if err != nil {
		return 0, err
	}
	return tc.netConn.Write(p)

}
func (tc *testConn) Close() error {
	return tc.netConn.Close()
}

func (tc *testConn) RemoteAddr() net.Addr {
	return tc.netConn.RemoteAddr()
}

type testClient struct {
	*client.Client
	name    string
	id      *zkidentity.FullIdentity
	rootDir string
	ctx     context.Context
	cancel  func()
	runC    chan error

	mtx             sync.Mutex
	conn            *testConn
	preventConn     error
	onKXCompleted   func(user *client.RemoteUser)
	onConnChanged   func(connected bool, pushRate, subRate uint64)
	onInvitedToGC   func(user *client.RemoteUser, iid uint64, invite rpc.RMGroupInvite)
	onGCListUpdated func(gc clientdb.GCAddressBookEntry)
	onGCUserParted  func(gcid client.GCID, uid clientintf.UserID, reason string, kicked bool)
}

// modifyHandlers calls f with the mutex held, so that the client handlers can
// be freely modified.
func (tc *testClient) modifyHandlers(f func()) {
	tc.mtx.Lock()
	f()
	tc.mtx.Unlock()
}

// preventFutureConns stops all future conns of this client from succeeding.
func (tc *testClient) preventFutureConns(err error) {
	tc.mtx.Lock()
	tc.preventConn = err
	tc.mtx.Unlock()
}

// acceptNextGCInvite replaces the onInvitedToGC handler with one that will
// accept the next GC invite received by the client as long as it is for the
// specified GC ID. It returns a chan that is written to when the invite is
// accepted.
func (tc *testClient) acceptNextGCInvite(gcID zkidentity.ShortID) chan error {
	c := make(chan error, 1)
	tc.mtx.Lock()
	tc.onInvitedToGC = func(user *client.RemoteUser, iid uint64, invite rpc.RMGroupInvite) {
		if invite.ID != gcID {
			return
		}
		tc.mtx.Lock()
		tc.onInvitedToGC = nil
		tc.mtx.Unlock()

		go func() {
			time.Sleep(5 * time.Millisecond)
			c <- tc.AcceptGroupChatInvite(iid)
		}()
	}
	tc.mtx.Unlock()
	return c
}

// nextGCUserPartedIs returns a chan that gets written when the next GC user
// parted even is the specified one.
func (tc *testClient) nextGCUserPartedIs(gcID client.GCID, uid client.UserID, kick bool) chan error {
	c := make(chan error, 1)
	tc.mtx.Lock()
	tc.onGCUserParted = func(gotGCID client.GCID, gotUID clientintf.UserID, reason string, gotKick bool) {
		var err error
		if err == nil && gotGCID != gcID {
			err = fmt.Errorf("unexpected GCID: got %s, want %s",
				gotGCID, gcID)
		}
		if err == nil && uid != gotUID {
			err = fmt.Errorf("unexpected UID: got %s, want %s",
				gotUID, uid)
		}
		if err == nil && kick != kick {
			err = fmt.Errorf("unexpected kick: got %v, want %v",
				gotKick, kick)

		}

		tc.mtx.Lock()
		tc.onGCUserParted = nil
		tc.mtx.Unlock()

		c <- err
	}
	tc.mtx.Unlock()
	return c
}

// testScaffold holds all scaffolding needed to run an E2E test that involves
// an instance of a BR server and client.
type testScaffold struct {
	t       testing.TB
	cfg     testScaffoldCfg
	showLog bool

	ctx    context.Context
	cancel func()

	svr     *server.ZKS
	svrRunC chan error
	svrAddr string
}

func (ts *testScaffold) newClientWithOpts(name string, rootDir string,
	id *zkidentity.FullIdentity) *testClient {

	ts.t.Helper()
	if name == "" {
		ts.t.Fatal("name cannot be empty")
	}

	dbLog := slog.Disabled
	logBknd := func(subsys string) slog.Logger { return slog.Disabled }
	if ts.showLog {
		logBknd = testutils.TestLoggerBackend(ts.t, name)
		dbLog = logBknd("FSDB")
	}

	idIniter := func(context.Context) (*zkidentity.FullIdentity, error) {
		c := new(zkidentity.FullIdentity)
		*c = *id
		return c, nil
	}

	var tc *testClient

	// Intercepting dialer: this allows arbitrarily breaking the connection
	// between the client and server.
	netDialer := clientintf.NetDialer(ts.svrAddr, slog.Disabled)
	dialer := func(ctx context.Context) (clientintf.Conn, *tls.ConnectionState, error) {
		netConn, tlsState, err := netDialer(ctx)
		if err != nil {
			return nil, nil, err
		}
		conn := &testConn{netConn: netConn}
		tc.mtx.Lock()
		err = tc.preventConn
		if err == nil {
			tc.conn = conn
		}
		tc.mtx.Unlock()
		return conn, tlsState, err
	}

	dbCfg := clientdb.Config{
		Root:          rootDir,
		DownloadsRoot: filepath.Join(rootDir, "downloads"),
		Logger:        dbLog,
		ChunkSize:     8,
	}
	db, err := clientdb.New(dbCfg)
	assert.NilErr(ts.t, err)

	cfg := client.Config{
		ReconnectDelay: 500 * time.Millisecond,
		Dialer:         dialer,
		CertConfirmer: func(context.Context, *tls.ConnectionState,
			*zkidentity.PublicIdentity) error {
			return nil
		},
		DB:            db,
		LocalIDIniter: idIniter,
		Logger:        logBknd,

		KXCompleted: func(user *client.RemoteUser) {
			tc.mtx.Lock()
			f := tc.onKXCompleted
			tc.mtx.Unlock()
			if f != nil {
				f(user)
			}
		},

		ServerSessionChanged: func(connected bool, pushRate, subRate, expDays uint64) {
			tc.mtx.Lock()
			f := tc.onConnChanged
			tc.mtx.Unlock()
			if f != nil {
				f(connected, pushRate, subRate)
			}
		},

		GCInviteHandler: func(user *client.RemoteUser, iid uint64, invite rpc.RMGroupInvite) {
			tc.mtx.Lock()
			f := tc.onInvitedToGC
			tc.mtx.Unlock()
			if f != nil {
				f(user, iid, invite)
			}
		},

		GCListUpdated: func(gc clientdb.GCAddressBookEntry) {
			tc.mtx.Lock()
			f := tc.onGCListUpdated
			tc.mtx.Unlock()
			if f != nil {
				f(gc)
			}
		},

		GCUserParted: func(gcid client.GCID, uid client.UserID, reason string, kicked bool) {
			tc.mtx.Lock()
			f := tc.onGCUserParted
			tc.mtx.Unlock()
			if f != nil {
				f(gcid, uid, reason, kicked)
			}
		},
	}
	c, err := client.New(cfg)
	assert.NilErr(ts.t, err)

	ctx, cancel := context.WithCancel(ts.ctx)
	tc = &testClient{
		name:    name,
		Client:  c,
		ctx:     ctx,
		cancel:  cancel,
		id:      id,
		rootDir: rootDir,
		runC:    make(chan error, 1),
	}
	go func() { tc.runC <- c.Run(ctx) }()

	// Wait until address book is loaded.
	tc.AddressBook()

	return tc
}

// newClient instantiates a new client that can connect to the scaffold's
// server. This MUST be called only from the main test goroutine.
func (ts *testScaffold) newClient(name string) *testClient {
	ts.t.Helper()

	rootDir, err := os.MkdirTemp("", "br-client-"+name+"-*")
	assert.NilErr(ts.t, err)
	ts.t.Cleanup(func() {
		if ts.t.Failed() {
			ts.t.Logf("%s DB dir: %s", name, rootDir)
		} else {
			os.RemoveAll(rootDir)
		}
	})

	id, err := zkidentity.New(name, name)
	assert.NilErr(ts.t, err)
	return ts.newClientWithOpts(name, rootDir, id)
}

// recreateClient stops the specified client and re-creates it using the same
// database.
func (ts *testScaffold) recreateClient(tc *testClient) *testClient {
	ts.t.Helper()

	// Stop existing client.
	tc.cancel()
	err := assert.ChanWritten(ts.t, tc.runC)
	assert.ErrorIs(ts.t, err, context.Canceled)

	// Recreate client.
	return ts.newClientWithOpts(tc.name, tc.rootDir, tc.id)
}

// kxUsers performs a kx between the two users, so that they can communicate
// with each other. This MUST be called only from the main test goroutine.
func (ts *testScaffold) kxUsers(inviter, invitee *testClient) {
	ts.t.Helper()
	invite, err := inviter.WriteNewInvite(io.Discard)
	assert.NilErr(ts.t, err)
	assert.NilErr(ts.t, invitee.AcceptInvite(invite))
	assertClientsKXd(ts.t, inviter, invitee)
}

func (ts *testScaffold) run() {
	// Run the server.
	go func() {
		ts.svrRunC <- ts.svr.Run(ts.ctx)
	}()
}

func newTestServer(t testing.TB, showLog bool) *server.ZKS {
	t.Helper()

	cfg := settings.New()
	dir, err := os.MkdirTemp("", "br-server")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if t.Failed() {
			t.Logf("Server data location: %s", dir)
		} else {
			os.RemoveAll(dir)
		}
	})

	cfg.Root = dir
	cfg.RoutedMessages = filepath.Join(dir, settings.ZKSRoutedMessages)
	cfg.LogFile = filepath.Join(dir, "brserver.log")
	cfg.Listen = []string{"127.0.0.1:0"}
	cfg.InitSessTimeout = time.Second
	cfg.DebugLevel = "debug"
	if showLog {
		cfg.LogStdOut = testutils.NewTestLogBackend(t)
	} else {
		cfg.LogStdOut = nil
	}

	s, err := server.NewServer(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func newTestScaffold(t *testing.T, cfg testScaffoldCfg) *testScaffold {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	ts := &testScaffold{
		t:       t,
		cfg:     cfg,
		ctx:     ctx,
		cancel:  cancel,
		svr:     newTestServer(t, cfg.showLog),
		svrRunC: make(chan error, 1),
		showLog: cfg.showLog,
	}
	go ts.run()

	// Figure out the actual server address.
	for i := 0; i <= 100; i++ {
		addrs := ts.svr.BoundAddrs()
		if len(addrs) == 0 {
			if i == 100 {
				ts.t.Fatal("Timeout waiting for server address")
				return ts
			}
			time.Sleep(10 * time.Millisecond)
			continue
		}
		ts.svrAddr = addrs[0].String()
	}

	return ts
}
