package momentumapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/konsbe/trading-agent/services/data-analyzer/internal/momentum"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/store"
)

func request(t *testing.T, srv *Server, method, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	return rec
}

// ── detail: facts, gates, weights, penalty rules ─────────────────────────────

func detailFixture() *fakeStore {
	st := fixtureStore()
	d := st.details["NEXR"]
	d.Facts = store.DetailFacts{
		Close: ptr(1.64), PriorClose: ptr(1.42), ChangePct: ptr(15.49), Volume: ptr(2.77e6),
		DollarVolume: ptr(4.55e6), RVol20: ptr(6.74), RSI14: ptr(36.0), High52w: ptr(847.0),
		PctOf52wHigh: ptr(0.0019), BreakoutState: ptr("none"), FloatSharesEst: ptr(1.2e7), FloatIsProxy: true,
		MarketCap: ptr(2.1e7), ComputedAt: time.Date(2026, 9, 18, 12, 43, 3, 0, time.UTC),
	}
	d.Score.Penalties = []string{momentum.ReasonAlreadyExtended}
	st.details["NEXR"] = d
	return st
}

func TestDetail_FactsAreTheStoredFeatures(t *testing.T) {
	body := decode(t, get(t, newTestServer(t, detailFixture(), freshNow), "/api/v1/scanner/today/NEXR"))
	f := body["facts"].(map[string]any)
	if f["close"] != 1.64 || f["prior_close"] != 1.42 || f["rvol_20"] != 6.74 || f["float_is_proxy"] != true {
		t.Errorf("facts = %v", f)
	}
	if got := f["change_abs"].(float64); got < 0.2199 || got > 0.2201 {
		t.Errorf("change_abs = %v, want close - prior_close = 0.22", got)
	}
	if f["computed_at"] != "2026-09-18T12:43:03Z" {
		t.Errorf("computed_at = %v", f["computed_at"])
	}
	if v, ok := f["catalyst_headline"]; !ok || v != nil {
		t.Errorf("catalyst_headline = %v (present=%v), want explicit null", v, ok)
	}
}

func TestDetail_GateChecksUseBucketThresholdsAndStoredFailures(t *testing.T) {
	st := detailFixture()
	body := decode(t, get(t, newTestServer(t, st, freshNow), "/api/v1/scanner/today/NEXR"))
	g := body["gates"].(map[string]any)
	if g["passed_count"] != 6.0 || g["total"] != 6.0 {
		t.Fatalf("gates = %v", g)
	}
	penny := momentum.DefaultGateConfig().Penny
	checks := map[string]map[string]any{}
	for _, c := range g["checks"].([]any) {
		m := c.(map[string]any)
		checks[m["key"].(string)] = m
	}
	if c := checks["rvol_20"]; c["min"] != penny.MinRVol20 || c["value"] != 6.74 || c["max"] != nil {
		t.Errorf("rvol_20 check = %v, want penny min %v", c, penny.MinRVol20)
	}
	if c := checks["change_pct"]; c["min"] != penny.MinChangePct || c["max"] != penny.MaxChangePct {
		t.Errorf("change_pct check = %v", c)
	}
	if c := checks["market_cap"]; c["min"] != nil || c["max"] != penny.MaxMarketCap {
		t.Errorf("penny market cap has no lower bound: %v", c)
	}

	// A failure code marks exactly its own check as failed.
	kvue := decode(t, get(t, newTestServer(t, st, freshNow), "/api/v1/scanner/today/KVUE"))["gates"].(map[string]any)
	if kvue["passed_count"] != 5.0 {
		t.Errorf("KVUE passed_count = %v, want 5 of 6", kvue["passed_count"])
	}
	for _, c := range kvue["checks"].([]any) {
		m := c.(map[string]any)
		if (m["key"] == "change_pct") == m["passed"].(bool) {
			t.Errorf("check %v passed=%v; only change_pct should fail", m["key"], m["passed"])
		}
	}
}

func TestDetail_ProvenanceMarkerIsNotAGateFailure(t *testing.T) {
	st := detailFixture()
	d := st.details["KVUE"]
	d.GateFailures = []string{momentum.GateMarketCapNull, "some_future_code"}
	st.details["KVUE"] = d
	g := decode(t, get(t, newTestServer(t, st, freshNow), "/api/v1/scanner/today/KVUE"))["gates"].(map[string]any)
	if g["passed_count"] != 6.0 {
		t.Errorf("market_cap_null is provenance, not a failure; passed_count = %v", g["passed_count"])
	}
	unmapped := g["unmapped_failures"].([]any)
	if len(unmapped) != 1 || unmapped[0] != "some_future_code" {
		t.Errorf("unmapped_failures = %v, want the unknown code surfaced", unmapped)
	}
}

