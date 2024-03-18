package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/companyzero/bisonrelay/brclient/internal/sloglinesbuffer"
	"github.com/decred/slog"
	"github.com/jrick/logrotate/rotator"
)

// errMsgRE is a regexp that matches error log msgs.
var errMsgRE = regexp.MustCompile(`^\d{4}-\d\d-\d\d \d\d:\d\d:\d\d\.\d{3} \[ERR] `)

var internalLog = slog.Disabled

type logBackend struct {
	logRotator      *rotator.Rotator
	bknd            *slog.Backend
	defaultLogLevel slog.Level
	logLevels       map[string]slog.Level

	loggersMtx sync.Mutex
	loggers    map[string]slog.Logger

	logCb    func(string)
	errorMsg func(string)
	logLines *sloglinesbuffer.Buffer
}

func newLogBackend(logCb func(string), errMsg func(string),
	logFile, debugLevel string, maxLogFiles int) (*logBackend, error) {

	var logRotator *rotator.Rotator
	if logFile != "" {
		logDir, _ := filepath.Split(logFile)
		err := os.MkdirAll(logDir, 0700)
		if err != nil {
			return nil, fmt.Errorf("failed to create log directory: %w", err)
		}
		logRotator, err = rotator.New(logFile, 1024, false, maxLogFiles)
		if err != nil {
			return nil, fmt.Errorf("failed to create file rotator: %w", err)
		}
	}

	b := &logBackend{
		logRotator:      logRotator,
		defaultLogLevel: slog.LevelInfo,
		logLevels:       make(map[string]slog.Level),
		logLines:        new(sloglinesbuffer.Buffer),
		logCb:           logCb,
		errorMsg:        errMsg,
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
	//os.Stdout.Write(b)
	if bknd.logRotator != nil {
		bknd.logRotator.Write(b)
	}

	// Add to in-memory list of last log lines.
	if n, err := bknd.logLines.Write(b); err != nil {
		return n, err
	}

	if bknd.logCb != nil {
		line := string(b)
		bknd.logCb(line)
	}
	if bknd.errorMsg != nil && errMsgRE.Match(b) {
		line := string(b[24:])
		bknd.errorMsg(line)
	}

	return len(b), nil
}

func (bknd *logBackend) logger(subsys string) slog.Logger {
	bknd.loggersMtx.Lock()
	defer bknd.loggersMtx.Unlock()

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

func (bknd *logBackend) setLogLevel(s string) error {
	if s == "" {
		return nil
	}

	fields := strings.Split(s, "=")
	if len(fields) == 1 {
		var ok bool
		bknd.defaultLogLevel, ok = slog.LevelFromString(fields[0])
		if !ok {
			return fmt.Errorf("unknown log level %q", fields[0])
		}

		for _, l := range bknd.loggers {
			l.SetLevel(bknd.defaultLogLevel)
		}
	} else if len(fields) == 2 {
		subsys := fields[0]
		level, _ := slog.LevelFromString(fields[1])
		bknd.logLevels[subsys] = level
		if l, ok := bknd.loggers[subsys]; ok {
			l.SetLevel(level)
		}
	} else {
		return fmt.Errorf("unable to parse %q as subsys=level "+
			"debuglevel string", s)
	}

	return nil
}

func (bknd *logBackend) lastLogLines(n int) []string {
	return bknd.logLines.LastLogLines(n)
}
