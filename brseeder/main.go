package main

import (
	"fmt"
	"net"
	"os"

	seederserver "github.com/companyzero/bisonrelay/brseeder/server"
)

func realMain() error {
	ctx, cancel := shutdownListener()
	defer cancel()

	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("loadConfig: %v", err)
	}
	tokens := make(map[string]struct{})
	for i := range cfg.Tokens {
		tokens[cfg.Tokens[i]] = struct{}{}
	}

	var listenCfg net.ListenConfig
	listener, err := listenCfg.Listen(ctx, "tcp", cfg.Listen)
	if err != nil {
		return fmt.Errorf("unable to listen on %v: %v", cfg.Listen, err)
	}

	logger, err := initLog()
	if err != nil {
		return fmt.Errorf("unable to init logger: %v", err)
	}

	srv, err := seederserver.New(seederserver.WithListeners([]net.Listener{listener}),
		seederserver.WithLogger(logger),
		seederserver.WithTokens(tokens))
	if err != nil {
		return fmt.Errorf("unable to init server: %v", err)
	}

	return srv.Run(ctx)
}

func main() {
	if err := realMain(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err.Error())
		os.Exit(1)
	}
}