func TestDetail_WeightsAndPenaltyRules(t *testing.T) {
	score := decode(t, get(t, newTestServer(t, detailFixture(), freshNow), "/api/v1/scanner/today/NEXR"))["score"].(map[string]any)
	w := score["weights"].(map[string]any)
	sum := 0.0
	for _, v := range w {
		sum += v.(float64)
	}
	if w["rvol"] != float64(momentum.WeightRVol) || sum != float64(momentum.WeightAllocated) {
		t.Errorf("weights = %v (sum %v), want model weights summing to %d", w, sum, momentum.WeightAllocated)
	}
	rules := score["penalty_rules"].([]any)
	if len(rules) != 3 {
		t.Fatalf("penalty_rules = %v, want all three §4.3 rules", rules)
	}
	for _, r := range rules {
		m := r.(map[string]any)
		wantApplied := m["code"] == momentum.ReasonAlreadyExtended
		if m["applied"] != wantApplied {
			t.Errorf("rule %v applied=%v, want %v", m["code"], m["applied"], wantApplied)
		}
	}
}

// ── bars ─────────────────────────────────────────────────────────────────────

func dailySeries(end time.Time, n int) []store.PriceBar {
	out := make([]store.PriceBar, 0, n)
	for i := n - 1; i >= 0; i-- {
		d := end.AddDate(0, 0, -i)
		out = append(out, store.PriceBar{TS: d, Open: 1, High: 2, Low: 0.5, Close: 1.5, Volume: 100})
	}
	return out
}

func TestBars_DailyRangesAnchorOnTheLatestBar(t *testing.T) {
	st := fixtureStore()
	latest := time.Date(2026, 9, 21, 0, 0, 0, 0, time.UTC)
	st.daily = map[string][]store.PriceBar{"VGZ": dailySeries(latest, 800)}
	srv := newTestServer(t, st, freshNow)

	cases := map[string]time.Time{
		"1M": latest.AddDate(0, -1, 0), "6M": latest.AddDate(0, -6, 0), "1Y": latest.AddDate(-1, 0, 0), "ALL": {},
	}
	for rng, wantFrom := range cases {
		st.dailyFrom = nil
		rec := request(t, srv, http.MethodGet, "/api/v1/scanner/symbols/vgz/bars?range="+rng)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: %d %s", rng, rec.Code, rec.Body)
		}
		body := decode(t, rec)
		if body["interval"] != "1Day" || body["adjusted"] != true || body["fallback"] != nil {
			t.Errorf("%s: %v", rng, body)
		}
		if len(st.dailyFrom) != 1 || !st.dailyFrom[0].Equal(wantFrom) {
			t.Errorf("%s: from = %v, want %v", rng, st.dailyFrom, wantFrom)
		}
		first := body["bars"].([]any)[0].(map[string]any)
		for _, k := range []string{"time", "open", "high", "low", "close", "volume"} {
			if _, ok := first[k]; !ok {
				t.Errorf("%s: bar missing %q", rng, k)
			}
		}
	}
}

func TestBars_IntradayRangesPreferStoredIntradayBars(t *testing.T) {
	st := fixtureStore()
	st.intraday = map[string][]store.PriceBar{"VGZ": dailySeries(time.Date(2026, 9, 21, 14, 0, 0, 0, time.UTC), 3)}
	srv := newTestServer(t, st, freshNow)

	body := decode(t, request(t, srv, http.MethodGet, "/api/v1/scanner/symbols/VGZ/bars?range=5D"))
	if body["interval"] != "5Min" || body["adjusted"] != false || body["fallback"] != nil || len(body["bars"].([]any)) != 3 {
		t.Errorf("5D = %v", body)
	}
	if len(st.intradayAsked) != 1 || st.intradayAsked[0] != "VGZ:5Min:5" {
		t.Errorf("intraday asked = %v, want 5 sessions of 5Min", st.intradayAsked)
	}
}

func TestBars_IntradayRangesFallBackToDailyAndSaySo(t *testing.T) {
	st := fixtureStore()
	st.daily = map[string][]store.PriceBar{"VGZ": dailySeries(time.Date(2026, 9, 21, 0, 0, 0, 0, time.UTC), 30)}
	srv := newTestServer(t, st, freshNow)

	for rng, n := range map[string]int{"1D": 1, "5D": 5} {
		body := decode(t, request(t, srv, http.MethodGet, "/api/v1/scanner/symbols/VGZ/bars?range="+rng))
		if body["interval"] != "1Day" || body["fallback"] != "no_intraday_data" || len(body["bars"].([]any)) != n {
			t.Errorf("%s fallback = %v", rng, body)
		}
	}
}

