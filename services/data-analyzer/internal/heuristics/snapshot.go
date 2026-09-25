// Package heuristics is the point-in-time replay of the live classical-TA
// signals (docs/HEURISTIC_SIGNALS_PREREGISTRATION.md §5 step 2).
//
// ComputeAt(bars, i) answers "what would the technical-analysis worker and the
// analyst-bot liquidity-sweep rule have said at the close of bar i", using the
// same internal/technical functions the worker calls, on the same trailing
// window the worker loads (TECHNICAL_COMPUTE_LOOKBACK bars ending at i).
package heuristics

import (
	"time"

	"github.com/konsbe/trading-agent/services/data-analyzer/internal/compute"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/technical"
)

// Config is the worker window plus indicator settings.
type Config struct {
	Lookback int // TECHNICAL_COMPUTE_LOOKBACK
	Params   technical.Params
}

// DefaultConfig is the live worker as configured in the repo .env (which
// differs from the code defaults in one place: TECHNICAL_HS_TOLERANCE_PCT is
// 5.0 in .env, 15.0 in internal/config).
func DefaultConfig() Config {
	return Config{
		Lookback: 500,
		Params: technical.Params{
			RSIPeriod:              14,
			MACDFast:               12,
			MACDSlow:               26,
			MACDSignal:             9,
			BBPeriod:               20,
			BBStd:                  2,
			KeltnerEMAPeriod:       20,
			KeltnerATRPeriod:       10,
			KeltnerMult:            2,
			TrendLookback:          60,
			ATRPeriod:              14,
			OBSwingStrength:        3,
			OBImpulseMinPct:        1.5,
			OBLookback:             100,
			LiquiditySwingStrength: 3,
			LiquidityLookback:      50,
			HSSwingStrength:        5,
			HSTolerancePct:         5.0,
			HSLookback:             100,
			FlagPolePct:            5.0,
			FlagMaxRetracePct:      50.0,
			FlagPoleLen:            5,
			FlagLen:                10,
		},
	}
}

// Sweep is one liquidity sweep as the worker publishes it (last_sweep).
type Sweep struct {
	Kind       string
	BarClose   float64
	SweptLevel float64
	OnBar      bool // the sweep happened on bar i itself
}

// Snapshot is every signal input at the close of one bar. Value fields are
// meaningful only when their OK flag is set, mirroring the worker, which skips
// an indicator it cannot compute rather than storing a number.
type Snapshot struct {
	OK            bool // i was in range
	BarsAvailable int  // window length actually used
	TS            time.Time
	Close         float64

	RSIOK bool
	RSI   float64

	MACDOK        bool
	MACDHist      float64
	MACDBullCross bool // bullish_cross_line_signal
	MACDBearCross bool // bearish_cross_line_signal

	SqueezeOK bool // both BB and Keltner computable
	Squeeze   bool

	TrendOK  bool
	TrendDir string // "up" | "down" | "sideways"

	BullFlag bool
	BearFlag bool

	HSFound            bool
	HSNecklineBreak    bool
	InvHSFound         bool
	InvHSNecklineBreak bool

	TotalSweeps int
	LastSweep   *Sweep
	// SweepsOnBar are the sweeps whose bar is i, in detection order.
	SweepsOnBar []Sweep

	LastBullishOB bool
	LastBearishOB bool

	ATROK bool
	ATR   float64

	VIXOK     bool
	VIX       float64
	VIXRegime string // "" when no VIX observation

	// SweepRule is the analyst-bot rule output. It exists only when the live
	// liquidity_sweep alert would fire, i.e. total_sweeps > 0.
	SweepRule *SweepRuleResult
}

// Window returns the slice the worker would have loaded at the close of bar i.
func Window(bars []compute.Bar, i int, lookback int) []compute.Bar {
	lo := 0
	if lookback > 0 && i+1-lookback > lo {
		lo = i + 1 - lookback
	}
	return bars[lo : i+1]
}

