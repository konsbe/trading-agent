package twelvedata

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/konsbe/trading-agent/services/data-ingestion/internal/fetch/barsource"
)

type noWait struct{}

func (noWait) Wait(context.Context) error { return nil }

func newTestClient(t *testing.T, h http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	c := NewWithLimiter("tok", noWait{}, Options{MaxRetries: 0, BackoffBase: time.Millisecond, BackoffMax: time.Millisecond})
	c.baseURL = srv.URL
	return c, srv
}

func day(s string) time.Time {
	t, err := time.Parse(time.DateOnly, s)
	if err != nil {
		panic(err)
	}
	return t.UTC()
}

// ─── The split fixture: volume adjustment ──────────────────────────────────────

// NVDA's 10:1 split of 2024-06-10, as Twelve Data actually returns it with
// adjust=all. Ground truth from Tiingo for the last pre-split session
// (2024-06-07): raw close 1208.88 / raw volume 41,238,580, fully adjusted
// close 120.5447 / adjusted volume 412,385,800.
const nvdaSplitBody = `{
  "meta": {"symbol": "NVDA", "interval": "1day"},
  "values": [
    {"datetime": "2024-06-06", "open": "120.00000", "high": "121.00000", "low": "119.00000", "close": "120.65106", "volume": "664696000"},
    {"datetime": "2024-06-07", "open": "120.10000", "high": "121.20000", "low": "119.50000", "close": "120.54137", "volume": "412386000"},
    {"datetime": "2024-06-10", "open": "120.90000", "high": "123.10000", "low": "117.00000", "close": "121.44078", "volume": "314162700"}
  ],
  "status": "ok"
}`

// This is the test that would have caught a provider rescaling OHLC while
// leaving volume raw — the split-contaminates-RVOL defect, reproduced on a new
// provider. It asserts the ABSOLUTE volume against Tiingo's adjusted figure
// rather than merely checking that bars parse.
//
// If volume came back unadjusted it would be ~41.2M rather than ~412.4M, and a
// 20-day average spanning 2024-06-10 would mix two share bases: §3.4's RVOL
// would read the post-split sessions as a 10x volume collapse, and §3.6 would
// see the corresponding price series as a support break. Both are
// indistinguishable from real moves once stored.
func TestFetchBarsRange_VolumeIsSplitAdjustedAcrossTheNVDASplit(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, nvdaSplitBody)
	})

	bars, err := c.FetchBarsRange(context.Background(), "NVDA", "1Day", day("2024-06-05"), day("2024-06-12"))
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if len(bars) != 3 {
		t.Fatalf("got %d bars, want 3", len(bars))
	}

	var preSplit *struct {
		close, volume float64
	}
	for _, b := range bars {
		if b.TS.Format(time.DateOnly) == "2024-06-07" {
			preSplit = &struct{ close, volume float64 }{b.Close, b.Volume}
		}
	}
	if preSplit == nil {
		t.Fatal("2024-06-07 bar missing")
	}

	// Volume must be the split-adjusted figure (10x Tiingo's raw 41,238,580),
	// not the raw one.
	const tiingoRawVolume = 41238580.0
	const tiingoAdjVolume = 412385800.0
	ratio := preSplit.volume / tiingoRawVolume
	if ratio < 9.9 || ratio > 10.1 {
		t.Errorf("volume %.0f is %.2fx Tiingo's RAW volume %.0f; want ~10x (the split factor). "+
			"A ratio near 1.0 means volume was NOT split-adjusted, which corrupts RVOL and "+
			"volume acceleration for every symbol that ever split",
			preSplit.volume, ratio, tiingoRawVolume)
	}
	if d := absPct(preSplit.volume, tiingoAdjVolume); d > 0.01 {
		t.Errorf("volume %.0f differs from Tiingo adjVolume %.0f by %.4f%%, want <0.01%%",
			preSplit.volume, tiingoAdjVolume, d)
	}

	// And price must be the dividend-adjusted figure, not the raw close.
	const tiingoRawClose = 1208.88
	const tiingoAdjClose = 120.5447288816
	if d := absPct(preSplit.close, tiingoAdjClose); d > 0.01 {
		t.Errorf("close %.5f differs from Tiingo adjClose %.5f by %.4f%%, want <0.01%%",
			preSplit.close, tiingoAdjClose, d)
	}
	if absPct(preSplit.close, tiingoRawClose) < 50 {
		t.Errorf("close %.2f is suspiciously near Tiingo's RAW close %.2f — the series looks unadjusted",
			preSplit.close, tiingoRawClose)
	}
}

