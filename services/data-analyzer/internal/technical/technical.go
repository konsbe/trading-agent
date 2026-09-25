// Package technical holds the single implementation of the indicators that
// both the live technical-analysis worker and the heuristics replay read:
// RSI, MACD, Bollinger, Keltner, BB squeeze, trend, ATR, order blocks,
// liquidity sweeps, head & shoulders and flags.
//
// The worker builds its stored payloads from these results; the replay
// (internal/heuristics) evaluates the same functions on a historical window.
// Keeping one code path is what makes a replayed signal mean the same thing as
// a live one, so neither caller may re-derive these values on its own.
package technical

import (
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/compute"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/config"
)

// Params are the worker settings these indicators depend on.
type Params struct {
	RSIPeriod int

	MACDFast   int
	MACDSlow   int
	MACDSignal int

	BBPeriod int
	BBStd    float64

	KeltnerEMAPeriod int
	KeltnerATRPeriod int
	KeltnerMult      float64

	TrendLookback int
	ATRPeriod     int

	OBSwingStrength int
	OBImpulseMinPct float64
	OBLookback      int

	LiquiditySwingStrength int
	LiquidityLookback      int

	HSSwingStrength int
	HSTolerancePct  float64
	HSLookback      int

	FlagPolePct       float64
	FlagMaxRetracePct float64
	FlagPoleLen       int
	FlagLen           int
}

// ParamsFrom takes the indicator settings out of the worker config.
func ParamsFrom(c config.TechnicalAnalysis) Params {
	return Params{
		RSIPeriod:              c.RSIPeriod,
		MACDFast:               c.MACDFast,
		MACDSlow:               c.MACDSlow,
		MACDSignal:             c.MACDSignal,
		BBPeriod:               c.BBPeriod,
		BBStd:                  c.BBStd,
		KeltnerEMAPeriod:       c.KeltnerEMAPeriod,
		KeltnerATRPeriod:       c.KeltnerATRPeriod,
		KeltnerMult:            c.KeltnerMult,
		TrendLookback:          c.TrendLookback,
		ATRPeriod:              c.ATRPeriod,
		OBSwingStrength:        c.OBSwingStrength,
		OBImpulseMinPct:        c.OBImpulseMinPct,
		OBLookback:             c.OBLookback,
		LiquiditySwingStrength: c.LiquiditySwingStrength,
		LiquidityLookback:      c.LiquidityLookback,
		HSSwingStrength:        c.HSSwingStrength,
		HSTolerancePct:         c.HSTolerancePct,
		HSLookback:             c.HSLookback,
		FlagPolePct:            c.FlagPolePct,
		FlagMaxRetracePct:      c.FlagMaxRetracePct,
		FlagPoleLen:            c.FlagPoleLen,
		FlagLen:                c.FlagLen,
	}
}

func RSI(closes []float64, p Params) (float64, bool) {
	return compute.RSI(closes, p.RSIPeriod)
}

func MACD(closes []float64, p Params) (compute.MACDSnapshot, bool) {
	return compute.MACDSnapshotWithPrev(closes, p.MACDFast, p.MACDSlow, p.MACDSignal)
}

func Bollinger(closes []float64, p Params) (compute.BollingerResult, bool) {
	return compute.BollingerLast(closes, p.BBPeriod, p.BBStd)
}

// Keltner is the channel on the last bar.
type Keltner struct {
	Middle, Upper, Lower float64
}

func KeltnerChannel(highs, lows, closes []float64, p Params) (Keltner, bool) {
	mid, up, lo, ok := compute.KeltnerLast(highs, lows, closes, p.KeltnerEMAPeriod, p.KeltnerATRPeriod, p.KeltnerMult)
	return Keltner{Middle: mid, Upper: up, Lower: lo}, ok
}

// Squeeze is true when the Bollinger bands sit strictly inside the Keltner
// channel (low-volatility coil). Both inputs must come from the same bar.
func Squeeze(bb compute.BollingerResult, k Keltner) bool {
	return bb.Lower > k.Lower && bb.Upper < k.Upper
}

func Trend(closes, highs, lows []float64, p Params) (compute.TrendResult, bool) {
	return compute.AnalyzeTrend(closes, highs, lows, p.TrendLookback)
}

func ATR(highs, lows, closes []float64, p Params) (float64, bool) {
	return compute.ATRWilder(highs, lows, closes, p.ATRPeriod)
}

// OrderBlocks is the detector result plus the count of blocks not yet
// invalidated, which the worker stores as the indicator value.
type OrderBlocks struct {
	compute.OrderBlocksResult
	Active int
}

func DetectOrderBlocks(bars []compute.Bar, p Params) OrderBlocks {
	ob := compute.DetectOrderBlocks(bars, p.OBSwingStrength, p.OBImpulseMinPct, p.OBLookback)
	active := 0
	for _, o := range ob.All {
		if !o.Invalidated {
			active++
		}
	}
	return OrderBlocks{OrderBlocksResult: ob, Active: active}
}

// Sweeps are the liquidity sweeps inside the lookback, in detection order
// (by bar; a high sweep precedes a low sweep on the same bar).
type Sweeps struct {
	All  []compute.LiquiditySweep
	High int
	Low  int
}

// Last is the sweep the worker publishes as last_sweep, or nil.
func (s Sweeps) Last() *compute.LiquiditySweep {
	if len(s.All) == 0 {
		return nil
	}
	return &s.All[len(s.All)-1]
}

func DetectSweeps(bars []compute.Bar, p Params) Sweeps {
	all := compute.DetectLiquiditySweeps(bars, p.LiquiditySwingStrength, p.LiquidityLookback)
	s := Sweeps{All: all}
	for _, sv := range all {
		if sv.Kind == compute.SweepHigh {
			s.High++
		} else {
			s.Low++
		}
	}
	return s
}

func HeadAndShoulders(bars []compute.Bar, p Params) compute.HSPatternResult {
	return compute.DetectHSPattern(bars, p.HSSwingStrength, p.HSTolerancePct, p.HSLookback)
}

func Flag(bars []compute.Bar, p Params) compute.FlagResult {
	return compute.DetectFlag(bars, p.FlagPolePct, p.FlagMaxRetracePct, p.FlagPoleLen, p.FlagLen)
}
