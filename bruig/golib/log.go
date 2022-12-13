package golib

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/decred/slog"
	"github.com/jrick/logrotate/rotator"
)

type logBackend struct {
	logRotator      *rotator.Rotator
	bknd            *slog.Backend
	defaultLogLevel slog.Level
	logLevels       map[string]slog.Level
	notify          bool
}

func newLogBackend(logFile, debugLevel string) (*logBackend, error) {
	var logRotator *rotator.Rotator
	if logFile != "" {
		logDir, _ := filepath.Split(logFile)
		err := os.MkdirAll(logDir, 0700)
		if err != nil {
			return nil, fmt.Errorf("failed to create log directory: %v\n", err)
		}
		logRotator, err = rotator.New(logFile, 1024*1024, false, 3)
		if err != nil {
			return nil, fmt.Errorf("failed to create file rotator: %v\n", err)
		}
	}

	b := &logBackend{
		logRotator:      logRotator,
		defaultLogLevel: slog.LevelInfo,
		logLevels:       make(map[string]slog.Level),
	}
	b.bknd = slog.NewBackend(b)

	// Parse the debugLevel string into log levels for each subsystem.
	for _, v := range strings.Split(debugLevel, ",") {
		fields := strings.Split(v, "=")
		if len(fields) == 1 {
			b.defaultLogLevel, _ = slog.LevelFromString(fields[0])
		} else if len(fields) == 2 {
			subsys := fields[0]
			level, _ := slog.LevelFromString(fields[1])
			b.logLevels[subsys] = level
		} else {
			return nil, fmt.Errorf("unable to parse %q as subsys=level "+
				"debuglevel string", v)
		}
	}

	return b, nil
}

func (bknd *logBackend) Write(b []byte) (int, error) {
	os.Stdout.Write(b)
	if bknd.logRotator != nil {
		bknd.logRotator.Write(b)
	}
	if bknd.notify {
		go func() { notify(NTLogLine, string(b), nil) }()
	}
	return len(b), nil
}

func (bknd *logBackend) logger(subsys string) slog.Logger {
	l := bknd.bknd.Logger(subsys)
	if level, ok := bknd.logLevels[subsys]; ok {
		l.SetLevel(level)
	} else {
		l.SetLevel(bknd.defaultLogLevel)
	}

	return l
}
