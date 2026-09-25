package runner

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/konsbe/trading-agent/services/data-analyzer/internal/compute"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/store"
)

func (w *run) computeRSBenchmark(ctx context.Context, ts time.Time, symbol, exchange, interval string, assetBars []compute.Bar) {
	if !w.cfg.EnableRSBenchmark {
		return
	}
	bench := ""
	switch exchange {
	case "equity":
		bench = w.cfg.RSBenchmarkEquity
	case "binance":
		bench = w.cfg.RSBenchmarkCrypto
	default:
		return
	}
	if bench == "" || bench == symbol {
		return
	}
	var (
		other []compute.Bar
		err   error
	)
	switch exchange {
	case "equity":
		other, err = equityBars(ctx, w.pool, bench, interval, w.cfg.ComputeLookback)
	case "binance":
		other, err = cryptoBars(ctx, w.pool, bench, interval, w.cfg.ComputeLookback)
	}
	if err != nil {
		w.log.Error("rs benchmark query", "benchmark", bench, "err", err)
		return
	}
	aC, bC, _ := compute.AlignClosesByTimestamp(assetBars, other)
	if len(aC) < w.cfg.RSBenchmarkMinAligned {
		w.log.Debug("rs benchmark insufficient overlap", "symbol", symbol, "benchmark", bench, "aligned", len(aC))
		return
	}
	rs, ok := compute.RelativeStrengthLast(aC, bC)
	if !ok {
		return
	}
	name := fmt.Sprintf("rs_vs_%s", strings.ToLower(strings.ReplaceAll(bench, "/", "_")))
	upsert := func(indicator string, value *float64, payload any) {
		if err := store.UpsertIndicator(ctx, w.pool, ts, symbol, exchange, interval, indicator, value, payload); err != nil {
			w.log.Error("upsert rs benchmark", "indicator", indicator, "err", err)
		}
	}
	ptr := func(v float64) *float64 { return &v }
	upsert(name, ptr(rs.Ratio), map[string]any{
		"benchmark":          bench,
		"ratio":              rs.Ratio,
		"ratio_change_pct_1": rs.RatioChange1,
		"asset_roc_1":        rs.AssetROC1,
		"benchmark_roc_1":    rs.BenchmarkROC1,
		"outperformance_1":   rs.Outperformance1,
		"aligned_bars":       rs.AlignedBars,
	})
}

func (w *run) computeMTFConfluence(ctx context.Context, ts time.Time, symbol, exchange, interval string, primaryBars []compute.Bar) {
	if !w.cfg.EnableMTFConfluence {
		return
	}
	var secondaries []string
	switch exchange {
	case "equity":
		secondaries = w.cfg.MTFEquitySecondary
	case "binance":
		secondaries = w.cfg.MTFCryptoSecondary
	}
	if len(secondaries) == 0 {
		return
	}
	pc := compute.Closes(primaryBars)
	ph := compute.Highs(primaryBars)
	pl := compute.Lows(primaryBars)
	pLB := w.cfg.TrendLookback
	if pLB > len(pc) {
		pLB = len(pc)
	}
	if pLB < 5 {
		return
	}
	pTrend, ok := compute.AnalyzeTrend(pc, ph, pl, pLB)
	if !ok {
		return
	}
	layers := make([]map[string]any, 0, len(secondaries))
	match, total := 0, 0
	for _, iv2 := range secondaries {
		if iv2 == interval {
			continue
		}
		var (
			sec []compute.Bar
			err error
		)
		switch exchange {
		case "equity":
			sec, err = equityBars(ctx, w.pool, symbol, iv2, w.cfg.ComputeLookback)
		case "binance":
			sec, err = cryptoBars(ctx, w.pool, symbol, iv2, w.cfg.ComputeLookback)
		}
		if err != nil || len(sec) < 5 {
			continue
		}
		sc := compute.Closes(sec)
		sh := compute.Highs(sec)
		sl := compute.Lows(sec)
		sLB := w.cfg.TrendLookback
		if sLB > len(sc) {
			sLB = len(sc)
		}
		if sLB < 5 {
			continue
		}
		sTrend, ok2 := compute.AnalyzeTrend(sc, sh, sl, sLB)
		if !ok2 {
			continue
		}
		total++
		aligned := sTrend.Direction == pTrend.Direction && pTrend.Direction != "sideways"
		if aligned {
			match++
		}
		layers = append(layers, map[string]any{
			"interval":        iv2,
			"trend":           sTrend.Direction,
			"aligned_primary": aligned,
		})
	}
	if total == 0 {
		return
	}
	score := float64(match) / float64(total)
	upsert := func(indicator string, value *float64, payload any) {
		if err := store.UpsertIndicator(ctx, w.pool, ts, symbol, exchange, interval, indicator, value, payload); err != nil {
			w.log.Error("upsert mtf", "indicator", indicator, "err", err)
		}
	}
	ptr := func(v float64) *float64 { return &v }
	upsert("mtf_confluence", ptr(score), map[string]any{
		"primary_interval": interval,
		"primary_trend":    pTrend.Direction,
		"layers":           layers,
		"match_count":      match,
		"layer_count":      total,
		"confluence_score": score,
		"trend_lookback":   w.cfg.TrendLookback,
	})
}

