package momentumapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/konsbe/trading-agent/services/data-analyzer/internal/compute"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/config"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/severity"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/store"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/technical"
)

func (f *fakeStore) AnalysisFreshness(_ context.Context, sym string) (store.AnalysisFreshness, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.fresh[sym], f.queryErr
}

func (f *fakeStore) Analysis(_ context.Context, sym string) (store.AnalysisInputs, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.analysisReads++
	return f.analysis[sym], f.queryErr
}

func (f *fakeStore) setFresh(sym string, fr store.AnalysisFreshness) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.fresh[sym] = fr
}

// analysisNames are the live worker's stored names.
func analysisNames(t *testing.T) technical.Names {
	t.Helper()
	t.Setenv("DATABASE_URL", "postgres://unused")
	cfg, err := config.LoadTechnicalAnalysis()
	if err != nil {
		t.Fatal(err)
	}
	return technical.NamesFor(technical.Emitter{Cfg: cfg})
}

var (
	barTS     = time.Date(2026, 9, 17, 13, 30, 0, 0, time.UTC)
	derivedTS = time.Date(2026, 9, 17, 20, 4, 0, 0, time.UTC)
	rawTS     = time.Date(2026, 9, 16, 8, 0, 0, 0, time.UTC)
)

func freshAnalysis() store.AnalysisFreshness {
	return store.AnalysisFreshness{LatestBarTS: ptr(barTS), TechnicalTS: ptr(barTS), DerivedTS: ptr(derivedTS), RawTS: ptr(rawTS), ScannerData: true}
}

func rows(t *testing.T, m map[string]string, values map[string]float64) map[string]store.StoredRow {
	t.Helper()
	out := map[string]store.StoredRow{}
	for name, p := range m {
		r := store.StoredRow{TS: barTS}
		if p != "" {
			if !json.Valid([]byte(p)) {
				t.Fatalf("fixture payload for %s is not JSON", name)
			}
			r.Payload = json.RawMessage(p)
		}
		if v, ok := values[name]; ok {
			r.Value = ptr(v)
		}
		out[name] = r
	}
	return out
}