func TestBars_Errors(t *testing.T) {
	srv := newTestServer(t, fixtureStore(), freshNow)
	cases := []struct {
		path, code string
		status     int
	}{
		{"/api/v1/scanner/symbols/VGZ/bars?range=2W", "invalid_range", http.StatusBadRequest},
		{"/api/v1/scanner/symbols/VGZ/bars", "invalid_range", http.StatusBadRequest},
		{"/api/v1/scanner/symbols/BAD$SYM/bars?range=1M", "invalid_symbol", http.StatusBadRequest},
		{"/api/v1/scanner/symbols/NOBARS/bars?range=1M", "no_data_for_symbol", http.StatusNotFound},
	}
	for _, c := range cases {
		rec := request(t, srv, http.MethodGet, c.path)
		if rec.Code != c.status || decode(t, rec)["error"] != c.code {
			t.Errorf("%s: %d %s, want %d %s", c.path, rec.Code, rec.Body, c.status, c.code)
		}
	}
}

// ── watchlist ────────────────────────────────────────────────────────────────

func TestWatchlist_AddListRemove(t *testing.T) {
	st := fixtureStore()
	st.known = map[string]bool{"VGZ": true}
	srv := newTestServer(t, st, freshNow)

	rec := request(t, srv, http.MethodPut, "/api/v1/watchlist/vgz")
	if rec.Code != http.StatusCreated {
		t.Fatalf("first add: %d %s", rec.Code, rec.Body)
	}
	body := decode(t, rec)
	items := body["items"].([]any)
	if body["owner"] != "unauthenticated" || len(items) != 1 || items[0].(map[string]any)["symbol"] != "VGZ" {
		t.Errorf("after add = %v", body)
	}

	if rec := request(t, srv, http.MethodPut, "/api/v1/watchlist/VGZ"); rec.Code != http.StatusOK {
		t.Errorf("repeat add should be an idempotent 200, got %d", rec.Code)
	}
	if n := len(decode(t, request(t, srv, http.MethodGet, "/api/v1/watchlist"))["items"].([]any)); n != 1 {
		t.Errorf("list after repeat add = %d items, want 1", n)
	}

	rec = request(t, srv, http.MethodDelete, "/api/v1/watchlist/VGZ")
	if rec.Code != http.StatusOK || len(decode(t, rec)["items"].([]any)) != 0 {
		t.Errorf("remove: %d %s", rec.Code, rec.Body)
	}
	if rec := request(t, srv, http.MethodDelete, "/api/v1/watchlist/VGZ"); rec.Code != http.StatusOK {
		t.Errorf("repeat remove should be an idempotent 200, got %d", rec.Code)
	}

	for _, o := range st.owners {
		if o != nil {
			t.Errorf("owner = %q; with no auth every write must go to the unauthenticated (nil) list", *o)
		}
	}
}

func TestWatchlist_RejectsUnknownAndInvalidSymbols(t *testing.T) {
	st := fixtureStore()
	st.known = map[string]bool{}
	srv := newTestServer(t, st, freshNow)
	if rec := request(t, srv, http.MethodPut, "/api/v1/watchlist/NOPE"); rec.Code != http.StatusNotFound || decode(t, rec)["error"] != "unknown_symbol" {
		t.Errorf("unknown: %d %s", rec.Code, rec.Body)
	}
	if rec := request(t, srv, http.MethodPut, "/api/v1/watchlist/bad%20sym"); rec.Code != http.StatusBadRequest {
		t.Errorf("invalid: %d %s", rec.Code, rec.Body)
	}
	if len(st.watchlist) != 0 {
		t.Errorf("rejected symbols were written: %v", st.watchlist)
	}
}

func TestWatchlist_IsNeverCached(t *testing.T) {
	st := fixtureStore()
	st.known = map[string]bool{"VGZ": true}
	srv := newTestServer(t, st, freshNow)
	request(t, srv, http.MethodGet, "/api/v1/watchlist")
	request(t, srv, http.MethodPut, "/api/v1/watchlist/VGZ")
	if n := len(decode(t, request(t, srv, http.MethodGet, "/api/v1/watchlist"))["items"].([]any)); n != 1 {
		t.Errorf("list served stale after a write: %d items", n)
	}
}

func TestCORS_AllowsWatchlistWriteMethods(t *testing.T) {
	srv := newTestServer(t, fixtureStore(), freshNow)
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/watchlist/VGZ", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if got := rec.Header().Get("Access-Control-Allow-Methods"); got != "GET, PUT, DELETE, OPTIONS" {
		t.Errorf("Allow-Methods = %q", got)
	}
}

