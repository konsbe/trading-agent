package yahoo

import (
	"context"
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

// Options configures a Client for bulk work.
//
// The default New() client is tuned for the handful of symbols data-technical
// follows. The momentum scanner's backfill walks several thousand, so the rate,
// concurrency and retry behaviour all have to be tunable from env
// (spec §2.3), and the request window has to be expressible as explicit dates
// rather than one of Yahoo's coarse `range` buckets.
type Options struct {
	// RequestsPerSecond throttles outbound requests. §2.3 budgets the backfill
	// at "~5 requests/second", but Yahoo's chart endpoint is unofficial and §2.2
	// says to rate-limit politely and treat it as best-effort, so the caller
	// picks. Values above ~5 invite sustained 429s.
	RequestsPerSecond float64

	// Burst allows short bursts above the sustained rate. 1 keeps spacing even.
	Burst int

	Timeout time.Duration

	// MaxRetries is the number of retries after the first attempt. Retries only
	// happen for transient conditions — see retryableStatus.
	MaxRetries int

	// BackoffBase is the first retry delay; it doubles each attempt. A
	// Retry-After header, when present, overrides the computed delay.
	BackoffBase time.Duration

	// BackoffMax caps the per-retry delay.
	BackoffMax time.Duration
}

// DefaultOptions mirrors the conservative behaviour of New() while enabling
// retries, which the original client did not have.
func DefaultOptions() Options {
	return Options{
		RequestsPerSecond: 0.67, // 1 request / 1.5s, matching New()
		Burst:             1,
		Timeout:           30 * time.Second,
		MaxRetries:        3,
		BackoffBase:       2 * time.Second,
		BackoffMax:        60 * time.Second,
	}
}

// NewWithOptions builds a rate- and retry-configurable client.
//
// Zero or negative fields fall back to DefaultOptions, so a partially-populated
// Options never yields an unthrottled client hammering an unofficial endpoint.
func NewWithOptions(o Options) *Client {
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
	return &Client{
		HTTP:    httpclient.New(o.Timeout),
		Limiter: rate.NewLimiter(rate.Limit(o.RequestsPerSecond), o.Burst),
		opts:    &o,
	}
}

// ErrNoData reports that Yahoo answered successfully with an empty series.
//
// Distinguished from an error so callers can tell "this symbol has no bars"
// (a delisting, a bad ticker, a brand-new listing) from "the request failed".
// The backfill records the former as a completed symbol with zero bars rather
// than retrying it forever.
//
// Aliased to the shared sentinel so errors.Is(err, barsource.ErrNoBars) matches
// regardless of which provider ran — the backfill must not name a provider.
var ErrNoData = barsource.ErrNoBars

// SourceName satisfies barsource.Fetcher.
func (c *Client) SourceName() string { return SourceName }

var _ barsource.Fetcher = (*Client)(nil)

// FetchBarsRange fetches bars for an explicit [from, to] window.
//
// This exists because the `range` parameter used by FetchBars tops out at "2y"
// for daily bars, so it cannot satisfy §8.1.3's three-year backfill. Explicit
// period1/period2 Unix bounds have no such cap and make the request window
// auditable.
//
// Bounds are inclusive of `from` and exclusive of `to`'s bar if `to` falls
// mid-session; Yahoo decides. Returns ErrNoData when the series comes back
// empty.
func (c *Client) FetchBarsRange(ctx context.Context, symbol, alpacaInterval string, from, to time.Time) ([]store.EquityBar, error) {
	yInterval := toYahooInterval(alpacaInterval)
	if yInterval == "" {
		return nil, fmt.Errorf("yahoo: unsupported interval %q", alpacaInterval)
	}
	if !to.After(from) {
		return nil, fmt.Errorf("yahoo: empty window %s..%s", from.Format(time.DateOnly), to.Format(time.DateOnly))
	}

	q := url.Values{}
	q.Set("interval", yInterval)
	q.Set("period1", strconv.FormatInt(from.UTC().Unix(), 10))
	q.Set("period2", strconv.FormatInt(to.UTC().Unix(), 10))
	q.Set("includePrePost", "false")
	// Regular session only (§3.13). Yahoo defaults to regular hours for daily
	// bars, but stating it keeps the request self-documenting.
	q.Set("events", "div,split")

	u := fmt.Sprintf("%s/%s?%s", c.base(), url.PathEscape(symbol), q.Encode())

	bars, err := c.doWithRetry(ctx, u, symbol, alpacaInterval)
	if err != nil {
		return nil, err
	}
	if len(bars) == 0 {
		return nil, ErrNoData
	}
	return bars, nil
}

// doWithRetry performs the request, retrying transient failures with
// exponential backoff. The rate limiter is waited on before every attempt,
// including retries, so a retry storm cannot exceed the configured rate.
func (c *Client) doWithRetry(ctx context.Context, u, symbol, alpacaInterval string) ([]store.EquityBar, error) {
	o := c.options()
	var lastErr error

	for attempt := 0; attempt <= o.MaxRetries; attempt++ {
		if err := c.Limiter.Wait(ctx); err != nil {
			return nil, err
		}

		bars, retryAfter, err := c.attempt(ctx, u, symbol, alpacaInterval)
		if err == nil {
			return bars, nil
		}
		lastErr = err
		if retryAfter < 0 {
			// Permanent: a 404 for a delisted ticker, a decode failure, a bad
			// interval. Retrying wastes the rate budget on a certain failure.
			return nil, err
		}
		if attempt == o.MaxRetries {
			break
		}

		delay := backoffDelay(o, attempt, retryAfter)
		t := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			t.Stop()
			return nil, ctx.Err()
		case <-t.C:
		}
	}
	return nil, fmt.Errorf("yahoo %s: exhausted %d retries: %w", symbol, o.MaxRetries, lastErr)
}