// ComputeAt evaluates the signals at bar i. vix/vixOK is the latest VIXCLS
// observation dated on or before bar i (vixOK false when none exists).
func ComputeAt(bars []compute.Bar, i int, cfg Config, vix float64, vixOK bool) Snapshot {
	if i < 0 || i >= len(bars) {
		return Snapshot{}
	}
	// Nothing below may read bars past i: order-block invalidation and FVG
	// fill scans run to the end of the slice they are given.
	w := Window(bars, i, cfg.Lookback)
	p := cfg.Params
	n := len(w)
	closes := compute.Closes(w)
	highs := compute.Highs(w)
	lows := compute.Lows(w)

	s := Snapshot{OK: true, BarsAvailable: n, TS: w[n-1].TS, Close: w[n-1].Close}

	if v, ok := technical.RSI(closes, p); ok {
		s.RSIOK, s.RSI = true, v
	}
	if m, ok := technical.MACD(closes, p); ok {
		s.MACDOK = true
		s.MACDHist = m.Cur.Hist
		s.MACDBullCross = m.BullishCross
		s.MACDBearCross = m.BearishCross
	}
	bb, bbOK := technical.Bollinger(closes, p)
	kc, kcOK := technical.KeltnerChannel(highs, lows, closes, p)
	if bbOK && kcOK {
		s.SqueezeOK = true
		s.Squeeze = technical.Squeeze(bb, kc)
	}
	if t, ok := technical.Trend(closes, highs, lows, p); ok {
		s.TrendOK, s.TrendDir = true, t.Direction
	}
	fl := technical.Flag(w, p)
	s.BullFlag, s.BearFlag = fl.BullFlag, fl.BearFlag

	hs := technical.HeadAndShoulders(w, p)
	s.HSFound, s.HSNecklineBreak = hs.HSFound, hs.HSNecklineBreak
	s.InvHSFound, s.InvHSNecklineBreak = hs.InvHSFound, hs.InvHSNecklineBreak

	sw := technical.DetectSweeps(w, p)
	s.TotalSweeps = len(sw.All)
	for _, x := range sw.All {
		if x.BarIndex == n-1 {
			s.SweepsOnBar = append(s.SweepsOnBar, Sweep{Kind: string(x.Kind), BarClose: x.BarClose, SweptLevel: x.SweptLevel, OnBar: true})
		}
	}
	if last := sw.Last(); last != nil {
		s.LastSweep = &Sweep{Kind: string(last.Kind), BarClose: last.BarClose, SweptLevel: last.SweptLevel, OnBar: last.BarIndex == n-1}
	}

	ob := technical.DetectOrderBlocks(w, p)
	s.LastBullishOB = ob.LastBullish != nil
	s.LastBearishOB = ob.LastBearish != nil

	if v, ok := technical.ATR(highs, lows, closes, p); ok {
		s.ATROK, s.ATR = true, v
	}

	if vixOK {
		s.VIXOK, s.VIX = true, vix
		s.VIXRegime = compute.ClassifyVIX(vix, BotVIXThresholds)
	}

	if s.TotalSweeps > 0 {
		r := EvaluateSweepRule(s.sweepRuleInput())
		s.SweepRule = &r
	}
	return s
}

// sweepRuleInput is what the bot reads from the stored payloads: last_sweep
// from liquidity_sweep_*, OB presence from order_blocks_*, trend direction
// (missing when the worker could not compute trend) and the VIX regime.
func (s Snapshot) sweepRuleInput() SweepRuleInput {
	in := SweepRuleInput{
		BullishOB: s.LastBullishOB,
		BearishOB: s.LastBearishOB,
		VIXRegime: s.VIXRegime,
	}
	if s.TrendOK {
		in.TrendDir = s.TrendDir
	}
	if s.LastSweep != nil {
		bc, sl := s.LastSweep.BarClose, s.LastSweep.SweptLevel
		in.Kind, in.BarClose, in.SweptLevel = s.LastSweep.Kind, &bc, &sl
	}
	return in
}
