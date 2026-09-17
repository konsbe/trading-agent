//go:build integration

package store

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/konsbe/trading-agent/services/data-ingestion/internal/universe"
)

// Integration coverage for the universe_symbols store layer against a real
// TimescaleDB with migration 007 applied.
//
//	go test -tags=integration ./internal/store/ -run Universe -v
//
// Requires TEST_DATABASE_URL. Skipped otherwise so the default `go test ./...`
// stays hermetic.
func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// requireScratchDB refuses to run destructive fixtures unless the connected
// database is named as a test database.
//
// clearUniverse truncates universe_symbols wholesale. That is fine on a scratch
// database and catastrophic anywhere else, and TEST_DATABASE_URL is one careless
// copy-paste from a populated instance. The failure is silent in both
// directions: the real universe is destroyed, and the fixtures are left behind
// marked is_eligible, where the next pilot draw will select them — a stratified
// sample containing MKT0042 still looks like a perfectly normal row count.
//
// The check is on the database NAME rather than on row counts or symbol shapes.
// Row counts cannot separate the two (seedPriced legitimately creates 1,000
// rows, more than some real fixtures) and symbol names cannot either, because
// other tests in this package use bare tickers like AAA and AAPL. A naming
// convention is unambiguous, needs no allowlist maintenance, and cannot produce
// a false positive on a genuine scratch database.
//
// It is also not an opt-in env flag on purpose: a flag gets set once and then
// forgotten, at which point it protects nothing.
func requireScratchDB(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	var dbName string
	if err := pool.QueryRow(context.Background(), `SELECT current_database()`).Scan(&dbName); err != nil {
		t.Fatalf("scratch-db guard: %v", err)
	}
	if !strings.Contains(strings.ToLower(dbName), "test") {
		t.Fatalf("refusing to run destructive fixtures against database %q: these tests truncate "+
			"universe_symbols and equity_ohlcv wholesale. Point TEST_DATABASE_URL at a database "+
			"whose name contains \"test\" (e.g. trading_test).", dbName)
	}
}

// clearUniverse empties universe_symbols for a fixture and registers cleanup so
// the fixtures do not outlive the test. A test that leaves eligible symbols
// behind is not merely untidy — it seeds the next run's universe.
func clearUniverse(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	requireScratchDB(t, pool)
	if _, err := pool.Exec(context.Background(), `DELETE FROM universe_symbols`); err != nil {
		t.Fatalf("clear: %v", err)
	}
	t.Cleanup(func() {
		ctx := context.Background()
		pool.Exec(ctx, `DELETE FROM universe_symbols`)
		pool.Exec(ctx, `DELETE FROM equity_ohlcv`)
		pool.Exec(ctx, `DELETE FROM fundamental_fetch_state`)
	})
}

// A realistic slice of Finnhub's US directory: real common stock, ETFs on both
// Arca and NASDAQ, a warrant, a unit, a preferred series, an OTC name, a
// share-class ticker, and a duplicate listing across NASDAQ tiers.
func sampleDirectory() []universe.Record {
	return []universe.Record{
		{Symbol: "AAPL", Type: "Common Stock", MIC: "XNAS", DisplayName: "APPLE INC"},
		{Symbol: "MSFT", Type: "Common Stock", MIC: "XNGS", DisplayName: "MICROSOFT CORP"},
		{Symbol: "XOM", Type: "Common Stock", MIC: "XNYS", DisplayName: "EXXON MOBIL CORP"},
		{Symbol: "UUUU", Type: "Common Stock", MIC: "XASE", DisplayName: "ENERGY FUELS INC"},
		{Symbol: "BRK.B", Type: "Common Stock", MIC: "XNYS", DisplayName: "BERKSHIRE HATHAWAY-CL B"},
		{Symbol: "U", Type: "Common Stock", MIC: "XNYS", DisplayName: "UNITY SOFTWARE INC"},
		{Symbol: "AAPL", Type: "Common Stock", MIC: "XNMS", DisplayName: "APPLE INC (dup tier)"},
		{Symbol: "SPY", Type: "ETP", MIC: "ARCX", DisplayName: "SPDR S&P 500 ETF TRUST"},
		{Symbol: "QQQ", Type: "ETP", MIC: "XNAS", DisplayName: "INVESCO QQQ TRUST"},
		{Symbol: "ABCDW", Type: "Warrant", MIC: "XNAS", DisplayName: "SOME SPAC WARRANT"},
		{Symbol: "ABCD.U", Type: "Common Stock", MIC: "XNAS", DisplayName: "SOME SPAC UNIT"},
		{Symbol: "BAC-PB", Type: "Common Stock", MIC: "XNYS", DisplayName: "BANK OF AMERICA PFD B"},
		{Symbol: "PENNYOTC", Type: "Common Stock", MIC: "OTCM", DisplayName: "SOMETHING OTC"},
		{Symbol: "CEFX", Type: "Closed-End Fund", MIC: "XNYS", DisplayName: "A CLOSED END FUND"},
	}
}

