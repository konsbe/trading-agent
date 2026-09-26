package momentumapi

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// HEURISTIC_TA_CAVEAT states the heuristic-signals round-1 result. The retired
// "never tested" claim is inverted here so it cannot come back (the same
// discipline as analyst-bot's notifier/discord/test_momentum.py).
func TestHeuristicTACaveat_StatesTheRound1NullResult(t *testing.T) {
	raw, err := os.ReadFile(sharedCaveatsPath(t))
	if err != nil {
		t.Fatal(err)
	}
	var file map[string]any
	if err := json.Unmarshal(raw, &file); err != nil {
		t.Fatal(err)
	}
	text, _ := file["heuristic_ta_caveat"].(string)
	if text == "" {
		t.Fatal("shared file has no heuristic_ta_caveat")
	}
	// Order blocks were only ever one input of the BUY/TRIM composite (H9/H10),
	// never a standalone hypothesis; the caveat must not list them as tested alone.
	for _, must := range []string{"not confirmed", "lockbox", "not a demonstrated edge",
		"order blocks were tested only as one input inside the BUY/TRIM composite"} {
		if !strings.Contains(text, must) {
			t.Errorf("heuristic_ta_caveat must say %q", must)
		}
	}
	for _, retired := range []string{"neither validated nor refuted", "have not been tested", "0.991",
		"liquidity sweeps, order blocks,"} {
		if strings.Contains(text, retired) {
			t.Errorf("heuristic_ta_caveat still carries the retired claim %q", retired)
		}
	}
	if text == file["research_score_caveat"] {
		t.Error("heuristic_ta_caveat must differ from research_score_caveat")
	}
	if got := loadSharedCaveats(t).HeuristicTA; got != text {
		t.Errorf("LoadCaveats HeuristicTA = %q\nshared file         = %q", got, text)
	}
}
