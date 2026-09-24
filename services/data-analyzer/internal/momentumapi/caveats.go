package momentumapi

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/konsbe/trading-agent/services/data-analyzer/internal/momentum"
)

// Caveats are the claims every surface makes about the scanner, read from
// shared/content/momentum_caveats.json — the same file analyst-bot reads, so
// the web app and Discord cannot drift apart. There is no compiled-in default:
// a missing file fails startup, since a fallback string would be a second
// source of the claim.
type Caveats struct {
	// Evidence is the screener framing ("SCREENER, not a forecast...").
	Evidence string `json:"evidence_caveat"`
	// ResearchScore accompanies the score wherever it is shown.
	ResearchScore string `json:"research_score_caveat"`
	// ExitReasonNotes holds one note per §5 exit reason, served with every
	// tracked row that carries that reason. Specific per rule, never a blanket
	// warning (Tracked Positions addendum §1).
	ExitReasonNotes map[string]string `json:"exit_reason_notes"`
}

// exitReasons are every value momentum_tracked.exit_reason can hold.
var exitReasons = []string{
	momentum.ExitBreakoutFailed, momentum.ExitLostVWAP, momentum.ExitMomentumStalled,
	momentum.ExitStopATR, momentum.ExitTimeout,
}

func LoadCaveats(path string) (Caveats, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Caveats{}, fmt.Errorf("read momentum caveats: %w", err)
	}
	var c Caveats
	if err := json.Unmarshal(raw, &c); err != nil {
		return Caveats{}, fmt.Errorf("parse momentum caveats %s: %w", path, err)
	}
	if c.Evidence == "" || c.ResearchScore == "" {
		return Caveats{}, fmt.Errorf("momentum caveats %s: evidence_caveat and research_score_caveat are both required", path)
	}
	for _, r := range exitReasons {
		if c.ExitReasonNotes[r] == "" {
			return Caveats{}, fmt.Errorf("momentum caveats %s: exit_reason_notes.%s is required", path, r)
		}
	}
	return c, nil
}
