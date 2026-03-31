// Package yahoo fetches OHLCV bars from the Yahoo Finance v8 chart API.
// No API key or account is required. Rate-limited conservatively to avoid 429s.
package yahoo

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"time"

	"github.com/berdelis/trading-agent/services/data-ingestion/internal/store"
	"golang.org/x/time/rate"
)

const apiBase = "https://query1.finance.yahoo.com/v8/finance/chart"

// Client fetches OHLCV bars from Yahoo Finance (no API key required).
type Client struct {
	HTTP    *http.Client
	Limiter *rate.Limiter
}

// New creates a Yahoo Finance client with a conservative rate limit (1 req / 1.5 s).
func New() *Client {
	return &Client{
		HTTP:    &http.Client{Timeout: 30 * time.Second},
		Limiter: rate.NewLimiter(rate.Every(1500*time.Millisecond), 1),
	}
}

// FetchBars fetches equity OHLCV bars from Yahoo Finance.
//
// alpacaInterval must be an Alpaca-format timeframe string ("1Day", "1Week",
// "1Hour", etc.) — it is converted to Yahoo's format internally and is also
// stored verbatim in the returned EquityBar.Interval field so that the rows
// are consistent with data already in equity_ohlcv.
//
// limit caps the number of returned bars (most-recent). Pass 0 for no cap.
func (c *Client) FetchBars(ctx context.Context, symbol, alpacaInterval string, limit int) ([]store.EquityBar, error) {
	if err := c.Limiter.Wait(ctx); err != nil {
		return nil, err
	}

	yInterval, yRange := toYahooParams(alpacaInterval, limit)
	if yInterval == "" {
		return nil, fmt.Errorf("yahoo: unsupported interval %q", alpacaInterval)
	}

	q := url.Values{}
	q.Set("interval", yInterval)
	q.Set("range", yRange)
	q.Set("includePrePost", "false")

	u := fmt.Sprintf("%s/%s?%s", apiBase, url.PathEscape(symbol), q.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	// Minimal browser-like headers to satisfy Yahoo's CDN checks.
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64; rv:125.0) Gecko/20100101 Firefox/125.0")
	req.Header.Set("Accept", "application/json, */*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("yahoo fetch %s: %w", symbol, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("yahoo finance %s: HTTP %s", symbol, resp.Status)
	}

	var body chartResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("yahoo decode %s: %w", symbol, err)
	}
	if len(body.Chart.Error) > 0 && string(body.Chart.Error) != "null" {
		return nil, fmt.Errorf("yahoo error %s: %s", symbol, body.Chart.Error)
	}
	if len(body.Chart.Result) == 0 || len(body.Chart.Result[0].Timestamp) == 0 {
		return nil, nil
	}

	result := body.Chart.Result[0]
	if len(result.Indicators.Quote) == 0 {
		return nil, nil
	}
	qt := result.Indicators.Quote[0]
	n := len(result.Timestamp)

	bars := make([]store.EquityBar, 0, n)
	for i := 0; i < n; i++ {
		if i >= len(qt.Open) || i >= len(qt.High) || i >= len(qt.Low) ||
			i >= len(qt.Close) || i >= len(qt.Volume) {
			continue
		}
		o := deref(qt.Open[i])
		h := deref(qt.High[i])
		l := deref(qt.Low[i])
		cl := deref(qt.Close[i])
		v := deref(qt.Volume[i])

		// Skip bars with any null/NaN field or zero volume (non-trading day).
		if math.IsNaN(o) || math.IsNaN(h) || math.IsNaN(l) || math.IsNaN(cl) || math.IsNaN(v) || v == 0 {
			continue
		}

		bars = append(bars, store.EquityBar{
			TS:       time.Unix(result.Timestamp[i], 0).UTC(),
			Symbol:   symbol,
			Interval: alpacaInterval, // store in Alpaca format for DB consistency
			Open:     o,
			High:     h,
			Low:      l,
			Close:    cl,
			Volume:   v,
			Source:   "yahoo_finance",
		})
	}

	// Trim to the most recent `limit` bars.
	if limit > 0 && len(bars) > limit {
		bars = bars[len(bars)-limit:]
	}
	return bars, nil
}

// toYahooParams converts an Alpaca-format interval string to the Yahoo Finance
// API's interval and range parameters, choosing a range that comfortably covers
// the requested bar count.
func toYahooParams(alpacaInterval string, limit int) (interval, rangeStr string) {
	switch alpacaInterval {
	case "1Min":
		return "1m", "7d"
	case "5Min":
		return "5m", "60d"
	case "15Min":
		return "15m", "60d"
	case "30Min":
		return "30m", "60d"
	case "1Hour":
		if limit <= 168 {
			return "1h", "7d"
		}
		return "1h", "730d"
	case "4Hour":
		// Yahoo does not have a 4-hour bar; use 60-minute as the nearest option.
		return "60m", "730d"
	case "1Day":
		if limit <= 126 {
			return "1d", "6mo"
		}
		if limit <= 252 {
			return "1d", "1y"
		}
		return "1d", "2y" // ~520 trading days — enough for 200-period daily indicators
	case "1Week":
		return "1wk", "5y"
	case "1Month":
		return "1mo", "max"
	default:
		return "", ""
	}
}

// chartResponse mirrors the Yahoo Finance v8 chart API JSON structure.
// Volume is kept as *float64 — Go's JSON decoder converts integers to float64.
type chartResponse struct {
	Chart struct {
		Result []struct {
			Timestamp  []int64 `json:"timestamp"`
			Indicators struct {
				Quote []struct {
					Open   []*float64 `json:"open"`
					High   []*float64 `json:"high"`
					Low    []*float64 `json:"low"`
					Close  []*float64 `json:"close"`
					Volume []*float64 `json:"volume"`
				} `json:"quote"`
			} `json:"indicators"`
		} `json:"result"`
		Error json.RawMessage `json:"error"`
	} `json:"chart"`
}

func deref(p *float64) float64 {
	if p == nil {
		return math.NaN()
	}
	return *p
}
