//go:build integration

package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func seedForSubset(t *testing.T, n int) {
	t.Helper()
	ctx := context.Background()
	pool := testPool(t)
	clearUniverse(t, pool)
	rows := make([]UniverseRow, 0, n)
	for i := 0; i < n; i++ {
		rows = append(rows, UniverseRow{
			Symbol: fmt.Sprintf("SYM%04d", i), Exchange: "NASDAQ", MIC: "XNAS",
			Type: "Common Stock", IsEligible: true,
		})
	}
	// Plus an ineligible row that must never be selected.
	rows = append(rows, UniverseRow{
		Symbol: "WARRANT", Exchange: "NASDAQ", Type: "Warrant",
		IsEligible: false, ExcludedReason: "type_not_common_stock",
	})
	if _, err := UpsertUniverseSymbols(ctx, pool, rows); err != nil {
		t.Fatalf("seed: %v", err)
	}
}

// THE guard: nothing else enforces Tiingo's 500-unique-symbols/month allowance,
// so an over-large request must fail loudly and change nothing.
func TestSelectPilotSubset_SizeCapIsAssertedBeforeAnyWrite(t *testing.T) {
	ctx := context.Background()
	seedForSubset(t, 600)
	pool := testPool(t)

	_, err := SelectPilotSubset(ctx, pool, SelectSubsetParams{
		Strategy: SubsetRandom, Size: 501, MaxSize: 450,
	})
	var sizeErr *SubsetSizeError
	if !errors.As(err, &sizeErr) {
		t.Fatalf("err = %v, want *SubsetSizeError", err)
	}
	if sizeErr.Requested != 501 || sizeErr.Max != 450 {
		t.Errorf("error carried %d/%d, want 501/450", sizeErr.Requested, sizeErr.Max)
	}
	// The message must explain the consequence, not just the arithmetic — this is
	// read weeks later by someone who does not know why 450 matters.
	if !contains(err.Error(), "500 unique symbols per month") {
		t.Errorf("error should explain why the cap exists, got: %v", err)
	}

	// Nothing written.
	var n int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM universe_symbols WHERE backfill_selected`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("%d rows selected despite a rejected request — the assertion must run before any write", n)
	}
}

func TestSelectPilotSubset_RandomSelectsExactlyNEligible(t *testing.T) {
	ctx := context.Background()
	seedForSubset(t, 600)
	pool := testPool(t)

	n, err := SelectPilotSubset(ctx, pool, SelectSubsetParams{
		Strategy: SubsetRandom, Size: 450, MaxSize: 450,
	})
	if err != nil {
		t.Fatalf("select: %v", err)
	}
	if n != 450 {
		t.Fatalf("selected %d, want 450", n)
	}

	members, err := SelectedSubset(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	if len(members) != 450 {
		t.Errorf("SelectedSubset returned %d, want 450", len(members))
	}
	// The ineligible row must never appear.
	var bad int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM universe_symbols WHERE backfill_selected AND NOT is_eligible`).Scan(&bad); err != nil {
		t.Fatal(err)
	}
	if bad != 0 {
		t.Errorf("%d ineligible symbols were selected", bad)
	}
}