// decisionsToRows maps a directory through the same universe.BuildPlan the
// worker uses, so this test exercises the production mapping rather than a
// parallel copy of it.
func decisionsToRows(recs []universe.Record) ([]UniverseRow, universe.Plan) {
	byName := map[string]universe.Record{}
	for _, rec := range recs {
		k := strings.ToUpper(strings.TrimSpace(rec.Symbol))
		if _, ok := byName[k]; !ok {
			byName[k] = rec
		}
	}
	plan := universe.NewRules(nil, nil, nil, false).BuildPlan(recs)
	rows := make([]UniverseRow, 0, len(plan.Decisions))
	for _, d := range plan.Decisions {
		rec := byName[d.Symbol]
		rows = append(rows, UniverseRow{
			Symbol: d.Symbol, Exchange: d.Exchange, MIC: rec.MIC,
			Name: rec.DisplayName, Type: rec.Type,
			IsEligible: d.Eligible, ExcludedReason: d.Reason,
		})
	}
	return rows, plan
}

func TestUniverseUpsertIsIdempotentAndAuditable(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	clearUniverse(t, pool)

	rows, _ := decisionsToRows(sampleDirectory())

	if _, err := UpsertUniverseSymbols(ctx, pool, rows); err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	// Re-running the whole pass must not duplicate or change anything.
	if _, err := UpsertUniverseSymbols(ctx, pool, rows); err != nil {
		t.Fatalf("second upsert: %v", err)
	}

	counts, err := LoadUniverseCounts(ctx, pool)
	if err != nil {
		t.Fatalf("counts: %v", err)
	}
	if counts.Total != len(rows) {
		t.Errorf("total = %d, want %d (upsert duplicated rows)", counts.Total, len(rows))
	}

	// AAPL, MSFT, XOM, UUUU, BRK.B, U — the duplicate AAPL tier collapses.
	if counts.Eligible != 6 {
		t.Errorf("eligible = %d, want 6", counts.Eligible)
	}

	// Exclusions must be retained with a reason, not dropped.
	for reason, wantAtLeast := range map[string]int{
		universe.ReasonTypeNotCommonStock: 2, // QQQ, CEFX
		universe.ReasonTickerSuffix:       2, // ABCD.U, BAC-PB
	} {
		if counts.ByReason[reason] < wantAtLeast {
			t.Errorf("excluded_reason %q = %d, want >= %d", reason, counts.ByReason[reason], wantAtLeast)
		}
	}
	if counts.ByReason["unknown"] != 0 {
		t.Errorf("%d ineligible rows have no reason — exclusions must be auditable", counts.ByReason["unknown"])
	}

	if counts.ByExchange["NASDAQ"] == 0 || counts.ByExchange["NYSE"] == 0 || counts.ByExchange["NYSE American"] == 0 {
		t.Errorf("expected eligible rows on all three venues, got %v", counts.ByExchange)
	}
}

// Off-venue rows (ARCX, OTC) have no resolvable exchange and so cannot be keyed;
// they must be absent rather than stored under a blank exchange.
func TestUniverseOffVenueRowsAreNotPersisted(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	clearUniverse(t, pool)

	rows, plan := decisionsToRows(sampleDirectory())
	if _, err := UpsertUniverseSymbols(ctx, pool, rows); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	// SPY (ARCX) and PENNYOTC (OTCM) have no allowed venue and cannot be keyed.
	if plan.SkippedOffVenue != 2 {
		t.Errorf("off-venue skips = %d, want 2", plan.SkippedOffVenue)
	}
	// The duplicate AAPL listing on XNMS collapses into the XNAS row. This is
	// deduplication, not exclusion, and must be counted separately.
	if plan.SkippedDuplicate != 1 {
		t.Errorf("duplicate skips = %d, want 1", plan.SkippedDuplicate)
	}
	if got := plan.Tally.Total - len(rows); got != 3 {
		t.Errorf("total unpersisted = %d, want 3 (2 off-venue + 1 duplicate)", got)
	}
	var n int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM universe_symbols WHERE symbol IN ('SPY','PENNYOTC')`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("off-venue rows persisted: %d", n)
	}
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM universe_symbols WHERE exchange = ''`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("%d rows stored with a blank exchange", n)
	}
}

