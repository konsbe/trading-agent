// fundamental-analysis reads raw fundamental metrics stored by data-ingestion/data-fundamental
// and derives Tier 1 FA signals that the analyst-bot can consume directly.
//
// Data flow:
//   data-fundamental (data-ingestion)
//     → equity_fundamentals (TimescaleDB, source = finnhub_*)
//     → fundamental-analysis (this binary, data-analyzer)
//     → equity_fundamentals (TimescaleDB, source = "fundamental_analysis")
//
// Tier 1 signals derived (period = "derived"):
//   eps_strength            "strong" / "neutral" / "weak" classification
//   revenue_strength        same scale as EPS strength
//   pe_vs_5y_mean           deviation of current P/E from own 5-year mean (%)
//   fcf_yield               FCF ÷ Market Cap × 100
//   fcf_yield_tier          "attractive" / "fair" / "avoid"
//   fcf_eps_divergence      flag: EPS growing fast but FCF yield flat/falling
//   gross_margin_tier       "strong_moat" / "average" / "margin_pressure"
//   net_margin_tier         same scale
//   earnings_surprise_avg   rolling 4-quarter average EPS surprise (%)
//   composite_score         weighted Tier 1 quality score [-1 … +1]
//
// TODO: migrate this worker to Python (analyst-bot or a dedicated service).
// pandas + psycopg3 makes the pivoting and ratio math far simpler.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"github.com/konsbe/trading-agent/services/data-analyzer/internal/config"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/db"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/logx"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/store"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	_ = godotenv.Load()
	cfg, err := config.LoadFundamentalAnalysis()
	if err != nil {
		slog.Error("config", "err", err)
		os.Exit(1)
	}
	log := logx.New(cfg.LogLevel)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("db", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	w := &worker{cfg: cfg, pool: pool, log: log}

	// Allow data-fundamental time to complete its initial ingestion pass before
	// the first scoring run. Controlled by FUNDAMENTAL_STARTUP_DELAY_SECS
	// (default 30). Set to 0 when running against a pre-populated database.
	if delaySecs := fundamentalStartupDelay(); delaySecs > 0 {
		log.Info("waiting for data-fundamental backfill", "delay_secs", delaySecs)
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Duration(delaySecs) * time.Second):
		}
	}

	log.Info("running initial fundamental analysis")
	w.analyzeAll(ctx)

	ticker := time.NewTicker(cfg.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Info("shutdown")
			return
		case <-ticker.C:
			w.analyzeAll(ctx)
		}
	}
}

type worker struct {
	cfg  config.FundamentalAnalysis
	pool *pgxpool.Pool
	log  *slog.Logger
}

func (w *worker) analyzeAll(ctx context.Context) {
	for _, sym := range w.cfg.Symbols {
		rows, err := store.QueryLatestMetrics(ctx, w.pool, sym)
		if err != nil {
			w.log.Error("query metrics", "symbol", sym, "err", err)
			continue
		}
		if len(rows) < w.cfg.MinMetrics {
			w.log.Debug("not enough metrics to score", "symbol", sym, "have", len(rows))
			continue
		}
		w.score(ctx, sym, rows)
		w.analyzeMarginTrend(ctx, sym)
	}
}