// Reselecting must replace, not accumulate — otherwise a second run would double
// the subset and silently blow through the monthly allowance.
func TestSelectPilotSubset_ReselectionReplacesRatherThanAccumulates(t *testing.T) {
	ctx := context.Background()
	seedForSubset(t, 600)
	pool := testPool(t)

	if _, err := SelectPilotSubset(ctx, pool, SelectSubsetParams{
		Strategy: SubsetRandom, Size: 300, MaxSize: 450,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := SelectPilotSubset(ctx, pool, SelectSubsetParams{
		Strategy: SubsetRandom, Size: 200, MaxSize: 450,
	}); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM universe_symbols WHERE backfill_selected`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 200 {
		t.Errorf("selected = %d after reselecting 300 then 200, want 200 — reselection must replace", n)
	}
}

// A seeded random selection must be reproducible, so a cited result can be
// re-derived rather than only re-approximated.
func TestSelectPilotSubset_SeedMakesSelectionReproducible(t *testing.T) {
	ctx := context.Background()
	seedForSubset(t, 600)
	pool := testPool(t)

	first := func() []string {
		if _, err := SelectPilotSubset(ctx, pool, SelectSubsetParams{
			Strategy: SubsetRandom, Size: 50, MaxSize: 450, Seed: "pilot-2026-09",
		}); err != nil {
			t.Fatal(err)
		}
		ms, err := SelectedSubset(ctx, pool)
		if err != nil {
			t.Fatal(err)
		}
		out := make([]string, len(ms))
		for i, m := range ms {
			out[i] = m.Symbol
		}
		return out
	}
	a, b := first(), first()
	if len(a) != 50 || len(b) != 50 {
		t.Fatalf("sizes %d/%d, want 50/50", len(a), len(b))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("same seed produced different samples at %d: %s vs %s", i, a[i], b[i])
		}
	}
}

func TestSelectPilotSubset_ExplicitUsesTheListAndStillRespectsTheCap(t *testing.T) {
	ctx := context.Background()
	seedForSubset(t, 600)
	pool := testPool(t)

	// Duplicates and case variants must collapse, so the cap counts real symbols.
	n, err := SelectPilotSubset(ctx, pool, SelectSubsetParams{
		Strategy: SubsetExplicit,
		Symbols:  []string{"SYM0001", "sym0001", " SYM0002 ", "SYM0003", "NOTINUNIVERSE"},
		MaxSize:  450,
	})
	if err != nil {
		t.Fatalf("select: %v", err)
	}
	// Three of the four distinct names exist and are eligible.
	if n != 3 {
		t.Errorf("selected %d, want 3 (dupes collapsed, unknown symbol ignored)", n)
	}

	// And the cap still applies to an explicit list.
	big := make([]string, 0, 500)
	for i := 0; i < 500; i++ {
		big = append(big, fmt.Sprintf("SYM%04d", i))
	}
	if _, err := SelectPilotSubset(ctx, pool, SelectSubsetParams{
		Strategy: SubsetExplicit, Symbols: big, MaxSize: 450,
	}); err == nil {
		t.Error("an explicit list over the cap must be rejected too")
	}
}

func TestSelectPilotSubset_RejectsNonPositiveSizeAndUnknownStrategy(t *testing.T) {
	ctx := context.Background()
	seedForSubset(t, 10)
	pool := testPool(t)

	if _, err := SelectPilotSubset(ctx, pool, SelectSubsetParams{Strategy: SubsetRandom, Size: 0, MaxSize: 450}); err == nil {
		t.Error("size 0 must be rejected")
	}
	if _, err := SelectPilotSubset(ctx, pool, SelectSubsetParams{Strategy: "curated", Size: 5, MaxSize: 450}); err == nil {
		t.Error("an unknown strategy must be rejected rather than silently defaulting")
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}

// ── stratified sampling ────────────────────────────────────────────────────

// seedPriced creates eligible symbols with a selection-time price in
// equity_ohlcv, so bucket membership is determinable before any bars exist.
//
// Writes quote_snapshot/finnhub_quote rows, matching what the pricing pass
// produces. That is the whole point of the bootstrap fix: stratification must be
// decidable without having spent bar-provider quota, so the fixture must not
// pre-supply 1Day bars.
func seedPriced(t *testing.T, nPenny, nMarket int) {
	t.Helper()
	ctx := context.Background()
	pool := testPool(t)
	clearUniverse(t, pool)
	if _, err := pool.Exec(ctx, `DELETE FROM equity_ohlcv WHERE source IN ('tiingo', 'finnhub_quote')`); err != nil {
		t.Fatal(err)
	}

	rows := make([]UniverseRow, 0, nPenny+nMarket)
	type pr struct {
		sym   string
		close float64
	}
	var prices []pr
	for i := 0; i < nPenny; i++ {
		s := fmt.Sprintf("PNY%04d", i)
		rows = append(rows, UniverseRow{Symbol: s, Exchange: "NASDAQ", Type: "Common Stock", IsEligible: true})
		prices = append(prices, pr{s, 0.50 + float64(i%100)/100}) // 0.50–1.49, inside $0.30–$2.00
	}
	for i := 0; i < nMarket; i++ {
		s := fmt.Sprintf("MKT%04d", i)
		rows = append(rows, UniverseRow{Symbol: s, Exchange: "NASDAQ", Type: "Common Stock", IsEligible: true})
		prices = append(prices, pr{s, 10 + float64(i%500)}) // >= $2
	}
	if _, err := UpsertUniverseSymbols(ctx, pool, rows); err != nil {
		t.Fatalf("seed universe: %v", err)
	}
	for _, p := range prices {
		if _, err := pool.Exec(ctx, `
INSERT INTO equity_ohlcv (ts, symbol, interval, open, high, low, close, volume, source)
VALUES (date_trunc('day', now()), $1, 'quote_snapshot', $2, $2, $2, $2, 0, 'finnhub_quote')
ON CONFLICT DO NOTHING`, p.sym, p.close); err != nil {
			t.Fatalf("seed price %s: %v", p.sym, err)
		}
	}
}

func stratParams(size int, floor float64, seed string) SelectSubsetParams {
	return SelectSubsetParams{
		Strategy: SubsetStratified, Size: size, MaxSize: 450,
		PennyFloorPct: floor, PennyMinPrice: 0.30, PennyMaxPrice: 2.00,
		PriceInterval: "quote_snapshot", PriceSource: "finnhub_quote", Seed: seed,
	}
}

// The point of stratifying: the penny share is a decision, not whatever the
// universe's natural proportion happens to yield at a 9 % sampling rate.
func TestSelectPilotSubset_StratifiedHonoursThePennyFloor(t *testing.T) {
	ctx := context.Background()
	// A deliberately penny-poor universe: 5 % penny by population. An
	// unstratified draw of 100 would expect ~5 penny symbols.
	seedPriced(t, 50, 950)
	pool := testPool(t)

	n, err := SelectPilotSubset(ctx, pool, stratParams(100, 0.20, "strat-test"))
	if err != nil {
		t.Fatalf("select: %v", err)
	}
	if n != 100 {
		t.Fatalf("selected %d, want 100", n)
	}

	st, err := LoadSubsetStats(ctx, pool, "quote_snapshot", "finnhub_quote", "1Day", "tiingo", 2.00)
	if err != nil {
		t.Fatal(err)
	}
	if st.Selected != 100 {
		t.Errorf("stats selected = %d, want 100", st.Selected)
	}
	// 20 % floor of 100 = 20, against a population that would have given ~5.
	if st.UnderTwoDollars != 20 {
		t.Errorf("penny symbols = %d, want exactly 20 (the 20%% floor) — a natural draw from this 5%%-penny universe would have given ~5",
			st.UnderTwoDollars)
	}
}

// Failing loudly beats silently returning a market-only sample: the row count
// would look correct and the missing bucket would surface only at Step 7.
func TestSelectPilotSubset_StratifiedFailsWhenPennyBucketTooThin(t *testing.T) {
	ctx := context.Background()
	seedPriced(t, 5, 500) // only 5 penny symbols exist
	pool := testPool(t)

	_, err := SelectPilotSubset(ctx, pool, stratParams(100, 0.20, "x")) // needs 20
	var se *StratificationError
	if !errors.As(err, &se) {
		t.Fatalf("err = %v, want *StratificationError", err)
	}
	if se.WantPenny != 20 || se.HavePenny != 5 {
		t.Errorf("error carried want %d have %d, want 20/5", se.WantPenny, se.HavePenny)
	}
	// Nothing written.
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM universe_symbols WHERE backfill_selected`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("%d rows selected despite a failed stratification", n)
	}
}

// The bootstrap dependency, made explicit: no prices means no stratification,
// and the error has to say how to proceed rather than just refusing.
func TestSelectPilotSubset_StratifiedExplainsTheBootstrapProblem(t *testing.T) {
	ctx := context.Background()
	seedForSubset(t, 200) // eligible symbols, but NO bars anywhere
	pool := testPool(t)

	_, err := SelectPilotSubset(ctx, pool, stratParams(100, 0.20, "x"))
	var se *StratificationError
	if !errors.As(err, &se) {
		t.Fatalf("err = %v, want *StratificationError", err)
	}
	if se.Priced != 0 {
		t.Errorf("priced = %d, want 0", se.Priced)
	}
	// The message must tell a fresh-install operator what to do, since
	// stratifying needs the prices the backfill produces.
	for _, want := range []string{"unstratified draw first", "reselection replaces"} {
		if !contains(err.Error(), want) {
			t.Errorf("error should explain the bootstrap path (%q missing): %v", want, err)
		}
	}
}

func TestSelectPilotSubset_StratifiedIsReproducibleWithASeed(t *testing.T) {
	ctx := context.Background()
	seedPriced(t, 200, 800)
	pool := testPool(t)

	draw := func() []string {
		if _, err := SelectPilotSubset(ctx, pool, stratParams(60, 0.25, "repeat-me")); err != nil {
			t.Fatal(err)
		}
		ms, err := SelectedSubset(ctx, pool)
		if err != nil {
			t.Fatal(err)
		}
		out := make([]string, len(ms))
		for i, m := range ms {
			out[i] = m.Symbol
		}
		return out
	}
	a, b := draw(), draw()
	if len(a) != 60 {
		t.Fatalf("size %d, want 60", len(a))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("same seed gave different samples at %d: %s vs %s", i, a[i], b[i])
		}
	}
}

