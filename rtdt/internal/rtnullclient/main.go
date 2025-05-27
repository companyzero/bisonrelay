package main

// Test app ("null client") that generates random bursts of data to the RTDT
// server.

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/companyzero/bisonrelay/rpc"
	"github.com/companyzero/bisonrelay/zkidentity"
	"github.com/decred/slog"
	"github.com/jrick/logrotate/rotator"
)

func realMain() error {
	// Main context.
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Wait for termination signals.
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		cancel()
	}()

	// Settings.
	cfg, err := obtainSettings()
	if err != nil {
		return err
	}

	// Log.
	logBackend := &logBackend{
		stdOut: os.Stdout,
	}
	if cfg.LogFile != "" {
		logDir := filepath.Dir(cfg.LogFile)
		err := os.MkdirAll(logDir, 0700)
		if err != nil {
			return fmt.Errorf("failed to create log directory: %w", err)
		}
		logRotator, err := rotator.New(cfg.LogFile, 1024, false, maxLogFiles)
		if err != nil {
			return fmt.Errorf("failed to create file rotator: %w", err)
		}
		logBackend.logRotator = logRotator
	}
	logBknd := slog.NewBackend(logBackend)
	log := logBknd.Logger("RTDT")
	logLevel, ok := slog.LevelFromString(cfg.DebugLevel)
	if !ok {
		return fmt.Errorf("unknown log level %q", cfg.DebugLevel)
	}
	log.SetLevel(logLevel)

	// Profiler
	if cfg.Profile != "" {
		profileRedirect := http.RedirectHandler("/debug/pprof", http.StatusSeeOther)
		http.Handle("/", profileRedirect)
		go func() {
			log.Infof("Starting profile server on %s", cfg.Profile)
			err := http.ListenAndServe(cfg.Profile, nil)
			if err != nil {
				log.Errorf("Unable to run profiler: %v", err)
			}
		}()
	}

	serverAddr, err := net.ResolveUDPAddr("udp", cfg.ServerAddr)
	if err != nil {
		return err
	}

	var serverPubKey *zkidentity.FixedSizeSntrupPublicKey
	if cfg.ServerPubKey != "" {
		pubBytes, err := os.ReadFile(cfg.ServerPubKey)
		if err != nil {
			return err
		}
		serverPubKey = new(zkidentity.FixedSizeSntrupPublicKey)
		copy(serverPubKey[:], pubBytes)
	}

	log.Infof("Starting RTDT null client version %s", version())

	ccfg := clientCfg{
		serverAddr:   serverAddr,
		serverPubKey: serverPubKey,
		readRoutines: cfg.ReadRoutines,
		cookieKey:    cfg.CookieKey,
		basePeerID:   rpc.RTDTPeerID(cfg.BasePeerID & 0x00ff),
		log:          log,
	}

	sigKey, sigPubKey, publisherKey := e2eKeysForBasePeerID(ccfg.basePeerID)
	log.Debugf("Generated E2E Keys")
	if cfg.EnableE2E {
		ccfg.publisherKey = publisherKey
		log.Debugf("Initialized demo peer E2E key %x", ccfg.publisherKey[:])
	}
	if cfg.EnableE2EAuth {
		ccfg.sigKey = sigKey
		log.Debugf("Initialized demo peer E2E Auth with pubkey %x", sigPubKey[:])
	}

	c, err := newClient(ctx, ccfg)
	if err != nil {
		return err
	}

	log.Infof("Listening for admin commands on %s", cfg.Listen)
	server := &http.Server{Addr: cfg.Listen, Handler: c.handler()}
	go func() {
		<-ctx.Done()
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		server.Shutdown(ctx)
	}()
	err = server.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		log.Infof("Finished running server")
		return nil
	}
	return err
}

func main() {
	err := realMain()
	if err != nil {
		fmt.Println("Error:", err.Error())
		os.Exit(1)
	}
}
