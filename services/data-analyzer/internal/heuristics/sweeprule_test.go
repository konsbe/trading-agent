package heuristics

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

type parityCase struct {
	Name           string   `json:"name"`
	Kind           string   `json:"kind"`
	BarClose       *float64 `json:"bar_close"`
	SweptLevel     *float64 `json:"swept_level"`
	BullishOB      bool     `json:"bullish_ob"`
	BearishOB      bool     `json:"bearish_ob"`
	TrendDir       string   `json:"trend_dir"`
	VIXRegime      string   `json:"vix_regime"`
	WantAction     string   `json:"want_action"`
	WantConfluence int      `json:"want_confluence"`
}

// Every row's expected (action, confluence) was produced by running the live
// services/analyst-bot/actions/rules/liquidity_sweep.py:evaluate — regenerate
// with testdata/gen_sweep_rule_parity.py, never by hand.
func TestSweepRuleMatchesPython(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "sweep_rule_parity.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fx struct {
		Cases []parityCase `json:"cases"`
	}
	if err := json.Unmarshal(raw, &fx); err != nil {
		t.Fatal(err)
	}
	if len(fx.Cases) < 1000 {
		t.Fatalf("only %d parity cases", len(fx.Cases))
	}
	outcomes := map[SweepRuleResult]int{}
	for _, c := range fx.Cases {
		got := EvaluateSweepRule(SweepRuleInput{
			Kind: c.Kind, BarClose: c.BarClose, SweptLevel: c.SweptLevel,
			BullishOB: c.BullishOB, BearishOB: c.BearishOB,
			TrendDir: c.TrendDir, VIXRegime: c.VIXRegime,
		})
		want := SweepRuleResult{Action: c.WantAction, Confluence: c.WantConfluence}
		if got != want {
			t.Errorf("%s: go %+v, python %+v", c.Name, got, want)
		}
		outcomes[want]++
	}
	if outcomes[SweepRuleResult{ActionBuyWatch, 4}] == 0 || outcomes[SweepRuleResult{ActionTrimWatch, 4}] == 0 {
		t.Error("fixture must cover both 4/4 composites")
	}
	if outcomes[SweepRuleResult{ActionWatch, 3}] == 0 {
		t.Error("fixture must cover the risk-off veto (low sweep reclaimed + OB, no uptrend, elevated VIX → WATCH at confluence 3)")
	}
}