// analysisInputs are shaped like TSM's / MSFT's live rows on 2026-09-25.
func analysisInputs(t *testing.T) store.AnalysisInputs {
	ind := rows(t, map[string]string{
		"rsi_14":       "",
		"atr_14":       "",
		"adx_14":       `{"dx": 26.78, "adx": 14.19, "plus_di": 33.1, "minus_di": 19.1}`,
		"macd_12_26_9": `{"histogram": 2.87, "bullish_cross_line_signal": true, "bearish_cross_line_signal": false}`,
		"trend":        `{"r2": 0.1, "direction": "down", "slope_pct": -0.0674, "higher_lows": true, "higher_highs": false}`,
		"ma_ribbon":    `{"golden_cross": false, "death_cross": true, "bull_stack": true}`,
		"bb_squeeze":   `{"squeeze": true, "bb_lower": 403.2, "bb_upper": 457.4}`,
		"vix_regime":   `{"vix": 27.5, "regime": "elevated", "series_id": "VIXCLS"}`,
		"pivots_prior_bar": `{"classic": {"PP": 448.10, "R1": 455.60, "R2": 460.06, "S1": 443.64, "S2": 436.14}, ` +
			`"reference_ts": "2026-09-16T13:30:00Z"}`,
		"fvg_min0.1_lb50":         `{"active_count": 3, "total_count": 21}`,
		"order_blocks_sw3_imp1.5": `{"active_count": 5, "last_bearish_ob": {"low": 453, "high": 479}, "last_bullish_ob": null}`,
		"liquidity_sweep_sw3": `{"last_sweep": {"kind": "high_sweep", "bar_low": 419.4, "bar_high": 429.8, ` +
			`"bar_close": 428.9, "bar_index": 485, "swept_level": 429.6}, "high_sweeps": 2, "low_sweeps": 1, "total_sweeps": 3}`,
		"hs_pattern_sw5":      `{"hs_found": false, "hs_neckline_break": false, "inv_hs_found": true, "inv_hs_neckline_break": true}`,
		"flag_pole5_len10":    `{"bull_flag": false, "bear_flag": true, "pole_pct": 6.1}`,
		"chart_pattern_hints": `{"double_top_candidate": true, "double_bottom_candidate": false}`,
		"triangle_sw3":        `{"kind": "none", "breakout": ""}`,
	}, map[string]float64{
		"rsi_14": 72.4, "atr_14": 10.5, "adx_14": 14.19, "macd_12_26_9": 2.87, "trend": -0.0674,
		"bb_squeeze": 1, "vix_regime": 27.5, "pivots_prior_bar": 448.10, "fvg_min0.1_lb50": 3,
		"order_blocks_sw3_imp1.5": 5, "liquidity_sweep_sw3": 3, "hs_pattern_sw5": 2, "flag_pole5_len10": -1,
	})
	der := rows(t, map[string]string{
		"composite_score":        `{"tier": "neutral", "score": 0.4166}`,
		"eps_strength":           `{"tier": "strong", "eps_growth_ttm_yoy_pct": 31.56}`,
		"revenue_strength":       `{"tier": "strong"}`,
		"pe_vs_5y_mean":          `{"tier": "expensive", "pe_ratio_ttm": 27.3884}`,
		"fcf_yield":              `{"tier": "avoid", "fcf_yield_pct": 1.2}`,
		"fcf_yield_tier":         `{"tier": "avoid"}`,
		"gross_margin_tier":      `{"tier": "strong_moat", "gross_margin_pct": 67.94}`,
		"gross_margin_trend_8q":  `{"direction": "stable"}`,
		"net_margin_tier":        `{"tier": "strong", "net_margin_pct": 40.31}`,
		"t2_health_score":        `{"tier": "healthy", "score": 0.75}`,
		"t2_roe":                 `{"tier": "excellent", "roe_pct": 33.22}`,
		"t2_roa":                 `{"tier": "high", "roa_pct": 19.42}`,
		"t2_current_ratio":       `{"tier": "monitor", "current_ratio": 1.2303}`,
		"t2_quick_ratio":         `{"tier": "adequate", "quick_ratio": 1.2221}`,
		"t2_leverage":            `{"tier": "conservative", "debt_to_equity": 0.35}`,
		"corr_summary":           `{"tier": "mixed_positive", "overall_score": 0.33}`,
		"corr_earnings_quality":  `{"tier": "healthy", "warnings": null, "positives": ["Revenue and EPS growing together — genuine organic quality growth"]}`,
		"corr_operational":       `{"tier": "mixed_positive", "warnings": ["High CapEx intensity"], "positives": ["High ROIC + strong revenue growth"]}`,
		"corr_master_signals":    `{"net_signal": "bearish", "value_trap": {"fired": false}, "deterioration_warning": {"fired": true, "score": 2}}`,
		"qual_moat_proxy":        `{"tier": "strong_moat_proxy"}`,
		"qual_news_sentiment_7d": `{"tier": "insufficient_data"}`,
	}, map[string]float64{
		"composite_score": 0.4166, "eps_strength": 1, "revenue_strength": 1, "fcf_yield": 1.2, "fcf_yield_tier": -1,
		"gross_margin_tier": 1, "net_margin_tier": 1, "t2_health_score": 0.75, "t2_roe": 33.22, "t2_roa": 19.42,
		"t2_current_ratio": 1.2303, "t2_quick_ratio": 1.2221, "t2_leverage": 0.35, "corr_summary": 0.33,
		"corr_earnings_quality": 1, "corr_operational": 0, "corr_master_signals": -1, "qual_moat_proxy": 1,
	})
	return store.AnalysisInputs{
		Indicators: ind, Derived: der, MarketCap: ptr(3.66317e12), VIX: ptr(27.5),
		Headlines: []store.NewsHeadline{{Title: "TSMC beats", URL: ptr("https://example.test/a"), Source: "finnhub",
			PublishedAt: time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC), Sentiment: ptr(0.4)}},
		Macro: map[string]store.MacroRow{
			"mc_market_cycle":      {Metric: "mc_market_cycle", Payload: json.RawMessage(`{"tone": "neutral", "symbol": "SPY", "composite_phase": "late_cycle_stretched", "price_phase": "bull_extended"}`)},
			"mc_macro_correlation": {Metric: "mc_macro_correlation", Payload: json.RawMessage(`{"tone": "stressed", "regime": "global_liquidity_stress"}`)},
			"mc_price_phase:NEXR":  {Metric: "mc_price_phase:NEXR", Payload: json.RawMessage(`{"price_phase": "pullback", "drawdown_pct": -4.54}`)},
		},
	}
}

type computeCall struct {
	symbol string
	parts  AnalysisParts
}

type analysisHarness struct {
	t      *testing.T
	st     *fakeStore
	srv    *Server
	mu     sync.Mutex
	now    time.Time
	calls  []computeCall
	result func(symbol string) error
	gate   chan struct{} // when non-nil, computations block until it is closed
}

func newAnalysisHarness(t *testing.T) *analysisHarness {
	t.Helper()
	h := &analysisHarness{t: t, st: fixtureStore(), now: freshNow}
	h.st.fresh = map[string]store.AnalysisFreshness{}
	h.st.analysis = map[string]store.AnalysisInputs{"NEXR": analysisInputs(t)}
	h.result = func(sym string) error {
		// A successful computation writes current rows.
		h.st.setFresh(sym, freshAnalysis())
		return nil
	}
	h.srv = NewServer(Config{
		Store:           h.st,
		Caveats:         loadSharedCaveats(t),
		CorrelationText: loadSharedCorrelationText(t),
		BacktestReport:  loadSharedReport(t),
		Log:             slog.New(slog.NewTextHandler(io.Discard, nil)),
		CacheTTL:        5 * time.Minute,
		AnalysisNames:   analysisNames(t),
		AnalysisCompute: func(ctx context.Context, sym string, parts AnalysisParts) error {
			h.mu.Lock()
			h.calls = append(h.calls, computeCall{sym, parts})
			gate := h.gate
			h.mu.Unlock()
			if gate != nil {
				select {
				case <-gate:
				case <-ctx.Done():
					return ctx.Err()
				}
			}
			return h.result(sym)
		},
		AnalysisTimeout:          time.Minute,
		AnalysisRetryAfter:       1500 * time.Millisecond,
		AnalysisFailedRetryAfter: time.Minute,
		Now: func() time.Time {
			h.mu.Lock()
			defer h.mu.Unlock()
			return h.now
		},
	})
	return h
}

