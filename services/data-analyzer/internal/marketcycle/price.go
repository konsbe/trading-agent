// Package marketcycle classifies equity index drawdown vs 200DMA and combines with macro stances.
package marketcycle

import (
	"math"
	"time"

	"github.com/konsbe/trading-agent/services/data-analyzer/internal/store"
)

// Thresholds tune classification vs macro_analysis_reference.html Market Cycles panel.
type Thresholds struct {
	PullbackPct       float64 // e.g. -0.03  (−3% from peak)
	CorrectionPct     float64 // e.g. -0.10
	BearPct           float64 // e.g. -0.20
	CrashVs10DHighPct float64 // e.g. -0.12 vs max high last 10 sessions
	CrashVs5BarPct    float64 // e.g. -0.10 vs close 5 bars ago
	BullExtendedSMAPct float64 // close >= SMA200 × (1+x) e.g. 0.05
	PeakLookback      int     // trading bars for peak (e.g. 252)
	SMAPeriod         int     // e.g. 200
}

// PriceResult is derived from daily OHLCV only.
type PriceResult struct {
	Symbol       string
	LastTS       time.Time
	Close        float64
	PeakHigh     float64
	PeakTS       time.Time
	DrawdownPct  float64 // negative fraction, e.g. -0.057 = −5.7%
	DaysOffPeak  int     // trading sessions since peak bar in lookback window
	SMA200       float64
	HasSMA200    bool
	PctVsSMA200  float64 // (close/sma-1)*100
	Phase        string // crash, bear, correction, pullback, bull_extended, bull, below_sma, insufficient_data
	CrashWarning bool
}

// AnalyzePrice computes drawdown, 200DMA distance, and price-only phase.
func AnalyzePrice(symbol string, bars []store.EquityOHLCVBar, t Thresholds) PriceResult {
	out := PriceResult{Symbol: symbol, Phase: "insufficient_data"}
	n := len(bars)
	if n == 0 {
		return out
	}
	last := bars[n-1]
	out.LastTS = last.TS
	out.Close = last.Close

	need := t.SMAPeriod
	if need <= 0 {
		need = 200
	}
	if n < need {
		return out
	}

	lb := t.PeakLookback
	if lb <= 0 {
		lb = 252
	}
	if lb > n {
		lb = n
	}
	start := n - lb
	peakHigh := bars[start].High
	peakTS := bars[start].TS
	peakIdx := start
	for i := start; i < n; i++ {
		if bars[i].High >= peakHigh {
			peakHigh = bars[i].High
			peakTS = bars[i].TS
			peakIdx = i
		}
	}
	out.PeakHigh = peakHigh
	out.PeakTS = peakTS
	out.DaysOffPeak = n - 1 - peakIdx
	if peakHigh > 0 {
		out.DrawdownPct = (last.Close - peakHigh) / peakHigh
	}

	var sum float64
	for i := n - need; i < n; i++ {
		sum += bars[i].Close
	}
	sma := sum / float64(need)
	out.SMA200 = sma
	out.HasSMA200 = true
	if sma > 0 {
		out.PctVsSMA200 = (last.Close/sma - 1) * 100
	}

	// Crash: violent short-term move (HTML "Market Crash" card)
	crash := false
	if n >= 10 {
		h10 := bars[n-10].High
		for i := n - 9; i < n; i++ {
			if bars[i].High > h10 {
				h10 = bars[i].High
			}
		}
		if h10 > 0 && (last.Close-h10)/h10 <= t.CrashVs10DHighPct {
			crash = true
		}
	}
	if n >= 6 {
		c5 := bars[n-6].Close
		if c5 > 0 && (last.Close-c5)/c5 <= t.CrashVs5BarPct {
			crash = true
		}
	}
	out.CrashWarning = crash

	switch {
	case crash:
		out.Phase = "crash"
	case out.DrawdownPct <= t.BearPct:
		out.Phase = "bear"
	case out.DrawdownPct <= t.CorrectionPct:
		out.Phase = "correction"
	case out.DrawdownPct <= t.PullbackPct:
		out.Phase = "pullback"
	case last.Close >= sma*(1.0+t.BullExtendedSMAPct):
		out.Phase = "bull_extended"
	case last.Close >= sma:
		out.Phase = "bull"
	default:
		out.Phase = "below_sma"
	}
	return out
}

// Round2 rounds for JSON payloads.
func Round2(x float64) float64 { return math.Round(x*100) / 100 }
