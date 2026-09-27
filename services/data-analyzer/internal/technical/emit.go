package technical

import (
	"fmt"
	"sort"
	"strings"

	"github.com/konsbe/trading-agent/services/data-analyzer/internal/compute"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/config"
)

// Emitter computes every enabled single-series indicator on bars and hands
// each (name, value, payload) to upsert. Pure: no DB, no clock.
//
// The names, values and payloads are a live contract (the analyst bot and
// momentum-api read them), pinned by
// cmd/technical-analysis/testdata/indicators_golden.json.
type Emitter struct {
	Cfg config.TechnicalAnalysis
}

func (e Emitter) Emit(bars []compute.Bar, upsert func(indicator string, value *float64, payload any)) {
	closes := compute.Closes(bars)
	highs := compute.Highs(bars)
	lows := compute.Lows(bars)
	volumes := compute.Volumes(bars)
	currentPrice := closes[len(closes)-1]
	tp := ParamsFrom(e.Cfg)

	ptr := func(v float64) *float64 { return &v }

	// ── Moving Averages ──────────────────────────────────────────────────────
	if e.Cfg.EnableMA {
		for _, p := range e.Cfg.SMAPeriods {
			if v, ok := compute.SMA(closes, p); ok {
				upsert(fmt.Sprintf("sma_%d", p), ptr(v), nil)
			}
		}
		for _, p := range e.Cfg.EMAPeriods {
			if v, ok := compute.EMA(closes, p); ok {
				upsert(fmt.Sprintf("ema_%d", p), ptr(v), nil)
			}
		}
	}

	// ── RSI ──────────────────────────────────────────────────────────────────
	if e.Cfg.EnableRSI {
		if v, ok := RSI(closes, tp); ok {
			upsert(fmt.Sprintf("rsi_%d", e.Cfg.RSIPeriod), ptr(v), nil)
		}
	}

	// ── Volume ───────────────────────────────────────────────────────────────
	if e.Cfg.EnableVolume {
		if v, ok := compute.VolumeSMA(volumes, e.Cfg.VolSMAPeriod); ok {
			upsert(fmt.Sprintf("vol_sma_%d", e.Cfg.VolSMAPeriod), ptr(v), nil)
		}
		if v, ok := compute.RelativeVolume(volumes, e.Cfg.VolSMAPeriod); ok {
			upsert("rel_vol", ptr(v), nil)
		}
	}

	// ── Support & Resistance ─────────────────────────────────────────────────
	if e.Cfg.EnableSR {
		sr := compute.FindSR(highs, lows, currentPrice,
			e.Cfg.SRSwingStrength, e.Cfg.SRLevels, e.Cfg.SRClusterPct)

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
	if e.Cfg.EnableTrend {
		if t, ok := Trend(closes, highs, lows, tp); ok {
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
	if e.Cfg.EnableCandles {
		// Scan only the last CandleWindow bars (TECHNICAL_CANDLE_WINDOW, default 3).
		window := bars
		if cw := e.Cfg.CandleWindow; cw > 0 && len(window) > cw {
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
	if e.Cfg.EnableMACD {
		if snap, ok := MACD(closes, tp); ok {
			name := fmt.Sprintf("macd_%d_%d_%d", e.Cfg.MACDFast, e.Cfg.MACDSlow, e.Cfg.MACDSignal)
			payload := map[string]any{
				"macd_line":                 snap.Cur.Line,
				"signal_line":               snap.Cur.Signal,
				"histogram":                 snap.Cur.Hist,
				"fast":                      e.Cfg.MACDFast,
				"slow":                      e.Cfg.MACDSlow,
				"signal":                    e.Cfg.MACDSignal,
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
	if e.Cfg.EnableOBV {
		if total, dlt, ok := compute.OBVLast(bars); ok {
			upsert("obv", ptr(total), map[string]any{
				"obv":            total,
				"last_bar_delta": dlt,
			})
		}
	}

	// ── Bollinger Bands ──────────────────────────────────────────────────────
	// bbBands is captured for the Bollinger Squeeze check below.
	var bbBands compute.BollingerResult
	var bbCaptured bool
	if e.Cfg.EnableBollinger {
		if bb, ok := Bollinger(closes, tp); ok {
			bbBands = bb
			bbCaptured = true
			bname := fmt.Sprintf("bb_%d_%s", e.Cfg.BBPeriod, formatFloatKey(e.Cfg.BBStd))
			upsert(bname, ptr(bb.PctB), map[string]any{
				"middle":    bb.Middle,
				"upper":     bb.Upper,
				"lower":     bb.Lower,
				"bandwidth": bb.Bandwidth,
				"pct_b":     bb.PctB,
				"period":    e.Cfg.BBPeriod,
				"std_mult":  e.Cfg.BBStd,
				"close":     currentPrice,
			})
		}
	}

	// ── Fibonacci ────────────────────────────────────────────────────────────
	if e.Cfg.EnableFib {
		fs := e.Cfg.FibSwing()
		if fib, ok := compute.FibRetracementFromSwings(highs, lows, closes, fs, e.Cfg.FibExtensions); ok {
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
	if e.Cfg.EnableRSIDivergence {
		divSw := e.Cfg.RSIDivSwing()
		div := compute.DetectRSIDivergence(highs, lows, closes, divSw, e.Cfg.RSIPeriod)
		dname := fmt.Sprintf("rsi_divergence_rsi%d_sw%d", e.Cfg.RSIPeriod, divSw)
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
	if e.Cfg.EnableRSIHidden {
		divSw := e.Cfg.RSIDivSwing()
		hid := compute.DetectRSIHiddenDivergence(
			highs, lows, closes,
			divSw, e.Cfg.RSIPeriod,
			e.Cfg.RSIHiddenMinPivotSep,
			e.Cfg.RSIHiddenRequireTrend,
			e.Cfg.TrendLookback,
		)
		hname := fmt.Sprintf("rsi_hidden_rsi%d_sw%d", e.Cfg.RSIPeriod, divSw)
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
			"min_pivot_sep":      e.Cfg.RSIHiddenMinPivotSep,
			"require_trend_gate": e.Cfg.RSIHiddenRequireTrend,
		})
	}

	// ── Volume profile proxy ─────────────────────────────────────────────────
	if e.Cfg.EnableVolProfileProxy && e.Cfg.VolProfileBins >= 2 {
		if bins, poc, pocIdx, ok := compute.VolumeProfileProxy(bars, e.Cfg.VolProfileBins, e.Cfg.VolProfileTypical); ok {
			method := "close"
			if e.Cfg.VolProfileTypical {
				method = "typical_price"
			}
			vname := fmt.Sprintf("vol_profile_proxy_b%d_%s", e.Cfg.VolProfileBins, method)
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
				"value_area_pct_target": e.Cfg.VolProfileValueAreaPct,
			}
			if vaLo, vaHi, cov, tot, vaOK := compute.ValueAreaAroundPOC(bins, pocIdx, e.Cfg.VolProfileValueAreaPct); vaOK {
				pl["value_area_low"] = vaLo
				pl["value_area_high"] = vaHi
				pl["value_area_volume"] = cov
				pl["histogram_total_volume"] = tot
			}
			upsert(vname, ptr(poc), pl)
		}
	}

	// ── Stochastic slow ───────────────────────────────────────────────────────
	if e.Cfg.EnableStochastic {
		if k, d, raw, ok := compute.SlowStochasticLast(highs, lows, closes, e.Cfg.StochKPeriod, e.Cfg.StochDSmooth, e.Cfg.StochDSignal); ok {
			sname := fmt.Sprintf("stoch_slow_%d_%d_%d", e.Cfg.StochKPeriod, e.Cfg.StochDSmooth, e.Cfg.StochDSignal)
			upsert(sname, ptr(k), map[string]any{"k": k, "d": d, "raw_k": raw})
		}
	}

	// ── ATR ───────────────────────────────────────────────────────────────────
	if e.Cfg.EnableATR {
		if v, ok := ATR(highs, lows, closes, tp); ok {
			upsert(fmt.Sprintf("atr_%d", e.Cfg.ATRPeriod), ptr(v), nil)
		}
	}

	// ── Ichimoku ─────────────────────────────────────────────────────────────
	if e.Cfg.EnableIchimoku {
		disp := e.Cfg.IchimokuDisplace
		if disp <= 0 {
			disp = e.Cfg.IchimokuKijun
		}
		if ic, ok := compute.IchimokuLast(highs, lows, closes, e.Cfg.IchimokuTenkan, e.Cfg.IchimokuKijun, e.Cfg.IchimokuSpanB, disp); ok {
			iname := fmt.Sprintf("ichimoku_%d_%d_%d", e.Cfg.IchimokuTenkan, e.Cfg.IchimokuKijun, e.Cfg.IchimokuSpanB)
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
	if e.Cfg.EnableADLine {
		if v, ok := compute.ADLineLast(bars); ok {
			upsert("ad_line", ptr(v), map[string]any{"cumulative": v})
		}
	}

	// ── ADX ──────────────────────────────────────────────────────────────────
	if e.Cfg.EnableADX {
		if adx, ok := compute.ADXWilderLast(highs, lows, closes, e.Cfg.ADXPeriod); ok {
			upsert(fmt.Sprintf("adx_%d", e.Cfg.ADXPeriod), ptr(adx.ADX), map[string]any{
				"adx": adx.ADX, "plus_di": adx.PlusDI, "minus_di": adx.MinusDI, "dx": adx.DX,
			})
		}
	}

	// ── Pivot levels ─────────────────────────────────────────────────────────
	if e.Cfg.EnablePivots && len(bars) >= 2 {
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
	if e.Cfg.EnableWilliamsR {
		if r, ok := compute.WilliamsRLast(highs, lows, closes, e.Cfg.WilliamsRPeriod); ok {
			upsert(fmt.Sprintf("williams_r_%d", e.Cfg.WilliamsRPeriod), ptr(r), nil)
		}
	}

	// ── VWAP proxy ───────────────────────────────────────────────────────────
	if e.Cfg.EnableVWAP {
		useTyp := e.Cfg.VWAPUseTypical
		mode := strings.ToLower(strings.TrimSpace(e.Cfg.VWAPMode))
		switch mode {
		case "session":
			if v, day, ok := compute.VWAPSessionLastDay(bars, useTyp); ok {
				upsert("vwap_session_last_day", ptr(v), map[string]any{
					"vwap": v, "utc_day": day, "mode": "session",
				})
			}
		default:
			if v, ok := compute.VWAPRolling(bars, e.Cfg.VWAPRollingN, useTyp); ok {
				upsert(fmt.Sprintf("vwap_rolling_%d", e.Cfg.VWAPRollingN), ptr(v), map[string]any{
					"vwap": v, "bars": e.Cfg.VWAPRollingN, "mode": "rolling",
				})
			}
		}
	}

	// ── MA ribbon + golden/death cross ──────────────────────────────────────
	if e.Cfg.EnableMARibbon && len(e.Cfg.RibbonPeriods) >= 2 {
		rp := append([]int(nil), e.Cfg.RibbonPeriods...)
		sort.Ints(rp)
		if rib, ok := compute.MARibbonEval(closes, rp, e.Cfg.MACrossFast, e.Cfg.MACrossSlow); ok {
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
	if e.Cfg.EnableChartPatterns {
		h := compute.DetectChartPatternHints(highs, lows, closes, e.Cfg.SRSwingStrength, e.Cfg.ChartPatternClusterPct)
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
			"cluster_pct":             e.Cfg.ChartPatternClusterPct,
		})
	}

	// ── CMF ───────────────────────────────────────────────────────────────────
	if e.Cfg.EnableCMF {
		if v, ok := compute.ChaikinMoneyFlow(bars, e.Cfg.CMFPeriod); ok {
			upsert(fmt.Sprintf("cmf_%d", e.Cfg.CMFPeriod), ptr(v), nil)
		}
	}

	// ── Keltner channels ─────────────────────────────────────────────────────
	// kc is captured for the Bollinger Squeeze check below.
	var kc Keltner
	var keltnerCaptured bool
	if e.Cfg.EnableKeltner {
		if k, ok := KeltnerChannel(highs, lows, closes, tp); ok {
			kc = k
			keltnerCaptured = true
			upsert(fmt.Sprintf("keltner_e%d_a%d_m%s", e.Cfg.KeltnerEMAPeriod, e.Cfg.KeltnerATRPeriod, formatFloatKey(e.Cfg.KeltnerMult)), ptr(k.Middle), map[string]any{
				"middle": k.Middle, "upper": k.Upper, "lower": k.Lower,
				"close":         currentPrice,
				"outside_upper": currentPrice > k.Upper,
				"outside_lower": currentPrice < k.Lower,
			})
		}
	}

	// ── Bollinger Squeeze ─────────────────────────────────────────────────────
	// Squeeze = BB bands are entirely inside Keltner channels: recent volatility
	// is low. It describes the condition only — the pre-registered test (H5)
	// found squeezes followed by slightly smaller moves, not larger ones.
	if e.Cfg.EnableBBSqueeze && bbCaptured && keltnerCaptured {
		squeeze := Squeeze(bbBands, kc)
		sq := 0.0
		if squeeze {
			sq = 1.0
		}
		upsert("bb_squeeze", ptr(sq), map[string]any{
			"squeeze":       squeeze,
			"bb_lower":      bbBands.Lower,
			"bb_upper":      bbBands.Upper,
			"keltner_lower": kc.Lower,
			"keltner_upper": kc.Upper,
			"explanation":   "The Bollinger Bands are inside the Keltner Channel (recent volatility is low).",
		})
	}

	// ── Donchian ─────────────────────────────────────────────────────────────
	if e.Cfg.EnableDonchian {
		if up, lo, mid, ok := compute.DonchianLast(highs, lows, e.Cfg.DonchianPeriod); ok {
			upsert(fmt.Sprintf("donchian_%d", e.Cfg.DonchianPeriod), ptr(mid), map[string]any{
				"upper": up, "lower": lo, "middle": mid, "close": currentPrice,
			})
		}
	}

	// ── Trendline break ──────────────────────────────────────────────────────
	if e.Cfg.EnableTrendlineBreak {
		if tl, ok := compute.TrendlineBreakLast(highs, lows, closes, e.Cfg.SRSwingStrength, e.Cfg.TrendlinePivots); ok {
			var v float64
			if tl.ResistanceBreak {
				v += 1
			}
			if tl.SupportBreak {
				v -= 1
			}
			upsert(fmt.Sprintf("trendline_break_sw%d_p%d", e.Cfg.SRSwingStrength, e.Cfg.TrendlinePivots), ptr(v), map[string]any{
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
	if e.Cfg.EnableCCI {
		if v, ok := compute.CCILast(highs, lows, closes, e.Cfg.CCIPeriod); ok {
			upsert(fmt.Sprintf("cci_%d", e.Cfg.CCIPeriod), ptr(v), nil)
		}
	}

	// ── ROC ───────────────────────────────────────────────────────────────────
	if e.Cfg.EnableROC {
		if v, ok := compute.ROCLast(closes, e.Cfg.ROCPeriod); ok {
			upsert(fmt.Sprintf("roc_%d", e.Cfg.ROCPeriod), ptr(v), nil)
		}
	}

	// ── Parabolic SAR ────────────────────────────────────────────────────────
	if e.Cfg.EnableParabolicSAR {
		if sar, bull, ok := compute.ParabolicSARLast(highs, lows, closes, e.Cfg.ParabolicStep, e.Cfg.ParabolicMaxAF); ok {
			trend := -1.0
			if bull {
				trend = 1
			}
			upsert(fmt.Sprintf("parabolic_sar_s%s_m%s", formatFloatKey(e.Cfg.ParabolicStep), formatFloatKey(e.Cfg.ParabolicMaxAF)), ptr(sar), map[string]any{
				"sar": sar, "bullish": bull, "trend": trend,
			})
		}
	}

	// ── MFI ───────────────────────────────────────────────────────────────────
	if e.Cfg.EnableMFI {
		if v, ok := compute.MFILast(bars, e.Cfg.MFIPeriod); ok {
			upsert(fmt.Sprintf("mfi_%d", e.Cfg.MFIPeriod), ptr(v), nil)
		}
	}

	// ── Market structure (BOS / CHoCH) ───────────────────────────────────────
	if e.Cfg.EnableMarketStructure {
		if ms, ok := compute.MarketStructureLast(highs, lows, closes, e.Cfg.SRSwingStrength); ok {
			var sc float64
			if ms.BullishBOS || ms.CHoCHUp {
				sc += 1
			}
			if ms.BearishBOS || ms.CHoCHDown {
				sc -= 1
			}
			upsert(fmt.Sprintf("market_structure_sw%d", e.Cfg.SRSwingStrength), ptr(sc), map[string]any{
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
	if e.Cfg.EnableElliottHint {
		if eh, ok := compute.ElliottContextFromSwings(highs, lows, e.Cfg.SRSwingStrength); ok {
			upsert("elliott_context_hint", ptr(float64(eh.LegEstimate)), map[string]any{
				"swing_highs":  eh.SwingHighCount,
				"swing_lows":   eh.SwingLowCount,
				"leg_estimate": eh.LegEstimate,
				"note":         eh.Note,
			})
		}
	}

	// ── Gann regression hint ─────────────────────────────────────────────────
	if e.Cfg.EnableGannHint {
		if g, ok := compute.GannRegressionHint(closes, e.Cfg.GannLookback); ok {
			upsert(fmt.Sprintf("gann_regression_lb%d", e.Cfg.GannLookback), ptr(g.SlopeDegrees), map[string]any{
				"slope_per_bar":    g.SlopePerBar,
				"slope_degrees":    g.SlopeDegrees,
				"one_to_one_delta": g.OneToOneDelta,
				"disclaimer":       "Price/time scaling not applied; geometric angle is illustrative only.",
			})
		}
	}

	// ── Open interest gap documentation ─────────────────────────────────────
	if e.Cfg.EnableOpenInterestInfo {
		upsert("open_interest", nil, map[string]any{
			"available": false,
			"reason":    "Open interest is not part of OHLCV; add a futures/options OI feed to data-ingestion.",
		})
	}

	// ── Fair Value Gaps (SMC) ─────────────────────────────────────────────────
	if e.Cfg.EnableFVG {
		fvgs := compute.DetectFVGs(bars, e.Cfg.FVGMinGapPct, e.Cfg.FVGLookback)
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
		upsert(fmt.Sprintf("fvg_min%s_lb%d", formatFloatKey(e.Cfg.FVGMinGapPct), e.Cfg.FVGLookback),
			ptr(float64(fvgs.ActiveCount)), map[string]any{
				"active_count":  fvgs.ActiveCount,
				"total_count":   len(fvgs.All),
				"last_bullish":  lastBullPL,
				"last_bearish":  lastBearPL,
				"min_gap_pct":   e.Cfg.FVGMinGapPct,
				"lookback_bars": e.Cfg.FVGLookback,
			})
	}

	// ── Order Blocks (SMC) ────────────────────────────────────────────────────
	if e.Cfg.EnableOrderBlocks {
		ob := DetectOrderBlocks(bars, tp)
		active := ob.Active
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
		upsert(fmt.Sprintf("order_blocks_sw%d_imp%s", e.Cfg.OBSwingStrength, formatFloatKey(e.Cfg.OBImpulseMinPct)),
			ptr(float64(active)), map[string]any{
				"active_count":    active,
				"total_count":     len(ob.All),
				"last_bullish_ob": lastBullOBPL,
				"last_bearish_ob": lastBearOBPL,
				"swing_strength":  e.Cfg.OBSwingStrength,
				"impulse_min_pct": e.Cfg.OBImpulseMinPct,
				"lookback_bars":   e.Cfg.OBLookback,
			})
	}

	// ── Liquidity Sweeps (SMC) ────────────────────────────────────────────────
	if e.Cfg.EnableLiquiditySweep {
		sw := DetectSweeps(bars, tp)
		sweeps, highSweeps, lowSweeps := sw.All, sw.High, sw.Low
		var lastSweepPL map[string]any
		if last := sw.Last(); last != nil {
			lastSweepPL = map[string]any{
				"kind":        string(last.Kind),
				"swept_level": last.SweptLevel,
				"bar_high":    last.BarHigh,
				"bar_low":     last.BarLow,
				"bar_close":   last.BarClose,
				"bar_index":   last.BarIndex,
			}
		}
		upsert(fmt.Sprintf("liquidity_sweep_sw%d", e.Cfg.LiquiditySwingStrength),
			ptr(float64(len(sweeps))), map[string]any{
				"total_sweeps":   len(sweeps),
				"high_sweeps":    highSweeps,
				"low_sweeps":     lowSweeps,
				"last_sweep":     lastSweepPL,
				"swing_strength": e.Cfg.LiquiditySwingStrength,
				"lookback_bars":  e.Cfg.LiquidityLookback,
			})
	}

	// ── Head & Shoulders ──────────────────────────────────────────────────────
	if e.Cfg.EnableHSPattern {
		hs := HeadAndShoulders(bars, tp)
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
		upsert(fmt.Sprintf("hs_pattern_sw%d", e.Cfg.HSSwingStrength), ptr(score), map[string]any{
			"hs_found":               hs.HSFound,
			"hs_left_shoulder":       hs.HSLeftShoulder,
			"hs_head":                hs.HSHead,
			"hs_right_shoulder":      hs.HSRightShoulder,
			"hs_neckline":            hs.HSNeckline,
			"hs_symmetry_pct":        hs.HSShouldersSymmetryPct,
			"hs_neckline_break":      hs.HSNecklineBreak,
			"inv_hs_found":           hs.InvHSFound,
			"inv_hs_left_shoulder":   hs.InvHSLeftShoulder,
			"inv_hs_head":            hs.InvHSHead,
			"inv_hs_right_shoulder":  hs.InvHSRightShoulder,
			"inv_hs_neckline":        hs.InvHSNeckline,
			"inv_hs_symmetry_pct":    hs.InvHSShouldersSymmetryPct,
			"inv_hs_neckline_break":  hs.InvHSNecklineBreak,
			"swing_strength":         e.Cfg.HSSwingStrength,
			"shoulder_tolerance_pct": e.Cfg.HSTolerancePct,
		})
	}

	// ── Triangle Patterns ─────────────────────────────────────────────────────
	if e.Cfg.EnableTriangle {
		tri := compute.DetectTriangle(bars, e.Cfg.TriangleSwingStrength, e.Cfg.TriangleMinPivots,
			e.Cfg.TriangleFlatThresholdPct, e.Cfg.TriangleLookback)
		triScore := 0.0
		switch tri.Breakout {
		case "up":
			triScore = 1
		case "down":
			triScore = -1
		}
		upsert(fmt.Sprintf("triangle_sw%d", e.Cfg.TriangleSwingStrength), ptr(triScore), map[string]any{
			"kind":            string(tri.Kind),
			"high_slope_pct":  tri.HighSlopePct,
			"low_slope_pct":   tri.LowSlopePct,
			"apex_bars_away":  tri.ApexBarsAway,
			"breakout":        tri.Breakout,
			"swing_strength":  e.Cfg.TriangleSwingStrength,
			"min_pivots":      e.Cfg.TriangleMinPivots,
			"flat_thresh_pct": e.Cfg.TriangleFlatThresholdPct,
		})
	}

	// ── Flag / Pennant ────────────────────────────────────────────────────────
	if e.Cfg.EnableFlag {
		flag := Flag(bars, tp)
		flagScore := 0.0
		if flag.BullFlag {
			flagScore = 1
		}
		if flag.BearFlag {
			flagScore = -1
		}
		upsert(fmt.Sprintf("flag_pole%s_len%d", formatFloatKey(e.Cfg.FlagPolePct), e.Cfg.FlagLen), ptr(flagScore), map[string]any{
			"bull_flag":           flag.BullFlag,
			"bear_flag":           flag.BearFlag,
			"pole_pct":            flag.PolePct,
			"pole_min_pct":        e.Cfg.FlagPolePct,
			"max_retracement_pct": e.Cfg.FlagMaxRetracePct,
			"pole_len_bars":       e.Cfg.FlagPoleLen,
			"flag_len_bars":       e.Cfg.FlagLen,
		})
	}
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
