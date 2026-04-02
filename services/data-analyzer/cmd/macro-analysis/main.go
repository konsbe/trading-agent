// macro-analysis reads raw FRED observations stored by data-ingestion/data-equity
// and derives monetary-policy signals that the analyst-bot can consume directly.
//
// Data flow:
//   data-equity (data-ingestion)
//     → macro_fred (TimescaleDB, raw FRED series observations)
//     → macro-analysis (this binary, data-analyzer)
//     → macro_derived (TimescaleDB, source = "macro_analysis")
//
// Monetary Policy signals derived (Tier 1 — free data):
//   mp_rate           Fed Funds Rate level + YoY direction (hiking/cutting/neutral)
//   mp_yield_curve    2s10s + 3m10y spreads + regime (steep/normal/flat/inverted/re_steepening)
//   mp_real_rate      10Y TIPS yield + regime (deeply_negative/balanced/headwind)
//   mp_balance_sheet  Fed total assets (QE/QT/neutral) — 4-week change
//   mp_credit_spread  HY + IG OAS spreads + regime (benign/elevated/crisis)
//
// Bond Market signals derived (Tier 2 — free data):
//   mp_breakeven_inflation  10Y + 5Y breakeven rates + regime (anchored/rising/unanchored)
//   mp_treasury_yields      2Y / 10Y / 30Y term structure snapshot
//   mp_m2_supply            M2 YoY growth rate + regime (normal/inflationary/deflationary)
//
// Composite:
//   mp_stance   Weighted aggregate of all signals (-1 = restrictive … +1 = accommodative)
//
// TODO [LLM]:  Score FOMC statements and minutes hawkish/dovish on -5 to +5 scale.
// TODO [PAID]: CME FedWatch implied rate probabilities (requires CME API subscription).
// TODO [PAID]: 5Y5Y forward inflation swap — Bloomberg or ICE Data.
// TODO [FUTURE]: Add ECB (ECBDFR via FRED), BoE (UKBANKRATE via FRED), BoJ policy rate.
// TODO [FUTURE]: Growth panel — GDP (GDPC1), ISM PMI (MANEMP proxy), jobless claims (IC4WSA).
// TODO [FUTURE]: Inflation panel — CPI (CPIAUCSL), Core PCE (PCEPILFE), PPI (WPSFD4131).
// TODO [FUTURE]: Global panel — China PMI, EM stress signals, DXY (no direct FRED series).
// TODO [FUTURE]: Equity Risk Premium — link to fundamental-analysis composite P/E for ERP calc.
// TODO [PYTHON]: Migrate to Python + asyncpg once LLM scoring layer is added; SQL is identical.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"os"
	"os/signal"
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

	w := &worker{cfg: cfg, pool: pool, log: log}

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

type worker struct {
	cfg  config.MacroAnalysis
	pool *pgxpool.Pool
	log  *slog.Logger
}

func (w *worker) analyzeAll(ctx context.Context) {
	w.analyzeMonetary(ctx)
	// TODO [FUTURE]: w.analyzeGrowth(ctx)     — GDP, ISM PMI, jobless claims
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
