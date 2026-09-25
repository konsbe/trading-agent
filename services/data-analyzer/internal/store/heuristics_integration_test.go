//go:build integration

package store

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/konsbe/trading-agent/services/data-analyzer/internal/heuristics"
)

// End to end on real Postgres, inside a rolled-back transaction: stored bars →
// live loader → ReplaySymbol → migration-027 tables. Lockbox rows must land
// with their signal columns and every label column NULL, non-lockbox rows with
// labels, and a re-run must replace rather than duplicate.
func TestReplaceHeuristicSymbol_LockboxRowsStoredUnlabelled(t *testing.T) {
	ctx := context.Background()
	tx := fixtureTx(t)
	const sym = "HEURTEST"

	// ~2.3 years of weekday bars spanning the region start (2025-03-28).
	day := time.Date(2023, 6, 1, 0, 0, 0, 0, time.UTC)
	price := 50.0
	for n := 0; n < 600; day = day.AddDate(0, 0, 1) {
		if wd := day.Weekday(); wd == time.Saturday || wd == time.Sunday {
			continue
		}
		o := price
		price *= 1 + 0.02*math.Sin(float64(n)/5) + 0.01*math.Cos(float64(n)/2.3)
		hi, lo := math.Max(o, price)*1.01, math.Min(o, price)*0.99
		if _, err := tx.Exec(ctx, `
			INSERT INTO equity_ohlcv (ts, symbol, interval, open, high, low, close, volume, source)
			VALUES ($1, $2, '1Day', $3, $4, $5, $6, 1e5, 'tiingo')`, day, sym, o, hi, lo, price); err != nil {
			t.Fatalf("insert bar: %v", err)
		}
		n++
	}

	lb, err := QueryHeuristicLockbox(ctx, tx, "phase2_lockbox_v2")
	if err != nil {
		t.Fatal(err)
	}
	if lb.Pilot[sym] {
		t.Fatalf("%s must not be a pilot symbol", sym)
	}
	bars, err := QueryEquityBars(ctx, tx, sym, "1Day", 1_000_000)
	if err != nil || len(bars) != 600 {
		t.Fatalf("loader returned %d bars, err %v", len(bars), err)
	}
	res := heuristics.ReplaySymbol(sym, bars, heuristics.VIXSeries{}, lb, heuristics.DefaultReplayConfig())

	n1, err := ReplaceHeuristicSymbol(ctx, tx, sym, "test-v1", res)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	n2, err := ReplaceHeuristicSymbol(ctx, tx, sym, "test-v1", res)
	if err != nil || n2 != n1 {
		t.Fatalf("re-run wrote %d rows (err %v), first run %d", n2, err, n1)
	}

	var total, lockbox, lockboxLabelled, openLabelled int
	if err := tx.QueryRow(ctx, `
		SELECT count(*),
		       count(*) FILTER (WHERE in_lockbox),
		       count(*) FILTER (WHERE in_lockbox AND (fwd_return_5s IS NOT NULL OR fwd_return_10s IS NOT NULL
		                         OR fwd_return_20s IS NOT NULL OR fwd_abs_move_5s IS NOT NULL
		                         OR fwd_abs_move_10s IS NOT NULL OR fwd_abs_move_20s IS NOT NULL
		                         OR label_complete IS NOT NULL)),
		       count(*) FILTER (WHERE NOT in_lockbox AND label_complete IS NOT NULL)
		FROM heuristic_comparison_days WHERE symbol = $1`, sym).
		Scan(&total, &lockbox, &lockboxLabelled, &openLabelled); err != nil {
		t.Fatal(err)
	}
	if total != len(res.Comparison) {
		t.Errorf("comparison rows = %d, want %d (re-run must not duplicate)", total, len(res.Comparison))
	}
	if lockbox == 0 || lockbox == total {
		t.Fatalf("fixture must straddle the region: %d of %d lockbox", lockbox, total)
	}
	if lockboxLabelled != 0 {
		t.Errorf("%d lockbox comparison rows carry a label", lockboxLabelled)
	}
	if openLabelled != total-lockbox {
		t.Errorf("non-lockbox rows labelled = %d, want %d", openLabelled, total-lockbox)
	}

	// The same property for every episode table.
	for _, sig := range heuristics.Signals {
		var bad int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM `+sig.EpisodeTable()+`
			WHERE symbol = $1 AND in_lockbox AND (fwd_return_10s IS NOT NULL OR label_complete IS NOT NULL)`, sym).Scan(&bad); err != nil {
			t.Fatal(err)
		}
		if bad != 0 {
			t.Errorf("%s: %d lockbox episodes carry labels", sig, bad)
		}
	}

	// And the schema refuses a labelled lockbox row outright.
	if _, err := tx.Exec(ctx, `SAVEPOINT chk`); err != nil {
		t.Fatal(err)
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO heuristic_comparison_days (symbol, t, close_t,
		    fired_rsi_overbought, fired_rsi_oversold, fired_macd_bull_cross, fired_macd_bear_cross,
		    fired_bb_squeeze, fired_bearish_pattern, fired_bullish_pattern, fired_low_sweep_reclaim,
		    fired_high_sweep_reject, fired_buy_watch_c4, fired_trim_watch_c4,
		    fwd_return_10s, in_lockbox, harness_version)
		VALUES ('HEURCHK', '2025-06-02', 1, false, false, false, false, false, false, false, false, false, false, false,
		        1.0, true, 'test')`)
	if err == nil {
		t.Error("a lockbox row with a label must violate the CHECK constraint")
	}
	_, _ = tx.Exec(ctx, `ROLLBACK TO SAVEPOINT chk`)
}
