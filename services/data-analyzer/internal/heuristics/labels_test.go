package heuristics

import (
	"math"
	"testing"
	"time"

	"github.com/konsbe/trading-agent/services/data-analyzer/internal/compute"
)

func closesBars(closes []float64) []compute.Bar {
	t0 := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	out := make([]compute.Bar, len(closes))
	for i, c := range closes {
		out[i] = compute.Bar{TS: t0.AddDate(0, 0, i), Open: c, High: c, Low: c, Close: c, Volume: 1}
	}
	return out
}

func approx(t *testing.T, name string, got *float64, want float64) {
	t.Helper()
	if got == nil {
		t.Fatalf("%s is nil, want %v", name, want)
	}
	if math.Abs(*got-want) > 1e-9 {
		t.Errorf("%s = %v, want %v", name, *got, want)
	}
}

func TestForwardLabels_HandComputed(t *testing.T) {
	// close[t] = 100 at i=0, then 21 closes. Chosen so every window's answer
	// differs from its neighbour's:
	//   i:      1   2   3   4   5    6 ..  9  10   11 .. 19  20
	//   close: 101 97  104 99  102  100..100 90  100..100 130
	closes := []float64{100, 101, 97, 104, 99, 102}
	for i := 6; i <= 9; i++ {
		closes = append(closes, 100)
	}
	closes = append(closes, 90)
	for i := 11; i <= 19; i++ {
		closes = append(closes, 100)
	}
	closes = append(closes, 130)
	l := LabelsFor(closesBars(closes), 0, false)

	approx(t, "fwd_return_5s", l.FwdReturn[0], 2)      // 102/100
	approx(t, "fwd_return_10s", l.FwdReturn[1], -10)   // 90/100
	approx(t, "fwd_return_20s", l.FwdReturn[2], 30)    // 130/100
	approx(t, "fwd_abs_move_5s", l.FwdAbsMove[0], 4)   // max |.| over 1..5 = 104
	approx(t, "fwd_abs_move_10s", l.FwdAbsMove[1], 10) // the 90 at i=10 is inside
	approx(t, "fwd_abs_move_20s", l.FwdAbsMove[2], 30)
	if l.Complete == nil || !*l.Complete {
		t.Error("exactly 20 sessions after t must be label_complete")
	}

	// 19 sessions after t: 20s labels unknown, incomplete.
	l = LabelsFor(closesBars(closes[:20]), 0, false)
	if l.FwdReturn[2] != nil || l.FwdAbsMove[2] != nil {
		t.Error("20s labels must be nil with only 19 forward sessions")
	}
	approx(t, "fwd_return_10s (short)", l.FwdReturn[1], -10)
	if l.Complete == nil || *l.Complete {
		t.Error("19 forward sessions must be label_complete = false")
	}
}

func testLockbox() Lockbox {
	return Lockbox{
		Start: time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC),
		End:   time.Date(2024, 3, 31, 0, 0, 0, 0, time.UTC),
		Pilot: map[string]bool{"PILOT": true},
	}
}

// The lockbox boundary with its 20-session label purge. Bars are daily from
// 2024-01-01, so 2024-03-01 is index 60 and 2024-03-31 is index 90.
func TestLockboxContains_RegionAndPurge(t *testing.T) {
	bars := closesBars(make([]float64, 200))
	lb := testLockbox()
	for i, want := range map[int]bool{
		39: false, // label window ends at 59 = 2024-02-29
		40: true,  // label window reaches 60 = 2024-03-01 (purged)
		60: true,
		90: true,  // 2024-03-31
		91: false, // after the window; labels only read later bars
	} {
		if got := lb.Contains("OTHER", bars, i); got != want {
			t.Errorf("i=%d (%s) lockbox = %v, want %v", i, bars[i].TS.Format("2006-01-02"), got, want)
		}
		if lb.Contains("PILOT", bars, i) {
			t.Errorf("pilot-cohort symbol is never lockbox (i=%d)", i)
		}
	}
	// Near the end of the series the purge window is whatever sessions exist:
	// a series ending at index 45 cannot reach the region from index 30.
	if lb.Contains("OTHER", bars[:46], 30) {
		t.Error("a label window cut short by the end of the series must not reach the region")
	}
}

