// Copyright (c) 2016-2020 Company 0, LLC.
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package server

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	_ "net/http/pprof"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/companyzero/bisonrelay/ratchet"
	"github.com/companyzero/bisonrelay/rpc"
	brfsdb "github.com/companyzero/bisonrelay/server/internal/fsdb"
	brpgdb "github.com/companyzero/bisonrelay/server/internal/pgdb"
	"github.com/companyzero/bisonrelay/server/serverdb"
	"github.com/companyzero/bisonrelay/server/settings"
	"github.com/companyzero/bisonrelay/session"
	"github.com/companyzero/bisonrelay/zkidentity"
	"github.com/davecgh/go-spew/spew"
	"github.com/decred/dcrlnd/lnrpc"
	"github.com/decred/dcrlnd/lnrpc/invoicesrpc"
	"github.com/decred/slog"
	"golang.org/x/sync/errgroup"
)

const (
	tagDepth = 32
)

// RPCWrapper is a wrapped RPC Message for internal use.  This is required because RPC messages
// consist of 2 discrete pieces.
type RPCWrapper struct {
	Message    rpc.Message
	Payload    interface{}
	Identifier string

	// CloseAfterWritingErr is set to a non nil error if the server session
	// should be closed after writing this message.
	CloseAfterWritingErr error
}

type ZKS struct {
	sync.Mutex
	now         func() time.Time
	listenAddrs []net.Addr // Actual addresses we're bound to

	// subscribers track which session is subscribed to which RVPoint.
	subscribers map[ratchet.RVPoint]*sessionContext

	// Not mutex entries
	db          serverdb.ServerDB
	settings    *settings.Settings
	id          *zkidentity.FullIdentity
	logBknd     *logBackend
	log         slog.Logger
	logConn     slog.Logger
	dbCtx       context.Context
	dbCtxCancel func()

	// pingLimit is the max time between pings.
	pingLimit time.Duration
	logPings  bool // Only set in some tests

	stats stats

	// Payment.
	lnRpc      lnrpc.LightningClient
	lnInvoices invoicesrpc.InvoicesClient
	lnNode     string
}

// BoundAddrs returns the addresses the server is bound to listen to.
func (z *ZKS) BoundAddrs() []net.Addr {
	z.Lock()
	res := append([]net.Addr{}, z.listenAddrs...)
	z.Unlock()
	return res
}

// unmarshal performs a limited json Unmarshal operation.
func (z *ZKS) unmarshal(dec *json.Decoder, v interface{}) error {
	return dec.Decode(&v)
}

// writeMessage marshals and sends encrypted message to client.
func (z *ZKS) writeMessage(kx *session.KX, msg *RPCWrapper) error {
	var bb bytes.Buffer

	enc := json.NewEncoder(&bb)
	err := enc.Encode(msg.Message)
	if err != nil {
		return fmt.Errorf("could not marshal message %v",
			msg.Message.Command)
	}
	err = enc.Encode(msg.Payload)
	if err != nil {
		return fmt.Errorf("could not marshal payload, %v",
			msg.Message.Command)
	}

	payload := bb.Bytes()
	err = kx.Write(payload)
	if err != nil {
		return fmt.Errorf("could not write %v: %v",
			msg.Message.Command, err)
	}
	z.stats.bytesSent.add(int64(len(payload)))

	return nil
}