// score derives all Tier 1 FA signals from the raw rows fetched by data-fundamental.
//
// Metric name convention: keys in `latest` exactly match the snake_case names
// that data-fundamental/runMetrics() stores — e.g. "eps_ttm", "pe_ratio_ttm".
// Do NOT use Finnhub's original camelCase names here.
//
// TODO: replace with Python pandas logic when migrating to analyst-bot:
//   df = pd.DataFrame(rows).pivot(index='period', columns='metric', values='value')
func (w *worker) score(ctx context.Context, symbol string, rows []store.FundamentalRow) {
	// ── Build lookup maps ────────────────────────────────────────────────────
	// latest: one value per metric name (most recent across all periods).
	// surprises: all eps_surprise_pct values across quarters for rolling avg.
	latest := make(map[string]float64, len(rows))
	var surprises []float64

	for _, r := range rows {
		if r.Value == nil {
			continue
		}
		if _, exists := latest[r.Metric]; !exists {
			latest[r.Metric] = *r.Value
		}
		if r.Metric == "eps_surprise_pct" {
			surprises = append(surprises, *r.Value)
		}
	}

	ts := time.Now().UTC()
	ptr := func(v float64) *float64 { return &v }

	upsert := func(metric string, value *float64, payload any) {
		if err := store.UpsertFundamentalDerived(ctx, w.pool, ts, symbol, "derived", metric, value, payload); err != nil {
			w.log.Error("upsert derived metric", "symbol", symbol, "metric", metric, "err", err)
		}
	}

	var scorePoints, maxPoints float64

	cfg := w.cfg

	// ── 1. EPS Strength ───────────────────────────────────────────────────────
	// Use Finnhub's pre-computed YoY growth where available.
	// Tier 1 thresholds: configurable via FUNDAMENTAL_EPS_GROWTH_STRONG/WEAK.
	//
	// TODO: Python — pd.Series([eps_ttm_by_quarter]).pct_change(4)*100
	if epsGrowth, ok := latest["eps_growth_ttm_yoy"]; ok {
		tier := classifyGrowth(epsGrowth, cfg.EPSGrowthStrong, cfg.EPSGrowthWeak)
		score := growthScore(epsGrowth, cfg.EPSGrowthStrong, cfg.EPSGrowthWeak)
		scorePoints += score
		maxPoints++
		upsert("eps_strength", ptr(score), map[string]any{
			"eps_growth_ttm_yoy_pct": epsGrowth,
			"tier":                   tier,
			"thresholds":             fmt.Sprintf("strong >%.0f%%, neutral %.0f-%.0f%%, weak <%.0f%%", cfg.EPSGrowthStrong, cfg.EPSGrowthWeak, cfg.EPSGrowthStrong, cfg.EPSGrowthWeak),
		})
	} else if epsTTM, okT := latest["eps_ttm"]; okT {
		// Fallback: positive EPS is a minimal pass when growth rate not available.
		s := -1.0
		tier := "weak"
		if epsTTM > 0 {
			s = 0.0
			tier = "neutral"
		}
		scorePoints += s
		maxPoints++
		upsert("eps_strength", ptr(s), map[string]any{
			"eps_ttm": epsTTM,
			"tier":    tier,
			"note":    "growth rate not available; using sign of TTM EPS only",
		})
	}

	// ── 2. Revenue Growth Strength ────────────────────────────────────────────
	// Strong/weak thresholds: FUNDAMENTAL_REV_GROWTH_STRONG/WEAK.
	//
	// TODO: Python — quarterly revenue series from financials-reported for organic growth.
	if revGrowth, ok := latest["revenue_growth_ttm_yoy"]; ok {
		tier := classifyGrowth(revGrowth, cfg.RevGrowthStrong, cfg.RevGrowthWeak)
		score := growthScore(revGrowth, cfg.RevGrowthStrong, cfg.RevGrowthWeak)
		scorePoints += score
		maxPoints++
		upsert("revenue_strength", ptr(score), map[string]any{
			"revenue_growth_ttm_yoy_pct": revGrowth,
			"tier":                       tier,
			"thresholds":                 fmt.Sprintf("strong >%.0f%%, neutral %.0f-%.0f%%, weak <%.0f%%", cfg.RevGrowthStrong, cfg.RevGrowthWeak, cfg.RevGrowthStrong, cfg.RevGrowthWeak),
			"note":                       "organic growth (ex-acquisitions) not yet available",
		})
	}

	// ── 3. P/E Ratio Evaluation ───────────────────────────────────────────────
	// Compare trailing P/E to own 5-year mean — the most actionable single
	// P/E signal. Negative P/E (loss-making) scores as weak.
	// Forward P/E is nil on Finnhub free tier (documented in data-fundamental).
	//
	// TODO: add sector P/E comparison once a sector classification feed is added.
	if pe, ok := latest["pe_ratio_ttm"]; ok {
		pe5y, has5y := latest["pe_ratio_5y_avg"]

		var pePct *float64
		var tier string
		var score float64

		switch {
		case pe <= 0:
			tier = "loss_making"
			score = -1
		case has5y && pe5y > 0:
			// % deviation from own 5-year mean — negative = cheaper than historical.
			// Thresholds: FUNDAMENTAL_PE_5Y_CHEAP_PCT / FUNDAMENTAL_PE_5Y_EXPENSIVE_PCT.
			dev := (pe - pe5y) / pe5y * 100
			pePct = ptr(dev)
			if dev < -cfg.PEVs5YCheapPct {
				tier = "cheap_vs_history"
				score = 1
			} else if dev < cfg.PEVs5YExpPct {
				tier = "fair_vs_history"
				score = 0
			} else {
				tier = "expensive_vs_history"
				score = -1
			}
		case pe < cfg.PEAbsValue:
			// Fallback absolute bands: FUNDAMENTAL_PE_ABS_VALUE / FUNDAMENTAL_PE_ABS_GROWTH.
			tier = "value"
			score = 1
		case pe < cfg.PEAbsGrowth:
			tier = "growth_fair"
			score = 0
		default:
			tier = "expensive"
			score = -0.5
		}

		scorePoints += score
		maxPoints++

		payload := map[string]any{
			"pe_ratio_ttm": pe,
			"tier":         tier,
		}
		if has5y && pe5y > 0 {
			payload["pe_ratio_5y_avg"] = pe5y
			payload["pct_vs_5y_mean"] = pePct
		}
		// Forward P/E comparison is handled separately in step 8 using Alpha Vantage data.
		upsert("pe_vs_5y_mean", pePct, payload)
	}

	// ── 4. FCF Yield ─────────────────────────────────────────────────────────
	// FCF Yield = FCF ÷ Market Cap × 100.
	// Finnhub provides fcf_yield_1y directly; we also derive it from raw values.
	// Tier 1: >5% attractive, 2–5% fair, <2% or negative → avoid.
	//
	// TODO: Python — compute trailing 4-quarter FCF from quarterly CF statements.
	var fcfYield *float64
	var fcfYieldSource string

	if fy, ok := latest["fcf_yield_1y"]; ok && fy != 0 {
		fcfYield = ptr(fy * 100) // Finnhub returns as decimal (0.05 = 5%)
		fcfYieldSource = "fcf_yield_1y × 100"
	} else if fcf, okF := latest["fcf_ttm"]; okF {
		if mktCap, okM := latest["market_cap"]; okM && mktCap > 0 {
			fy := fcf / mktCap * 100
			fcfYield = ptr(fy)
			fcfYieldSource = "fcf_ttm ÷ market_cap × 100"
		}
	}

	if fcfYield != nil {
		// Thresholds: FUNDAMENTAL_FCF_YIELD_ATTRACTIVE / FUNDAMENTAL_FCF_YIELD_FAIR.
		tier := "avoid"
		score := -1.0
		if *fcfYield >= cfg.FCFYieldAttractive {
			tier = "attractive"
			score = 1
		} else if *fcfYield >= cfg.FCFYieldFair {
			tier = "fair"
			score = 0
		}
		scorePoints += score
		maxPoints++
		upsert("fcf_yield", fcfYield, map[string]any{
			"fcf_yield_pct": *fcfYield,
			"tier":          tier,
			"source":        fcfYieldSource,
			"thresholds":    fmt.Sprintf(">%.0f%% attractive, %.0f-%.0f%% fair, <%.0f%% avoid", cfg.FCFYieldAttractive, cfg.FCFYieldFair, cfg.FCFYieldAttractive, cfg.FCFYieldFair),
		})
		upsert("fcf_yield_tier", ptr(score), map[string]any{"tier": tier})
	}

	// ── 5. FCF vs EPS Divergence ──────────────────────────────────────────────
	// One of the most important red flags: rising EPS + falling/negative FCF
	// signals earnings quality risk (accounting manipulation, deferred costs).
	// Also watch for rising FCF + stagnant EPS → undervalued earnings quality.
	//
	// TODO: Python — compute trailing FCF growth rate across quarters for proper trend.
	if epsGrowth, okE := latest["eps_growth_ttm_yoy"]; okE {
		if fcfYield != nil {
			// Thresholds: FUNDAMENTAL_FCF_DIV_EPS_GROWTH / _YIELD_LOW / _YIELD_HIGH.
			divergent := epsGrowth > cfg.FCFDivEPSGrowth && *fcfYield < cfg.FCFDivYieldLow
			quality := "normal"
			score := 0.0
			if divergent {
				quality = "warning_eps_growing_fcf_low"
				score = -1
			} else if epsGrowth > cfg.FCFDivEPSGrowth && *fcfYield >= cfg.FCFDivYieldHigh {
				quality = "high_quality_earnings"
				score = 1
			}
			upsert("fcf_eps_divergence", ptr(score), map[string]any{
				"eps_growth_ttm_yoy_pct": epsGrowth,
				"fcf_yield_pct":          *fcfYield,
				"quality":                quality,
				"rule":                   fmt.Sprintf("EPS growth >%.0f%% with FCF yield <%.0f%% = suspect earnings quality", cfg.FCFDivEPSGrowth, cfg.FCFDivYieldLow),
			})
		}
	}

	// ── 6. Gross Margin Tier ──────────────────────────────────────────────────
	// Gross margin reveals pricing power and production efficiency.
	// >40% = strong moat, 20–40% = average, <20% = margin pressure.
	// Trend direction over 8 quarters is more important than any snapshot.
	//
	// TODO: Python — compute 8-quarter trend from financials-reported series.
	if gm, ok := latest["gross_margin_ttm"]; ok {
		// Finnhub returns margins already in percentage form (e.g. 47.33 = 47.33%).
		// Thresholds: FUNDAMENTAL_GROSS_MARGIN_MOAT / FUNDAMENTAL_GROSS_MARGIN_AVG.
		gmPct := gm
		tier := "margin_pressure"
		score := -1.0
		if gmPct >= cfg.GrossMarginMoat {
			tier = "strong_moat"
			score = 1
		} else if gmPct >= cfg.GrossMarginAvg {
			tier = "average"
			score = 0
		}
		scorePoints += score
		maxPoints++
		upsert("gross_margin_tier", ptr(score), map[string]any{
			"gross_margin_pct": gmPct,
			"tier":             tier,
			"thresholds":       fmt.Sprintf(">%.0f%% strong_moat, %.0f-%.0f%% average, <%.0f%% margin_pressure", cfg.GrossMarginMoat, cfg.GrossMarginAvg, cfg.GrossMarginMoat, cfg.GrossMarginAvg),
		})
	}

	// ── 7. Operating & Net Margin ─────────────────────────────────────────────
	// Stored for downstream analyst-bot context; not weighted in composite score
	// since gross margin already represents margin quality.
	if om, ok := latest["operating_margin_ttm"]; ok {
		// Finnhub returns operating margin already in percentage form.
		upsert("operating_margin_signal", ptr(om), map[string]any{
			"operating_margin_pct": om,
			"tier":                 classifyNetMargin(om, cfg.NetMarginStrong, cfg.NetMarginAvg),
		})
	}
	if nm, ok := latest["net_margin_ttm"]; ok {
		// Finnhub returns net margin already in percentage form (e.g. 27.04 = 27.04%).
		// Thresholds: FUNDAMENTAL_NET_MARGIN_STRONG / FUNDAMENTAL_NET_MARGIN_AVG.
		nmPct := nm
		tier := classifyNetMargin(nmPct, cfg.NetMarginStrong, cfg.NetMarginAvg)
		score := 0.0
		if nmPct >= cfg.NetMarginStrong {
			score = 1
		} else if nmPct < cfg.NetMarginAvg {
			score = -1
		}
		scorePoints += score
		maxPoints++
		upsert("net_margin_tier", ptr(score), map[string]any{
			"net_margin_pct": nmPct,
			"tier":           tier,
			"thresholds":     fmt.Sprintf(">%.0f%% strong, %.0f-%.0f%% average, <%.0f%% pressure", cfg.NetMarginStrong, cfg.NetMarginAvg, cfg.NetMarginStrong, cfg.NetMarginAvg),
		})
	}

	// ── 8. Forward P/E Evaluation (Alpha Vantage) ────────────────────────────
	// Forward P/E is now available from alphavantage_overview rows (source differs
	// from finnhub_metric so it lives in the same QueryLatestMetrics result set).
	// Compare trailing P/E vs forward P/E: compression = earnings expected to grow.
	if fpe, ok := latest["forward_pe"]; ok && fpe > 0 {
		if tpe, ok2 := latest["pe_ratio_ttm"]; ok2 && tpe > 0 {
			// Negative ratio = earnings growing (forward cheaper than trailing = bullish).
			// Flat band: FUNDAMENTAL_PE_COMPRESSION_FLAT.
			compression := (fpe - tpe) / tpe * 100
			direction := "expanding" // market expects lower future earnings
			if compression < -cfg.PECompressionFlat {
				direction = "compressing" // forward P/E cheaper = earnings growing
			} else if compression < cfg.PECompressionFlat {
				direction = "flat"
			}
			upsert("pe_compression", ptr(compression), map[string]any{
				"trailing_pe":    tpe,
				"forward_pe":     fpe,
				"compression_pct": compression,
				"direction":      direction,
				"note":           "negative = forward P/E < trailing = market expects EPS growth",
			})
		}
	}

	// ── 9. PEG Ratio Tier ─────────────────────────────────────────────────────
	// PEG = P/E ÷ EPS growth rate. Accounts for growth in valuation.
	// <1 = undervalued relative to growth, 1–2 = fair, >2 = expensive for the growth rate.
	if peg, ok := latest["peg_ratio"]; ok && peg != 0 {
		// Thresholds: FUNDAMENTAL_PEG_UNDERVALUED / FUNDAMENTAL_PEG_FAIR.
		tier := "expensive_growth"
		score := -0.5
		if peg < cfg.PEGUndervalued {
			tier = "undervalued_growth"
			score = 1
		} else if peg < cfg.PEGFair {
			tier = "fairly_valued_growth"
			score = 0
		}
		scorePoints += score
		maxPoints++
		upsert("peg_tier", ptr(score), map[string]any{
			"peg_ratio":  peg,
			"tier":       tier,
			"thresholds": fmt.Sprintf("<%.0f undervalued, %.0f-%.0f fair, >%.0f expensive", cfg.PEGUndervalued, cfg.PEGUndervalued, cfg.PEGFair, cfg.PEGFair),
		})
	}

	// ── 10. Earnings Surprise Rolling Average ─────────────────────────────────
	// Average EPS surprise (actual vs estimate %) over up to last 4 quarters.
	// Positive average = company consistently beats expectations.
	// Negative = misses that erode market confidence.
	if len(surprises) > 0 {
		// Quarter cap: FUNDAMENTAL_SURPRISE_QUARTERS.
		// Beat/miss thresholds: FUNDAMENTAL_SURPRISE_BEAT_PCT / FUNDAMENTAL_SURPRISE_MISS_PCT.
		n := len(surprises)
		if n > cfg.SurpriseQuarters {
			surprises = surprises[:cfg.SurpriseQuarters]
			n = cfg.SurpriseQuarters
		}
		sum := 0.0
		for _, s := range surprises {
			sum += s
		}
		avg := sum / float64(n)
		tier := "beat"
		if avg < -cfg.SurpriseMissPct {
			tier = "miss"
		} else if avg < cfg.SurpriseBeatPct {
			tier = "inline"
		}
		upsert("earnings_surprise_avg", ptr(avg), map[string]any{
			"avg_surprise_pct": avg,
			"quarters_sampled": n,
			"tier":             tier,
			"note":             fmt.Sprintf("positive = beats consensus; >%.0f%% sustained = strong earnings momentum", cfg.SurpriseBeatPct),
		})
	}

	// ── 11. Composite Tier 1 Score ────────────────────────────────────────────
	// Mean of all scored components: EPS, revenue, P/E vs 5Y, FCF yield,
	// gross margin, net margin, PEG (when available via Alpha Vantage).
	// Normalised to [-1, +1] where +1 = all Tier 1 signals are strong.
	//
	// TODO: replace with Piotroski F-score or trained classifier in Python analyst-bot.
	if maxPoints > 0 {
		// Boundaries: FUNDAMENTAL_COMPOSITE_STRONG / FUNDAMENTAL_COMPOSITE_WEAK.
		composite := scorePoints / maxPoints
		tier := "neutral"
		if composite >= cfg.CompositeStrong {
			tier = "strong"
		} else if composite <= -cfg.CompositeWeak {
			tier = "weak"
		}
		upsert("composite_score", ptr(composite), map[string]any{
			"score":        composite,
			"tier":         tier,
			"score_points": scorePoints,
			"max_points":   maxPoints,
			"components":   []string{"eps_strength", "revenue_strength", "pe_vs_5y", "fcf_yield_tier", "gross_margin_tier", "net_margin_tier", "peg_tier"},
			"method":       "tier1_weighted_v3",
		})
	}

	w.log.Info("fundamental scores computed",
		"symbol", symbol,
		"metrics_used", len(latest),
		"score_components", maxPoints,
	)
}

