//go:build integration

package store

import (
	"context"
	"testing"
)

// The reference series (SPY, IWM, VIXCLS) exist for Phase 2 §3.6's regime
// features. They are NOT candidates: SPY and IWM are ETFs, which §3.1 excludes
// by instrument type, and VIXCLS is not a tradable security at all.
//
// Keeping them out of the universe has to be structural, not remembered. If
// they lived in universe_symbols (and so in the scanner's input), then the eligibility
// filter, every report scope, the denominator guard and the scanner would each
// have to exclude three symbols by name — and the first one to forget would
// add three phantom candidates whose features compute perfectly well.
//
// These tests assert the separation holds at every layer that could leak it.

func refSeriesIDs() []string { return []string{"SPY", "IWM", "VIXCLS"} }

func TestReferenceSeries_AreNotInTheUniverse(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)

	for _, id := range refSeriesIDs() {
		var n int
		if err := pool.QueryRow(ctx,
			`SELECT count(*) FROM universe_symbols WHERE symbol = $1`, id).Scan(&n); err != nil {
			t.Fatalf("query: %v", err)
		}
		if n != 0 {
			t.Errorf("%s is in universe_symbols (%d row(s)); it is a reference series, not a candidate — "+
				"its presence would add a phantom candidate whose features compute perfectly well", id, n)
		}
	}
}

// Narrowed 2026-09-25 (was TestReferenceSeries_AreNotInEquityOHLCV).
//
// SPY, IWM and other ETFs now DO have rows in equity_ohlcv, deliberately: the
// daily market report pipeline (data-technical -> macro-analysis, additional,
// the bot's price cards) reads its benchmark and instrument bars there, and did
// long before migration 023. Membership in equity_ohlcv was never the risk;
// being reachable as a scanner candidate is. That is guarded by the tests in
// this file on every layer that could leak it: universe_symbols, eligibility,
// the report-scope join, the scanner's exact input query (below) — and one
// independent barrier here: such rows must never carry the scanner's source.
func TestReferenceSeries_InEquityOHLCVOnlyUnderANonScannerSource(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	var n int
	if err := pool.QueryRow(ctx, `
SELECT count(*) FROM equity_ohlcv WHERE symbol = ANY($1) AND source = 'tiingo'`, refSeriesIDs()).Scan(&n); err != nil {
		t.Fatalf("query: %v", err)
	}
	if n != 0 {
		t.Errorf("%d reference-series row(s) in equity_ohlcv under source 'tiingo' — the source the scanner, "+
			"tracker and momentum-daily read; report bars must come from another source (e.g. yahoo_finance)", n)
	}
}

// The scanner's input, byte-for-byte its loadBars predicate
// (cmd/momentum-scanner/main.go): reference series must be unreachable through it.
func TestReferenceSeries_AbsentFromTheScannerInput(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	var n int
	if err := pool.QueryRow(ctx, `
SELECT count(*)
FROM equity_ohlcv o
JOIN universe_symbols u ON u.symbol = o.symbol AND u.is_eligible
                       AND u.data_unavailable_reason IS NULL
WHERE o.interval = '1Day' AND o.source = 'tiingo' AND o.close > 0
  AND o.symbol = ANY($1)`, refSeriesIDs()).Scan(&n); err != nil {
		t.Fatalf("query: %v", err)
	}
	if n != 0 {
		t.Errorf("%d reference-series bar(s) reach the scanner's input; they would become candidates", n)
	}
}

// The eligibility filter is what the scanner and every report scope key on, so
// this is the check that matters most: even if a reference symbol somehow got
// a universe row, it must never be eligible.
func TestReferenceSeries_CannotBeEligible(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)

	var n int
	if err := pool.QueryRow(ctx, `
SELECT count(*) FROM universe_symbols
WHERE is_eligible AND symbol = ANY($1)`, refSeriesIDs()).Scan(&n); err != nil {
		t.Fatalf("query: %v", err)
	}
	if n != 0 {
		t.Errorf("%d reference series are marked eligible; they would enter the scanner, "+
			"the denominator guard's expected count, and every base rate computed from it", n)
	}
}

// reportscope's eligible join is the single clause every report reads through.
// Asserted here against the live schema rather than by inspecting the string,
// so a change to the clause is caught by behaviour.
func TestReferenceSeries_AbsentFromTheReportScopeJoin(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)

	rows, err := pool.Query(ctx, `
SELECT DISTINCT o.symbol
FROM equity_ohlcv o
JOIN universe_symbols us ON us.symbol = o.symbol AND us.is_eligible
                        AND us.data_unavailable_reason IS NULL
WHERE o.symbol = ANY($1)`, refSeriesIDs())
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer rows.Close()

	var leaked []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatal(err)
		}
		leaked = append(leaked, s)
	}
	if len(leaked) > 0 {
		t.Errorf("reference series reachable through the eligible report scope: %v", leaked)
	}
}

// The data must actually be there — a test suite that passes because the
// table is empty would be worse than no test.
func TestReferenceSeries_ArePresentInTheirOwnTable(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)

	for _, id := range refSeriesIDs() {
		var n int
		if err := pool.QueryRow(ctx,
			`SELECT count(*) FROM reference_series WHERE series_id = $1`, id).Scan(&n); err != nil {
			t.Fatalf("query: %v", err)
		}
		if n < 500 {
			t.Errorf("reference_series has only %d row(s) for %s; the isolation tests above would "+
				"pass vacuously if the data were simply missing", n, id)
		}
	}
}