// The weekly symbol refresh knows nothing about bar progress and must not reset
// an in-flight or completed backfill to 'pending'.
func TestUniverseRefreshPreservesBackfillState(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	clearUniverse(t, pool)

	rows, _ := decisionsToRows(sampleDirectory())
	if _, err := UpsertUniverseSymbols(ctx, pool, rows); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if _, err := pool.Exec(ctx, `
UPDATE universe_symbols
SET backfill_status = 'done', backfill_attempts = 3, backfill_completed_at = now()
WHERE symbol = 'AAPL'`); err != nil {
		t.Fatal(err)
	}

	if _, err := UpsertUniverseSymbols(ctx, pool, rows); err != nil {
		t.Fatalf("re-upsert: %v", err)
	}

	var status string
	var attempts int
	if err := pool.QueryRow(ctx,
		`SELECT backfill_status, backfill_attempts FROM universe_symbols WHERE symbol='AAPL'`,
	).Scan(&status, &attempts); err != nil {
		t.Fatal(err)
	}
	if status != "done" || attempts != 3 {
		t.Errorf("backfill state clobbered by symbol refresh: status=%q attempts=%d", status, attempts)
	}
}

// A symbol that changes type or venue must stop carrying its old reason.
func TestUniverseStaleReasonIsCleared(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	clearUniverse(t, pool)

	ineligible := []UniverseRow{{
		Symbol: "FLIP", Exchange: "NASDAQ", MIC: "XNAS", Type: "Warrant",
		IsEligible: false, ExcludedReason: universe.ReasonTypeNotCommonStock,
	}}
	if _, err := UpsertUniverseSymbols(ctx, pool, ineligible); err != nil {
		t.Fatal(err)
	}

	eligible := []UniverseRow{{
		Symbol: "FLIP", Exchange: "NASDAQ", MIC: "XNAS", Type: "Common Stock",
		IsEligible: true, ExcludedReason: "",
	}}
	if _, err := UpsertUniverseSymbols(ctx, pool, eligible); err != nil {
		t.Fatal(err)
	}

	var isEligible bool
	var reason *string
	if err := pool.QueryRow(ctx,
		`SELECT is_eligible, excluded_reason FROM universe_symbols WHERE symbol='FLIP'`,
	).Scan(&isEligible, &reason); err != nil {
		t.Fatal(err)
	}
	if !isEligible {
		t.Error("re-listed symbol should be eligible")
	}
	if reason != nil {
		t.Errorf("stale excluded_reason retained: %q", *reason)
	}
}

// EligibleUniverse must not filter on bar_count: the backfill iterates this list
// to create the bars in the first place.
func TestEligibleUniverseIgnoresBarCount(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	clearUniverse(t, pool)

	rows, _ := decisionsToRows(sampleDirectory())
	if _, err := UpsertUniverseSymbols(ctx, pool, rows); err != nil {
		t.Fatal(err)
	}

	members, err := EligibleUniverse(ctx, pool)
	if err != nil {
		t.Fatalf("eligible universe: %v", err)
	}
	if len(members) != 6 {
		t.Fatalf("eligible members = %d, want 6 (bar_count is NULL for all of them)", len(members))
	}
	for _, m := range members {
		if m.Symbol == "" || m.Exchange == "" {
			t.Errorf("incomplete member: %+v", m)
		}
	}
}

