package heuristics

import (
	"time"

	"github.com/konsbe/trading-agent/services/data-analyzer/internal/compute"
)

// Alert onsets: the analyst bot's alerts fire when a condition STARTS, not
// while it stays true. An onset at bar i is a firing at i with no firing in the
// OnsetGapSessions bars before it — the same episode rule the pre-registered
// replay used (SIGNAL_EPISODE_GAP_SESSIONS, isEpisodeStart), so the alerts
// mark the events the test evaluated. Computed from bars, because
// technical_indicators keeps only the latest bar's reading.

// OnsetGapSessions is the quiet run an onset needs before it.
const OnsetGapSessions = 5

// RSI lines of the pre-registered H1 / H2 (Fires).
const (
	RSIOverboughtLine = 70.0
	RSIOversoldLine   = 30.0
)

// Onsets is what technical-analysis stores for the latest bar (indicator
// alert_onsets). A flag is false when the condition is not met at i, fired in
// the gap, or cannot be judged (fewer than OnsetGapSessions earlier bars, or
// an input not computable on one of them).
type Onsets struct {
	BarDate     string // UTC session date of bar i
	GapSessions int

	RSIOK          bool
	RSI            float64
	RSIOverbought  bool
	RSIOversold    bool
	BBSqueezeOnset bool
	SweepOnset     bool
	Sweep          *Sweep // the first sweep on bar i, when SweepOnset
	BarsEvaluated  int    // bars ComputeAt evaluated (gap + 1 when judged)
	Judged         bool   // enough history to judge an onset at all
}

// sweepOnBar: any liquidity sweep whose bar is the snapshot's bar.
func sweepOnBar(s Snapshot) bool { return len(s.SweepsOnBar) > 0 }

// OnsetsAt judges onsets at bar i of bars (one bar per session, ascending).
func OnsetsAt(bars []compute.Bar, i int, cfg Config) Onsets {
	out := Onsets{GapSessions: OnsetGapSessions}
	if i < 0 || i >= len(bars) {
		return out
	}
	out.BarDate = bars[i].TS.UTC().Format(time.DateOnly)
	cur := ComputeAt(bars, i, cfg, 0, false)
	out.RSIOK, out.RSI = cur.RSIOK, cur.RSI
	if i < OnsetGapSessions {
		return out
	}
	prior := make([]Snapshot, 0, OnsetGapSessions)
	for j := i - OnsetGapSessions; j < i; j++ {
		prior = append(prior, ComputeAt(bars, j, cfg, 0, false))
	}
	out.Judged, out.BarsEvaluated = true, OnsetGapSessions+1

	// quiet: every prior bar could be judged and did not fire.
	quiet := func(ok func(Snapshot) bool, fires func(Snapshot) bool) bool {
		for _, p := range prior {
			if !ok(p) || fires(p) {
				return false
			}
		}
		return true
	}
	rsiOK := func(s Snapshot) bool { return s.RSIOK }
	sqOK := func(s Snapshot) bool { return s.SqueezeOK }
	always := func(Snapshot) bool { return true }

	out.RSIOverbought = cur.Fires(RSIOverbought) && quiet(rsiOK, func(s Snapshot) bool { return s.Fires(RSIOverbought) })
	out.RSIOversold = cur.Fires(RSIOversold) && quiet(rsiOK, func(s Snapshot) bool { return s.Fires(RSIOversold) })
	out.BBSqueezeOnset = cur.Fires(BBSqueeze) && quiet(sqOK, func(s Snapshot) bool { return s.Fires(BBSqueeze) })
	if sweepOnBar(cur) && quiet(always, sweepOnBar) {
		out.SweepOnset = true
		sw := cur.SweepsOnBar[0]
		out.Sweep = &sw
	}
	return out
}

// Count is how many onsets the bar has (the stored indicator value).
func (o Onsets) Count() int {
	n := 0
	for _, b := range []bool{o.RSIOverbought, o.RSIOversold, o.BBSqueezeOnset, o.SweepOnset} {
		if b {
			n++
		}
	}
	return n
}
