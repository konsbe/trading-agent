package momentumapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/konsbe/trading-agent/services/data-analyzer/internal/store"
)

// fakeStore records calls and returns canned data.
type fakeStore struct {
	scanDate   time.Time
	hasScan    bool
	summary    store.ScanSummary
	candidates []store.CandidateRow
	details    map[string]store.SymbolDetailRow

	queryErr error
	pingErr  error

	calls        int
	detailLookup []string

	daily         map[string][]store.PriceBar
	intraday      map[string][]store.PriceBar
	dailyFrom     []time.Time
	intradayAsked []string
	known         map[string]bool
	watchlist     []store.WatchlistItem
	owners        []*string

	tracked       []store.TrackedPositionRow
	trackedCounts store.TrackedCounts
	trackedAsked  []string
	trackedLatest []*time.Time
}

func (f *fakeStore) LatestScanDate(context.Context) (time.Time, bool, error) {
	f.calls++
	if f.queryErr != nil {
		return time.Time{}, false, f.queryErr
	}
	return f.scanDate, f.hasScan, nil
}
func (f *fakeStore) ScanSummary(context.Context, time.Time) (store.ScanSummary, error) {
	return f.summary, f.queryErr
}
func (f *fakeStore) Candidates(context.Context, time.Time) ([]store.CandidateRow, error) {
	return f.candidates, f.queryErr
}
func (f *fakeStore) SymbolDetail(_ context.Context, _ time.Time, sym string) (store.SymbolDetailRow, bool, error) {
	f.detailLookup = append(f.detailLookup, sym)
	d, ok := f.details[sym]
	return d, ok, f.queryErr
}
func (f *fakeStore) Ping(context.Context) error { return f.pingErr }

func (f *fakeStore) TrackedPositions(_ context.Context, st store.TrackedStatusFilter, latest *time.Time) ([]store.TrackedPositionRow, error) {
	f.trackedAsked = append(f.trackedAsked, string(st))
	f.trackedLatest = append(f.trackedLatest, latest)
	var out []store.TrackedPositionRow
	for _, r := range f.tracked {
		if st == "all" || r.Status == string(st) {
			out = append(out, r)
		}
	}
	return out, f.queryErr
}
func (f *fakeStore) TrackedCounts(context.Context) (store.TrackedCounts, error) {
	return f.trackedCounts, f.queryErr
}

func (f *fakeStore) LatestDailyBarTS(_ context.Context, sym string) (time.Time, bool, error) {
	bars := f.daily[sym]
	if len(bars) == 0 {
		return time.Time{}, false, f.queryErr
	}
	return bars[len(bars)-1].TS, true, f.queryErr
}
func (f *fakeStore) DailyBars(_ context.Context, sym string, from time.Time) ([]store.PriceBar, error) {
	f.dailyFrom = append(f.dailyFrom, from)
	var out []store.PriceBar
	for _, b := range f.daily[sym] {
		if !b.TS.Before(from) {
			out = append(out, b)
		}
	}
	return out, f.queryErr
}
func (f *fakeStore) IntradayBars(_ context.Context, sym, interval string, sessions int) ([]store.PriceBar, error) {
	f.intradayAsked = append(f.intradayAsked, fmt.Sprintf("%s:%s:%d", sym, interval, sessions))
	return f.intraday[sym], f.queryErr
}
func (f *fakeStore) SymbolKnown(_ context.Context, sym string) (bool, error) {
	return f.known[sym], f.queryErr
}
func (f *fakeStore) ListWatchlist(_ context.Context, owner *string) ([]store.WatchlistItem, error) {
	f.owners = append(f.owners, owner)
	return f.watchlist, f.queryErr
}
func (f *fakeStore) AddToWatchlist(_ context.Context, owner *string, sym string) (bool, error) {
	f.owners = append(f.owners, owner)
	for _, it := range f.watchlist {
		if it.Symbol == sym {
			return false, f.queryErr
		}
	}
	f.watchlist = append([]store.WatchlistItem{{Symbol: sym, AddedAt: scanDay}}, f.watchlist...)
	return true, f.queryErr
}
func (f *fakeStore) RemoveFromWatchlist(_ context.Context, owner *string, sym string) (bool, error) {
	f.owners = append(f.owners, owner)
	for i, it := range f.watchlist {
		if it.Symbol == sym {
			f.watchlist = append(f.watchlist[:i], f.watchlist[i+1:]...)
			return true, f.queryErr
		}
	}
	return false, f.queryErr
}

