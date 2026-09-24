//go:build integration

package store

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

// Query-layer tests for momentum-api against a real Postgres, per
// docs/MOMENTUM_SCANNER_API.md §5.
//
// Every fixture is written inside a transaction that is always rolled back.
// These tables are what momentum-scanner writes and analyst-bot reads; a
// committed fixture on a future date would become "the latest scan" for both.

// fixtureDay is after any real scan, so within the transaction it is the
// latest scan date.
var fixtureDay = time.Date(2099, 1, 2, 0, 0, 0, 0, time.UTC)

func fixtureTx(t *testing.T) pgx.Tx {
	t.Helper()
	ctx := context.Background()
	tx, err := testPool(t).Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
	return tx
}

type fixtureRow struct {
	symbol       string
	inUniverse   bool
	bucket       *string
	passed       bool
	rvol         *float64
	catalystTier *string
	score        *int
	failures     []string
}

func strp(s string) *string   { return &s }
func f64p(v float64) *float64 { return &v }
func intp(v int) *int         { return &v }

func insertFixture(t *testing.T, tx pgx.Tx, rows []fixtureRow) {
	t.Helper()
	ctx := context.Background()
	computedAt := time.Date(2099, 1, 2, 21, 0, 0, 0, time.UTC)
	scoredAt := computedAt.Add(90 * time.Second)
	for _, r := range rows {
		if r.inUniverse {
			if _, err := tx.Exec(ctx, `
INSERT INTO universe_symbols (symbol, exchange, name, is_eligible)
VALUES ($1, 'NASDAQ', $2, true)`, r.symbol, r.symbol+" Holdings"); err != nil {
				t.Fatalf("universe %s: %v", r.symbol, err)
			}
		}
		failures := r.failures
		if failures == nil {
			failures = []string{}
		}
		if _, err := tx.Exec(ctx, `
INSERT INTO momentum_features (ts, symbol, close, change_pct, rvol_20, dollar_volume, rsi_14,
    breakout_state, pct_of_52w_high, catalyst_tier, bucket, gates_passed, gate_failures, computed_at)
VALUES ($1, $2, 3.21, 12.5, $3, 1500000, 61.0, 'none', 0.42, $4, $5, $6, $7, $8)`,
			fixtureDay, r.symbol, r.rvol, r.catalystTier, r.bucket, r.passed, failures, computedAt); err != nil {
			t.Fatalf("features %s: %v", r.symbol, err)
		}
		if r.score != nil {
			if _, err := tx.Exec(ctx, `
INSERT INTO momentum_scores (ts, symbol, bucket, momentum_score_100, score_rvol, score_vol_accel,
    score_catalyst, score_float, score_vwap, score_breakout, score_52w,
    penalty_total, penalties, null_inputs, scored_at)
VALUES ($1, $2, $3, $4, 30.5, 25, 0, 10, 0, 0, 0, 5, '["exhausted_momentum_rsi_gt_85"]', '{catalyst_tier}', $5)`,
				fixtureDay, r.symbol, *r.bucket, *r.score, scoredAt); err != nil {
				t.Fatalf("scores %s: %v", r.symbol, err)
			}
		}
	}
}

// marketFixture: 12 market candidates (more than the frontend's window of 10),
// zero penny candidates, one penny gate failure, one candidate with no score
// row, one outside universe_symbols, and catalyst_tier null on most rows.
func marketFixture() []fixtureRow {
	var rows []fixtureRow
	for i := 1; i <= 12; i++ {
		r := fixtureRow{
			symbol: fmt.Sprintf("ZZM%02d", i), inUniverse: true, bucket: strp("market"), passed: true,
			rvol: f64p(float64(i)), score: intp(40 + i),
		}
		switch i {
		case 3:
			r.catalystTier = strp("B")
		case 5:
			r.score = nil // gate-passing but never scored
		case 7:
			r.rvol = nil // must sort last, not first
		case 9:
			r.inUniverse = false // missing directory row must not drop the candidate
		}
		rows = append(rows, r)
	}
	rows = append(rows, fixtureRow{
		symbol: "ZZP01", inUniverse: true, bucket: strp("penny"), passed: false,
		rvol: f64p(50), failures: []string{"rvol_20_below_min"},
	})
	return rows
}