func (z *ZKS) welcome(kx *session.KX) error {
	var err error
	properties := rpc.SupportedServerProperties
	for k, v := range properties {
		switch v.Key {
		case rpc.PropTagDepth:
			properties[k].Value = strconv.FormatUint(tagDepth, 10)
		case rpc.PropServerTime:
			properties[k].Value = strconv.FormatInt(time.Now().Unix(), 10)
		case rpc.PropPaymentScheme:
			properties[k].Value = z.settings.PayScheme
		case rpc.PropServerLNNode:
			properties[k].Value = z.lnNode
		case rpc.PropPushPaymentRate:
			properties[k].Value = strconv.FormatUint(z.settings.MilliAtomsPerByte, 10)
		case rpc.PropSubPaymentRate:
			properties[k].Value = strconv.FormatUint(z.settings.MilliAtomsPerSub, 10)
		case rpc.PropExpirationDays:
			properties[k].Value = strconv.FormatInt(int64(z.settings.ExpirationDays), 10)
		}
	}

	// Handle the new 'expirationdays' prop differently: add it if the
	// current setting is different than the default. This allows old
	// clients still to work while the prop is the old amount.
	if z.settings.ExpirationDays != rpc.PropExpirationDaysDefault {
		prop := rpc.DefaultPropExpirationDays
		prop.Value = strconv.Itoa(z.settings.ExpirationDays)
		properties = append(properties, prop)
	}

	// assemble command
	message := rpc.Message{
		Command: rpc.SessionCmdWelcome,
	}
	payload := rpc.Welcome{
		Version:    rpc.ProtocolVersion,
		Properties: properties,
	}

	// encode command
	var bb bytes.Buffer
	enc := json.NewEncoder(&bb)
	err = enc.Encode(message)
	if err != nil {
		return fmt.Errorf("could not marshal Welcome message")
	}
	err = enc.Encode(payload)
	if err != nil {
		return fmt.Errorf("could not marshal Welcome payload")
	}

	// write command over encrypted transport
	err = kx.Write(bb.Bytes())
	if err != nil {
		return fmt.Errorf("could not write Welcome message: %v", err)
	}

	return nil
}

func (z *ZKS) preSession(ctx context.Context, conn net.Conn) {
	z.log.Debugf("incoming connection: %v", conn.RemoteAddr())

	// Max time before we expect an InitialCmdSession and will drop the
	// connection if we don't receive one.
	initSessTimeout := z.settings.InitSessTimeout
	conn.SetReadDeadline(time.Now().Add(initSessTimeout))
	initSessDeadline := time.Now().Add(initSessTimeout)

	// Pre session state
	enc := json.NewEncoder(conn)
	dec := json.NewDecoder(conn)
	var mode string

	var err error

loop:
	for err == nil {
		err = dec.Decode(&mode)
		if err != nil {
			break loop
		}

		if time.Now().After(initSessDeadline) {
			err = fmt.Errorf("client did not start session before deadline: %v",
				conn.RemoteAddr())
			break loop
		}

		switch mode {
		case rpc.InitialCmdIdentify:
			z.log.Tracef("InitialCmdIdentify: %v", conn.RemoteAddr())
			err = enc.Encode(z.id.Public)
			if err != nil {
				err = fmt.Errorf("could not marshal "+
					"z.id.Public: %v",
					conn.RemoteAddr())
				break loop
			}

			z.log.Debugf("identifying self to: %v",
				conn.RemoteAddr())

		case rpc.InitialCmdSession:
			z.log.Tracef("InitialCmdSession: %v", conn.RemoteAddr())

			// go full session
			kx := new(session.KX)
			kx.Conn = conn
			kx.MaxMessageSize = rpc.MaxMsgSize
			kx.OurPublicKey = &z.id.Public.Key
			kx.OurPrivateKey = &z.id.PrivateKey
			err = kx.Respond()
			if err != nil {
				err = fmt.Errorf("kx.Respond: %v %v",
					conn.RemoteAddr(),
					err)
				break loop
			}

			// send welcome
			err = z.welcome(kx)
			if err != nil {
				err = fmt.Errorf("welcome failed: %v %v",
					conn.RemoteAddr(),
					err)
				break loop
			}

			// Move to full session.
			go z.runNewSession(ctx, conn, kx)
			return

		default:
			err = fmt.Errorf("invalid mode: %v: %v",
				conn.RemoteAddr(),
				mode)
			break loop
		}
	}

	// This is reached only if we error before moving on to a full session.
	conn.Close()
	z.log.Infof("Connection %v closed: %v", conn.RemoteAddr(), err)
}

