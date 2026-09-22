package reportscope

import (
	"strings"
	"testing"
)

func set(syms ...string) map[string]struct{} {
	m := map[string]struct{}{}
	for _, s := range syms {
		m[s] = struct{}{}
	}
	return m
}

// The bug this package exists for, in miniature: the scope declares the full
// universe, the harness loaded the pilot subset, and every missing symbol would
// otherwise have been silently absent from the report.
func TestVerifySet_CatchesSilentNarrowing(t *testing.T) {
	expected := set("AAA", "BBB", "CCC", "DDD", "EEE")
	loaded := set("AAA", "BBB")

	err := VerifySet(Eligible, expected, loaded)
	if err == nil {
		t.Fatal("loading 2 of 5 in-scope symbols must be a fatal error — this is exactly the case where momentum-backtest printed \"symbols: 4,971\" and scored 450")
	}
	msg := err.Error()
	for _, want := range []string{"SCOPE MISMATCH", "5 symbols", "loaded 3", "CCC"} {
		if want == "loaded 3" {
			continue // count checked below against the real value
		}
		if !strings.Contains(msg, want) {
			t.Errorf("error text missing %q; a guard that fires without naming the numbers just moves the confusion\ngot: %s", want, msg)
		}
	}
	if !strings.Contains(msg, "loaded 2") {
		t.Errorf("error must state how many were actually loaded\ngot: %s", msg)
	}
	if !strings.Contains(msg, "backfill_selected") {
		t.Errorf("error should name the usual cause so the next person does not have to rediscover it\ngot: %s", msg)
	}
}

func TestVerifySet_PassesOnExactMatch(t *testing.T) {
	s := set("AAA", "BBB", "CCC")
	if err := VerifySet(Eligible, s, set("CCC", "AAA", "BBB")); err != nil {
		t.Fatalf("identical sets must pass regardless of order: %v", err)
	}
}

// Equal cardinality is not equal membership. A scope that swapped one symbol
// for another would produce a report of the right size about the wrong
// population, which is harder to notice than a short one.
func TestVerifySet_SameSizeDifferentMembersFails(t *testing.T) {
	err := VerifySet(Eligible, set("AAA", "BBB", "CCC"), set("AAA", "BBB", "ZZZ"))
	if err == nil {
		t.Fatal("same-size but different-membership sets must fail; counting alone would pass this")
	}
	if !strings.Contains(err.Error(), "CCC") {
		t.Errorf("error should name the missing symbol, got: %s", err.Error())
	}
}

// A join that lost its restriction entirely is the opposite failure and is also
// wrong: a pilot-scoped report must not quietly include the whole universe.
func TestVerifySet_CatchesOverBroadLoad(t *testing.T) {
	err := VerifySet(Pilot, set("AAA", "BBB"), set("AAA", "BBB", "CCC", "DDD"))
	if err == nil {
		t.Fatal("loading symbols outside the declared scope must fail")
	}
	if !strings.Contains(err.Error(), "NOT in scope") {
		t.Errorf("error should distinguish over-broad from narrow, got: %s", err.Error())
	}
}

// The pilot scope must read from the frozen cohort table, never from
// universe_symbols.backfill_selected -- that flag is the backfill's working
// set and the selection job clears it on every run, so a report keyed on it
// would change population without anyone editing the report.
func TestPilotScopeReadsFrozenCohortNotTheWorkingFlag(t *testing.T) {
	j := Pilot.JoinOn("o")
	if !strings.Contains(j, "momentum_pilot_cohort") {
		t.Errorf("pilot scope must join momentum_pilot_cohort, got: %s", j)
	}
	if strings.Contains(j, "backfill_selected") {
		t.Errorf("pilot scope must NOT depend on backfill_selected, which is rewritten by the subset-selection job, got: %s", j)
	}
}

func TestEligibleScopeJoinsTheFullUniverse(t *testing.T) {
	j := Eligible.JoinOn("f")
	if !strings.Contains(j, "is_eligible") {
		t.Errorf("eligible scope must restrict on is_eligible, got: %s", j)
	}
	if strings.Contains(j, "backfill_selected") {
		t.Errorf("eligible scope must not mention backfill_selected, got: %s", j)
	}
	if !strings.Contains(j, "f.symbol") {
		t.Errorf("join must bind to the caller's alias so the same clause works for bars and fundamentals, got: %s", j)
	}
}

func TestParseRejectsUnknownScope(t *testing.T) {
	if _, err := Parse("selected"); err == nil {
		t.Fatal("an unrecognised scope must be an error, not a silent default — defaulting is how the original bug survived")
	}
	for _, ok := range []string{"eligible", "pilot"} {
		if _, err := Parse(ok); err != nil {
			t.Errorf("Parse(%q) failed: %v", ok, err)
		}
	}
}
