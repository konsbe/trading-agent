package heuristics

import (
	"time"

	"github.com/konsbe/trading-agent/services/data-analyzer/internal/compute"
)

// ReplayConfig controls how one symbol's history is walked.
type ReplayConfig struct {
	Config Config
	// MinBars is the smallest window a day is evaluated on. Earlier days are
	// not evaluated at all (no signal, no episode, no comparison row).
	MinBars int
	// GapSessions is SIGNAL_EPISODE_GAP_SESSIONS: an episode is a firing day
	// preceded by at least this many evaluated sessions without a firing.
	GapSessions int
	// ComparisonEvery samples comparison days: session index i (0-based in the
	// symbol's one-bar-per-session series) with i % ComparisonEvery ==
	// ComparisonOffset.
	ComparisonEvery  int
	ComparisonOffset int
}

func DefaultReplayConfig() ReplayConfig {
	return ReplayConfig{
		Config:           DefaultConfig(),
		MinBars:          252,
		GapSessions:      5,
		ComparisonEvery:  5,
		ComparisonOffset: 0,
	}
}

// Row is one output row: an episode (Signal set) or a comparison day.
type Row struct {
	Symbol string
	T      time.Time // session date
	Snap   Snapshot
	// GapSessions is the number of sessions since the previous firing of the
	// same signal (nil if none in the evaluated history). Episodes only.
	GapSessions *int
	InLockbox   bool
	Labels      Labels
	// Fired is the per-signal firing flag on T. Comparison rows only.
	Fired map[Signal]bool
}

// Uptrend is trend direction == "up"; nil when trend was not computable.
func (r Row) Uptrend() *bool {
	if !r.Snap.TrendOK {
		return nil
	}
	up := r.Snap.TrendDir == "up"
	return &up
}

// ATRPct is atr14 / close * 100; nil when ATR is unavailable.
func (r Row) ATRPct() *float64 {
	if !r.Snap.ATROK || r.Snap.Close == 0 {
		return nil
	}
	v := r.Snap.ATR / r.Snap.Close * 100
	return &v
}

// SymbolResult is everything the replay writes for one symbol.
type SymbolResult struct {
	Episodes   map[Signal][]Row
	Comparison []Row
	Evaluated  int // sessions evaluated
}

// ReplaySymbol walks bars (one per session, ascending) and returns episodes per
// signal type plus the comparison sample. Labels are attached per row via
// LabelsFor, so lockbox rows never read a forward close.
func ReplaySymbol(symbol string, bars []compute.Bar, vix VIXSeries, lb Lockbox, rc ReplayConfig) SymbolResult {
	res := SymbolResult{Episodes: map[Signal][]Row{}}
	first := rc.MinBars - 1
	if first < 0 {
		first = 0
	}
	if len(bars) <= first {
		return res
	}
	lastFire := map[Signal]int{}
	for i := first; i < len(bars); i++ {
		v, vok := vix.At(bars[i].TS)
		snap := ComputeAt(bars, i, rc.Config, v, vok)
		res.Evaluated++

		var fired map[Signal]bool
		sample := rc.ComparisonEvery > 0 && i%rc.ComparisonEvery == rc.ComparisonOffset
		if sample {
			fired = make(map[Signal]bool, len(Signals))
		}
		var inLB *bool
		lockbox := func() bool {
			if inLB == nil {
				b := lb.Contains(symbol, bars, i)
				inLB = &b
			}
			return *inLB
		}
		for _, sig := range Signals {
			f := snap.Fires(sig)
			if sample {
				fired[sig] = f
			}
			if !f {
				continue
			}
			prev, seen := lastFire[sig]
			lastFire[sig] = i
			if !isEpisodeStart(i, prev, seen, first, rc.GapSessions) {
				continue
			}
			row := Row{Symbol: symbol, T: sessionDate(bars[i].TS), Snap: snap, InLockbox: lockbox()}
			if seen {
				g := i - prev
				row.GapSessions = &g
			}
			row.Labels = LabelsFor(bars, i, row.InLockbox)
			res.Episodes[sig] = append(res.Episodes[sig], row)
		}
		if sample {
			row := Row{Symbol: symbol, T: sessionDate(bars[i].TS), Snap: snap, InLockbox: lockbox(), Fired: fired}
			row.Labels = LabelsFor(bars, i, row.InLockbox)
			res.Comparison = append(res.Comparison, row)
		}
	}
	return res
}

// isEpisodeStart: a firing at i starts an episode when at least gap evaluated
// sessions without a firing precede it. prev is the previous firing (seen
// false if none since first, the first evaluated session — which cannot vouch
// for the sessions before it).
func isEpisodeStart(i, prev int, seen bool, first, gap int) bool {
	if seen {
		return i-prev-1 >= gap
	}
	return i-first >= gap
}
