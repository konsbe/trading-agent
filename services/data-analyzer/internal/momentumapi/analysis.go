package momentumapi

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/konsbe/trading-agent/services/data-analyzer/internal/compute"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/heuristics"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/severity"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/store"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/technical"
)

// GET /api/v1/scanner/today/{symbol}/analysis — full-stock-analysis addendum
// Part A. Every band and tier is the stored classification; the only bands
// computed here are RSI and ADX, which no worker stores, through the ported
// bot thresholds in internal/compute (bands.go). Severities come from
// internal/severity. Nothing here re-derives a classification from a number.

// AnalysisInterval is the bar interval the analysis reads and computes: the
// bot's and the scanner's daily equity bars.
const AnalysisInterval = "1Day"

const (
	analysisReady     = "ready"
	analysisComputing = "computing"
	analysisFailed    = "failed"

	sectionReady  = "ready"
	sectionStale  = "stale"
	sectionNoData = "no_data"

	analysisHeadlines = 10
)

// analysisFreshness applies the addendum's freshness rule. Technical rows are
// current when their newest ts is the symbol's latest bar (the ts the worker
// would write now). Derived fundamentals are current when they are newer than
// every raw fundamental row they are computed from and younger than maxAge
// (the insider and news-sentiment passes read NOW()-relative windows, so a
// derived row ages even without new raw rows).
func analysisFreshness(f store.AnalysisFreshness, now time.Time, maxAge time.Duration) (techFresh, faFresh bool) {
	techFresh = f.TechnicalTS != nil && f.LatestBarTS != nil && f.TechnicalTS.Equal(*f.LatestBarTS)
	faFresh = f.DerivedTS != nil && (f.RawTS == nil || !f.DerivedTS.Before(*f.RawTS)) && now.Sub(*f.DerivedTS) <= maxAge
	return techFresh, faFresh
}

func keyOf(f store.AnalysisFreshness) freshnessKey {
	var k freshnessKey
	if f.LatestBarTS != nil {
		k.bar = *f.LatestBarTS
	}
	if f.RawTS != nil {
		k.raw = *f.RawTS
	}
	return k
}

type analysisPending struct {
	Symbol       string `json:"symbol"`
	Status       string `json:"status"`
	Message      string `json:"message"`
	RetryAfterMS int64  `json:"retry_after_ms"`
	Error        string `json:"error,omitempty"`
	ScannerData  bool   `json:"scanner_data"`
}

func (s *Server) handleAnalysis(w http.ResponseWriter, r *http.Request) {
	symbol := strings.ToUpper(strings.TrimSpace(r.PathValue("symbol")))
	key := "analysis:" + symbol
	if body, ok := s.cache.get(key); ok {
		writeBody(w, http.StatusOK, body)
		return
	}
	ctx := r.Context()

	// Same 503s as GET /today/{symbol}.
	if _, ok, err := s.cfg.Store.LatestScanDate(ctx); err != nil {
		s.storeError(w, r, err)
		return
	} else if !ok {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "no_scan_available"})
		return
	}

	fr, err := s.cfg.Store.AnalysisFreshness(ctx, symbol)
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	// Any symbol with daily bars is served, scanned or not: the analysis is
	// computed from bars and fundamentals, not from the scanner's row.
	if fr.LatestBarTS == nil {
		writeJSON(w, http.StatusNotFound, errorResponse{Error: "no_data_for_symbol"})
		return
	}
	now := s.cfg.Now()
	techFresh, faFresh := analysisFreshness(fr, now, s.cfg.FundamentalsMaxAge)
	if (!techFresh || !faFresh) && s.analysis != nil {
		k := keyOf(fr)
		job, seen := s.analysis.get(symbol)
		settled := seen && job.state != jobRunning && job.key == k
		switch {
		case seen && job.state == jobRunning:
			s.writeComputing(w, symbol, fr.ScannerData)
			return
		case settled && job.state == jobFailed && now.Sub(job.finished) < s.cfg.AnalysisFailedRetryAfter:
			s.writeAnalysisFailed(w, symbol, fr.ScannerData, job, now)
			return
		case settled && job.state == jobDone && now.Sub(job.finished) <= s.cfg.FundamentalsMaxAge:
			// Computed against these inputs and still not current: the symbol
			// has too little data for that part. Serve what is stored.
		default:
			s.analysis.start(symbol, k, AnalysisParts{Technical: !techFresh, Fundamentals: !faFresh})
			s.writeComputing(w, symbol, fr.ScannerData)
			return
		}
	}

	in, err := s.cfg.Store.Analysis(ctx, symbol)
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	resp := buildAnalysis(symbol, in, fr, techFresh, faFresh, s.cfg.AnalysisNames, s.cfg.Caveats)
	body, err := json.Marshal(resp)
	if err != nil {
		s.cfg.Log.Error("momentum-api: encode analysis", "err", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "internal_error"})
		return
	}
	// Only a fully current answer is cached: a partial one must be re-checked
	// as soon as the missing part could have been computed.
	if techFresh && faFresh {
		s.cache.set(key, body)
	}
	writeBody(w, http.StatusOK, body)
}