func absPct(got, want float64) float64 {
	d := (got/want - 1) * 100
	if d < 0 {
		return -d
	}
	return d
}

// The adjustment is a query parameter, and omitting it yields a plausible but
// dividend-unadjusted series. Pinning it here means a refactor of the query
// builder cannot silently drop it.
func TestFetchBarsRange_AlwaysRequestsFullAdjustment(t *testing.T) {
	var got url.Values
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		got = r.URL.Query()
		fmt.Fprint(w, nvdaSplitBody)
	})
	if _, err := c.FetchBarsRange(context.Background(), "NVDA", "1Day", day("2024-06-05"), day("2024-06-12")); err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if got.Get("adjust") != "all" {
		t.Errorf("adjust=%q, want \"all\" — without it prices are split-adjusted but NOT "+
			"dividend-adjusted, the same defect that disqualified Yahoo", got.Get("adjust"))
	}
	if AdjustParam != "all" {
		t.Errorf("AdjustParam = %q, want \"all\"", AdjustParam)
	}
	if got.Get("interval") != "1day" {
		t.Errorf("interval=%q, want 1day", got.Get("interval"))
	}
	if got.Get("order") != "ASC" {
		t.Errorf("order=%q, want ASC", got.Get("order"))
	}
	if got.Get("outputsize") == "" {
		t.Error("outputsize missing; 3 years of dailies is ~753 rows and the default page is smaller")
	}
}

// ─── Parsing ───────────────────────────────────────────────────────────────────

func TestFetchBarsRange_ParsesStringEncodedNumbers(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, nvdaSplitBody)
	})
	bars, err := c.FetchBarsRange(context.Background(), "NVDA", "1Day", day("2024-06-05"), day("2024-06-12"))
	if err != nil {
		t.Fatal(err)
	}
	b := bars[0]
	if b.Source != SourceName {
		t.Errorf("source = %q, want SourceName %q", b.Source, SourceName)
	}
	if b.Interval != "1Day" {
		t.Errorf("interval = %q, want 1Day", b.Interval)
	}
	if b.Symbol != "NVDA" {
		t.Errorf("symbol = %q", b.Symbol)
	}
	if b.Open != 120.0 || b.High != 121.0 || b.Low != 119.0 {
		t.Errorf("OHL mis-parsed: %+v", b)
	}
	if !b.TS.Equal(day("2024-06-06")) {
		t.Errorf("ts = %v, want 2024-06-06 (ASC order)", b.TS)
	}
}

func TestFetchBarsRange_SkipsUnusableRowsButKeepsTheRest(t *testing.T) {
	body := `{"values":[
      {"datetime":"2024-06-06","open":"1","high":"1","low":"1","close":"1","volume":"100"},
      {"datetime":"2024-06-07","open":"1","high":"1","low":"1","close":"0","volume":"100"},
      {"datetime":"2024-06-10","open":"1","high":"1","low":"1","close":"1","volume":"0"},
      {"datetime":"2024-06-11","open":"1","high":"1","low":"1","close":"1","volume":""},
      {"datetime":"bogus","open":"1","high":"1","low":"1","close":"1","volume":"100"},
      {"datetime":"2024-06-12","open":"2","high":"2","low":"2","close":"2","volume":"200"}
    ],"status":"ok"}`
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, body) })
	bars, err := c.FetchBarsRange(context.Background(), "X", "1Day", day("2024-06-05"), day("2024-06-13"))
	if err != nil {
		t.Fatal(err)
	}
	if len(bars) != 2 {
		t.Fatalf("got %d bars, want 2 (zero close, zero volume, null volume and bad date dropped): %+v", len(bars), bars)
	}
}

