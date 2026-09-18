package finnhub

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/konsbe/trading-agent/services/data-ingestion/internal/httpclient"
	"github.com/konsbe/trading-agent/services/data-ingestion/internal/store"
	"golang.org/x/time/rate"
)

const base = "https://finnhub.io/api/v1"

// Limiter is the subset of *golang.org/x/time/rate.Limiter this client uses.
//
// Declared structurally so the client can be handed either its own in-process
// bucket or a cross-process shared budget (internal/ratelimit) without this
// package importing either one.
type Limiter interface {
	Wait(ctx context.Context) error
}

type Client struct {
	Token   string
	HTTP    *http.Client
	Limiter Limiter
}

// defaultLimiter is the per-process bucket this client has always used:
// 1 request / 2s, burst 2.
//
// It is correct in isolation and wrong in aggregate — five workers sharing one
// API key each allow this rate, summing to five times the intended budget. It
// remains the FALLBACK for the shared limiter, so a coordination outage
// degrades to exactly this, never to unlimited.
func defaultLimiter() *rate.Limiter {
	return rate.NewLimiter(rate.Every(2*time.Second), 2)
}

func New(token string) *Client {
	return &Client{
		Token:   token,
		HTTP:    httpclient.New(25 * time.Second),
		Limiter: defaultLimiter(),
	}
}

// NewWithLimiter builds a client pacing itself against the supplied limiter,
// normally a cross-process shared budget.
//
// A nil limiter yields exactly New()'s behaviour, so a caller that cannot build
// a shared limiter passes nil and keeps working rather than special-casing.
func NewWithLimiter(token string, l Limiter) *Client {
	if l == nil {
		return New(token)
	}
	return &Client{
		Token:   token,
		HTTP:    httpclient.New(25 * time.Second),
		Limiter: l,
	}
}

// DefaultLimiter exposes the per-process bucket so callers can pass it as the
// shared limiter's fallback without re-deriving the rate and drifting from it.
func DefaultLimiter() Limiter { return defaultLimiter() }

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

// CompanyNews fetches recent company-specific headlines for a stock symbol.
// Endpoint: GET /company-news?symbol=<sym>&from=<date>&to=<date>
// Uses a 30-day window to capture recent articles.
func (c *Client) CompanyNews(ctx context.Context, symbol string) ([]map[string]any, error) {
	if !c.HasToken() {
		return nil, fmt.Errorf("finnhub token missing")
	}
	if err := c.Limiter.Wait(ctx); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	q := url.Values{}
	q.Set("symbol", symbol)
	q.Set("from", now.AddDate(0, 0, -30).Format("2006-01-02"))
	q.Set("to", now.Format("2006-01-02"))
	q.Set("token", c.Token)
	u := base + "/company-news?" + q.Encode()
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
		return nil, fmt.Errorf("finnhub company-news %s: %s", symbol, resp.Status)
	}
	var items []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		return nil, err
	}
	return items, nil
}

// CompanyNewsRange fetches company headlines over an EXPLICIT date window.
//
// CompanyNews hardcodes a trailing 30 days, which is right for live scanning and
// useless for §3.11's historical evaluation: testing whether a catalyst preceded
// a runner needs the 48 hours before THAT candidate's date, not before today.
//
// Finnhub's free tier serves roughly the last 12 months of company news —
// measured, not documented: a 2026-01 window returns 243 AAPL articles while
// 2025-09 returns 0. Windows older than that come back as an empty list rather
// than an error, so callers must treat "no articles" as possibly-unavailable
// rather than definitely-no-coverage.
func (c *Client) CompanyNewsRange(ctx context.Context, symbol string, from, to time.Time) ([]map[string]any, error) {
	if !c.HasToken() {
		return nil, fmt.Errorf("finnhub token missing")
	}
	if err := c.Limiter.Wait(ctx); err != nil {
		return nil, err
	}
	q := url.Values{}
	q.Set("symbol", symbol)
	q.Set("from", from.UTC().Format("2006-01-02"))
	q.Set("to", to.UTC().Format("2006-01-02"))
	q.Set("token", c.Token)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/company-news?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("finnhub company-news %s %s..%s: %s",
			symbol, from.Format("2006-01-02"), to.Format("2006-01-02"), resp.Status)
	}
	var items []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		return nil, err
	}
	return items, nil
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

// ─── Fundamental data endpoints ───────────────────────────────────────────────

// Metrics fetches Finnhub's key financial metrics snapshot for a symbol.
// Endpoint: GET /stock/metric?symbol=<sym>&metric=all
// Returns the raw response map; callers extract the fields they need.
func (c *Client) Metrics(ctx context.Context, symbol string) (map[string]any, error) {
	if !c.HasToken() {
		return nil, fmt.Errorf("finnhub token missing")
	}
	if err := c.Limiter.Wait(ctx); err != nil {
		return nil, err
	}
	q := url.Values{}
	q.Set("symbol", symbol)
	q.Set("metric", "all")
	q.Set("token", c.Token)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/stock/metric?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("finnhub metrics %s: %s", symbol, resp.Status)
	}
	var m map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		return nil, err
	}
	return m, nil
}

