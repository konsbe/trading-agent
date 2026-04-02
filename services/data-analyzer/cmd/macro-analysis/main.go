// macro-analysis reads raw FRED observations stored by data-ingestion/data-equity
// and derives macro signals that the analyst-bot can consume directly.
//
// Data flow:
//   data-equity (data-ingestion)
//     → macro_fred (TimescaleDB, raw FRED series observations)
//     → macro-analysis (this binary, data-analyzer)
//     → macro_derived (TimescaleDB, source = "macro_analysis")
//
// Monetary Policy signals (analyzeMonetary):
//   mp_rate, mp_yield_curve, mp_real_rate, mp_balance_sheet,
//   mp_credit_spread, mp_breakeven_inflation, mp_treasury_yields,
//   mp_m2_supply, mp_stance
//
// Growth Cycle signals (analyzeGrowth) — all free FRED data:
//   gc_pmi            ISM Manufacturing PMI (NAPM)
//   gc_lei            Conference Board LEI level + 6m trend (USSLIND)
//   gc_claims         Initial + continuing jobless claims (ICSA / CCSA)
//   gc_housing        Housing starts + permits (HOUST / PERMIT)
//   gc_gdp            Real GDP annualized QoQ growth (GDPC1)
//   gc_employment     Nonfarm payrolls + unemployment + AHE + Sahm Rule
//   gc_consumer       Retail sales YoY (RRSFS) + Michigan sentiment (UMCSENT)
//   gc_capex          Core capex trend (NEWORDER)
//   gc_stance         Composite weighted score (-1 contraction … +1 expansion)
//
// TODO [LLM]:  Score FOMC statements and minutes hawkish/dovish on -5 to +5 scale.
// TODO [PAID]: CME FedWatch implied rate probabilities (requires CME API subscription).
// TODO [PAID]: S&P Global (Markit) Manufacturing PMI — higher frequency and sector breakdown.
// TODO [PAID]: ISM Services PMI — requires ISM membership or paid feed.
// TODO [PAID]: China Caixin PMI / Eurozone / UK / Japan PMI — paid subscriptions.
// TODO [SCRAPE]: GDPNow (Atlanta Fed real-time GDP) — no public API; needs web scraping.
// TODO [PAID]: ADP National Employment Report — no free API.
// TODO [FUTURE]: Inflation panel — CPI (CPIAUCSL), Core PCE (PCEPILFE), PPI (WPSFD4131).
// TODO [FUTURE]: Global panel — ECB rate (ECBDFR), BoE (UKBANKRATE), EM stress signals.
// TODO [FUTURE]: Equity Risk Premium — link FA composite P/E → forward ERP calculation.
// TODO [PYTHON]: Migrate to Python + asyncpg once the LLM scoring layer is added.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"github.com/konsbe/trading-agent/services/data-analyzer/internal/config"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/db"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/logx"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/store"
)

func main() {
	_ = godotenv.Load()
	cfg, err := config.LoadMacroAnalysis()
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

	w := &worker{cfg: cfg, growthCfg: config.LoadGrowthCycle(), pool: pool, log: log}

	// Wait for data-equity to complete its initial FRED fetch before the first
	// macro-analysis pass. Without a delay, macro-analysis runs immediately and
	// finds empty macro_fred rows → stores "insufficient_data" stance.
	// Controlled by DATA_MACRO_ANALYSIS_STARTUP_DELAY_SECS (default 120).
	if delaySecs := macroStartupDelay(); delaySecs > 0 {
		log.Info("waiting for data-equity FRED backfill", "delay_secs", delaySecs)
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Duration(delaySecs) * time.Second):
		}
	}

	log.Info("running initial macro analysis")
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

func macroStartupDelay() int {
	v := os.Getenv("DATA_MACRO_ANALYSIS_STARTUP_DELAY_SECS")
	if v == "" {
		return 120
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return 120
	}
	return n
}

type worker struct {
	cfg        config.MacroAnalysis
	growthCfg  config.GrowthCycle
	pool       *pgxpool.Pool
	log        *slog.Logger
}

func (w *worker) analyzeAll(ctx context.Context) {
	w.analyzeMonetary(ctx)
	w.analyzeGrowth(ctx)
	// TODO [FUTURE]: w.analyzeInflation(ctx)  — CPI, Core PCE, PPI, OER
	// TODO [FUTURE]: w.analyzeGlobal(ctx)     — China PMI, EM risk, global CBs
	// TODO [FUTURE]: w.analyzeCycles(ctx)     — composite regime detection
}

