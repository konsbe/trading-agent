package momentumapi

import "encoding/json"

// The Daily Market Report's one-line descriptions are stored with the macro
// rows, written for the Discord report, and several tell the reader what to
// do ("Reduce risk / hedge", "tighten stops") or assert outcomes the rule does
// not test ("classic late-cycle / hard-landing pipeline"). The web gets a
// description of what the code matched instead, keyed by the stored code.
// Stored payloads and the bot's Discord output keep the original text; an
// unknown code keeps its stored text. TestMarketReportTextCoversEveryCode
// fails when a producer gains a code these tables do not describe.

// cycleText: marketcycle.BuildComposite's Phase codes (composite_label).
var cycleText = map[string]string{
	"crash_panic":              "Price phase crash: SPY has fallen sharply against its recent highs. The macro stances do not change this reading.",
	"bear_structural":          "Price phase bear: SPY's drawdown from its one-year high is in the bear band. The macro stances do not change this reading.",
	"correction_risk":          "Price phase correction: SPY's drawdown from its one-year high is in the correction band.",
	"pullback_healthy":         "Price phase pullback: SPY is a little below its one-year high.",
	"late_cycle_stretched":     "Price phase bull extended, with the Inflation stance hot and Monetary Policy restrictive or neutral.",
	"bull_fragile_global":      "Price phase bull extended, with the Global stance elevated stress or moderate.",
	"bull_overextended":        "Price phase bull extended, with neither hot inflation under tight policy nor global stress.",
	"trend_soft":               "Price phase below the 200-day average, without a deep drawdown.",
	"neutral_mixed":            "Price phase bull: above the 200-day average and near the one-year high.",
	"bull_macro_aligned":       "Price phase bull, with the macro stances adding up above the aligned threshold.",
	"bull_macro_divergent":     "Price phase bull, with the macro stances adding up below the divergent threshold.",
	"insufficient_equity_data": "Fewer than 200 daily bars of SPY stored, so there is no reading.",
}

// regimeText: macrocorr.Build's Regime codes (label).
var regimeText = map[string]string{
	"recession_pipeline":            "Yield curve inverted or re-steepening, credit spreads elevated, and growth weak.",
	"stagflation_risk":              "The Inflation stance is hot while the Growth stance is slowdown or contraction.",
	"rising_inflation_tight_policy": "Inflation hot, Monetary Policy restrictive, and the yield curve flat, inverted or re-steepening.",
	"global_liquidity_stress":       "The Global stance is elevated stress, with a strong dollar or a yen carry unwind.",
	"deflation_risk":                "The Inflation stance is deflationary.",
	"goldilocks_light":              "Inflation moderate, policy not restrictive, growth in expansion and credit spreads contained.",
	"disinflation_soft_landing":     "Growth in expansion, with inflation not hot, policy not restrictive and credit spreads contained.",
	"neutral_mixed":                 "The stances do not match any of the listed patterns.",
}

// intermarketText: the additional package's regime codes per pair (label).
// Thresholds are the code's fixed cut-offs.
var intermarketText = map[string]map[string]string{
	"bond_equity_60d": {
		"deflationary_hedge":    "Bond prices and stocks moved in opposite directions over the window (ρ ≤ −0.25).",
		"inflationary_positive": "Bond prices and stocks moved together over the window (ρ ≥ 0.25).",
		"transition_neutral":    "No clear link between bond prices and stocks over the window.",
	},
	"oil_equity_60d": {
		"procyclical":   "Stocks and oil moved together over the window (ρ ≥ 0.25).",
		"decoupled":     "Stocks and oil moved in opposite directions over the window (ρ ≤ −0.25).",
		"neutral_mixed": "No clear link between stocks and oil over the window.",
	},
	"vix_equity_60d": {
		"typical_fear_greed": "Stocks rose as the VIX fell, and fell as it rose (ρ ≤ −0.25).",
		"unusual_positive":   "Stocks and the VIX moved in the same direction (ρ ≥ 0.15).",
		"compressed_link":    "Only a weak link between stock returns and VIX changes over the window.",
	},
}

const seasonalityDisclaimer = "A fixed reference table, not computed from market data; no reading on this page uses it."

// rewriteText replaces payload[textField] with table[payload[codeField]] when
// the code is known.
func rewriteText(raw json.RawMessage, codeField, textField string, table map[string]string) json.RawMessage {
	var m map[string]any
	if json.Unmarshal(raw, &m) != nil || m == nil {
		return raw
	}
	code, _ := m[codeField].(string)
	text, ok := table[code]
	if !ok {
		return raw
	}
	if _, has := m[textField]; !has {
		return raw
	}
	m[textField] = text
	out, err := json.Marshal(m)
	if err != nil {
		return raw
	}
	return out
}

// rewriteIntermarket rewrites each pair's label by its regime.
func rewriteIntermarket(raw json.RawMessage) json.RawMessage {
	var pairs map[string]json.RawMessage
	if json.Unmarshal(raw, &pairs) != nil || pairs == nil {
		return raw
	}
	for name, table := range intermarketText {
		if p, ok := pairs[name]; ok {
			pairs[name] = rewriteText(p, "regime", "label", table)
		}
	}
	out, err := json.Marshal(pairs)
	if err != nil {
		return raw
	}
	return out
}

// rewriteSeasonality replaces the stored disclaimer ("… do not trade in
// isolation") with a description of what the table is.
func rewriteSeasonality(raw json.RawMessage) json.RawMessage {
	var m map[string]any
	if json.Unmarshal(raw, &m) != nil || m == nil {
		return raw
	}
	if _, has := m["disclaimer"]; !has {
		return raw
	}
	m["disclaimer"] = seasonalityDisclaimer
	out, err := json.Marshal(m)
	if err != nil {
		return raw
	}
	return out
}