// Profile2 fetches the company profile.
//
// Endpoint: GET /stock/profile2?symbol=<sym>
//
// This exists because shareOutstanding is NOT on /stock/metric — verified absent
// even for AAPL, not merely null for micro-caps. Without it §3.9's
// market_cap_est fallback (shares_outstanding x close) has no input, so the
// symbols that fallback exists to rescue stay ungateable, and §4.2's float
// sub-score is permanently null.
//
// Returns the raw response map; callers extract what they need. NOTE the units:
// both shareOutstanding and marketCapitalization come back in MILLIONS
// (AAPL: shareOutstanding 14687.36 = 14.69 billion shares).
func (c *Client) Profile2(ctx context.Context, symbol string) (map[string]any, error) {
	if !c.HasToken() {
		return nil, fmt.Errorf("finnhub token missing")
	}
	if err := c.Limiter.Wait(ctx); err != nil {
		return nil, err
	}
	q := url.Values{}
	q.Set("symbol", symbol)
	q.Set("token", c.Token)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/stock/profile2?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("finnhub profile2 %s: %s", symbol, resp.Status)
	}
	var m map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		return nil, err
	}
	return m, nil
}

// FinancialsReported fetches the most recent reported quarterly or annual financials.
// freq: "quarterly" or "annual"
// Endpoint: GET /stock/financials-reported?symbol=<sym>&freq=<freq>
func (c *Client) FinancialsReported(ctx context.Context, symbol, freq string) (map[string]any, error) {
	if !c.HasToken() {
		return nil, fmt.Errorf("finnhub token missing")
	}
	if err := c.Limiter.Wait(ctx); err != nil {
		return nil, err
	}
	q := url.Values{}
	q.Set("symbol", symbol)
	q.Set("freq", freq)
	q.Set("token", c.Token)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/stock/financials-reported?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("finnhub financials-reported %s: %s", symbol, resp.Status)
	}
	var m map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		return nil, err
	}
	return m, nil
}

// Earnings fetches historical EPS actuals vs estimates (surprises).
// Endpoint: GET /stock/earnings?symbol=<sym>
func (c *Client) Earnings(ctx context.Context, symbol string) ([]map[string]any, error) {
	if !c.HasToken() {
		return nil, fmt.Errorf("finnhub token missing")
	}
	if err := c.Limiter.Wait(ctx); err != nil {
		return nil, err
	}
	q := url.Values{}
	q.Set("symbol", symbol)
	q.Set("token", c.Token)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/stock/earnings?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("finnhub earnings %s: %s", symbol, resp.Status)
	}
	var items []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		return nil, err
	}
	return items, nil
}

// Recommendation fetches the analyst recommendation trend for a symbol.
// Endpoint: GET /stock/recommendation?symbol=<sym>
// Returns an array of monthly objects with buy/hold/sell/strongBuy/strongSell counts.
// Newest month is first in the slice.
func (c *Client) Recommendation(ctx context.Context, symbol string) ([]map[string]any, error) {
	if !c.HasToken() {
		return nil, fmt.Errorf("finnhub token missing")
	}
	if err := c.Limiter.Wait(ctx); err != nil {
		return nil, err
	}
	q := url.Values{}
	q.Set("symbol", symbol)
	q.Set("token", c.Token)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/stock/recommendation?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("finnhub recommendation %s: %s", symbol, resp.Status)
	}
	var items []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		return nil, err
	}
	return items, nil
}

// InsiderTransactions fetches SEC Form 4 insider buy/sell records for a symbol.
// Endpoint: GET /stock/insider-transactions?symbol=<sym>
// Returns an array of transaction objects. Key fields:
//
//	transactionDate, name, transactionCode (P/S/A/F/M), change (shares), transactionPrice, filingDate
//
// Available on Finnhub free tier (limited to recent filings).
func (c *Client) InsiderTransactions(ctx context.Context, symbol string) ([]map[string]any, error) {
	if !c.HasToken() {
		return nil, fmt.Errorf("finnhub token missing")
	}
	if err := c.Limiter.Wait(ctx); err != nil {
		return nil, err
	}
	q := url.Values{}
	q.Set("symbol", symbol)
	q.Set("token", c.Token)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/stock/insider-transactions?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("finnhub insider-transactions %s: %s", symbol, resp.Status)
	}
	// Response: {"data": [...], "symbol": "AAPL"}
	var raw struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}
	return raw.Data, nil
}

