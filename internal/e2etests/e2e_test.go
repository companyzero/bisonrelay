package e2etests

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"testing"
	"time"

	"github.com/companyzero/bisonrelay/client"
	"github.com/companyzero/bisonrelay/client/clientdb"
	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/companyzero/bisonrelay/client/resources"
	"github.com/companyzero/bisonrelay/internal/assert"
	"github.com/companyzero/bisonrelay/internal/testutils"
	"github.com/companyzero/bisonrelay/rpc"
	"github.com/companyzero/bisonrelay/server"
	"github.com/companyzero/bisonrelay/server/settings"
	"github.com/companyzero/bisonrelay/zkidentity"
	"github.com/decred/slog"
	"golang.org/x/exp/slices"
)

type testScaffoldCfg struct {
	skipNewServer bool
	logScanner    io.Writer
	rootDir       string
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

type loggerSubsysIniter func(subsys string) slog.Logger

// clientCfg holds config for an E2E test client.
type clientCfg struct {
	name      string
	rootDir   string
	id        *zkidentity.FullIdentity
	idIniter  func(context.Context) (*zkidentity.FullIdentity, error)
	netDialer func(context.Context) (clientintf.Conn, *tls.ConnectionState, error)
	pcIniter  func(loggerSubsysIniter) clientintf.PaymentClient
}

type newClientOpt func(*clientCfg)

func withPCIniter(pcIniter func(loggerSubsysIniter) clientintf.PaymentClient) newClientOpt {
	return func(cfg *clientCfg) {
		cfg.pcIniter = pcIniter
	}
}

func withSimnetEnvDcrlndPayClient(t testing.TB, alt bool) newClientOpt {
	pcIniter := func(logBknd loggerSubsysIniter) clientintf.PaymentClient {
		t.Helper()
		homeDir, err := os.UserHomeDir()
		assert.NilErr(t, err)

		dcrlndDir := filepath.Join(homeDir, "dcrlndsimnetnodes", "dcrlnd2")
		dcrlndAddr := "localhost:20200"
		if alt {
			dcrlndDir = filepath.Join(homeDir, "dcrlndsimnetnodes", "dcrlnd1")
			dcrlndAddr = "localhost:20100"
		}

		pc, err := client.NewDcrlndPaymentClient(context.Background(), client.DcrlnPaymentClientCfg{
			TLSCertPath: filepath.Join(dcrlndDir, "tls.cert"),
			MacaroonPath: filepath.Join(dcrlndDir, "chain", "decred", "simnet",
				"admin.macaroon"),
			Address: dcrlndAddr,
			Log:     logBknd("LNPY"),
		})
		assert.NilErr(t, err)
		return pc
	}
	return withPCIniter(pcIniter)
}

type testClient struct {
	*client.Client
	db      *clientdb.DB
	name    string
	rootDir string
	ctx     context.Context
	cancel  func()
	runC    chan error
	mpc     *testutils.MockPayClient
	nccfg   *clientCfg
	cfg     *client.Config
	log     slog.Logger

	mtx               sync.Mutex
	conn              *testConn
	preventConn       error
	resourcesProvider resources.Provider
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

// handleSync is syntatic sugar for tc.NotificationManager().RegisterSync()
func (tc *testClient) handleSync(handler client.NotificationHandler) client.NotificationRegistration {
	return tc.NotificationManager().RegisterSync(handler)
}

// waitTippingSubsysRunning waits until the tipping subsystem is running tipping
// attempts (plus an additional time for its actions to possibly complete).
func (tc *testClient) waitTippingSubsysRunning(extra time.Duration) {
	tc.ListRunningTipUserAttempts()

	// Sleep additional time for actions to start.
	time.Sleep(extra)
}

// testInterface returns a filled unsafe test interface for this client.
func (tc *testClient) testInterface() *testutils.UnsafeTestInterface {
	i := &testutils.UnsafeTestInterface{}
	tc.FillTestInterface(i)
	return i
}

// testLogLineScanner looks for the regexp on every write to the log and
// collects the matches.
type testLogLineScanner struct {
	mtx   sync.Mutex
	re    regexp.Regexp
	found []string
}

func (scanner *testLogLineScanner) Write(b []byte) (int, error) {
	if scanner.re.Match(b) {
		b := slices.Clone(b)
		scanner.mtx.Lock()
		scanner.found = append(scanner.found, string(b))
		scanner.mtx.Unlock()
	}
	return len(b), nil
}

func (scanner *testLogLineScanner) hasMatches() bool {
	scanner.mtx.Lock()
	res := len(scanner.found) > 0
	scanner.mtx.Unlock()
	return res
}

// testScaffold holds all scaffolding needed to run an E2E test that involves
// an instance of a BR server and client.
type testScaffold struct {
	t       testing.TB
	cfg     testScaffoldCfg
	showLog bool
	tlb     *testutils.TestLogBackend
	log     slog.Logger

	ctx    context.Context
	cancel func()

	svr     *server.ZKS
	svrAddr string
}

func (ts *testScaffold) defaultNewClientCfg(name string) *clientCfg {
	rootDir, err := os.MkdirTemp(ts.cfg.rootDir, "br-client-"+name+"-*")
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
	return &clientCfg{
		name:    name,
		rootDir: rootDir,
		id:      id,
		pcIniter: func(loggerSubsysIniter) clientintf.PaymentClient {
			return &testutils.MockPayClient{}
		},
		netDialer: clientintf.NetDialer(ts.svrAddr, slog.Disabled),
		idIniter: func(context.Context) (*zkidentity.FullIdentity, error) {
			c := new(zkidentity.FullIdentity)
			*c = *id
			return c, nil
		},
	}
}

func (ts *testScaffold) newClientWithCfg(nccfg *clientCfg) *testClient {
	ts.t.Helper()

	name := nccfg.name
	rootDir := nccfg.rootDir

	if name == "" {
		ts.t.Fatal("name cannot be empty")
	}

	logf, err := os.Create(filepath.Join(rootDir, "applog.log"))
	if err != nil {
		ts.t.Fatalf("unable to create log file: %v", err)
	}
	ts.t.Cleanup(func() { logf.Close() })
	logBknd := ts.tlb.NamedSubLogger(name, logf)
	dbLog := logBknd("FSDB")

	var tc *testClient

	// Intercepting dialer: this allows arbitrarily breaking the connection
	// between the client and server.
	dialer := func(ctx context.Context) (clientintf.Conn, *tls.ConnectionState, error) {
		netConn, tlsState, err := nccfg.netDialer(ctx)
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

	pc := nccfg.pcIniter(logBknd)
	mpc, _ := pc.(*testutils.MockPayClient)

	cfg := client.Config{
		ReconnectDelay: 500 * time.Millisecond,
		Dialer:         dialer,
		CertConfirmer: func(context.Context, *tls.ConnectionState,
			*zkidentity.PublicIdentity) error {
			return nil
		},
		DB:            db,
		LocalIDIniter: nccfg.idIniter,
		Logger:        logBknd,
		PayClient:     pc,

		TipUserRestartDelay:          2 * time.Second,
		TipUserReRequestInvoiceDelay: time.Second,
		TipUserMaxLifetime:           20 * time.Second,
		TipUserPayRetryDelayFactor:   100 * time.Millisecond,

		GCMQUpdtDelay:    100 * time.Millisecond,
		GCMQMaxLifetime:  time.Second,
		GCMQInitialDelay: time.Second,

		RecentMediateIDThreshold:   time.Second,
		UnkxdWarningTimeout:        time.Millisecond * 250,
		MaxAutoKXMediateIDRequests: 3,

		ResourcesProvider: resources.ProviderFunc(func(ctx context.Context,
			uid clientintf.UserID,
			request *rpc.RMFetchResource) (*rpc.RMFetchResourceReply, error) {

			tc.mtx.Lock()
			rp := tc.resourcesProvider
			tc.mtx.Unlock()
			if rp == nil {
				return nil, fmt.Errorf("test client was not setup with resources provider")

			}

			return rp.Fulfill(ctx, uid, request)
		}),
	}
	c, err := client.New(cfg)
	assert.NilErr(ts.t, err)

	ctx, cancel := context.WithCancel(ts.ctx)
	tc = &testClient{
		name:    name,
		Client:  c,
		ctx:     ctx,
		cancel:  cancel,
		rootDir: rootDir,
		runC:    make(chan error, 1),
		db:      db,
		mpc:     mpc,
		cfg:     &cfg,
		nccfg:   nccfg,
		log:     logBknd("TEST"),
	}
	go func() { tc.runC <- c.Run(ctx) }()

	// Wait until address book is loaded.
	tc.AddressBook()
	tc.log.Infof("Test client ready for use as instance %p", tc)

	return tc
}

// newClient instantiates a new client that can connect to the scaffold's
// server. This MUST be called only from the main test goroutine.
func (ts *testScaffold) newClient(name string, opts ...newClientOpt) *testClient {
	ts.t.Helper()

	nccfg := ts.defaultNewClientCfg(name)
	for _, opt := range opts {
		opt(nccfg)
	}
	return ts.newClientWithCfg(nccfg)
}

// stopClient stops this client. It can't be used after this.
func (ts *testScaffold) stopClient(tc *testClient) {
	ts.t.Helper()
	time.Sleep(10 * time.Millisecond) // Wait for any final queued ops before stop.
	tc.cancel()
	err := assert.ChanWritten(ts.t, tc.runC)
	assert.ErrorIs(ts.t, err, context.Canceled)
	tc.log.Infof("Test client shut down instance %p", tc)
}

// recreateClient stops the specified client and re-creates it using the same
// database.
func (ts *testScaffold) recreateClient(tc *testClient) *testClient {
	ts.t.Helper()

	// Stop existing client.
	ts.stopClient(tc)

	// Recreate client.
	return ts.newClientWithCfg(tc.nccfg)
}

// recreateStoppedClient recreates a client that was previously stopped. If
// the client was not stopped, results are undefined.
func (ts *testScaffold) recreateStoppedClient(tc *testClient) *testClient {
	ts.t.Helper()
	return ts.newClientWithCfg(tc.nccfg)
}

// kxUsers performs a kx between the two users with an additional gc invite.
// This MUST be called only from the main test goroutine.
func (ts *testScaffold) kxUsersWithInvite(inviter, invitee *testClient, gcID zkidentity.ShortID) {
	ts.t.Helper()
	invite, err := inviter.WriteNewInvite(io.Discard, nil)
	assert.NilErr(ts.t, err)
	assert.NilErr(ts.t, inviter.AddInviteOnKX(invite.InitialRendezvous, gcID))
	assert.NilErr(ts.t, invitee.AcceptInvite(invite))
	assertClientsKXd(ts.t, inviter, invitee)
}

// kxUsers performs a kx between the two users, so that they can communicate
// with each other. This MUST be called only from the main test goroutine.
func (ts *testScaffold) kxUsers(inviter, invitee *testClient) {
	ts.t.Helper()
	invite, err := inviter.WriteNewInvite(io.Discard, nil)
	assert.NilErr(ts.t, err)
	assert.NilErr(ts.t, invitee.AcceptInvite(invite))
	assertClientsKXd(ts.t, inviter, invitee)
}

func (ts *testScaffold) newTestServer() {
	t := ts.t
	t.Helper()

	cfg := settings.New()
	dir, err := os.MkdirTemp(ts.cfg.rootDir, "br-server-*")
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
	cfg.LogStdOut = ts.tlb

	s, err := server.NewServer(cfg)
	if err != nil {
		t.Fatal(err)
	}
	ts.svr = s

	// Run the server.
	go ts.svr.Run(ts.ctx)
}

func newTestScaffold(t *testing.T, cfg testScaffoldCfg) *testScaffold {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	logEnv := os.Getenv("BR_E2E_LOG")
	showLog := logEnv == "1" || logEnv == t.Name()

	if cfg.rootDir == "" {
		rootDir, err := os.MkdirTemp("", "br-e2etest-*")
		assert.NilErr(t, err)
		cfg.rootDir = rootDir
		t.Cleanup(func() {
			if t.Failed() {
				t.Logf("Root test dir: %s", rootDir)
			} else {
				os.RemoveAll(rootDir)
			}
		})
	}

	tlb := testutils.NewTestLogBackend(t, testutils.WithShowLog(showLog),
		testutils.WithMiddlewareWriter(cfg.logScanner))
	log := tlb.NamedSubLogger("XXXXXXX", nil)("XXXX")

	ts := &testScaffold{
		t:       t,
		tlb:     tlb,
		cfg:     cfg,
		ctx:     ctx,
		cancel:  cancel,
		showLog: showLog,
		log:     log,
	}

	if !cfg.skipNewServer {
		ts.newTestServer()

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
	}

	return ts
}