// analyzeMonetary computes all monetary-policy signals from FRED series stored
// in macro_fred and writes classified signals to macro_derived.
func (w *worker) analyzeMonetary(ctx context.Context) {
	ts := time.Now().UTC()
	cfg := w.cfg

	ptr := func(v float64) *float64 { return &v }

	upsert := func(metric string, value *float64, payload any) {
		if err := store.UpsertMacroDerived(ctx, w.pool, ts, metric, value, payload); err != nil {
			w.log.Error("upsert macro derived", "metric", metric, "err", err)
		}
	}

	// fredLatest returns (value, ok) for the most recent observation of a FRED series.
	fredLatest := func(seriesID string) (float64, bool) {
		v, _, ok := store.QueryMacroFredLatest(ctx, w.pool, seriesID)
		return v, ok
	}

	// fredNAgo returns the value from N observations ago in a series (newest-first).
	// Returns nil if the series has fewer than N+1 observations.
	fredNAgo := func(seriesID string, n int) *float64 {
		rows, err := store.QueryMacroFredSeries(ctx, w.pool, seriesID, n+1)
		if err != nil || len(rows) <= n {
			return nil
		}
		v := rows[n].Value
		return &v
	}

	// fredMinInWindow returns the minimum value across the last n observations.
	fredMinInWindow := func(seriesID string, n int) *float64 {
		rows, err := store.QueryMacroFredSeries(ctx, w.pool, seriesID, n)
		if err != nil || len(rows) == 0 {
			return nil
		}
		min := rows[0].Value
		for _, r := range rows[1:] {
			if r.Value < min {
				min = r.Value
			}
		}
		return &min
	}

	// ── Tier 1.1 — Policy Rate (FEDFUNDS, monthly) ────────────────────────────
	// FEDFUNDS = effective federal funds rate in percent (e.g. 5.33 = 5.33%).
	// YoY direction distinguishes hiking/cutting/neutral regime.
	//
	// TODO [LLM]:  Integrate FOMC statement hawkish/dovish scoring.
	// TODO [PAID]: CME FedWatch implied rate probabilities (CME API required).
	// TODO [FUTURE]: ECB (ECBDFR), BoE (UKBANKRATE), BoJ via FRED series.
	var mpRateRegime string
	var mpRateScore float64

	ff, ffOK := fredLatest("FEDFUNDS")
	if ffOK {
		ffYearAgo := fredNAgo("FEDFUNDS", 12) // monthly series; 12 periods = 1 year
		var changeYoYBps float64
		if ffYearAgo != nil {
			changeYoYBps = (ff - *ffYearAgo) * 100 // convert pct points to bps
		}
		switch {
		case changeYoYBps > 25:
			mpRateRegime = "hiking"
			mpRateScore = -1
		case changeYoYBps < -25:
			mpRateRegime = "cutting"
			mpRateScore = 1
		default:
			mpRateRegime = "neutral"
			mpRateScore = 0
		}
		upsert("mp_rate", ptr(ff), map[string]any{
			"fedfunds_pct":    ff,
			"change_yoy_bps":  changeYoYBps,
			"regime":          mpRateRegime,
			"thresholds":      "hiking: YoY change >+25bps | neutral: ±25bps | cutting: <-25bps",
			"strategy":        "cutting=bullish growth/tech, buy duration | neutral=stock-picking | hiking=value>growth, banks outperform",
			"todo_cme_watch":  "CME FedWatch rate probabilities require paid CME API — adds real-time forward rate expectations",
			"todo_fomc_llm":   "FOMC statement hawkish/dovish scoring scheduled for LLM layer integration",
		})
	}

	// ── Tier 1.2 — Yield Curve (T10Y2Y + T10Y3M, daily, in pp) ──────────────
	// T10Y2Y = 10Y minus 2Y Treasury yield spread (percentage points).
	// T10Y3M = 10Y minus 3-month Treasury yield spread.
	// The 3m10y spread is the most statistically reliable recession predictor.
	// Re-steepening AFTER inversion signals recession is arriving — not just warning.
	var ycRegime string
	var ycScore float64

	yc2s10s, yc2sOK := fredLatest("T10Y2Y")
	yc3m10y, _ := fredLatest("T10Y3M")
	if yc2sOK {
		minYC := fredMinInWindow("T10Y2Y", cfg.YCLookbackDays)
		resteepening := minYC != nil &&
			*minYC < cfg.YCInvertedThreshold &&
			yc2s10s > *minYC+cfg.YCRestepeningBps &&
			yc2s10s > cfg.YCFlatThreshold

		switch {
		case resteepening:
			ycRegime = "re_steepening" // worst signal — recession likely arriving now
			ycScore = -2
		case yc2s10s > cfg.YCSteepThreshold:
			ycRegime = "steep"
			ycScore = 1
		case yc2s10s > cfg.YCFlatThreshold:
			ycRegime = "normal"
			ycScore = 0.5
		case yc2s10s > cfg.YCInvertedThreshold:
			ycRegime = "flat"
			ycScore = -0.5
		default:
			ycRegime = "inverted"
			ycScore = -1
		}

		payload := map[string]any{
			"spread_2s10s_pct": yc2s10s,
			"spread_3m10y_pct": yc3m10y,
			"regime":           ycRegime,
			"thresholds":       fmt.Sprintf(">%.1f steep | 0–%.1f normal | %.1f–0 flat | <%.1f inverted", cfg.YCSteepThreshold, cfg.YCSteepThreshold, cfg.YCInvertedThreshold, cfg.YCInvertedThreshold),
			"note":             "3m10y is most reliable recession predictor (12-18 month lag). Re-steepening AFTER inversion = recession arriving. Source: FRED T10Y2Y / T10Y3M.",
		}
		if minYC != nil {
			payload["lookback_min_pct"] = *minYC
		}
		upsert("mp_yield_curve", ptr(yc2s10s), payload)
	}

	// ── Tier 1.3 — Real Interest Rate (DFII10, daily, in %) ──────────────────
	// DFII10 = 10Y TIPS yield = cleanest real-time real rate measure.
	// Deeply negative real rates drive capital into risk assets, gold, real estate.
	// Rapidly rising real rates = most destabilizing macro event for growth stocks.
	var rrRegime string
	var rrScore float64

	realRate, rrOK := fredLatest("DFII10")
	if rrOK {
		switch {
		case realRate < cfg.RealRateDeeplyNeg:
			rrRegime = "deeply_negative" // max risk-on; 2020-21 bubble environment
			rrScore = 1
		case realRate < cfg.RealRateHeadwind:
			rrRegime = "balanced" // normal equity environment
			rrScore = 0
		default:
			rrRegime = "headwind" // significant drag on growth stocks and gold
			rrScore = -1
		}
		be10y, _ := fredLatest("T10YIE")
		upsert("mp_real_rate", ptr(realRate), map[string]any{
			"real_rate_10y_pct":  realRate,
			"breakeven_10y_pct":  be10y,
			"regime":             rrRegime,
			"thresholds":         fmt.Sprintf("<%.0f%% deeply negative | %.0f–%.0f%% balanced | >%.0f%% headwind", cfg.RealRateDeeplyNeg, cfg.RealRateDeeplyNeg, cfg.RealRateHeadwind, cfg.RealRateHeadwind),
			"note":               "2020-21 deeply negative real rates caused asset bubble. 2022 rapid rise to +2% was most destabilizing event for growth stocks. Source: FRED DFII10.",
		})
	}

	// ── Tier 1.4 — Fed Balance Sheet / QE-QT (WALCL, weekly, in millions USD) ─
	// WALCL is reported weekly in millions of dollars.
	// QE = balance sheet expanding (liquidity injection → suppresses yields).
	// QT = balance sheet contracting (liquidity removal → upward yield pressure).
	var bsRegime string
	var bsScore float64

	walcl, bsOK := fredLatest("WALCL")  // millions USD
	if bsOK {
		walcl4wAgo := fredNAgo("WALCL", 4) // 4 weekly observations back
		var change4wBn float64              // in billions
		if walcl4wAgo != nil {
			change4wBn = (walcl - *walcl4wAgo) / 1000 // millions → billions
		}
		switch {
		case change4wBn > cfg.BSExpandThresholdBn:
			bsRegime = "qe"
			bsScore = 1
		case change4wBn < -cfg.BSContractThresholdBn:
			bsRegime = "qt"
			bsScore = -1
		default:
			bsRegime = "neutral"
			bsScore = 0
		}
		upsert("mp_balance_sheet", ptr(walcl/1000), map[string]any{ // store in billions
			"total_assets_billions": math.Round(walcl/1000*10) / 10,
			"4w_change_billions":    math.Round(change4wBn*10) / 10,
			"regime":                bsRegime,
			"thresholds":            fmt.Sprintf("4w change >+%.0fBn=QE | <-%.0fBn=QT | else neutral", cfg.BSExpandThresholdBn, cfg.BSContractThresholdBn),
			"note":                  "Fed balance sheet peaked ~$9T (2022). QT pace 2022-24 ~$60-95B/month. Material pace change signals policy shift before formal announcement. Source: FRED WALCL.",
		})
	}

	// ── Tier 1.5 — Credit Spreads (BAMLH0A0HYM2 + BAMLC0A0CM, daily, in %) ──
	// ICE BofA series measure option-adjusted spread vs equivalent Treasuries.
	// Series values are in percent; multiply × 100 for basis-point display.
	// Credit stress consistently precedes equity drawdowns by 4–8 weeks.
	//
	// NOTE: TEDRATE (TED Spread) was discontinued by FRED in May 2023.
	// HY OAS is now used as the primary credit stress indicator.
	// TODO [FUTURE]: SOFR-OIS spread as modern interbank stress proxy — FRED SOFR + DFF.
	var creditRegime string
	var creditScore float64

	hyPct, hyOK := fredLatest("BAMLH0A0HYM2") // in percent (1.0 = 100 bps)
	igPct, _ := fredLatest("BAMLC0A0CM")
	if hyOK {
		hyBps := hyPct * 100 // convert to basis points for thresholds
		igBps := igPct * 100
		switch {
		case hyBps > cfg.HYCrisisThreshold:
			creditRegime = "crisis"
			creditScore = -2
		case hyBps > cfg.HYElevatedThreshold:
			creditRegime = "elevated"
			creditScore = -1
		default:
			creditRegime = "benign"
			creditScore = 1
		}
		upsert("mp_credit_spread", ptr(hyBps), map[string]any{
			"hy_spread_bps":  hyBps,
			"ig_spread_bps":  igBps,
			"regime":         creditRegime,
			"thresholds":     fmt.Sprintf("<%.0fbps benign | %.0f–%.0fbps elevated | >%.0fbps crisis", cfg.HYElevatedThreshold, cfg.HYElevatedThreshold, cfg.HYCrisisThreshold, cfg.HYCrisisThreshold),
			"note":           "Credit stress leads equity drawdowns 4-8 weeks. 2020 peak 1100bps, 2009 peak 1900bps. TEDRATE discontinued May 2023.",
			"todo_ted":       "TEDRATE discontinued FRED May 2023. Consider SOFR-OIS spread as modern interbank stress proxy.",
			"source":         "FRED BAMLH0A0HYM2 (HY) / BAMLC0A0CM (IG)",
		})
	}

	// ── Tier 2.1 — Breakeven Inflation (T10YIE + T5YIE, daily, in %) ─────────
	// Breakeven = nominal yield minus TIPS yield = bond market inflation expectation.
	// Rising rapidly signals market expects Fed to hike → leading rate signal.
	//
	// TODO [PAID]: 5Y5Y forward inflation swap (Fed's preferred anchor measure) via Bloomberg.
	var beRegime string
	var beScore float64

	be10y, be10yOK := fredLatest("T10YIE")
	be5y, _ := fredLatest("T5YIE")
	if be10yOK {
		switch {
		case be10y > cfg.BreakevenUnanchoredPct:
			beRegime = "unanchored" // Fed will act aggressively; 2022 scenario
			beScore = -1
		case be10y > cfg.BreakevenRisingPct:
			beRegime = "rising" // growing risk; watch closely
			beScore = -0.5
		default:
			beRegime = "anchored" // Fed comfortable
			beScore = 0.5
		}
		upsert("mp_breakeven_inflation", ptr(be10y), map[string]any{
			"breakeven_10y_pct":  be10y,
			"breakeven_5y_pct":   be5y,
			"regime":             beRegime,
			"thresholds":         fmt.Sprintf("<%.1f%% anchored | %.1f–%.1f%% rising | >%.1f%% unanchored", cfg.BreakevenRisingPct, cfg.BreakevenRisingPct, cfg.BreakevenUnanchoredPct, cfg.BreakevenUnanchoredPct),
			"note":               "Stable 2.0-2.5% = Fed comfortable. Rising >2.5% = unanchoring risk. >3% = 2022 scenario, expect aggressive hikes. Source: FRED T10YIE / T5YIE.",
			"todo_5y5y":          "5Y5Y forward inflation swap (Fed's preferred long-run anchor) requires Bloomberg terminal or paid ICE data.",
		})
	}

	// ── Tier 2.2 — Treasury Yield Term Structure (DGS2/10/30, daily, in %) ───
	// Benchmark rates for all asset valuations globally.
	// Every +100bps in 10Y compresses equity P/E by ~10-15% via discount rate effect.
	//
	// TODO [FUTURE]: Equity Risk Premium = (1/P·E × 100) − DGS10.
	// Requires S&P composite P/E from fundamental-analysis derived table.
	dgs2, _ := fredLatest("DGS2")
	dgs10, dgs10OK := fredLatest("DGS10")
	dgs30, _ := fredLatest("DGS30")
	if dgs10OK {
		upsert("mp_treasury_yields", ptr(dgs10), map[string]any{
			"2y_pct":  dgs2,
			"10y_pct": dgs10,
			"30y_pct": dgs30,
			"note":    "Every +100bps in 10Y reduces equity fair value ~10-15% via discount rate. Earnings Yield = 1/PE*100 vs 10Y = Equity Risk Premium. ERP <1% = equities expensive vs bonds.",
			"todo_erp": "Full Equity Risk Premium calculation requires S&P composite P/E from fundamental-analysis derived table (future cross-service query).",
			"source":  "FRED DGS2 / DGS10 / DGS30",
		})
	}

	// ── Tier 2.3 — M2 Money Supply (M2SL, monthly, in billions USD) ──────────
	// M2 YoY growth rate leads inflation by 12–24 months.
	// M2 contraction in 2022-23 was first since 1930s — lagged deflationary signal.
	//
	// TODO [FUTURE]: M2V (velocity, quarterly) adds monetarist context but low signal frequency.
	var m2Regime string
	var m2Score float64

	m2, m2OK := fredLatest("M2SL") // in billions USD
	if m2OK {
		m2YearAgo := fredNAgo("M2SL", 12) // monthly series; 12 = 1 year
		var yoyPct float64
		if m2YearAgo != nil && *m2YearAgo > 0 {
			yoyPct = (m2 - *m2YearAgo) / *m2YearAgo * 100
		}
		switch {
		case yoyPct > cfg.M2InflationaryPct:
			m2Regime = "inflationary" // inflationary surge incoming 12-24 months
			m2Score = -1
		case yoyPct >= cfg.M2NormalMin:
			m2Regime = "normal"
			m2Score = 0
		case yoyPct >= 0:
			m2Regime = "slow" // slowing but not contracting
			m2Score = 0.25
		default:
			m2Regime = "deflationary" // rare; historically deflationary pressure
			m2Score = 0.5             // deflationary = rates eventually fall = bullish duration
		}
		upsert("mp_m2_supply", ptr(yoyPct), map[string]any{
			"m2_billions": m2,
			"yoy_pct":     fmt.Sprintf("%.2f", yoyPct),
			"regime":      m2Regime,
			"thresholds":  fmt.Sprintf(">%.0f%% inflationary | %.0f–%.0f%% normal | <0%% deflationary", cfg.M2InflationaryPct, cfg.M2NormalMin, cfg.M2InflationaryPct),
			"note":        "M2 leads inflation 12-24 months. 2020: +27% surge → 2021-22 inflation. 2022-23: first contraction since 1930s → lagged disinflation. Source: FRED M2SL.",
			"todo_m2v":    "M2 Velocity (M2V, quarterly) provides monetarist context but updates infrequently.",
		})
	}

	// ── Composite Monetary Policy Stance ─────────────────────────────────────
	// Weighted sum of all individual signals.
	// Positive = accommodative (bullish for risk assets).
	// Negative = restrictive (headwind for risk assets).
	type signalEntry struct {
		name   string
		score  float64
		weight float64
		ok     bool
	}

	signals := []signalEntry{
		{"rate_regime",      mpRateScore,  2.0, ffOK},
		{"yield_curve",      ycScore,      2.0, yc2sOK},
		{"real_rate",        rrScore,      1.5, rrOK},
		{"balance_sheet",    bsScore,      1.0, bsOK},
		{"credit_spread",    creditScore,  2.0, hyOK},
		{"breakeven_infl",   beScore,      1.0, be10yOK},
		{"m2_supply",        m2Score,      0.5, m2OK},
	}

	var weightedSum, totalWeight float64
	var usedSignals []string
	for _, s := range signals {
		if !s.ok {
			continue
		}
		weightedSum += s.score * s.weight
		totalWeight += s.weight
		usedSignals = append(usedSignals, s.name)
	}

	var stance string
	var stanceScore float64
	if totalWeight > 0 {
		stanceScore = weightedSum / totalWeight
		// Clamp to [-1, +1]
		if stanceScore > 1 {
			stanceScore = 1
		} else if stanceScore < -1 {
			stanceScore = -1
		}
		switch {
		case stanceScore > cfg.MPAccommodativeScore:
			stance = "accommodative"
		case stanceScore > cfg.MPRestrictiveScore:
			stance = "neutral"
		default:
			stance = "restrictive"
		}
	} else {
		stance = "insufficient_data"
	}

	upsert("mp_stance", ptr(stanceScore), map[string]any{
		"stance":        stance,
		"score":         fmt.Sprintf("%.2f", stanceScore),
		"signals_used":  usedSignals,
		"thresholds":    fmt.Sprintf("accommodative >%.1f | neutral %.1f–%.1f | restrictive <%.1f", cfg.MPAccommodativeScore, cfg.MPRestrictiveScore, cfg.MPAccommodativeScore, cfg.MPRestrictiveScore),
		"weights":       "rate×2 + yield_curve×2 + real_rate×1.5 + balance_sheet×1 + credit×2 + breakeven×1 + m2×0.5",
		"note":          "Composite of free FRED data only. Adding CME FedWatch + FOMC LLM scoring would substantially improve signal.",
		"todo_growth":   "Growth panel (GDP/PMI/jobless claims) would enable full macro regime detection",
		"todo_global":   "ECB/BoE/BoJ rates + global PMI required for cross-border regime overlay",
	})

	w.log.Info("monetary policy analysis complete",
		"stance", stance,
		"score", fmt.Sprintf("%.2f", stanceScore),
		"signals", len(usedSignals),
	)
}