// InvestorOwnership fetches the top institutional holders for a symbol.
// Endpoint: GET /stock/investor-ownership?symbol=<sym>&limit=<n>
// Returns an array of holder objects sorted by shares (largest first). Key fields:
//
//	investorName, share (shares held), change (quarterly change), date, portfolioPercent
//
// Available on Finnhub free tier.
func (c *Client) InvestorOwnership(ctx context.Context, symbol string, limit int) ([]map[string]any, error) {
	if !c.HasToken() {
		return nil, fmt.Errorf("finnhub token missing")
	}
	if err := c.Limiter.Wait(ctx); err != nil {
		return nil, err
	}
	q := url.Values{}
	q.Set("symbol", symbol)
	q.Set("limit", fmt.Sprintf("%d", limit))
	q.Set("token", c.Token)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/stock/investor-ownership?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("finnhub investor-ownership %s: %s", symbol, resp.Status)
	}
	// Response: {"data": [...], "symbol": "AAPL"}
	var raw struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}
	return raw.Data, nil
}

// ─── Quote helpers ─────────────────────────────────────────────────────────────

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

// ─── Calendars (macro / event intelligence) ─────────────────────────────────

// CalendarEconomic fetches macro economic calendar events in [from, to] (YYYY-MM-DD).
// Endpoint: GET /calendar/economic?from=&to=
// TODO [PAID]: Some Finnhub tiers restrict this endpoint — handle 403 gracefully.
func (c *Client) CalendarEconomic(ctx context.Context, from, to string) ([]map[string]any, error) {
	if !c.HasToken() {
		return nil, fmt.Errorf("finnhub token missing")
	}
	if err := c.Limiter.Wait(ctx); err != nil {
		return nil, err
	}
	q := url.Values{}
	q.Set("from", from)
	q.Set("to", to)
	q.Set("token", c.Token)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/calendar/economic?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("finnhub calendar-economic: %s — %s", resp.Status, trimAPIErr(body))
	}
	return decodeCalendarEventsJSON(body, "economicCalendar")
}

// CalendarEarnings fetches earnings calendar in [from, to]. Optional symbol filters to one ticker.
// Endpoint: GET /calendar/earnings?from=&to=&symbol=
func (c *Client) CalendarEarnings(ctx context.Context, from, to, symbol string) ([]map[string]any, error) {
	if !c.HasToken() {
		return nil, fmt.Errorf("finnhub token missing")
	}
	if err := c.Limiter.Wait(ctx); err != nil {
		return nil, err
	}
	q := url.Values{}
	q.Set("from", from)
	q.Set("to", to)
	if symbol != "" {
		q.Set("symbol", symbol)
	}
	q.Set("token", c.Token)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/calendar/earnings?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("finnhub calendar-earnings: %s — %s", resp.Status, trimAPIErr(body))
	}
	out, err := decodeCalendarEventsJSON(body, "earningsCalendar")
	if err != nil {
		return nil, err
	}
	if len(out) > 0 {
		return out, nil
	}
	return decodeCalendarEventsJSON(body, "data")
}

// decodeCalendarEventsJSON accepts either { "economicCalendar": [...] } or a raw JSON array.
func decodeCalendarEventsJSON(body []byte, objectArrayKey string) ([]map[string]any, error) {
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		var asArr []map[string]any
		if err2 := json.Unmarshal(body, &asArr); err2 != nil {
			return nil, fmt.Errorf("calendar json: %w", err)
		}
		return asArr, nil
	}
	out, _ := sliceMap(raw, objectArrayKey)
	if len(out) > 0 {
		return out, nil
	}
	// Some error payloads are objects without the expected key — return empty, not nil decode error.
	if objectArrayKey == "earningsCalendar" {
		if alt, _ := sliceMap(raw, "data"); len(alt) > 0 {
			return alt, nil
		}
	}
	return out, nil
}

func trimAPIErr(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > 200 {
		return s[:200] + "…"
	}
	return s
}

// MarketNews fetches Finnhub market news by category: general, forex, crypto, merger.
func (c *Client) MarketNews(ctx context.Context, category string) ([]map[string]any, error) {
	if !c.HasToken() {
		return nil, fmt.Errorf("finnhub token missing")
	}
	if err := c.Limiter.Wait(ctx); err != nil {
		return nil, err
	}
	if category == "" {
		category = "general"
	}
	q := url.Values{}
	q.Set("category", category)
	q.Set("token", c.Token)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/news?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("finnhub market news: %s", resp.Status)
	}
	var items []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		return nil, err
	}
	return items, nil
}

func sliceMap(raw map[string]any, key string) ([]map[string]any, error) {
	v, ok := raw[key]
	if !ok {
		return nil, nil
	}
	arr, ok := v.([]any)
	if !ok {
		return nil, nil
	}
	out := make([]map[string]any, 0, len(arr))
	for _, x := range arr {
		if m, ok := x.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out, nil
}