func ptr[T any](v T) *T { return &v }

// sharedCaveatsPath locates shared/content/momentum_caveats.json from this file.
func sharedCaveatsPath(t *testing.T) string {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "shared", "content", "momentum_caveats.json")
}

func loadSharedCaveats(t *testing.T) Caveats {
	t.Helper()
	c, err := LoadCaveats(sharedCaveatsPath(t))
	if err != nil {
		t.Fatalf("load shared caveats: %v", err)
	}
	return c
}

var scanDay = time.Date(2026, 9, 17, 0, 0, 0, 0, time.UTC)

func fixtureStore() *fakeStore {
	return &fakeStore{
		scanDate: scanDay,
		hasScan:  true,
		summary: store.ScanSummary{
			Date:             scanDay,
			CompletedAt:      time.Date(2026, 9, 18, 12, 43, 3, 0, time.UTC),
			UniverseScanned:  4971,
			UniverseEligible: 4975,
		},
		candidates: []store.CandidateRow{
			{Symbol: "NEXR", Exchange: ptr("NASDAQ"), CompanyName: ptr("Nexien Inc"), Bucket: "penny",
				Close: ptr(1.64), ChangePct: ptr(15.5), RVol20: ptr(6.74), BreakoutState: ptr("none"),
				MomentumScore: ptr(68), ScoreNullInputs: []string{"catalyst_tier"}},
			{Symbol: "NOSC", Bucket: "penny", RVol20: ptr(5.0)},
		},
		details: map[string]store.SymbolDetailRow{
			"NEXR": {
				Symbol: "NEXR", Exchange: ptr("NASDAQ"), Bucket: ptr("penny"), TS: scanDay, GatesPassed: true,
				Score: &store.ScoreRow{
					Total: 68, RVol: ptr(32.7), VolAccel: ptr(25.0), Float: ptr(10.0),
					PenaltyTotal: ptr(0.0), NullInputs: []string{"catalyst_tier"},
				},
			},
			"KVUE": {
				Symbol: "KVUE", Bucket: ptr("market"), TS: scanDay,
				GateFailures: []string{"change_pct_below_min"},
			},
		},
	}
}

func newTestServer(t *testing.T, st Store, now time.Time) *Server {
	t.Helper()
	return NewServer(Config{
		Store:             st,
		Caveats:           loadSharedCaveats(t),
		Log:               slog.New(slog.NewTextHandler(io.Discard, nil)),
		SessionReadyAfter: 6 * time.Hour,
		CacheTTL:          5 * time.Minute,
		CORSOrigins:       []string{"http://localhost:3000"},
		Now:               func() time.Time { return now },
	})
}

func get(t *testing.T, srv *Server, path string, header ...string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	for i := 0; i+1 < len(header); i += 2 {
		req.Header.Set(header[i], header[i+1])
	}
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	return rec
}

func decode(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode %q: %v", rec.Body.String(), err)
	}
	return out
}

// Friday morning, after Thursday 2026-09-17's session became expected.
var freshNow = ny("2026-09-18 09:00")

func TestToday_ShapeAndDefaults(t *testing.T) {
	srv := newTestServer(t, fixtureStore(), freshNow)
	rec := get(t, srv, "/api/v1/scanner/today")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body)
	}
	body := decode(t, rec)
	scan := body["scan"].(map[string]any)
	if scan["date"] != "2026-09-17" || scan["completed_at"] != "2026-09-18T12:43:03Z" {
		t.Errorf("scan = %v", scan)
	}
	if scan["universe_scanned"] != 4971.0 || scan["universe_eligible"] != 4975.0 || scan["is_stale"] != false {
		t.Errorf("scan = %v", scan)
	}

	buckets := body["buckets"].(map[string]any)
	market := buckets["market"].(map[string]any)
	if market["total_candidates"] != 0.0 || len(market["candidates"].([]any)) != 0 {
		t.Errorf("empty bucket must be present with [] and 0, got %v", market)
	}
	penny := buckets["penny"].(map[string]any)
	if penny["total_candidates"] != 2.0 {
		t.Errorf("penny total = %v", penny["total_candidates"])
	}
	first := penny["candidates"].([]any)[0].(map[string]any)
	if first["symbol"] != "NEXR" || first["momentum_score_100"] != 68.0 || first["catalyst_tier"] != nil {
		t.Errorf("first candidate = %v", first)
	}
	if first["score_attainable"] != 75.0 {
		t.Errorf("score_attainable = %v, want 75 (90 minus the null catalyst's 15)", first["score_attainable"])
	}
}