func (h *analysisHarness) get(sym string) (int, map[string]any, http.Header) {
	h.t.Helper()
	rec := get(h.t, h.srv, "/api/v1/scanner/today/"+sym+"/analysis")
	return rec.Code, decode(h.t, rec), rec.Header()
}

func (h *analysisHarness) wait()                   { h.srv.analysis.wg.Wait() }
func (h *analysisHarness) advance(d time.Duration) { h.mu.Lock(); h.now = h.now.Add(d); h.mu.Unlock() }
func (h *analysisHarness) callCount() int          { h.mu.Lock(); defer h.mu.Unlock(); return len(h.calls) }

func obj(t *testing.T, m map[string]any, path ...string) map[string]any {
	t.Helper()
	cur := m
	for _, k := range path {
		next, ok := cur[k].(map[string]any)
		if !ok {
			t.Fatalf("%v: %q is %T %v", path, k, cur[k], cur[k])
		}
		cur = next
	}
	return cur
}

func TestAnalysis_ReadyServesStoredRowsWithoutComputing(t *testing.T) {
	h := newAnalysisHarness(t)
	h.st.fresh["NEXR"] = freshAnalysis()
	code, body, _ := h.get("nexr")
	if code != http.StatusOK || body["status"] != "ready" || body["symbol"] != "NEXR" || body["scanner_data"] != true {
		t.Fatalf("got %d %v", code, body)
	}
	if h.callCount() != 0 {
		t.Error("current rows must not trigger a computation")
	}
	if body["as_of"] != "2026-09-17" || body["fundamentals_computed_at"] != "2026-09-17T20:04:00Z" {
		t.Errorf("as_of = %v, fundamentals_computed_at = %v", body["as_of"], body["fundamentals_computed_at"])
	}
	if s := obj(t, body, "sections"); s["technical"] != "ready" || s["fundamentals"] != "ready" {
		t.Errorf("sections = %v", s)
	}

	tech := obj(t, body, "technical")
	if r := obj(t, tech, "rsi_14"); r["value"] != 72.4 || r["band"] != "overbought" || r["severity"] != "notice" {
		t.Errorf("rsi_14 = %v", r)
	}
	if m := obj(t, tech, "macd"); m["hist"] != 2.87 || m["cross"] != "bullish" {
		t.Errorf("macd = %v", m)
	}
	if a := obj(t, tech, "adx_14"); a["value"] != 14.19 || a["band"] != "not_strong_trend" {
		t.Errorf("adx_14 = %v", a)
	}
	if tr := obj(t, tech, "trend"); tr["direction"] != "down" || tr["slope_pct"] != -0.0674 {
		t.Errorf("trend = %v", tr)
	}
	if tech["ma_cross"] != "death_cross" || tech["atr_14"] != 10.5 {
		t.Errorf("ma_cross %v atr %v", tech["ma_cross"], tech["atr_14"])
	}
	if b := obj(t, tech, "bb_squeeze"); b["active"] != true || b["severity"] != "info" {
		t.Errorf("bb_squeeze = %v", b)
	}
	if v := obj(t, tech, "vix_regime"); v["value"] != 27.5 || v["band"] != "elevated" || v["severity"] != "warning" {
		t.Errorf("vix_regime = %v", v)
	}
	if p := obj(t, tech, "pivots"); p["pp"] != 448.10 || p["r1"] != 455.60 || p["s1"] != 443.64 {
		t.Errorf("pivots = %v", p)
	}
	if s := obj(t, tech, "smc"); s["fvgs_active"] != 3.0 || s["obs_active"] != 5.0 || s["liq_sweeps"] != 3.0 {
		t.Errorf("smc = %v", s)
	}

	fa := obj(t, body, "fundamentals")
	if c := obj(t, fa, "composite"); c["score"] != 0.4166 || c["tier"] != "neutral" {
		t.Errorf("composite = %v", c)
	}
	if fa["eps_strength"] != "strong" || fa["revenue"] != "strong" || fa["ttm_pe"] != 27.3884 || fa["market_cap"] != 3.66317e12 {
		t.Errorf("fundamentals = %v", fa)
	}
	if pe := obj(t, fa, "pe_vs_5y"); pe["band"] != "expensive" || pe["value"] != nil {
		t.Errorf("pe_vs_5y = %v", pe)
	}
	if g := obj(t, fa, "gross_margin"); g["value"] != 67.94 || g["tier"] != "strong_moat" || g["trend"] != "stable" {
		t.Errorf("gross_margin = %v", g)
	}
	if n := obj(t, fa, "net_margin"); n["value"] != 40.31 || n["trend"] != nil {
		t.Errorf("net_margin = %v", n)
	}
	if f := obj(t, fa, "fcf_yield"); f["value"] != 1.2 || f["tier"] != "avoid" {
		t.Errorf("fcf_yield = %v", f)
	}

	bs := obj(t, body, "balance_sheet")
	if r := obj(t, bs, "roe"); r["value"] != 33.22 || r["band"] != "excellent" {
		t.Errorf("roe = %v", r)
	}
	if r := obj(t, bs, "roic"); r["value"] != nil || r["band"] != nil {
		t.Errorf("roic with no row = %v; want nulls", r)
	}

	corr := obj(t, body, "correlations")
	if c := obj(t, corr, "composite"); c["score"] != 0.33 || c["tier"] != "mixed_positive" {
		t.Errorf("corr composite = %v", c)
	}
	clusters := corr["clusters"].([]any)
	if len(clusters) != 2 || clusters[0].(map[string]any)["name"] != "earnings_quality" || clusters[1].(map[string]any)["name"] != "operational" {
		t.Errorf("clusters = %v", clusters)
	}
	if w := clusters[0].(map[string]any)["warnings"].([]any); len(w) != 0 {
		t.Errorf("null warnings must serve as [], got %v", w)
	}
	// Known stored sentences are served as their display text; an unknown one as stored.
	if a := corr["aligned_signals"].([]any); len(a) != 2 || a[0] != "Strong revenue growth and strong EPS growth" || a[1] != "High ROIC + strong revenue growth" {
		t.Errorf("aligned_signals = %v", a)
	}
	if m := obj(t, corr, "master_signals"); m["net_signal"] != "bearish" || len(m["fired"].([]any)) != 1 || m["fired"].([]any)[0] != "deterioration_warning" {
		t.Errorf("master_signals = %v", m)
	}

	q := obj(t, body, "qualitative")
	if m := obj(t, q, "moat_proxy"); m["tier"] != "strong_moat_proxy" || m["value"] != 1.0 {
		t.Errorf("moat = %v", m)
	}
	if n := obj(t, q, "news_sentiment_7d"); n["tier"] != "insufficient_data" || n["value"] != nil {
		t.Errorf("sentiment 7d = %v", n)
	}
	hl := obj(t, body, "sentiment")["headlines"].([]any)
	if len(hl) != 1 || hl[0].(map[string]any)["published_at"] != "2026-09-17T12:00:00Z" {
		t.Errorf("headlines = %v", hl)
	}

	ctxb := obj(t, body, "context_vs_benchmark")
	if ctxb["benchmark_symbol"] != "SPY" || ctxb["market_cycle_composite"] != "late_cycle_stretched" || ctxb["market_cycle_tone"] != "neutral" ||
		ctxb["price_phase"] != "pullback" || ctxb["drawdown_from_peak_pct"] != -4.54 ||
		ctxb["correlation_regime"] != "global_liquidity_stress" || ctxb["correlation_regime_tone"] != "stressed" ||
		ctxb["relative_strength_20d_pp"] != nil {
		t.Errorf("context = %v", ctxb)
	}

	hs := obj(t, body, "heuristic_signals")
	var got []string
	for _, p := range hs["chart_patterns"].([]any) {
		m := p.(map[string]any)
		got = append(got, m["pattern"].(string)+":"+map[bool]string{true: "c", false: "u"}[m["confirmed"].(bool)]+":"+m["severity"].(string))
	}
	if want := []string{"inv_head_shoulders:c:notice", "bear_flag:c:notice", "double_top:u:info"}; !slices.Equal(got, want) {
		t.Errorf("chart_patterns = %v, want %v", got, want)
	}
	act := obj(t, hs, "action_signal")
	// High sweep closed back below + bearish OB + downtrend = TRIM_WATCH 4/4.
	if act["alert_type"] != "liquidity_sweep" || act["action"] != "TRIM_WATCH" || act["severity"] != "warning" || act["vix_regime"] != "elevated" {
		t.Errorf("action_signal = %v", act)
	}
	if c := obj(t, act, "confluence"); c["score"] != 4.0 || c["max"] != 4.0 {
		t.Errorf("confluence = %v", c)
	}
	reasons := act["reasoning"].([]any)
	if len(reasons) != 5 || reasons[0] != "📍 High sweep (3 recent): stop-hunt above swing high detected" {
		t.Errorf("reasoning = %q", reasons)
	}
}

