package tiingo

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/konsbe/trading-agent/services/data-ingestion/internal/fetch/barsource"
)

func testClient(t *testing.T, srv *httptest.Server, retries int) *Client {
	t.Helper()
	c := NewWithLimiter("tok", nil, Options{
		RequestsPerSecond: 1000, Burst: 1000, Timeout: 5 * time.Second,
		MaxRetries: retries, BackoffBase: time.Millisecond, BackoffMax: 2 * time.Millisecond,
	})
	c.HTTP = srv.Client()
	c.baseURL = srv.URL
	return c
}

func fetch(t *testing.T, c *Client) ([]struct{}, error) {
	t.Helper()
	_, err := c.FetchBarsRange(context.Background(), "AAPL", "1Day",
		time.Now().AddDate(-1, 0, 0), time.Now())
	return nil, err
}

// The response shape is real, captured from the live API.
const realBody = `[
 {"date":"2026-09-08T00:00:00.000Z","close":316.22,"high":320.7,"low":314.9,"open":317.1,"volume":35477090,
  "adjClose":158.11,"adjHigh":160.35,"adjLow":157.45,"adjOpen":158.55,"adjVolume":70954180,"divCash":0.0,"splitFactor":2.0},
 {"date":"2026-09-09T00:00:00.000Z","close":315.34,"high":319.15,"low":309.9,"open":315.485,"volume":65639962,
  "adjClose":315.34,"adjHigh":319.15,"adjLow":309.9,"adjOpen":315.485,"adjVolume":65639962,"divCash":0.0,"splitFactor":1.0}
]`

// §3's opening line requires split/dividend-adjusted bars. The fixture's first
// row has a 2:1 split, so raw and adjusted differ by exactly 2× — if the adapter
// read the raw fields the test fails on a specific, meaningful number rather than
// on a formatting difference.
func TestFetchBarsRange_UsesAdjustedFieldsNotRaw(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("startDate") == "" || q.Get("endDate") == "" {
			t.Errorf("expected explicit startDate/endDate, got %v", q)
		}
		// The token must travel in the Authorization header and must NOT appear
		// in the URL: *url.Error embeds the full request URL, so a credential in
		// the query string ends up in error text, logs, and
		// universe_symbols.backfill_last_error.
		if got := r.Header.Get("Authorization"); got != "Token tok" {
			t.Errorf("Authorization = %q, want \"Token tok\"", got)
		}
		if q.Get("token") != "" {
			t.Errorf("token leaked into the query string (%q); it belongs in the Authorization header", q.Get("token"))
		}
		_, _ = w.Write([]byte(realBody))
	}))
	defer srv.Close()

	bars, err := testClient(t, srv, 0).FetchBarsRange(context.Background(), "AAPL", "1Day",
		time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC), time.Date(2026, 9, 9, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if len(bars) != 2 {
		t.Fatalf("bars = %d, want 2", len(bars))
	}

	// Adjusted close 158.11, NOT the raw 316.22.
	if bars[0].Close != 158.11 {
		t.Errorf("close = %v, want the ADJUSTED 158.11 (raw is 316.22) — unadjusted bars break every windowed feature across a split", bars[0].Close)
	}
	// Adjusted volume 70,954,180, NOT the raw 35,477,090. Using raw volume would
	// make §3.4's 20-day average mix two different share bases across the split.
	if bars[0].Volume != 70954180 {
		t.Errorf("volume = %v, want the ADJUSTED 70954180 (raw is 35477090)", bars[0].Volume)
	}
	// Pinned against the literal as well as the constant, so renaming the
	// constant cannot silently change what lands in equity_ohlcv.source.
	if bars[0].Source != SourceName {
		t.Errorf("source = %q, want SourceName", bars[0].Source)
	}
	if SourceName != "tiingo" {
		t.Errorf("SourceName = %q, want tiingo — the scanner filters bars by this exact value", SourceName)
	}
	if bars[0].Interval != "1Day" {
		t.Errorf("interval = %q, want 1Day for DB consistency", bars[0].Interval)
	}
	// Oldest first, matching every other adapter and what compute expects.
	if !bars[0].TS.Before(bars[1].TS) {
		t.Errorf("bars not oldest-first: %v then %v", bars[0].TS, bars[1].TS)
	}
}