// analyzeMarginTrend derives the 8-quarter direction of gross, operating, and net
// margins from the quarterly financials-reported rows stored by data-fundamental.
//
// Margin per quarter = income_statement_line ÷ revenue_reported.
// Trend is "expanding", "stable", or "compressing" based on comparing the
// newest 2-quarter mean to the oldest 2-quarter mean in the series.
//
// This is only meaningful once FUNDAMENTAL_FINANCIALS_LIMIT ≥ 4 quarters of
// data have accumulated in equity_fundamentals.
//
// TODO: Python — use pandas rolling().mean() and scipy linregress() for
// a proper slope-based trend test with p-value confidence.
func (w *worker) analyzeMarginTrend(ctx context.Context, symbol string) {
	// Quarter window: FUNDAMENTAL_MARGIN_TREND_QUARTERS.
	nQuarters := w.cfg.MarginTrendQuarters

	revRows, err := store.QueryMetricSeries(ctx, w.pool, symbol, "revenue_reported", nQuarters)
	if err != nil || len(revRows) < 4 {
		return // not enough data yet
	}

	// Build period→revenue map.
	revByPeriod := make(map[string]float64, len(revRows))
	for _, r := range revRows {
		if r.Value != nil {
			revByPeriod[r.Period] = *r.Value
		}
	}

	ts := time.Now().UTC()

	computeTrend := func(numeratorMetric, outputMetric string) {
		numRows, err := store.QueryMetricSeries(ctx, w.pool, symbol, numeratorMetric, nQuarters)
		if err != nil || len(numRows) < 4 {
			return
		}

		type qPoint struct {
			period string
			margin float64
		}
		var points []qPoint
		for _, r := range numRows {
			if r.Value == nil {
				continue
			}
			rev, ok := revByPeriod[r.Period]
			if !ok || rev == 0 {
				continue
			}
			points = append(points, qPoint{
				period: r.Period,
				margin: *r.Value / rev * 100, // convert to %
			})
		}
		if len(points) < 4 {
			return
		}

		// Newest-first from QueryMetricSeries. Compare recent 2Q mean vs oldest 2Q mean.
		n := len(points)
		recentMean := (points[0].margin + points[1].margin) / 2
		oldMean := (points[n-1].margin + points[n-2].margin) / 2

		diff := recentMean - oldMean
		// Stable band: FUNDAMENTAL_MARGIN_TREND_STABLE_PP.
		direction := "stable"
		if diff > w.cfg.MarginTrendStablePP {
			direction = "expanding"
		} else if diff < -w.cfg.MarginTrendStablePP {
			direction = "compressing"
		}

		trendScore := 0.0
		if direction == "expanding" {
			trendScore = 1
		} else if direction == "compressing" {
			trendScore = -1
		}

		payload := map[string]any{
			"direction":    direction,
			"recent_mean":  recentMean,
			"old_mean":     oldMean,
			"diff_pct_pts": diff,
			"quarters":     n,
			"note":         "expanding = improving competitive position; compressing with revenue growth = cost structure breaking",
		}

		if err := store.UpsertFundamentalDerived(ctx, w.pool, ts, symbol, "derived", outputMetric, &trendScore, payload); err != nil {
			w.log.Error("upsert margin trend", "symbol", symbol, "metric", outputMetric, "err", err)
		}
	}

	computeTrend("gross_profit_reported", "gross_margin_trend_8q")
	computeTrend("operating_income_reported", "operating_margin_trend_8q")
	computeTrend("net_income_reported", "net_margin_trend_8q")

	w.log.Info("margin trend computed", "symbol", symbol, "quarters_available", len(revRows))
}