// §5 / §2.4: the caveat is the shared file's text, byte for byte, read
// independently of LoadCaveats.
func TestAnalysis_HeuristicCaveatIsTheSharedConstantByteForByte(t *testing.T) {
	raw, err := os.ReadFile(sharedCaveatsPath(t))
	if err != nil {
		t.Fatal(err)
	}
	var file map[string]any
	if err := json.Unmarshal(raw, &file); err != nil {
		t.Fatal(err)
	}
	want, _ := file["heuristic_ta_caveat"].(string)
	if want == "" {
		t.Fatal("shared file has no heuristic_ta_caveat")
	}
	if want == file["research_score_caveat"] {
		t.Error("HEURISTIC_TA_CAVEAT must not reuse RESEARCH_SCORE_CAVEAT")
	}
	h := newAnalysisHarness(t)
	h.st.fresh["NEXR"] = freshAnalysis()
	_, body, _ := h.get("NEXR")
	if got := obj(t, body, "heuristic_signals")["caveat"]; got != want {
		t.Errorf("caveat = %q\nshared   = %q", got, want)
	}
	// The same, on a symbol with bars but no stored rows at all.
	h.st.fresh["KVUE"] = store.AnalysisFreshness{LatestBarTS: ptr(barTS)}
	h.result = func(string) error { return nil }
	h.get("KVUE")
	h.wait()
	_, body, _ = h.get("KVUE")
	if got := obj(t, body, "heuristic_signals")["caveat"]; got != want {
		t.Errorf("caveat on an empty analysis = %q", got)
	}
}

