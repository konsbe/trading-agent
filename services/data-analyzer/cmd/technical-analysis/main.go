// technical-analysis reads OHLCV bars that data-ingestion workers have already
// stored in TimescaleDB, runs every enabled indicator from the compute package,
// and writes results to the technical_indicators table.
//
// Data flow:
//   data-crypto / data-equity (data-ingestion)
//     → equity_ohlcv / crypto_ohlcv (TimescaleDB)
//     → technical-analysis (this binary, data-analyzer)
//     → technical_indicators (TimescaleDB)
//
// TODO: migrate compute/ to Python (pandas-ta / ta-lib) and retire this binary.
// See internal/compute/doc.go for the migration checklist.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"github.com/konsbe/trading-agent/services/data-analyzer/internal/compute"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/config"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/db"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/logx"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/store"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	_ = godotenv.Load()
	cfg, err := config.LoadTechnicalAnalysis()
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

	// Allow data-ingestion workers time to complete their initial OHLCV backfill
	// before the first computation run. Controlled by ANALYZER_STARTUP_DELAY_SECS
	// (default 60). Set to 0 when running against a pre-populated database.
	if delaySecs := startupDelay(); delaySecs > 0 {
		log.Info("waiting for ingestion backfill", "delay_secs", delaySecs)
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Duration(delaySecs) * time.Second):
		}
	}

	log.Info("running initial indicator computation")
	w.computeAll(ctx)

	ticker := time.NewTicker(cfg.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Info("shutdown")
			return
		case <-ticker.C:
			w.computeAll(ctx)
		}
	}
}

type worker struct {
	cfg  config.TechnicalAnalysis
	pool *pgxpool.Pool
	log  *slog.Logger
}

// computeAll runs every enabled indicator for all configured symbols × intervals.
// It reads bars from the DB (written by data-ingestion workers) — it never calls
// any external API.
func (w *worker) computeAll(ctx context.Context) {
	for _, sym := range w.cfg.EquitySymbols {
		for _, iv := range w.cfg.EquityIntervals {
			bars, err := store.QueryEquityBars(ctx, w.pool, sym, iv, w.cfg.ComputeLookback)
			if err != nil {
				w.log.Error("query equity bars", "symbol", sym, "interval", iv, "err", err)
				continue
			}
			if len(bars) < 2 {
				w.log.Warn("not enough equity bars", "symbol", sym, "interval", iv, "have", len(bars))
				continue
			}
			w.computeAndStore(ctx, bars, sym, "equity", iv)
		}
	}
	for _, sym := range w.cfg.CryptoSymbols {
		for _, iv := range w.cfg.CryptoIntervals {
			bars, err := store.QueryCryptoBars(ctx, w.pool, sym, iv, w.cfg.ComputeLookback)
			if err != nil {
				w.log.Error("query crypto bars", "symbol", sym, "interval", iv, "err", err)
				continue
			}
			if len(bars) < 2 {
				w.log.Warn("not enough crypto bars", "symbol", sym, "interval", iv, "have", len(bars))
				continue
			}
			w.computeAndStore(ctx, bars, sym, "binance", iv)
		}
	}
}

