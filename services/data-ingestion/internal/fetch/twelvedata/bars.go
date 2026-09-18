// Package twelvedata fetches split- and dividend-adjusted daily bars from
// Twelve Data.
//
// Role in Phase 1: **NOT USED — known defective.** Retained as a working adapter
// so the defect stays reproducible and so a future fix can be verified quickly,
// not because it feeds anything.
//
// # The defect
//
// `adjust=all` applies adjustment INCONSISTENTLY, bar by bar, inside a single
// response. ABTS over a 1-for-15 reverse split returned unadjusted values on
// 2025-02-20..27, adjusted on 02-28 and 03-03, unadjusted again on 03-04..07,
// then adjusted from 03-10. Each bar is internally consistent, so nothing in the
// response looks wrong; the damage is at the seams, which fabricate one-day moves
// of +1382%, -94%, +796% and +925%.
//
// Measured scope on the 450-symbol pilot: 12-17% of symbols affected, and **41%
// of the penny-price bucket** — the population §3.2 exists to score.
//
// For a momentum scanner this is the worst possible failure mode, because a
// fabricated +796% day is exactly the signal being hunted: every corrupted
// symbol sorts to the top of the scan looking like a flawless breakout.
//
// A local jump detector cannot rescue it either. APAM, AQN and ARX disagree with
// Tiingo by 4.02%, 3.52% and 2.09% with no detectable discontinuity anywhere in
// their series, so the subset that looks clean cannot be certified clean.
//
// The fixture lives in internal/barquality; `cmd/bar-audit` reproduces the scope
// figure against stored rows. Tiingo is the primary provider — slower by 9x, but
// verified to apply one consistent adjustment factor per series.
//
// Free tier, measured against this account on 2026-09-17:
//
//	8 API credits / minute   enforced, with an explicit 429 naming the count
//	800 requests / day       published
//	no unique-symbol cap for US equities
//	3 years of daily history available (752 bars for AAPL over 3y)
//
// 8/minute would make a 450-symbol backfill ~56 minutes against Tiingo's ~9
// hours. That speed is why this provider was adopted, and the adjustment defect
// above is why it was withdrawn.
//
// # Adjustment semantics — the part that must not be guessed
//
// `adjust=all` is REQUIRED and its absence is silent. Verified against NVDA's
// 10:1 split of 2024-06-10, using Tiingo's raw/adjusted pair as ground truth for
// the last pre-split session (2024-06-07):
//
//	                         close        volume
//	Tiingo raw               1208.88      41,238,580
//	Tiingo fully adjusted     120.5447    412,385,800
//	Twelve Data default       120.888     412,386,000
//	Twelve Data adjust=all    120.5414    412,386,000
//
// Three facts follow, and only the first is obvious from the parameter's name:
//
//  1. Prices are dividend-adjusted ONLY with `adjust=all`. The default matches
//     Tiingo's *raw* close (see the ALL/AWR check in the tests: a constant
//     +6.32% and +7.52% offset over three years, i.e. cumulative dividends).
//     Omitting the flag reproduces the exact defect that disqualified Yahoo.
//
//  2. Volume IS split-adjusted, in BOTH modes — 412,386,000 against Tiingo's raw
//     41,238,580 is exactly 10.0000x. This was checked rather than assumed
//     because a provider that rescales OHLC while leaving volume raw would
//     silently corrupt §3.4's RVOL and volume acceleration across every symbol
//     that ever split, and the name "adjust=all" is not evidence.
//
//  3. `adjust=all` does not change volume, and that is CORRECT, not a bug. A
//     dividend does not alter share count, so only splits may rescale volume.
//     Volume matching between the two modes is the expected result; it would be
//     wrong if it differed.
//
// Agreement with Tiingo's fully adjusted series is -0.0028% on close and
// +0.0000% on volume, so the two providers are interchangeable in precision
// while differing enormously in throughput.
package twelvedata

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

const apiBase = "https://api.twelvedata.com"

// SourceName is the equity_ohlcv.source value every bar from this package
// carries.
const SourceName = "twelve_data"

// AdjustParam is the adjustment mode this adapter always sends.
//
// Exported and asserted in the tests: the difference between sending it and not
// is an undividend-adjusted series that looks completely plausible, so the
// constant exists to be pinned rather than trusted to a string literal buried in
// a query builder.
const AdjustParam = "all"

// ErrNoData aliases the shared sentinel so callers can check either.
var ErrNoData = barsource.ErrNoBars

// Options configures request pacing and retry.
type Options struct {
	// RequestsPerSecond paces against the measured 8 credits/minute ceiling.
	// 0.1333/s is 8/minute exactly; the default backs off slightly to 0.125/s
	// (7.5/minute) so clock skew between the limiter and Twelve Data's own
	// minute boundary cannot push a burst one request over.
	RequestsPerSecond float64
	Burst             int
	Timeout           time.Duration
	MaxRetries        int
	BackoffBase       time.Duration
	BackoffMax        time.Duration
}

