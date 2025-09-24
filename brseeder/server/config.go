package seederserver

import (
	"net"
	"time"

	"github.com/decred/slog"
)

type config struct {
	waitForMaster   time.Duration
	tokens          map[string]struct{}
	listeners       []net.Listener
	log             slog.Logger
	appName         string
	httpTimeout     time.Duration
	shutdownTimeout time.Duration
}

// Option is a functional option for configuring the seeder server.
type Option func(c *config)

// WithTokens sets the tokens for the configuration.
func WithTokens(tokens map[string]struct{}) Option {
	return func(c *config) {
		c.tokens = tokens
	}
}

// WithListeners sets the listeners for the configuration.
func WithListeners(listeners []net.Listener) Option {
	return func(c *config) {
		c.listeners = listeners
	}
}

// WithLogger sets the logger for the configuration.
func WithLogger(log slog.Logger) Option {
	return func(c *config) {
		c.log = log
	}
}
