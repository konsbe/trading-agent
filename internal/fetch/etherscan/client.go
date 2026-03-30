package etherscan

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
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
		HTTP:    &http.Client{Timeout: 25 * time.Second},
		Limiter: rate.NewLimiter(rate.Every(250*time.Millisecond), 3),
	}
}

func (c *Client) HasKey() bool {
	return c.APIKey != ""
}

// EthSupply returns total ETH supply as reported by Etherscan stats API.
func (c *Client) EthSupply(ctx context.Context) (float64, error) {
	if !c.HasKey() {
		return 0, fmt.Errorf("etherscan api key missing")
	}
	if err := c.Limiter.Wait(ctx); err != nil {
		return 0, err
	}
	q := url.Values{}
	q.Set("module", "stats")
	q.Set("action", "ethsupply")
	q.Set("apikey", c.APIKey)
	u := "https://api.etherscan.io/api?" + q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return 0, err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("etherscan: %s", resp.Status)
	}
	var body struct {
		Status  string `json:"status"`
		Message string `json:"message"`
		Result  string `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return 0, err
	}
	if body.Result == "" {
		return 0, fmt.Errorf("etherscan: %s %s", body.Status, body.Message)
	}
	i := new(big.Int)
	if _, ok := i.SetString(body.Result, 10); !ok {
		return 0, fmt.Errorf("etherscan: parse result")
	}
	f := new(big.Float).Quo(new(big.Float).SetInt(i), big.NewFloat(1e18))
	v, _ := f.Float64()
	return v, nil
}
