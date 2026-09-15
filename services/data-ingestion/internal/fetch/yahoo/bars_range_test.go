package yahoo

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestRetryableStatus(t *testing.T) {
	retryable := []int{429, 500, 502, 503, 504, 599}
	permanent := []int{200, 301, 400, 401, 403, 404, 410, 422}

	for _, c := range retryable {
		if !retryableStatus(c) {
			t.Errorf("status %d should be retryable", c)
		}
	}
	// 404 is the important one: a delisted or misspelled ticker will never
	// succeed, and retrying burns rate budget the rest of the universe needs.
	for _, c := range permanent {
		if retryableStatus(c) {
			t.Errorf("status %d should be permanent", c)
		}
	}
}

func TestParseRetryAfter(t *testing.T) {
	if got := parseRetryAfter(""); got != 0 {
		t.Errorf("absent header = %v, want 0", got)
	}
	if got := parseRetryAfter("30"); got != 30*time.Second {
		t.Errorf("delta-seconds = %v, want 30s", got)
	}
	if got := parseRetryAfter("garbage"); got != 0 {
		t.Errorf("unparseable = %v, want 0", got)
	}
	// Negative deltas are nonsense; treat as no advice rather than a negative wait.
	if got := parseRetryAfter("-5"); got != 0 {
		t.Errorf("negative = %v, want 0", got)
	}
	// HTTP-date form, comfortably in the future.
	future := time.Now().Add(45 * time.Second).UTC().Format(http.TimeFormat)
	if got := parseRetryAfter(future); got <= 30*time.Second || got > 46*time.Second {
		t.Errorf("http-date = %v, want ~45s", got)
	}
	// A date in the past means "retry now", not a negative duration.
	past := time.Now().Add(-time.Minute).UTC().Format(http.TimeFormat)
	if got := parseRetryAfter(past); got != 0 {
		t.Errorf("past http-date = %v, want 0", got)
	}
}

func TestBackoffDelay_ExponentialCappedAndServerOverrides(t *testing.T) {
	o := Options{BackoffBase: time.Second, BackoffMax: 10 * time.Second}

	for attempt, want := range map[int]time.Duration{
		0: 1 * time.Second,
		1: 2 * time.Second,
		2: 4 * time.Second,
		3: 8 * time.Second,
		4: 10 * time.Second, // capped
		9: 10 * time.Second, // still capped, no overflow
	} {
		if got := backoffDelay(o, attempt, 0); got != want {
			t.Errorf("attempt %d: delay = %v, want %v", attempt, got, want)
		}
	}

	// A longer Retry-After wins over the computed delay — the server knows more
	// about its own throttle window than our exponent does.
	if got := backoffDelay(o, 0, 30*time.Second); got != 30*time.Second {
		t.Errorf("Retry-After override = %v, want 30s", got)
	}
	// A shorter Retry-After does not let us retry sooner than our own backoff.
	if got := backoffDelay(o, 2, time.Millisecond); got != 4*time.Second {
		t.Errorf("short Retry-After = %v, want 4s", got)
	}
}

func TestToYahooInterval(t *testing.T) {
	for in, want := range map[string]string{
		"1Day": "1d", "1Week": "1wk", "1Month": "1mo", "1Hour": "1h",
		"1Min": "", "5Min": "", "4Hour": "", "": "", "garbage": "",
	} {
		if got := toYahooInterval(in); got != want {
			t.Errorf("toYahooInterval(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNewWithOptions_ZeroFieldsFallBackToDefaults(t *testing.T) {
	c := NewWithOptions(Options{})
	o := c.options()
	d := DefaultOptions()
	if o.RequestsPerSecond != d.RequestsPerSecond {
		t.Errorf("rate = %v, want default %v — an unthrottled client would hammer an unofficial endpoint",
			o.RequestsPerSecond, d.RequestsPerSecond)
	}
	if o.Burst != d.Burst || o.Timeout != d.Timeout ||
		o.BackoffBase != d.BackoffBase || o.BackoffMax != d.BackoffMax {
		t.Errorf("zero fields not defaulted: %+v", o)
	}
	// A Client from New() has no opts and must still report sane values.
	if got := New().options(); got.MaxRetries != d.MaxRetries {
		t.Errorf("New() client options = %+v, want defaults", got)
	}
}

// chartJSON builds a minimal but structurally faithful v8 chart response.
func chartJSON(tsAndClose [][2]int64) string {
	var ts, o, h, l, c, v []string
	for _, p := range tsAndClose {
		ts = append(ts, fmt.Sprint(p[0]))
		o = append(o, fmt.Sprint(p[1]))
		h = append(h, fmt.Sprint(p[1]+1))
		l = append(l, fmt.Sprint(p[1]-1))
		c = append(c, fmt.Sprint(p[1]))
		v = append(v, "1000000")
	}
	return fmt.Sprintf(`{"chart":{"result":[{"timestamp":[%s],"indicators":{"quote":[{"open":[%s],"high":[%s],"low":[%s],"close":[%s],"volume":[%s]}]}}],"error":null}}`,
		strings.Join(ts, ","), strings.Join(o, ","), strings.Join(h, ","),
		strings.Join(l, ","), strings.Join(c, ","), strings.Join(v, ","))
}

// testClient points a fast, retry-enabled client at a stub server.
func testClient(t *testing.T, srv *httptest.Server, maxRetries int) *Client {
	t.Helper()
	c := NewWithOptions(Options{
		RequestsPerSecond: 1000, // no throttling in tests
		Burst:             1000,
		Timeout:           5 * time.Second,
		MaxRetries:        maxRetries,
		BackoffBase:       time.Millisecond,
		BackoffMax:        2 * time.Millisecond,
	})
	c.HTTP = srv.Client()
	c.baseURL = srv.URL
	return c
}

func TestFetchBarsRange_ParsesBarsAndStampsSource(t *testing.T) {
	day := int64(1757894400) // 2025-09-15T00:00:00Z
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The range path must use explicit period bounds, not Yahoo's coarse
		// `range` buckets — that cap is why this function exists.
		q := r.URL.Query()
		if q.Get("period1") == "" || q.Get("period2") == "" {
			t.Errorf("expected period1/period2, got %v", q)
		}
		if q.Get("range") != "" {
			t.Errorf("range must not be sent on the explicit-window path: %v", q)
		}
		if q.Get("interval") != "1d" {
			t.Errorf("interval = %q, want 1d", q.Get("interval"))
		}
		if q.Get("includePrePost") != "false" {
			t.Errorf("includePrePost = %q, want false (regular session only)", q.Get("includePrePost"))
		}
		_, _ = w.Write([]byte(chartJSON([][2]int64{{day, 100}, {day + 86400, 101}})))
	}))
	defer srv.Close()

	c := testClient(t, srv, 0)
	bars, err := c.FetchBarsRange(context.Background(), "TEST", "1Day",
		time.Now().AddDate(-3, 0, 0), time.Now())
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if len(bars) != 2 {
		t.Fatalf("bars = %d, want 2", len(bars))
	}
	for _, b := range bars {
		if b.Source != SourceName {
			t.Errorf("source = %q, want %q — the scanner reads bars back by this exact value", b.Source, SourceName)
		}
		if b.Interval != "1Day" {
			t.Errorf("interval = %q, want Alpaca-format 1Day for DB consistency", b.Interval)
		}
		if b.Symbol != "TEST" {
			t.Errorf("symbol = %q", b.Symbol)
		}
	}
}

