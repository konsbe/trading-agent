package main

import (
	"encoding/json"
	"testing"

	"github.com/konsbe/trading-agent/services/data-analyzer/internal/heuristics"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/technical"
)

// The replay's pinned config must be the live worker's.
func TestHeuristicsDefaultConfigIsTheLiveWorker(t *testing.T) {
	live := liveTechnicalConfig(t)
	hc := heuristics.DefaultConfig()
	if live.ComputeLookback != hc.Lookback {
		t.Errorf("lookback: worker %d, replay %d", live.ComputeLookback, hc.Lookback)
	}
	if got := technical.ParamsFrom(live); got != hc.Params {
		t.Errorf("params differ:\n worker %+v\n replay %+v", got, hc.Params)
	}
}

// heuristics.ComputeAt(bars, i) must say what the worker's stored payloads
// say when the worker runs on the window it would have loaded at bar i.
func TestComputeAtMatchesWorkerPayloads(t *testing.T) {
	cfg := liveTechnicalConfig(t)
	hc := heuristics.DefaultConfig()
	bars := goldenBars(800)
	checked := 0
	for i := 20; i < len(bars); i += 7 {
		window := heuristics.Window(bars, i, cfg.ComputeLookback)
		got := map[string]map[string]any{}
		vals := map[string]*float64{}
		indicatorEmitter{cfg: cfg}.emit(window, func(name string, v *float64, payload any) {
			var m map[string]any
			if payload != nil {
				jb, _ := json.Marshal(payload)
				_ = json.Unmarshal(jb, &m)
			}
			got[name], vals[name] = m, v
		})
		s := heuristics.ComputeAt(bars, i, hc, 0, false)

		checkOpt := func(name string, ok bool, want float64) {
			v, present := vals[name]
			if present != ok {
				t.Fatalf("i=%d %s: worker present=%v, snapshot ok=%v", i, name, present, ok)
			}
			if ok && *v != want {
				t.Fatalf("i=%d %s: worker %v, snapshot %v", i, name, *v, want)
			}
		}
		checkOpt("rsi_14", s.RSIOK, s.RSI)
		checkOpt("atr_14", s.ATROK, s.ATR)
		checkOpt("macd_12_26_9", s.MACDOK, s.MACDHist)
		if s.MACDOK {
			m := got["macd_12_26_9"]
			if m["bullish_cross_line_signal"] != s.MACDBullCross || m["bearish_cross_line_signal"] != s.MACDBearCross {
				t.Fatalf("i=%d macd crosses differ: %v vs %+v", i, m, s)
			}
		}
		if sq, ok := got["bb_squeeze"]; ok != s.SqueezeOK || (ok && sq["squeeze"] != s.Squeeze) {
			t.Fatalf("i=%d bb_squeeze: worker %v, snapshot ok=%v squeeze=%v", i, sq, s.SqueezeOK, s.Squeeze)
		}
		if tr, ok := got["trend"]; ok != s.TrendOK || (ok && tr["direction"] != s.TrendDir) {
			t.Fatalf("i=%d trend: worker %v, snapshot %q", i, tr, s.TrendDir)
		}
		fl := got["flag_pole5_len10"]
		if fl["bull_flag"] != s.BullFlag || fl["bear_flag"] != s.BearFlag {
			t.Fatalf("i=%d flag differs", i)
		}
		hs := got["hs_pattern_sw5"]
		if hs["hs_found"] != s.HSFound || hs["hs_neckline_break"] != s.HSNecklineBreak ||
			hs["inv_hs_found"] != s.InvHSFound || hs["inv_hs_neckline_break"] != s.InvHSNecklineBreak {
			t.Fatalf("i=%d hs differs", i)
		}
		ob := got["order_blocks_sw3_imp1.5"]
		if (ob["last_bullish_ob"] != nil) != s.LastBullishOB || (ob["last_bearish_ob"] != nil) != s.LastBearishOB {
			t.Fatalf("i=%d order blocks differ", i)
		}
		ls := got["liquidity_sweep_sw3"]
		if int(ls["total_sweeps"].(float64)) != s.TotalSweeps {
			t.Fatalf("i=%d total_sweeps differ", i)
		}
		last, _ := ls["last_sweep"].(map[string]any)
		if (last != nil) != (s.LastSweep != nil) {
			t.Fatalf("i=%d last_sweep presence differs", i)
		}
		if last != nil {
			if last["kind"] != s.LastSweep.Kind || last["bar_close"] != s.LastSweep.BarClose || last["swept_level"] != s.LastSweep.SweptLevel {
				t.Fatalf("i=%d last_sweep differs: %v vs %+v", i, last, s.LastSweep)
			}
			onBar := int(last["bar_index"].(float64)) == len(window)-1
			if onBar != s.LastSweep.OnBar {
				t.Fatalf("i=%d last_sweep on-bar differs", i)
			}
		}
		// The sweep rule reads exactly these payload fields; feed the worker's
		// payloads through it and compare with the snapshot's rule output.
		if s.TotalSweeps > 0 {
			in := heuristics.SweepRuleInput{
				BullishOB: ob["last_bullish_ob"] != nil,
				BearishOB: ob["last_bearish_ob"] != nil,
			}
			if tr, ok := got["trend"]; ok {
				in.TrendDir, _ = tr["direction"].(string)
			}
			if last != nil {
				bc, sl := last["bar_close"].(float64), last["swept_level"].(float64)
				in.Kind, in.BarClose, in.SweptLevel = last["kind"].(string), &bc, &sl
			}
			if want := heuristics.EvaluateSweepRule(in); s.SweepRule == nil || *s.SweepRule != want {
				t.Fatalf("i=%d sweep rule: from payloads %+v, snapshot %+v", i, want, s.SweepRule)
			}
		} else if s.SweepRule != nil {
			t.Fatalf("i=%d rule output without a firing liquidity_sweep alert", i)
		}
		checked++
	}
	if checked < 100 {
		t.Fatalf("checked only %d bars", checked)
	}
}