func TestAnalysis_StaleRowsComputeInTheBackgroundThenServeReady(t *testing.T) {
	h := newAnalysisHarness(t)
	stale := freshAnalysis()
	stale.LatestBarTS = ptr(barTS.AddDate(0, 0, 1)) // a new bar the worker has not seen
	h.st.fresh["NEXR"] = stale
	h.gate = make(chan struct{})

	code, body, hdr := h.get("NEXR")
	if code != http.StatusAccepted || body["status"] != "computing" || body["symbol"] != "NEXR" ||
		body["message"] != "Computing analysis for this symbol -- first view only" || body["retry_after_ms"] != 1500.0 {
		t.Fatalf("got %d %v", code, body)
	}
	if hdr.Get("Retry-After") != "2" {
		t.Errorf("Retry-After = %q, want 2 (seconds, rounded up)", hdr.Get("Retry-After"))
	}
	if code, body, _ := h.get("NEXR"); code != http.StatusAccepted || body["status"] != "computing" {
		t.Errorf("while running: %d %v", code, body)
	}
	close(h.gate)
	h.wait()
	if code, body, _ := h.get("NEXR"); code != http.StatusOK || body["status"] != "ready" {
		t.Fatalf("after computing: %d %v", code, body)
	}
	if h.callCount() != 1 {
		t.Errorf("computations = %d, want 1", h.callCount())
	}
	if c := h.calls[0]; c.symbol != "NEXR" || !c.parts.Technical || c.parts.Fundamentals {
		t.Errorf("computed %+v; only the stale technical part should run", c)
	}
}

func TestAnalysis_OnlyStaleFundamentalsRecomputeOnlyFundamentals(t *testing.T) {
	h := newAnalysisHarness(t)
	fr := freshAnalysis()
	fr.RawTS = ptr(derivedTS.Add(time.Hour)) // new raw metrics since the last scoring
	h.st.fresh["NEXR"] = fr
	h.get("NEXR")
	h.wait()
	if c := h.calls[0]; c.parts.Technical || !c.parts.Fundamentals {
		t.Errorf("parts = %+v", c.parts)
	}

	h2 := newAnalysisHarness(t)
	old := freshAnalysis()
	h2.st.fresh["NEXR"] = old
	h2.advance(derivedTS.Add(27 * time.Hour).Sub(freshNow)) // derived rows older than 26h
	if code, _, _ := h2.get("NEXR"); code != http.StatusAccepted {
		t.Errorf("derived rows past FundamentalsMaxAge must recompute, got %d", code)
	}
}

func TestAnalysis_ConcurrentRequestsShareOneComputation(t *testing.T) {
	h := newAnalysisHarness(t)
	h.st.fresh["NEXR"] = store.AnalysisFreshness{LatestBarTS: ptr(barTS)}
	h.gate = make(chan struct{})
	var wg sync.WaitGroup
	var accepted atomic.Int32
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rec := get(t, h.srv, "/api/v1/scanner/today/NEXR/analysis")
			if rec.Code == http.StatusAccepted {
				accepted.Add(1)
			}
		}()
	}
	wg.Wait()
	close(h.gate)
	h.wait()
	if accepted.Load() != 20 {
		t.Errorf("%d of 20 got 202", accepted.Load())
	}
	if n := h.callCount(); n != 1 {
		t.Errorf("computations = %d, want 1 (single-flight per symbol)", n)
	}
}

func TestAnalysis_FailureIsReportedThenRetried(t *testing.T) {
	h := newAnalysisHarness(t)
	h.st.fresh["NEXR"] = store.AnalysisFreshness{LatestBarTS: ptr(barTS)}
	h.result = func(string) error { return errors.New("upsert indicator: connection reset") }
	h.get("NEXR")
	h.wait()

	code, body, hdr := h.get("NEXR")
	if code != http.StatusInternalServerError || body["status"] != "failed" || body["error"] != "analysis_failed" || body["message"] == "" {
		t.Fatalf("after failure: %d %v", code, body)
	}
	if hdr.Get("Retry-After") != "60" || body["retry_after_ms"] != 60000.0 {
		t.Errorf("Retry-After %q, retry_after_ms %v", hdr.Get("Retry-After"), body["retry_after_ms"])
	}
	if h.callCount() != 1 {
		t.Errorf("a reported failure must not start another attempt yet (calls=%d)", h.callCount())
	}

	h.advance(time.Minute)
	h.result = func(sym string) error { h.st.setFresh(sym, freshAnalysis()); return nil }
	if code, body, _ := h.get("NEXR"); code != http.StatusAccepted || body["status"] != "computing" {
		t.Fatalf("after the retry window: %d %v", code, body)
	}
	h.wait()
	if code, _, _ := h.get("NEXR"); code != http.StatusOK {
		t.Errorf("after a successful retry: %d", code)
	}
}

