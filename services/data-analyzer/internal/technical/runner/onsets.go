package runner

import (
	"context"
	"time"

	"github.com/konsbe/trading-agent/services/data-analyzer/internal/compute"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/heuristics"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/store"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/technical"
)

// alertOnsetsIndicator is read by the analyst bot's alert scan: it posts an
// alert only for an onset flagged here (heuristics.OnsetsAt), keyed by
// bar_date, so an unchanged daily reading is never re-posted.
const alertOnsetsIndicator = "alert_onsets"

func (w *run) computeAlertOnsets(ctx context.Context, ts time.Time, symbol, exchange, interval string, bars []compute.Bar) {
	cfg := heuristics.Config{Lookback: w.cfg.ComputeLookback, Params: technical.ParamsFrom(w.cfg)}
	o := heuristics.OnsetsAt(bars, len(bars)-1, cfg)
	payload := map[string]any{
		"bar_date":                 o.BarDate,
		"gap_sessions":             o.GapSessions,
		"judged":                   o.Judged,
		"rsi_overbought_onset":     o.RSIOverbought,
		"rsi_oversold_onset":       o.RSIOversold,
		"rsi_overbought_threshold": heuristics.RSIOverboughtLine,
		"rsi_oversold_threshold":   heuristics.RSIOversoldLine,
		"bb_squeeze_onset":         o.BBSqueezeOnset,
		"sweep_onset":              o.SweepOnset,
	}
	if o.RSIOK {
		payload["rsi_14"] = o.RSI
	}
	if o.Sweep != nil {
		payload["sweep"] = map[string]any{"kind": o.Sweep.Kind, "swept_level": o.Sweep.SweptLevel, "bar_close": o.Sweep.BarClose}
	}
	n := float64(o.Count())
	if err := store.UpsertIndicator(ctx, w.pool, ts, symbol, exchange, interval, alertOnsetsIndicator, &n, payload); err != nil {
		w.log.Error("upsert alert_onsets", "symbol", symbol, "err", err)
	}
}
