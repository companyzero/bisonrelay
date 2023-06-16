package embeddeddcrlnd

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/decred/dcrlnd"
	"github.com/decred/dcrlnd/lncfg"
	"github.com/decred/dcrlnd/lnrpc"
	"github.com/decred/dcrlnd/lnrpc/autopilotrpc"
	"github.com/decred/dcrlnd/lnrpc/chainrpc"
	"github.com/decred/dcrlnd/lnrpc/initchainsyncrpc"
	"github.com/decred/dcrlnd/lnrpc/invoicesrpc"
	"github.com/decred/dcrlnd/lnrpc/walletrpc"
	"github.com/decred/dcrlnd/lnrpc/watchtowerrpc"
	"github.com/decred/dcrlnd/lnrpc/wtclientrpc"
	"github.com/decred/dcrlnd/macaroons"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
	"gopkg.in/macaroon.v2"
)

// Config are the config parameters of the dcrlnd instance.
type Config struct {
	// RootDir is the root data dir where dcrlnd data is stored.
	RootDir string

	// Network is one of the supported dcrlnd networks.
	Network string

	// DebugLevel is the logging level to use.
	DebugLevel string

	// MaxLogFiles is the max number of log files to keep around.
	MaxLogFiles int

	// RPCAddresses are addresses to bind gRPC to. When non-empty, the
	// first address MUST be one accessible for the local host (e.g.
	// 127.0.0.1:<port>), otherwise initialization may hang forever.
	RPCAddresses []string

	// DialFunc can be set to specify a non standard dialer.
	DialFunc func(context.Context, string, string) (net.Conn, error)

	// TorAddr is the host:port the Tor's SOCKS5 proxy is listening on.
	TorAddr string

	// TorIsolation enables Tor stream isolation.
	TorIsolation bool
}

// Dcrlnd is a running instance of an embedded dcrlnd instance.
type Dcrlnd struct {
	runErr       error
	runDone      chan struct{}
	rpcAddr      string
	shutdownChan chan struct{}
	connOpts     []grpc.DialOption
	conn         *grpc.ClientConn
	macaroonPath string
	tlsCertPath  string

	mtx      sync.Mutex
	unlocked bool
}

var ErrLNWalletNotFound = errors.New("wallet not found")

// RPCAddr returns the address of the gRPC endpoint of the running dcrlnd
// instance.
func (lndc *Dcrlnd) RPCAddr() string {
	return lndc.rpcAddr
}

// TLSCertPath returns the path to the TLS cert of the dcrlnd instance.
func (lndc *Dcrlnd) TLSCertPath() string {
	return lndc.tlsCertPath
}

// MacaroonPath returns the path to the macaroon file of the dcrlnd instance.
func (lndc *Dcrlnd) MacaroonPath() string {
	return lndc.macaroonPath
}

// TryUnlock attempts to unlock the wallet with the given passphrase.
func (lndc *Dcrlnd) TryUnlock(ctx context.Context, pass string) error {
	lnUnlocker := lnrpc.NewWalletUnlockerClient(lndc.conn)
	uwr := lnrpc.UnlockWalletRequest{
		WalletPassword: []byte(pass),
	}
	_, err := lnUnlocker.UnlockWallet(ctx, &uwr)
	if err != nil && strings.Contains(err.Error(), "wallet not found") {
		return ErrLNWalletNotFound
	}
	if err != nil {
		return err
	}

	// In case of successful unlock, we'll re-create the connection so
	// that this call only returns once the next RPC service is running.
	//
	// This helps to simplify the logic for tracking the chain sync state.
	time.Sleep(time.Second)
	if err := lndc.reconnect(ctx); err != nil {
		return fmt.Errorf("unable to reconnect after unlock: %v", err)
	}

	lndc.mtx.Lock()
	lndc.unlocked = true
	lndc.mtx.Unlock()
	return nil
}

