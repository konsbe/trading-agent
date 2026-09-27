package heuristics

import (
	"math"
	"testing"
	"time"

	"github.com/konsbe/trading-agent/services/data-analyzer/internal/compute"
)

func onsetBars(closes []float64) []compute.Bar {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	bars := make([]compute.Bar, len(closes))
	for i, c := range closes {
		bars[i] = compute.Bar{TS: start.AddDate(0, 0, i), Open: c, High: c * 1.01, Low: c * 0.99, Close: c, Volume: 1e6}
	}
	return bars
}

// A long overbought run is one onset, on its first bar — not one per bar.
func TestOnsetsAt_LongRunIsOneOnset(t *testing.T) {
	var closes []float64
	c := 100.0
	for i := 0; i < 60; i++ { // sideways: RSI near 50
		c += 0.5 * math.Sin(float64(i))
		closes = append(closes, c)
	}
	for i := 0; i < 30; i++ { // steady climb: RSI above 70 for many bars
		c *= 1.02
		closes = append(closes, c)
	}
	bars := onsetBars(closes)
	cfg := DefaultConfig()
	fires, onsets := 0, 0
	for i := range bars {
		if ComputeAt(bars, i, cfg, 0, false).Fires(RSIOverbought) {
			fires++
		}
		if OnsetsAt(bars, i, cfg).RSIOverbought {
			onsets++
		}
	}
	if fires < 10 || onsets != 1 {
		t.Errorf("RSI > 70 on %d bars, onsets %d; want many firing bars and exactly one onset", fires, onsets)
	}
}

// On a noisy series every flag is exactly "fires at i, judged and quiet in
// the OnsetGapSessions bars before" — the replay's episode rule.
func TestOnsetsAt_MatchesEpisodeRule(t *testing.T) {
	var closes []float64
	c, seed := 100.0, uint32(7)
	for i := 0; i < 260; i++ {
		seed = seed*1664525 + 1013904223
		c *= 1 + (float64(seed%2001)/1000-1)*0.03
		closes = append(closes, c)
	}
	bars := onsetBars(closes)
	cfg := DefaultConfig()
	snaps := make([]Snapshot, len(bars))
	for i := range bars {
		snaps[i] = ComputeAt(bars, i, cfg, 0, false)
	}
	rule := func(i int, ok, fires func(Snapshot) bool) bool {
		if i < OnsetGapSessions || !fires(snaps[i]) {
			return false
		}
		for j := i - OnsetGapSessions; j < i; j++ {
			if !ok(snaps[j]) || fires(snaps[j]) {
				return false
			}
		}
		return true
	}
	counts := map[string]int{}
	for i := range bars {
		o := OnsetsAt(bars, i, cfg)
		want := map[string][2]bool{
			"rsi_overbought": {o.RSIOverbought, rule(i, func(s Snapshot) bool { return s.RSIOK }, func(s Snapshot) bool { return s.Fires(RSIOverbought) })},
			"rsi_oversold":   {o.RSIOversold, rule(i, func(s Snapshot) bool { return s.RSIOK }, func(s Snapshot) bool { return s.Fires(RSIOversold) })},
			"bb_squeeze":     {o.BBSqueezeOnset, rule(i, func(s Snapshot) bool { return s.SqueezeOK }, func(s Snapshot) bool { return s.Fires(BBSqueeze) })},
			"sweep":          {o.SweepOnset, rule(i, func(Snapshot) bool { return true }, sweepOnBar)},
		}
		for k, v := range want {
			if v[0] != v[1] {
				t.Fatalf("bar %d %s onset = %v, episode rule says %v", i, k, v[0], v[1])
			}
			if v[0] {
				counts[k]++
			}
		}
		if o.SweepOnset != (o.Sweep != nil) {
			t.Fatalf("bar %d: sweep onset %v but sweep %v", i, o.SweepOnset, o.Sweep)
		}
		if o.BarDate != bars[i].TS.Format(time.DateOnly) {
			t.Fatalf("bar %d date %s", i, o.BarDate)
		}
	}
	if counts["sweep"] == 0 || counts["rsi_overbought"]+counts["rsi_oversold"] == 0 {
		t.Errorf("fixture exercises too little: %v", counts)
	}
}

func TestOnsetsAt_NotJudgedWithoutHistory(t *testing.T) {
	bars := onsetBars([]float64{1, 2, 3})
	o := OnsetsAt(bars, 2, DefaultConfig())
	if o.Judged || o.Count() != 0 {
		t.Errorf("3 bars: %+v, want not judged, no onsets", o)
	}
}