// ── Growth Cycle Analysis ─────────────────────────────────────────────────────

// analyzeGrowth derives growth-cycle signals from FRED series stored in
// macro_fred and writes the results to macro_derived.
//
// All signals are market-wide (not per-symbol).  The composite gc_stance score
// is the primary output consumed by the analyst-bot daily-report embed.
//
// Tier 1 (Leading indicators — move before the economy):
//   PMI (NAPM), LEI (USSLIND), jobless claims (ICSA/CCSA), housing (HOUST/PERMIT)
//
// Tier 2 (Coincident indicators — move with the economy):
//   Real GDP (GDPC1), payrolls (PAYEMS), unemployment (UNRATE),
//   avg hourly earnings (CES0500000003), Sahm Rule (SAHMREALTIME),
//   retail sales (RSAFS/RRSFS)
//
// Tier 3 (Lagging / Sentiment):
//   Michigan Sentiment (UMCSENT), Durable Goods (DGORDER), Core Capex (NEWORDER)
func (w *worker) analyzeGrowth(ctx context.Context) {
	ts := time.Now().UTC()
	gc := w.growthCfg

	ptr := func(v float64) *float64 { return &v }

	upsert := func(metric string, value *float64, payload any) {
		if err := store.UpsertMacroDerived(ctx, w.pool, ts, metric, value, payload); err != nil {
			w.log.Error("upsert growth derived", "metric", metric, "err", err)
		}
	}

	fredLatest := func(seriesID string) (float64, bool) {
		v, _, ok := store.QueryMacroFredLatest(ctx, w.pool, seriesID)
		return v, ok
	}

	// fredSeries returns the last n observations (newest first).
	fredSeries := func(seriesID string, n int) []store.MacroObs {
		rows, err := store.QueryMacroFredSeries(ctx, w.pool, seriesID, n)
		if err != nil {
			return nil
		}
		return rows
	}

	// scaledScore clamps a value to [-1, +1] and returns a *float64.
	clamp := func(v float64) float64 {
		if v > 1 {
			return 1
		}
		if v < -1 {
			return -1
		}
		return v
	}

	// ── Scoring accumulator ───────────────────────────────────────────────────
	// Each signal contributes a score in [-1, +1] with an associated weight.
	// Tier 1 (leading) weight 0.35, Tier 2 (coincident) weight 0.45,
	// Tier 3 (lagging/sentiment) weight 0.20 — distributed across sub-signals.
	type scored struct {
		score  float64
		weight float64
	}
	var scores []scored
	usedSignals := 0

	addScore := func(s, w float64) {
		scores = append(scores, scored{clamp(s), w})
		usedSignals++
	}

	// ── Tier 1: PMI — ISM Manufacturing (NAPM) ──────────────────────────────
	// Monthly.  Values: >55 strong_expansion, >50 expansion, >45 slowing,
	// >40 contraction, <40 severe_contraction.
	// TODO [PAID]: Replace/supplement with S&P Global Markit PMI for higher frequency.
	// TODO [PAID]: Add ISM Services PMI when an API feed is available.
	pmiObs := fredSeries("NAPM", 12)
	pmiRegime := "no_data"
	var pmiScore float64
	if len(pmiObs) > 0 {
		pmi := pmiObs[0].Value
		switch {
		case pmi >= gc.PMIStrong:
			pmiRegime = "strong_expansion"
			pmiScore = 1.0
		case pmi >= gc.PMIExpansion:
			pmiRegime = "expansion"
			pmiScore = 0.4
		case pmi >= gc.PMISlow:
			pmiRegime = "slowing"
			pmiScore = -0.2
		case pmi >= gc.PMISevere:
			pmiRegime = "contraction"
			pmiScore = -0.6
		default:
			pmiRegime = "severe_contraction"
			pmiScore = -1.0
		}
		// 3-month trend: compare latest to 3 months ago.
		trend3m := "stable"
		if len(pmiObs) >= 4 {
			delta := pmi - pmiObs[3].Value
			if delta >= 2 {
				trend3m = "improving"
			} else if delta <= -2 {
				trend3m = "deteriorating"
			}
		}
		addScore(pmiScore, 0.15) // Tier 1 sub-signal
		upsert("gc_pmi", ptr(pmi), map[string]any{
			"regime":  pmiRegime,
			"score":   pmiScore,
			"trend3m": trend3m,
			"series":  "NAPM",
		})
	} else {
		upsert("gc_pmi", nil, map[string]any{"regime": pmiRegime})
	}

	// ── Tier 1: LEI — Conference Board Leading Economic Index (USSLIND) ──────
	// Monthly level.  We compute 6-month change and annualise it.
	// 3 consecutive declines ("rule of three") = classic recession warning.
	// Note: FRED provides the composite index only; sub-components are paid.
	leiObs := fredSeries("USSLIND", 12)
	leiRegime := "no_data"
	var leiScore float64
	if len(leiObs) >= 2 {
		lei := leiObs[0].Value
		leiScore6m := 0.0
		leiTrend := "stable"
		if len(leiObs) >= 7 {
			// 6-month annualised rate of change.
			leiScore6m = ((lei/leiObs[6].Value) - 1) * 100
			// Rule of 3: count consecutive monthly declines.
			consecutiveDeclines := 0
			for i := 0; i < len(leiObs)-1 && i < 6; i++ {
				if leiObs[i].Value < leiObs[i+1].Value {
					consecutiveDeclines++
				} else {
					break
				}
			}
			if consecutiveDeclines >= 3 {
				leiTrend = "rule_of_three_decline"
			} else if leiScore6m > gc.LEIExpansionRate {
				leiTrend = "expanding"
			} else if leiScore6m < gc.LEIRecessionRate {
				leiTrend = "recession_risk"
			} else {
				leiTrend = "slowing"
			}
		}
		switch leiTrend {
		case "expanding":
			leiRegime = "expanding"
			leiScore = 0.8
		case "slowing":
			leiRegime = "slowing"
			leiScore = -0.2
		case "recession_risk", "rule_of_three_decline":
			leiRegime = leiTrend
			leiScore = -0.9
		default:
			leiRegime = "stable"
			leiScore = 0.0
		}
		addScore(leiScore, 0.12)
		upsert("gc_lei", ptr(lei), map[string]any{
			"regime":            leiRegime,
			"score":             leiScore,
			"six_month_rate_pct": math.Round(leiScore6m*100) / 100,
			"series":            "USSLIND",
		})
	} else {
		upsert("gc_lei", nil, map[string]any{"regime": leiRegime})
	}

	// ── Tier 1: Initial Jobless Claims (ICSA) + Continuing Claims (CCSA) ─────
	// Weekly.  Use 4-week moving average to smooth single-week noise.
	claimsRegime := "no_data"
	var claimsScore float64
	icsa := fredSeries("ICSA", 8)
	if len(icsa) >= 4 {
		// 4-week MA.
		ma4 := (icsa[0].Value + icsa[1].Value + icsa[2].Value + icsa[3].Value) / 4
		var contClaims *float64
		if cv, ok := fredLatest("CCSA"); ok {
			contClaims = ptr(cv)
		}
		switch {
		case ma4 < gc.ClaimsTight:
			claimsRegime = "tight_labor"
			claimsScore = 0.9
		case ma4 < gc.ClaimsNormalizing:
			claimsRegime = "normal"
			claimsScore = 0.3
		case ma4 < gc.ClaimsCrisis:
			claimsRegime = "normalizing"
			claimsScore = -0.3
		default:
			claimsRegime = "crisis"
			claimsScore = -1.0
		}
		payload := map[string]any{
			"regime":       claimsRegime,
			"score":        claimsScore,
			"icsa_4w_ma":   math.Round(ma4),
			"icsa_latest":  icsa[0].Value,
		}
		if contClaims != nil {
			payload["ccsa_latest"] = *contClaims
		}
		addScore(claimsScore, 0.08)
		upsert("gc_claims", ptr(ma4), payload)
	} else {
		upsert("gc_claims", nil, map[string]any{"regime": claimsRegime})
	}

	// ── Tier 1: Housing Starts (HOUST) + Building Permits (PERMIT) ───────────
	// Monthly, annualised thousands of units.
	housingRegime := "no_data"
	var housingScore float64
	if hv, ok := fredLatest("HOUST"); ok {
		var permitVal *float64
		if pv, ok2 := fredLatest("PERMIT"); ok2 {
			permitVal = ptr(pv)
		}
		switch {
		case hv >= gc.HousingStrong:
			housingRegime = "strong"
			housingScore = 0.8
		case hv >= gc.HousingWeak:
			housingRegime = "moderate"
			housingScore = 0.2
		default:
			housingRegime = "weak"
			housingScore = -0.8
		}
		payload := map[string]any{
			"regime":       housingRegime,
			"score":        housingScore,
			"houst_k_ann":  math.Round(hv),
		}
		if permitVal != nil {
			payload["permit_k_ann"] = math.Round(*permitVal)
		}
		addScore(housingScore, 0.08)
		upsert("gc_housing", ptr(hv), payload)
	} else {
		upsert("gc_housing", nil, map[string]any{"regime": housingRegime})
	}

	// ── Tier 2: Real GDP (GDPC1) ──────────────────────────────────────────────
	// Quarterly levels (billions of chained 2012 dollars).  Convert to
	// annualised quarter-on-quarter percentage change.
	// Signal is always 1–3 months stale — supplement with LEI + claims for timeliness.
	gdpRegime := "no_data"
	var gdpScore float64
	gdpObs := fredSeries("GDPC1", 8)
	if len(gdpObs) >= 2 {
		gdp := gdpObs[0].Value
		gdpPrior := gdpObs[1].Value
		// Annualised QoQ: ((current/prior)^4 - 1) × 100.
		gdpAnnPct := (math.Pow(gdp/gdpPrior, 4) - 1) * 100
		switch {
		case gdpAnnPct >= gc.GDPStrong:
			gdpRegime = "strong"
			gdpScore = 1.0
		case gdpAnnPct >= gc.GDPStall:
			gdpRegime = "moderate"
			gdpScore = 0.3
		case gdpAnnPct >= 0:
			gdpRegime = "stall_speed"
			gdpScore = -0.3
		default:
			gdpRegime = "recession"
			gdpScore = -1.0
		}
		addScore(gdpScore, 0.14)
		upsert("gc_gdp", ptr(gdpAnnPct), map[string]any{
			"regime":     gdpRegime,
			"score":      gdpScore,
			"ann_pct":    math.Round(gdpAnnPct*100) / 100,
			"gdpc1_bn":   math.Round(gdp),
			"series":     "GDPC1",
		})
	} else {
		upsert("gc_gdp", nil, map[string]any{"regime": gdpRegime})
	}

	// ── Tier 2: Employment — Payrolls + Unemployment + AHE + Sahm Rule ───────
	// PAYEMS: monthly net jobs added (thousands).
	// UNRATE: unemployment rate (%).
	// CES0500000003: avg hourly earnings all employees (%).
	// SAHMREALTIME: Sahm Rule indicator — >= 0.5 = recession confirmed.
	// TODO [PAID]: ADP National Employment Report — supplement monthly payrolls.
	emplRegime := "no_data"
	var emplScore float64
	if nfp, ok := fredLatest("PAYEMS"); ok {
		var unrate, ahe, sahm *float64
		if v, ok2 := fredLatest("UNRATE"); ok2 {
			unrate = ptr(v)
		}
		if v, ok2 := fredLatest("CES0500000003"); ok2 {
			ahe = ptr(v)
		}
		if v, ok2 := fredLatest("SAHMREALTIME"); ok2 {
			sahm = ptr(v)
		}

		// Sahm rule overrides everything when triggered.
		if sahm != nil && *sahm >= gc.SahmThreshold {
			emplRegime = "recession_confirmed"
			emplScore = -1.0
		} else {
			switch {
			case nfp >= gc.NFPStrong:
				emplRegime = "strong"
				emplScore = 1.0
			case nfp >= gc.NFPModerate:
				emplRegime = "moderate"
				emplScore = 0.3
			case nfp >= 0:
				emplRegime = "slowing"
				emplScore = -0.3
			default:
				emplRegime = "contraction"
				emplScore = -1.0
			}
		}
		payload := map[string]any{
			"regime":      emplRegime,
			"score":       emplScore,
			"payems_k":    nfp,
		}
		if unrate != nil {
			payload["unrate_pct"] = *unrate
		}
		if ahe != nil {
			payload["ahe_pct"] = *ahe
		}
		if sahm != nil {
			payload["sahm_pp"] = math.Round(*sahm*100) / 100
		}
		addScore(emplScore, 0.14)
		upsert("gc_employment", ptr(nfp), payload)
	} else {
		upsert("gc_employment", nil, map[string]any{"regime": emplRegime})
	}

	// ── Tier 2: Consumer — Real Retail Sales (RRSFS) YoY % ───────────────────
	// RRSFS is inflation-adjusted — it cleanly reflects volume growth.
	// RSAFS (nominal) included in payload for context.
	consumerRegime := "no_data"
	var consumerScore float64
	rrsfSeries := fredSeries("RRSFS", 14)
	if len(rrsfSeries) >= 13 {
		current := rrsfSeries[0].Value
		priorYear := rrsfSeries[12].Value
		yoyPct := (current/priorYear - 1) * 100
		switch {
		case yoyPct >= gc.RetailHealthy:
			consumerRegime = "healthy"
			consumerScore = 0.8
		case yoyPct >= 0:
			consumerRegime = "slowing"
			consumerScore = 0.0
		default:
			consumerRegime = "contraction"
			consumerScore = -0.8
		}
		payload := map[string]any{
			"regime":          consumerRegime,
			"score":           consumerScore,
			"rrsfs_yoy_pct":   math.Round(yoyPct*100) / 100,
			"rrsfs_current_mn": math.Round(current),
		}
		if nv, ok := fredLatest("RSAFS"); ok {
			payload["rsafs_nominal_mn"] = math.Round(nv)
		}
		addScore(consumerScore, 0.10)
		upsert("gc_consumer", ptr(yoyPct), payload)
	} else {
		upsert("gc_consumer", nil, map[string]any{"regime": consumerRegime})
	}

	// ── Tier 3: Michigan Consumer Sentiment (UMCSENT) ─────────────────────────
	// Contrarian signal: <60 = near historical market bottoms.
	// >100 + VIX<12 = potential complacency zone.
	umichRegime := "no_data"
	var umichScore float64
	if umics, ok := fredLatest("UMCSENT"); ok {
		switch {
		case umics < gc.UMichBottom:
			umichRegime = "near_bottom"
			umichScore = 0.3 // contrarian bullish — panic buying opportunity
		case umics > gc.UMichComplacency:
			umichRegime = "complacency"
			umichScore = -0.3 // contrarian bearish — peak optimism
		case umics < 80:
			umichRegime = "pessimistic"
			umichScore = 0.1
		default:
			umichRegime = "normal"
			umichScore = 0.2
		}
		addScore(umichScore, 0.05)
		upsert("gc_consumer_sentiment", ptr(umics), map[string]any{
			"regime": umichRegime,
			"score":  umichScore,
			"series": "UMCSENT",
		})
	} else {
		upsert("gc_consumer_sentiment", nil, map[string]any{"regime": umichRegime})
	}

	// ── Tier 3: Core Capex — New Orders, Capital Goods Nondefense Ex-Aircraft ─
	// (NEWORDER — monthly, millions of dollars, seasonally adjusted)
	// 3-month rolling change vs 3 months prior as a trend signal.
	// DGORDER (Durable Goods total) also stored for context.
	capexRegime := "no_data"
	var capexScore float64
	newOrderObs := fredSeries("NEWORDER", 7)
	if len(newOrderObs) >= 4 {
		latest3avg := (newOrderObs[0].Value + newOrderObs[1].Value + newOrderObs[2].Value) / 3
		prior3avg := (newOrderObs[3].Value + newOrderObs[4].Value + newOrderObs[5].Value) / 3
		trend3m := (latest3avg/prior3avg - 1) * 100
		switch {
		case trend3m >= gc.CapexExpansion:
			capexRegime = "expanding"
			capexScore = 0.8
		case trend3m >= 0:
			capexRegime = "stable"
			capexScore = 0.2
		case trend3m >= gc.CapexWarning:
			capexRegime = "slowing"
			capexScore = -0.2
		default:
			capexRegime = "warning"
			capexScore = -0.8
		}
		payload := map[string]any{
			"regime":          capexRegime,
			"score":           capexScore,
			"neworder_3m_pct": math.Round(trend3m*100) / 100,
			"neworder_latest": math.Round(newOrderObs[0].Value),
			"series":          "NEWORDER",
		}
		if dgv, ok := fredLatest("DGORDER"); ok {
			payload["dgorder_latest"] = math.Round(dgv)
		}
		addScore(capexScore, 0.05)
		upsert("gc_capex", ptr(trend3m), payload)
	} else {
		upsert("gc_capex", nil, map[string]any{"regime": capexRegime})
	}

	// ── Composite Growth Stance ───────────────────────────────────────────────
	// Weighted mean of all scored signals.  Weights sum to 0.91 when all
	// signals are available (remaining 0.09 reserved for future Tier 1 paid PMI).
	stanceScore := 0.0
	weightSum := 0.0
	for _, s := range scores {
		stanceScore += s.score * s.weight
		weightSum += s.weight
	}
	var gcStance string
	var gcStanceScore *float64
	if usedSignals == 0 {
		gcStance = "insufficient_data"
		upsert("gc_stance", nil, map[string]any{
			"stance":  gcStance,
			"signals": 0,
			"todo_pmi_paid": "S&P Global / ISM Services PMI would improve leading-indicator coverage",
		})
	} else {
		if weightSum > 0 {
			stanceScore /= weightSum
		}
		switch {
		case stanceScore >= gc.GrowthExpansionScore:
			gcStance = "expansion"
		case stanceScore <= gc.GrowthContractionScore:
			gcStance = "contraction"
		default:
			gcStance = "slowdown"
		}
		gcStanceScore = ptr(stanceScore)
		upsert("gc_stance", gcStanceScore, map[string]any{
			"stance":         gcStance,
			"score":          math.Round(stanceScore*1000) / 1000,
			"signals_used":   usedSignals,
			"pmi_regime":     pmiRegime,
			"lei_regime":     leiRegime,
			"claims_regime":  claimsRegime,
			"housing_regime": housingRegime,
			"gdp_regime":     gdpRegime,
			"empl_regime":    emplRegime,
			"consumer_regime": consumerRegime,
			"capex_regime":   capexRegime,
		})
	}

	w.log.Info("growth cycle analysis complete",
		"stance", gcStance,
		"score", fmt.Sprintf("%.3f", stanceScore),
		"signals", usedSignals,
	)
}
