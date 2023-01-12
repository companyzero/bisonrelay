package rpc

import "github.com/decred/slog"

var log slog.Logger = slog.Disabled

// SetLog sets the package-level logger.
func SetLog(v slog.Logger) {
	log = v
}
