package main

import (
	"fmt"
	"os"
	"sort"

	"github.com/konsbe/trading-agent/services/data-analyzer/internal/momentum"
)

// dumpFull writes one row per gate-passing, complete-label candidate with every
// field the post-upgrade out-of-sample report needs.
//
// # WHY A SEPARATE, WIDER DUMP
//
// The existing -dump-candidates emits six columns, which is enough to check a
// base rate and nothing else. The post-upgrade report has to split candidates
// into in-sample and out-of-sample populations, test each scoring component
// separately, and do all of it at the episode level. Those need the sub-scores
// and the underlying features, not just the total.
//
// # WHY THE ANALYSIS IS NOT DONE HERE
//
// This writes data and computes no statistics. The populations, the tests and
// the thresholds all live downstream, so that changing how a result is measured
// never means re-running a two-hour scan — and so that a statistic can never be
// quietly recomputed on a different extraction than the one it was reported on.
//
// # EPISODE FLAG
//
// episode_start marks the first gate pass for a symbol after >= gap sessions
// with none. Consecutive gate-passing days of a single move share nearly the
// same forward label, so counting them as independent observations inflates n
// and makes p-values look stronger than the evidence supports. The flag is
// computed here, over bar INDEXES rather than calendar dates, because the gap
// is defined in trading sessions and a calendar gap would count weekends and
// halts as if the symbol had traded through them.
func dumpFull(path string, rows []row, gap int) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	cols := []string{
		"symbol", "date", "bar_index", "bucket", "episode_start",
		"score_total",
		"sub_rvol", "sub_vol_accel", "sub_catalyst", "sub_float", "sub_vwap",
		"sub_breakout", "sub_high52w",
		"rvol_20", "vol_accel", "change_pct", "gap_pct", "atr_pct",
		"dollar_volume", "rsi_14", "vwap_dist_pct", "above_vwap",
		"breakout_state", "pct_of_52w_high", "range_20",
		"fwd_max_gain_pct", "fwd_max_drawdown_pct", "days_to_peak",
	}
	for i, c := range cols {
		sep := ","
		if i == len(cols)-1 {
			sep = "\n"
		}
		if _, err := fmt.Fprintf(f, "%s%s", c, sep); err != nil {
			return err
		}
	}

	sorted := make([]row, len(rows))
	copy(sorted, rows)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].symbol != sorted[j].symbol {
			return sorted[i].symbol < sorted[j].symbol
		}
		return sorted[i].barIndex < sorted[j].barIndex
	})

	// Walk each symbol's passes in bar order, marking a new episode whenever the
	// previous pass is more than `gap` sessions back. The first pass a symbol
	// ever makes always starts one.
	prevIdx := map[string]int{}
	for _, r := range sorted {
		last, seen := prevIdx[r.symbol]
		start := !seen || r.barIndex-last > gap
		prevIdx[r.symbol] = r.barIndex

		if _, err := fmt.Fprintf(f,
			"%s,%s,%d,%s,%t,%d,"+
				"%.4f,%.4f,%.4f,%.4f,%.4f,%.4f,%.4f,"+
				"%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,"+
				"%.4f,%.4f,%d\n",
			r.symbol, r.date, r.barIndex, r.bucket, start, r.score.Total,
			r.score.Sub.RVol, r.score.Sub.VolAccel, r.score.Sub.Catalyst,
			r.score.Sub.Float, r.score.Sub.VWAP, r.score.Sub.Breakout, r.score.Sub.High52w,
			fnum(r.feat.RVol20), fnum(r.feat.VolAccel), fnum(r.feat.ChangePct),
			fnum(r.feat.GapPct), fnum(r.feat.ATRPct), fnum(r.feat.DollarVolume),
			fnum(r.feat.RSI14), fnum(r.feat.VWAPDistPct), fbool(r.feat.AboveVWAP),
			fstate(r.feat.BreakoutState), fnum(r.feat.PctOf52wHigh), fnum(r.feat.Range20),
			r.label.FwdMaxGainPct, r.label.FwdMaxDrawdownPct, r.label.DaysToPeak,
		); err != nil {
			return err
		}
	}
	return nil
}

// fnum renders a nullable feature as an empty field when absent.
//
// Empty rather than 0, and never imputed: Phase 1 §12 forbids silent
// imputation, and a zero RVOL is a meaningful value that must not be
// confused with an unmeasurable one. The analysis side drops empty fields from
// a test rather than treating them as data.
func fnum(p *float64) string {
	if p == nil {
		return ""
	}
	return fmt.Sprintf("%.6f", *p)
}

func fbool(p *bool) string {
	if p == nil {
		return ""
	}
	return fmt.Sprintf("%t", *p)
}

func fstate(p *momentum.BreakoutState) string {
	if p == nil {
		return ""
	}
	return string(*p)
}
