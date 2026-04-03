package additional

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/konsbe/trading-agent/services/data-analyzer/internal/store"
)

// Config drives the additional-analysis snapshot (additional_analysis_reference.html).
type Config struct {
	Enabled    bool
	CorrWindow int
	MinCorObs  int
	MaxBars    int
	Symbol     string
	Interval   string
}

func packRoll(r RollCorrResult, benchmark string, extra map[string]any) map[string]any {
	m := map[string]any{
		"symbol":              benchmark,
		"series_id":           r.SeriesID,
		"window_trading_days": 0, // filled by caller
		"correlation_60d":     r.Correlation60d,
		"observations_used":   r.ObservationsUsed,
		"regime":              r.Regime,
		"label":               r.Label,
		"insufficient_data":   r.InsufficientData,
	}
	for k, v := range extra {
		m[k] = v
	}
	return m
}

// BuildSnapshot computes intermarket rolling correlations, calendars, and reference-module coverage.
func BuildSnapshot(ctx context.Context, pool *pgxpool.Pool, cfg Config) (map[string]any, *float64) {
	if !cfg.Enabled {
		return nil, nil
	}
	if cfg.CorrWindow <= 0 {
		cfg.CorrWindow = 60
	}
	if cfg.MinCorObs <= 0 {
		cfg.MinCorObs = 40
	}
	if cfg.MaxBars <= 0 {
		cfg.MaxBars = 180
	}
	if cfg.Symbol == "" {
		cfg.Symbol = "SPY"
	}
	if cfg.Interval == "" {
		cfg.Interval = "1Day"
	}

	now := time.Now().UTC()
	out := map[string]any{
		"reference":         "services/data-analyzer/cmd/additional_analysis_reference.html",
		"as_of":             now.Format(time.RFC3339),
		"reference_modules": ReferenceModules(),
	}

	sm := StaticAlmanacSeasonality(int(now.Month()))
	out["seasonality"] = map[string]any{
		"month":      sm.Month,
		"month_name": sm.Name,
		"bias":       sm.Bias,
		"score":      sm.Score,
		"note":       sm.Note,
		"avg_hist":   sm.AvgHist,
		"disclaimer": "Static almanac from reference doc — tie-breaker only; do not trade in isolation.",
	}

	pc := ComputePresidentialCycle(now)
	out["presidential_cycle"] = map[string]any{
		"cycle_year": pc.CycleYear,
		"label":      pc.Label,
		"bias":       pc.Bias,
		"note":       pc.Note,
	}

	bars, err := store.QueryEquityOHLCVAsc(ctx, pool, cfg.Symbol, cfg.Interval, cfg.MaxBars)
	if err != nil || len(bars) < cfg.CorrWindow+2 {
		out["intermarket"] = map[string]any{
			"note": "Not enough benchmark OHLCV for rolling windows — check equity_ohlcv and MARKET_CYCLE_*.",
			"bond_equity_60d": map[string]any{"insufficient_data": true, "note": "No bar window."},
			"oil_equity_60d":  map[string]any{"insufficient_data": true, "note": "No bar window."},
			"vix_equity_60d":  map[string]any{"insufficient_data": true, "note": "No bar window."},
		}
		return out, nil
	}

	from := dateUTC(bars[0].TS).Add(-45 * 24 * time.Hour)
	to := dateUTC(bars[len(bars)-1].TS).Add(24 * time.Hour)

	im := map[string]any{}

	// DGS10 — bond / equity
	if dgs10, e := store.QueryMacroFredRangeAsc(ctx, pool, "DGS10", from, to); e == nil && len(dgs10) >= 5 {
		be := ComputeRollCorrEquityVsFredDelta(bars, dgs10, cfg.CorrWindow, cfg.MinCorObs, "DGS10", regimeBondEquity,
			"Need more overlapping benchmark bars and DGS10 observations.")
		m := packRoll(be, cfg.Symbol, map[string]any{"yield_series": "DGS10"})
		m["window_trading_days"] = cfg.CorrWindow
		im["bond_equity_60d"] = m
	} else {
		im["bond_equity_60d"] = map[string]any{
			"insufficient_data": true,
			"note":                "DGS10 (macro_fred) missing or sparse — run data-equity FRED ingestion.",
		}
	}

	// WTI
	if oilFred, e := store.QueryMacroFredRangeAsc(ctx, pool, "DCOILWTICO", from, to); e == nil && len(oilFred) >= 5 {
		o := ComputeRollCorrEquityVsFredDelta(bars, oilFred, cfg.CorrWindow, cfg.MinCorObs, "DCOILWTICO", regimeOilEquity,
			"WTI (DCOILWTICO) missing or sparse in macro_fred.")
		m := packRoll(o, cfg.Symbol, map[string]any{"description": "Benchmark log return vs daily change in WTI ($/bbl)"})
		m["window_trading_days"] = cfg.CorrWindow
		im["oil_equity_60d"] = m
	} else {
		im["oil_equity_60d"] = map[string]any{
			"insufficient_data": true,
			"note":              "DCOILWTICO not available in macro_fred for this window.",
		}
	}

	// VIX level change vs equity return
	if vixFred, e := store.QueryMacroFredRangeAsc(ctx, pool, "VIXCLS", from, to); e == nil && len(vixFred) >= 5 {
		v := ComputeRollCorrEquityVsFredDelta(bars, vixFred, cfg.CorrWindow, cfg.MinCorObs, "VIXCLS", regimeVIXEquity,
			"VIXCLS missing or sparse in macro_fred.")
		m := packRoll(v, cfg.Symbol, map[string]any{"description": "Benchmark log return vs daily change in VIX index"})
		m["window_trading_days"] = cfg.CorrWindow
		im["vix_equity_60d"] = m
	} else {
		im["vix_equity_60d"] = map[string]any{
			"insufficient_data": true,
			"note":              "VIXCLS not available in macro_fred for this window.",
		}
	}

	out["intermarket"] = im

	var val *float64
	if beM, ok := im["bond_equity_60d"].(map[string]any); ok {
		if ins, _ := beM["insufficient_data"].(bool); !ins {
			if c, ok := beM["correlation_60d"].(float64); ok {
				val = &c
			}
		}
	}
	if val == nil {
		if oilM, ok := im["oil_equity_60d"].(map[string]any); ok {
			if ins, _ := oilM["insufficient_data"].(bool); !ins {
				if c, ok := oilM["correlation_60d"].(float64); ok {
					val = &c
				}
			}
		}
	}
	if val == nil {
		if vixM, ok := im["vix_equity_60d"].(map[string]any); ok {
			if ins, _ := vixM["insufficient_data"].(bool); !ins {
				if c, ok := vixM["correlation_60d"].(float64); ok {
					val = &c
				}
			}
		}
	}
	return out, val
}