func (lndc *Dcrlnd) reconnect(ctx context.Context) error {
	var err error
	rpcAddr := lndc.rpcAddr
	lndc.conn, err = grpc.DialContext(ctx, rpcAddr, append(lndc.connOpts, grpc.WithBlock())...)
	return err
}

// Create attempts to create a new wallet using a new seed and protects the
// wallet with the given passphrase. The seed for the wallet is returned.
func (lndc *Dcrlnd) Create(ctx context.Context, pass string, existingSeed []string,
	multiChanBackup []byte) ([]byte, error) {

	lnUnlocker := lnrpc.NewWalletUnlockerClient(lndc.conn)

	var seedMnemonic []string
	if len(existingSeed) == 0 {
		genSeedRes, err := lnUnlocker.GenSeed(ctx, &lnrpc.GenSeedRequest{})
		if err != nil {
			return nil, err
		}
		seedMnemonic = genSeedRes.CipherSeedMnemonic
	} else {
		seedMnemonic = existingSeed
	}

	initReq := &lnrpc.InitWalletRequest{
		WalletPassword:     []byte(pass),
		CipherSeedMnemonic: seedMnemonic,
	}
	if len(multiChanBackup) > 0 {
		initReq.ChannelBackups = &lnrpc.ChanBackupSnapshot{
			MultiChanBackup: &lnrpc.MultiChanBackup{
				MultiChanBackup: multiChanBackup,
			},
		}
	}
	_, err := lnUnlocker.InitWallet(ctx, initReq)
	if err != nil {
		return nil, err
	}

	lndc.mtx.Lock()
	lndc.unlocked = true
	lndc.mtx.Unlock()

	// Wait until the macaroon file is created.
	for {
		macBytes, err := os.ReadFile(lndc.macaroonPath)
		if err == nil {
			mac := &macaroon.Macaroon{}
			if err = mac.UnmarshalBinary(macBytes); err == nil {
				// Recreate the conn, now using the macaroon file.
				opt := grpc.WithPerRPCCredentials(macaroons.NewMacaroonCredential(mac))
				lndc.connOpts = append(lndc.connOpts, opt)
				if err := lndc.reconnect(ctx); err != nil {
					return nil, fmt.Errorf("unable to reconnect after create: %v", err)
				}
				break
			}
		}

		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("macaroon file was not created")
		case <-time.After(time.Millisecond * 200):
		}
	}

	var bb bytes.Buffer
	for i, word := range seedMnemonic {
		bb.Write([]byte(word))
		if i < len(seedMnemonic) {
			bb.Write([]byte(" "))
		}
	}
	return bb.Bytes(), nil
}

type ChainSyncNotifier func(*initchainsyncrpc.ChainSyncUpdate, error)

