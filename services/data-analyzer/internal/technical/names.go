package technical

import "fmt"

// Names are the stored technical_indicators names the analysis endpoint reads,
// built from the config exactly as Emitter and the runner name them. Several
// carry their parameters (macd_12_26_9, order_blocks_sw3_imp1.5), so a reader
// must not hardcode them.
type Names struct {
	RSI, MACD, ADX, ATR, Trend, MARibbon, BBSqueeze string
	FVG, OrderBlocks, LiquiditySweep                string
	HSPattern, Flag, Triangle, ChartPatternHints    string
	// Written by the runner, not Emitter.
	VIXRegime, PivotsPriorBar, PivotsWeekly string
}

func NamesFor(c Emitter) Names {
	cfg := c.Cfg
	return Names{
		RSI:               fmt.Sprintf("rsi_%d", cfg.RSIPeriod),
		MACD:              fmt.Sprintf("macd_%d_%d_%d", cfg.MACDFast, cfg.MACDSlow, cfg.MACDSignal),
		ADX:               fmt.Sprintf("adx_%d", cfg.ADXPeriod),
		ATR:               fmt.Sprintf("atr_%d", cfg.ATRPeriod),
		Trend:             "trend",
		MARibbon:          "ma_ribbon",
		BBSqueeze:         "bb_squeeze",
		FVG:               fmt.Sprintf("fvg_min%s_lb%d", formatFloatKey(cfg.FVGMinGapPct), cfg.FVGLookback),
		OrderBlocks:       fmt.Sprintf("order_blocks_sw%d_imp%s", cfg.OBSwingStrength, formatFloatKey(cfg.OBImpulseMinPct)),
		LiquiditySweep:    fmt.Sprintf("liquidity_sweep_sw%d", cfg.LiquiditySwingStrength),
		HSPattern:         fmt.Sprintf("hs_pattern_sw%d", cfg.HSSwingStrength),
		Flag:              fmt.Sprintf("flag_pole%s_len%d", formatFloatKey(cfg.FlagPolePct), cfg.FlagLen),
		Triangle:          fmt.Sprintf("triangle_sw%d", cfg.TriangleSwingStrength),
		ChartPatternHints: "chart_pattern_hints",
		VIXRegime:         "vix_regime",
		PivotsPriorBar:    "pivots_prior_bar",
		PivotsWeekly:      "pivots_weekly",
	}
}
