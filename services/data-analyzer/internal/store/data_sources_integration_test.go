//go:build integration

package store

import (
	"context"
	"testing"
	"time"
)

// A budget whose stored window is not today's reports 0 used today — the
// limiter only rolls the window on its next request.
func TestProviderBudgets_StaleWindowIsZeroToday(t *testing.T) {
	ctx := context.Background()
	tx := fixtureTx(t)
	if _, err := tx.Exec(ctx, `
INSERT INTO api_rate_budget (budget_key, tokens, refill_per_sec, burst, daily_limit, daily_used, daily_window_start, daily_reset_tz) VALUES
 ('zz_today', 0, 2, 5, 1000, 40, (now() AT TIME ZONE 'UTC')::date, 'UTC'),
 ('zz_old',   0, 1, 2, NULL, 9000, (now() AT TIME ZONE 'UTC')::date - 1, 'UTC')`); err != nil {
		t.Fatal(err)
	}
	got, err := ProviderBudgets(ctx, tx, []string{"zz_today", "zz_old", "zz_absent"})
	if err != nil {
		t.Fatal(err)
	}
	by := map[string]ProviderBudget{}
	for _, b := range got {
		by[b.Key] = b
	}
	if len(got) != 2 {
		t.Fatalf("got %d rows, want 2 (absent keys are simply missing)", len(got))
	}
	if b := by["zz_today"]; !b.WindowIsCurrent || b.DailyUsed != 40 || b.DailyLimit == nil || *b.DailyLimit != 1000 {
		t.Errorf("current window = %+v", b)
	}
	if b := by["zz_old"]; b.WindowIsCurrent || b.DailyUsed != 0 || b.StoredDailyUsed != 9000 || b.DailyLimit != nil {
		t.Errorf("stale window = %+v; want 0 used today, stored 9000 kept, no limit", b)
	}
}

func TestChainRunsBetweenAndLastClean(t *testing.T) {
	ctx := context.Background()
	tx := fixtureTx(t)
	if _, err := tx.Exec(ctx, `DELETE FROM momentum_chain_runs`); err != nil { // inside the rolled-back fixture
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO momentum_chain_runs (session, attempts, scanner_completed_at, tracker_completed_at, gave_up_at, last_error) VALUES
 ('2099-06-01', 1, now(), now(), NULL, NULL),
 ('2099-06-02', 2, now(), now(), NULL, 'momentum-tracker: exit status 1'),
 ('2099-06-03', 3, NULL, NULL, now(), 'chain failed 3 times')`); err != nil {
		t.Fatal(err)
	}
	runs, first, ok, err := ChainRunsBetween(ctx, tx, time.Date(2099, 6, 2, 0, 0, 0, 0, time.UTC), time.Date(2099, 6, 9, 0, 0, 0, 0, time.UTC))
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	if !first.Equal(time.Date(2099, 6, 1, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("first recorded = %s, want 2099-06-01 (outside the range, still reported)", first)
	}
	if len(runs) != 2 || runs["2099-06-03"].GaveUpAt == nil || runs["2099-06-02"].Attempts != 2 {
		t.Errorf("runs = %+v", runs)
	}
	clean, err := LastCleanSession(ctx, tx)
	if err != nil || clean == nil || clean.UTC().Format(time.DateOnly) != "2099-06-01" {
		t.Errorf("last clean = %v err %v; a retried session is not clean", clean, err)
	}
}

func TestSessionCoverage_MatchesMomentumDailysDefinition(t *testing.T) {
	ctx := context.Background()
	tx := fixtureTx(t)
	s1 := time.Date(2099, 7, 1, 0, 0, 0, 0, time.UTC)
	s2 := time.Date(2099, 7, 2, 0, 0, 0, 0, time.UTC)
	if _, err := tx.Exec(ctx, `
INSERT INTO universe_symbols (symbol, exchange, name, is_eligible) VALUES ('ZZC01','NASDAQ','Cov One', true), ('ZZC02','NASDAQ','Cov Two', false);
`); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO equity_ohlcv (symbol, interval, source, ts, open, high, low, close, volume) VALUES
 ('ZZC01','1Day','tiingo',$1,1,1,1,1,1),
 ('ZZC02','1Day','tiingo',$1,1,1,1,1,1)`, s1); err != nil {
		t.Fatal(err)
	}
	var scannable float64
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM universe_symbols WHERE is_eligible AND data_unavailable_reason IS NULL`).Scan(&scannable); err != nil {
		t.Fatal(err)
	}
	cov, err := SessionCoverage(ctx, tx, []time.Time{s1, s2}, "tiingo")
	if err != nil {
		t.Fatal(err)
	}
	// Only the eligible ZZC01 counts; ZZC02 is not scannable.
	if want := 1 / scannable * 100; cov["2099-07-01"] != want {
		t.Errorf("coverage 07-01 = %v, want %v (1 of %v scannable)", cov["2099-07-01"], want, scannable)
	}
	if cov["2099-07-02"] != 0 {
		t.Errorf("coverage 07-02 = %v, want 0", cov["2099-07-02"])
	}
}
