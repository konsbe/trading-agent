//go:build integration

package store

import (
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/testdb"

	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// testPool is read-only by default; fixtures go through fixtureTx (see testdb).
func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	return testdb.Pool(t)
}

// Guards the source preference in QueryEquityBars.
//
// This is not a cosmetic tie-break. Tiingo rows are split- and dividend-adjusted;
// yahoo_finance rows are not dividend-adjusted, because the ingestion adapter
// decodes indicators.quote and never indicators.adjclose. Preferring Yahoo — as
// this query originally did — meant a symbol covered by both silently resolved
// to the unadjusted series, which is strictly worse than either source alone:
// the symbol looks fully covered while serving prices that drift from the
// adjusted series by the cumulative dividend, shifting every price-derived
// feature across any ex-dividend date.
func TestQueryEquityBars_PrefersTiingoOverYahooWhenBothExist(t *testing.T) {
	ctx := context.Background()
	tx := fixtureTx(t)

	const sym = "DEDUPTEST"

	ts := time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC)

	// Same timestamp, both sources, deliberately distinguishable closes. Yahoo
	// is inserted first so a query that merely takes "whatever came first" would
	// also pick the wrong row and be caught here.
	for _, row := range []struct {
		source string
		close  float64
	}{
		{"yahoo_finance", 100.0}, // unadjusted
		{"tiingo", 97.5},         // dividend-adjusted
	} {
		if _, err := tx.Exec(ctx, `
INSERT INTO equity_ohlcv (ts, symbol, interval, open, high, low, close, volume, source)
VALUES ($1, $2, '1Day', $3, $3, $3, $3, 1000000, $4)`, ts, sym, row.close, row.source); err != nil {
			t.Fatalf("insert %s: %v", row.source, err)
		}
	}

	bars, err := QueryEquityBars(ctx, tx, sym, "1Day", 10)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(bars) != 1 {
		t.Fatalf("got %d bars, want 1 — DISTINCT ON should collapse the two sources", len(bars))
	}
	if bars[0].Close != 97.5 {
		t.Errorf("close = %v, want 97.5 (the tiingo adjusted row); got the yahoo_finance unadjusted row instead", bars[0].Close)
	}
}

// Preference must not become a filter: when only Yahoo has a bar, that bar is
// still the best available and must be returned rather than dropped.
func TestQueryEquityBars_StillReturnsYahooWhenItIsTheOnlySource(t *testing.T) {
	ctx := context.Background()
	tx := fixtureTx(t)

	const sym = "YAHOOONLY"

	ts := time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC)
	if _, err := tx.Exec(ctx, `
INSERT INTO equity_ohlcv (ts, symbol, interval, open, high, low, close, volume, source)
VALUES ($1, $2, '1Day', 42, 42, 42, 42, 1000000, 'yahoo_finance')`, ts, sym); err != nil {
		t.Fatal(err)
	}

	bars, err := QueryEquityBars(ctx, tx, sym, "1Day", 10)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(bars) != 1 || bars[0].Close != 42 {
		t.Fatalf("got %d bars (%v), want the single yahoo_finance bar at 42", len(bars), bars)
	}
}

// Dedup must be per timestamp, not per symbol: a symbol whose history is partly
// Tiingo and partly Yahoo must keep every distinct bar, otherwise preferring
// Tiingo would silently truncate history to only the dates Tiingo covers.
func TestQueryEquityBars_PreferencePerTimestampDoesNotDropYahooOnlyDates(t *testing.T) {
	ctx := context.Background()
	tx := fixtureTx(t)

	const sym = "MIXEDHIST"

	day := func(d int) time.Time { return time.Date(2026, 3, d, 0, 0, 0, 0, time.UTC) }

	rows := []struct {
		ts     time.Time
		close  float64
		source string
	}{
		{day(2), 10, "yahoo_finance"}, // Yahoo only
		{day(3), 20, "yahoo_finance"}, // contested
		{day(3), 21, "tiingo"},        // contested — tiingo must win
		{day(4), 30, "tiingo"},        // Tiingo only
	}
	for _, r := range rows {
		if _, err := tx.Exec(ctx, `
INSERT INTO equity_ohlcv (ts, symbol, interval, open, high, low, close, volume, source)
VALUES ($1, $2, '1Day', $3, $3, $3, $3, 1000000, $4)`, r.ts, sym, r.close, r.source); err != nil {
			t.Fatal(err)
		}
	}

	bars, err := QueryEquityBars(ctx, tx, sym, "1Day", 10)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(bars) != 3 {
		t.Fatalf("got %d bars, want 3 (one per distinct date)", len(bars))
	}
	// Chronological, oldest first.
	want := []float64{10, 21, 30}
	for i, w := range want {
		if bars[i].Close != w {
			t.Errorf("bar %d close = %v, want %v", i, bars[i].Close, w)
		}
	}
}
