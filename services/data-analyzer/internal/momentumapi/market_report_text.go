package momentumapi

import (
	"encoding/json"
	"fmt"
	"os"
)

// The Daily Market Report's one-line descriptions are stored with the macro
// rows, written for the Discord report, and several tell the reader what to
// do ("Reduce risk / hedge", "tighten stops") or assert outcomes the rule does
// not test ("classic late-cycle / hard-landing pipeline"). The web gets a
// description of what the code matched instead, keyed by the stored code.
// Stored payloads and the bot's Discord output keep the original text; an
// unknown code keeps its stored text. TestMarketReportTextCoversEveryCode
// fails when a producer gains a code the shared file does not describe.

// MarketReportText is shared/content/market_report_descriptions.json, the
// single source of these lines: analyst-bot reads the same file for Discord.
type MarketReportText struct {
	MarketCycle           map[string]string            `json:"market_cycle"`           // composite_phase → composite_label
	MacroRegime           map[string]string            `json:"macro_regime"`           // regime → label
	Intermarket           map[string]map[string]string `json:"intermarket"`            // pair → regime → label
	SeasonalityDisclaimer string                       `json:"seasonality_disclaimer"` // replaces the stored disclaimer
}

// LoadMarketReportText reads the shared descriptions. Like the caveats there is
// no fallback copy: a missing or incomplete file stops momentum-api at start.
func LoadMarketReportText(path string) (MarketReportText, error) {
	var t MarketReportText
	b, err := os.ReadFile(path)
	if err != nil {
		return t, fmt.Errorf("market report descriptions: %w", err)
	}
	if err := json.Unmarshal(b, &t); err != nil {
		return t, fmt.Errorf("market report descriptions %s: %w", path, err)
	}
	if len(t.MarketCycle) == 0 || len(t.MacroRegime) == 0 || len(t.Intermarket) == 0 || t.SeasonalityDisclaimer == "" {
		return t, fmt.Errorf("market report descriptions %s: a section is missing or empty", path)
	}
	return t, nil
}

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
func rewriteIntermarket(raw json.RawMessage, intermarketText map[string]map[string]string) json.RawMessage {
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
func rewriteSeasonality(raw json.RawMessage, disclaimer string) json.RawMessage {
	var m map[string]any
	if json.Unmarshal(raw, &m) != nil || m == nil {
		return raw
	}
	if _, has := m["disclaimer"]; !has || disclaimer == "" {
		return raw
	}
	m["disclaimer"] = disclaimer
	out, err := json.Marshal(m)
	if err != nil {
		return raw
	}
	return out
}
