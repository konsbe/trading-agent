package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/konsbe/trading-agent/services/data-analyzer/internal/marketcycle"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/store"
)

// Per-instrument price phase for the Daily Market Report
// (docs/MOMENTUM_SCANNER_API_DAILY_MARKET_REPORT.md §1.1).
//
// Each instrument gets its own macro_derived row, metric "mc_price_phase:<SYMBOL>",
// written BESIDE the existing mc_market_cycle row (SPY + macro composite) that
// the Discord report reads — that row and its code path are unchanged. Only the
// price-derived facts are per-instrument; the composite blends market-wide
// macro stances and stays one market-wide reading.

const instrumentMetricPrefix = "mc_price_phase:"

type instrument struct {
	symbol string
	kind   string // "equity" (equity_ohlcv) or "crypto" (crypto_ohlcv)
}

// instrumentList is the fixed list, then crypto, then any watchlist symbol not
// already present, in that order and without duplicates.
func instrumentList(fixed, crypto, watchlist []string) []instrument {
	seen := map[string]bool{}
	var out []instrument
	add := func(sym, kind string) {
		sym = strings.ToUpper(strings.TrimSpace(sym))
		if sym == "" || seen[sym] {
			return
		}
		seen[sym] = true
		out = append(out, instrument{sym, kind})
	}
	for _, s := range fixed {
		add(s, "equity")
	}
	for _, s := range crypto {
		add(s, "crypto")
	}
	for _, s := range watchlist {
		add(s, "equity")
	}
	return out
}

func (w *worker) analyzeInstrumentCycles(ctx context.Context, ts time.Time, equityTh marketcycle.Thresholds) {
	cfg := w.cycleCfg
	var watch []string
	if cfg.IncludeWatchlist {
		var err error
		if watch, err = store.WatchlistSymbols(ctx, w.pool); err != nil {
			w.log.Warn("instrument cycles: watchlist", "err", err)
		}
	}
	cryptoTh := equityTh
	cryptoTh.PeakLookback = cfg.CryptoPeakLookback
	cryptoTh.CrashHighWindow = cfg.CryptoCrashHighWindow
	cryptoTh.CrashCloseBars = cfg.CryptoCrashCloseBars

	written := 0
	for _, in := range instrumentList(cfg.Instruments, cfg.CryptoInstruments, watch) {
		var (
			bars     []store.EquityOHLCVBar
			err      error
			th       = equityTh
			interval = cfg.Interval
			asOf     = "session close"
		)
		limit := cfg.FetchLimit
		if in.kind == "crypto" {
			th, interval = cryptoTh, cfg.CryptoInterval
			asOf = "00:00 UTC daily close (closed candles only)"
			if need := max(cryptoTh.PeakLookback, cryptoTh.SMAPeriod) + 20; limit < need {
				limit = need
			}
			bars, err = store.QueryCryptoClosedDailyAsc(ctx, w.pool, in.symbol, interval, limit, ts)
		} else {
			bars, err = store.QueryEquityOHLCVAsc(ctx, w.pool, in.symbol, interval, limit)
		}
		if err != nil {
			w.log.Error("instrument cycle bars", "symbol", in.symbol, "err", err)
			continue
		}
		payload := instrumentPayload(in, interval, asOf, bars, th, cfg.MinBars)
		var value *float64
		if v, ok := payload["drawdown_pct"].(float64); ok {
			value = &v
		}
		if err := store.UpsertMacroDerived(ctx, w.pool, ts, instrumentMetricPrefix+in.symbol, value, payload); err != nil {
			w.log.Error("upsert instrument cycle", "symbol", in.symbol, "err", err)
			continue
		}
		written++
	}
	w.log.Info("instrument cycles complete", "written", written, "watchlist_symbols", len(watch))
}

// instrumentPayload is pure, so the whole per-instrument output is testable.
func instrumentPayload(in instrument, interval, asOf string, bars []store.EquityOHLCVBar,
	th marketcycle.Thresholds, minBars int) map[string]any {
	p := map[string]any{
		"symbol":      in.symbol,
		"type":        in.kind,
		"interval":    interval,
		"as_of_basis": asOf,
		"bars_used":   len(bars),
		"windows": map[string]any{
			"sma_period":        th.SMAPeriod,
			"peak_lookback":     th.PeakLookback,
			"crash_high_window": orDefault(th.CrashHighWindow, 10),
			"crash_close_bars":  orDefault(th.CrashCloseBars, 5),
		},
	}
	need := th.SMAPeriod
	if need < minBars {
		need = minBars
	}
	if len(bars) < need {
		p["price_phase"] = "insufficient_data"
		p["unavailable_reason"] = fmt.Sprintf("%d %s bars stored for %s; need at least %d", len(bars), interval, in.symbol, need)
		if len(bars) > 0 {
			p["as_of"] = bars[len(bars)-1].TS.UTC().Format(time.DateOnly)
			p["close"] = marketcycle.Round2(bars[len(bars)-1].Close)
		}
		return p
	}
	pr := marketcycle.AnalyzePrice(in.symbol, bars, th)
	p["as_of"] = pr.LastTS.UTC().Format(time.DateOnly)
	p["close"] = marketcycle.Round2(pr.Close)
	p["peak_high"] = marketcycle.Round2(pr.PeakHigh)
	p["peak_ts"] = pr.PeakTS.UTC().Format(time.RFC3339)
	p["days_off_peak"] = pr.DaysOffPeak
	p["drawdown_pct"] = marketcycle.Round2(pr.DrawdownPct * 100)
	p["sma200"] = marketcycle.Round2(pr.SMA200)
	p["pct_vs_sma200"] = marketcycle.Round2(pr.PctVsSMA200)
	p["price_phase"] = pr.Phase
	p["crash_warning"] = pr.CrashWarning
	return p
}

func orDefault(v, def int) int {
	if v > 0 {
		return v
	}
	return def
}