func TestMomentumRead_LatestScanDateAndSummary(t *testing.T) {
	ctx := context.Background()
	tx := fixtureTx(t)

	var eligibleBefore int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM universe_symbols WHERE is_eligible`).Scan(&eligibleBefore); err != nil {
		t.Fatal(err)
	}
	insertFixture(t, tx, marketFixture())

	date, ok, err := LatestScanDate(ctx, tx)
	if err != nil || !ok || !date.Equal(fixtureDay) {
		t.Fatalf("LatestScanDate = %v %v %v, want %v", date, ok, err, fixtureDay)
	}
	s, err := GetScanSummary(ctx, tx, date)
	if err != nil {
		t.Fatal(err)
	}
	if s.UniverseScanned != 13 {
		t.Errorf("UniverseScanned = %d, want 13 (every features row for the day, pass or fail)", s.UniverseScanned)
	}
	if s.UniverseEligible != eligibleBefore+12 {
		t.Errorf("UniverseEligible = %d, want %d", s.UniverseEligible, eligibleBefore+12)
	}
	wantCompleted := time.Date(2099, 1, 2, 21, 1, 30, 0, time.UTC)
	if !s.CompletedAt.Equal(wantCompleted) {
		t.Errorf("CompletedAt = %v, want the later of computed_at and scored_at (%v)", s.CompletedAt, wantCompleted)
	}
}

func TestMomentumRead_Candidates(t *testing.T) {
	ctx := context.Background()
	tx := fixtureTx(t)
	insertFixture(t, tx, marketFixture())

	rows, err := Candidates(ctx, tx, fixtureDay)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 12 {
		t.Fatalf("got %d candidates, want all 12 market gate-passers uncapped and no penny rows", len(rows))
	}
	bySymbol := map[string]CandidateRow{}
	for i, r := range rows {
		if r.Bucket != "market" {
			t.Errorf("row %s in bucket %s; the only penny row failed its gates", r.Symbol, r.Bucket)
		}
		bySymbol[r.Symbol] = r
		if i > 0 && rows[i-1].RVol20 != nil && r.RVol20 != nil && *rows[i-1].RVol20 < *r.RVol20 {
			t.Errorf("not rvol_20 DESC at %d: %v then %v", i, *rows[i-1].RVol20, *r.RVol20)
		}
	}
	if rows[0].Symbol != "ZZM12" || rows[len(rows)-1].Symbol != "ZZM07" {
		t.Errorf("order = %s..%s, want ZZM12 first and the null-rvol ZZM07 last", rows[0].Symbol, rows[len(rows)-1].Symbol)
	}
	if r := bySymbol["ZZM05"]; r.MomentumScore != nil {
		t.Errorf("ZZM05 has no score row; MomentumScore = %v, want nil", *r.MomentumScore)
	}
	if r := bySymbol["ZZM01"]; r.MomentumScore == nil || *r.MomentumScore != 41 || r.CatalystTier != nil ||
		len(r.ScoreNullInputs) != 1 || r.ScoreNullInputs[0] != "catalyst_tier" {
		t.Errorf("ZZM01 = %+v", r)
	}
	if r := bySymbol["ZZM05"]; r.ScoreNullInputs != nil {
		t.Errorf("ZZM05 has no score row; ScoreNullInputs = %v, want nil", r.ScoreNullInputs)
	}
	if r := bySymbol["ZZM03"]; r.CatalystTier == nil || *r.CatalystTier != "B" {
		t.Errorf("ZZM03 catalyst_tier = %v, want B", r.CatalystTier)
	}
	if r, ok := bySymbol["ZZM09"]; !ok || r.Exchange != nil || r.CompanyName != nil {
		t.Errorf("ZZM09 (no universe row) = %+v, present=%v; want kept with null exchange/name", r, ok)
	}
	if r := bySymbol["ZZM02"]; r.Exchange == nil || *r.Exchange != "NASDAQ" || r.Close == nil || *r.Close != 3.21 {
		t.Errorf("ZZM02 = %+v", r)
	}
}

func TestMomentumRead_EmptyBucketDay(t *testing.T) {
	ctx := context.Background()
	tx := fixtureTx(t)
	insertFixture(t, tx, []fixtureRow{{
		symbol: "ZZP02", inUniverse: true, bucket: strp("penny"), passed: false,
		rvol: f64p(1), failures: []string{"change_pct_below_min"},
	}})
	date, ok, err := LatestScanDate(ctx, tx)
	if err != nil || !ok || !date.Equal(fixtureDay) {
		t.Fatalf("a day with zero candidates is still the latest scan: got %v %v %v", date, ok, err)
	}
	rows, err := Candidates(ctx, tx, date)
	if err != nil || len(rows) != 0 {
		t.Errorf("Candidates = %d rows, err %v; want an empty day, not an error", len(rows), err)
	}
}

func TestMomentumRead_SymbolDetail(t *testing.T) {
	ctx := context.Background()
	tx := fixtureTx(t)
	insertFixture(t, tx, marketFixture())

	d, ok, err := SymbolDetail(ctx, tx, "zzm01")
	if err != nil || !ok {
		t.Fatalf("SymbolDetail(zzm01) = %v %v", ok, err)
	}
	if d.Symbol != "ZZM01" || !d.GatesPassed || len(d.GateFailures) != 0 || d.Score == nil {
		t.Fatalf("detail = %+v", d)
	}
	sc := d.Score
	if sc.Total != 41 || sc.RVol == nil || *sc.RVol != 30.5 || sc.PenaltyTotal == nil || *sc.PenaltyTotal != 5 {
		t.Errorf("score = %+v", sc)
	}
	if len(sc.Penalties) != 1 || sc.Penalties[0] != "exhausted_momentum_rsi_gt_85" {
		t.Errorf("penalties = %v, want the jsonb array of reason names", sc.Penalties)
	}
	if len(sc.NullInputs) != 1 || sc.NullInputs[0] != "catalyst_tier" {
		t.Errorf("null_inputs = %v", sc.NullInputs)
	}

	failed, ok, err := SymbolDetail(ctx, tx, "ZZP01")
	if err != nil || !ok || failed.GatesPassed || failed.Score != nil ||
		len(failed.GateFailures) != 1 || failed.GateFailures[0] != "rvol_20_below_min" {
		t.Errorf("gate-failed detail = %+v %v %v", failed, ok, err)
	}

	if _, ok, err := SymbolDetail(ctx, tx, "ZZNONE"); err != nil || ok {
		t.Errorf("unknown symbol: ok=%v err=%v, want not found", ok, err)
	}
}

// momentum_scores.penalties defaults to the empty OBJECT '{}', while the writer
// stores an array. A row left at the default must read as "no penalties", not
// fail the detail endpoint.
func TestMomentumRead_DefaultPenaltiesObject(t *testing.T) {
	ctx := context.Background()
	tx := fixtureTx(t)
	if _, err := tx.Exec(ctx, `
INSERT INTO momentum_features (ts, symbol, rvol_20, bucket, gates_passed) VALUES ($1, 'ZZD01', 4, 'market', true);
`, fixtureDay); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO momentum_scores (ts, symbol, bucket, momentum_score_100) VALUES ($1, 'ZZD01', 'market', 50)`, fixtureDay); err != nil {
		t.Fatal(err)
	}
	d, ok, err := SymbolDetail(ctx, tx, "ZZD01")
	if err != nil || !ok || d.Score == nil {
		t.Fatalf("detail = %+v ok=%v err=%v", d, ok, err)
	}
	if len(d.Score.Penalties) != 0 {
		t.Errorf("penalties = %v, want none", d.Score.Penalties)
	}
}