func (z *ZKS) listen(ctx context.Context, l net.Listener) error {
	z.log.Debugf("Server Public ID: %v", spew.Sdump(z.id.Public))
	cert, err := tls.LoadX509KeyPair(filepath.Join(z.settings.Root,
		settings.ZKSCertFilename),
		filepath.Join(z.settings.Root, settings.ZKSKeyFilename))
	if err != nil {
		return fmt.Errorf("could not load certificates: %v", err)
	}
	config := tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
		},
	}

	z.log.Infof("Listening on %v", l.Addr())
	z.Lock()
	z.listenAddrs = []net.Addr{l.Addr()}
	z.Unlock()
	for {
		conn, err := l.Accept()
		if err != nil {
			return err
		}
		conn.(*net.TCPConn).SetKeepAlive(true)
		go z.preSession(ctx, tls.Server(conn, &config))
	}
}

// expirationLoop expires old messages from time to time.
func (z *ZKS) expirationLoop(ctx context.Context) error {
	const day = time.Hour * 24

	// Expire data older than this limit.
	expirationLimit := time.Duration(z.settings.ExpirationDays) * day
	if expirationLimit < day {
		return fmt.Errorf("expirationdays cannot be less than a day")
	}

	// Preemptively expire this number of dates from before the expiration
	// limit. This handles cases of old stale data when starting up, clock
	// changes and the computer having remained in hibernation.
	const nbPriorExpirations = 4

	for {
		now := z.now().UTC()
		expirationDate := now.Add(-expirationLimit)

		for i := nbPriorExpirations - 1; i >= 0; i-- {
			date := expirationDate.Add(-time.Duration(i) * day)

			z.log.Debugf("Attempting to expire data from %s",
				date.Format("2006-01-02"))
			count, err := z.db.Expire(ctx, date)
			if err != nil {
				return fmt.Errorf("unable to expire data from %s: %v",
					date.Format("2006-01-02"), err)
			}
			if count > 0 {
				z.log.Infof("Expired %d records from %s",
					count, date.Format("2006-01-02"))
			}
		}

		// Schedule expiration for the next day, UTC time.
		whenNextExpire := time.Date(now.Year(), now.Month(), now.Day()+1,
			0, 0, 0, 0, time.UTC)
		timeToNextExpire := whenNextExpire.Sub(now)
		z.log.Debugf("Scheduling next expiration for %s (%s from now)",
			whenNextExpire.Format(time.RFC3339), timeToNextExpire)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(timeToNextExpire):
		}
	}
}

func (z *ZKS) Run(ctx context.Context) error {
	defer z.log.Infof("End of times")

	l, err := net.Listen("tcp", z.settings.Listen)
	if err != nil {
		return fmt.Errorf("could not listen: %v", err)
	}

	g, gctx := errgroup.WithContext(ctx)

	// Cancel DB ops once we are commanded to stop.
	g.Go(func() error {
		<-gctx.Done()
		z.dbCtxCancel()
		return nil
	})

	// Cancel listening interfaces once context is done.
	g.Go(func() error {
		<-gctx.Done()
		return l.Close()
	})

	// Run the expiration loop.
	g.Go(func() error { return z.expirationLoop(ctx) })

	// Listen for connections.
	g.Go(func() error {
		err := z.listen(gctx, l)
		select {
		case <-ctx.Done():
			// Close() was requested, so ignore the error.
			return nil
		default:
			// Unexpected listen error.
			return err
		}
	})
	statLog := z.logBknd.logger("STAT")
	g.Go(func() error { return z.stats.runPrinter(gctx, statLog) })

	// Wait until all subsystems are done.
	err = g.Wait()

	// Close DB if needed.
	type dbcloser interface {
		Close() error
	}
	if db, ok := z.db.(dbcloser); ok {
		closeErr := db.Close()
		if closeErr != nil {
			z.log.Errorf("Error while closing DB: %v", closeErr)
		} else {
			z.log.Debugf("Closed database")
		}
	}

	return err
}