// computeAndStore runs all enabled indicator groups and persists results.
// ts is anchored to the last bar's close timestamp so re-running identical data
// is idempotent (ON CONFLICT DO UPDATE in UpsertIndicator).
//
// TODO: each indicator block below maps 1-to-1 to a pandas-ta call in Python.
// When migrating, keep the indicator name strings identical (e.g. "rsi_14",
// "macd_12_26_9") so the technical_indicators schema and downstream consumers
// need no changes.
func (w *worker) computeAndStore(ctx context.Context, bars []compute.Bar, symbol, exchange, interval string) {
	ts := bars[len(bars)-1].TS

	closes := compute.Closes(bars)
	highs := compute.Highs(bars)
	lows := compute.Lows(bars)
	volumes := compute.Volumes(bars)
	currentPrice := closes[len(closes)-1]

	upsert := func(indicator string, value *float64, payload any) {
		if err := store.UpsertIndicator(ctx, w.pool, ts, symbol, exchange, interval, indicator, value, payload); err != nil {
			w.log.Error("upsert indicator", "indicator", indicator, "symbol", symbol, "err", err)
		}
	}
	ptr := func(v float64) *float64 { return &v }

	// ── Moving Averages ──────────────────────────────────────────────────────
	if w.cfg.EnableMA {
		for _, p := range w.cfg.SMAPeriods {
			if v, ok := compute.SMA(closes, p); ok {
				upsert(fmt.Sprintf("sma_%d", p), ptr(v), nil)
			}
		}
		for _, p := range w.cfg.EMAPeriods {
			if v, ok := compute.EMA(closes, p); ok {
				upsert(fmt.Sprintf("ema_%d", p), ptr(v), nil)
			}
		}
	}

	// ── RSI ──────────────────────────────────────────────────────────────────
	if w.cfg.EnableRSI {
		if v, ok := compute.RSI(closes, w.cfg.RSIPeriod); ok {
			upsert(fmt.Sprintf("rsi_%d", w.cfg.RSIPeriod), ptr(v), nil)
		}
	}

	// ── Volume ───────────────────────────────────────────────────────────────
	if w.cfg.EnableVolume {
		if v, ok := compute.VolumeSMA(volumes, w.cfg.VolSMAPeriod); ok {
			upsert(fmt.Sprintf("vol_sma_%d", w.cfg.VolSMAPeriod), ptr(v), nil)
		}
		if v, ok := compute.RelativeVolume(volumes, w.cfg.VolSMAPeriod); ok {
			upsert("rel_vol", ptr(v), nil)
		}
	}

	// ── Support & Resistance ─────────────────────────────────────────────────
	if w.cfg.EnableSR {
		sr := compute.FindSR(highs, lows, currentPrice,
			w.cfg.SRSwingStrength, w.cfg.SRLevels, w.cfg.SRClusterPct)

		suppPrices := make([]float64, len(sr.Support))
		suppTouches := make([]int, len(sr.Support))
		for i, s := range sr.Support {
			suppPrices[i] = s.Price
			suppTouches[i] = s.Touches
		}
		resPrices := make([]float64, len(sr.Resistance))
		resTouches := make([]int, len(sr.Resistance))
		for i, r := range sr.Resistance {
			resPrices[i] = r.Price
			resTouches[i] = r.Touches
		}
		upsert("sr_levels", ptr(currentPrice), map[string]any{
			"current_price":      currentPrice,
			"support":            suppPrices,
			"support_touches":    suppTouches,
			"resistance":         resPrices,
			"resistance_touches": resTouches,
		})
	}

	// ── Trend ────────────────────────────────────────────────────────────────
	if w.cfg.EnableTrend {
		if t, ok := compute.AnalyzeTrend(closes, highs, lows, w.cfg.TrendLookback); ok {
			upsert("trend", ptr(t.SlopePct), map[string]any{
				"direction":    t.Direction,
				"slope_pct":    t.SlopePct,
				"r2":           t.R2,
				"higher_highs": t.HigherHighs,
				"higher_lows":  t.HigherLows,
			})
		}
	}

	// ── Candlestick Patterns ─────────────────────────────────────────────────
	if w.cfg.EnableCandles {
		// Scan only the last CandleWindow bars (TECHNICAL_CANDLE_WINDOW, default 3).
		window := bars
		if cw := w.cfg.CandleWindow; cw > 0 && len(window) > cw {
			window = window[len(window)-cw:]
		}
		patterns := compute.DetectPatterns(window)
		names := make([]string, len(patterns))
		for i, p := range patterns {
			names[i] = p.Name
		}
		cur := bars[len(bars)-1]
		upsert("candle_patterns", ptr(float64(compute.PatternSentiment(patterns))), map[string]any{
			"patterns": names,
			"bar": map[string]any{
				"open":   cur.Open,
				"high":   cur.High,
				"low":    cur.Low,
				"close":  cur.Close,
				"volume": cur.Volume,
			},
		})
	}

	// ── MACD ─────────────────────────────────────────────────────────────────
	if w.cfg.EnableMACD {
		if snap, ok := compute.MACDSnapshotWithPrev(closes, w.cfg.MACDFast, w.cfg.MACDSlow, w.cfg.MACDSignal); ok {
			name := fmt.Sprintf("macd_%d_%d_%d", w.cfg.MACDFast, w.cfg.MACDSlow, w.cfg.MACDSignal)
			payload := map[string]any{
				"macd_line":                 snap.Cur.Line,
				"signal_line":               snap.Cur.Signal,
				"histogram":                 snap.Cur.Hist,
				"fast":                      w.cfg.MACDFast,
				"slow":                      w.cfg.MACDSlow,
				"signal":                    w.cfg.MACDSignal,
				"start_idx":                 snap.Cur.StartIdx,
				"bullish_cross_line_signal": snap.BullishCross,
				"bearish_cross_line_signal": snap.BearishCross,
				"hist_bull_zero_cross":      snap.HistBullZeroCross,
				"hist_bear_zero_cross":      snap.HistBearZeroCross,
			}
			if snap.PrevBarAvailable {
				payload["prev_macd_line"] = snap.Prev.Line
				payload["prev_signal_line"] = snap.Prev.Signal
				payload["prev_histogram"] = snap.Prev.Hist
			}
			upsert(name, ptr(snap.Cur.Hist), payload)
		}
	}

	// ── OBV ──────────────────────────────────────────────────────────────────
	if w.cfg.EnableOBV {
		if total, dlt, ok := compute.OBVLast(bars); ok {
			upsert("obv", ptr(total), map[string]any{
				"obv":            total,
				"last_bar_delta": dlt,
			})
		}
	}

	// ── Bollinger Bands ──────────────────────────────────────────────────────
	// bbLower/bbUpper are captured for the Bollinger Squeeze check below.
	var bbLower, bbUpper float64
	var bbCaptured bool
	if w.cfg.EnableBollinger {
		if bb, ok := compute.BollingerLast(closes, w.cfg.BBPeriod, w.cfg.BBStd); ok {
			bbLower, bbUpper = bb.Lower, bb.Upper
			bbCaptured = true
			bname := fmt.Sprintf("bb_%d_%s", w.cfg.BBPeriod, formatFloatKey(w.cfg.BBStd))
			upsert(bname, ptr(bb.PctB), map[string]any{
				"middle":    bb.Middle,
				"upper":     bb.Upper,
				"lower":     bb.Lower,
				"bandwidth": bb.Bandwidth,
				"pct_b":     bb.PctB,
				"period":    w.cfg.BBPeriod,
				"std_mult":  w.cfg.BBStd,
				"close":     currentPrice,
			})
		}
	}

	// ── Fibonacci ────────────────────────────────────────────────────────────
	if w.cfg.EnableFib {
		fs := w.cfg.FibSwing()
		if fib, ok := compute.FibRetracementFromSwings(highs, lows, closes, fs, w.cfg.FibExtensions); ok {
			fname := fmt.Sprintf("fib_retrace_sw%d", fs)
			upsert(fname, ptr(fib.DistPctToNear), map[string]any{
				"rule":             "last_swing_high_vs_last_swing_low",
				"direction":        fib.Direction,
				"impulse_low":      fib.ImpulseLow,
				"impulse_high":     fib.ImpulseHigh,
				"leg_size":         fib.LegSize,
				"levels":           fib.Levels,
				"extensions":       fib.Extensions,
				"current_close":    fib.CurrentClose,
				"nearest_level":    fib.NearestLevel,
				"nearest_price":    fib.NearestPrice,
				"dist_pct_to_near": fib.DistPctToNear,
				"swing_strength":   fs,
			})
		}
	}

	// ── RSI divergence ───────────────────────────────────────────────────────
	if w.cfg.EnableRSIDivergence {
		divSw := w.cfg.RSIDivSwing()
		div := compute.DetectRSIDivergence(highs, lows, closes, divSw, w.cfg.RSIPeriod)
		dname := fmt.Sprintf("rsi_divergence_rsi%d_sw%d", w.cfg.RSIPeriod, divSw)
		upsert(dname, ptr(compute.RSIDivergenceScore(div)), map[string]any{
			"kind": string(div.Kind),
			"bearish_regular": map[string]any{
				"pattern":    div.BearishPattern,
				"selected":   div.Kind == compute.RSIDivBearish,
				"price_hi_1": div.PriceHigh1,
				"price_hi_2": div.PriceHigh2,
				"rsi_hi_1":   div.RSIHigh1,
				"rsi_hi_2":   div.RSIHigh2,
			},
			"bullish_regular": map[string]any{
				"pattern":    div.BullishPattern,
				"selected":   div.Kind == compute.RSIDivBullish,
				"price_lo_1": div.PriceLow1,
				"price_lo_2": div.PriceLow2,
				"rsi_lo_1":   div.RSILow1,
				"rsi_lo_2":   div.RSILow2,
			},
		})
	}

	// ── RSI hidden divergence ────────────────────────────────────────────────
	if w.cfg.EnableRSIHidden {
		divSw := w.cfg.RSIDivSwing()
		hid := compute.DetectRSIHiddenDivergence(
			highs, lows, closes,
			divSw, w.cfg.RSIPeriod,
			w.cfg.RSIHiddenMinPivotSep,
			w.cfg.RSIHiddenRequireTrend,
			w.cfg.TrendLookback,
		)
		hname := fmt.Sprintf("rsi_hidden_rsi%d_sw%d", w.cfg.RSIPeriod, divSw)
		upsert(hname, ptr(compute.RSIHiddenScore(hid)), map[string]any{
			"kind": string(hid.Kind),
			"bearish_hidden": map[string]any{
				"pattern":    hid.BearishHiddenPattern,
				"selected":   hid.Kind == compute.RSIHiddenBearish,
				"price_hi_1": hid.PriceHigh1,
				"price_hi_2": hid.PriceHigh2,
				"rsi_hi_1":   hid.RSIHigh1,
				"rsi_hi_2":   hid.RSIHigh2,
			},
			"bullish_hidden": map[string]any{
				"pattern":    hid.BullishHiddenPattern,
				"selected":   hid.Kind == compute.RSIHiddenBullish,
				"price_lo_1": hid.PriceLow1,
				"price_lo_2": hid.PriceLow2,
				"rsi_lo_1":   hid.RSILow1,
				"rsi_lo_2":   hid.RSILow2,
			},
			"min_pivot_sep":      w.cfg.RSIHiddenMinPivotSep,
			"require_trend_gate": w.cfg.RSIHiddenRequireTrend,
		})
	}

	// ── Volume profile proxy ─────────────────────────────────────────────────
	if w.cfg.EnableVolProfileProxy && w.cfg.VolProfileBins >= 2 {
		if bins, poc, pocIdx, ok := compute.VolumeProfileProxy(bars, w.cfg.VolProfileBins, w.cfg.VolProfileTypical); ok {
			method := "close"
			if w.cfg.VolProfileTypical {
				method = "typical_price"
			}
			vname := fmt.Sprintf("vol_profile_proxy_b%d_%s", w.cfg.VolProfileBins, method)
			binRows := make([]map[string]float64, len(bins))
			for i, b := range bins {
				binRows[i] = map[string]float64{
					"price_low":  b.PriceLow,
					"price_high": b.PriceHigh,
					"volume":     b.Volume,
				}
			}
			pl := map[string]any{
				"method":                method,
				"disclaimer":            "Each bar's volume assigned to one bin via typical price or close; not true volume-at-price.",
				"bins":                  binRows,
				"poc_price":             poc,
				"poc_bin":               pocIdx,
				"bar_count":             len(bars),
				"value_area_pct_target": w.cfg.VolProfileValueAreaPct,
			}
			if vaLo, vaHi, cov, tot, vaOK := compute.ValueAreaAroundPOC(bins, pocIdx, w.cfg.VolProfileValueAreaPct); vaOK {
				pl["value_area_low"] = vaLo
				pl["value_area_high"] = vaHi
				pl["value_area_volume"] = cov
				pl["histogram_total_volume"] = tot
			}
			upsert(vname, ptr(poc), pl)
		}
	}

	// ── Stochastic slow ───────────────────────────────────────────────────────
	if w.cfg.EnableStochastic {
		if k, d, raw, ok := compute.SlowStochasticLast(highs, lows, closes, w.cfg.StochKPeriod, w.cfg.StochDSmooth, w.cfg.StochDSignal); ok {
			sname := fmt.Sprintf("stoch_slow_%d_%d_%d", w.cfg.StochKPeriod, w.cfg.StochDSmooth, w.cfg.StochDSignal)
			upsert(sname, ptr(k), map[string]any{"k": k, "d": d, "raw_k": raw})
		}
	}

	// ── ATR ───────────────────────────────────────────────────────────────────
	if w.cfg.EnableATR {
		if v, ok := compute.ATRWilder(highs, lows, closes, w.cfg.ATRPeriod); ok {
			upsert(fmt.Sprintf("atr_%d", w.cfg.ATRPeriod), ptr(v), nil)
		}
	}

	// ── Ichimoku ─────────────────────────────────────────────────────────────
	if w.cfg.EnableIchimoku {
		disp := w.cfg.IchimokuDisplace
		if disp <= 0 {
			disp = w.cfg.IchimokuKijun
		}
		if ic, ok := compute.IchimokuLast(highs, lows, closes, w.cfg.IchimokuTenkan, w.cfg.IchimokuKijun, w.cfg.IchimokuSpanB, disp); ok {
			iname := fmt.Sprintf("ichimoku_%d_%d_%d", w.cfg.IchimokuTenkan, w.cfg.IchimokuKijun, w.cfg.IchimokuSpanB)
			upsert(iname, ptr(currentPrice), map[string]any{
				"tenkan":       ic.Tenkan,
				"kijun":        ic.Kijun,
				"senkou_a":     ic.SenkouA,
				"senkou_b":     ic.SenkouB,
				"cloud_top":    ic.CloudTop,
				"cloud_bottom": ic.CloudBot,
				"chikou_close": ic.ChikouClose,
				"displace":     ic.Displace,
				"close_vs_cloud": map[string]any{
					"above_cloud": currentPrice > ic.CloudTop,
					"below_cloud": currentPrice < ic.CloudBot,
					"in_cloud":    currentPrice <= ic.CloudTop && currentPrice >= ic.CloudBot,
				},
			})
		}
	}

	// ── A/D Line ─────────────────────────────────────────────────────────────
	if w.cfg.EnableADLine {
		if v, ok := compute.ADLineLast(bars); ok {
			upsert("ad_line", ptr(v), map[string]any{"cumulative": v})
		}
	}

	// ── ADX ──────────────────────────────────────────────────────────────────
	if w.cfg.EnableADX {
		if adx, ok := compute.ADXWilderLast(highs, lows, closes, w.cfg.ADXPeriod); ok {
			upsert(fmt.Sprintf("adx_%d", w.cfg.ADXPeriod), ptr(adx.ADX), map[string]any{
				"adx": adx.ADX, "plus_di": adx.PlusDI, "minus_di": adx.MinusDI, "dx": adx.DX,
			})
		}
	}

	// ── Pivot levels ─────────────────────────────────────────────────────────
	if w.cfg.EnablePivots && len(bars) >= 2 {
		prev := bars[len(bars)-2]
		pv := compute.PivotsFromPriorBar(prev)
		upsert("pivots_prior_bar", ptr(pv.PP), map[string]any{
			"reference_ts": prev.TS,
			"classic": map[string]float64{
				"PP": pv.PP, "R1": pv.R1, "R2": pv.R2, "R3": pv.R3,
				"S1": pv.S1, "S2": pv.S2, "S3": pv.S3,
			},
			"camarilla": pv.Camarilla,
			"woodie":    pv.Woodie,
		})
	}

	// ── Williams %R ──────────────────────────────────────────────────────────
	if w.cfg.EnableWilliamsR {
		if r, ok := compute.WilliamsRLast(highs, lows, closes, w.cfg.WilliamsRPeriod); ok {
			upsert(fmt.Sprintf("williams_r_%d", w.cfg.WilliamsRPeriod), ptr(r), nil)
		}
	}

	// ── VWAP proxy ───────────────────────────────────────────────────────────
	if w.cfg.EnableVWAP {
		useTyp := w.cfg.VWAPUseTypical
		mode := strings.ToLower(strings.TrimSpace(w.cfg.VWAPMode))
		switch mode {
		case "session":
			if v, day, ok := compute.VWAPSessionLastDay(bars, useTyp); ok {
				upsert("vwap_session_last_day", ptr(v), map[string]any{
					"vwap": v, "utc_day": day, "mode": "session",
				})
			}
		default:
			if v, ok := compute.VWAPRolling(bars, w.cfg.VWAPRollingN, useTyp); ok {
				upsert(fmt.Sprintf("vwap_rolling_%d", w.cfg.VWAPRollingN), ptr(v), map[string]any{
					"vwap": v, "bars": w.cfg.VWAPRollingN, "mode": "rolling",
				})
			}
		}
	}

	// ── MA ribbon + golden/death cross ──────────────────────────────────────
	if w.cfg.EnableMARibbon && len(w.cfg.RibbonPeriods) >= 2 {
		rp := append([]int(nil), w.cfg.RibbonPeriods...)
		sort.Ints(rp)
		if rib, ok := compute.MARibbonEval(closes, rp, w.cfg.MACrossFast, w.cfg.MACrossSlow); ok {
			upsert("ma_ribbon", ptr(rib.Compression), map[string]any{
				"periods":      rib.Periods,
				"smas":         rib.SMAs,
				"bull_stack":   rib.BullStack,
				"bear_stack":   rib.BearStack,
				"compression":  rib.Compression,
				"golden_cross": rib.GoldenCross,
				"death_cross":  rib.DeathCross,
				"cross_fast":   rib.CrossFast,
				"cross_slow":   rib.CrossSlow,
			})
		}
	}

	// ── Chart-pattern hints ──────────────────────────────────────────────────
	if w.cfg.EnableChartPatterns {
		h := compute.DetectChartPatternHints(highs, lows, closes, w.cfg.SRSwingStrength, w.cfg.ChartPatternClusterPct)
		var score float64
		if h.DoubleTopCandidate {
			score -= 1
		}
		if h.DoubleBottomCandidate {
			score += 1
		}
		upsert("chart_pattern_hints", ptr(score), map[string]any{
			"double_top_candidate":    h.DoubleTopCandidate,
			"double_bottom_candidate": h.DoubleBottomCandidate,
			"high1":                   h.High1,
			"high2":                   h.High2,
			"low1":                    h.Low1,
			"low2":                    h.Low2,
			"cluster_pct":             w.cfg.ChartPatternClusterPct,
		})
	}

	// ── CMF ───────────────────────────────────────────────────────────────────
	if w.cfg.EnableCMF {
		if v, ok := compute.ChaikinMoneyFlow(bars, w.cfg.CMFPeriod); ok {
			upsert(fmt.Sprintf("cmf_%d", w.cfg.CMFPeriod), ptr(v), nil)
		}
	}

	// ── Keltner channels ─────────────────────────────────────────────────────
	// keltnerLower/keltnerUpper captured for the Bollinger Squeeze check below.
	var keltnerLower, keltnerUpper float64
	var keltnerCaptured bool
	if w.cfg.EnableKeltner {
		if mid, up, lo, ok := compute.KeltnerLast(highs, lows, closes, w.cfg.KeltnerEMAPeriod, w.cfg.KeltnerATRPeriod, w.cfg.KeltnerMult); ok {
			keltnerLower, keltnerUpper = lo, up
			keltnerCaptured = true
			upsert(fmt.Sprintf("keltner_e%d_a%d_m%s", w.cfg.KeltnerEMAPeriod, w.cfg.KeltnerATRPeriod, formatFloatKey(w.cfg.KeltnerMult)), ptr(mid), map[string]any{
				"middle": mid, "upper": up, "lower": lo,
				"close":         currentPrice,
				"outside_upper": currentPrice > up,
				"outside_lower": currentPrice < lo,
			})
		}
	}

	// ── Bollinger Squeeze ─────────────────────────────────────────────────────
	// Squeeze = BB bands are entirely inside Keltner channels → low volatility
	// coil. A breakout expansion typically follows.
	if w.cfg.EnableBBSqueeze && bbCaptured && keltnerCaptured {
		squeeze := bbLower > keltnerLower && bbUpper < keltnerUpper
		sq := 0.0
		if squeeze {
			sq = 1.0
		}
		upsert("bb_squeeze", ptr(sq), map[string]any{
			"squeeze":       squeeze,
			"bb_lower":      bbLower,
			"bb_upper":      bbUpper,
			"keltner_lower": keltnerLower,
			"keltner_upper": keltnerUpper,
			"explanation":   "BB inside Keltner = low-volatility coil. Watch for expansion breakout.",
		})
	}

	// ── Donchian ─────────────────────────────────────────────────────────────
	if w.cfg.EnableDonchian {
		if up, lo, mid, ok := compute.DonchianLast(highs, lows, w.cfg.DonchianPeriod); ok {
			upsert(fmt.Sprintf("donchian_%d", w.cfg.DonchianPeriod), ptr(mid), map[string]any{
				"upper": up, "lower": lo, "middle": mid, "close": currentPrice,
			})
		}
	}

	// ── Trendline break ──────────────────────────────────────────────────────
	if w.cfg.EnableTrendlineBreak {
		if tl, ok := compute.TrendlineBreakLast(highs, lows, closes, w.cfg.SRSwingStrength, w.cfg.TrendlinePivots); ok {
			var v float64
			if tl.ResistanceBreak {
				v += 1
			}
			if tl.SupportBreak {
				v -= 1
			}
			upsert(fmt.Sprintf("trendline_break_sw%d_p%d", w.cfg.SRSwingStrength, w.cfg.TrendlinePivots), ptr(v), map[string]any{
				"resistance_break": tl.ResistanceBreak,
				"support_break":    tl.SupportBreak,
				"high_line_at_end": tl.HighLineAtEnd,
				"low_line_at_end":  tl.LowLineAtEnd,
				"prev_high_line":   tl.PrevHighLine,
				"prev_low_line":    tl.PrevLowLine,
			})
		}
	}

	// ── CCI ───────────────────────────────────────────────────────────────────
	if w.cfg.EnableCCI {
		if v, ok := compute.CCILast(highs, lows, closes, w.cfg.CCIPeriod); ok {
			upsert(fmt.Sprintf("cci_%d", w.cfg.CCIPeriod), ptr(v), nil)
		}
	}

	// ── ROC ───────────────────────────────────────────────────────────────────
	if w.cfg.EnableROC {
		if v, ok := compute.ROCLast(closes, w.cfg.ROCPeriod); ok {
			upsert(fmt.Sprintf("roc_%d", w.cfg.ROCPeriod), ptr(v), nil)
		}
	}

	// ── Parabolic SAR ────────────────────────────────────────────────────────
	if w.cfg.EnableParabolicSAR {
		if sar, bull, ok := compute.ParabolicSARLast(highs, lows, closes, w.cfg.ParabolicStep, w.cfg.ParabolicMaxAF); ok {
			trend := -1.0
			if bull {
				trend = 1
			}
			upsert(fmt.Sprintf("parabolic_sar_s%s_m%s", formatFloatKey(w.cfg.ParabolicStep), formatFloatKey(w.cfg.ParabolicMaxAF)), ptr(sar), map[string]any{
				"sar": sar, "bullish": bull, "trend": trend,
			})
		}
	}

	// ── MFI ───────────────────────────────────────────────────────────────────
	if w.cfg.EnableMFI {
		if v, ok := compute.MFILast(bars, w.cfg.MFIPeriod); ok {
			upsert(fmt.Sprintf("mfi_%d", w.cfg.MFIPeriod), ptr(v), nil)
		}
	}

	// ── Market structure (BOS / CHoCH) ───────────────────────────────────────
	if w.cfg.EnableMarketStructure {
		if ms, ok := compute.MarketStructureLast(highs, lows, closes, w.cfg.SRSwingStrength); ok {
			var sc float64
			if ms.BullishBOS || ms.CHoCHUp {
				sc += 1
			}
			if ms.BearishBOS || ms.CHoCHDown {
				sc -= 1
			}
			upsert(fmt.Sprintf("market_structure_sw%d", w.cfg.SRSwingStrength), ptr(sc), map[string]any{
				"bullish_bos":      ms.BullishBOS,
				"bearish_bos":      ms.BearishBOS,
				"choch_up":         ms.CHoCHUp,
				"choch_down":       ms.CHoCHDown,
				"prior_swing_high": ms.PriorSwingHigh,
				"last_swing_high":  ms.LastSwingHigh,
				"prior_swing_low":  ms.PriorSwingLow,
				"last_swing_low":   ms.LastSwingLow,
			})
		}
	}

	// ── Elliott hint ─────────────────────────────────────────────────────────
	if w.cfg.EnableElliottHint {
		if eh, ok := compute.ElliottContextFromSwings(highs, lows, w.cfg.SRSwingStrength); ok {
			upsert("elliott_context_hint", ptr(float64(eh.LegEstimate)), map[string]any{
				"swing_highs":  eh.SwingHighCount,
				"swing_lows":   eh.SwingLowCount,
				"leg_estimate": eh.LegEstimate,
				"note":         eh.Note,
			})
		}
	}

	// ── Gann regression hint ─────────────────────────────────────────────────
	if w.cfg.EnableGannHint {
		if g, ok := compute.GannRegressionHint(closes, w.cfg.GannLookback); ok {
			upsert(fmt.Sprintf("gann_regression_lb%d", w.cfg.GannLookback), ptr(g.SlopeDegrees), map[string]any{
				"slope_per_bar":    g.SlopePerBar,
				"slope_degrees":    g.SlopeDegrees,
				"one_to_one_delta": g.OneToOneDelta,
				"disclaimer":       "Price/time scaling not applied; geometric angle is illustrative only.",
			})
		}
	}

	// ── Open interest gap documentation ─────────────────────────────────────
	if w.cfg.EnableOpenInterestInfo {
		upsert("open_interest", nil, map[string]any{
			"available": false,
			"reason":    "Open interest is not part of OHLCV; add a futures/options OI feed to data-ingestion.",
		})
	}

	// ── Fair Value Gaps (SMC) ─────────────────────────────────────────────────
	if w.cfg.EnableFVG {
		fvgs := compute.DetectFVGs(bars, w.cfg.FVGMinGapPct, w.cfg.FVGLookback)
		var lastBullPL, lastBearPL map[string]any
		if fvgs.LastBullish != nil {
			lastBullPL = map[string]any{
				"bar_index": fvgs.LastBullish.BarIndex,
				"gap_low":   fvgs.LastBullish.GapLow,
				"gap_high":  fvgs.LastBullish.GapHigh,
				"gap_pct":   fvgs.LastBullish.GapPct,
			}
		}
		if fvgs.LastBearish != nil {
			lastBearPL = map[string]any{
				"bar_index": fvgs.LastBearish.BarIndex,
				"gap_low":   fvgs.LastBearish.GapLow,
				"gap_high":  fvgs.LastBearish.GapHigh,
				"gap_pct":   fvgs.LastBearish.GapPct,
			}
		}
		upsert(fmt.Sprintf("fvg_min%s_lb%d", formatFloatKey(w.cfg.FVGMinGapPct), w.cfg.FVGLookback),
			ptr(float64(fvgs.ActiveCount)), map[string]any{
				"active_count":  fvgs.ActiveCount,
				"total_count":   len(fvgs.All),
				"last_bullish":  lastBullPL,
				"last_bearish":  lastBearPL,
				"min_gap_pct":   w.cfg.FVGMinGapPct,
				"lookback_bars": w.cfg.FVGLookback,
			})
	}

	// ── Order Blocks (SMC) ────────────────────────────────────────────────────
	if w.cfg.EnableOrderBlocks {
		ob := compute.DetectOrderBlocks(bars, w.cfg.OBSwingStrength, w.cfg.OBImpulseMinPct, w.cfg.OBLookback)
		active := 0
		for _, o := range ob.All {
			if !o.Invalidated {
				active++
			}
		}
		var lastBullOBPL, lastBearOBPL map[string]any
		if ob.LastBullish != nil {
			lastBullOBPL = map[string]any{
				"bar_index": ob.LastBullish.BarIndex,
				"high":      ob.LastBullish.High,
				"low":       ob.LastBullish.Low,
				"open":      ob.LastBullish.Open,
				"close":     ob.LastBullish.Close,
			}
		}
		if ob.LastBearish != nil {
			lastBearOBPL = map[string]any{
				"bar_index": ob.LastBearish.BarIndex,
				"high":      ob.LastBearish.High,
				"low":       ob.LastBearish.Low,
				"open":      ob.LastBearish.Open,
				"close":     ob.LastBearish.Close,
			}
		}
		upsert(fmt.Sprintf("order_blocks_sw%d_imp%s", w.cfg.OBSwingStrength, formatFloatKey(w.cfg.OBImpulseMinPct)),
			ptr(float64(active)), map[string]any{
				"active_count":    active,
				"total_count":     len(ob.All),
				"last_bullish_ob": lastBullOBPL,
				"last_bearish_ob": lastBearOBPL,
				"swing_strength":  w.cfg.OBSwingStrength,
				"impulse_min_pct": w.cfg.OBImpulseMinPct,
				"lookback_bars":   w.cfg.OBLookback,
			})
	}

	// ── Liquidity Sweeps (SMC) ────────────────────────────────────────────────
	if w.cfg.EnableLiquiditySweep {
		sweeps := compute.DetectLiquiditySweeps(bars, w.cfg.LiquiditySwingStrength, w.cfg.LiquidityLookback)
		highSweeps, lowSweeps := 0, 0
		var lastSweepPL map[string]any
		for _, sv := range sweeps {
			if sv.Kind == compute.SweepHigh {
				highSweeps++
			} else {
				lowSweeps++
			}
		}
		if len(sweeps) > 0 {
			last := sweeps[len(sweeps)-1]
			lastSweepPL = map[string]any{
				"kind":        string(last.Kind),
				"swept_level": last.SweptLevel,
				"bar_high":    last.BarHigh,
				"bar_low":     last.BarLow,
				"bar_close":   last.BarClose,
				"bar_index":   last.BarIndex,
			}
		}
		upsert(fmt.Sprintf("liquidity_sweep_sw%d", w.cfg.LiquiditySwingStrength),
			ptr(float64(len(sweeps))), map[string]any{
				"total_sweeps":   len(sweeps),
				"high_sweeps":    highSweeps,
				"low_sweeps":     lowSweeps,
				"last_sweep":     lastSweepPL,
				"swing_strength": w.cfg.LiquiditySwingStrength,
				"lookback_bars":  w.cfg.LiquidityLookback,
			})
	}

	// ── Head & Shoulders ──────────────────────────────────────────────────────
	if w.cfg.EnableHSPattern {
		hs := compute.DetectHSPattern(bars, w.cfg.HSSwingStrength, w.cfg.HSTolerancePct, w.cfg.HSLookback)
		score := 0.0
		if hs.HSFound {
			score -= 1
			if hs.HSNecklineBreak {
				score -= 1
			}
		}
		if hs.InvHSFound {
			score += 1
			if hs.InvHSNecklineBreak {
				score += 1
			}
		}
		upsert(fmt.Sprintf("hs_pattern_sw%d", w.cfg.HSSwingStrength), ptr(score), map[string]any{
			"hs_found":                hs.HSFound,
			"hs_left_shoulder":        hs.HSLeftShoulder,
			"hs_head":                 hs.HSHead,
			"hs_right_shoulder":       hs.HSRightShoulder,
			"hs_neckline":             hs.HSNeckline,
			"hs_symmetry_pct":         hs.HSShouldersSymmetryPct,
			"hs_neckline_break":       hs.HSNecklineBreak,
			"inv_hs_found":            hs.InvHSFound,
			"inv_hs_left_shoulder":    hs.InvHSLeftShoulder,
			"inv_hs_head":             hs.InvHSHead,
			"inv_hs_right_shoulder":   hs.InvHSRightShoulder,
			"inv_hs_neckline":         hs.InvHSNeckline,
			"inv_hs_symmetry_pct":     hs.InvHSShouldersSymmetryPct,
			"inv_hs_neckline_break":   hs.InvHSNecklineBreak,
			"swing_strength":          w.cfg.HSSwingStrength,
			"shoulder_tolerance_pct":  w.cfg.HSTolerancePct,
		})
	}

	// ── Triangle Patterns ─────────────────────────────────────────────────────
	if w.cfg.EnableTriangle {
		tri := compute.DetectTriangle(bars, w.cfg.TriangleSwingStrength, w.cfg.TriangleMinPivots,
			w.cfg.TriangleFlatThresholdPct, w.cfg.TriangleLookback)
		triScore := 0.0
		switch tri.Breakout {
		case "up":
			triScore = 1
		case "down":
			triScore = -1
		}
		upsert(fmt.Sprintf("triangle_sw%d", w.cfg.TriangleSwingStrength), ptr(triScore), map[string]any{
			"kind":            string(tri.Kind),
			"high_slope_pct":  tri.HighSlopePct,
			"low_slope_pct":   tri.LowSlopePct,
			"apex_bars_away":  tri.ApexBarsAway,
			"breakout":        tri.Breakout,
			"swing_strength":  w.cfg.TriangleSwingStrength,
			"min_pivots":      w.cfg.TriangleMinPivots,
			"flat_thresh_pct": w.cfg.TriangleFlatThresholdPct,
		})
	}

	// ── Flag / Pennant ────────────────────────────────────────────────────────
	if w.cfg.EnableFlag {
		flag := compute.DetectFlag(bars, w.cfg.FlagPolePct, w.cfg.FlagMaxRetracePct, w.cfg.FlagPoleLen, w.cfg.FlagLen)
		flagScore := 0.0
		if flag.BullFlag {
			flagScore = 1
		}
		if flag.BearFlag {
			flagScore = -1
		}
		upsert(fmt.Sprintf("flag_pole%s_len%d", formatFloatKey(w.cfg.FlagPolePct), w.cfg.FlagLen), ptr(flagScore), map[string]any{
			"bull_flag":           flag.BullFlag,
			"bear_flag":           flag.BearFlag,
			"pole_pct":            flag.PolePct,
			"pole_min_pct":        w.cfg.FlagPolePct,
			"max_retracement_pct": w.cfg.FlagMaxRetracePct,
			"pole_len_bars":       w.cfg.FlagPoleLen,
			"flag_len_bars":       w.cfg.FlagLen,
		})
	}

	w.computeRSBenchmark(ctx, ts, symbol, exchange, interval, bars)
	w.computeMTFConfluence(ctx, ts, symbol, exchange, interval, bars)
	w.computeVIXRegime(ctx, ts, symbol, exchange, interval)
	w.computeMultiTFPivots(ctx, ts, symbol, exchange, interval)

	w.log.Info("indicators computed",
		"symbol", symbol,
		"exchange", exchange,
		"interval", interval,
		"ts", ts.Format("2006-01-02"),
	)
}

