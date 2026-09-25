package heuristics

// Signal is one pre-registered signal type. Each has its own episode table
// (§2: never pooled).
type Signal string

const (
	RSIOverbought   Signal = "rsi_overbought"    // H1: rsi14 > 70
	RSIOversold     Signal = "rsi_oversold"      // H2: rsi14 < 30
	MACDBullCross   Signal = "macd_bull_cross"   // H3: line crosses above signal
	MACDBearCross   Signal = "macd_bear_cross"   // H4: line crosses below signal
	BBSqueeze       Signal = "bb_squeeze"        // H5: BB inside Keltner
	BearishPattern  Signal = "bearish_pattern"   // H6: bear flag OR (H&S AND neckline break)
	BullishPattern  Signal = "bullish_pattern"   // H7: bull flag OR (inv H&S AND neckline break)
	LowSweepReclaim Signal = "low_sweep_reclaim" // H8a: low sweep on bar t, close > swept level
	HighSweepReject Signal = "high_sweep_reject" // H8b: high sweep on bar t, close < swept level
	BuyWatchC4      Signal = "buy_watch_c4"      // H9: sweep rule BUY_WATCH, confluence 4
	TrimWatchC4     Signal = "trim_watch_c4"     // H10: sweep rule TRIM_WATCH, confluence 4
)

// Signals is every signal type, in hypothesis order.
var Signals = []Signal{
	RSIOverbought, RSIOversold, MACDBullCross, MACDBearCross, BBSqueeze,
	BearishPattern, BullishPattern, LowSweepReclaim, HighSweepReject,
	BuyWatchC4, TrimWatchC4,
}

// EpisodeTable is the migration-027 table holding sig's episodes.
func (sig Signal) EpisodeTable() string { return "heuristic_ep_" + string(sig) }

// Fires reports whether sig fired at the snapshot's bar.
func (s Snapshot) Fires(sig Signal) bool {
	switch sig {
	case RSIOverbought:
		return s.RSIOK && s.RSI > 70
	case RSIOversold:
		return s.RSIOK && s.RSI < 30
	case MACDBullCross:
		return s.MACDOK && s.MACDBullCross
	case MACDBearCross:
		return s.MACDOK && s.MACDBearCross
	case BBSqueeze:
		return s.SqueezeOK && s.Squeeze
	case BearishPattern:
		return s.BearFlag || (s.HSFound && s.HSNecklineBreak)
	case BullishPattern:
		return s.BullFlag || (s.InvHSFound && s.InvHSNecklineBreak)
	case LowSweepReclaim:
		return s.sweepOnBar("low_sweep", func(x Sweep) bool { return x.BarClose > x.SweptLevel })
	case HighSweepReject:
		return s.sweepOnBar("high_sweep", func(x Sweep) bool { return x.BarClose < x.SweptLevel })
	case BuyWatchC4:
		return s.SweepRule != nil && s.SweepRule.Action == ActionBuyWatch && s.SweepRule.Confluence == 4
	case TrimWatchC4:
		return s.SweepRule != nil && s.SweepRule.Action == ActionTrimWatch && s.SweepRule.Confluence == 4
	}
	return false
}

func (s Snapshot) sweepOnBar(kind string, ok func(Sweep) bool) bool {
	for _, x := range s.SweepsOnBar {
		if x.Kind == kind && ok(x) {
			return true
		}
	}
	return false
}

// OnBarSweep returns the first sweep of kind on bar t, for episode context.
func (s Snapshot) OnBarSweep(kind string) *Sweep {
	for i := range s.SweepsOnBar {
		if s.SweepsOnBar[i].Kind == kind {
			return &s.SweepsOnBar[i]
		}
	}
	return nil
}
