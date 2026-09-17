// Package tiingo fetches split/dividend-adjusted daily bars from Tiingo.
//
// Role in Phase 1: the **pilot subset** provider. Tiingo's free allowance is 500
// UNIQUE SYMBOLS PER MONTH, and re-reading an already-counted symbol is free —
// which makes it ideal for a fixed ~450-symbol subset refreshed daily, and
// unusable for a one-off sweep of thousands. The full-universe backfill uses
// Twelve Data instead. See data_ingestion.md for why both exist.
//
// That quota shape is why this package has no daily ceiling and no monthly
// counter: "500 distinct symbols per month" is not a rate and not a daily count,
// so internal/ratelimit structurally cannot express it. Enforcement lives in the
// subset-selection job's size assertion. The limiter here is politeness only.
package tiingo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/konsbe/trading-agent/services/data-ingestion/internal/fetch/barsource"
	"github.com/konsbe/trading-agent/services/data-ingestion/internal/httpclient"
	"github.com/konsbe/trading-agent/services/data-ingestion/internal/store"
	"golang.org/x/time/rate"
)

const apiBase = "https://api.tiingo.com/tiingo/daily"

// SourceName is the equity_ohlcv.source value every bar from this package
// carries.
const SourceName = "tiingo"

// ErrNoData aliases the shared sentinel so callers can check either.
var ErrNoData = barsource.ErrNoBars

// Options configures request pacing and retry.
type Options struct {
	// RequestsPerSecond is politeness pacing, not quota enforcement — Tiingo's
	// constraint is monthly unique symbols, not a rate. 1.5/s was observed
	// without throttling across a 12-symbol burst.
	RequestsPerSecond float64
	Burst             int
	Timeout           time.Duration
	MaxRetries        int
	BackoffBase       time.Duration
	BackoffMax        time.Duration
}

func DefaultOptions() Options {
	return Options{
		RequestsPerSecond: 1.5,
		Burst:             2,
		Timeout:           30 * time.Second,
		MaxRetries:        3,
		BackoffBase:       2 * time.Second,
		BackoffMax:        60 * time.Second,
	}
}

// Client fetches daily bars from Tiingo.
type Client struct {
	Token   string
	HTTP    *http.Client
	Limiter barsource.Limiter

	opts    Options
	baseURL string // test override; empty means apiBase
}

// New builds a client with its own in-process limiter.
func New(token string) *Client {
	return NewWithLimiter(token, nil, DefaultOptions())
}

// NewWithLimiter builds a client pacing against the supplied limiter, normally
// the cross-process shared budget.
//
// A nil limiter yields a private in-process bucket at the configured rate, so a
// caller unable to build a shared limiter still paces rather than running free.
func NewWithLimiter(token string, l barsource.Limiter, o Options) *Client {
	d := DefaultOptions()
	if o.RequestsPerSecond <= 0 {
		o.RequestsPerSecond = d.RequestsPerSecond
	}
	if o.Burst <= 0 {
		o.Burst = d.Burst
	}
	if o.Timeout <= 0 {
		o.Timeout = d.Timeout
	}
	if o.MaxRetries < 0 {
		o.MaxRetries = d.MaxRetries
	}
	if o.BackoffBase <= 0 {
		o.BackoffBase = d.BackoffBase
	}
	if o.BackoffMax <= 0 {
		o.BackoffMax = d.BackoffMax
	}
	if l == nil {
		l = rate.NewLimiter(rate.Limit(o.RequestsPerSecond), o.Burst)
	}
	return &Client{
		Token:   token,
		HTTP:    httpclient.New(o.Timeout),
		Limiter: l,
		opts:    o,
	}
}

func (c *Client) HasToken() bool     { return c.Token != "" }
func (c *Client) SourceName() string { return SourceName }
func (c *Client) base() string {
	if c.baseURL != "" {
		return c.baseURL
	}
	return apiBase
}