// Lockbox rows carry signal info but every label column is NULL — including
// label_complete, which is derived from what lies after t.
func TestLockboxEpisodesHaveNullLabels(t *testing.T) {
	bars := walk(700) // 2018-01-02 .. 2019-12-02
	lb := Lockbox{
		Start: time.Date(2018, 11, 1, 0, 0, 0, 0, time.UTC),
		End:   time.Date(2019, 2, 28, 0, 0, 0, 0, time.UTC),
		Pilot: map[string]bool{},
	}
	rc := DefaultReplayConfig()
	res := ReplaySymbol("OTHER", bars, VIXSeries{}, lb, rc)

	inLB, outLB := 0, 0
	check := func(where string, r Row) {
		if r.InLockbox {
			inLB++
			for k := range LabelHorizons {
				if r.Labels.FwdReturn[k] != nil || r.Labels.FwdAbsMove[k] != nil {
					t.Fatalf("%s %s is lockbox but has a label", where, r.T.Format("2006-01-02"))
				}
			}
			if r.Labels.Complete != nil {
				t.Fatalf("%s %s is lockbox but has label_complete", where, r.T.Format("2006-01-02"))
			}
			if !r.Snap.OK {
				t.Fatalf("%s lockbox row lost its signal info", where)
			}
		} else {
			outLB++
			if r.Labels.Complete == nil {
				t.Fatalf("%s %s is not lockbox but has no label_complete", where, r.T.Format("2006-01-02"))
			}
			// walk() is one bar per calendar day, so 20 sessions = 20 days.
			if !r.T.After(lb.End) && !r.T.AddDate(0, 0, 20).Before(lb.Start) {
				t.Fatalf("%s %s is within 20 sessions of the region but not purged", where, r.T.Format("2006-01-02"))
			}
		}
	}
	for sig, rows := range res.Episodes {
		for _, r := range rows {
			check(string(sig), r)
		}
	}
	for _, r := range res.Comparison {
		check("comparison", r)
	}
	if inLB == 0 || outLB == 0 {
		t.Fatalf("fixture must produce rows both in (%d) and out of (%d) the lockbox", inLB, outLB)
	}

	// The same symbol in the pilot cohort is never lockbox, so labels exist.
	lb.Pilot["OTHER"] = true
	for _, r := range ReplaySymbol("OTHER", bars, VIXSeries{}, lb, rc).Comparison {
		if r.InLockbox || r.Labels.Complete == nil {
			t.Fatalf("pilot symbol row %s must be labelled", r.T.Format("2006-01-02"))
		}
	}
}

func TestEpisodeGap(t *testing.T) {
	const first, gap = 251, 5
	cases := []struct {
		i, prev int
		seen    bool
		want    bool
	}{
		{251, 0, false, false}, // first evaluated day cannot vouch for the 5 before it
		{255, 0, false, false},
		{256, 0, false, true},  // 251..255 evaluated, none fired
		{300, 294, true, true}, // 295..299 quiet: 5 sessions
		{300, 295, true, false},
		{300, 299, true, false}, // consecutive firing continues the episode
	}
	for _, c := range cases {
		if got := isEpisodeStart(c.i, c.prev, c.seen, first, gap); got != c.want {
			t.Errorf("isEpisodeStart(i=%d, prev=%d, seen=%v) = %v, want %v", c.i, c.prev, c.seen, got, c.want)
		}
	}
}

// Episodes are per signal type and respect the gap on real replay output.
func TestReplayEpisodesRespectGapAndComparisonSampling(t *testing.T) {
	bars := walk(700)
	rc := DefaultReplayConfig()
	res := ReplaySymbol("X", bars, VIXSeries{}, Lockbox{}, rc)
	idx := map[time.Time]int{}
	for i, b := range bars {
		idx[sessionDate(b.TS)] = i
	}
	total := 0
	for sig, rows := range res.Episodes {
		for k, r := range rows {
			total++
			if !r.Snap.Fires(sig) {
				t.Fatalf("%s episode on %s does not fire", sig, r.T)
			}
			if idx[r.T] < rc.MinBars-1 {
				t.Fatalf("%s episode before the evaluated range", sig)
			}
			if k > 0 && idx[r.T]-idx[rows[k-1].T] <= rc.GapSessions {
				t.Fatalf("%s episodes %s and %s are within the gap", sig, rows[k-1].T, r.T)
			}
		}
	}
	if total == 0 {
		t.Fatal("no episodes on the fixture")
	}
	for _, r := range res.Comparison {
		if i := idx[r.T]; i%rc.ComparisonEvery != rc.ComparisonOffset || i < rc.MinBars-1 {
			t.Fatalf("comparison row at session %d is outside the documented sample", i)
		}
		if len(r.Fired) != len(Signals) {
			t.Fatalf("comparison row has %d fired flags, want %d", len(r.Fired), len(Signals))
		}
		for sig, f := range r.Fired {
			if f != r.Snap.Fires(sig) {
				t.Fatalf("comparison fired[%s] disagrees with the snapshot", sig)
			}
		}
	}
	if want := (700 - 251 + 4) / 5; len(res.Comparison) < want-1 || len(res.Comparison) > want+1 {
		t.Errorf("comparison rows = %d, want ~%d (every 5th evaluated session)", len(res.Comparison), want)
	}
}