// §5: the score_status marker can never be silently dropped.
func TestToday_EveryCandidateCarriesScoreStatus(t *testing.T) {
	srv := newTestServer(t, fixtureStore(), freshNow)
	body := decode(t, get(t, srv, "/api/v1/scanner/today"))
	n := 0
	for name, b := range body["buckets"].(map[string]any) {
		for _, c := range b.(map[string]any)["candidates"].([]any) {
			n++
			if got, ok := c.(map[string]any)["score_status"]; !ok || got != "unvalidated" {
				t.Errorf("%s candidate %v: score_status = %v", name, c.(map[string]any)["symbol"], got)
			}
		}
	}
	if n == 0 {
		t.Fatal("no candidates checked; the assertion would pass vacuously")
	}
}

func TestToday_MissingScoreIsNullNotZero(t *testing.T) {
	srv := newTestServer(t, fixtureStore(), freshNow)
	body := decode(t, get(t, srv, "/api/v1/scanner/today"))
	second := body["buckets"].(map[string]any)["penny"].(map[string]any)["candidates"].([]any)[1].(map[string]any)
	v, present := second["momentum_score_100"]
	if !present || v != nil {
		t.Errorf("momentum_score_100 = %v (present=%v), want explicit null", v, present)
	}
	if a, present := second["score_attainable"]; !present || a != nil {
		t.Errorf("score_attainable = %v (present=%v), want explicit null alongside a null score", a, present)
	}
}

// The ceiling is per row: it follows each score's own null inputs.
func TestToday_ScoreAttainableIsPerRow(t *testing.T) {
	st := fixtureStore()
	st.candidates = []store.CandidateRow{
		{Symbol: "FULL", Bucket: "market", MomentumScore: ptr(80)},
		{Symbol: "NOCAT", Bucket: "market", MomentumScore: ptr(60), ScoreNullInputs: []string{"catalyst_tier"}},
		{Symbol: "TWO", Bucket: "market", MomentumScore: ptr(40), ScoreNullInputs: []string{"catalyst_tier", "float_shares_est"}},
	}
	body := decode(t, get(t, newTestServer(t, st, freshNow), "/api/v1/scanner/today"))
	want := map[string]float64{"FULL": 90, "NOCAT": 75, "TWO": 65}
	for _, c := range body["buckets"].(map[string]any)["market"].(map[string]any)["candidates"].([]any) {
		m := c.(map[string]any)
		if m["score_attainable"] != want[m["symbol"].(string)] {
			t.Errorf("%v: score_attainable = %v, want %v", m["symbol"], m["score_attainable"], want[m["symbol"].(string)])
		}
	}
}

func TestToday_NonFiniteNumbersAreNull(t *testing.T) {
	st := fixtureStore()
	st.candidates = []store.CandidateRow{{Symbol: "NAN", Bucket: "market", RVol20: ptr(math.NaN()), Close: ptr(math.Inf(1))}}
	body := decode(t, get(t, newTestServer(t, st, freshNow), "/api/v1/scanner/today"))
	c := body["buckets"].(map[string]any)["market"].(map[string]any)["candidates"].([]any)[0].(map[string]any)
	if c["rvol_20"] != nil || c["close"] != nil {
		t.Errorf("non-finite values should be null, got rvol_20=%v close=%v", c["rvol_20"], c["close"])
	}
}

func TestToday_IsStale(t *testing.T) {
	cases := []struct {
		name string
		now  time.Time
		want bool
	}{
		{"same session", ny("2026-09-18 09:00"), false},
		{"one session after the stored scan", ny("2026-09-19 12:00"), true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			body := decode(t, get(t, newTestServer(t, fixtureStore(), c.now), "/api/v1/scanner/today"))
			if got := body["scan"].(map[string]any)["is_stale"]; got != c.want {
				t.Errorf("is_stale = %v, want %v", got, c.want)
			}
		})
	}

	t.Run("holiday gap is not stale", func(t *testing.T) {
		st := fixtureStore()
		st.scanDate = time.Date(2026, 9, 4, 0, 0, 0, 0, time.UTC) // Friday before Labor Day
		body := decode(t, get(t, newTestServer(t, st, ny("2026-09-08 09:00")), "/api/v1/scanner/today"))
		if got := body["scan"].(map[string]any)["is_stale"]; got != false {
			t.Errorf("is_stale = %v across Labor Day, want false", got)
		}
	})
}

