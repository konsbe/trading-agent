//go:build integration

package store

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// mustExec runs a statement and fails the test on error.
func mustExec(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("exec %.60s: %v", sql, err)
	}
}

// THE DEFECT THESE TESTS PIN
//
// equity_fundamentals is written by five sources, and two of them write the
// SAME (symbol, metric, period, ts) with different values -- one NULL by
// design, because /stock/metric does not report a share count while
// /stock/profile2 does. A latest-row query ordered only by ts is therefore a
// coin flip that can land on NULL.
//
// It did: the universe-fundamentals loader picked the NULL for 2,951 of 4,975
// eligible symbols (59%). That loader has since been DELETED — it wrote columns
// nothing read — so these tests assert the ORDERING CONTRACT directly against
// the database instead of through a Go function.
//
// That is deliberate. The contract is what the live consumers depend on:
// momentum-scanner, momentum-backtest, momentum-tracker, momentum-dryrun and
// the analyst-bot queries all rely on NULL-last-then-source-rank. Testing the
// contract rather than one caller means the test keeps its value when callers
// come and go, which is exactly what just happened to the original one.

const canonicalLatestRow = `
SELECT DISTINCT ON (symbol) value
FROM equity_fundamentals
WHERE symbol = $1 AND metric = 'shares_outstanding'
ORDER BY symbol, ts DESC,
         (value IS NULL AND payload IS NULL),
         fundamental_source_rank(source) DESC`

// The exact tie that broke production: same symbol, same metric, same period,
// same timestamp, one row NULL by design and one carrying the number.
func TestLatestRowOrdering_PrefersNonNullOverNullAtTheSameTimestamp(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	ts := time.Now().UTC().Truncate(time.Second)
	sym := "ZZTESTSRC"

	cleanup := func() { mustExec(t, pool, `DELETE FROM equity_fundamentals WHERE symbol = $1`, sym) }
	cleanup()
	t.Cleanup(cleanup)

	mustExec(t, pool, `
INSERT INTO equity_fundamentals (ts, symbol, period, metric, value, source)
VALUES ($1, $2, 'ttm', 'shares_outstanding', NULL, 'finnhub_metric'),
       ($1, $2, 'ttm', 'shares_outstanding', 123456789, 'finnhub_profile2')`, ts, sym)

	var got *float64
	if err := pool.QueryRow(ctx, canonicalLatestRow, sym).Scan(&got); err != nil {
		t.Fatalf("query: %v", err)
	}
	if got == nil {
		t.Fatal("the canonical ordering returned NULL over a real value at the same ts — " +
			"this is the defect that cost 2,951 symbols their share count; the NULL-last " +
			"clause must come BEFORE the source rank")
	}
	if *got != 123456789 {
		t.Errorf("value = %v, want 123456789", *got)
	}
}

// Source rank must decide when BOTH rows carry a usable value, so the result
// does not depend on physical row order. NULL-last cannot help here.
func TestLatestRowOrdering_SourceRankDecidesBetweenTwoRealValues(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	ts := time.Now().UTC().Truncate(time.Second)
	sym := "ZZTESTRNK"

	cleanup := func() { mustExec(t, pool, `DELETE FROM equity_fundamentals WHERE symbol = $1`, sym) }
	cleanup()
	t.Cleanup(cleanup)

	// finnhub_profile2 (rank 100) must beat alphavantage_overview (rank 10).
	mustExec(t, pool, `
INSERT INTO equity_fundamentals (ts, symbol, period, metric, value, source)
VALUES ($1, $2, 'ttm', 'shares_outstanding', 999, 'alphavantage_overview'),
       ($1, $2, 'ttm', 'shares_outstanding', 111, 'finnhub_profile2')`, ts, sym)

	var got float64
	if err := pool.QueryRow(ctx, canonicalLatestRow, sym).Scan(&got); err != nil {
		t.Fatalf("query: %v", err)
	}
	if got != 111 {
		t.Errorf("value = %v, want 111 from finnhub_profile2 (rank 100) rather than "+
			"alphavantage_overview (rank 10); without the source rank this result depends "+
			"on physical row order and can change between runs", got)
	}
}

// Source rank alone is NOT sufficient, and this test exists to stop someone
// "simplifying" the ordering by dropping the NULL-last clause. finnhub_metric
// outranks alphavantage_overview, so a source-only ordering would choose
// finnhub_metric's NULL over a real number from the lower-ranked source.
func TestLatestRowOrdering_NullLastMustPrecedeSourceRank(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	ts := time.Now().UTC().Truncate(time.Second)
	sym := "ZZTESTORD"

	cleanup := func() { mustExec(t, pool, `DELETE FROM equity_fundamentals WHERE symbol = $1`, sym) }
	cleanup()
	t.Cleanup(cleanup)

	mustExec(t, pool, `
INSERT INTO equity_fundamentals (ts, symbol, period, metric, value, source)
VALUES ($1, $2, 'ttm', 'shares_outstanding', NULL, 'finnhub_metric'),
       ($1, $2, 'ttm', 'shares_outstanding', 555, 'alphavantage_overview')`, ts, sym)

	var got *float64
	if err := pool.QueryRow(ctx, canonicalLatestRow, sym).Scan(&got); err != nil {
		t.Fatalf("query: %v", err)
	}
	if got == nil {
		t.Fatal("ordering chose the higher-ranked source's NULL over a lower-ranked source's " +
			"real value — the NULL-last clause is missing or is placed after the source rank")
	}
	if *got != 555 {
		t.Errorf("value = %v, want 555", *got)
	}
}

// The ranking functions themselves, so a migration that reorders them fails
// here rather than silently changing which provider wins across the codebase.
func TestSourceRankFunctions_PinTheDocumentedOrder(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)

	for _, c := range []struct {
		src  string
		want int
	}{
		{"finnhub_profile2", 100},
		{"finnhub_metric", 90},
		{"finnhub_financials_reported", 80},
		{"finnhub_earnings", 70},
		{"alphavantage_overview", 10},
		{"something_new", 0},
	} {
		var got int
		if err := pool.QueryRow(ctx, `SELECT fundamental_source_rank($1)`, c.src).Scan(&got); err != nil {
			t.Fatalf("rank(%s): %v", c.src, err)
		}
		if got != c.want {
			t.Errorf("fundamental_source_rank(%q) = %d, want %d", c.src, got, c.want)
		}
	}

	for _, c := range []struct {
		src  string
		want int
	}{
		{"tiingo", 100},
		{"yahoo_finance", 50},
		{"finnhub_quote", 10},
		{"twelve_data", 5},
		{"unknown", 0},
	} {
		var got int
		if err := pool.QueryRow(ctx, `SELECT bar_source_rank($1)`, c.src).Scan(&got); err != nil {
			t.Fatalf("rank(%s): %v", c.src, err)
		}
		if got != c.want {
			t.Errorf("bar_source_rank(%q) = %d, want %d", c.src, got, c.want)
		}
	}

	// An unranked source must degrade to lowest priority rather than error or
	// outrank a known one, so adding a provider cannot silently win.
	var unknown, twelve int
	_ = pool.QueryRow(ctx, `SELECT bar_source_rank('brand_new_provider')`).Scan(&unknown)
	_ = pool.QueryRow(ctx, `SELECT bar_source_rank('twelve_data')`).Scan(&twelve)
	if unknown >= twelve {
		t.Errorf("an unranked source (%d) must not outrank twelve_data (%d)", unknown, twelve)
	}
}
