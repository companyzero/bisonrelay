package golib

import (
	"context"
	"sync"

	"github.com/companyzero/bisonrelay/embeddeddcrlnd"
)

var (
	currentLndcMtx sync.Mutex
	currentLndc    *embeddeddcrlnd.Dcrlnd
)

func runDcrlnd(rootDir string, network string) (*embeddeddcrlnd.Dcrlnd, error) {

	debugLevel := "info"
	cfg := embeddeddcrlnd.Config{
		RootDir:    rootDir,
		Network:    network,
		DebugLevel: debugLevel,
	}
	lndc, err := embeddeddcrlnd.RunDcrlnd(context.Background(), cfg)
	if err != nil {
		return nil, err
	}

	// Track the active lnd connection.
	currentLndcMtx.Lock()
	currentLndc = lndc
	currentLndcMtx.Unlock()

	return lndc, nil
}