func TestAnalysis_TimeoutEndsInFailedNotComputing(t *testing.T) {
	h := newAnalysisHarness(t)
	h.srv.analysis.timeout = 20 * time.Millisecond
	h.st.fresh["NEXR"] = store.AnalysisFreshness{LatestBarTS: ptr(barTS)}
	h.gate = make(chan struct{}) // never closed: the computation hangs
	h.get("NEXR")
	h.wait()
	if code, body, _ := h.get("NEXR"); code != http.StatusInternalServerError || body["status"] != "failed" {
		t.Errorf("hung computation: %d %v", code, body)
	}
}

func TestAnalysis_ConcurrencyIsBounded(t *testing.T) {
	h := newAnalysisHarness(t)
	h.srv.analysis.sem = make(chan struct{}, 1)
	for _, s := range []string{"NEXR", "KVUE"} {
		h.st.fresh[s] = store.AnalysisFreshness{LatestBarTS: ptr(barTS)}
	}
	h.gate = make(chan struct{})
	h.get("NEXR")
	h.get("KVUE")
	deadline := time.Now().Add(time.Second)
	for h.callCount() < 1 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	time.Sleep(20 * time.Millisecond)
	if n := h.callCount(); n != 1 {
		t.Errorf("running = %d with a limit of 1", n)
	}
	close(h.gate)
	h.wait()
	if n := h.callCount(); n != 2 {
		t.Errorf("computations = %d after the slot freed, want 2", n)
	}
}

// A symbol with too little data computes successfully and still has no rows:
// that is "ready" with nulls and no_data sections, never an endless computing.
func TestAnalysis_NothingToComputeIsReadyWithNulls(t *testing.T) {
	h := newAnalysisHarness(t)
	h.st.fresh["KVUE"] = store.AnalysisFreshness{LatestBarTS: ptr(barTS)}
	h.result = func(string) error { return nil }
	if code, _, _ := h.get("KVUE"); code != http.StatusAccepted {
		t.Fatalf("first view = %d", code)
	}
	h.wait()
	code, body, _ := h.get("KVUE")
	if code != http.StatusOK || body["status"] != "ready" {
		t.Fatalf("got %d %v", code, body)
	}
	if s := obj(t, body, "sections"); s["technical"] != "no_data" || s["fundamentals"] != "no_data" {
		t.Errorf("sections = %v", s)
	}
	if body["as_of"] != nil || obj(t, body, "technical", "rsi_14")["value"] != nil || obj(t, body, "fundamentals")["market_cap"] != nil ||
		obj(t, body, "heuristic_signals")["action_signal"] != nil || len(obj(t, body, "heuristic_signals")["chart_patterns"].([]any)) != 0 {
		t.Errorf("empty analysis = %v", body)
	}
	// Every leaf is null, false-free, or an empty list: no number defaults to 0.
	var walk func(path string, v any)
	walk = func(path string, v any) {
		switch x := v.(type) {
		case map[string]any:
			for k, c := range x {
				walk(path+"."+k, c)
			}
		case float64:
			t.Errorf("%s = %v on a symbol with no stored rows; want null", path, x)
		}
	}
	walk("", body)
	if h.callCount() != 1 {
		t.Errorf("a settled no-data symbol must not recompute on every view (calls=%d)", h.callCount())
	}
}

// A symbol the scanner never wrote a row for (an ETF, a foreign listing) is
// served as long as it has daily bars, flagged scanner_data=false; only a
// symbol with no daily bars is 404. GET /today/{symbol} keeps its own 404.
func TestAnalysis_UnscannedSymbolWithBarsIsServed(t *testing.T) {
	h := newAnalysisHarness(t)
	h.st.fresh["TSM"] = store.AnalysisFreshness{LatestBarTS: ptr(barTS)}
	h.result = func(sym string) error {
		fr := freshAnalysis()
		fr.ScannerData = false
		h.st.setFresh(sym, fr)
		return nil
	}
	code, body, _ := h.get("tsm")
	if code != http.StatusAccepted || body["status"] != "computing" || body["scanner_data"] != false {
		t.Fatalf("first view: %d %v", code, body)
	}
	h.wait()
	code, body, _ = h.get("TSM")
	if code != http.StatusOK || body["status"] != "ready" || body["scanner_data"] != false {
		t.Fatalf("after computing: %d %v", code, body)
	}
	if slices.Contains(h.st.detailLookup, "TSM") {
		t.Error("the analysis must not depend on the scanner's detail row")
	}
	if rec := get(t, h.srv, "/api/v1/scanner/today/TSM"); rec.Code != http.StatusNotFound {
		t.Errorf("GET /today/TSM = %d, want its own 404 unchanged", rec.Code)
	}
}