// ─── Error handling ────────────────────────────────────────────────────────────

// Twelve Data reports failures in-band, often with HTTP 200. Trusting the
// transport status alone would turn an error envelope into zero bars and mark
// the symbol done with no data.
func TestFetchBarsRange_InBandErrorWithHTTP200IsNotSilentlyEmpty(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"code":401,"message":"Invalid API key","status":"error"}`)
	})
	_, err := c.FetchBarsRange(context.Background(), "X", "1Day", day("2024-06-05"), day("2024-06-12"))
	if err == nil {
		t.Fatal("want error, got nil — an error envelope must not read as an empty series")
	}
	if errors.Is(err, barsource.ErrNoBars) {
		t.Errorf("an auth failure must not be reported as ErrNoBars: %v", err)
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("error should carry the in-band code: %v", err)
	}
}

// "No data is available on the specified dates" means the window holds no bars
// for this symbol — a delisting or a pre-listing window. That is ErrNoBars, so
// the backfill marks it done rather than retrying it every round.
func TestFetchBarsRange_NoDataMessageMapsToErrNoBars(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"code":400,"message":"No data is available on the specified dates. Try setting different start/end dates.","status":"error"}`)
	})
	_, err := c.FetchBarsRange(context.Background(), "DEAD", "1Day", day("2024-06-05"), day("2024-06-12"))
	if !errors.Is(err, barsource.ErrNoBars) {
		t.Fatalf("want ErrNoBars, got %v", err)
	}
}

