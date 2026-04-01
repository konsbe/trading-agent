// Package alphavantage provides a minimal client for the Alpha Vantage free API.
//
// Free tier: 25 API requests/day (5 requests/minute).
// API key: https://www.alphavantage.co/support/#api-key (free, instant).
//
// We use a single endpoint — COMPANY_OVERVIEW — which returns forward P/E,
// sector, beta, and analyst target price in one call per symbol.
// At 5 symbols × 1 call/day this is well within the free tier.
package alphavantage

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/konsbe/trading-agent/services/data-ingestion/internal/httpclient"
	"golang.org/x/time/rate"
)

const base = "https://www.alphavantage.co/query"

// Client wraps the Alpha Vantage REST API.
type Client struct {
	APIKey  string
	HTTP    *http.Client
	Limiter *rate.Limiter
}

// New returns a Client. Rate limiter is set conservatively at 1 req/15s
// to stay well within the 5 req/min free-tier limit across all workers.
func New(apiKey string) *Client {
	return &Client{
		APIKey:  apiKey,
		HTTP:    httpclient.New(20 * time.Second),
		Limiter: rate.NewLimiter(rate.Every(15*time.Second), 1),
	}
}

// HasKey returns false when no API key is configured.
func (c *Client) HasKey() bool { return c.APIKey != "" }

// Overview fetches the COMPANY_OVERVIEW endpoint for a symbol.
// Returns the raw flat string map; callers use FloatField / StringField to extract values.
//
// Key fields returned:
//
//	ForwardPE, TrailingPE, PEGRatio, EPS, DilutedEPSTTM
//	Beta, AnalystTargetPrice, Sector, Industry
//	QuarterlyEarningsGrowthYOY, QuarterlyRevenueGrowthYOY
//	PriceToBookRatio, EVToEBITDA, ProfitMargin, OperatingMarginTTM
//	SharesOutstanding, DividendYield, PayoutRatio, 52WeekHigh, 52WeekLow
func (c *Client) Overview(ctx context.Context, symbol string) (map[string]string, error) {
	if !c.HasKey() {
		return nil, fmt.Errorf("alpha vantage: API key not configured")
	}
	if err := c.Limiter.Wait(ctx); err != nil {
		return nil, err
	}
	url := fmt.Sprintf("%s?function=OVERVIEW&symbol=%s&apikey=%s", base, symbol, c.APIKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("alpha vantage overview %s: HTTP %s", symbol, resp.Status)
	}
	var m map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		return nil, err
	}
	// Alpha Vantage returns {"Information": "..."} when the key is invalid or rate-limited.
	if info, ok := m["Information"]; ok {
		return nil, fmt.Errorf("alpha vantage: %s", info)
	}
	// Empty symbol returns {"Symbol": ""}.
	if m["Symbol"] == "" {
		return nil, fmt.Errorf("alpha vantage: no data for %s (ETF or invalid symbol)", symbol)
	}
	return m, nil
}

// FloatField parses a numeric string field from an Overview response.
// Returns nil when the field is absent, "None", or non-numeric.
func FloatField(m map[string]string, key string) *float64 {
	v, ok := m[key]
	if !ok || v == "" || v == "None" || v == "-" {
		return nil
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return nil
	}
	return &f
}

// StringField returns the raw string value of a field, or "" if absent.
func StringField(m map[string]string, key string) string {
	return m[key]
}
