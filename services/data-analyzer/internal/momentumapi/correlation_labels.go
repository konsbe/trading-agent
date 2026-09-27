package momentumapi

import (
	"encoding/json"
	"fmt"
	"os"
)

// CorrelationLabels is shared/content/correlation_labels.json: the correlation
// card's labels by stored code — cluster names, cluster tiers, the
// not-evaluated wording, pattern names, net-signal wording and headings.
// momentum-api serves them with the analysis (mfe-scanner renders them as
// served) and analyst-bot reads the same file for its Discord embed, so the
// two surfaces label the same reading the same way. Stored codes are unchanged.
type CorrelationLabels struct {
	Clusters     map[string]string `json:"clusters"`
	ClusterTiers map[string]string `json:"cluster_tiers"`
	NotEvaluated string            `json:"not_evaluated"`
	Patterns     map[string]string `json:"patterns"`
	NetSignal    map[string]string `json:"net_signal"`
	Text         struct {
		PatternsHeading string `json:"patterns_heading"`
		NetCount        string `json:"net_count"`
		Met             string `json:"met"`
	} `json:"text"`
}

// LoadCorrelationLabels reads the shared labels. No fallback copy: a missing or
// incomplete file stops momentum-api at start.
func LoadCorrelationLabels(path string) (CorrelationLabels, error) {
	var l CorrelationLabels
	b, err := os.ReadFile(path)
	if err != nil {
		return l, fmt.Errorf("correlation labels: %w", err)
	}
	if err := json.Unmarshal(b, &l); err != nil {
		return l, fmt.Errorf("correlation labels %s: %w", path, err)
	}
	if len(l.Clusters) == 0 || len(l.ClusterTiers) == 0 || l.NotEvaluated == "" || len(l.Patterns) == 0 ||
		len(l.NetSignal) == 0 || l.Text.PatternsHeading == "" || l.Text.NetCount == "" || l.Text.Met == "" {
		return l, fmt.Errorf("correlation labels %s: a section is missing or empty", path)
	}
	return l, nil
}

// label is table[code], or the code itself when the table does not know it.
func label(table map[string]string, code string) string {
	if v, ok := table[code]; ok {
		return v
	}
	return code
}