func DefaultOptions() Options {
	return Options{
		RequestsPerSecond: 0.125,
		Burst:             1,
		Timeout:           30 * time.Second,
		MaxRetries:        3,
		// A minute-scoped cap needs a backoff on that order: retrying a
		// credit-exhaustion 429 two seconds later just spends another attempt on
		// a certain refusal.
		BackoffBase: 20 * time.Second,
		BackoffMax:  90 * time.Second,
	}
}

// Client fetches daily bars from Twelve Data.
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

// timeSeriesResponse mirrors Twelve Data's /time_series envelope.
//
// Errors arrive as HTTP 200 with a body of {"code":.., "message":.., "status":
// "error"} as often as with a real status code, so Status/Code/Message are
// decoded and checked rather than relying on the transport status alone.
type timeSeriesResponse struct {
	Meta struct {
		Symbol   string `json:"symbol"`
		Interval string `json:"interval"`
	} `json:"meta"`
	Values []valueRow `json:"values"`
	Status string     `json:"status"`
	Code   int        `json:"code"`
	Msg    string     `json:"message"`
}

// valueRow is one daily bar. All numbers arrive as JSON strings.
type valueRow struct {
	Datetime string `json:"datetime"`
	Open     string `json:"open"`
	High     string `json:"high"`
	Low      string `json:"low"`
	Close    string `json:"close"`
	Volume   string `json:"volume"`
}

// FetchBarsRange fetches [from, to] daily bars for one symbol.
//
// Returns ErrNoBars when Twelve Data answers with an empty series or its
// "No data is available on the specified dates" error, which it also emits for a
// single-day window even on a valid trading day.
func (c *Client) FetchBarsRange(ctx context.Context, symbol, alpacaInterval string, from, to time.Time) ([]store.EquityBar, error) {
	if !c.HasToken() {
		return nil, fmt.Errorf("twelvedata: token missing")
	}
	if alpacaInterval != "1Day" {
		// Phase 1 is daily-only (§3). Refusing loudly beats silently returning
		// daily bars labelled as something else.
		return nil, fmt.Errorf("twelvedata: unsupported interval %q (daily only)", alpacaInterval)
	}
	if !to.After(from) {
		return nil, fmt.Errorf("twelvedata: empty window %s..%s",
			from.Format(time.DateOnly), to.Format(time.DateOnly))
	}

	q := url.Values{}
	q.Set("symbol", symbol)
	q.Set("interval", "1day")
	q.Set("start_date", from.UTC().Format(time.DateOnly))
	q.Set("end_date", to.UTC().Format(time.DateOnly))
	// Without this the series is split-adjusted but NOT dividend-adjusted, which
	// violates §3 and is invisible in the response shape. See the package note.
	q.Set("adjust", AdjustParam)
	q.Set("order", "ASC")
	// 3 years of daily bars is ~753 rows; the default page is smaller.
	q.Set("outputsize", "5000")
	q.Set("apikey", c.Token)

	u := fmt.Sprintf("%s/time_series?%s", c.base(), q.Encode())

	rows, err := c.doWithRetry(ctx, u, symbol)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("%w (twelvedata %s)", barsource.ErrNoBars, symbol)
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
		return nil, fmt.Errorf("%w (twelvedata %s: %d rows, none usable)", barsource.ErrNoBars, symbol, len(rows))
	}
	return bars, nil
}

// barToStore converts one Twelve Data row.
//
// There is no adj* field set to choose between: with adjust=all the OHLC values
// ARE the adjusted series and volume is already split-adjusted, so the plain
// fields are the adjusted fields. That is the one structural difference from the
// Tiingo adapter, and the reason the package note spells out the verification —
// a reader cannot tell adjusted from unadjusted by looking at the field names.
//
// Rows with a non-positive close or zero volume are skipped, matching the Tiingo
// and Yahoo adapters so all three yield comparable series.
func barToStore(symbol, interval string, r valueRow) (store.EquityBar, bool) {
	ts, err := parseDate(r.Datetime)
	if err != nil {
		return store.EquityBar{}, false
	}
	o, ok1 := parseNum(r.Open)
	h, ok2 := parseNum(r.High)
	l, ok3 := parseNum(r.Low)
	cl, ok4 := parseNum(r.Close)
	v, ok5 := parseNum(r.Volume)
	if !ok1 || !ok2 || !ok3 || !ok4 || !ok5 {
		return store.EquityBar{}, false
	}
	if cl <= 0 || v == 0 {
		return store.EquityBar{}, false
	}
	return store.EquityBar{
		TS:       ts,
		Symbol:   symbol,
		Interval: interval,
		Open:     o,
		High:     h,
		Low:      l,
		Close:    cl,
		Volume:   v,
		Source:   SourceName,
	}, true
}

// parseNum parses one of Twelve Data's string-encoded numbers. An empty string
// is a null in their encoding, not a zero.
func parseNum(s string) (float64, bool) {
	if s == "" {
		return 0, false
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
		return 0, false
	}
	return f, true
}

