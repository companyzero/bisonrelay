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

func runDcrlnd(ctx context.Context, cfg embeddeddcrlnd.Config) (*embeddeddcrlnd.Dcrlnd, error) {
	lndc, err := embeddeddcrlnd.RunDcrlnd(ctx, cfg)
	if err != nil {
		return nil, err
	}

	// Track the active lnd connection.
	currentLndcMtx.Lock()
	currentLndc = lndc
	currentLndcMtx.Unlock()

	return lndc, nil
}

func isDcrlndRunning() bool {
	currentLndcMtx.Lock()
	res := currentLndc != nil
	currentLndcMtx.Unlock()
	return res
}

func runningDcrlnd() *embeddeddcrlnd.Dcrlnd {
	currentLndcMtx.Lock()
	res := currentLndc
	currentLndcMtx.Unlock()
	return res

}