// priceRow mirrors one element of Tiingo's /prices response.
//
// Both raw and adjusted fields are returned; this adapter uses the ADJUSTED set
// exclusively — see the note in barToStore.
type priceRow struct {
	Date        string  `json:"date"`
	Open        float64 `json:"open"`
	High        float64 `json:"high"`
	Low         float64 `json:"low"`
	Close       float64 `json:"close"`
	Volume      float64 `json:"volume"`
	AdjOpen     float64 `json:"adjOpen"`
	AdjHigh     float64 `json:"adjHigh"`
	AdjLow      float64 `json:"adjLow"`
	AdjClose    float64 `json:"adjClose"`
	AdjVolume   float64 `json:"adjVolume"`
	DivCash     float64 `json:"divCash"`
	SplitFactor float64 `json:"splitFactor"`
}

// FetchBarsRange fetches [from, to] daily bars for one symbol.
//
// Returns ErrNoBars when Tiingo answers successfully with an empty array, and a
// permanent error for a 404 (Tiingo's response for an unknown or delisted
// ticker, `{"detail":"Error: Ticker 'X' not found"}`).
func (c *Client) FetchBarsRange(ctx context.Context, symbol, alpacaInterval string, from, to time.Time) ([]store.EquityBar, error) {
	if !c.HasToken() {
		return nil, fmt.Errorf("tiingo: token missing")
	}
	if alpacaInterval != "1Day" {
		// Phase 1 is daily-only (§3). Refusing loudly beats silently returning
		// daily bars labelled as something else.
		return nil, fmt.Errorf("tiingo: unsupported interval %q (daily only)", alpacaInterval)
	}
	if !to.After(from) {
		return nil, fmt.Errorf("tiingo: empty window %s..%s",
			from.Format(time.DateOnly), to.Format(time.DateOnly))
	}

	q := url.Values{}
	q.Set("startDate", from.UTC().Format(time.DateOnly))
	q.Set("endDate", to.UTC().Format(time.DateOnly))
	q.Set("format", "json")
	q.Set("resampleFreq", "daily")
	q.Set("token", c.Token)

	u := fmt.Sprintf("%s/%s/prices?%s", c.base(), url.PathEscape(symbol), q.Encode())

	rows, err := c.doWithRetry(ctx, u, symbol)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("%w (tiingo %s)", barsource.ErrNoBars, symbol)
	}

	bars := make([]store.EquityBar, 0, len(rows))
	for _, r := range rows {
		b, ok := barToStore(symbol, alpacaInterval, r)
		if !ok {
			continue
		}
		bars = append(bars, b)
	}
	if len(bars) == 0 {
		// Every row was unusable (nulls, zero volume). Same meaning as an empty
		// response for the caller's purposes.
		return nil, fmt.Errorf("%w (tiingo %s: %d rows, none usable)", barsource.ErrNoBars, symbol, len(rows))
	}
	return bars, nil
}

// barToStore converts one Tiingo row, using the ADJUSTED fields.
//
// §3's opening line requires "split/dividend-adjusted" bars, and the adjustment
// has to be consistent across the whole window or windowed features break at
// every corporate action. A split would otherwise halve the price and double the
// volume mid-window, which §3.4's RVOL reads as a 2× volume surge and §3.6 reads
// as a support break — both indistinguishable from real moves.
//
// adjVolume is used for the same reason: raw volume is not rescaled across a
// split, so a 20-day average spanning one would mix two different share bases.
//
// Rows with a non-positive close or zero volume are skipped, matching the Yahoo
// adapter's filtering so the two providers yield comparable series.
func barToStore(symbol, interval string, r priceRow) (store.EquityBar, bool) {
	ts, err := parseTiingoDate(r.Date)
	if err != nil {
		return store.EquityBar{}, false
	}
	o, h, l, c, v := r.AdjOpen, r.AdjHigh, r.AdjLow, r.AdjClose, r.AdjVolume
	if isBad(o) || isBad(h) || isBad(l) || isBad(c) || isBad(v) || c <= 0 || v == 0 {
		return store.EquityBar{}, false
	}
	return store.EquityBar{
		TS:       ts,
		Symbol:   symbol,
		Interval: interval,
		Open:     o,
		High:     h,
		Low:      l,
		Close:    c,
		Volume:   v,
		Source:   SourceName,
	}, true
}

func isBad(f float64) bool { return math.IsNaN(f) || math.IsInf(f, 0) }

