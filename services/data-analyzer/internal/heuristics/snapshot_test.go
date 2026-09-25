package heuristics

import (
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/konsbe/trading-agent/services/data-analyzer/internal/compute"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/technical"
)

// walk is a deterministic regime-switching random walk that produces every
// structure the snapshot reads: sweeps, order blocks, flags, H&S pivots.
func walk(n int) []compute.Bar {
	bars := make([]compute.Bar, n)
	seed := uint64(0x2545F4914F6CDD1D)
	rnd := func() float64 {
		seed ^= seed << 13
		seed ^= seed >> 7
		seed ^= seed << 17
		return float64(seed%1_000_000)/1_000_000*2 - 1
	}
	price := 50.0
	t0 := time.Date(2018, 1, 2, 0, 0, 0, 0, time.UTC)
	for i := 0; i < n; i++ {
		vol := 0.008 + 0.025*math.Abs(math.Sin(float64(i)/41))
		drift := 0.003 * math.Sin(float64(i)/67)
		o := price
		c := price * (1 + drift + vol*rnd())
		h := math.Max(o, c) * (1 + vol*math.Abs(rnd()))
		l := math.Min(o, c) * (1 - vol*math.Abs(rnd()))
		bars[i] = compute.Bar{TS: t0.AddDate(0, 0, i), Open: o, High: h, Low: l, Close: c, Volume: 1e5}
		price = c
	}
	return bars
}

// THE no-lookahead test (§5 step 2, mirroring momentum's
// TestNoLookahead_ComputeAtMatchesTruncatedSeries): the snapshot at i computed
// from the full series must equal the one computed from the series truncated
// at i. Order-block invalidation and the sweep scan read to the end of the
// slice they receive, so the only thing protecting them is the window ending at
// i — this test is what pins that.
func TestNoLookahead_ComputeAtMatchesTruncatedSeries(t *testing.T) {
	full := walk(700)
	cfg := DefaultConfig()
	seen := map[string]int{}
	for i := range full {
		a := ComputeAt(full, i, cfg, 18.5, true)
		b := ComputeAt(full[:i+1], i, cfg, 18.5, true)
		if !reflect.DeepEqual(a, b) {
			t.Fatalf("lookahead at i=%d:\n full      %+v\n truncated %+v", i, a, b)
		}
		if a.LastBullishOB {
			seen["bull_ob"]++
		}
		if a.LastBearishOB {
			seen["bear_ob"]++
		}
		if len(a.SweepsOnBar) > 0 {
			seen["sweep_on_bar"]++
		}
		if a.SweepRule != nil {
			seen["sweep_rule"]++
		}
		if a.RSIOK && a.MACDOK && a.SqueezeOK && a.TrendOK && a.ATROK {
			seen["all_numeric"]++
		}
		if a.BullFlag || a.BearFlag {
			seen["flag"]++
		}
		if a.HSFound || a.InvHSFound {
			seen["hs"]++
		}
		if a.BarsAvailable == cfg.Lookback {
			seen["full_window"]++
		}
	}
	for _, k := range []string{"bull_ob", "bear_ob", "sweep_on_bar", "sweep_rule", "all_numeric", "flag", "hs", "full_window"} {
		if seen[k] == 0 {
			t.Errorf("fixture never exercises %s; the no-lookahead test proves nothing for it", k)
		}
	}
}

// The windowing is load-bearing, not incidental: on the same fixture, running
// the order-block detector on a slice that extends past i changes the answer
// for some i. If this ever stops failing without the window, the no-lookahead
// test above has lost its teeth.
func TestOrderBlocksWouldLeakWithoutTheWindow(t *testing.T) {
	full := walk(700)
	p := DefaultConfig().Params
	leaks := 0
	for i := 200; i < 650; i++ {
		lo := i + 1 - DefaultConfig().Lookback
		if lo < 0 {
			lo = 0
		}
		windowed := technical.DetectOrderBlocks(full[lo:i+1], p)
		// Same start, 50 future bars: invalidation scans see them.
		leaky := technical.DetectOrderBlocks(full[lo:i+51], p)
		if (windowed.LastBullish != nil) != (leaky.LastBullish != nil) || (windowed.LastBearish != nil) != (leaky.LastBearish != nil) {
			leaks++
		}
	}
	if leaks == 0 {
		t.Fatal("future bars never changed order-block presence; fixture too weak to prove the window matters")
	}
}

func TestNoLookahead_AppendingFutureBarsChangesNothing(t *testing.T) {
	base := walk(650)
	ext := append(append([]compute.Bar{}, base...), walk(80)...)
	for _, i := range []int{100, 300, 499, 500, 600, 649} {
		if a, b := ComputeAt(base, i, DefaultConfig(), 25, true), ComputeAt(ext, i, DefaultConfig(), 25, true); !reflect.DeepEqual(a, b) {
			t.Errorf("snapshot at i=%d changed after appending future bars", i)
		}
	}
}

func TestComputeAtUsesTheWorkerWindow(t *testing.T) {
	bars := walk(700)
	if s := ComputeAt(bars, 699, DefaultConfig(), 0, false); s.BarsAvailable != 500 {
		t.Errorf("bars_available = %d, want the 500-bar TECHNICAL_COMPUTE_LOOKBACK window", s.BarsAvailable)
	}
	if s := ComputeAt(bars, 99, DefaultConfig(), 0, false); s.BarsAvailable != 100 {
		t.Errorf("bars_available = %d at i=99, want 100", s.BarsAvailable)
	}
	for _, i := range []int{-1, 700} {
		if s := ComputeAt(bars, i, DefaultConfig(), 0, false); s.OK {
			t.Errorf("ComputeAt(i=%d) must reject an out-of-range index", i)
		}
	}
}

func TestVIXRegimeUsesBotThresholds(t *testing.T) {
	bars := walk(300)
	for vix, want := range map[float64]string{35.01: "extreme_fear", 35: "elevated", 20.01: "elevated", 20: "normal", 12: "normal", 11.99: "complacency"} {
		if got := ComputeAt(bars, 299, DefaultConfig(), vix, true).VIXRegime; got != want {
			t.Errorf("vix %.2f → %q, want %q", vix, got, want)
		}
	}
	if got := ComputeAt(bars, 299, DefaultConfig(), 0, false).VIXRegime; got != "" {
		t.Errorf("no VIX observation must leave the regime empty (bot: macro has no vix_regime), got %q", got)
	}
}

func TestVIXSeriesIsPointInTime(t *testing.T) {
	d := func(s string) time.Time { v, _ := time.Parse("2006-01-02", s); return v }
	v := VIXSeries{Dates: []time.Time{d("2020-01-02"), d("2020-01-03"), d("2020-01-07")}, Values: []float64{10, 20, 30}}
	cases := []struct {
		ts   time.Time
		want float64
		ok   bool
	}{
		{d("2020-01-01"), 0, false},
		{d("2020-01-02"), 10, true},
		{d("2020-01-03").Add(13*time.Hour + 30*time.Minute), 20, true}, // yahoo-stamped bar, same session
		{d("2020-01-06"), 20, true},
		{d("2020-01-07"), 30, true},
		{d("2021-01-01"), 30, true},
	}
	for _, c := range cases {
		got, ok := v.At(c.ts)
		if ok != c.ok || got != c.want {
			t.Errorf("At(%s) = %v,%v want %v,%v", c.ts, got, ok, c.want, c.ok)
		}
	}
}
