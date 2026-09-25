// Package severity is the §2.3 scale (docs/MOMENTUM_SCANNER_FULL_STOCK_ANALYSIS_API.md)
// for everything momentum-api flags: alert kinds, chart patterns and the
// liquidity-sweep action signal. One table, no per-call-site guessing.
//
//	info    worth knowing, not urgent
//	notice  worth a second look
//	warning actively flagged as elevated risk or urgency
//
// The alert-kind table is the same as services/analyst-bot/reports/alert_severity.py,
// which writes fired_alerts.severity; severity_test.go reads that file and fails
// if the two disagree or if either side has a kind the other lacks.
package severity

// Level is one step of the scale.
type Level string

const (
	Info    Level = "info"
	Notice  Level = "notice"
	Warning Level = "warning"
)

// Alert kinds the bot's scan emits (reports/builder.py scan_alerts).
const (
	RSIOversold    = "rsi_oversold"
	RSIOverbought  = "rsi_overbought"
	BBSqueeze      = "bb_squeeze"
	VIXElevated    = "vix_elevated"
	FATierFlip     = "fa_tier_flip"
	LiquiditySweep = "liquidity_sweep"
)

// AlertKinds is every kind the scan emits.
var AlertKinds = []string{RSIOversold, RSIOverbought, BBSqueeze, VIXElevated, FATierFlip, LiquiditySweep}

var byKind = map[string]Level{
	RSIOversold:    Notice,
	RSIOverbought:  Notice,
	BBSqueeze:      Info,
	VIXElevated:    Warning,
	LiquiditySweep: Notice,
}

// ForAlert returns an alert kind's severity. fa_tier_flip depends on its
// direction: a flip to weak is a warning, any other flip a notice. ok is false
// for a kind with no severity.
func ForAlert(kind, newTier string) (Level, bool) {
	if kind == FATierFlip {
		if newTier == "weak" {
			return Warning, true
		}
		return Notice, true
	}
	l, ok := byKind[kind]
	return l, ok
}

// BotVIXAlertThreshold is when the bot's scan fires vix_elevated
// (config.py bot_vix_alert_threshold / BOT_VIX_ALERT_THRESHOLD, default 25:
// vix > 25). It is not the vix_regime "elevated" band (> 20).
const BotVIXAlertThreshold = 25.0

// Chart patterns momentum-api lists under heuristic_signals.chart_patterns.
// Each is read from a stored technical_indicators payload.
const (
	HeadShoulders       = "head_shoulders"       // hs_pattern_sw*: hs_found, confirmed by hs_neckline_break
	InvHeadShoulders    = "inv_head_shoulders"   // hs_pattern_sw*: inv_hs_found, confirmed by inv_hs_neckline_break
	BullFlag            = "bull_flag"            // flag_pole*: bull_flag (detection is the signal)
	BearFlag            = "bear_flag"            // flag_pole*: bear_flag (detection is the signal)
	DoubleTop           = "double_top"           // chart_pattern_hints: double_top_candidate, never confirmed
	DoubleBottom        = "double_bottom"        // chart_pattern_hints: double_bottom_candidate, never confirmed
	AscendingTriangle   = "ascending_triangle"   // triangle_sw*: kind, confirmed by breakout up/down
	DescendingTriangle  = "descending_triangle"  // triangle_sw*
	SymmetricalTriangle = "symmetrical_triangle" // triangle_sw*
)

// ChartPatterns is every pattern name momentum-api can emit.
var ChartPatterns = []string{
	HeadShoulders, InvHeadShoulders, BullFlag, BearFlag,
	DoubleTop, DoubleBottom, AscendingTriangle, DescendingTriangle, SymmetricalTriangle,
}

type patternLevels struct{ confirmed, unconfirmed Level }

// §2.3: a confirmed chart pattern is a notice; a formed-but-unconfirmed one is
// only worth knowing.
var byPattern = map[string]patternLevels{
	HeadShoulders:       {Notice, Info},
	InvHeadShoulders:    {Notice, Info},
	BullFlag:            {Notice, Info},
	BearFlag:            {Notice, Info},
	DoubleTop:           {Notice, Info},
	DoubleBottom:        {Notice, Info},
	AscendingTriangle:   {Notice, Info},
	DescendingTriangle:  {Notice, Info},
	SymmetricalTriangle: {Notice, Info},
}

// ForPattern returns a chart pattern's severity; ok is false for an unmapped
// pattern.
func ForPattern(pattern string, confirmed bool) (Level, bool) {
	l, ok := byPattern[pattern]
	if !ok {
		return "", false
	}
	if confirmed {
		return l.confirmed, true
	}
	return l.unconfirmed, true
}

// MaxConfluence is the liquidity-sweep rule's ceiling (1 + closed back
// across + order block + trend).
const MaxConfluence = 4

// ForSweepAction is the action signal's severity: the liquidity_sweep alert's
// (notice), raised to warning for §2.3's "high confluence TRIM_WATCH" — the
// 4/4 composite (the same bar H10 pins in the heuristics pre-registration).
func ForSweepAction(action string, confluence int) Level {
	if action == "TRIM_WATCH" && confluence >= MaxConfluence {
		return Warning
	}
	l, _ := ForAlert(LiquiditySweep, "")
	return l
}