func (w *worker) computeRSBenchmark(ctx context.Context, ts time.Time, symbol, exchange, interval string, assetBars []compute.Bar) {
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
		other, err = store.QueryEquityBars(ctx, w.pool, bench, interval, w.cfg.ComputeLookback)
	case "binance":
		other, err = store.QueryCryptoBars(ctx, w.pool, bench, interval, w.cfg.ComputeLookback)
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

func (w *worker) computeMTFConfluence(ctx context.Context, ts time.Time, symbol, exchange, interval string, primaryBars []compute.Bar) {
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
			sec, err = store.QueryEquityBars(ctx, w.pool, symbol, iv2, w.cfg.ComputeLookback)
		case "binance":
			sec, err = store.QueryCryptoBars(ctx, w.pool, symbol, iv2, w.cfg.ComputeLookback)
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
func (w *worker) computeVIXRegime(ctx context.Context, ts time.Time, symbol, exchange, interval string) {
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

	regime := "normal"
	switch {
	case vix > w.cfg.VIXFearThreshold:
		regime = "extreme_fear"
	case vix > w.cfg.VIXElevatedThreshold:
		regime = "elevated"
	case vix < w.cfg.VIXComplacencyThreshold:
		regime = "complacency"
	}

	upsert := func(indicator string, value *float64, payload any) {
		if err := store.UpsertIndicator(ctx, w.pool, ts, symbol, exchange, interval, indicator, value, payload); err != nil {
			w.log.Error("upsert vix_regime", "indicator", indicator, "err", err)
		}
	}
	ptr := func(v float64) *float64 { return &v }
	upsert("vix_regime", ptr(vix), map[string]any{
		"vix":                     vix,
		"regime":                  regime,
		"fear_threshold":          w.cfg.VIXFearThreshold,
		"elevated_threshold":      w.cfg.VIXElevatedThreshold,
		"complacency_threshold":   w.cfg.VIXComplacencyThreshold,
		"series_id":               "VIXCLS",
	})
}

// computeMultiTFPivots queries weekly (and optionally monthly) bars and stores
// Classic, Camarilla, and Woodie's pivot levels derived from the prior period bar.
// These are separate from the per-bar pivot (pivots_prior_bar) which uses the
// prior daily bar.
func (w *worker) computeMultiTFPivots(ctx context.Context, ts time.Time, symbol, exchange, interval string) {
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
				wBars, err = store.QueryEquityBars(ctx, w.pool, symbol, weekIv, w.cfg.WeeklyPivotLookback)
			case "binance":
				wBars, err = store.QueryCryptoBars(ctx, w.pool, symbol, weekIv, w.cfg.WeeklyPivotLookback)
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
				mBars, err = store.QueryEquityBars(ctx, w.pool, symbol, monIv, w.cfg.MonthlyPivotLookback)
			case "binance":
				mBars, err = store.QueryCryptoBars(ctx, w.pool, symbol, monIv, w.cfg.MonthlyPivotLookback)
			}
			if err != nil {
				w.log.Warn("monthly pivot query", "symbol", symbol, "interval", monIv, "err", err)
			} else {
				pivotUpsert("pivots_monthly", monIv, mBars)
			}
		}
	}
}

func startupDelay() int {
	s := strings.TrimSpace(os.Getenv("ANALYZER_STARTUP_DELAY_SECS"))
	if s == "" {
		return 60
	}
	v, err := strconv.Atoi(s)
	if err != nil || v < 0 {
		return 60
	}
	return v
}

// formatFloatKey turns 2.0 → "2", 2.5 → "2.5" for stable indicator names.
func formatFloatKey(f float64) string {
	s := fmt.Sprintf("%.4f", f)
	s = strings.TrimRight(s, "0")
	s = strings.TrimRight(s, ".")
	if s == "" {
		return "0"
	}
	return s
}
