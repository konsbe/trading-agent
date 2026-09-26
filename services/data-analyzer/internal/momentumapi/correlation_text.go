package momentumapi

import (
	"fmt"
	"regexp"
	"strings"
)

// correlationSentences maps each aligned / divergent sentence fundamental
// analysis stores (its format string, as in internal/fundamental/analyze.go)
// to the text the web shows. The stored sentences state implications the
// comparison behind them does not test ("cascade to insolvency risk",
// "compounding machine"); the web text says which comparison was met. Stored
// payloads and the bot's Discord output keep the original sentences.
//
// A number the stored sentence carries (%.1f) is copied into the display text
// as rendered (%s, in the same order). Thresholds are not written out: they are
// configuration. TestCorrelationSentencesCoverAnalyzer fails when analyze.go
// gains, loses or edits a sentence this table does not match.
var correlationSentences = []struct{ stored, display string }{
	// Earnings quality
	{"EPS growing but FCF accrual concern — possible earnings inflation via non-cash accounting",
		"Fast EPS growth with a low FCF yield"},
	{"EPS growth backed by real FCF — high-quality earnings",
		"Fast EPS growth with a high FCF yield"},
	{"EPS rising but revenue falling — buybacks or cost cuts masking organic deterioration",
		"Strong EPS growth with weak revenue growth"},
	{"Revenue rising but EPS falling — cost structure breaking down, margins compressing at scale",
		"Strong revenue growth with weak EPS growth"},
	{"Revenue and EPS growing together — genuine organic quality growth",
		"Strong revenue growth and strong EPS growth"},
	{"Gross margin expanding but net margin compressing — SG&A, interest, or tax costs surging below gross line",
		"Gross margin trend expanding, net margin trend compressing"},
	{"Both margins compressing — broad profitability deterioration",
		"Gross and net margin trends both compressing"},
	{"Both margins expanding — improving operating leverage throughout the P&L",
		"Gross and net margin trends both expanding"},
	{"Fast revenue growth (%.1f%%) with compressing gross margin — scaling without pricing power",
		"Revenue growth of %s%% with a compressing gross margin trend"},
	{"Fast revenue growth (%.1f%%) with stable/expanding gross margin — quality growth combination",
		"Revenue growth of %s%% with a stable or expanding gross margin trend"},
	{"Revenue declining AND gross margin compressing — double deterioration signal",
		"Revenue growth below zero with a compressing gross margin trend"},

	// Valuation vs quality
	{"High P/E %.1f× + low EPS growth %.1f%% — valuation trap risk, paying premium for deteriorating earnings",
		"High P/E (%s×) with low EPS growth (%s%%)"},
	{"Low P/E %.1f× + strong EPS growth %.1f%% — potential undervaluation (check PEG)",
		"Low P/E (%s×) with strong EPS growth (%s%%)"},
	{"High P/E + low ROIC — unjustified valuation premium, investors paying for quality that doesn't exist",
		"High P/E with low ROIC"},
	{"Low P/E + high ROIC (moat quality) — rare value opportunity in a quality business",
		"Low P/E with high ROIC"},
	{"Dividend yield %.1f%% > FCF yield %.1f%% — dividend not covered by free cash flow, cut risk elevated",
		"Dividend yield (%s%%) above FCF yield (%s%%)"},
	{"FCF yield comfortably covers dividend — sustainable income with room for growth",
		"FCF yield at least 1.5× the dividend yield"},
	{"Low P/B + high sustained ROE — classic deep value signal (low price for high quality capital allocation)",
		"Low P/B with high ROE"},
	{"High P/B + declining/moderate ROE — multiple contraction risk, paying for quality that is eroding",
		"High P/B with ROE below its high band"},

	// Leverage & liquidity
	{"High Net Debt/EBITDA AND low interest coverage — accelerating financial distress, credit event risk in rising rate environment",
		"High net debt / operating income with low interest coverage"},
	{"One leverage/coverage metric at high-risk level — balance sheet vulnerable to rate rises or revenue shortfall",
		"Net debt / operating income or interest coverage in its high-risk band"},
	{"Conservative net debt + very safe interest coverage — highly resilient balance sheet",
		"Low net debt / operating income with high interest coverage"},
	{"Deteriorating current ratio + weak FCF conversion — cash burn accelerating, may need to raise capital or issue expensive debt",
		"Low current ratio with low FCF conversion"},
	{"Strong liquidity + high-quality cash earnings — resilient in economic stress scenarios",
		"High current ratio with high FCF conversion"},
	{"High D/E + thin net margin %.1f%% — any revenue shortfall can cascade to insolvency risk",
		"High debt / equity with a net margin of %s%%"},
	{"Low leverage + strong net margins — excellent financial resilience across market cycles",
		"Low debt / equity with a high net margin"},
	{"High goodwill exposure + weak FCF conversion — acquisitions generating reported earnings but not real cash, impairment risk",
		"Goodwill and intangibles a large share of assets, with low FCF conversion"},
	{"Low goodwill exposure + high FCF conversion — organic, cash-backed growth (not acquisition-dependent)",
		"Goodwill and intangibles a small share of assets, with high FCF conversion"},

	// Operational
	{"Strong revenue growth + low ROIC — growth is dilutive, company investing in sub-cost-of-capital projects",
		"Strong revenue growth with low ROIC"},
	{"High ROIC + strong revenue growth — compounding machine, every reinvested dollar earns above cost of capital",
		"High ROIC with strong revenue growth"},
	{"High ROIC + slowing revenue — mature harvest phase, strong returns on existing capital",
		"High ROIC with weak revenue growth"},
	{"Gross margin compressing — pricing power weakening or input costs rising faster than selling prices",
		"Gross margin trend compressing"},
	{"Gross margin expanding — pricing power intact, favourable demand/cost dynamics",
		"Gross margin trend expanding"},
	{"Low CapEx intensity + attractive FCF yield — asset-light model generating strong free cash for shareholders",
		"Low capital spending relative to revenue, with a high FCF yield"},
	{"High CapEx intensity + weak FCF yield — heavy reinvestment absorbing all cash, limited shareholder returns",
		"High capital spending relative to revenue, with a low FCF yield"},
}

type sentenceRule struct {
	match   *regexp.Regexp
	display string
}

var correlationSentenceRules = compileSentenceRules()

// compileSentenceRules turns each stored format string into an anchored
// pattern: %.1f captures the rendered number, %% is a literal percent sign.
func compileSentenceRules() []sentenceRule {
	rules := make([]sentenceRule, 0, len(correlationSentences))
	for _, s := range correlationSentences {
		pattern := regexp.QuoteMeta(s.stored)
		pattern = strings.ReplaceAll(pattern, `%\.1f`, `(-?[0-9]+(?:\.[0-9]+)?)`)
		pattern = strings.ReplaceAll(pattern, `%%`, `%`)
		rules = append(rules, sentenceRule{match: regexp.MustCompile(`^` + pattern + `$`), display: s.display})
	}
	return rules
}

// correlationDisplayText is the web text for one stored sentence. A sentence
// the table does not know is served as stored, so nothing is dropped.
func correlationDisplayText(stored string) string {
	for _, r := range correlationSentenceRules {
		m := r.match.FindStringSubmatch(stored)
		if m == nil {
			continue
		}
		args := make([]any, len(m)-1)
		for i, v := range m[1:] {
			args[i] = v
		}
		return fmt.Sprintf(r.display, args...)
	}
	return stored
}

func correlationDisplayTexts(stored []string) []string {
	out := make([]string, len(stored))
	for i, s := range stored {
		out[i] = correlationDisplayText(s)
	}
	return out
}
