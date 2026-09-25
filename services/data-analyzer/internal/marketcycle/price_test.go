package marketcycle

import (
	"testing"
	"time"

	"github.com/konsbe/trading-agent/services/data-analyzer/internal/store"
)

func series(n int, closeAt func(i int) float64) []store.EquityOHLCVBar {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	out := make([]store.EquityOHLCVBar, n)
	for i := range out {
		c := closeAt(i)
		out[i] = store.EquityOHLCVBar{TS: start.AddDate(0, 0, i), Open: c, High: c * 1.01, Low: c * 0.99, Close: c}
	}
	return out
}

var equityTh = Thresholds{
	PullbackPct: -0.03, CorrectionPct: -0.10, BearPct: -0.20,
	CrashVs10DHighPct: -0.12, CrashVs5BarPct: -0.10, BullExtendedSMAPct: 0.05,
	PeakLookback: 252, SMAPeriod: 200,
}

// Unset crash windows must behave exactly like the original hard-coded 10 / 5,
// so the SPY row the Discord report reads is unchanged by this parameterisation.
func TestAnalyzePrice_DefaultCrashWindowsAreTheOriginal10And5(t *testing.T) {
	// Flat, then an 11% drop over the last 6 bars: crosses -10% vs 5 bars ago.
	bars := series(300, func(i int) float64 {
		if i >= 294 {
			return 100 * (1 - 0.022*float64(i-293))
		}
		return 100
	})
	implicit := AnalyzePrice("X", bars, equityTh)
	explicit := equityTh
	explicit.CrashHighWindow, explicit.CrashCloseBars = 10, 5
	if got := AnalyzePrice("X", bars, explicit); got != implicit {
		t.Fatalf("explicit 10/5 = %+v\nimplicit = %+v; defaults must be identical", got, implicit)
	}
	if !implicit.CrashWarning || implicit.Phase != "crash" {
		t.Fatalf("an 11%% drop in 5 bars must flag a crash: %+v", implicit)
	}
}

// A slower slide that only crosses the threshold over 7 bars is a crash on the
// crypto (7-day) window but not the equity (5-session) one.
func TestAnalyzePrice_CryptoWindowsAreApplied(t *testing.T) {
	bars := series(400, func(i int) float64 {
		if i >= 393 {
			return 100 * (1 - 0.016*float64(i-392))
		}
		return 100
	})
	for i := range bars { // highs = closes, so only the close-to-close rule is in play
		bars[i].High = bars[i].Close
	}
	eq := AnalyzePrice("X", bars, equityTh)
	crypto := equityTh
	crypto.PeakLookback, crypto.CrashHighWindow, crypto.CrashCloseBars = 365, 14, 7
	cr := AnalyzePrice("X", bars, crypto)
	if eq.CrashWarning {
		t.Errorf("equity 5-bar window flagged a crash on a %.1f%% 5-bar move", (bars[399].Close/bars[394].Close-1)*100)
	}
	if !cr.CrashWarning {
		t.Errorf("crypto 7-bar window missed a %.1f%% 7-bar move", (bars[399].Close/bars[392].Close-1)*100)
	}
}

func TestAnalyzePrice_PeakLookbackBoundsThePeak(t *testing.T) {
	// A high 300 bars ago, then flat: inside a 365 window, outside 252.
	bars := series(400, func(i int) float64 {
		if i == 99 {
			return 150
		}
		return 100
	})
	short := AnalyzePrice("X", bars, equityTh)
	long := equityTh
	long.PeakLookback = 365
	if got := AnalyzePrice("X", bars, long); got.PeakHigh <= short.PeakHigh || got.DrawdownPct >= short.DrawdownPct {
		t.Fatalf("365-bar lookback should see the older peak: short %+v long %+v", short, got)
	}
}
