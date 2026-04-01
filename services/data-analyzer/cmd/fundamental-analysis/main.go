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
		w.scoreTier2(ctx, sym, rows)
		w.scoreTier3(ctx, sym, rows)
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

// scoreTier2 derives Tier 2 FA signals (reference ranks 06–10) from the same
// raw rows fetched by data-fundamental.  Results are stored with metric names
// prefixed by "t2_" so the analyst-bot and dashboard can query them distinctly.
//
// Tier 2 metrics computed:
//   t2_roe          — Return on Equity tier (rank 06)
//   t2_roa          — Return on Assets (informational)
//   t2_leverage     — D/E ratio tier (rank 07, composite-scored)
//   t2_net_debt_ebitda — Net Debt / EBITDA proxy (rank 07 extended)
//   t2_ev_ebitda    — EV/EBITDA tier (rank 08, informational — sector-dependent)
//   t2_current_ratio — Current ratio tier (rank 10, composite-scored)
//   t2_quick_ratio  — Quick ratio (informational supplement)
//   t2_pb           — Price/Book tier (rank 11, informational)
//   t2_dividend     — Dividend yield + payout sustainability (rank 12)
//   t2_capex_intensity — CapEx as % of revenue (rank 20, informational)
//   t2_health_score — Composite of Tier 2 balance-sheet metrics
//
// TODO: migrate to Python pandas — quarterly series joins and ratio math are
// simpler with DataFrame.resample() and vectorised pct_change().
func (w *worker) scoreTier2(ctx context.Context, symbol string, rows []store.FundamentalRow) {
	latest := make(map[string]float64, len(rows))
	for _, r := range rows {
		if r.Value == nil {
			continue
		}
		if _, exists := latest[r.Metric]; !exists {
			latest[r.Metric] = *r.Value
		}
	}

	ts := time.Now().UTC()
	cfg := w.cfg
	ptr := func(v float64) *float64 { return &v }

	upsert := func(metric string, value *float64, payload any) {
		if err := store.UpsertFundamentalDerived(ctx, w.pool, ts, symbol, "derived", metric, value, payload); err != nil {
			w.log.Error("upsert tier2 metric", "symbol", symbol, "metric", metric, "err", err)
		}
	}

	var t2Score, t2Max float64

	// ── T2.1 Return on Equity (ROE) ───────────────────────────────────────────
	// ROIC/ROE rank 06 from reference. Sustained ROE >15% = Buffett moat signal.
	// Source: Finnhub roeTTM (upserted as roe_ttm by data-fundamental Tier 2 block).
	// Thresholds: FUNDAMENTAL_ROE_EXCELLENT (15) / FUNDAMENTAL_ROE_ADEQUATE (8).
	if roe, ok := latest["roe_ttm"]; ok {
		tier := "destroying_value"
		score := -1.0
		if roe >= cfg.ROEExcellent {
			tier = "excellent"
			score = 1
		} else if roe >= cfg.ROEAdequate {
			tier = "adequate"
			score = 0
		}
		t2Score += score
		t2Max++
		upsert("t2_roe", ptr(roe), map[string]any{
			"roe_pct":    roe,
			"tier":       tier,
			"thresholds": fmt.Sprintf(">%.0f%% excellent (moat), %.0f-%.0f%% adequate, <%.0f%% destroying value", cfg.ROEExcellent, cfg.ROEAdequate, cfg.ROEExcellent, cfg.ROEAdequate),
		})
	}

	// ── T2.2 Return on Assets (ROA) — informational ───────────────────────────
	if roa, ok := latest["roa_ttm"]; ok {
		tier := "low"
		if roa >= 10 {
			tier = "high"
		} else if roa >= 5 {
			tier = "moderate"
		}
		upsert("t2_roa", ptr(roa), map[string]any{
			"roa_pct": roa,
			"tier":    tier,
			"note":    "informational only; >10% high efficiency, 5-10% moderate, <5% low asset utilisation",
		})
	}

	// ── T2.3 Debt-to-Equity leverage ──────────────────────────────────────────
	// Rank 07 from reference. D/E above 2× demands scrutiny of debt maturity.
	// Primary: debt_to_equity_quarterly (Finnhub).
	// Fallback: total_debt_reported / total_equity_reported (XBRL).
	// Thresholds: FUNDAMENTAL_DE_CONSERVATIVE (1.0) / FUNDAMENTAL_DE_MANAGEABLE (2.0).
	var de *float64
	var deSource string
	if v, ok := latest["debt_to_equity_quarterly"]; ok {
		de = ptr(v)
		deSource = "finnhub_quarterly"
	} else if v, ok := latest["debt_to_equity_annual"]; ok {
		de = ptr(v)
		deSource = "finnhub_annual"
	} else if td, okD := latest["total_debt_reported"]; okD {
		if eq, okE := latest["total_equity_reported"]; okE && eq > 0 {
			de = ptr(td / eq)
			deSource = "xbrl_reported"
		}
	}
	if de != nil {
		tier := "high_leverage"
		score := -1.0
		if *de < cfg.DEConservative {
			tier = "conservative"
			score = 1
		} else if *de < cfg.DEManageable {
			tier = "manageable"
			score = 0
		}
		t2Score += score
		t2Max++
		upsert("t2_leverage", de, map[string]any{
			"debt_to_equity": *de,
			"tier":           tier,
			"source":         deSource,
			"thresholds":     fmt.Sprintf("<%.1f× conservative, %.1f-%.1f× manageable, >%.1f× high risk", cfg.DEConservative, cfg.DEConservative, cfg.DEManageable, cfg.DEManageable),
			"note":           "industry context essential — utilities/banks operate at higher D/E safely",
		})
	}

	// ── T2.4 Net Debt / EBITDA ────────────────────────────────────────────────
	// Cleaner leverage metric than D/E as it accounts for cash holdings.
	// Net Debt = total_debt – cash. EBITDA proxy = operating_income (EBIT; D&A excluded).
	// Thresholds: FUNDAMENTAL_NET_DEBT_EBITDA_LOW (2) / FUNDAMENTAL_NET_DEBT_EBITDA_HIGH (4).
	if td, okD := latest["total_debt_reported"]; okD {
		if cash, okC := latest["cash_reported"]; okC {
			netDebt := td - cash
			// Annualise operating_income_reported if available as EBITDA proxy.
			if opInc, ok := latest["operating_income_reported"]; ok && opInc > 0 {
				// Quarterly value × 4 = rough TTM EBITDA proxy (excludes D&A, conservative).
				ebitdaProxy := opInc * 4
				ratio := netDebt / ebitdaProxy
				tier := "high_risk"
				if ratio < 0 {
					tier = "net_cash" // company holds more cash than debt
				} else if ratio < cfg.NetDebtEBITDALow {
					tier = "conservative"
				} else if ratio < cfg.NetDebtEBITDAHigh {
					tier = "manageable"
				}
				upsert("t2_net_debt_ebitda", ptr(ratio), map[string]any{
					"net_debt":      netDebt,
					"ebitda_proxy":  ebitdaProxy,
					"ratio":         ratio,
					"tier":          tier,
					"thresholds":    fmt.Sprintf("<%.0f× conservative, %.0f-%.0f× manageable, >%.0f× high risk", cfg.NetDebtEBITDALow, cfg.NetDebtEBITDALow, cfg.NetDebtEBITDAHigh, cfg.NetDebtEBITDAHigh),
					"note":          "EBITDA proxy = latest quarter operating income × 4 (excludes D&A — slightly conservative)",
				})
			}
		}
	}

	// ── T2.5 EV / EBITDA ──────────────────────────────────────────────────────
	// Rank 08 from reference. Capital-structure neutral — compare within sector.
	// Source: Alpha Vantage EVToEBITDA (stored as ev_to_ebitda by data-fundamental).
	// Informational only — sector medians vary widely (tech 20–30×, utilities 8–12×).
	// Thresholds: FUNDAMENTAL_EV_EBITDA_VALUE (10) / FUNDAMENTAL_EV_EBITDA_FAIR (20).
	if evEBITDA, ok := latest["ev_to_ebitda"]; ok && evEBITDA > 0 {
		tier := "growth_premium_required"
		if evEBITDA < cfg.EVEBITDAValue {
			tier = "value_territory"
		} else if evEBITDA < cfg.EVEBITDAFair {
			tier = "fairly_valued"
		}
		upsert("t2_ev_ebitda", ptr(evEBITDA), map[string]any{
			"ev_to_ebitda": evEBITDA,
			"tier":         tier,
			"thresholds":   fmt.Sprintf("<%.0f× value, %.0f-%.0f× fair, >%.0f× growth premium required", cfg.EVEBITDAValue, cfg.EVEBITDAValue, cfg.EVEBITDAFair, cfg.EVEBITDAFair),
			"note":         "compare within sector only — tech 20-30×, industrials 10-15×, utilities 8-12×",
		})
	}

	// ── T2.6 Current Ratio ────────────────────────────────────────────────────
	// Rank 10 from reference. Can the company meet near-term obligations?
	// Below 1.0 = short-term liabilities exceed liquid assets (not always fatal).
	// Source: Finnhub currentRatioQuarterly → current_ratio_quarterly.
	// Thresholds: FUNDAMENTAL_CURRENT_RATIO_SAFE (1.5) / _MONITOR (1.0).
	var cr *float64
	if v, ok := latest["current_ratio_quarterly"]; ok {
		cr = ptr(v)
	} else if v, ok := latest["current_ratio_annual"]; ok {
		cr = ptr(v)
	}
	if cr != nil {
		tier := "liquidity_risk"
		score := -1.0
		if *cr >= cfg.CurrentRatioSafe {
			tier = "safe"
			score = 1
		} else if *cr >= cfg.CurrentRatioMonitor {
			tier = "monitor"
			score = 0
		}
		t2Score += score
		t2Max++
		upsert("t2_current_ratio", cr, map[string]any{
			"current_ratio": *cr,
			"tier":          tier,
			"thresholds":    fmt.Sprintf(">%.1f safe, %.1f-%.1f monitor, <%.1f liquidity risk", cfg.CurrentRatioSafe, cfg.CurrentRatioMonitor, cfg.CurrentRatioSafe, cfg.CurrentRatioMonitor),
		})
	}

	// ── T2.7 Quick Ratio — informational supplement ───────────────────────────
	if qr, ok := latest["quick_ratio_quarterly"]; ok {
		tier := "low"
		if qr >= 1.0 {
			tier = "adequate"
		} else if qr >= 0.7 {
			tier = "monitor"
		}
		upsert("t2_quick_ratio", ptr(qr), map[string]any{
			"quick_ratio": qr,
			"tier":        tier,
			"note":        "stricter than current ratio — excludes inventory; >1.0 adequate, 0.7-1.0 monitor, <0.7 risk",
		})
	}

	// ── T2.8 Price/Book (P/B) ─────────────────────────────────────────────────
	// Rank 11 from reference. Most relevant for banks, insurers, asset-heavy sectors.
	// Source: Alpha Vantage PriceToBookRatio (stored as price_to_book).
	// Informational only — tech companies with heavy intangibles make P/B less meaningful.
	// Thresholds: FUNDAMENTAL_PB_VALUE (1.5) / FUNDAMENTAL_PB_EXPENSIVE (5.0).
	if pb, ok := latest["price_to_book"]; ok && pb > 0 {
		tier := "fair"
		if pb < cfg.PBValue {
			tier = "value_signal"
		} else if pb > cfg.PBExpensive {
			tier = "limited_safety_margin"
		}
		upsert("t2_pb", ptr(pb), map[string]any{
			"price_to_book": pb,
			"tier":          tier,
			"thresholds":    fmt.Sprintf("<%.1f× value signal (asset-heavy), %.1f-%.0f× fair, >%.0f× limited safety", cfg.PBValue, cfg.PBValue, cfg.PBExpensive, cfg.PBExpensive),
			"note":          "tech/SaaS companies naturally carry high P/B due to intangibles — use EV/EBITDA instead",
		})
	}

	// ── T2.9 Dividend Yield & Payout Ratio ───────────────────────────────────
	// Rank 12 from reference. High yield + low payout = sustainable income.
	// Source: Alpha Vantage DividendYield + PayoutRatio (already stored by data-fundamental).
	// Informational only — growth companies often pay no dividend, which is not negative.
	// Thresholds: FUNDAMENTAL_DIVIDEND_YIELD_MIN/HIGH / FUNDAMENTAL_PAYOUT_RATIO_SAFE/DANGER.
	if divYield, ok := latest["dividend_yield"]; ok {
		// Alpha Vantage returns as decimal (0.0148 = 1.48%)
		yieldPct := divYield * 100

		payload := map[string]any{
			"dividend_yield_pct": yieldPct,
			"note":               "high yield + payout <60% = sustainable income; payout >80% = cut risk",
		}

		sustainability := "no_dividend"
		if yieldPct >= cfg.DividendYieldMin {
			sustainability = "low_yield"
			if yieldPct >= cfg.DividendYieldHigh {
				sustainability = "verify_payout" // high yield — check payout ratio
			} else {
				sustainability = "moderate_yield"
			}
			// Overlay payout ratio if available.
			if payout, ok := latest["payout_ratio"]; ok {
				payoutPct := payout * 100
				payload["payout_ratio_pct"] = payoutPct
				if yieldPct >= cfg.DividendYieldMin && payoutPct < cfg.PayoutRatioSafe {
					sustainability = "sustainable_income"
				} else if payoutPct > cfg.PayoutRatioDanger {
					sustainability = "cut_risk"
				} else {
					sustainability = "monitor_payout"
				}
			}
		}
		payload["sustainability"] = sustainability
		upsert("t2_dividend", ptr(yieldPct), payload)
	}

	// ── T2.10 CapEx Intensity ─────────────────────────────────────────────────
	// Rank 20 from reference. Asset-light businesses (SaaS, brands) keep CapEx <5%.
	// Capital-intensive industries (semis, airlines, mining) >20%.
	// Computed from: capex_reported (XBRL, most recent quarter × 4) ÷ revenue_ttm.
	// Thresholds: FUNDAMENTAL_CAPEX_INTENSITY_LOW (5) / HIGH (20).
	if capex, okC := latest["capex_reported"]; okC {
		if rev, okR := latest["revenue_ttm"]; okR && rev > 0 {
			// capex_reported is quarterly; × 4 to annualise. revenue_ttm is already annual.
			// Capex from CF statements is typically negative; take absolute value.
			annualCapex := absFloat(capex) * 4
			intensityPct := annualCapex / rev * 100
			tier := "moderate_intensity"
			if intensityPct < cfg.CapExIntensityLow {
				tier = "asset_light"
			} else if intensityPct > cfg.CapExIntensityHigh {
				tier = "capital_intensive"
			}
			upsert("t2_capex_intensity", ptr(intensityPct), map[string]any{
				"capex_intensity_pct": intensityPct,
				"annual_capex_proxy":  annualCapex,
				"revenue_ttm":         rev,
				"tier":                tier,
				"thresholds":          fmt.Sprintf("<%.0f%% asset-light, %.0f-%.0f%% moderate, >%.0f%% capital-intensive", cfg.CapExIntensityLow, cfg.CapExIntensityLow, cfg.CapExIntensityHigh, cfg.CapExIntensityHigh),
			})
		}
	}

	// ── T2.11 Tier 2 Balance-Sheet Health Score ───────────────────────────────
	// Composite of the 3 balance-sheet metrics that contribute to scoring:
	// ROE (quality), D/E (leverage), Current Ratio (liquidity).
	// Range: [-1, +1]. Separate from Tier 1 composite to preserve backward compat.
	// TODO: merge Tier 1 + Tier 2 into a single Piotroski-style F-score in Python.
	if t2Max > 0 {
		healthScore := t2Score / t2Max
		tier := "neutral"
		if healthScore >= cfg.CompositeStrong {
			tier = "healthy"
		} else if healthScore <= -cfg.CompositeWeak {
			tier = "stressed"
		}
		upsert("t2_health_score", ptr(healthScore), map[string]any{
			"score":      healthScore,
			"tier":       tier,
			"components": []string{"t2_roe", "t2_leverage", "t2_current_ratio"},
			"method":     "tier2_balance_sheet_v1",
		})
	}

	w.log.Info("tier2 fundamental scores computed", "symbol", symbol, "components", t2Max)
}

