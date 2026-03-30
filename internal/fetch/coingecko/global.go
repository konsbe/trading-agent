package coingecko

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"golang.org/x/time/rate"
)

const baseURL = "https://api.coingecko.com/api/v3"

type Client struct {
	HTTP    *http.Client
	Limiter *rate.Limiter
}

func New() *Client {
	return &Client{
		HTTP:    &http.Client{Timeout: 25 * time.Second},
		Limiter: rate.NewLimiter(rate.Every(2*time.Second), 2),
	}
}

// FetchGlobal returns the full /global JSON document (market cap, dominance, etc.).
func (c *Client) FetchGlobal(ctx context.Context) (map[string]any, error) {
	if err := c.Limiter.Wait(ctx); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/global", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("coingecko global: %s", resp.Status)
	}
	var wrap struct {
		Data map[string]any `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&wrap); err != nil {
		return nil, err
	}
	return wrap.Data, nil
}