func NewServer(cfg *settings.Settings) (*ZKS, error) {
	logBknd, err := newLogBackend(cfg.LogFile, cfg.DebugLevel, cfg.LogStdOut)
	if err != nil {
		return nil, err
	}

	dbCtx, dbCtxCancel := context.WithCancel(context.Background())

	z := &ZKS{
		now:         time.Now,
		settings:    cfg,
		logBknd:     logBknd,
		log:         logBknd.logger("SERV"),
		logConn:     logBknd.logger("CONN"),
		subscribers: make(map[ratchet.RVPoint]*sessionContext),
		pingLimit:   rpc.PingLimit,
		dbCtx:       dbCtx,
		dbCtxCancel: dbCtxCancel,
	}

	z.log.Infof("Settings %v", spew.Sdump(z.settings))

	// Init db.
	if cfg.PGEnabled {
		opts := []brpgdb.Option{
			brpgdb.WithHost(cfg.PGHost),
			brpgdb.WithPort(cfg.PGPort),
			brpgdb.WithRole(cfg.PGRoleName),
			brpgdb.WithDBName(cfg.PGDBName),
			brpgdb.WithPassphrase(cfg.PGPassphrase),
			brpgdb.WithBulkDataTablespace(cfg.PGBulkTableSpace),
			brpgdb.WithIndexTablespace(cfg.PGIndexTableSpace),
		}
		if cfg.PGServerCA != "" {
			opts = append(opts, brpgdb.WithTLS(cfg.PGServerCA))
		}

		ctx := context.Background()
		z.db, err = brpgdb.Open(ctx, opts...)
		if err != nil {
			return nil, err
		}
		z.log.Infof("Initialized PG Database backend %s@%s:%s", cfg.PGDBName,
			cfg.PGHost, cfg.PGPort)
	} else {
		z.db, err = brfsdb.NewFSDB(z.settings.RoutedMessages, z.settings.PaidRVs)
		if err != nil {
			return nil, err
		}
		z.log.Infof("Initialized FileSystem Database backend")
	}

	// create paths
	err = os.MkdirAll(z.settings.Root, 0700)
	if err != nil {
		return nil, err
	}

	// print version
	z.log.Infof("%s version: %v, RPC Protocol: %v",
		filepath.Base(os.Args[0]), cfg.Versioner(), rpc.ProtocolVersion)

	// identity
	id, err := os.ReadFile(filepath.Join(z.settings.Root,
		settings.ZKSIdentityFilename))
	if err != nil {
		z.log.Infof("Creating a new identity")
		fid, err := zkidentity.New("brserver", "brserver")
		if err != nil {
			return nil, err
		}
		id, err = json.Marshal(fid)
		if err != nil {
			return nil, err
		}
		err = os.WriteFile(filepath.Join(z.settings.Root,
			settings.ZKSIdentityFilename), id, 0600)
		if err != nil {
			return nil, err
		}
	}
	err = json.Unmarshal(id, &z.id)
	if err != nil {
		return nil, err
	}

	// certs
	cert, err := tls.LoadX509KeyPair(filepath.Join(z.settings.Root,
		settings.ZKSCertFilename),
		filepath.Join(z.settings.Root, settings.ZKSKeyFilename))
	if err != nil {
		// create a new cert
		valid := time.Date(2049, 12, 31, 23, 59, 59, 0, time.UTC)
		cp, kp, err := newTLSCertPair("", valid, []string{})
		if err != nil {
			return nil, fmt.Errorf("could not create a new cert: %v",
				err)
		}

		// save on disk
		err = os.WriteFile(filepath.Join(z.settings.Root,
			settings.ZKSCertFilename), cp, 0600)
		if err != nil {
			return nil, fmt.Errorf("could not save cert: %v", err)
		}
		err = os.WriteFile(filepath.Join(z.settings.Root,
			settings.ZKSKeyFilename), kp, 0600)
		if err != nil {
			return nil, fmt.Errorf("could not save key: %v", err)
		}

		cert, err = tls.X509KeyPair(cp, kp)
		if err != nil {
			return nil, fmt.Errorf("X509KeyPair: %v", err)
		}
	}

	z.log.Infof("Start of day")
	z.log.Infof("Our outer fingerprint: %v", fingerprintDER(cert))
	z.log.Infof("Our inner fingerprint: %v", z.id.Public.Fingerprint())

	// Profiler
	if z.settings.Profiler != "" {
		z.log.Infof("Profiler enabled on http://%v/debug/pprof",
			z.settings.Profiler)
		go http.ListenAndServe(z.settings.Profiler, nil)
	}

	// Setup payment stuff (connect to dcrlnd, etc).
	err = z.initPayments()
	if err != nil {
		return nil, fmt.Errorf("unable to setup payment subsystem: %v", err)
	}

	return z, nil
}