// ── Classification helpers ────────────────────────────────────────────────────

// classifyGrowth returns "strong", "neutral", or "weak" based on thresholds.
func classifyGrowth(pct, strongThresh, weakThresh float64) string {
	if pct >= strongThresh {
		return "strong"
	}
	if pct >= weakThresh {
		return "neutral"
	}
	return "weak"
}

// growthScore maps a growth rate to [-1, +1] using linear thresholds.
func growthScore(pct, strongThresh, weakThresh float64) float64 {
	if pct >= strongThresh {
		return 1
	}
	if pct >= weakThresh {
		return 0
	}
	if pct < 0 {
		return -1
	}
	return -0.5
}

// classifyNetMargin returns tier label for operating or net margin (already in %).
// strong/avg thresholds are read from cfg (FUNDAMENTAL_NET_MARGIN_STRONG/AVG).
func classifyNetMargin(pct, strong, avg float64) string {
	if pct >= strong {
		return "strong"
	}
	if pct >= avg {
		return "average"
	}
	return "pressure"
}

func absFloat(f float64) float64 { return math.Abs(f) }

// fundamentalStartupDelay reads FUNDAMENTAL_STARTUP_DELAY_SECS from the
// environment. Defaults to 30 seconds — enough for data-fundamental to finish
// its first metrics + financials pass before the scoring worker runs.
// Set to 0 to skip the delay (useful when the DB is already populated).
func fundamentalStartupDelay() int {
	s := strings.TrimSpace(os.Getenv("FUNDAMENTAL_STARTUP_DELAY_SECS"))
	if s == "" {
		return 30
	}
	v, err := strconv.Atoi(s)
	if err != nil || v < 0 {
		return 30
	}
	return v
}