// scoreTier3 derives Tier 3 FA context metrics (reference ranks 13–19).
//
// These are "important context" signals — they supplement but do not replace
// Tier 1/2 scoring. No composite score is produced; each metric is stored
// individually with a tier label for the analyst-bot to display.
//
// Metrics computed:
//   t3_share_trend      — rank 13: share count trend (buybacks vs dilution)
//   t3_dcf              — rank 14: simplified DCF margin of safety
//   t3_interest_coverage — rank 15: EBIT ÷ interest expense
//   t3_asset_turnover   — rank 16: revenue ÷ total assets (informational)
//   t3_inventory_turnover — rank 16: COGS ÷ inventory (when available)
//   t3_analyst_target   — rank 17: analyst target price vs current price
//   t3_goodwill_risk    — rank 18: (goodwill+intangibles) as % of total assets
//   t3_ps_ratio         — rank 19: price-to-sales ratio
//
// TODO: Python migration — pandas rolling join for share count series;
// scenario-range DCF with Monte Carlo; Finviz/Seeking Alpha revision trend.
func (w *worker) scoreTier3(ctx context.Context, symbol string, rows []store.FundamentalRow) {
	latest := make(map[string]float64, len(rows))
	for _, r := range rows {
		if r.Value == nil {
			continue
		}
		if _, exists := latest[r.Metric]; !exists {
			latest[r.Metric] = *r.Value
		}
	}

	ts := time.Now().UTC()
	cfg := w.cfg
	ptr := func(v float64) *float64 { return &v }

	upsert := func(metric string, value *float64, payload any) {
		if err := store.UpsertFundamentalDerived(ctx, w.pool, ts, symbol, "derived", metric, value, payload); err != nil {
			w.log.Error("upsert tier3 metric", "symbol", symbol, "metric", metric, "err", err)
		}
	}

	// ── T3.1 Share Count Trend — buybacks vs dilution (rank 13) ──────────────
	// Queries the time-ordered shares_outstanding series from equity_fundamentals.
	// Compares the oldest sampled value to the newest to derive annual % change.
	// Sources: Finnhub shareOutstanding (period = "ttm", source = finnhub_metric).
	// Thresholds: FUNDAMENTAL_SHARE_DECLINE_BUYBACK (2%) / _GROWTH_DILUTION (3%).
	// TODO: Python — use pandas resample('A').last() on shares series for clean annual comparison.
	shareRows, err := w.pool.Query(ctx, `
		SELECT value, ts FROM equity_fundamentals
		WHERE symbol = $1 AND metric = 'shares_outstanding' AND value IS NOT NULL
		ORDER BY ts DESC LIMIT $2`,
		symbol, cfg.ShareTrendYears*12+1) // enough rows to span the trend years
	if err == nil {
		type sharePoint struct {
			v float64
			t time.Time
		}
		var pts []sharePoint
		for shareRows.Next() {
			var v float64
			var t time.Time
			if e := shareRows.Scan(&v, &t); e == nil {
				pts = append(pts, sharePoint{v, t})
			}
		}
		shareRows.Close()

		if len(pts) >= 2 {
			newest := pts[0]
			oldest := pts[len(pts)-1]
			years := newest.t.Sub(oldest.t).Hours() / 8766
			if years > 0.1 {
				annualChg := (newest.v - oldest.v) / oldest.v / years * 100
				tier := "flat"
				if annualChg <= -cfg.ShareDeclineBuyback {
					tier = "buyback"
				} else if annualChg >= cfg.ShareGrowthDilution {
					tier = "dilution_risk"
				}
				upsert("t3_share_trend", ptr(annualChg), map[string]any{
					"annual_change_pct": annualChg,
					"newest_shares":     newest.v,
					"oldest_shares":     oldest.v,
					"years_observed":    years,
					"tier":              tier,
					"thresholds":        fmt.Sprintf("decline >%.0f%%/yr = buyback, flat ±%.0f%%, growth >%.0f%%/yr = dilution", cfg.ShareDeclineBuyback, cfg.ShareDeclineBuyback, cfg.ShareGrowthDilution),
				})
			}
		}
	}

	// ── T3.2 DCF Intrinsic Value — simplified 5-year model (rank 14) ─────────
	// Formula: Σ FCF_t / (1+WACC)^t  for t=1..N + terminal_value / (1+WACC)^N
	// Terminal value = FCF_N × (1+g) / (WACC - g)   [Gordon Growth Model]
	// FCF growth assumption = min(eps_growth_5y, revenue_growth_5y, DCFMaxGrowthPct).
	// Margin of safety = price_as_pct_of_dcf (100% = exactly at fair value).
	// Thresholds: FUNDAMENTAL_DCF_SAFETY_MARGIN_PCT (<70% = strong buy) / _OVERVALUED_PCT (>110%).
	// TODO: Python — add Monte Carlo scenario ranges; CAPM-derived WACC from FRED.
	if fcf, okF := latest["fcf_ttm"]; okF && fcf > 0 {
		if mktCap, okM := latest["market_cap"]; okM && mktCap > 0 {
			// Choose the most conservative FCF growth estimate available.
			growthPct := cfg.DCFMaxGrowthPct
			if eg5, ok := latest["eps_growth_5y"]; ok && eg5 < growthPct {
				growthPct = eg5
			}
			if rg5, ok := latest["revenue_growth_5y"]; ok && rg5 < growthPct {
				growthPct = rg5
			}
			if growthPct < 0 {
				growthPct = 0 // no negative growth assumption for terminal FCF
			}

			wacc := cfg.DCFWACCPct / 100
			g := cfg.DCFTerminalGrowth / 100
			growthR := growthPct / 100
			n := cfg.DCFGrowthYears

			// Explicit FCF growth stage.
			dcfValue := 0.0
			for t := 1; t <= n; t++ {
				fcfT := fcf * math.Pow(1+growthR, float64(t))
				dcfValue += fcfT / math.Pow(1+wacc, float64(t))
			}
			// Terminal value (Gordon Growth Model) — only valid when WACC > g.
			if wacc > g {
				fcfN := fcf * math.Pow(1+growthR, float64(n))
				terminalVal := fcfN * (1 + g) / (wacc - g)
				dcfValue += terminalVal / math.Pow(1+wacc, float64(n))
			}

			// mktCap from Finnhub is in millions (same unit as fcf_ttm).
			// marginPct: 100 = priced exactly at DCF intrinsic; <100 = cheap; >100 = expensive.
			marginPct := mktCap / dcfValue * 100

			tier := "fairly_valued"
			if marginPct < cfg.DCFSafetyMargin {
				tier = "strong_margin_of_safety"
			} else if marginPct > cfg.DCFOvervalued {
				tier = "downside_risk"
			}

			upsert("t3_dcf", ptr(marginPct), map[string]any{
				"market_cap_vs_dcf_pct": marginPct,
				"dcf_value_millions":    dcfValue,
				"market_cap_millions":   mktCap,
				"fcf_ttm_millions":      fcf,
				"growth_rate_pct":       growthPct,
				"wacc_pct":              cfg.DCFWACCPct,
				"terminal_growth_pct":   cfg.DCFTerminalGrowth,
				"growth_years":          n,
				"tier":                  tier,
				"thresholds":            fmt.Sprintf("<%.0f%% strong safety margin, %.0f-%.0f%% fairly valued, >%.0f%% downside risk", cfg.DCFSafetyMargin, cfg.DCFSafetyMargin, cfg.DCFOvervalued, cfg.DCFOvervalued),
				"note":                  "simplified 5-year DCF — use as directional check, not precise target",
			})
		}
	}

	// ── T3.3 Interest Coverage Ratio (rank 15) ─────────────────────────────────
	// Interest Coverage = EBIT ÷ Interest Expense.
	// EBIT proxy = operating_income_reported (most recent quarter × 4 to annualise).
	// Interest expense from XBRL is typically reported as a negative number; abs() applied.
	// Thresholds: FUNDAMENTAL_INTEREST_COVERAGE_SAFE (5×) / _ADEQUATE (2×).
	if opInc, okO := latest["operating_income_reported"]; okO {
		if intExp, okI := latest["interest_expense_reported"]; okI {
			annualEBIT := opInc * 4 // approximate annualisation from quarterly
			absIntExp := absFloat(intExp) * 4
			if absIntExp > 0 {
				coverage := annualEBIT / absIntExp
				tier := "high_risk"
				if coverage >= cfg.InterestCoverageSafe {
					tier = "very_safe"
				} else if coverage >= cfg.InterestCoverageAdequate {
					tier = "adequate"
				}
				upsert("t3_interest_coverage", ptr(coverage), map[string]any{
					"coverage_ratio":  coverage,
					"ebit_proxy":      annualEBIT,
					"interest_annual": absIntExp,
					"tier":            tier,
					"thresholds":      fmt.Sprintf(">%.0f× very safe, %.0f-%.0f× adequate, <%.0f× high risk", cfg.InterestCoverageSafe, cfg.InterestCoverageAdequate, cfg.InterestCoverageSafe, cfg.InterestCoverageAdequate),
					"note":            "EBIT = quarterly operating income × 4; interest expense abs value × 4",
				})
			}
		}
	}

	// ── T3.4 Asset Turnover — operational efficiency (rank 16) ───────────────
	// Asset Turnover = Revenue (TTM) ÷ Total Assets.
	// Informational — compare within sector; declining trend is an early warning.
	// No composite scoring: cross-sector comparison is meaningless.
	// TODO: Python — compute sector-relative z-score once sector classification feed added.
	if revTTM, okR := latest["revenue_ttm"]; okR && revTTM > 0 {
		if totAssets, okA := latest["total_assets_reported"]; okA && totAssets > 0 {
			assetTurnover := revTTM / totAssets
			tier := "low"
			if assetTurnover >= 1.0 {
				tier = "high"
			} else if assetTurnover >= 0.5 {
				tier = "moderate"
			}
			upsert("t3_asset_turnover", ptr(assetTurnover), map[string]any{
				"asset_turnover":    assetTurnover,
				"revenue_ttm":       revTTM,
				"total_assets":      totAssets,
				"tier":              tier,
				"note":              "informational — compare to sector peers over time; >1.0 efficient, <0.5 asset-heavy",
			})
		}
	}

	// ── T3.5 Inventory Turnover (rank 16) ─────────────────────────────────────
	// Inventory Turnover = COGS ÷ Inventory.
	// COGS proxy = revenue_reported - gross_profit_reported (both from XBRL).
	// Slowing inventory turnover signals demand weakness before it hits revenue.
	// Informational only — only meaningful for product companies (not SaaS/services).
	if rev, okR := latest["revenue_reported"]; okR {
		if gp, okG := latest["gross_profit_reported"]; okG {
			cogsProxy := rev - gp // gross_profit = revenue - COGS → COGS = revenue - gross_profit
			if inv, okI := latest["inventory_reported"]; okI && inv > 0 && cogsProxy > 0 {
				inventoryTurnover := (cogsProxy * 4) / inv // annualise quarterly COGS
				upsert("t3_inventory_turnover", ptr(inventoryTurnover), map[string]any{
					"inventory_turnover": inventoryTurnover,
					"cogs_proxy_annual":  cogsProxy * 4,
					"inventory":          inv,
					"note":               "COGS proxy = revenue_reported - gross_profit_reported; slowing turnover = demand warning",
				})
			}
		}
	}

	// ── T3.6 Analyst Target Price — consensus upside/downside (rank 17) ──────
	// Source: Alpha Vantage AnalystTargetPrice (stored as analyst_target_price).
	// Upside % = (target - current_price) / current_price × 100.
	// Current price is fetched from equity_ohlcv (latest daily close).
	// Thresholds: FUNDAMENTAL_ANALYST_UPSIDE_BULLISH (15%) / _DOWNSIDE_BEARISH (-5%).
	// TODO: Add revision trend (30/60/90-day changes) once analyst estimate history is available.
	if target, okT := latest["analyst_target_price"]; okT && target > 0 {
		currentPrice, hasPx, pxErr := store.QueryLatestEquityClose(ctx, w.pool, symbol, cfg.EquityInterval)
		if pxErr == nil && hasPx && currentPrice > 0 {
			upside := (target - currentPrice) / currentPrice * 100
			tier := "neutral"
			if upside >= cfg.AnalystUpsideBullish {
				tier = "bullish_consensus"
			} else if upside <= cfg.AnalystDownsideBearish {
				tier = "bearish_consensus"
			}
			upsert("t3_analyst_target", ptr(upside), map[string]any{
				"upside_pct":     upside,
				"target_price":   target,
				"current_price":  currentPrice,
				"tier":           tier,
				"thresholds":     fmt.Sprintf(">%.0f%% upside = bullish consensus; <%.0f%% = bearish", cfg.AnalystUpsideBullish, cfg.AnalystDownsideBearish),
				"note":           "single analyst target; revision trend requires paid data (Seeking Alpha / Finviz)",
			})
		}
	}

	// ── T3.7 Goodwill & Intangibles as % of Total Assets (rank 18) ───────────
	// Heavy goodwill (>40% of assets) carries impairment write-down risk.
	// Source: XBRL balance sheet goodwill_reported + intangible_assets_reported.
	// Thresholds: FUNDAMENTAL_GOODWILL_LOW_PCT (20%) / _HIGH_PCT (40%).
	if totAssets, okA := latest["total_assets_reported"]; okA && totAssets > 0 {
		goodwill := latest["goodwill_reported"]
		intangibles := latest["intangible_assets_reported"]
		combined := goodwill + intangibles
		if combined > 0 {
			pct := combined / totAssets * 100
			tier := "low_risk"
			if pct >= cfg.GoodwillHighPct {
				tier = "impairment_risk"
			} else if pct >= cfg.GoodwillLowPct {
				tier = "monitor"
			}
			upsert("t3_goodwill_risk", ptr(pct), map[string]any{
				"goodwill_intangibles_pct": pct,
				"goodwill":                 goodwill,
				"intangibles":              intangibles,
				"total_assets":             totAssets,
				"tier":                     tier,
				"thresholds":               fmt.Sprintf("<%.0f%% low risk, %.0f-%.0f%% monitor, >%.0f%% impairment risk", cfg.GoodwillLowPct, cfg.GoodwillLowPct, cfg.GoodwillHighPct, cfg.GoodwillHighPct),
			})
		}
	}

	// ── T3.8 Price-to-Sales (P/S) Ratio (rank 19) ─────────────────────────────
	// P/S = Market Cap ÷ TTM Revenue. Most useful for unprofitable / early-stage growth.
	// Both market_cap and revenue_ttm from Finnhub are in millions → ratio is unit-clean.
	// Thresholds: FUNDAMENTAL_PS_VALUE (5×) / _FAIR (10×) / _SPECULATIVE (15×).
	// Compare within sector — SaaS/tech commands higher P/S than industrials or retail.
	if mktCap, okM := latest["market_cap"]; okM && mktCap > 0 {
		if revTTM, okR := latest["revenue_ttm"]; okR && revTTM > 0 {
			ps := mktCap / revTTM
			tier := "fairly_valued"
			if ps < cfg.PSValue {
				tier = "value"
			} else if ps >= cfg.PSSpeculative {
				tier = "speculative"
			} else if ps >= cfg.PSFair {
				tier = "growth_premium_required"
			}
			upsert("t3_ps_ratio", ptr(ps), map[string]any{
				"ps_ratio":       ps,
				"market_cap_millions": mktCap,
				"revenue_ttm_millions": revTTM,
				"tier":           tier,
				"thresholds":     fmt.Sprintf("<%.0f× value, %.0f-%.0f× fair, %.0f-%.0f× growth premium, >%.0f× speculative", cfg.PSValue, cfg.PSValue, cfg.PSFair, cfg.PSFair, cfg.PSSpeculative, cfg.PSSpeculative),
				"note":           "compare within sector; SaaS 5-15× is normal, industrials >3× is expensive",
			})
		}
	}

	w.log.Info("tier3 fundamental context computed", "symbol", symbol)
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
