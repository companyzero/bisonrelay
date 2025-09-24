package main

import (
	"fmt"
	"net"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/decred/slog"
	"github.com/jrick/logrotate/rotator"
)

type config struct {
	Listen string

	Tokens []string
}

var defaultHomeDir = AppDataDir("brseeder", false)

func loadConfig() (*config, error) {
	err := os.MkdirAll(defaultHomeDir, 0o700)
	if err != nil {
		return nil, err
	}
	cfgBytes, err := os.ReadFile(filepath.Join(defaultHomeDir, "brseeder.conf"))
	if err != nil {
		return nil, err
	}

	var cfg config
	err = toml.Unmarshal(cfgBytes, &cfg)
	if err != nil {
		return nil, err
	}
	if _, _, err = net.SplitHostPort(cfg.Listen); err != nil {
		return nil, fmt.Errorf("invalid listen address: %w", err)
	}

	if len(cfg.Tokens) == 0 {
		return nil, fmt.Errorf("no tokens specified")
	}
	return &cfg, nil
}

type logWriter struct {
	r *rotator.Rotator
}

func (l *logWriter) Write(p []byte) (n int, err error) {
	os.Stdout.Write(p)
	return l.r.Write(p)
}

func initLog() (slog.Logger, error) {
	logDir := filepath.Join(defaultHomeDir, "logs")
	if err := os.MkdirAll(logDir, 0o700); err != nil {
		return slog.Disabled, fmt.Errorf("failed to create %v: %v", logDir, err)
	}
	logPath := filepath.Join(logDir, "brseeder.log")
	logFd, err := rotator.New(logPath, 32*1024, true, 0)
	if err != nil {
		return slog.Disabled, fmt.Errorf("Failed to setup logfile %s: %v", logPath, err)
	}
	defer logFd.Close()

	bknd := slog.NewBackend(&logWriter{logFd}, slog.WithFlags(slog.LUTC))
	logger := bknd.Logger("BRSEED")

	return logger, nil
}