func (s *Server) writeComputing(w http.ResponseWriter, symbol string, scannerData bool) {
	ra := s.cfg.AnalysisRetryAfter
	w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(ra.Seconds()))))
	writeJSON(w, http.StatusAccepted, analysisPending{
		Symbol: symbol, Status: analysisComputing,
		Message:      "Computing analysis for this symbol -- first view only",
		RetryAfterMS: ra.Milliseconds(), ScannerData: scannerData,
	})
}

func (s *Server) writeAnalysisFailed(w http.ResponseWriter, symbol string, scannerData bool, job analysisJob, now time.Time) {
	retry := s.cfg.AnalysisFailedRetryAfter - now.Sub(job.finished)
	if retry < time.Second {
		retry = time.Second
	}
	w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(retry.Seconds()))))
	writeJSON(w, http.StatusInternalServerError, analysisPending{
		Symbol: symbol, Status: analysisFailed, Error: "analysis_failed",
		Message:      "Computing the analysis for this symbol failed; it will be retried on a later request.",
		RetryAfterMS: retry.Milliseconds(), ScannerData: scannerData,
	})
}

// ── Response shape ───────────────────────────────────────────────────────────

type analysisResponse struct {
	Symbol string `json:"symbol"`
	Status string `json:"status"`
	// ScannerData: momentum-scanner has a row for the symbol. False for a
	// symbol served only because it has daily bars (never scanned).
	ScannerData bool `json:"scanner_data"`
	// AsOf is the session (UTC date) of the technical rows.
	AsOf *string `json:"as_of"`
	// FundamentalsComputedAt is when the derived fundamentals were written.
	FundamentalsComputedAt *string           `json:"fundamentals_computed_at"`
	Sections               map[string]string `json:"sections"`

	Technical          technicalOut    `json:"technical"`
	Fundamentals       fundamentalsOut `json:"fundamentals"`
	BalanceSheet       balanceOut      `json:"balance_sheet"`
	Correlations       correlationsOut `json:"correlations"`
	Qualitative        qualitativeOut  `json:"qualitative"`
	Sentiment          sentimentOut    `json:"sentiment"`
	ContextVsBenchmark contextOut      `json:"context_vs_benchmark"`
	HeuristicSignals   heuristicOut    `json:"heuristic_signals"`
}

type valueBand struct {
	Value *float64 `json:"value"`
	Band  *string  `json:"band"`
}

type flaggedValueBand struct {
	Value    *float64 `json:"value"`
	Band     *string  `json:"band"`
	Severity *string  `json:"severity"`
}

type scoreTier struct {
	Score *float64 `json:"score"`
	Tier  *string  `json:"tier"`
}

type valueTier struct {
	Value *float64 `json:"value"`
	Tier  *string  `json:"tier"`
}

