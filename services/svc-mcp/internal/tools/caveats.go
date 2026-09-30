package tools

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

// Caveats are read from shared/content/momentum_caveats.json, the file the
// web app and analyst-bot read, so Claude is told the same claims verbatim.
type Caveats struct {
	Evidence      string `json:"evidence_caveat"`
	ResearchScore string `json:"research_score_caveat"`
	HeuristicTA   string `json:"heuristic_ta_caveat"`
}

func LoadCaveats(path string) (Caveats, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Caveats{}, fmt.Errorf("caveats: %w", err)
	}
	var c Caveats
	if err := json.Unmarshal(raw, &c); err != nil {
		return Caveats{}, fmt.Errorf("caveats: %w", err)
	}
	if c.Evidence == "" || c.ResearchScore == "" || c.HeuristicTA == "" {
		return Caveats{}, errors.New("caveats: evidence_caveat, research_score_caveat and heuristic_ta_caveat are required")
	}
	return c, nil
}
