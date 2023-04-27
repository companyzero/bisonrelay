package golib

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const urlExchangeRate = "https://explorer.dcrdata.org/api/exchangerate"

type exchangeRate struct {
	DCRPrice float64 `json:"dcrPrice"`
	BTCPrice float64 `json:"btcPrice"`
}

func (cctx *clientCtx) fetchExchangeRate(ctx context.Context) (exchangeRate, error) {
	var eRate exchangeRate
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		urlExchangeRate, nil)
	if err != nil {
		return eRate, fmt.Errorf("failed to create new http request: %v", err)
	}
	req.Header.Del("User-Agent")

	resp, err := cctx.httpClient.Do(req)
	if err != nil {
		return eRate, fmt.Errorf("failed to get exchange rate: %v", err)
	}
	b, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return eRate, fmt.Errorf("failed to read exchange rate response: %v", err)
	}
	if err = json.Unmarshal(b, &eRate); err != nil {
		return eRate, fmt.Errorf("failed to decode exchange rate: %v", err)
	}
	return eRate, nil
}

func (cctx *clientCtx) trackExchangeRate() {
	const shortTimeout = time.Second * 30
	const longTimeout = time.Minute * 10

	var timeout time.Duration
	for {
		select {
		case <-cctx.ctx.Done():
			return
		case <-time.After(timeout):
			eRate, err := cctx.fetchExchangeRate(cctx.ctx)
			if err != nil {
				cctx.log.Warnf("Unable to fetch exchange rate: %v", err)
				timeout = shortTimeout
			} else {
				cctx.erMtx.Lock()
				if eRate.DCRPrice != cctx.eRate.DCRPrice {
					cctx.log.Infof("Using exchange rate of %.4f USD/DCR",
						eRate.DCRPrice)
				}
				cctx.eRate = eRate
				cctx.erMtx.Unlock()

				timeout = longTimeout
			}
		}
	}
}

func (cctx *clientCtx) exchangeRate() exchangeRate {
	cctx.erMtx.Lock()
	res := cctx.eRate
	cctx.erMtx.Unlock()
	return res
}
