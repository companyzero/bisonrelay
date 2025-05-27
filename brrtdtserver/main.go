package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	_ "net/http/pprof"
	"net/netip"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/companyzero/bisonrelay/brrtdtserver/internal/version"
	rtdtserver "github.com/companyzero/bisonrelay/rtdt/server"
	"github.com/companyzero/bisonrelay/zkidentity"
	"github.com/decred/slog"
	"github.com/jrick/logrotate/rotator"
)

// minKernelBufferSize is used as the minimum size for kernel buffers when
// initializing sockets.
const minKernelBufferSize = 1024 * 1024

func realMain() error {
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
	log.Infof("Running brrtdtserver version %s", version.String())

	// Main context.
	errMainCtxCanceled := errors.New("main context canceled")
	sigCtx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	ctx, mainCancel := context.WithCancelCause(context.Background())
	go func() {
		<-sigCtx.Done()
		log.Infof("Interrupt detected. Shutting down server.")
		mainCancel(errMainCtxCanceled)
	}()

	// Profiler.
	if cfg.Profiler != "" {
		log.Infof("Profiler enabled on http://%v/debug/pprof",
			cfg.Profiler)
		go http.ListenAndServe(cfg.Profiler, nil)
	}

	// Listeners.
	var listeners []*net.UDPConn
	for _, addr := range cfg.Listen {
		addrPort, err := netip.ParseAddrPort(addr)
		if err != nil {
			return fmt.Errorf("unable to parse addr '%s': %v", addr, err)
		}

		udpAddr := net.UDPAddrFromAddrPort(addrPort)

		// Double check we can bind to it.
		socket, err := net.ListenUDP("udp", udpAddr)
		if err != nil {
			return fmt.Errorf("unable to bind to address %s: %v", addr, err)
		}

		if err := checkKernelUDPBufferSize(socket, cfg.IgnoreSmallKernelBuffers, log); err != nil {
			return err
		}

		listeners = append(listeners, socket)
	}

	// Server options.
	opts := []rtdtserver.Option{
		rtdtserver.WithLogger(log),
		rtdtserver.WithListeners(listeners...),
		rtdtserver.WithPrometheusListenAddr(cfg.ListenPrometheus),
		rtdtserver.WithPerListenerReadRoutines(cfg.ReadRoutines),
		rtdtserver.WithReportStatsInterval(cfg.StatsInterval),
	}

	if cfg.PrivateKeyFile != "" {
		pk, err := loadPrivateKey(cfg.PrivateKeyFile, log)
		if err != nil {
			return err
		}
		opts = append(opts, rtdtserver.WithPrivateKey(pk))
	} else {
		log.Warnf("RUNNING WITHOUT TRANSPORT-LEVEL ENCRYPTION")
	}

	if cfg.CookieKey != "" {
		var key zkidentity.FixedSizeSymmetricKey
		if err := key.FromString(cfg.CookieKey); err != nil {
			return err
		}

		var decodeKeys []*zkidentity.FixedSizeSymmetricKey
		for _, decodeKey := range cfg.DecodeCookieKeys {
			key := new(zkidentity.FixedSizeSymmetricKey)
			if err := key.FromString(decodeKey); err != nil {
				return err
			}
			decodeKeys = append(decodeKeys, key)
		}

		opts = append(opts, rtdtserver.WithCookieKey(&key, decodeKeys))
	} else {
		log.Warnf("RUNNING WITHOUT JOIN COOKIE VALIDATION")
	}

	if cfg.LogErrors {
		opts = append(opts, rtdtserver.WithLogErrors())
		log.Warn("Logging remotely-generated errors")
	}

	// Server.
	server, err := rtdtserver.New(opts...)
	if err != nil {
		return err
	}

	err = server.Run(ctx)
	if errors.Is(err, context.Canceled) && context.Cause(ctx) == errMainCtxCanceled {
		// Ignore graceful shutdown error.
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