func TestFetchBarsRange_EmptyValuesArrayIsErrNoBars(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"values":[],"status":"ok"}`)
	})
	_, err := c.FetchBarsRange(context.Background(), "X", "1Day", day("2024-06-05"), day("2024-06-12"))
	if !errors.Is(err, barsource.ErrNoBars) {
		t.Fatalf("want ErrNoBars, got %v", err)
	}
}

// A per-minute credit cap clears on its own, so 429 must be retried — unlike
// Tiingo's hourly ceiling, where a fast retry just burns an attempt.
func TestFetchBarsRange_RetriesCreditExhaustion(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			fmt.Fprint(w, `{"code":429,"message":"You have run out of API credits for the current minute. 9 API credits were used, with the current limit being 8.","status":"error"}`)
			return
		}
		fmt.Fprint(w, nvdaSplitBody)
	}))
	defer srv.Close()

	c := NewWithLimiter("tok", noWait{}, Options{MaxRetries: 2, BackoffBase: time.Millisecond, BackoffMax: 2 * time.Millisecond})
	c.baseURL = srv.URL

	bars, err := c.FetchBarsRange(context.Background(), "NVDA", "1Day", day("2024-06-05"), day("2024-06-12"))
	if err != nil {
		t.Fatalf("429 should be retried: %v", err)
	}
	if len(bars) != 3 || calls != 2 {
		t.Errorf("bars=%d calls=%d, want 3 and 2", len(bars), calls)
	}
}

// A bad ticker is a certain failure. Retrying it across a 450-symbol sweep
// spends the daily allowance on nothing.
func TestFetchBarsRange_BadSymbolIsPermanent(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		fmt.Fprint(w, `{"code":404,"message":"**symbol** not found: BOGUS","status":"error"}`)
	}))
	defer srv.Close()
	c := NewWithLimiter("tok", noWait{}, Options{MaxRetries: 3, BackoffBase: time.Millisecond, BackoffMax: time.Millisecond})
	c.baseURL = srv.URL

	if _, err := c.FetchBarsRange(context.Background(), "BOGUS", "1Day", day("2024-06-05"), day("2024-06-12")); err == nil {
		t.Fatal("want error")
	}
	if calls != 1 {
		t.Errorf("made %d calls, want 1 — a 404 must not be retried", calls)
	}
}

// The limiter is the only thing standing between this adapter and an 8/minute
// cap, so a retry loop must pass through it on every attempt.
func TestDoWithRetry_WaitsOnLimiterForEveryAttempt(t *testing.T) {
	var waits, calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, "boom")
	}))
	defer srv.Close()

	c := NewWithLimiter("tok", countingLimiter{&waits}, Options{MaxRetries: 2, BackoffBase: time.Millisecond, BackoffMax: time.Millisecond})
	c.baseURL = srv.URL
	_, _ = c.FetchBarsRange(context.Background(), "X", "1Day", day("2024-06-05"), day("2024-06-12"))

	if waits != calls {
		t.Errorf("limiter waits = %d but HTTP calls = %d; retries must also be paced", waits, calls)
	}
	if waits != 3 {
		t.Errorf("waits = %d, want 3 (initial + 2 retries)", waits)
	}
}

type countingLimiter struct{ n *int }

func (c countingLimiter) Wait(context.Context) error { *c.n++; return nil }

// A limiter refusal (e.g. an exhausted daily ceiling) must surface unchanged so
// the caller can tell "come back tomorrow" from a transient failure.
func TestDoWithRetry_LimiterErrorIsReturnedUnchangedAndNotRetried(t *testing.T) {
	sentinel := errors.New("daily quota exhausted")
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		fmt.Fprint(w, nvdaSplitBody)
	}))
	defer srv.Close()

	c := NewWithLimiter("tok", failingLimiter{sentinel}, DefaultOptions())
	c.baseURL = srv.URL
	_, err := c.FetchBarsRange(context.Background(), "X", "1Day", day("2024-06-05"), day("2024-06-12"))
	if !errors.Is(err, sentinel) {
		t.Fatalf("want the limiter's error, got %v", err)
	}
	if calls != 0 {
		t.Errorf("made %d HTTP calls; a refused limiter must not reach the network", calls)
	}
}

type failingLimiter struct{ err error }

func (f failingLimiter) Wait(context.Context) error { return f.err }

// ─── Contract ──────────────────────────────────────────────────────────────────

func TestFetchBarsRange_RejectsNonDailyInterval(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("must not reach the network")
	})
	if _, err := c.FetchBarsRange(context.Background(), "X", "1Min", day("2024-06-05"), day("2024-06-12")); err == nil {
		t.Fatal("want error for a non-daily interval")
	}
}

func TestFetchBarsRange_RejectsEmptyWindowAndMissingToken(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("must not reach the network")
	})
	if _, err := c.FetchBarsRange(context.Background(), "X", "1Day", day("2024-06-12"), day("2024-06-05")); err == nil {
		t.Error("want error for an inverted window")
	}
	c.Token = ""
	if _, err := c.FetchBarsRange(context.Background(), "X", "1Day", day("2024-06-05"), day("2024-06-12")); err == nil {
		t.Error("want error for a missing token")
	}
}

func TestDefaultOptions_PaceStaysUnderTheMeasuredCreditCap(t *testing.T) {
	o := DefaultOptions()
	perMin := o.RequestsPerSecond * 60
	if perMin > 8 {
		t.Errorf("default pace is %.2f req/min, above the measured 8 credits/minute cap", perMin)
	}
	if perMin < 6 {
		t.Errorf("default pace is %.2f req/min, needlessly slow against an 8/minute cap", perMin)
	}
	// A minute-scoped cap needs a minute-scale backoff; two seconds just spends
	// an attempt on a refusal that has not cleared.
	if o.BackoffBase < 10*time.Second {
		t.Errorf("BackoffBase %v is too short for a per-minute credit cap", o.BackoffBase)
	}
}

func TestSourceName_MatchesWhatTheScannerReadsBack(t *testing.T) {
	if SourceName != "twelve_data" {
		t.Errorf("SourceName = %q; equity_ohlcv.source and UNIVERSE_BAR_SOURCE must agree", SourceName)
	}
}
