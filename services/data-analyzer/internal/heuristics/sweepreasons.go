package heuristics

import (
	"fmt"
	"strings"
)

// SweepRuleReasons returns the reason lines liquidity_sweep.evaluate builds,
// verbatim (emoji and spacing included), for the input EvaluateSweepRule
// takes plus the payload's total_sweeps. It follows the same branches as
// EvaluateSweepRule, which is left untouched for the replay; both are pinned to
// the Python rule by testdata/sweep_rule_parity.json.
func SweepRuleReasons(in SweepRuleInput, totalSweeps int) []string {
	var reasons []string
	countStr := ""
	if totalSweeps != 0 {
		countStr = fmt.Sprintf(" (%d recent)", totalSweeps)
	}
	riskOff := in.VIXRegime == "elevated" || in.VIXRegime == "extreme_fear"

	switch {
	case strings.Contains(in.Kind, "low"):
		closedBackAbove := in.BarClose != nil && in.SweptLevel != nil && *in.BarClose > *in.SweptLevel
		reasons = append(reasons, "📍 Low sweep"+countStr+": stop-hunt below swing low detected")
		if closedBackAbove {
			reasons = append(reasons, "✅ Closed back above swept level — institutional accumulation pattern")
			if in.BullishOB {
				reasons = append(reasons, "✅ Bullish order block nearby — strong support confluence")
			}
			if in.TrendDir == "up" {
				reasons = append(reasons, "✅ Uptrend intact — sweep aligns with trend continuation")
			}
			// Python's BUY_WATCH / WATCH branch: the veto line is added exactly
			// when the action falls back to WATCH.
			if riskOff && in.TrendDir != "up" {
				trend := in.TrendDir
				if trend == "" {
					trend = "unknown"
				}
				reasons = append(reasons, "⚠️  VIX regime: "+in.VIXRegime+" + trend "+trend+" — "+
					"low sweep may be a liquidity grab, not genuine accumulation; "+
					"wait for trend confirmation before buying")
			}
		} else {
			reasons = append(reasons, "⚠️  Did not close back above swept level — wait for confirmation")
		}
	case strings.Contains(in.Kind, "high"):
		closedBackBelow := in.BarClose != nil && in.SweptLevel != nil && *in.BarClose < *in.SweptLevel
		reasons = append(reasons, "📍 High sweep"+countStr+": stop-hunt above swing high detected")
		if closedBackBelow {
			reasons = append(reasons, "⚠️  Closed back below swept level — possible distribution / fakeout")
			if in.BearishOB {
				reasons = append(reasons, "⚠️  Bearish order block overhead — resistance confluence")
			}
			if in.TrendDir == "down" {
				reasons = append(reasons, "⚠️  Downtrend active — sweep aligns with distribution")
			}
		} else {
			reasons = append(reasons, "⚠️  Did not close back below swept level — wait for confirmation")
		}
	default:
		reasons = append(reasons, "📍 Liquidity sweep detected"+countStr+" — direction undetermined")
		reasons = append(reasons, "ℹ️  No recent sweep data in payload — check chart")
	}
	return append(reasons, "ℹ️  SMC sweep signals are most reliable on higher timeframes with FVG or OB confluence")
}