// parseTiingoDate handles the RFC3339 form Tiingo returns
// ("2026-09-08T00:00:00.000Z") and tolerates a bare date.
func parseTiingoDate(s string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UTC(), nil
	}
	if t, err := time.Parse(time.DateOnly, s); err == nil {
		return t.UTC(), nil
	}
	return time.Time{}, fmt.Errorf("tiingo: unparseable date %q", s)
}

// doWithRetry performs the request, retrying transient failures. The limiter is
// waited on before every attempt including retries, so a retry storm cannot
// exceed the configured pace.
func (c *Client) doWithRetry(ctx context.Context, u, symbol string) ([]priceRow, error) {
	var lastErr error
	for attempt := 0; attempt <= c.opts.MaxRetries; attempt++ {
		if err := c.Limiter.Wait(ctx); err != nil {
			// Includes ratelimit.ErrDailyQuotaExhausted when a shared budget with
			// a daily ceiling is wired in. Returned unchanged so the caller can
			// distinguish "come back tomorrow" from a transient failure — never
			// retried here, which would burn attempts against a certain refusal.
			return nil, err
		}

		rows, retryAfter, err := c.attempt(ctx, u, symbol)
		if err == nil {
			return rows, nil
		}
		lastErr = err
		if retryAfter < 0 {
			return nil, err // permanent
		}
		if attempt == c.opts.MaxRetries {
			break
		}
		d := time.Duration(float64(c.opts.BackoffBase) * math.Pow(2, float64(attempt)))
		if d > c.opts.BackoffMax {
			d = c.opts.BackoffMax
		}
		if retryAfter > d {
			d = retryAfter
		}
		t := time.NewTimer(d)
		select {
		case <-ctx.Done():
			t.Stop()
			return nil, ctx.Err()
		case <-t.C:
		}
	}
	return nil, fmt.Errorf("tiingo %s: exhausted %d retries: %w", symbol, c.opts.MaxRetries, lastErr)
}

// attempt makes one request. The returned duration is the advised retry delay:
// >= 0 retryable (0 = no advice), < 0 permanent.
func (c *Client) attempt(ctx context.Context, u, symbol string) ([]priceRow, time.Duration, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, -1, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("tiingo fetch %s: %w", symbol, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<10))
		if !retryableStatus(resp.StatusCode) {
			return nil, -1, fmt.Errorf("tiingo %s: HTTP %s (permanent): %s",
				symbol, resp.Status, trimErr(body))
		}
		return nil, parseRetryAfter(resp.Header.Get("Retry-After")),
			fmt.Errorf("tiingo %s: HTTP %s: %s", symbol, resp.Status, trimErr(body))
	}

	var rows []priceRow
	if err := json.NewDecoder(resp.Body).Decode(&rows); err != nil {
		// A 200 with an unparseable body will not parse on retry either.
		return nil, -1, fmt.Errorf("tiingo decode %s: %w", symbol, err)
	}
	return rows, 0, nil
}

// retryableStatus reports whether a status is worth retrying.
//
// 404 is explicitly permanent: it is Tiingo's answer for an unknown or delisted
// ticker, and across a multi-thousand-symbol sweep retrying those would spend a
// meaningful slice of the quota on certain failures. 401/403 are permanent for
// the same reason — a bad token will not fix itself.
func retryableStatus(code int) bool {
	if code == http.StatusTooManyRequests {
		return true
	}
	return code >= 500 && code <= 599
}

func parseRetryAfter(v string) time.Duration {
	if v == "" {
		return 0
	}
	if secs, err := strconv.Atoi(v); err == nil {
		if secs < 0 {
			return 0
		}
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(v); err == nil {
		if d := time.Until(t); d > 0 {
			return d
		}
	}
	return 0
}

func trimErr(b []byte) string {
	s := string(b)
	if len(s) > 180 {
		return s[:180] + "…"
	}
	return s
}

// compile-time check that the client satisfies the provider contract.
var _ barsource.Fetcher = (*Client)(nil)

// ensure errors.Is works against both sentinels.
var _ = errors.Is(ErrNoData, barsource.ErrNoBars)