// computeVIXRegime reads the latest VIXCLS value from the macro_fred table and
// classifies the current volatility regime. Stored once per symbol/interval so
// every downstream consumer can join on (symbol, interval, indicator = "vix_regime").
func (w *run) computeVIXRegime(ctx context.Context, ts time.Time, symbol, exchange, interval string) {
	if !w.cfg.EnableVIXRegime {
		return
	}
	vix, ok, err := store.QueryLatestFREDValue(ctx, w.pool, "VIXCLS")
	if err != nil {
		w.log.Warn("vix regime: query error", "err", err)
		return
	}
	if !ok {
		w.log.Debug("vix regime: no VIXCLS data yet — ensure data-macro is running with FRED_SERIES_IDS=VIXCLS")
		return
	}

	regime := compute.ClassifyVIX(vix, compute.VIXThresholds{
		Fear:        w.cfg.VIXFearThreshold,
		Elevated:    w.cfg.VIXElevatedThreshold,
		Complacency: w.cfg.VIXComplacencyThreshold,
	})

	upsert := func(indicator string, value *float64, payload any) {
		if err := store.UpsertIndicator(ctx, w.pool, ts, symbol, exchange, interval, indicator, value, payload); err != nil {
			w.log.Error("upsert vix_regime", "indicator", indicator, "err", err)
		}
	}
	ptr := func(v float64) *float64 { return &v }
	upsert("vix_regime", ptr(vix), map[string]any{
		"vix":                   vix,
		"regime":                regime,
		"fear_threshold":        w.cfg.VIXFearThreshold,
		"elevated_threshold":    w.cfg.VIXElevatedThreshold,
		"complacency_threshold": w.cfg.VIXComplacencyThreshold,
		"series_id":             "VIXCLS",
	})
}

// computeMultiTFPivots queries weekly (and optionally monthly) bars and stores
// Classic, Camarilla, and Woodie's pivot levels derived from the prior period bar.
// These are separate from the per-bar pivot (pivots_prior_bar) which uses the
// prior daily bar.
func (w *run) computeMultiTFPivots(ctx context.Context, ts time.Time, symbol, exchange, interval string) {
	if !w.cfg.EnableWeeklyPivots && !w.cfg.EnableMonthlyPivots {
		return
	}

	upsert := func(indicator string, value *float64, payload any) {
		if err := store.UpsertIndicator(ctx, w.pool, ts, symbol, exchange, interval, indicator, value, payload); err != nil {
			w.log.Error("upsert multi_tf_pivots", "indicator", indicator, "err", err)
		}
	}
	ptr := func(v float64) *float64 { return &v }

	pivotUpsert := func(name, refInterval string, bars []compute.Bar) {
		if len(bars) < 2 {
			return
		}
		// Use the second-to-last bar (prior completed period).
		prev := bars[len(bars)-2]
		pv := compute.PivotsFromPriorBar(prev)
		upsert(name, ptr(pv.PP), map[string]any{
			"reference_ts": prev.TS,
			"interval":     refInterval,
			"classic": map[string]float64{
				"PP": pv.PP, "R1": pv.R1, "R2": pv.R2, "R3": pv.R3,
				"S1": pv.S1, "S2": pv.S2, "S3": pv.S3,
			},
			"camarilla": pv.Camarilla,
			"woodie":    pv.Woodie,
		})
	}

	if w.cfg.EnableWeeklyPivots {
		weekIv := ""
		switch exchange {
		case "equity":
			weekIv = w.cfg.WeeklyPivotEquityInterval
		case "binance":
			weekIv = w.cfg.WeeklyPivotCryptoInterval
		}
		if weekIv != "" {
			var (
				wBars []compute.Bar
				err   error
			)
			// Lookback: TECHNICAL_WEEKLY_PIVOT_LOOKBACK (default 10).
			switch exchange {
			case "equity":
				wBars, err = equityBars(ctx, w.pool, symbol, weekIv, w.cfg.WeeklyPivotLookback)
			case "binance":
				wBars, err = cryptoBars(ctx, w.pool, symbol, weekIv, w.cfg.WeeklyPivotLookback)
			}
			if err != nil {
				w.log.Warn("weekly pivot query", "symbol", symbol, "interval", weekIv, "err", err)
			} else {
				pivotUpsert("pivots_weekly", weekIv, wBars)
			}
		}
	}

	if w.cfg.EnableMonthlyPivots {
		monIv := ""
		switch exchange {
		case "equity":
			monIv = w.cfg.MonthlyPivotEquityInterval
		case "binance":
			monIv = w.cfg.MonthlyPivotCryptoInterval
		}
		if monIv != "" {
			var (
				mBars []compute.Bar
				err   error
			)
			// Lookback: TECHNICAL_MONTHLY_PIVOT_LOOKBACK (default 5).
			switch exchange {
			case "equity":
				mBars, err = equityBars(ctx, w.pool, symbol, monIv, w.cfg.MonthlyPivotLookback)
			case "binance":
				mBars, err = cryptoBars(ctx, w.pool, symbol, monIv, w.cfg.MonthlyPivotLookback)
			}
			if err != nil {
				w.log.Warn("monthly pivot query", "symbol", symbol, "interval", monIv, "err", err)
			} else {
				pivotUpsert("pivots_monthly", monIv, mBars)
			}
		}
	}
}
