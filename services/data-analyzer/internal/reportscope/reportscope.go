// Package reportscope fixes the symbol scope of a report harness and refuses to
// let a report describe fewer symbols than it claims.
//
// # WHY THIS EXISTS
//
// Three report harnesses independently hardcoded `u.backfill_selected` into
// their bar and fundamental queries. That was correct while the 450-symbol
// pilot was the only backfilled data, and silently wrong the moment it was not.
// After the full-universe backfill, `momentum-backtest` printed
//
//	symbols: 4,971
//
// while scoring 450 of them, and reported 739 candidates where there were
// 9,571. Nothing errored. The bar query returned the pilot; the fundamentals
// query returned the pilot; every other symbol-day failed the §3.2 market-cap
// gate with `market_cap_unavailable`, which is an ordinary gate rejection and
// therefore indistinguishable in the output from a symbol that was genuinely
// ineligible. A report over 9% of the data looked exactly like a complete one.
//
// This is the same failure class as every other bug this project has found:
// not a crash, not an error, a confidently wrong number. So the fix is not
// "remember to pass the scope" but a guard that makes the mistake fail loudly.
//
// TWO MECHANISMS, BOTH NEEDED
//
//  1. ONE scope value drives every query in a run. A harness cannot widen its
//     bars while leaving its fundamentals narrow, because both clauses come
//     from the same Scope.
//  2. The denominator is CHECKED, not printed and hoped over. Before scoring,
//     the harness asks the database how many symbols the declared scope
//     actually contains and compares that with the set it loaded. A mismatch
//     is a fatal error naming both numbers.
//
// Mechanism 2 is what catches a future regression that mechanism 1 cannot:
// someone editing the SQL string directly.
package reportscope

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Scope names the symbol population a report covers.
type Scope string

const (
	// Eligible is the full §3.1 universe. The default for every report: a
	// report should describe all the data unless it says otherwise.
	Eligible Scope = "eligible"

	// Pilot is the frozen 450-symbol Phase 1 cohort, IN-SAMPLE for score v2.
	//
	// Reads from momentum_pilot_cohort, never from
	// universe_symbols.backfill_selected. The latter is the backfill's working
	// set, which the subset-selection job clears on every run, so a report
	// keyed on it would silently change population whenever the backfill was
	// re-planned.
	Pilot Scope = "pilot"
)

// Parse validates a user-supplied scope name.
func Parse(s string) (Scope, error) {
	switch Scope(s) {
	case Eligible:
		return Eligible, nil
	case Pilot:
		return Pilot, nil
	}
	return "", fmt.Errorf("unknown scope %q (want %q or %q)", s, Eligible, Pilot)
}

// JoinOn returns the SQL join restricting `alias` to this scope.
//
// The joined table always exposes column `symbol`, so the same clause works for
// equity_ohlcv and equity_fundamentals alike. Returned as a fragment rather
// than a whole query so harnesses keep their own projections, but the fragment
// is the only sanctioned way to express the restriction.
func (s Scope) JoinOn(alias string) string {
	switch s {
	case Pilot:
		return fmt.Sprintf("JOIN momentum_pilot_cohort mpc ON mpc.symbol = %s.symbol", alias)
	default:
		// data_unavailable_reason excludes symbols the provider no longer
		// serves: their stored bars cannot be refreshed onto the current
		// adjustment basis, so including them would mix a frozen remnant into
		// every report. See migration 022.
		return fmt.Sprintf(
			"JOIN universe_symbols us ON us.symbol = %s.symbol AND us.is_eligible "+
				"AND us.data_unavailable_reason IS NULL", alias)
	}
}

// ExpectedBarSymbols returns the symbols this scope SHOULD yield for the given
// interval and source: in scope, and holding at least one bar.
//
// "Holding at least one bar" is part of the definition on purpose. An eligible
// symbol that was never backfilled cannot appear in a bar-driven report, and
// counting it as expected would make the guard fire on every run for a reason
// that is not a bug.
func (s Scope) ExpectedBarSymbols(ctx context.Context, pool *pgxpool.Pool, interval, source string) (map[string]struct{}, error) {
	q := `
SELECT DISTINCT o.symbol
FROM equity_ohlcv o
` + s.JoinOn("o") + `
WHERE o.interval = $1 AND o.source = $2 AND o.close > 0`
	rows, err := pool.Query(ctx, q, interval, source)
	if err != nil {
		return nil, fmt.Errorf("expected bar symbols for scope %s: %w", s, err)
	}
	defer rows.Close()
	out := map[string]struct{}{}
	for rows.Next() {
		var sym string
		if err := rows.Scan(&sym); err != nil {
			return nil, err
		}
		out[sym] = struct{}{}
	}
	return out, rows.Err()
}