// The cap still binds on a stratified draw.
func TestSelectPilotSubset_StratifiedRespectsTheSizeCap(t *testing.T) {
	ctx := context.Background()
	seedPriced(t, 200, 800)
	pool := testPool(t)

	p := stratParams(500, 0.20, "x") // over MaxSize 450
	if _, err := SelectPilotSubset(ctx, pool, p); err == nil {
		t.Error("a stratified draw over the cap must be rejected")
	} else {
		var se *SubsetSizeError
		if !errors.As(err, &se) {
			t.Errorf("err = %v, want *SubsetSizeError", err)
		}
	}
}

// seedBarsWithDrift writes the 1Day/tiingo bars the backfill would produce for
// already-selected symbols, moving driftSyms of them across the $2 boundary.
func seedBarsWithDrift(t *testing.T, driftSyms []string) {
	t.Helper()
	ctx := context.Background()
	pool := testPool(t)

	rows, err := pool.Query(ctx, `
SELECT o.symbol, o.close
FROM equity_ohlcv o
JOIN universe_symbols u ON u.symbol = o.symbol AND u.is_eligible AND u.backfill_selected
WHERE o.interval = 'quote_snapshot' AND o.source = 'finnhub_quote'`)
	if err != nil {
		t.Fatal(err)
	}
	type pr struct {
		sym   string
		close float64
	}
	var prices []pr
	for rows.Next() {
		var p pr
		if err := rows.Scan(&p.sym, &p.close); err != nil {
			t.Fatal(err)
		}
		prices = append(prices, p)
	}
	rows.Close()

	drift := map[string]bool{}
	for _, sy := range driftSyms {
		drift[sy] = true
	}
	for _, p := range prices {
		barClose := p.close
		if drift[p.sym] {
			// Cross the boundary: a penny quote becomes a market bar and vice
			// versa, which is exactly the $1.98-vs-$2.02 case.
			if p.close < 2.00 {
				barClose = 2.05
			} else {
				barClose = 1.95
			}
		}
		if _, err := pool.Exec(ctx, `
INSERT INTO equity_ohlcv (ts, symbol, interval, open, high, low, close, volume, source)
VALUES (date_trunc('day', now()), $1, '1Day', $2, $2, $2, $2, 1000000, 'tiingo')
ON CONFLICT DO NOTHING`, p.sym, barClose); err != nil {
			t.Fatalf("seed bar %s: %v", p.sym, err)
		}
	}
}