func TestToday_NoScanIs503(t *testing.T) {
	st := fixtureStore()
	st.hasScan = false
	rec := get(t, newTestServer(t, st, freshNow), "/api/v1/scanner/today")
	if rec.Code != http.StatusServiceUnavailable || decode(t, rec)["error"] != "no_scan_available" {
		t.Errorf("got %d %s", rec.Code, rec.Body)
	}
}

func TestStoreErrors(t *testing.T) {
	t.Run("database unreachable is 503", func(t *testing.T) {
		st := fixtureStore()
		st.queryErr, st.pingErr = errors.New("conn refused"), errors.New("conn refused")
		rec := get(t, newTestServer(t, st, freshNow), "/api/v1/scanner/today")
		if rec.Code != http.StatusServiceUnavailable || decode(t, rec)["error"] != "database_unavailable" {
			t.Errorf("got %d %s", rec.Code, rec.Body)
		}
	})
	t.Run("query failure on a healthy database is 500", func(t *testing.T) {
		st := fixtureStore()
		st.queryErr = errors.New("column does not exist")
		rec := get(t, newTestServer(t, st, freshNow), "/api/v1/scanner/today")
		if rec.Code != http.StatusInternalServerError || decode(t, rec)["error"] != "internal_error" {
			t.Errorf("got %d %s", rec.Code, rec.Body)
		}
	})
}

func TestToday_IsCachedAndExpires(t *testing.T) {
	st := fixtureStore()
	now := freshNow
	srv := NewServer(Config{
		Store: st, Caveats: loadSharedCaveats(t), SessionReadyAfter: 6 * time.Hour,
		CacheTTL: 5 * time.Minute, Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Now: func() time.Time { return now },
	})
	get(t, srv, "/api/v1/scanner/today")
	get(t, srv, "/api/v1/scanner/today")
	if st.calls != 1 {
		t.Errorf("store calls = %d within TTL, want 1", st.calls)
	}
	now = now.Add(5*time.Minute + time.Second)
	get(t, srv, "/api/v1/scanner/today")
	if st.calls != 2 {
		t.Errorf("store calls = %d after TTL, want 2", st.calls)
	}
}

func TestErrorsAreNotCached(t *testing.T) {
	st := fixtureStore()
	st.hasScan = false
	srv := newTestServer(t, st, freshNow)
	get(t, srv, "/api/v1/scanner/today")
	st.hasScan = true
	if rec := get(t, srv, "/api/v1/scanner/today"); rec.Code != http.StatusOK {
		t.Errorf("status = %d after the scan appeared; the error was cached", rec.Code)
	}
}

func TestDetail_ScoreBreakdown(t *testing.T) {
	caveats := loadSharedCaveats(t)
	st := fixtureStore()
	rec := get(t, newTestServer(t, st, freshNow), "/api/v1/scanner/today/nexr")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body)
	}
	if len(st.detailLookup) != 1 || st.detailLookup[0] != "NEXR" {
		t.Errorf("symbol lookup = %v, want case-insensitive NEXR", st.detailLookup)
	}
	body := decode(t, rec)
	if body["as_of"] != "2026-09-17" || body["gates_passed"] != true || len(body["gate_failures"].([]any)) != 0 {
		t.Errorf("detail = %v", body)
	}
	if body["evidence_note"] != caveats.Evidence {
		t.Errorf("evidence_note differs from the shared EVIDENCE_CAVEAT")
	}
	score := body["score"].(map[string]any)
	if score["total"] != 68.0 || score["attainable"] != 75.0 || score["allocated"] != 90.0 {
		t.Errorf("score totals = %v", score)
	}
	if score["status"] != "unvalidated" || score["model_version"] != "v2" || score["caveat"] != caveats.ResearchScore {
		t.Errorf("score framing = %v", score)
	}
	sub := score["sub_scores"].(map[string]any)
	for _, k := range []string{"rvol", "vol_accel", "catalyst", "float", "vwap", "breakout", "high52w"} {
		if _, ok := sub[k]; !ok {
			t.Errorf("sub_scores missing %q", k)
		}
	}
	if sub["catalyst"] != nil || sub["rvol"] != 32.7 {
		t.Errorf("sub_scores = %v", sub)
	}
	if p, ok := score["penalties"].([]any); !ok || len(p) != 0 {
		t.Errorf("penalties = %v, want []", score["penalties"])
	}
}