// Migration 012 / Phase 2 §3.2: the UNADJUSTED close and splitFactor must be
// captured alongside the adjusted series, without disturbing it.
//
// The fixture's first row is a 2:1 split, so raw (316.22) and adjusted (158.11)
// differ by exactly 2×. That makes the two assertions independent: reading the
// wrong field for either one fails on a specific number rather than by accident.
func TestFetchBarsRange_CapturesUnadjustedCloseAndSplitFactor(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(realBody))
	}))
	defer srv.Close()

	bars, err := testClient(t, srv, 0).FetchBarsRange(context.Background(), "AAPL", "1Day",
		time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC), time.Date(2026, 9, 9, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}

	if bars[0].RawClose == nil {
		t.Fatal("RawClose is nil — Phase 2 §3.2 cannot compute point-in-time market cap without it, and the field is right there in the same response")
	}
	if *bars[0].RawClose != 316.22 {
		t.Errorf("RawClose = %v, want the UNADJUSTED 316.22 (adjusted is 158.11)", *bars[0].RawClose)
	}
	if bars[0].SplitFactor == nil || *bars[0].SplitFactor != 2.0 {
		t.Errorf("SplitFactor = %v, want 2.0 for the 2:1 split row", bars[0].SplitFactor)
	}

	// The whole point of the pointer types: the ordinary second row still
	// carries 1.0, so "no split" is a stored fact, not an absence.
	if bars[1].SplitFactor == nil || *bars[1].SplitFactor != 1.0 {
		t.Errorf("SplitFactor[1] = %v, want 1.0 — an ordinary session records 1.0, it does not record nothing", bars[1].SplitFactor)
	}

	// Capturing the raw fields must not have disturbed the adjusted ones. This
	// is the regression that would matter: every feature reads Close.
	if bars[0].Close != 158.11 {
		t.Errorf("Close = %v, want the ADJUSTED 158.11 — adding raw capture must not change what features read", bars[0].Close)
	}
}

// A zero or absent splitFactor must store NULL rather than 0.0. A zero ratio is
// not a corporate action anyone can express, so treating it as data would put a
// number into §3.2 that means "Tiingo omitted the field".
func TestFetchBarsRange_AbsentUnadjustedFieldsStoreNullNotZero(t *testing.T) {
	const noRawBody = `[
	 {"date":"2026-09-08T00:00:00.000Z","close":0,"high":320.7,"low":314.9,"open":317.1,"volume":35477090,
	  "adjClose":158.11,"adjHigh":160.35,"adjLow":157.45,"adjOpen":158.55,"adjVolume":70954180,"divCash":0.0,"splitFactor":0}
	]`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(noRawBody))
	}))
	defer srv.Close()

	bars, err := testClient(t, srv, 0).FetchBarsRange(context.Background(), "AAPL", "1Day",
		time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC), time.Date(2026, 9, 9, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if len(bars) != 1 {
		t.Fatalf("bars = %d, want 1 — the row is usable on its adjusted fields and must not be dropped for missing raw ones", len(bars))
	}
	if bars[0].RawClose != nil {
		t.Errorf("RawClose = %v, want nil — a zero raw close is missing data, and storing 0.0 would make §3.2 compute a market cap of zero rather than skip the row", *bars[0].RawClose)
	}
	if bars[0].SplitFactor != nil {
		t.Errorf("SplitFactor = %v, want nil for an omitted field", *bars[0].SplitFactor)
	}
	// The row itself still survives on its adjusted fields.
	if bars[0].Close != 158.11 {
		t.Errorf("Close = %v, want 158.11", bars[0].Close)
	}
}

// A 404 is Tiingo's answer for an unknown or delisted ticker. Retrying it across
// a multi-thousand-symbol sweep spends quota on a certain failure.
func TestFetchBarsRange_NotFoundIsPermanent(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"detail":"Error: Ticker 'ZZZZ' not found"}`))
	}))
	defer srv.Close()

	if _, err := fetch(t, testClient(t, srv, 5)); err == nil {
		t.Fatal("expected an error")
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Errorf("calls = %d, want 1 — a 404 must not be retried", got)
	}
}

func TestFetchBarsRange_RetriesThrottlingThenSucceeds(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) <= 2 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_, _ = w.Write([]byte(realBody))
	}))
	defer srv.Close()

	bars, err := testClient(t, srv, 3).FetchBarsRange(context.Background(), "AAPL", "1Day",
		time.Now().AddDate(-1, 0, 0), time.Now())
	if err != nil {
		t.Fatalf("expected success after retries: %v", err)
	}
	if len(bars) != 2 {
		t.Errorf("bars = %d, want 2", len(bars))
	}
	if got := atomic.LoadInt32(&calls); got != 3 {
		t.Errorf("calls = %d, want 3", got)
	}
}

// An empty array is a real answer, and it must map to the SHARED sentinel so the
// provider-agnostic backfill can recognise it without naming Tiingo.
func TestFetchBarsRange_EmptyArrayIsSharedErrNoBars(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	_, err := fetch(t, testClient(t, srv, 2))
	if !errors.Is(err, barsource.ErrNoBars) {
		t.Errorf("err = %v, want barsource.ErrNoBars", err)
	}
	if !errors.Is(err, ErrNoData) {
		t.Errorf("err = %v, want the package alias to match too", err)
	}
}

// Rows that cannot yield a usable bar must not silently become a zero-volume or
// zero-price bar — that would feed §3 a fabricated candle.
func TestFetchBarsRange_UnusableRowsAreSkippedNotFabricated(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[
 {"date":"2026-09-08T00:00:00.000Z","adjClose":0,"adjHigh":0,"adjLow":0,"adjOpen":0,"adjVolume":0},
 {"date":"bad-date","adjClose":10,"adjHigh":11,"adjLow":9,"adjOpen":10,"adjVolume":1000},
 {"date":"2026-09-10T00:00:00.000Z","adjClose":10,"adjHigh":11,"adjLow":9,"adjOpen":10,"adjVolume":1000}
]`))
	}))
	defer srv.Close()

	bars, err := testClient(t, srv, 0).FetchBarsRange(context.Background(), "X", "1Day",
		time.Now().AddDate(-1, 0, 0), time.Now())
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if len(bars) != 1 {
		t.Fatalf("bars = %d, want 1 (zero-volume and bad-date rows skipped)", len(bars))
	}
	if bars[0].Close != 10 {
		t.Errorf("close = %v, want 10", bars[0].Close)
	}
}

