package server

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/decred/slog"
	"github.com/jrick/logrotate/rotator"
)

type logBackend struct {
	stdOut          io.Writer
	logRotator      *rotator.Rotator
	bknd            *slog.Backend
	defaultLogLevel slog.Level
	logLevels       map[string]slog.Level
	loggers         map[string]slog.Logger
}

func newLogBackend(logFile, debugLevel string, stdOut io.Writer) (*logBackend, error) {

	var logRotator *rotator.Rotator
	if logFile != "" {
		logDir, _ := filepath.Split(logFile)
		err := os.MkdirAll(logDir, 0700)
		if err != nil {
			return nil, fmt.Errorf("failed to create log directory: %v", err)
		}
		logRotator, err = rotator.New(logFile, 1024, false, 10)
		if err != nil {
			return nil, fmt.Errorf("failed to create file rotator: %v", err)
		}
	}

	b := &logBackend{
		stdOut:          stdOut,
		logRotator:      logRotator,
		defaultLogLevel: slog.LevelInfo,
		logLevels:       make(map[string]slog.Level),
		loggers:         make(map[string]slog.Logger),
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
	if bknd.stdOut != nil {
		bknd.stdOut.Write(b)
	}
	if bknd.logRotator != nil {
		bknd.logRotator.Write(b)
	}

	return len(b), nil
}

// untrackedLogger is a logger that is not retained by the backend.
func (bknd *logBackend) untrackedLogger(subsys string) slog.Logger {
	l := bknd.bknd.Logger(subsys)
	if level, ok := bknd.logLevels[subsys]; ok {
		l.SetLevel(level)
	} else {
		l.SetLevel(bknd.defaultLogLevel)
	}
	return l
}

func (bknd *logBackend) logger(subsys string) slog.Logger {
	if l, ok := bknd.loggers[subsys]; ok {
		return l
	}

	l := bknd.bknd.Logger(subsys)
	bknd.loggers[subsys] = l
	if level, ok := bknd.logLevels[subsys]; ok {
		l.SetLevel(level)
	} else {
		l.SetLevel(bknd.defaultLogLevel)
	}

	return l
}