func TestDetail_GateFailedSymbolHasNoScore(t *testing.T) {
	body := decode(t, get(t, newTestServer(t, fixtureStore(), freshNow), "/api/v1/scanner/today/KVUE"))
	if body["gates_passed"] != false || body["score"] != nil {
		t.Errorf("detail = %v", body)
	}
	if f := body["gate_failures"].([]any); len(f) != 1 || f[0] != "change_pct_below_min" {
		t.Errorf("gate_failures = %v", f)
	}
	if body["evidence_note"] == "" {
		t.Error("evidence_note must be present even without a score")
	}
}

func TestDetail_UnknownSymbolIs404(t *testing.T) {
	rec := get(t, newTestServer(t, fixtureStore(), freshNow), "/api/v1/scanner/today/NOPE")
	if rec.Code != http.StatusNotFound || decode(t, rec)["error"] != "no_data_for_symbol" {
		t.Errorf("got %d %s", rec.Code, rec.Body)
	}
}

// §5: evidence_note must match the shared constant byte-for-byte. The expected
// value is read straight from the file, independently of LoadCaveats.
func TestDetail_EvidenceNoteIsTheSharedConstantByteForByte(t *testing.T) {
	raw, err := os.ReadFile(sharedCaveatsPath(t))
	if err != nil {
		t.Fatal(err)
	}
	var file map[string]any
	if err := json.Unmarshal(raw, &file); err != nil {
		t.Fatal(err)
	}
	body := decode(t, get(t, newTestServer(t, fixtureStore(), freshNow), "/api/v1/scanner/today/NEXR"))
	if body["evidence_note"] != file["evidence_caveat"] {
		t.Errorf("evidence_note = %q\nshared file  = %q", body["evidence_note"], file["evidence_caveat"])
	}
	if body["score"].(map[string]any)["caveat"] != file["research_score_caveat"] {
		t.Error("score.caveat differs from the shared research_score_caveat")
	}
}

func TestHealthz(t *testing.T) {
	st := fixtureStore()
	if rec := get(t, newTestServer(t, st, freshNow), "/healthz"); rec.Code != http.StatusOK {
		t.Errorf("healthy: %d", rec.Code)
	}
	st.pingErr = errors.New("down")
	if rec := get(t, newTestServer(t, st, freshNow), "/healthz"); rec.Code != http.StatusServiceUnavailable {
		t.Errorf("unhealthy: %d", rec.Code)
	}
}

func TestCORS(t *testing.T) {
	srv := newTestServer(t, fixtureStore(), freshNow)
	if rec := get(t, srv, "/healthz", "Origin", "http://localhost:3000"); rec.Header().Get("Access-Control-Allow-Origin") != "http://localhost:3000" {
		t.Error("allowed origin not echoed")
	}
	if rec := get(t, srv, "/healthz", "Origin", "https://evil.example"); rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Error("disallowed origin was granted")
	}

	req := httptest.NewRequest(http.MethodOptions, "/api/v1/scanner/today", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent || rec.Header().Get("Access-Control-Allow-Methods") == "" {
		t.Errorf("preflight: %d %v", rec.Code, rec.Header())
	}
}

func TestWriteMethodsAreRejected(t *testing.T) {
	srv := newTestServer(t, fixtureStore(), freshNow)
	for _, m := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
		req := httptest.NewRequest(m, "/api/v1/scanner/today", nil)
		rec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s: status %d, want 405", m, rec.Code)
		}
	}
}

func TestLoadCaveats(t *testing.T) {
	if _, err := LoadCaveats(filepath.Join(t.TempDir(), "missing.json")); err == nil {
		t.Error("missing file should fail, not fall back")
	}
	partial := filepath.Join(t.TempDir(), "partial.json")
	if err := os.WriteFile(partial, []byte(`{"evidence_caveat":"x"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadCaveats(partial); err == nil {
		t.Error("a file missing research_score_caveat should fail")
	}
}
