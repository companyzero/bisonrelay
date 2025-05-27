package logutil

import "github.com/decred/slog"

type prefixLogger struct {
	log    slog.Logger
	prefix string
}

// Tracef formats message according to format specifier and writes to
// to log with LevelTrace.
func (p *prefixLogger) Tracef(format string, params ...interface{}) {
	p.log.Tracef(p.prefix+" "+format, params...)
}

// Debugf formats message according to format specifier and writes to
// log with LevelDebug.
func (p *prefixLogger) Debugf(format string, params ...interface{}) {
	p.log.Debugf(p.prefix+" "+format, params...)
}

// Infof formats message according to format specifier and writes to
// log with LevelInfo.
func (p *prefixLogger) Infof(format string, params ...interface{}) {
	p.log.Infof(p.prefix+" "+format, params...)
}

// Warnf formats message according to format specifier and writes to
// to log with LevelWarn.
func (p *prefixLogger) Warnf(format string, params ...interface{}) {
	p.log.Warnf(p.prefix+" "+format, params...)
}

// Errorf formats message according to format specifier and writes to
// to log with LevelError.
func (p *prefixLogger) Errorf(format string, params ...interface{}) {
	p.log.Errorf(p.prefix+" "+format, params...)
}

// Criticalf formats message according to format specifier and writes to
// log with LevelCritical.
func (p *prefixLogger) Criticalf(format string, params ...interface{}) {
	p.log.Criticalf(p.prefix+" "+format, params...)
}

// Trace formats message using the default formats for its operands
// and writes to log with LevelTrace.
func (p *prefixLogger) Trace(v ...interface{}) {
	p.log.Trace(append([]interface{}{p.prefix}, v...))
}

// Debug formats message using the default formats for its operands
// and writes to log with LevelDebug.
func (p *prefixLogger) Debug(v ...interface{}) {
	p.log.Debug(append([]interface{}{p.prefix}, v...))
}

// Info formats message using the default formats for its operands
// and writes to log with LevelInfo.
func (p *prefixLogger) Info(v ...interface{}) {
	p.log.Info(append([]interface{}{p.prefix}, v...))
}

// Warn formats message using the default formats for its operands
// and writes to log with LevelWarn.
func (p *prefixLogger) Warn(v ...interface{}) {
	p.log.Warn(append([]interface{}{p.prefix}, v...))
}

// Error formats message using the default formats for its operands
// and writes to log with LevelError.
func (p *prefixLogger) Error(v ...interface{}) {
	p.log.Error(append([]interface{}{p.prefix}, v...))
}

// Critical formats message using the default formats for its operands
// and writes to log with LevelCritical.
func (p *prefixLogger) Critical(v ...interface{}) {
	p.log.Critical(append([]interface{}{p.prefix}, v...))
}

// Level returns the current logging level.
func (p *prefixLogger) Level() slog.Level {
	return p.log.Level()
}

// SetLevel changes the logging level to the passed level.
func (p *prefixLogger) SetLevel(level slog.Level) {
	p.log.SetLevel(level)
}

// PrefixLogger returns a logger that prepends a string in every message.
func PrefixLogger(log slog.Logger, prefix string) slog.Logger {
	return &prefixLogger{log: log, prefix: prefix}
}
