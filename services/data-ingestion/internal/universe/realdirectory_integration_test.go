//go:build integration

package universe

import (
	"encoding/json"
	"os"
	"sort"
	"testing"
)

// Verifies the §3.1 eligibility rules against a real provider symbol directory
// rather than hand-written fixtures.
//
//	UNIVERSE_DIRECTORY_JSON=/path/to/us_syms.json \
//	  go test -tags=integration ./internal/universe/ -run RealDirectory -v
//
// Fixtures prove the rules do what they say; this proves the rules produce a
// sane universe from the data the provider actually returns. §2.3 expects
// 5,000–7,500 eligible US common stocks, and a count far outside that means the
// type or MIC allowlist is wrong — which would silently distort everything
// downstream, including §6's base rate.
//
// Skipped when the env var is unset, so the default suite stays hermetic.
func TestRealDirectory_EligibleCountIsInTheExpectedRange(t *testing.T) {
	path := os.Getenv("UNIVERSE_DIRECTORY_JSON")
	if path == "" {
		t.Skip("UNIVERSE_DIRECTORY_JSON not set")
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open directory: %v", err)
	}
	defer f.Close()

	// Mirrors finnhub.StockSymbol, decoded locally so this package keeps no
	// dependency on the fetch layer.
	var raw []struct {
		Symbol      string `json:"symbol"`
		Description string `json:"description"`
		Type        string `json:"type"`
		MIC         string `json:"mic"`
		Currency    string `json:"currency"`
	}
	if err := json.NewDecoder(f).Decode(&raw); err != nil {
		t.Fatalf("decode directory: %v", err)
	}
	if len(raw) == 0 {
		t.Fatal("directory is empty")
	}

	recs := make([]Record, 0, len(raw))
	for _, r := range raw {
		recs = append(recs, Record{
			Symbol: r.Symbol, DisplayName: r.Description,
			Type: r.Type, MIC: r.MIC, Currency: r.Currency,
		})
	}

	plan := NewRules(nil, nil, nil, false).BuildPlan(recs)

	t.Logf("directory records:      %d", plan.Tally.Total)
	t.Logf("eligible:               %d", plan.Tally.Eligible)
	t.Logf("persistable decisions:  %d", len(plan.Decisions))
	t.Logf("skipped off-venue:      %d", plan.SkippedOffVenue)
	t.Logf("skipped duplicate:      %d", plan.SkippedDuplicate)
	for _, reason := range plan.Tally.ReasonsSorted() {
		t.Logf("  excluded %-26s %d", reason, plan.Tally.ByReason[reason])
	}

	// Per-exchange split, to confirm all three venues are represented.
	byExchange := map[string]int{}
	for _, d := range plan.Decisions {
		if d.Eligible {
			byExchange[d.Exchange]++
		}
	}
	venues := make([]string, 0, len(byExchange))
	for v := range byExchange {
		venues = append(venues, v)
	}
	sort.Strings(venues)
	for _, v := range venues {
		t.Logf("  eligible on %-16s %d", v, byExchange[v])
	}

	// §2.3's stated expectation, with generous slack: the assertion is meant to
	// catch an allowlist mistake, not to pin a number that drifts with listings.
	const lo, hi = 3000, 12000
	if plan.Tally.Eligible < lo || plan.Tally.Eligible > hi {
		t.Errorf("eligible = %d, outside the sane range [%d, %d] — check UNIVERSE_ALLOWED_TYPES / UNIVERSE_ALLOWED_MICS",
			plan.Tally.Eligible, lo, hi)
	}
	for _, v := range []string{"NASDAQ", "NYSE", "NYSE American"} {
		if byExchange[v] == 0 {
			t.Errorf("no eligible symbols on %s — the MIC allowlist is likely wrong", v)
		}
	}
	// Every ineligible persisted row must carry a reason, or the filter is not
	// auditable (§10 step 2).
	for _, d := range plan.Decisions {
		if !d.Eligible && d.Reason == "" {
			t.Fatalf("symbol %q excluded with no reason", d.Symbol)
		}
	}
}