// NotifyInitialChainSync calls the especified notifier while the chain is
// syncing. The notifier will be called at least once, either with a message
// with Synced = true or with an error.
func (lndc *Dcrlnd) NotifyInitialChainSync(ctx context.Context, ntf ChainSyncNotifier) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		select {
		case <-ctx.Done():
		case <-lndc.shutdownChan:
			cancel()
		}
	}()

	// Wait until either the SubscribeChainSync call succeeds or the
	// GetInfo call succeeds. Either of those mean the sync has completed.

	lnInitSync := initchainsyncrpc.NewInitialChainSyncClient(lndc.conn)
	lnRPC := lnrpc.NewLightningClient(lndc.conn)
	reqSub := &initchainsyncrpc.ChainSyncSubscription{}
	var recv *initchainsyncrpc.ChainSyncUpdate
	var stream initchainsyncrpc.InitialChainSync_SubscribeChainSyncClient
	for {
		// Try subscribing to the chain sync stream.
		var err error
		if stream == nil {
			stream, err = lnInitSync.SubscribeChainSync(ctx, reqSub)
		}

		if err == nil {
			// Try to fetch an update.
			recv, err = stream.Recv()
			if err == nil {
				ntf(recv, nil)
				if recv.Synced {
					// All done!
					return
				}

				// Keep looping.
				continue
			}
		}

		// At this point, we have an error. Decide what to do.
		//
		// If err != unimplemented, it's a fatal error, so
		// notify and return.
		if (status.Code(err) != codes.Unimplemented) && !errors.Is(err, io.EOF) {
			err = fmt.Errorf("unable to query initChainSync service: %v", err)
			ntf(nil, err)
			return
		}

		// An unimplemented error means the initChainSync service isn't
		// online. Try to query via the standard rpc.
		res, err := lnRPC.GetInfo(ctx, &lnrpc.GetInfoRequest{})
		if err == nil {
			// RPC service is online, so chain sync is completed.
			bh, _ := hex.DecodeString(res.BlockHash)
			recv = &initchainsyncrpc.ChainSyncUpdate{
				BlockHeight:    int64(res.BlockHeight),
				BlockHash:      bh,
				BlockTimestamp: res.BestHeaderTimestamp,
				Synced:         true,
			}
			ntf(recv, nil)
			return
		}

		// An error != unimplemented is a fatal error.
		if err != nil && status.Code(err) != codes.Unimplemented {
			err = fmt.Errorf("unable to query rpc service: %v", err)
			ntf(nil, err)
			return
		}

		// An unimplemented error on both services means the startup
		// hasn't finished yet. Wait for a bit, then try again.
		select {
		case <-time.After(time.Second):
		case <-ctx.Done():
			return
		}
	}
}

// Wait blocks until this process is done or the passed context is canceled.
func (lndc *Dcrlnd) Wait(ctx context.Context) error {
	lndc.mtx.Lock()
	unlocked := lndc.unlocked
	lndc.mtx.Unlock()

	if !unlocked {
		// Return early because dcrlnd.Main() does not return when
		// it's waiting for a password.
		return nil
	}

	select {
	case <-ctx.Done():
		return fmt.Errorf("wait context error: %v", ctx.Err())
	case <-lndc.runDone:
		return lndc.runErr
	}
}

// Stop stops the running dcrlnd instance.
func (lndc *Dcrlnd) Stop() {
	select {
	case <-lndc.runDone:
		return
	case <-lndc.shutdownChan:
		return
	default:
		close(lndc.shutdownChan)
	}
}