// The detail row is the symbol's own newest row, even when that is older than
// the latest scan; a symbol with no row at all is not found.
func TestMomentumRead_SymbolDetailUsesTheSymbolsNewestRow(t *testing.T) {
	ctx := context.Background()
	tx := fixtureTx(t)
	older := time.Date(2099, 5, 1, 0, 0, 0, 0, time.UTC)
	newer := time.Date(2099, 5, 2, 0, 0, 0, 0, time.UTC)
	latest := time.Date(2099, 5, 9, 0, 0, 0, 0, time.UTC) // someone else's newer scan
	if _, err := tx.Exec(ctx, `
INSERT INTO momentum_features (ts, symbol, close, gates_passed) VALUES
 ($1, 'ZZDN1', 1.0, true), ($2, 'ZZDN1', 2.0, false), ($3, 'ZZDN2', 9.0, true)`, older, newer, latest); err != nil {
		t.Fatal(err)
	}
	d, ok, err := SymbolDetail(ctx, tx, "zzdn1")
	if err != nil || !ok || !d.TS.Equal(newer) || d.GatesPassed || d.Facts.Close == nil || *d.Facts.Close != 2.0 {
		t.Fatalf("detail = %+v ok=%v err=%v; want ZZDN1's own newest row (%s, close 2)", d, ok, err, newer.Format(time.DateOnly))
	}
}
