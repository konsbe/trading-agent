package finnhub

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// StockSymbol is one entry from Finnhub's exchange symbol directory.
//
// Field notes that matter for universe eligibility (§3.1):
//   - Type is free-text and inconsistent: "Common Stock", "ETP", "ADR",
//     "Closed-End Fund", "Unit", "Warrant", "PUBLIC", and sometimes "".
//     Never assume a closed set; match against an allowlist.
//   - MIC is the ISO 10383 venue code and is the only reliable way to tell
//     NASDAQ/NYSE/NYSE American from NYSE Arca or an OTC tier. The Description
//     and DisplaySymbol fields are cosmetic.
//   - MIC is occasionally blank. Callers decide whether to admit those.
type StockSymbol struct {
	Symbol         string `json:"symbol"`
	DisplaySymbol  string `json:"displaySymbol"`
	Description    string `json:"description"`
	Type           string `json:"type"`
	MIC            string `json:"mic"`
	Currency       string `json:"currency"`
	FIGI           string `json:"figi"`
	ShareClassFIGI string `json:"shareClassFIGI"`
	ISIN           string `json:"isin"`
	Symbol2        string `json:"symbol2"`
}

// StockSymbols returns the full symbol directory for an exchange.
//
// Endpoint: GET /stock/symbol?exchange=<code>
//
// For exchange="US" this is a single request returning the entire US listing —
// roughly 25-30k rows across every instrument type, of which §3.1's filters
// keep 5-7.5k. One call, so it costs almost nothing against the free tier's
// 60 req/min; the response is a few megabytes, hence the streaming decode.
func (c *Client) StockSymbols(ctx context.Context, exchange string) ([]StockSymbol, error) {
	if !c.HasToken() {
		return nil, fmt.Errorf("finnhub token missing")
	}
	if err := c.Limiter.Wait(ctx); err != nil {
		return nil, err
	}
	if exchange == "" {
		exchange = "US"
	}
	q := url.Values{}
	q.Set("exchange", exchange)
	q.Set("token", c.Token)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/stock/symbol?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("finnhub stock/symbol %s: %s", exchange, resp.Status)
	}
	var out []StockSymbol
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode stock/symbol %s: %w", exchange, err)
	}
	return out, nil
}