// RunDcrlnd initializes and runs a new embedded dcrlnd instance. It returns
// with the (locked) instance if no errors are found.
//
// The passed context is only used during the attempts to connect to the
// running node.
func RunDcrlnd(ctx context.Context, cfg Config) (*Dcrlnd, error) {
	var rpcAddrs []string
	if len(cfg.RPCAddresses) > 0 {
		rpcAddrs = cfg.RPCAddresses
	} else {
		port, err := findAvailablePort()
		if err != nil {
			return nil, err
		}
		rpcAddrs = []string{fmt.Sprintf("127.0.0.1:%d", port)}
	}
	conf := dcrlnd.DefaultConfig()

	rootDir := cfg.RootDir
	network := cfg.Network

	rpcAddr := rpcAddrs[0]
	conf.LndDir = rootDir
	conf.DataDir = filepath.Join(rootDir, "data")
	conf.ConfigFile = filepath.Join(rootDir, "dcrlnd.conf")
	conf.LogDir = filepath.Join(rootDir, "logs")
	conf.MaxLogFiles = cfg.MaxLogFiles
	conf.TLSCertPath = filepath.Join(rootDir, "tls.cert")
	conf.TLSKeyPath = filepath.Join(rootDir, "tls.key")
	conf.TLSDisableAutofill = true // FIXME: parametrize
	conf.RawRPCListeners = rpcAddrs
	conf.DisableRest = true
	conf.DisableListen = true
	conf.BackupFilePath = filepath.Join(rootDir, "channels.backup")
	conf.Node = "dcrw"
	conf.DebugLevel = cfg.DebugLevel
	conf.ProtocolOptions = &lncfg.ProtocolOptions{}
	conf.WtClient = &lncfg.WtClient{}
	conf.SubRPCServers.WalletKitRPC = &walletrpc.Config{}
	conf.SubRPCServers.AutopilotRPC = &autopilotrpc.Config{}
	conf.SubRPCServers.ChainRPC = &chainrpc.Config{}
	conf.SubRPCServers.InvoicesRPC = &invoicesrpc.Config{}
	conf.SubRPCServers.WatchtowerRPC = &watchtowerrpc.Config{}
	conf.SubRPCServers.WatchtowerClientRPC = &wtclientrpc.Config{}
	conf.Dcrwallet = &lncfg.DcrwalletConfig{
		SPV:      true,
		DialFunc: cfg.DialFunc,
	}
	if cfg.TorAddr != "" {
		if _, _, err := net.SplitHostPort(cfg.TorAddr); err != nil {
			return nil, err
		}
		conf.Tor = &lncfg.Tor{
			Active:          true,
			SOCKS:           cfg.TorAddr,
			StreamIsolation: cfg.TorIsolation,
		}
	}
	switch network {
	case "mainnet":
		// Default network.
	case "testnet":
		conf.TestNet3 = true
	case "simnet":
		conf.SimNet = true

		// In the case of simnet, add SPV Connect addresses to the
		// standard simnet dcrd node and the standard dcrlnd 3 node
		// setup. This is needed so that the simnet node can actually
		// sync.
		conf.Dcrwallet.SPVConnect = []string{"127.0.0.1:19555"}
	default:
		return nil, fmt.Errorf("unrecognized network %q", network)
	}

	validConf, err := dcrlnd.ValidateConfig(conf, "")
	if err != nil {
		return nil, fmt.Errorf("error validating dcrlnd conf: %v", err)
	}

	lndc := &Dcrlnd{
		runDone:      make(chan struct{}),
		shutdownChan: make(chan struct{}),
		rpcAddr:      rpcAddr,
		macaroonPath: filepath.Join(conf.DataDir, "chain", "decred",
			network, "admin.macaroon"),
		tlsCertPath: conf.TLSCertPath,
	}
	go func() {
		err := dcrlnd.Main(
			validConf, dcrlnd.ListenerCfg{}, lndc.shutdownChan,
		)
		if err != nil {
			err = fmt.Errorf("dcrlnd.Main error: %v", err)
		}
		lndc.runErr = err
		close(lndc.runDone)
	}()

	// Cleanup function in case of errors throughout the rest of this
	// function. This is deferred in a closure because cleanup is
	// overwritten further down the function.
	cleanup := func() {
		close(lndc.shutdownChan)
	}
	defer func() { cleanup() }()

	// Try to connect to dcrlnd (note this is done without a macaroon file
	// here).
	tlsCtx, cancel := context.WithTimeout(ctx, time.Second*60)
	defer cancel()

	// Wait until the tls file is created.
	var creds credentials.TransportCredentials
	for {
		creds, err = credentials.NewClientTLSFromFile(conf.TLSCertPath, "")
		if err == nil {
			break
		}
		select {
		case <-tlsCtx.Done():
			return nil, fmt.Errorf("unable to read cert file: %v", err)
		case <-time.After(time.Millisecond * 200):
		}
	}
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(creds),
	}

	// If the macaroon file exists, use it.
	macBytes, err := os.ReadFile(lndc.macaroonPath)
	if err == nil {
		mac := &macaroon.Macaroon{}
		if err = mac.UnmarshalBinary(macBytes); err != nil {
			return nil, fmt.Errorf("unable to read macaroon file: %v", err)
		}
		opts = append(opts, grpc.WithPerRPCCredentials(macaroons.NewMacaroonCredential(mac)))
	}
	lndc.connOpts = opts

	if err := lndc.reconnect(ctx); err != nil {
		return nil, fmt.Errorf("unable to connect to ln wallet: %v", err)
	}

	// Initialization succeeded, clear cleanup.
	cleanup = func() {}

	return lndc, nil
}
