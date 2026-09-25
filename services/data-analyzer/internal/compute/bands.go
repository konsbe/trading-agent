package compute

// RSI and ADX bands. technical-analysis stores rsi_14 and adx_14 as plain
// numbers, and until now only the analyst bot classified them, so the bands
// are ported from the bot here — the one Go source momentum-api reads them
// from:
//
//   - RSI: services/analyst-bot/config.py bot_rsi_oversold / bot_rsi_overbought
//     (BOT_RSI_OVERSOLD / BOT_RSI_OVERBOUGHT, default 30 / 70), the thresholds
//     the alert scan fires rsi_oversold / rsi_overbought on
//     (reports/builder.py scan_alerts: value < oversold, value > overbought).
//     The Discord technical embed hardcodes the same 30 / 70.
//   - ADX: services/analyst-bot/actions/rules/bb_squeeze.py, the bot's only
//     ADX threshold: adx > 25 is "strong trend behind the squeeze". The bot has
//     no lower band, so neither does this.
//
// bands_test.go fails if the bot's defaults move.

// RSIThresholds are the bot's alert bands.
type RSIThresholds struct {
	Oversold   float64 // < this = oversold
	Overbought float64 // > this = overbought
}

// BotRSIThresholds are the bot's defaults (config.py, .env.example).
var BotRSIThresholds = RSIThresholds{Oversold: 30, Overbought: 70}

const (
	RSIOversold   = "oversold"
	RSIOverbought = "overbought"
	RSINormal     = "normal"
)

// RSIBands is every value ClassifyRSI returns.
var RSIBands = []string{RSIOversold, RSINormal, RSIOverbought}

// ClassifyRSI returns oversold, overbought or normal, with the bot's strict
// inequalities (30 and 70 themselves are normal).
func ClassifyRSI(rsi float64, th RSIThresholds) string {
	switch {
	case rsi < th.Oversold:
		return RSIOversold
	case rsi > th.Overbought:
		return RSIOverbought
	}
	return RSINormal
}

// ADXStrongTrend is the bot's threshold (bb_squeeze.py: adx_val > 25).
const ADXStrongTrend = 25.0

const (
	ADXStrong    = "strong_trend"
	ADXNotStrong = "not_strong_trend"
)

// ADXBands is every value ClassifyADX returns.
var ADXBands = []string{ADXNotStrong, ADXStrong}

// ClassifyADX returns strong_trend above 25, else not_strong_trend.
func ClassifyADX(adx float64) string {
	if adx > ADXStrongTrend {
		return ADXStrong
	}
	return ADXNotStrong
}
