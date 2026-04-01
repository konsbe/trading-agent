package alpacadata

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/konsbe/trading-agent/services/data-ingestion/internal/store"
	"golang.org/x/time/rate"
)

const dataBase = "https://data.alpaca.markets"

type Client struct {
	KeyID     string
	SecretKey string
	HTTP      *http.Client
	Limiter   *rate.Limiter
}

func New(keyID, secret string) *Client {
	return &Client{
		KeyID:     keyID,
		SecretKey: secret,
		HTTP:      &http.Client{Timeout: 30 * time.Second},
		Limiter:   rate.NewLimiter(rate.Every(250*time.Millisecond), 4),
	}
}

func (c *Client) HasCredentials() bool {
	return c.KeyID != "" && c.SecretKey != ""
}

// FetchLatestBars requests recent hourly bars for a US equity symbol (e.g. AAPL).
func (c *Client) FetchLatestBars(ctx context.Context, symbol, timeframe string, limit int) ([]store.EquityBar, error) {
	if !c.HasCredentials() {
		return nil, fmt.Errorf("alpaca market data credentials missing")
	}
	if err := c.Limiter.Wait(ctx); err != nil {
		return nil, err
	}
	if timeframe == "" {
		timeframe = "1Hour"
	}
	if limit <= 0 {
		limit = 100
	}
	q := url.Values{}
	q.Set("timeframe", timeframe)
	q.Set("limit", fmt.Sprintf("%d", limit))
	u := fmt.Sprintf("%s/v2/stocks/%s/bars?%s", dataBase, url.PathEscape(symbol), q.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("APCA-API-KEY-ID", c.KeyID)
	req.Header.Set("APCA-API-SECRET-KEY", c.SecretKey)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("alpaca bars %s: %s", symbol, resp.Status)
	}
	var body struct {
		Bars []struct {
			T string  `json:"t"`
			O float64 `json:"o"`
			H float64 `json:"h"`
			L float64 `json:"l"`
			C float64 `json:"c"`
			V float64 `json:"v"`
		} `json:"bars"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}
	out := make([]store.EquityBar, 0, len(body.Bars))
	for _, b := range body.Bars {
		ts, err := time.Parse(time.RFC3339Nano, b.T)
		if err != nil {
			ts, err = time.Parse("2006-01-02T15:04:05Z07:00", b.T)
			if err != nil {
				continue
			}
		}
		out = append(out, store.EquityBar{
			TS:       ts.UTC(),
			Symbol:   symbol,
			Interval: timeframe,
			Open:     b.O,
			High:     b.H,
			Low:      b.L,
			Close:    b.C,
			Volume:   b.V,
			Source:   "alpaca_data",
		})
	}
	return out, nil
}
