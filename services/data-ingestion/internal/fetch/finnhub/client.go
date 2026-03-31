package finnhub

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/berdelis/trading-agent/services/data-ingestion/internal/httpclient"
	"github.com/berdelis/trading-agent/services/data-ingestion/internal/store"
	"golang.org/x/time/rate"
)

const base = "https://finnhub.io/api/v1"

type Client struct {
	Token   string
	HTTP    *http.Client
	Limiter *rate.Limiter
}

func New(token string) *Client {
	return &Client{
		Token:   token,
		HTTP:    httpclient.New(25 * time.Second),
		Limiter: rate.NewLimiter(rate.Every(2*time.Second), 2),
	}
}

func (c *Client) HasToken() bool {
	return c.Token != ""
}

// Quote returns latest OHLC-style snapshot fields when available.
func (c *Client) Quote(ctx context.Context, symbol string) (map[string]any, error) {
	if !c.HasToken() {
		return nil, fmt.Errorf("finnhub token missing")
	}
	if err := c.Limiter.Wait(ctx); err != nil {
		return nil, err
	}
	q := url.Values{}
	q.Set("symbol", symbol)
	q.Set("token", c.Token)
	u := base + "/quote?" + q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("finnhub quote: %s", resp.Status)
	}
	var m map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		return nil, err
	}
	return m, nil
}

// CryptoNews fetches recent crypto headlines (Finnhub category=crypto).
func (c *Client) CryptoNews(ctx context.Context) ([]map[string]any, error) {
	if !c.HasToken() {
		return nil, fmt.Errorf("finnhub token missing")
	}
	if err := c.Limiter.Wait(ctx); err != nil {
		return nil, err
	}
	q := url.Values{}
	q.Set("category", "crypto")
	q.Set("token", c.Token)
	u := base + "/news?" + q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("finnhub news: %s", resp.Status)
	}
	var items []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		return nil, err
	}
	return items, nil
}

// StoreQuoteAsEquityBar builds a coarse bar from quote snapshot (uses current price as close).
func StoreQuoteAsEquityBar(symbol string, q map[string]any) (store.EquityBar, bool) {
	c, ok := toFloat(q["c"])
	if !ok {
		return store.EquityBar{}, false
	}
	ts := time.Now().UTC().Truncate(time.Minute)
	return store.EquityBar{
		TS:       ts,
		Symbol:   symbol,
		Interval: "quote_snapshot",
		Open:     c,
		High:     c,
		Low:      c,
		Close:    c,
		Volume:   0,
		Source:   "finnhub_quote",
	}, true
}

func toFloat(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	default:
		return 0, false
	}
}
