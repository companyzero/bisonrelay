// Copyright (c) 2015-2023 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"log"
	"os"
	"os/signal"
)

// interruptSignals defines the default signals to catch in order to do a proper
// shutdown. This may be modified during init depending on the platform.
var interruptSignals = []os.Signal{os.Interrupt}

// shutdownListener returns a context whose done channel will be closed when OS
// signals such as SIGINT (Ctrl+C) are received.
func shutdownListener() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		interruptChannel := make(chan os.Signal, 1)
		signal.Notify(interruptChannel, interruptSignals...)

		// Listen for initial shutdown signal and cancel the context.
		select {
		case sig := <-interruptChannel:
			log.Printf("Received signal (%s). Shutting down...", sig)
			cancel()
		case <-ctx.Done():
		}

		// Listen for repeated signals and display a message so the user knows
		// the shutdown is in progress and the process is not hung.
		for {
			sig := <-interruptChannel
			log.Printf("Received signal (%s). Already shutting down...", sig)
		}
	}()

	return ctx, cancel
}