// attempt makes one request. The returned duration is the server-advised retry
// delay: >= 0 means retryable (0 = no advice given), < 0 means permanent.
func (c *Client) attempt(ctx context.Context, u, symbol, alpacaInterval string) ([]store.EquityBar, time.Duration, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, -1, err
	}
	// Minimal browser-like headers to satisfy Yahoo's CDN checks.
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64; rv:125.0) Gecko/20100101 Firefox/125.0")
	req.Header.Set("Accept", "application/json, */*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		// Transport errors (DNS, reset, timeout) are worth one more try.
		return nil, 0, fmt.Errorf("yahoo fetch %s: %w", symbol, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Drain so the connection can be reused.
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4<<10))
		if !retryableStatus(resp.StatusCode) {
			return nil, -1, fmt.Errorf("yahoo %s: HTTP %s (permanent)", symbol, resp.Status)
		}
		return nil, parseRetryAfter(resp.Header.Get("Retry-After")),
			fmt.Errorf("yahoo %s: HTTP %s", symbol, resp.Status)
	}

	bars, err := decodeChartBars(resp.Body, symbol, alpacaInterval)
	if err != nil {
		// A 200 with an unparseable body will not parse on retry either.
		return nil, -1, err
	}
	return bars, 0, nil
}

// retryableStatus reports whether an HTTP status is worth retrying.
//
// 429 is the one that matters: Yahoo throttles aggressively and a backfill will
// meet it. 5xx covers CDN hiccups. Everything else — notably 404 for a delisted
// or misspelled ticker, and 401/403 — is permanent, and retrying it burns rate
// budget that other symbols need.
func retryableStatus(code int) bool {
	if code == http.StatusTooManyRequests {
		return true
	}
	return code >= 500 && code <= 599
}

// parseRetryAfter reads a Retry-After header in either delta-seconds or HTTP
// date form. Returns 0 when absent or unparseable, meaning "retryable, no
// server advice, use the computed backoff".
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

// backoffDelay computes the wait before the next attempt: exponential from
// BackoffBase, capped at BackoffMax, unless the server advised a longer delay
// via Retry-After, which always wins.
func backoffDelay(o Options, attempt int, retryAfter time.Duration) time.Duration {
	d := time.Duration(float64(o.BackoffBase) * math.Pow(2, float64(attempt)))
	if d > o.BackoffMax {
		d = o.BackoffMax
	}
	if retryAfter > d {
		return retryAfter
	}
	return d
}

func (c *Client) options() Options {
	if c.opts != nil {
		return *c.opts
	}
	return DefaultOptions()
}

// toYahooInterval maps an Alpaca-format interval to Yahoo's, without the
// range-bucket guessing that toYahooParams has to do. Only the intervals the
// scanner uses are supported; Phase 1 is daily-only.
func toYahooInterval(alpacaInterval string) string {
	switch alpacaInterval {
	case "1Day":
		return "1d"
	case "1Week":
		return "1wk"
	case "1Month":
		return "1mo"
	case "1Hour":
		return "1h"
	default:
		return ""
	}
}
