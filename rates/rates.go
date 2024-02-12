package rates

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/decred/slog"
)

type Config struct {
	HTTPClient *http.Client
	Log        slog.Logger

	OnionEnable bool
}

type Rates struct {
	cfg Config

	mtx         sync.Mutex
	dcrPrice    float64
	btcPrice    float64
	lastUpdated time.Time
}

func New(cfg Config) *Rates {
	return &Rates{
		cfg: cfg,
	}
}

func (r *Rates) Run(ctx context.Context) {
	const shortTimeout = time.Second * 30
	const longTimeout = time.Minute * 10
	const triesBeforeErr = 20
	const requestTimeout = shortTimeout

	t := time.NewTicker(1)

	var failedTries int
	var err error
	for {
		select {
		case <-ctx.Done():
			t.Stop()
			return
		case <-t.C:
			t.Stop()

			// Try from api.decred.org.
			rctx, cancel := context.WithTimeout(ctx, requestTimeout)
			if err = r.dcrAPI(rctx); err == nil {
				cancel()
				failedTries = 0
				t.Reset(longTimeout)
				continue
			}
			cancel()
			r.cfg.Log.Debugf("Unable to fetch rate from api.decred.org: %v", err)

			// Try from dcrdata.
			rctx, cancel = context.WithTimeout(ctx, requestTimeout)
			if err = r.dcrData(rctx); err == nil {
				cancel()
				failedTries = 0
				t.Reset(longTimeout)
				continue
			}
			cancel()
			r.cfg.Log.Debugf("Unable to fetch rate from dcrdata: %v", err)

			// Only log these at a higher warning level once after
			// the rate has been successfully fetched. This prevents
			// spam in the UI.
			failedTries++
			if failedTries == triesBeforeErr {
				r.Set(0, 0) // Unset rates

				r.cfg.Log.Errorf("Unable to fetch recent exchange rate. Will keep retrying.")
			}
			t.Reset(shortTimeout)
		}
	}
}

// Get returns the last fetched USD/DCR and USD/BTC prices.
func (r *Rates) Get() (float64, float64) {
	r.mtx.Lock()
	dcrPrice, btcPrice := r.dcrPrice, r.btcPrice
	r.mtx.Unlock()

	return dcrPrice, btcPrice
}

// Set manually sets the USD/DCR and USD/BTC prices.
func (r *Rates) Set(dcrPrice, btcPrice float64) {
	r.cfg.Log.Infof("Exchange rate set manually: DCR:%0.2f BTC:%0.2f",
		dcrPrice, btcPrice)

	r.mtx.Lock()
	r.dcrPrice = dcrPrice
	r.btcPrice = btcPrice
	r.lastUpdated = time.Now()
	r.mtx.Unlock()
}

func (r *Rates) dcrData(ctx context.Context) error {
	dcrDataExchange := struct {
		DCRPrice float64 `json:"dcrPrice"`
		BTCPrice float64 `json:"btcPrice"`
	}{}

	var apiURL string
	if r.cfg.OnionEnable {
		apiURL = "http://dcrdata5oppwcotlxkrlsp6afncnxvw54sw6jqftc4bjytm4rn27j3ad.onion/api/exchangerate"
	} else {
		apiURL = "https://explorer.dcrdata.org/api/exchangerate"
	}
	b, err := r.getRaw(ctx, apiURL)
	if err != nil {
		return err
	}
	if err = json.Unmarshal(b, &dcrDataExchange); err != nil {
		return fmt.Errorf("failed to decode exchange rate: %v", err)
	}

	r.cfg.Log.Infof("Current exchange rate via dcrdata: DCR:%0.2f BTC:%0.2f",
		dcrDataExchange.DCRPrice, dcrDataExchange.BTCPrice)

	r.mtx.Lock()
	r.dcrPrice = dcrDataExchange.DCRPrice
	r.btcPrice = dcrDataExchange.BTCPrice
	r.lastUpdated = time.Now()
	r.mtx.Unlock()

	return nil
}

func (r *Rates) dcrAPI(ctx context.Context) error {
	dcrAPIExchange := struct {
		DCRPrice    float64 `json:"decred_usd"`
		BTCPrice    float64 `json:"bitcoin_usd"`
		LastUpdated int64   `json:"lastupdated"`
	}{}

	var apiURL string
	if r.cfg.OnionEnable {
		apiURL = "http://uhzsyccm5uobnd2mzzwp765vdqveampfacvbarl7xaopkdd3hfrfqqad.onion/?c=price"
	} else {
		apiURL = "https://api.decred.org/?c=price"
	}
	b, err := r.getRaw(ctx, apiURL)
	if err != nil {
		return err
	}
	if err = json.Unmarshal(b, &dcrAPIExchange); err != nil {
		return fmt.Errorf("failed to decode exchange rate: %w", err)
	}

	now := time.Now()
	last := now.Sub(time.Unix(dcrAPIExchange.LastUpdated, 0))
	if last > 24*time.Hour {
		return fmt.Errorf("api.decred.org data is stale")
	}

	r.cfg.Log.Infof("Current exchange rate via API: DCR:%0.2f BTC:%0.2f",
		dcrAPIExchange.DCRPrice, dcrAPIExchange.BTCPrice)

	r.mtx.Lock()
	r.dcrPrice = dcrAPIExchange.DCRPrice
	r.btcPrice = dcrAPIExchange.BTCPrice
	r.lastUpdated = now
	r.mtx.Unlock()

	return nil
}

func (r *Rates) getRaw(ctx context.Context, exchangeAPI string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		exchangeAPI, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create new http request: %v", err)
	}
	req.Header.Del("User-Agent")

	resp, err := r.cfg.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get exchange rate: %v", err)
	}
	b, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("failed to read exchange rate response: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s", http.StatusText(resp.StatusCode))
	}
	return b, nil
}
