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