type technicalOut struct {
	RSI14 flaggedValueBand `json:"rsi_14"`
	MACD  struct {
		Hist  *float64 `json:"hist"`
		Cross *string  `json:"cross"`
	} `json:"macd"`
	ADX14 valueBand `json:"adx_14"`
	Trend struct {
		Direction *string  `json:"direction"`
		SlopePct  *float64 `json:"slope_pct"`
	} `json:"trend"`
	MACross   *string  `json:"ma_cross"`
	ATR14     *float64 `json:"atr_14"`
	BBSqueeze struct {
		Active   *bool   `json:"active"`
		Severity *string `json:"severity"`
	} `json:"bb_squeeze"`
	VIXRegime flaggedValueBand `json:"vix_regime"`
	Pivots    struct {
		PP *float64 `json:"pp"`
		R1 *float64 `json:"r1"`
		S1 *float64 `json:"s1"`
	} `json:"pivots"`
	SMC struct {
		FVGsActive *int `json:"fvgs_active"`
		OBsActive  *int `json:"obs_active"`
		LiqSweeps  *int `json:"liq_sweeps"`
	} `json:"smc"`
}

type marginOut struct {
	Value *float64 `json:"value"`
	Tier  *string  `json:"tier"`
	Trend *string  `json:"trend"`
}

type fundamentalsOut struct {
	Composite   scoreTier `json:"composite"`
	EPSStrength *string   `json:"eps_strength"`
	Revenue     *string   `json:"revenue"`
	PEVs5Y      valueBand `json:"pe_vs_5y"`
	FCFYield    valueTier `json:"fcf_yield"`
	GrossMargin marginOut `json:"gross_margin"`
	NetMargin   marginOut `json:"net_margin"`
	TTMPE       *float64  `json:"ttm_pe"`
	MarketCap   *float64  `json:"market_cap"`
}

type balanceOut struct {
	Composite     scoreTier `json:"composite"`
	ROE           valueBand `json:"roe"`
	ROA           valueBand `json:"roa"`
	CurrentRatio  valueBand `json:"current_ratio"`
	QuickRatio    valueBand `json:"quick_ratio"`
	DebtToEquity  valueBand `json:"debt_to_equity"`
	NetDebtEBITDA valueBand `json:"net_debt_ebitda"`
	ROIC          valueBand `json:"roic"`
}

type clusterOut struct {
	Name  string   `json:"name"`
	Score *float64 `json:"score"`
	Tier  *string  `json:"tier"`
	// ChecksRun is how many comparisons the cluster ran. At 0 the stored score
	// is 0 and the tier "mixed_positive"; both are served null instead.
	ChecksRun *int     `json:"checks_run"`
	Positives []string `json:"positives"`
	Warnings  []string `json:"warnings"`
}

type correlationsOut struct {
	Composite      scoreTier    `json:"composite"`
	Clusters       []clusterOut `json:"clusters"`
	AlignedSignals []string     `json:"aligned_signals"`
	MasterSignals  struct {
		NetSignal *string  `json:"net_signal"`
		Fired     []string `json:"fired"`
	} `json:"master_signals"`
}

type qualitativeOut struct {
	MoatProxy        valueTier `json:"moat_proxy"`
	InsiderSignal    valueTier `json:"insider_signal"`
	NewsSentiment7d  valueTier `json:"news_sentiment_7d"`
	NewsSentiment30d valueTier `json:"news_sentiment_30d"`
	RDIntensity      valueTier `json:"rd_intensity"`
}

type headlineOut struct {
	Title       string   `json:"title"`
	URL         *string  `json:"url"`
	Source      string   `json:"source"`
	PublishedAt string   `json:"published_at"`
	Sentiment   *float64 `json:"sentiment"`
}

type sentimentOut struct {
	Headlines []headlineOut `json:"headlines"`
}