// §8.1.2 + the unit conventions: millions in equity_fundamentals must arrive as
// absolute values in universe_symbols, and COALESCE must preserve a previous
// value when the provider returns null.
func TestUniverseFundamentalsConvertMillionsAndPreserveOnNull(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	clearUniverse(t, pool)

	rows, _ := decisionsToRows(sampleDirectory())
	if _, err := UpsertUniverseSymbols(ctx, pool, rows); err != nil {
		t.Fatal(err)
	}

	// Seed equity_fundamentals the way data-fundamental does: millions.
	if _, err := pool.Exec(ctx, `
INSERT INTO equity_fundamentals (ts, symbol, period, metric, value, payload, source) VALUES
  (now(), 'AAPL', 'ttm', 'market_cap',         3_000_000, NULL, 'finnhub_metric'),
  (now(), 'AAPL', 'ttm', 'shares_outstanding',    15_000, NULL, 'finnhub_metric'),
  (now(), 'AAPL', 'ttm', 'sector_profile',           NULL,
     '{"sector":"Technology","industry":"Consumer Electronics"}', 'alphavantage_overview')
ON CONFLICT DO NOTHING`); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadFundamentalsFromEquityFundamentals(ctx, pool)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	var aapl *UniverseFundamentals
	for i := range loaded {
		if loaded[i].Symbol == "AAPL" {
			aapl = &loaded[i]
		}
	}
	if aapl == nil {
		t.Fatal("AAPL not returned")
	}
	if aapl.MarketCap == nil || *aapl.MarketCap != 3e12 {
		t.Errorf("market cap = %v, want 3e12 ($3,000,000M × 1e6)", aapl.MarketCap)
	}
	if aapl.SharesOutstanding == nil || *aapl.SharesOutstanding != 15e9 {
		t.Errorf("shares = %v, want 1.5e10 (15,000M × 1e6)", aapl.SharesOutstanding)
	}
	if aapl.Sector == nil || *aapl.Sector != "Technology" {
		t.Errorf("sector = %v, want Technology", aapl.Sector)
	}

	if _, err := UpdateUniverseFundamentals(ctx, pool, loaded); err != nil {
		t.Fatalf("update: %v", err)
	}

	// A later pass with nulls must not erase what we just stored.
	if _, err := UpdateUniverseFundamentals(ctx, pool, []UniverseFundamentals{{
		Symbol: "AAPL", Exchange: "NASDAQ",
	}}); err != nil {
		t.Fatalf("null update: %v", err)
	}

	var cap, shares *float64
	var sector *string
	if err := pool.QueryRow(ctx,
		`SELECT market_cap, shares_outstanding, sector FROM universe_symbols WHERE symbol='AAPL'`,
	).Scan(&cap, &shares, &sector); err != nil {
		t.Fatal(err)
	}
	if cap == nil || *cap != 3e12 {
		t.Errorf("market cap erased by a null refresh: %v", cap)
	}
	if shares == nil || *shares != 15e9 {
		t.Errorf("shares erased by a null refresh: %v", shares)
	}
	if sector == nil || *sector != "Technology" {
		t.Errorf("sector erased by a null refresh: %v", sector)
	}

	cov, err := LoadFundamentalsCoverage(ctx, pool)
	if err != nil {
		t.Fatalf("coverage: %v", err)
	}
	if cov.Eligible != 6 {
		t.Errorf("coverage eligible = %d, want 6", cov.Eligible)
	}
	if cov.WithMarketCap != 1 || cov.WithShares != 1 || cov.WithSector != 1 {
		t.Errorf("coverage = cap %d / shares %d / sector %d, want 1/1/1",
			cov.WithMarketCap, cov.WithShares, cov.WithSector)
	}
	// The other five have neither, so §3.9's proxy cannot rescue them either.
	if cov.WithNeitherCapNor != 5 {
		t.Errorf("neither cap nor shares = %d, want 5", cov.WithNeitherCapNor)
	}
}

// bar_count is refreshed for auditability and must never flip is_eligible.
func TestRefreshBarCountsIsInformationalOnly(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	clearUniverse(t, pool)

	rows, _ := decisionsToRows(sampleDirectory())
	if _, err := UpsertUniverseSymbols(ctx, pool, rows); err != nil {
		t.Fatal(err)
	}
	// equity_ohlcv is keyed on ts, so re-running this test would otherwise
	// accumulate a fresh set of bars at a new now() and inflate the count.
	if _, err := pool.Exec(ctx,
		`DELETE FROM equity_ohlcv WHERE symbol = 'AAPL' AND source = 'yahoo_finance'`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
INSERT INTO equity_ohlcv (ts, symbol, interval, open, high, low, close, volume, source)
SELECT date_trunc('day', now()) - (g || ' days')::interval, 'AAPL', '1Day', 1,1,1,1,1000, 'yahoo_finance'
FROM generate_series(1, 12) g
ON CONFLICT DO NOTHING`); err != nil {
		t.Fatal(err)
	}

	if _, err := RefreshUniverseBarCounts(ctx, pool, "1Day", "yahoo_finance"); err != nil {
		t.Fatalf("refresh: %v", err)
	}

	var barCount *int
	var eligible bool
	if err := pool.QueryRow(ctx,
		`SELECT bar_count, is_eligible FROM universe_symbols WHERE symbol='AAPL'`,
	).Scan(&barCount, &eligible); err != nil {
		t.Fatal(err)
	}
	if barCount == nil {
		t.Fatal("bar_count is NULL after refresh")
	}
	if *barCount != 12 {
		t.Errorf("bar_count = %d, want 12", *barCount)
	}
	// 12 bars is far below the 250 minimum, yet eligibility is untouched.
	if !eligible {
		t.Error("bar-count refresh must not change is_eligible — that check belongs to the §3.2 gate")
	}
}