func TestWatchlist_ItemFactsAndStaleness(t *testing.T) {
	st := fixtureStore()
	fresh := time.Date(2026, 9, 17, 0, 0, 0, 0, time.UTC) // the session expected at freshNow
	old := time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC)
	st.watchlist = []store.WatchlistItem{
		{Symbol: "NOW", AddedAt: scanDay, AsOf: &fresh, Close: ptr(2.5), ChangePct: ptr(0.12), RVol20: ptr(4.2)},
		{Symbol: "OLD", AddedAt: scanDay, AsOf: &old, Close: ptr(1.0)},
		{Symbol: "NONE", AddedAt: scanDay},
	}
	body := decode(t, get(t, newTestServer(t, st, freshNow), "/api/v1/watchlist"))
	got := map[string]map[string]any{}
	for _, it := range body["items"].([]any) {
		m := it.(map[string]any)
		got[m["symbol"].(string)] = m
	}
	if n := got["NOW"]; n["as_of"] != "2026-09-17" || n["is_stale"] != false || n["close"] != 2.5 || n["change_pct"] != 0.12 || n["rvol_20"] != 4.2 {
		t.Errorf("fresh item = %v", n)
	}
	if o := got["OLD"]; o["as_of"] != "2026-09-10" || o["is_stale"] != true {
		t.Errorf("old row must be flagged stale, same rule as scan.is_stale: %v", o)
	}
	nn := got["NONE"]
	for _, k := range []string{"as_of", "close", "change_pct", "rvol_20"} {
		if v, present := nn[k]; !present || v != nil {
			t.Errorf("no-data item %q = %v (present=%v), want explicit null", k, v, present)
		}
	}
	if nn["is_stale"] != true {
		t.Errorf("no-data item is_stale = %v, want true", nn["is_stale"])
	}
}

func TestSymbolSearch(t *testing.T) {
	st := fixtureStore()
	st.symbols = []store.SymbolMatch{{Symbol: "VGZ", IsEligible: true}, {Symbol: "VGZX"}}
	srv := newTestServer(t, st, freshNow)

	rec := get(t, srv, "/api/v1/symbols?q=%20vgz%20")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d %s", rec.Code, rec.Body)
	}
	body := decode(t, rec)
	res := body["results"].([]any)
	if body["query"] != "vgz" || len(res) != 2 || res[0].(map[string]any)["is_eligible"] != true || res[1].(map[string]any)["is_eligible"] != false {
		t.Errorf("body = %v", body)
	}
	if len(st.searched) != 1 || st.searched[0] != "vgz" {
		t.Errorf("store asked %v, want the trimmed query", st.searched)
	}
	for _, q := range []string{"", "%20%20", strings.Repeat("a", 41)} {
		if rec := get(t, srv, "/api/v1/symbols?q="+q); rec.Code != http.StatusBadRequest || decode(t, rec)["error"] != "invalid_query" {
			t.Errorf("q=%q: %d %s, want 400 invalid_query", q, rec.Code, rec.Body)
		}
	}
	st.symbols = nil
	if res := decode(t, get(t, srv, "/api/v1/symbols?q=zz"))["results"]; res == nil || len(res.([]any)) != 0 {
		t.Errorf("no matches must be [], got %v", res)
	}
}

// Any symbol with a features row is served, labelled by its own date — not
// only symbols in the latest scan.
func TestDetail_ServesAnySymbolLabelledByItsOwnDate(t *testing.T) {
	st := fixtureStore()
	old := time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC)
	st.details["OLDC"] = store.SymbolDetailRow{Symbol: "OLDC", TS: old, GatesPassed: true} // candidate a week ago
	st.details["TODAYFAIL"] = store.SymbolDetailRow{Symbol: "TODAYFAIL", TS: scanDay, GateFailures: []string{"rvol_20_below_min"}}
	srv := newTestServer(t, st, freshNow)

	cases := []struct {
		sym                   string
		asOf                  string
		stale, candidateToday bool
	}{
		{"NEXR", "2026-09-17", false, true},       // today's candidate
		{"TODAYFAIL", "2026-09-17", false, false}, // in today's scan, failed gates
		{"OLDC", "2026-09-10", true, false},       // passed on an old date: NOT a candidate today
	}
	for _, c := range cases {
		rec := get(t, srv, "/api/v1/scanner/today/"+c.sym)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: %d %s", c.sym, rec.Code, rec.Body)
		}
		b := decode(t, rec)
		if b["as_of"] != c.asOf || b["is_stale"] != c.stale || b["is_candidate_today"] != c.candidateToday || b["latest_scan_date"] != "2026-09-17" {
			t.Errorf("%s: as_of=%v is_stale=%v is_candidate_today=%v latest=%v; want %s %v %v 2026-09-17",
				c.sym, b["as_of"], b["is_stale"], b["is_candidate_today"], b["latest_scan_date"], c.asOf, c.stale, c.candidateToday)
		}
	}
}
