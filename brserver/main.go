package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/companyzero/bisonrelay/internal/version"
	"github.com/companyzero/bisonrelay/server"
)

func _main() error {
	// flags and settings
	cfg, err := ObtainSettings()
	if err != nil {
		return err
	}
	cfg.Versioner = version.String

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Wait for termination signals.
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigs
		cancel()
	}()

	// Init server.
	z, err := server.NewServer(cfg)
	if err != nil {
		return err
	}

	// Run server.
	err = z.Run(ctx)
	if errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}

func main() {
	err := _main()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}
