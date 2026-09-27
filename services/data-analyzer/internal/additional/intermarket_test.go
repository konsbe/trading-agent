package additional

import (
	"math"
	"testing"
	"time"

	"github.com/konsbe/trading-agent/services/data-analyzer/internal/store"
)

func TestPearsonPerfectPositive(t *testing.T) {
	x := []float64{1, 2, 3, 4, 5}
	y := []float64{2, 4, 6, 8, 10}
	c, ok := pearson(x, y)
	if !ok || math.Abs(c-1.0) > 1e-6 {
		t.Fatalf("corr = %v ok=%v", c, ok)
	}
}

func TestComputePresidentialCycle(t *testing.T) {
	pc := ComputePresidentialCycle(time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC))
	if pc.CycleYear != 2 || pc.Label != "midterm" {
		t.Fatalf("got %+v", pc)
	}
}

func TestForwardFillDGS10(t *testing.T) {
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	fred := []store.MacroObs{
		{TS: base, Value: 4.0},
		{TS: base.Add(24 * time.Hour), Value: 4.1},
	}
	spy := []time.Time{base, base.Add(24 * time.Hour)}
	out := forwardFillFred(spy, fred)
	if len(out) != 2 || math.Abs(out[0]-4.0) > 1e-9 || math.Abs(out[1]-4.1) > 1e-9 {
		t.Fatalf("out = %v", out)
	}
}

// bondEquityFixture builds 70 sessions in which the 10Y yield moves `sign`
// times SPY's move: sign +1 is a flight to quality (stocks fall and yields
// fall with them, i.e. bond prices rise), sign −1 a rates shock (yields rise
// as stocks fall, so bond prices fall with stocks).
func bondEquityFixture(sign float64) ([]store.EquityOHLCVBar, []store.MacroObs) {
	var bars []store.EquityOHLCVBar
	var yields []store.MacroObs
	price, y := 500.0, 4.0
	start := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 70; i++ {
		move := 0.01 * math.Sin(float64(i)*1.7) // −1% … +1%
		price *= 1 + move
		y += sign * move * 5 // 1% on SPY ↔ 5bp on the yield
		ts := start.AddDate(0, 0, i)
		bars = append(bars, store.EquityOHLCVBar{TS: ts, Close: price})
		yields = append(yields, store.MacroObs{TS: ts, Value: y})
	}
	return bars, yields
}

// The correlation was taken against the raw yield change while the labels
// describe bond prices, so each regime read backwards: on 2026-09-27 ρ −0.528
// (stocks up as yields fell, i.e. with bond prices) read "deflationary hedge —
// classic flight-to-quality".
func TestBondEquityCorrelatesWithBondPricesNotYields(t *testing.T) {
	bars, yields := bondEquityFixture(+1)
	flight := ComputeBondEquity60d(bars, yields, 60, 40)
	if flight.InsufficientData || flight.Correlation60d > -0.9 || flight.Regime != "deflationary_hedge" {
		t.Errorf("flight to quality: ρ %.3f %s, want ≈ −1 deflationary_hedge (bonds rise as stocks fall)", flight.Correlation60d, flight.Regime)
	}
	bars, yields = bondEquityFixture(-1)
	shock := ComputeBondEquity60d(bars, yields, 60, 40)
	if shock.Correlation60d < 0.9 || shock.Regime != "inflationary_positive" {
		t.Errorf("rates shock: ρ %.3f %s, want ≈ +1 inflationary_positive (bonds fall with stocks)", shock.Correlation60d, shock.Regime)
	}
	// The raw-yield reading had the opposite sign.
	raw := ComputeRollCorrEquityVsFredDelta(bars, yields, 60, 40, "DGS10", regimeBondEquity, "")
	if raw.Correlation60d != -shock.Correlation60d {
		t.Errorf("raw-yield ρ %.3f, want the negation of %.3f", raw.Correlation60d, shock.Correlation60d)
	}
}