// Accepting bucket drift is fine; not noticing it is not. A symbol quoted at
// $2.02 and adjusted to $1.98 gets scored against the wrong bucket's thresholds
// for the whole pilot, and that looks like a merely odd candidate rather than a
// classification error — so the stats must name it.
func TestLoadSubsetStats_FlagsSelectionVersusBackfillBucketDrift(t *testing.T) {
	ctx := context.Background()
	seedPriced(t, 200, 800)
	pool := testPool(t)

	if _, err := SelectPilotSubset(ctx, pool, stratParams(100, 0.20, "drift-test")); err != nil {
		t.Fatalf("select: %v", err)
	}

	// Before any bars: nothing to compare, so the check must report zero rather
	// than inventing agreement or disagreement.
	pre, err := LoadSubsetStats(ctx, pool, "quote_snapshot", "finnhub_quote", "1Day", "tiingo", 2.00)
	if err != nil {
		t.Fatal(err)
	}
	if pre.WithBars != 0 || pre.BucketChecked != 0 || pre.BucketDrifted != 0 {
		t.Fatalf("pre-backfill: with_bars=%d checked=%d drifted=%d, want all 0",
			pre.WithBars, pre.BucketChecked, pre.BucketDrifted)
	}
	if pre.Selected != 100 {
		t.Fatalf("selected = %d, want 100", pre.Selected)
	}

	// Pick three selected symbols to move across the boundary.
	var chosen []string
	srows, err := pool.Query(ctx, `
SELECT symbol FROM universe_symbols
WHERE is_eligible AND backfill_selected ORDER BY symbol LIMIT 3`)
	if err != nil {
		t.Fatal(err)
	}
	for srows.Next() {
		var sy string
		if err := srows.Scan(&sy); err != nil {
			t.Fatal(err)
		}
		chosen = append(chosen, sy)
	}
	srows.Close()
	if len(chosen) != 3 {
		t.Fatalf("got %d symbols to drift, want 3", len(chosen))
	}

	seedBarsWithDrift(t, chosen)

	post, err := LoadSubsetStats(ctx, pool, "quote_snapshot", "finnhub_quote", "1Day", "tiingo", 2.00)
	if err != nil {
		t.Fatal(err)
	}
	if post.WithBars != 100 {
		t.Errorf("with_bars = %d, want 100 (every selected symbol backfilled)", post.WithBars)
	}
	if post.BucketChecked != 100 {
		t.Errorf("bucket_checked = %d, want 100", post.BucketChecked)
	}
	if post.BucketDrifted != 3 {
		t.Errorf("bucket_drifted = %d, want exactly 3 — only the symbols moved across $2 should be flagged", post.BucketDrifted)
	}
	if len(post.BucketDriftExamples) != 3 {
		t.Errorf("examples = %v, want 3 actionable entries", post.BucketDriftExamples)
	}
	for _, sy := range chosen {
		found := false
		for _, ex := range post.BucketDriftExamples {
			if strings.HasPrefix(ex, sy+" ") {
				found = true
			}
		}
		if !found {
			t.Errorf("drifted symbol %s missing from examples %v", sy, post.BucketDriftExamples)
		}
	}
}

