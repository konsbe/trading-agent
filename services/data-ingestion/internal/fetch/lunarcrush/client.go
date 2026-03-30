package lunarcrush

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"golang.org/x/time/rate"
)

// Client targets LunarCrush API v4 style endpoints. Exact paths vary by plan; adjust Base if needed.
type Client struct {
	APIKey  string
	Base    string
	HTTP    *http.Client
	Limiter *rate.Limiter
}

func New(apiKey string) *Client {
	return &Client{
		APIKey:  apiKey,
		Base:    "https://lunarcrush.com/api4",
		HTTP:    &http.Client{Timeout: 30 * time.Second},
		Limiter: rate.NewLimiter(rate.Every(2*time.Second), 2),
	}
}

func (c *Client) HasKey() bool {
	return c.APIKey != ""
}

// FetchCoin returns raw JSON for a single coin topic (e.g. BTC, ETH).
func (c *Client) FetchCoin(ctx context.Context, symbol string) (map[string]any, error) {
	if !c.HasKey() {
		return nil, fmt.Errorf("lunarcrush api key missing")
	}
	if err := c.Limiter.Wait(ctx); err != nil {
		return nil, err
	}
	u, err := url.JoinPath(c.Base, "public", "coins", symbol, "v1")
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("lunarcrush %s: %s", symbol, resp.Status)
	}
	var m map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		return nil, err
	}
	return m, nil
}