type contextOut struct {
	BenchmarkSymbol       *string  `json:"benchmark_symbol"`
	MarketCycleComposite  *string  `json:"market_cycle_composite"`
	MarketCycleTone       *string  `json:"market_cycle_tone"`
	PricePhase            *string  `json:"price_phase"`
	DrawdownFromPeakPct   *float64 `json:"drawdown_from_peak_pct"`
	CorrelationRegime     *string  `json:"correlation_regime"`
	CorrelationRegimeTone *string  `json:"correlation_regime_tone"`
	RelativeStrength20dPP *float64 `json:"relative_strength_20d_pp"`
}

type chartPatternOut struct {
	Pattern   string `json:"pattern"`
	Confirmed bool   `json:"confirmed"`
	Severity  string `json:"severity"`
}

type actionSignalOut struct {
	AlertType  string `json:"alert_type"`
	Action     string `json:"action"`
	Confluence struct {
		Score int `json:"score"`
		Max   int `json:"max"`
	} `json:"confluence"`
	Severity string `json:"severity"`
	// VIXRegime is the regime the rule read: the bot's own bands on the newest
	// VIXCLS (heuristics.BotVIXThresholds), not the stored vix_regime row.
	VIXRegime *string  `json:"vix_regime"`
	Reasoning []string `json:"reasoning"`
}

type heuristicOut struct {
	Caveat        string            `json:"caveat"`
	ChartPatterns []chartPatternOut `json:"chart_patterns"`
	ActionSignal  *actionSignalOut  `json:"action_signal"`
}

// ── Building ────────────────────────────────────────────────────────────────

// payload is a stored row's JSON payload, read field by field.
type payload map[string]any

func rowPayload(rows map[string]store.StoredRow, name string) (store.StoredRow, payload, bool) {
	r, ok := rows[name]
	if !ok {
		return r, nil, false
	}
	var p payload
	_ = json.Unmarshal(r.Payload, &p)
	return r, p, true
}

func (p payload) str(key string) *string {
	if v, ok := p[key].(string); ok && v != "" {
		return &v
	}
	return nil
}

func (p payload) num(key string) *float64 {
	if v, ok := p[key].(float64); ok && !math.IsNaN(v) && !math.IsInf(v, 0) {
		return &v
	}
	return nil
}

func (p payload) boolean(key string) *bool {
	if v, ok := p[key].(bool); ok {
		return &v
	}
	return nil
}

func (p payload) sub(key string) payload {
	if m, ok := p[key].(map[string]any); ok {
		return m
	}
	return nil
}

