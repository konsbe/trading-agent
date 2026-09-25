package heuristics

import (
	"math"
	"time"

	"github.com/konsbe/trading-agent/services/data-analyzer/internal/compute"
)

// LabelHorizons are the pre-registered forward windows (§1), in sessions.
var LabelHorizons = [3]int{5, 10, 20}

// Labels are the §1 forward labels for a signal on bar t. A nil field is
// unknown (not enough sessions after t, or the row is in the lockbox).
type Labels struct {
	FwdReturn  [3]*float64 // (close[t+N]/close[t] - 1) * 100, N in LabelHorizons
	FwdAbsMove [3]*float64 // max_{i=1..N} |close[t+i]/close[t] - 1| * 100
	Complete   *bool       // at least 20 sessions exist after t
}

// Lockbox is the phase2_lockbox_v2 region: dates Start..End inclusive, for
// symbols outside the pilot cohort.
type Lockbox struct {
	Start, End time.Time // UTC midnight
	Pilot      map[string]bool
}

// Contains reports whether the row for bars[i] is lockbox: a non-pilot symbol
// whose signal date or any session of its label window (t+1..t+20) falls in
// the region. The label window is purged so that no in-sample label is built
// from a lockbox close.
func (lb Lockbox) Contains(symbol string, bars []compute.Bar, i int) bool {
	if lb.Pilot[symbol] {
		return false
	}
	h := LabelHorizons[len(LabelHorizons)-1]
	j := i + h
	if j > len(bars)-1 {
		j = len(bars) - 1
	}
	return !sessionDate(bars[i].TS).After(lb.End) && !sessionDate(bars[j].TS).Before(lb.Start)
}

// LabelsFor returns the forward labels for bars[i], or all-nil labels for a
// lockbox row. For lockbox rows no forward bar is read.
func LabelsFor(bars []compute.Bar, i int, inLockbox bool) Labels {
	if inLockbox {
		return Labels{}
	}
	return forwardLabels(bars, i)
}

func forwardLabels(bars []compute.Bar, i int) Labels {
	var l Labels
	h := LabelHorizons[len(LabelHorizons)-1]
	complete := len(bars)-1-i >= h
	l.Complete = &complete
	c0 := bars[i].Close
	if c0 <= 0 {
		return l
	}
	maxAbs := 0.0
	k := 0
	for n := 1; n <= h && i+n < len(bars); n++ {
		r := (bars[i+n].Close/c0 - 1) * 100
		maxAbs = math.Max(maxAbs, math.Abs(r))
		if k < len(LabelHorizons) && n == LabelHorizons[k] {
			rv, av := r, maxAbs
			l.FwdReturn[k], l.FwdAbsMove[k] = &rv, &av
			k++
		}
	}
	return l
}

// sessionDate is the UTC calendar date of a bar, the session key the worker's
// loader dedupes on.
func sessionDate(ts time.Time) time.Time {
	u := ts.UTC()
	return time.Date(u.Year(), u.Month(), u.Day(), 0, 0, 0, 0, time.UTC)
}

// VIXSeries is VIXCLS by observation date, ascending.
type VIXSeries struct {
	Dates  []time.Time
	Values []float64
}

// At returns the latest observation dated on or before the session of ts.
func (v VIXSeries) At(ts time.Time) (float64, bool) {
	d := sessionDate(ts)
	lo, hi := 0, len(v.Dates) // first index with date > d
	for lo < hi {
		m := (lo + hi) / 2
		if v.Dates[m].After(d) {
			hi = m
		} else {
			lo = m + 1
		}
	}
	if lo == 0 {
		return 0, false
	}
	return v.Values[lo-1], true
}