func TestAnalysis_SameErrorsAsDetail(t *testing.T) {
	h := newAnalysisHarness(t)
	code, body, _ := h.get("NOPE")
	if code != http.StatusNotFound || body["error"] != "no_data_for_symbol" {
		t.Errorf("symbol with no daily bars: %d %v", code, body)
	}
	// A scanned symbol whose bars are gone is no different.
	h.st.fresh["KVUE"] = store.AnalysisFreshness{ScannerData: true}
	if code, body, _ := h.get("KVUE"); code != http.StatusNotFound || body["error"] != "no_data_for_symbol" {
		t.Errorf("scanner row but no bars: %d %v", code, body)
	}
	if h.callCount() != 0 {
		t.Error("an unknown symbol must not trigger a computation")
	}
	h.st.hasScan = false
	if code, body, _ := h.get("NEXR"); code != http.StatusServiceUnavailable || body["error"] != "no_scan_available" {
		t.Errorf("no scan: %d %v", code, body)
	}
	h.st.hasScan = true
	h.st.queryErr, h.st.pingErr = errors.New("down"), errors.New("down")
	if code, body, _ := h.get("NEXR"); code != http.StatusServiceUnavailable || body["error"] != "database_unavailable" {
		t.Errorf("db down: %d %v", code, body)
	}
}

func TestAnalysis_OnlyReadyIsCached(t *testing.T) {
	h := newAnalysisHarness(t)
	h.st.fresh["NEXR"] = freshAnalysis()
	h.get("NEXR")
	h.get("NEXR")
	if h.st.analysisReads != 1 {
		t.Errorf("reads = %d within TTL, want 1", h.st.analysisReads)
	}
	h.st.fresh["KVUE"] = store.AnalysisFreshness{LatestBarTS: ptr(barTS)}
	h.gate = make(chan struct{})
	h.get("KVUE")
	close(h.gate)
	h.wait()
	h.st.setFresh("KVUE", store.AnalysisFreshness{LatestBarTS: ptr(barTS.AddDate(0, 0, 1))})
	if code, _, _ := h.get("KVUE"); code != http.StatusAccepted {
		t.Error("a computing response must never be served from the cache")
	}
}

// §5: every band/tier field is the stored classification (or, for RSI/ADX, the
// ported bot classifier), never re-derived from the number.
func TestAnalysis_BandsAreTheStoredClassification(t *testing.T) {
	n := analysisNames(t)
	for _, v := range []float64{0, 29.99, 30, 55, 70, 70.01, 99} {
		in := store.AnalysisInputs{Indicators: map[string]store.StoredRow{n.RSI: {Value: ptr(v)}, n.ADX: {Value: ptr(v / 2)}}}
		tech := buildTechnical(in.Indicators, n)
		if *tech.RSI14.Band != compute.ClassifyRSI(v, compute.BotRSIThresholds) || *tech.ADX14.Band != compute.ClassifyADX(v/2) {
			t.Errorf("rsi %v / adx %v bands = %q / %q", v, v/2, *tech.RSI14.Band, *tech.ADX14.Band)
		}
	}
	// A stored tier that disagrees with its number is served as stored.
	d := map[string]store.StoredRow{
		"t2_roe":          {Value: ptr(1.0), Payload: json.RawMessage(`{"tier": "excellent"}`)},
		"composite_score": {Value: ptr(-0.9), Payload: json.RawMessage(`{"tier": "strong"}`)},
	}
	if b := buildBalance(d); *b.ROE.Band != "excellent" {
		t.Errorf("roe band = %q", *b.ROE.Band)
	}
	if f := buildFundamentals(store.AnalysisInputs{Derived: d}); *f.Composite.Tier != "strong" {
		t.Errorf("composite tier = %q", *f.Composite.Tier)
	}
	ind := map[string]store.StoredRow{n.VIXRegime: {Value: ptr(40.0), Payload: json.RawMessage(`{"regime": "normal"}`)}}
	if v := buildTechnical(ind, n).VIXRegime; *v.Band != "normal" {
		t.Errorf("vix band = %q, want the stored regime", *v.Band)
	}
}

// Every pattern the builder can emit is in severity.ChartPatterns (which
// severity_test pins to a mapped level): turn every pattern on and check the
// output covers the whole list.
func TestAnalysis_EveryEmittablePatternIsMapped(t *testing.T) {
	n := analysisNames(t)
	seen := map[string]bool{}
	for _, kind := range []string{"ascending", "descending", "symmetrical"} {
		ind := map[string]store.StoredRow{
			n.HSPattern:         {Payload: json.RawMessage(`{"hs_found": true, "inv_hs_found": true, "hs_neckline_break": true}`)},
			n.Flag:              {Payload: json.RawMessage(`{"bull_flag": true, "bear_flag": true}`)},
			n.ChartPatternHints: {Payload: json.RawMessage(`{"double_top_candidate": true, "double_bottom_candidate": true}`)},
			n.Triangle:          {Payload: json.RawMessage(`{"kind": "` + kind + `", "breakout": "up"}`)},
		}
		for _, p := range buildChartPatterns(ind, n) {
			seen[p.Pattern] = true
			if want, _ := severity.ForPattern(p.Pattern, p.Confirmed); string(want) != p.Severity {
				t.Errorf("%s severity %q, table %q", p.Pattern, p.Severity, want)
			}
		}
	}
	for _, p := range severity.ChartPatterns {
		if !seen[p] {
			t.Errorf("severity.ChartPatterns lists %q but the builder never emits it", p)
		}
	}
	for p := range seen {
		if !slices.Contains(severity.ChartPatterns, p) {
			t.Errorf("builder emits %q, which severity.ChartPatterns does not list", p)
		}
	}
}