// Denominators is what a report must print: counts derived from the rows it
// actually handled, not from what it intended to handle.
type Denominators struct {
	Scope    Scope
	Expected int // symbols the declared scope contains, with bars
	Loaded   int // symbols the harness actually read
	Scored   int // symbols that survived the history minimum and were evaluated
	Bars     int // bars actually read
}

// Verify compares the loaded symbol set against the declared scope and returns
// a fatal error on any disagreement.
//
// Exact equality, deliberately. A tolerance would have let the original bug
// through — 450 against 4,971 is not a rounding difference, but a guard that
// accepts "close enough" invites a later one that is. If a legitimate reason to
// differ ever appears, it belongs in ExpectedBarSymbols as an explicit rule,
// where it is reviewable, not in a fuzzy comparison here.
func Verify(s Scope, expected map[string]struct{}, loaded map[string][]byte) error {
	return verifyKeys(s, expected, keysOf(loaded))
}

// VerifySet is Verify for callers holding a set rather than a data map.
func VerifySet(s Scope, expected, loaded map[string]struct{}) error {
	got := make([]string, 0, len(loaded))
	for k := range loaded {
		got = append(got, k)
	}
	return verifyKeys(s, expected, got)
}

func keysOf(m map[string][]byte) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func verifyKeys(s Scope, expected map[string]struct{}, loaded []string) error {
	if len(expected) == len(loaded) {
		// Same size is not the same set; check membership too, so a scope that
		// swapped one symbol for another still fails.
		var missing []string
		for k := range expected {
			if !contains(loaded, k) {
				missing = append(missing, k)
			}
		}
		if len(missing) == 0 {
			return nil
		}
		return mismatch(s, len(expected), len(loaded), missing, nil)
	}

	inLoaded := map[string]struct{}{}
	for _, k := range loaded {
		inLoaded[k] = struct{}{}
	}
	var missing, extra []string
	for k := range expected {
		if _, ok := inLoaded[k]; !ok {
			missing = append(missing, k)
		}
	}
	for k := range inLoaded {
		if _, ok := expected[k]; !ok {
			extra = append(extra, k)
		}
	}
	return mismatch(s, len(expected), len(loaded), missing, extra)
}

func contains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

func mismatch(s Scope, expected, loaded int, missing, extra []string) error {
	sort.Strings(missing)
	sort.Strings(extra)
	var b strings.Builder
	fmt.Fprintf(&b, "SCOPE MISMATCH: declared scope %q contains %d symbols with bars, but the harness loaded %d.\n",
		s, expected, loaded)
	b.WriteString("This is the silent-narrowing failure the denominator guard exists to catch:\n")
	b.WriteString("a report that covers part of the data looks identical to one that covers all of it,\n")
	b.WriteString("because the symbols it never saw simply never appear as candidates.\n")
	if len(missing) > 0 {
		fmt.Fprintf(&b, "  %d in scope but NOT loaded, e.g. %s\n", len(missing), sample(missing))
		b.WriteString("  -> usually a query still restricted to the pilot subset (backfill_selected).\n")
	}
	if len(extra) > 0 {
		fmt.Fprintf(&b, "  %d loaded but NOT in scope, e.g. %s\n", len(extra), sample(extra))
		b.WriteString("  -> usually a join that lost its scope restriction entirely.\n")
	}
	return fmt.Errorf("%s", b.String())
}

func sample(xs []string) string {
	if len(xs) > 8 {
		return strings.Join(xs[:8], ", ") + ", ..."
	}
	return strings.Join(xs, ", ")
}

// Print writes the denominators a report is required to show.
//
// Printed from what the run actually handled. The point is that a reader can
// check the report's own arithmetic without trusting its narration.
func (d Denominators) Print() {
	fmt.Printf("  scope:                   %s\n", d.Scope)
	fmt.Printf("  symbols in scope:        %d (verified against the database)\n", d.Expected)
	fmt.Printf("  symbols loaded:          %d\n", d.Loaded)
	fmt.Printf("  symbols scored:          %d", d.Scored)
	if d.Scored < d.Loaded {
		fmt.Printf("  (%d skipped: fewer bars than the §3.1 history minimum)", d.Loaded-d.Scored)
	}
	fmt.Println()
	fmt.Printf("  bars read:               %d\n", d.Bars)
}

// ReportMetricCoverage prints how much of the scope a fundamental metric covers.
//
// Not an error when low: coverage genuinely varies by metric
// (shares_outstanding reaches only 38.4% of the universe). It is printed
// because a coverage collapse is what a mis-scoped metric query looks like, and
// the original bug drove market_cap coverage to 9% while the report said
// nothing at all.
func ReportMetricCoverage(metric string, loaded, scopeSize int) {
	pct := 0.0
	if scopeSize > 0 {
		pct = 100 * float64(loaded) / float64(scopeSize)
	}
	fmt.Printf("  %-22s %d/%d symbols (%.1f%%)\n", metric+":", loaded, scopeSize, pct)
}
