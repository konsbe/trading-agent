package glassnode

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"golang.org/x/time/rate"
)

type Client struct {
	APIKey  string
	HTTP    *http.Client
	Limiter *rate.Limiter
}

func New(apiKey string) *Client {
	return &Client{
		APIKey:  apiKey,
		HTTP:    &http.Client{Timeout: 40 * time.Second},
		Limiter: rate.NewLimiter(rate.Every(2*time.Second), 2),
	}
}

func (c *Client) HasKey() bool {
	return c.APIKey != ""
}

// FetchMetric calls a Glassnode v1 metric path, e.g. "addresses/active_count" with query a=BTC&i=24h.
func (c *Client) FetchMetric(ctx context.Context, metricPath string, params url.Values) ([]map[string]any, error) {
	if !c.HasKey() {
		return nil, fmt.Errorf("glassnode api key missing")
	}
	if err := c.Limiter.Wait(ctx); err != nil {
		return nil, err
	}
	if params == nil {
		params = url.Values{}
	}
	params.Set("api_key", c.APIKey)
	u := fmt.Sprintf("https://api.glassnode.com/v1/metrics/%s?%s", metricPath, params.Encode())
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
		return nil, fmt.Errorf("glassnode %s: %s", metricPath, resp.Status)
	}
	var arr []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&arr); err != nil {
		return nil, err
	}
	return arr, nil
}