// The action signal is the bot's rule on the bot's inputs, and absent exactly
// when the bot's scan would raise no liquidity_sweep alert.
func TestAnalysis_ActionSignal(t *testing.T) {
	n := analysisNames(t)
	in := analysisInputs(t)
	in.Indicators[n.LiquiditySweep] = store.StoredRow{Value: ptr(0.0), Payload: json.RawMessage(`{"total_sweeps": 0}`)}
	if a := buildActionSignal(in, n); a != nil {
		t.Errorf("no sweeps: %+v", a)
	}
	in.Indicators[n.LiquiditySweep] = store.StoredRow{Value: ptr(6.0), Payload: json.RawMessage(
		`{"total_sweeps": 6, "last_sweep": {"kind": "low_sweep", "bar_close": 101.5, "swept_level": 100}}`)}
	in.Indicators[n.OrderBlocks] = store.StoredRow{Payload: json.RawMessage(`{"last_bullish_ob": {"low": 1}, "last_bearish_ob": {}}`)}
	in.Indicators[n.Trend] = store.StoredRow{Payload: json.RawMessage(`{"direction": "up"}`)}
	in.VIX = ptr(14.2)
	a := buildActionSignal(in, n)
	if a == nil || a.Action != "BUY_WATCH" || a.Confluence.Score != 4 || a.Severity != "notice" || *a.VIXRegime != "normal" {
		t.Fatalf("low sweep reclaimed = %+v", a)
	}
	want := []string{
		"📍 Low sweep (6 recent): stop-hunt below swing low detected",
		"✅ Closed back above swept level — institutional accumulation pattern",
		"✅ Bullish order block nearby — strong support confluence",
		"✅ Uptrend intact — sweep aligns with trend continuation",
		"ℹ️  SMC sweep signals are most reliable on higher timeframes with FVG or OB confluence",
	}
	if !slices.Equal(a.Reasoning, want) {
		t.Errorf("reasoning = %q", a.Reasoning)
	}
	// Risk-off veto: the bot classifies the raw VIX with its own bands (> 20
	// elevated), whatever the stored vix_regime row says.
	in.Indicators[n.Trend] = store.StoredRow{Payload: json.RawMessage(`{"direction": "sideways"}`)}
	in.VIX = ptr(21.0)
	if a := buildActionSignal(in, n); a.Action != "WATCH" || a.Confluence.Score != 3 || *a.VIXRegime != "elevated" {
		t.Errorf("risk-off veto = %+v", a)
	}
	in.VIX = nil
	if a := buildActionSignal(in, n); a.Action != "BUY_WATCH" || a.VIXRegime != nil {
		t.Errorf("no VIX = %+v", a)
	}
}

// A cluster that ran no comparison is stored as score 0, "mixed_positive"
// (ETFs: all four). It is served as not evaluated: checks_run 0, score and
// tier null; the composite too when no cluster ran any.
func TestCorrelations_EmptyClusterIsNotEvaluated(t *testing.T) {
	row := func(v float64, p string) store.StoredRow {
		return store.StoredRow{Value: ptr(v), Payload: json.RawMessage(p)}
	}
	empty := `{"tier": "mixed_positive", "checks_run": 0, "warnings": null, "positives": null}`
	d := map[string]store.StoredRow{
		"corr_summary":            row(0, `{"tier": "mixed_positive"}`),
		"corr_earnings_quality":   row(0, empty),
		"corr_valuation_quality":  row(0, empty),
		"corr_leverage_liquidity": row(0, empty),
		"corr_operational":        row(0, empty),
	}
	c := buildCorrelations(d, CorrelationText{})
	for _, cl := range c.Clusters {
		if cl.Score != nil || cl.Tier != nil || cl.ChecksRun == nil || *cl.ChecksRun != 0 {
			t.Errorf("%s: score %v tier %v checks %v, want null / null / 0", cl.Name, cl.Score, cl.Tier, cl.ChecksRun)
		}
	}
	if c.Composite.Score != nil || c.Composite.Tier != nil {
		t.Errorf("composite = %+v, want null when no cluster was evaluated", c.Composite)
	}

	// One evaluated cluster keeps its reading, and the stored composite stands.
	d["corr_operational"] = row(-0.5, `{"tier": "mixed_negative", "checks_run": 2}`)
	d["corr_summary"] = row(-0.13, `{"tier": "mixed_negative"}`)
	c = buildCorrelations(d, CorrelationText{})
	if op := c.Clusters[3]; op.Tier == nil || *op.Tier != "mixed_negative" || *op.ChecksRun != 2 {
		t.Errorf("operational = %+v, want mixed_negative over 2 checks", op)
	}
	if c.Composite.Tier == nil || *c.Composite.Tier != "mixed_negative" {
		t.Errorf("composite = %+v, want the stored mixed_negative", c.Composite)
	}
}