func TestFetchBarsRange_RetriesThrottlingThenSucceeds(t *testing.T) {
	var calls int32
	day := int64(1757894400)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n <= 2 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_, _ = w.Write([]byte(chartJSON([][2]int64{{day, 100}})))
	}))
	defer srv.Close()

	c := testClient(t, srv, 3)
	bars, err := c.doWithRetry(context.Background(), srv.URL, "TEST", "1Day")
	if err != nil {
		t.Fatalf("expected success after retries: %v", err)
	}
	if len(bars) != 1 {
		t.Errorf("bars = %d, want 1", len(bars))
	}
	if got := atomic.LoadInt32(&calls); got != 3 {
		t.Errorf("calls = %d, want 3 (two 429s then success)", got)
	}
}

// A 404 is a delisted or misspelled ticker. Retrying it is pure waste, and on a
// 6,000-symbol pass that waste is the difference between finishing and not.
func TestFetchBarsRange_DoesNotRetryPermanentStatus(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := testClient(t, srv, 5)
	if _, err := c.doWithRetry(context.Background(), srv.URL, "BADTICK", "1Day"); err == nil {
		t.Fatal("expected an error")
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Errorf("calls = %d, want 1 — a 404 must not be retried", got)
	}
}

func TestFetchBarsRange_ExhaustsRetriesAndReports(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	c := testClient(t, srv, 2)
	_, err := c.doWithRetry(context.Background(), srv.URL, "TEST", "1Day")
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "exhausted") {
		t.Errorf("error should say retries were exhausted, got: %v", err)
	}
	if got := atomic.LoadInt32(&calls); got != 3 {
		t.Errorf("calls = %d, want 3 (initial + 2 retries)", got)
	}
}

// An empty series is a real answer, not a failure: the backfill records such a
// symbol as done-with-zero-bars instead of retrying it every round.
func TestFetchBarsRange_EmptySeriesIsErrNoData(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"chart":{"result":[],"error":null}}`))
	}))
	defer srv.Close()

	c := testClient(t, srv, 2)

	// Drive the public entry point so the ErrNoData wrapping is covered.
	bars, err := c.FetchBarsRange(context.Background(), "TEST", "1Day",
		time.Now().AddDate(-3, 0, 0), time.Now())
	if !errors.Is(err, ErrNoData) {
		t.Fatalf("err = %v, want ErrNoData", err)
	}
	if bars != nil {
		t.Errorf("bars = %v, want nil", bars)
	}
}

func TestFetchBarsRange_RejectsBadArguments(t *testing.T) {
	c := NewWithOptions(Options{})
	now := time.Now()

	if _, err := c.FetchBarsRange(context.Background(), "T", "5Min", now.AddDate(-1, 0, 0), now); err == nil {
		t.Error("expected an error for an unsupported interval")
	}
	// An inverted or empty window is a caller bug; failing loudly beats silently
	// fetching nothing and recording the symbol as complete.
	if _, err := c.FetchBarsRange(context.Background(), "T", "1Day", now, now); err == nil {
		t.Error("expected an error for an empty window")
	}
	if _, err := c.FetchBarsRange(context.Background(), "T", "1Day", now, now.AddDate(-1, 0, 0)); err == nil {
		t.Error("expected an error for an inverted window")
	}
}

func TestFetchBarsRange_ContextCancellationStopsRetrying(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	c := NewWithOptions(Options{
		RequestsPerSecond: 1000, Burst: 1000, Timeout: time.Second,
		MaxRetries: 50, BackoffBase: 50 * time.Millisecond, BackoffMax: time.Second,
	})
	c.HTTP = srv.Client()

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Millisecond)
	defer cancel()

	start := time.Now()
	if _, err := c.doWithRetry(ctx, srv.URL, "TEST", "1Day"); err == nil {
		t.Fatal("expected an error")
	}
	// Must abandon on cancellation rather than grinding through 50 retries.
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("took %v; cancellation should abort the retry loop promptly", elapsed)
	}
}
