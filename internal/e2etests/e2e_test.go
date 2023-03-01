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
	db      *clientdb.DB
	name    string
	id      *zkidentity.FullIdentity
	rootDir string
	ctx     context.Context
	cancel  func()
	runC    chan error

	mtx           sync.Mutex
	conn          *testConn
	preventConn   error
	onConnChanged func(connected bool, pushRate, subRate uint64)
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

// acceptNextGCInvite adds a handler that will accept the next GC invite
// received by the client as long as it is for the specified GC ID. It returns
// a chan that is written to when the invite is accepted.
func (tc *testClient) acceptNextGCInvite(gcID zkidentity.ShortID) chan error {
	c := make(chan error, 1)
	tc.handle(client.OnInvitedToGCNtfn(func(user *client.RemoteUser, iid uint64, invite rpc.RMGroupInvite) {
		if invite.ID != gcID {
			return
		}
		go func() {
			time.Sleep(5 * time.Millisecond)
			c <- tc.AcceptGroupChatInvite(iid)
		}()
	}))
	return c
}

// nextGCUserPartedIs returns a chan that gets written when the client receives
// a GC parted event for the given user.
func (tc *testClient) nextGCUserPartedIs(gcID client.GCID, uid client.UserID, kick bool) chan error {
	c := make(chan error, 1)
	tc.handle(client.OnGCUserPartedNtfn(func(gotGCID client.GCID, gotUID clientintf.UserID, reason string, gotKick bool) {
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

		c <- err
	}))
	return c
}

// handle is syntatic sugar for tc.NotificationManager().Register()
func (tc *testClient) handle(handler client.NotificationHandler) client.NotificationRegistration {
	return tc.NotificationManager().Register(handler)
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

	var logBknd func(subsys string) slog.Logger
	if ts.showLog {
		logBknd = testutils.TestLoggerBackend(ts.t, name)
	} else {
		logf, err := os.Create(filepath.Join(rootDir, "applog.log"))
		if err != nil {
			ts.t.Fatalf("unable to create log file: %v", err)
		}
		bknd := slog.NewBackend(logf)
		logBknd = func(subsys string) slog.Logger {
			logger := bknd.Logger(subsys)
			logger.SetLevel(slog.LevelTrace)
			return logger
		}
		ts.t.Cleanup(func() { logf.Close() })
	}
	dbLog := logBknd("FSDB")

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

		ServerSessionChanged: func(connected bool, pushRate, subRate, expDays uint64) {
			tc.mtx.Lock()
			f := tc.onConnChanged
			tc.mtx.Unlock()
			if f != nil {
				f(connected, pushRate, subRate)
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
		db:      db,
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

// stopClient stops this client. It can't be used after this.
func (ts *testScaffold) stopClient(tc *testClient) {
	ts.t.Helper()
	tc.cancel()
	err := assert.ChanWritten(ts.t, tc.runC)
	assert.ErrorIs(ts.t, err, context.Canceled)
}

// recreateClient stops the specified client and re-creates it using the same
// database.
func (ts *testScaffold) recreateClient(tc *testClient) *testClient {
	ts.t.Helper()

	// Stop existing client.
	ts.stopClient(tc)

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