// parseDate handles the "2024-06-07" form daily bars use and the
// "2024-06-07 09:30:00" form intraday intervals return.
func parseDate(s string) (time.Time, error) {
	if t, err := time.Parse(time.DateOnly, s); err == nil {
		return t.UTC(), nil
	}
	if t, err := time.Parse("2006-01-02 15:04:05", s); err == nil {
		return t.UTC(), nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UTC(), nil
	}
	return time.Time{}, fmt.Errorf("twelvedata: unparseable date %q", s)
}

// doWithRetry performs the request, retrying transient failures. The limiter is
// waited on before every attempt including retries, so a retry storm cannot
// exceed the configured pace.
func (c *Client) doWithRetry(ctx context.Context, u, symbol string) ([]valueRow, error) {
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
	return nil, fmt.Errorf("twelvedata %s: exhausted %d retries: %w", symbol, c.opts.MaxRetries, lastErr)
}

// attempt makes one request. The returned duration is the advised retry delay:
// >= 0 retryable (0 = no advice), < 0 permanent.
func (c *Client) attempt(ctx context.Context, u, symbol string) ([]valueRow, time.Duration, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, -1, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		// *url.Error carries the full request URL, and this provider
		// authenticates by query parameter — so the raw error contains the API
		// key. It must be redacted before it reaches a log or
		// universe_symbols.backfill_last_error. %w is dropped deliberately:
		// wrapping would preserve the unredacted text under errors.Unwrap.
		return nil, 0, fmt.Errorf("twelvedata fetch %s: %s", symbol, barsource.RedactSecrets(err.Error()))
	}
	defer resp.Body.Close()

	body, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<22))
	if readErr != nil {
		return nil, 0, fmt.Errorf("twelvedata read %s: %w", symbol, readErr)
	}

	if resp.StatusCode != http.StatusOK && !looksLikeJSON(body) {
		if !retryableStatus(resp.StatusCode) {
			return nil, -1, fmt.Errorf("twelvedata %s: HTTP %s (permanent): %s",
				symbol, resp.Status, trimErr(body))
		}
		return nil, parseRetryAfter(resp.Header.Get("Retry-After")),
			fmt.Errorf("twelvedata %s: HTTP %s: %s", symbol, resp.Status, trimErr(body))
	}

	var out timeSeriesResponse
	if err := json.Unmarshal(body, &out); err != nil {
		// A parseable status with an unparseable body will not parse on retry.
		return nil, -1, fmt.Errorf("twelvedata decode %s: %w", symbol, err)
	}

	// Twelve Data reports most failures in-band, frequently with HTTP 200, so the
	// envelope is authoritative over the transport status.
	if out.Status == "error" || (out.Code != 0 && out.Code != http.StatusOK) {
		code := out.Code
		if code == 0 {
			code = resp.StatusCode
		}
		if isNoData(out.Msg) {
			return nil, -1, fmt.Errorf("%w (twelvedata %s: %s)", barsource.ErrNoBars, symbol, trimErrStr(out.Msg))
		}
		if !retryableStatus(code) {
			return nil, -1, fmt.Errorf("twelvedata %s: code %d (permanent): %s",
				symbol, code, trimErrStr(out.Msg))
		}
		return nil, parseRetryAfter(resp.Header.Get("Retry-After")),
			fmt.Errorf("twelvedata %s: code %d: %s", symbol, code, trimErrStr(out.Msg))
	}

	return out.Values, 0, nil
}

func looksLikeJSON(b []byte) bool {
	for _, ch := range b {
		switch ch {
		case ' ', '\t', '\r', '\n':
			continue
		case '{', '[':
			return true
		default:
			return false
		}
	}
	return false
}

// isNoData recognises the empty-range answer, which is a legitimate "this symbol
// has no bars here" rather than a failure — a delisted ticker, or a window that
// predates the listing.
func isNoData(msg string) bool {
	return containsFold(msg, "no data is available")
}

func containsFold(hay, needle string) bool {
	if len(needle) > len(hay) {
		return false
	}
	lower := func(b byte) byte {
		if b >= 'A' && b <= 'Z' {
			return b + 32
		}
		return b
	}
	for i := 0; i+len(needle) <= len(hay); i++ {
		match := true
		for j := 0; j < len(needle); j++ {
			if lower(hay[i+j]) != lower(needle[j]) {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// retryableStatus reports whether a status is worth retrying.
//
// 429 is retryable because Twelve Data's is a per-MINUTE credit cap that clears
// on its own, unlike Tiingo's hourly ceiling. 400/401/403/404 are permanent: a
// bad symbol or token will not fix itself, and across a 450-symbol sweep
// retrying certain failures wastes the daily allowance.
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

func trimErr(b []byte) string { return trimErrStr(string(b)) }

func trimErrStr(s string) string {
	if len(s) > 180 {
		return s[:180] + "…"
	}
	return s
}

// compile-time check that the client satisfies the provider contract.
var _ barsource.Fetcher = (*Client)(nil)

// ensure errors.Is works against both sentinels.
var _ = errors.Is(ErrNoData, barsource.ErrNoBars)