func (p payload) strings(key string) []string {
	out := []string{}
	arr, _ := p[key].([]any)
	for _, v := range arr {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func intPtr(v *float64) *int {
	if v == nil {
		return nil
	}
	i := int(*v)
	return &i
}

func strPtr(s string) *string { return &s }

func levelPtr(l severity.Level, ok bool) *string {
	if !ok {
		return nil
	}
	s := string(l)
	return &s
}

func sectionStatus(stored, fresh bool) string {
	switch {
	case !stored:
		return sectionNoData
	case fresh:
		return sectionReady
	}
	return sectionStale
}

func buildAnalysis(symbol string, in store.AnalysisInputs, fr store.AnalysisFreshness, techFresh, faFresh bool,
	n technical.Names, caveats Caveats) analysisResponse {
	out := analysisResponse{
		Symbol:      symbol,
		Status:      analysisReady,
		ScannerData: fr.ScannerData,
		Sections: map[string]string{
			"technical":    sectionStatus(len(in.Indicators) > 0, techFresh),
			"fundamentals": sectionStatus(len(in.Derived) > 0, faFresh),
		},
	}
	if fr.TechnicalTS != nil {
		out.AsOf = strPtr(fr.TechnicalTS.UTC().Format(time.DateOnly))
	}
	if fr.DerivedTS != nil {
		out.FundamentalsComputedAt = strPtr(fr.DerivedTS.UTC().Format(timeRFC3339))
	}
	out.Technical = buildTechnical(in.Indicators, n)
	out.Fundamentals = buildFundamentals(in)
	out.BalanceSheet = buildBalance(in.Derived)
	out.Correlations = buildCorrelations(in.Derived)
	out.Qualitative = buildQualitative(in.Derived)
	out.Sentiment = sentimentOut{Headlines: []headlineOut{}}
	for _, h := range in.Headlines {
		out.Sentiment.Headlines = append(out.Sentiment.Headlines, headlineOut{
			Title: h.Title, URL: h.URL, Source: h.Source,
			PublishedAt: h.PublishedAt.Format(timeRFC3339), Sentiment: finite(h.Sentiment),
		})
	}
	out.ContextVsBenchmark = buildContext(symbol, in.Macro)
	out.HeuristicSignals = buildHeuristics(in, n, caveats)
	return out
}

func buildTechnical(ind map[string]store.StoredRow, n technical.Names) technicalOut {
	var t technicalOut
	if r, ok := ind[n.RSI]; ok && r.Value != nil {
		t.RSI14.Value = finite(r.Value)
		band := compute.ClassifyRSI(*r.Value, compute.BotRSIThresholds)
		t.RSI14.Band = &band
		switch band {
		case compute.RSIOversold:
			t.RSI14.Severity = levelPtr(severity.ForAlert(severity.RSIOversold, ""))
		case compute.RSIOverbought:
			t.RSI14.Severity = levelPtr(severity.ForAlert(severity.RSIOverbought, ""))
		}
	}
	if r, p, ok := rowPayload(ind, n.MACD); ok {
		t.MACD.Hist = finite(r.Value)
		switch {
		case p.boolean("bullish_cross_line_signal") != nil && *p.boolean("bullish_cross_line_signal"):
			t.MACD.Cross = strPtr("bullish")
		case p.boolean("bearish_cross_line_signal") != nil && *p.boolean("bearish_cross_line_signal"):
			t.MACD.Cross = strPtr("bearish")
		}
	}
	if r, ok := ind[n.ADX]; ok && r.Value != nil {
		t.ADX14.Value = finite(r.Value)
		band := compute.ClassifyADX(*r.Value)
		t.ADX14.Band = &band
	}
	if _, p, ok := rowPayload(ind, n.Trend); ok {
		t.Trend.Direction = p.str("direction")
		t.Trend.SlopePct = p.num("slope_pct")
	}
	if _, p, ok := rowPayload(ind, n.MARibbon); ok {
		switch {
		case p.boolean("golden_cross") != nil && *p.boolean("golden_cross"):
			t.MACross = strPtr("golden_cross")
		case p.boolean("death_cross") != nil && *p.boolean("death_cross"):
			t.MACross = strPtr("death_cross")
		}
	}
	if r, ok := ind[n.ATR]; ok {
		t.ATR14 = finite(r.Value)
	}
	if _, p, ok := rowPayload(ind, n.BBSqueeze); ok {
		t.BBSqueeze.Active = p.boolean("squeeze")
		// The bot's scan fires bb_squeeze on the stored value >= 1.
		if t.BBSqueeze.Active != nil && *t.BBSqueeze.Active {
			t.BBSqueeze.Severity = levelPtr(severity.ForAlert(severity.BBSqueeze, ""))
		}
	}
	if r, p, ok := rowPayload(ind, n.VIXRegime); ok {
		t.VIXRegime.Value = finite(r.Value)
		t.VIXRegime.Band = p.str("regime")
		// Flagged when the bot's vix_elevated alert would fire (VIX > 25), which
		// is stricter than the stored "elevated" band (> 20).
		if r.Value != nil && *r.Value > severity.BotVIXAlertThreshold {
			t.VIXRegime.Severity = levelPtr(severity.ForAlert(severity.VIXElevated, ""))
		}
	}
	if _, p, ok := rowPayload(ind, n.PivotsPriorBar); ok {
		c := p.sub("classic")
		t.Pivots.PP, t.Pivots.R1, t.Pivots.S1 = c.num("PP"), c.num("R1"), c.num("S1")
	}
	if _, p, ok := rowPayload(ind, n.FVG); ok {
		t.SMC.FVGsActive = intPtr(p.num("active_count"))
	}
	if _, p, ok := rowPayload(ind, n.OrderBlocks); ok {
		t.SMC.OBsActive = intPtr(p.num("active_count"))
	}
	if _, p, ok := rowPayload(ind, n.LiquiditySweep); ok {
		t.SMC.LiqSweeps = intPtr(p.num("total_sweeps"))
	}
	return t
}

func derivedScoreTier(d map[string]store.StoredRow, metric string) scoreTier {
	r, p, ok := rowPayload(d, metric)
	if !ok {
		return scoreTier{}
	}
	return scoreTier{Score: finite(r.Value), Tier: p.str("tier")}
}

func derivedValueBand(d map[string]store.StoredRow, metric string) valueBand {
	r, p, ok := rowPayload(d, metric)
	if !ok {
		return valueBand{}
	}
	return valueBand{Value: finite(r.Value), Band: p.str("tier")}
}

func derivedValueTier(d map[string]store.StoredRow, metric string) valueTier {
	b := derivedValueBand(d, metric)
	return valueTier{Value: b.Value, Tier: b.Band}
}

func derivedTier(d map[string]store.StoredRow, metric string) *string {
	_, p, _ := rowPayload(d, metric)
	return p.str("tier")
}

func buildFundamentals(in store.AnalysisInputs) fundamentalsOut {
	d := in.Derived
	f := fundamentalsOut{
		Composite:   derivedScoreTier(d, "composite_score"),
		EPSStrength: derivedTier(d, "eps_strength"),
		Revenue:     derivedTier(d, "revenue_strength"),
		MarketCap:   finite(in.MarketCap),
	}
	if r, p, ok := rowPayload(d, "pe_vs_5y_mean"); ok {
		f.PEVs5Y = valueBand{Value: finite(r.Value), Band: p.str("tier")}
		f.TTMPE = p.num("pe_ratio_ttm")
	}
	if r, ok := d["fcf_yield"]; ok {
		f.FCFYield.Value = finite(r.Value)
	}
	f.FCFYield.Tier = derivedTier(d, "fcf_yield_tier")
	margin := func(tierMetric, pctKey, trendMetric string) marginOut {
		var m marginOut
		if _, p, ok := rowPayload(d, tierMetric); ok {
			m.Value, m.Tier = p.num(pctKey), p.str("tier")
		}
		_, tp, _ := rowPayload(d, trendMetric)
		m.Trend = tp.str("direction")
		return m
	}
	f.GrossMargin = margin("gross_margin_tier", "gross_margin_pct", "gross_margin_trend_8q")
	f.NetMargin = margin("net_margin_tier", "net_margin_pct", "net_margin_trend_8q")
	return f
}

func buildBalance(d map[string]store.StoredRow) balanceOut {
	return balanceOut{
		Composite:     derivedScoreTier(d, "t2_health_score"),
		ROE:           derivedValueBand(d, "t2_roe"),
		ROA:           derivedValueBand(d, "t2_roa"),
		CurrentRatio:  derivedValueBand(d, "t2_current_ratio"),
		QuickRatio:    derivedValueBand(d, "t2_quick_ratio"),
		DebtToEquity:  derivedValueBand(d, "t2_leverage"),
		NetDebtEBITDA: derivedValueBand(d, "t2_net_debt_ebitda"),
		ROIC:          derivedValueBand(d, "t2_roic"),
	}
}

// correlationClusters are the four cluster rows, in the worker's order.
var correlationClusters = []string{"earnings_quality", "valuation_quality", "leverage_liquidity", "operational"}

// masterSignals are corr_master_signals' patterns, each {fired, score, conditions_met}.
var masterSignals = []string{"value_trap", "hidden_value", "bullish_convergence", "deterioration_warning", "leverage_cycle_warning"}

func buildCorrelations(d map[string]store.StoredRow) correlationsOut {
	c := correlationsOut{Composite: derivedScoreTier(d, "corr_summary"), Clusters: []clusterOut{}, AlignedSignals: []string{}}
	c.MasterSignals.Fired = []string{}
	evaluated := 0
	for _, name := range correlationClusters {
		r, p, ok := rowPayload(d, "corr_"+name)
		if !ok {
			continue
		}
		cl := clusterOut{Name: name, Score: finite(r.Value), Tier: p.str("tier"), ChecksRun: intPtr(p.num("checks_run")),
			Positives: correlationDisplayTexts(p.strings("positives")), Warnings: correlationDisplayTexts(p.strings("warnings"))}
		if cl.ChecksRun != nil && *cl.ChecksRun == 0 {
			cl.Score, cl.Tier = nil, nil
		} else {
			evaluated++
		}
		c.Clusters = append(c.Clusters, cl)
		c.AlignedSignals = append(c.AlignedSignals, cl.Positives...)
	}
	// corr_summary averages the four scores, an empty cluster as 0: with none
	// evaluated its 0 / "mixed_positive" describes nothing.
	if len(c.Clusters) > 0 && evaluated == 0 {
		c.Composite = scoreTier{}
	}
	if _, p, ok := rowPayload(d, "corr_master_signals"); ok {
		c.MasterSignals.NetSignal = p.str("net_signal")
		for _, name := range masterSignals {
			if f := p.sub(name).boolean("fired"); f != nil && *f {
				c.MasterSignals.Fired = append(c.MasterSignals.Fired, name)
			}
		}
	}
	return c
}

func buildQualitative(d map[string]store.StoredRow) qualitativeOut {
	return qualitativeOut{
		MoatProxy:        derivedValueTier(d, "qual_moat_proxy"),
		InsiderSignal:    derivedValueTier(d, "qual_insider_signal"),
		NewsSentiment7d:  derivedValueTier(d, "qual_news_sentiment_7d"),
		NewsSentiment30d: derivedValueTier(d, "qual_news_sentiment_30d"),
		RDIntensity:      derivedValueTier(d, "qual_rd_intensity"),
	}
}

func macroPayloadOf(macro map[string]store.MacroRow, metric string) payload {
	r, ok := macro[metric]
	if !ok {
		return nil
	}
	var p payload
	_ = json.Unmarshal(r.Payload, &p)
	return p
}

func buildContext(symbol string, macro map[string]store.MacroRow) contextOut {
	var c contextOut
	mc := macroPayloadOf(macro, "mc_market_cycle")
	c.BenchmarkSymbol, c.MarketCycleComposite, c.MarketCycleTone = mc.str("symbol"), mc.str("composite_phase"), mc.str("tone")
	corr := macroPayloadOf(macro, "mc_macro_correlation")
	c.CorrelationRegime, c.CorrelationRegimeTone = corr.str("regime"), corr.str("tone")
	// Per-instrument phases exist only for the market report's instruments
	// (fixed list + watchlist); an insufficient-history row has no phase.
	if pp := macroPayloadOf(macro, "mc_price_phase:"+symbol); pp != nil && pp.str("unavailable_reason") == nil {
		c.PricePhase, c.DrawdownFromPeakPct = pp.str("price_phase"), pp.num("drawdown_pct")
	}
	return c
}

func pattern(name string, confirmed bool) chartPatternOut {
	l, ok := severity.ForPattern(name, confirmed)
	if !ok {
		// ChartPatterns is test-enforced to be fully mapped.
		panic(fmt.Sprintf("chart pattern %q has no severity", name))
	}
	return chartPatternOut{Pattern: name, Confirmed: confirmed, Severity: string(l)}
}

func buildChartPatterns(ind map[string]store.StoredRow, n technical.Names) []chartPatternOut {
	out := []chartPatternOut{}
	is := func(p payload, key string) bool { b := p.boolean(key); return b != nil && *b }
	if _, p, ok := rowPayload(ind, n.HSPattern); ok {
		if is(p, "hs_found") {
			out = append(out, pattern(severity.HeadShoulders, is(p, "hs_neckline_break")))
		}
		if is(p, "inv_hs_found") {
			out = append(out, pattern(severity.InvHeadShoulders, is(p, "inv_hs_neckline_break")))
		}
	}
	if _, p, ok := rowPayload(ind, n.Flag); ok {
		if is(p, "bull_flag") {
			out = append(out, pattern(severity.BullFlag, true))
		}
		if is(p, "bear_flag") {
			out = append(out, pattern(severity.BearFlag, true))
		}
	}
	if _, p, ok := rowPayload(ind, n.ChartPatternHints); ok {
		if is(p, "double_top_candidate") {
			out = append(out, pattern(severity.DoubleTop, false))
		}
		if is(p, "double_bottom_candidate") {
			out = append(out, pattern(severity.DoubleBottom, false))
		}
	}
	if _, p, ok := rowPayload(ind, n.Triangle); ok {
		kind := p.str("kind")
		breakout := p.str("breakout")
		confirmed := breakout != nil && (*breakout == "up" || *breakout == "down")
		if kind != nil {
			switch *kind {
			case "ascending":
				out = append(out, pattern(severity.AscendingTriangle, confirmed))
			case "descending":
				out = append(out, pattern(severity.DescendingTriangle, confirmed))
			case "symmetrical":
				out = append(out, pattern(severity.SymmetricalTriangle, confirmed))
			}
		}
	}
	return out
}

// buildActionSignal runs the live liquidity-sweep rule exactly when the bot's
// scan would raise a liquidity_sweep alert for these rows (stored sweep count
// > 0), on the inputs the bot's actions engine gives it.
func buildActionSignal(in store.AnalysisInputs, n technical.Names) *actionSignalOut {
	r, lp, ok := rowPayload(in.Indicators, n.LiquiditySweep)
	if !ok || r.Value == nil || *r.Value <= 0 {
		return nil
	}
	var rin heuristics.SweepRuleInput
	if last := lp.sub("last_sweep"); last != nil {
		if k := last.str("kind"); k != nil {
			rin.Kind = *k
		}
		rin.BarClose, rin.SweptLevel = last.num("bar_close"), last.num("swept_level")
	}
	if _, op, ok := rowPayload(in.Indicators, n.OrderBlocks); ok {
		// Python bool(dict): a missing, null or empty object is false.
		rin.BullishOB = len(op.sub("last_bullish_ob")) > 0
		rin.BearishOB = len(op.sub("last_bearish_ob")) > 0
	}
	if _, tp, ok := rowPayload(in.Indicators, n.Trend); ok {
		if d := tp.str("direction"); d != nil {
			rin.TrendDir = *d
		}
	}
	var regime *string
	if in.VIX != nil {
		rg := compute.ClassifyVIX(*in.VIX, heuristics.BotVIXThresholds)
		rin.VIXRegime, regime = rg, &rg
	}
	total := 0
	if t := lp.num("total_sweeps"); t != nil {
		total = int(*t)
	}
	res := heuristics.EvaluateSweepRule(rin)
	a := &actionSignalOut{
		AlertType: severity.LiquiditySweep, Action: res.Action,
		Severity:  string(severity.ForSweepAction(res.Action, res.Confluence)),
		VIXRegime: regime, Reasoning: heuristics.SweepRuleReasons(rin, total),
	}
	a.Confluence.Score, a.Confluence.Max = res.Confluence, severity.MaxConfluence
	return a
}

func buildHeuristics(in store.AnalysisInputs, n technical.Names, caveats Caveats) heuristicOut {
	return heuristicOut{
		Caveat:        caveats.HeuristicTA,
		ChartPatterns: buildChartPatterns(in.Indicators, n),
		ActionSignal:  buildActionSignal(in, n),
	}
}
