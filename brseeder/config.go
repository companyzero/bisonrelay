package main

import (
	"fmt"
	"net"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
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
