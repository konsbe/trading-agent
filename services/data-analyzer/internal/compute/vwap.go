package compute

import (
	"time"
)

// VWAPRolling computes Σ(typical×vol)/Σ(vol) over the last n bars (oldest-first bars).
func VWAPRolling(bars []Bar, n int, useTypical bool) (float64, bool) {
	if n <= 0 || len(bars) < n {
		return 0, false
	}
	seg := bars[len(bars)-n:]
	var pv, vv float64
	for _, b := range seg {
		var px float64
		if useTypical {
			px = (b.High + b.Low + b.Close) / 3
		} else {
			px = b.Close
		}
		pv += px * b.Volume
		vv += b.Volume
	}
	if vv == 0 {
		return 0, false
	}
	return pv / vv, true
}

// VWAPSessionLastDay aggregates all bars sharing the same UTC calendar date as the final bar.
// Meaningful for intraday intervals (e.g. 1h); on daily bars it collapses to that single bar's TP.
func VWAPSessionLastDay(bars []Bar, useTypical bool) (vwap float64, day string, ok bool) {
	if len(bars) == 0 {
		return 0, "", false
	}
	lastDay := bars[len(bars)-1].TS.UTC().Format("2006-01-02")
	var pv, vv float64
	for i := len(bars) - 1; i >= 0; i-- {
		if bars[i].TS.UTC().Format("2006-01-02") != lastDay {
			break
		}
		var px float64
		if useTypical {
			px = (bars[i].High + bars[i].Low + bars[i].Close) / 3
		} else {
			px = bars[i].Close
		}
		pv += px * bars[i].Volume
		vv += bars[i].Volume
	}
	if vv == 0 {
		return 0, lastDay, false
	}
	return pv / vv, lastDay, true
}

// BarDayUTC returns the UTC date key for a bar (for documentation / payloads).
func BarDayUTC(ts time.Time) string {
	return ts.UTC().Format("2006-01-02")
}