// The selection-time price must come from the quote snapshot, not from bars.
// If stratification could only bucket symbols that already had bars, the
// bootstrap problem would still be live: the pilot's first draw would silently
// see an empty universe.
func TestSelectPilotSubset_StratifiesFromQuotesWithNoBarsPresent(t *testing.T) {
	ctx := context.Background()
	seedPriced(t, 200, 800)
	pool := testPool(t)

	var barCount int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM equity_ohlcv WHERE interval = '1Day' AND source = 'tiingo'`).Scan(&barCount); err != nil {
		t.Fatal(err)
	}
	if barCount != 0 {
		t.Fatalf("fixture has %d tiingo bars; the point is to stratify with none", barCount)
	}

	n, err := SelectPilotSubset(ctx, pool, stratParams(100, 0.20, "no-bars"))
	if err != nil {
		t.Fatalf("stratified draw must work off quotes alone: %v", err)
	}
	if n != 100 {
		t.Fatalf("selected %d, want 100", n)
	}
	st, err := LoadSubsetStats(ctx, pool, "quote_snapshot", "finnhub_quote", "1Day", "tiingo", 2.00)
	if err != nil {
		t.Fatal(err)
	}
	if st.UnderTwoDollars != 20 {
		t.Errorf("penny share = %d, want 20 (the floor), proving the bucketing signal was the quote", st.UnderTwoDollars)
	}
}