// If every row is unusable the result is indistinguishable from empty, and must
// report as such rather than as success-with-no-bars.
func TestFetchBarsRange_AllRowsUnusableIsErrNoBars(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"date":"2026-09-08T00:00:00.000Z","adjClose":0,"adjVolume":0}]`))
	}))
	defer srv.Close()
	if _, err := fetch(t, testClient(t, srv, 0)); !errors.Is(err, barsource.ErrNoBars) {
		t.Errorf("err = %v, want ErrNoBars", err)
	}
}

func TestFetchBarsRange_RejectsBadArguments(t *testing.T) {
	c := New("tok")
	now := time.Now()
	// Phase 1 is daily-only; anything else must fail loudly rather than return
	// daily bars mislabelled as another interval.
	if _, err := c.FetchBarsRange(context.Background(), "A", "1Hour", now.AddDate(-1, 0, 0), now); err == nil {
		t.Error("expected an error for a non-daily interval")
	}
	if _, err := c.FetchBarsRange(context.Background(), "A", "1Day", now, now); err == nil {
		t.Error("expected an error for an empty window")
	}
	if _, err := NewWithLimiter("", nil, Options{}).FetchBarsRange(
		context.Background(), "A", "1Day", now.AddDate(-1, 0, 0), now); err == nil {
		t.Error("expected an error for a missing token")
	}
}

// A limiter error — including ratelimit.ErrDailyQuotaExhausted — must propagate
// unchanged and must not be retried. Retrying a certain refusal burns attempts.
type refusingLimiter struct {
	err   error
	calls atomic.Int32
}

func (r *refusingLimiter) Wait(context.Context) error { r.calls.Add(1); return r.err }

func TestFetchBarsRange_LimiterErrorPropagatesAndIsNotRetried(t *testing.T) {
	sentinel := errors.New("ratelimit: daily quota exhausted")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("no HTTP request should be made when the limiter refuses")
	}))
	defer srv.Close()

	lim := &refusingLimiter{err: sentinel}
	c := NewWithLimiter("tok", lim, Options{MaxRetries: 5, BackoffBase: time.Millisecond})
	c.HTTP = srv.Client()
	c.baseURL = srv.URL

	_, err := c.FetchBarsRange(context.Background(), "A", "1Day",
		time.Now().AddDate(-1, 0, 0), time.Now())
	if !errors.Is(err, sentinel) {
		t.Errorf("err = %v, want the limiter's error unchanged", err)
	}
	if got := lim.calls.Load(); got != 1 {
		t.Errorf("limiter consulted %d times, want 1 — a refusal must not be retried", got)
	}
}

func TestClientSatisfiesFetcherAndReportsSource(t *testing.T) {
	var f barsource.Fetcher = New("tok")
	if f.SourceName() != "tiingo" {
		t.Errorf("SourceName = %q, want tiingo", f.SourceName())
	}
}

// Share-class tickers are spelled differently by the two providers: Finnhub and
// universe_symbols use a dot (BRK.B), Tiingo uses a hyphen (BRK-B). Verified
// against the live API — the dotted form returns
// 404 {"detail":"Error: Ticker 'BRK.B' not found"}.
//
// Without the translation these fail as PERMANENT 404s, so they are never
// retried and drop out of the universe silently. 20 of the 4,975 eligible
// symbols are affected, including BRK.B, BF.B and HEI.A.
func TestFetchBarsRange_TranslatesShareClassTickersForTheRequest(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(realBody))
	}))
	defer srv.Close()

	bars, err := testClient(t, srv, 0).FetchBarsRange(context.Background(), "BRK.B", "1Day",
		time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if !strings.Contains(gotPath, "BRK-B") {
		t.Errorf("request path %q should use Tiingo's hyphen spelling BRK-B", gotPath)
	}
	if strings.Contains(gotPath, "BRK.B") {
		t.Errorf("request path %q still carries the dotted spelling, which 404s", gotPath)
	}

	// Stored rows must keep the CANONICAL dotted symbol: equity_ohlcv.symbol
	// joins universe_symbols, and a second spelling nothing else knows about
	// would be worse than the 404 it fixes.
	for _, b := range bars {
		if b.Symbol != "BRK.B" {
			t.Errorf("stored symbol = %q, want the canonical BRK.B", b.Symbol)
		}
	}
}

func TestProviderSymbol_OnlyRewritesDots(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"AAPL", "AAPL"},
		{"BRK.B", "BRK-B"},
		{"BF.A", "BF-A"},
		{"BRK-B", "BRK-B"}, // already hyphenated, left alone
	} {
		if got := providerSymbol(c.in); got != c.want {
			t.Errorf("providerSymbol(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
