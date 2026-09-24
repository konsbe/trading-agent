//go:build integration

package store

import (
	"context"
	"testing"
	"time"
)

// Watchlist and price-bar reads against a real Postgres. Fixtures live in a
// transaction that is always rolled back (see fixtureTx).

func TestWatchlist_RoundTripAndIdempotency(t *testing.T) {
	ctx := context.Background()
	tx := fixtureTx(t)
	if _, err := tx.Exec(ctx, `INSERT INTO universe_symbols (symbol, exchange, name) VALUES ('ZZW01','NASDAQ','Watch One'), ('ZZW02','NYSE','Watch Two')`); err != nil {
		t.Fatal(err)
	}

	for _, s := range []string{"ZZW01", "zzw02"} {
		if added, err := AddToWatchlist(ctx, tx, nil, s); err != nil || !added {
			t.Fatalf("add %s: added=%v err=%v", s, added, err)
		}
	}
	if added, err := AddToWatchlist(ctx, tx, nil, "ZZW01"); err != nil || added {
		t.Errorf("repeat add: added=%v err=%v, want idempotent no-op (NULL owners must still be unique)", added, err)
	}

	items, err := ListWatchlist(ctx, tx, nil)
	if err != nil {
		t.Fatal(err)
	}
	mine := map[string]WatchlistItem{}
	for _, it := range items {
		mine[it.Symbol] = it
	}
	if it, ok := mine["ZZW02"]; !ok || it.CompanyName == nil || *it.CompanyName != "Watch Two" {
		t.Errorf("ZZW02 (added lower-case) = %+v present=%v", it, ok)
	}

	// Another owner's list is separate from the unauthenticated one.
	alice := "alice-sub"
	if _, err := AddToWatchlist(ctx, tx, &alice, "ZZW01"); err != nil {
		t.Fatal(err)
	}
	aliceItems, _ := ListWatchlist(ctx, tx, &alice)
	if len(aliceItems) != 1 || aliceItems[0].Symbol != "ZZW01" {
		t.Errorf("alice's list = %+v, want only her own row", aliceItems)
	}

	if removed, err := RemoveFromWatchlist(ctx, tx, nil, "ZZW01"); err != nil || !removed {
		t.Errorf("remove: removed=%v err=%v", removed, err)
	}
	if aliceItems, _ := ListWatchlist(ctx, tx, &alice); len(aliceItems) != 1 {
		t.Error("removing from the unauthenticated list touched alice's row")
	}
	if removed, _ := RemoveFromWatchlist(ctx, tx, nil, "ZZW01"); removed {
		t.Error("second remove reported a row")
	}

	known, err := SymbolKnown(ctx, tx, "zzw02")
	if err != nil || !known {
		t.Errorf("SymbolKnown(zzw02) = %v %v", known, err)
	}
	if known, _ := SymbolKnown(ctx, tx, "ZZNOPE"); known {
		t.Error("SymbolKnown(ZZNOPE) = true")
	}
}

func TestPriceBars_DailyDedupesBySourceRankAndIntradayBySession(t *testing.T) {
	ctx := context.Background()
	tx := fixtureTx(t)
	day := func(d int) time.Time { return time.Date(2099, 3, d, 0, 0, 0, 0, time.UTC) }
	for d := 2; d <= 6; d++ {
		if _, err := tx.Exec(ctx, `
INSERT INTO equity_ohlcv (ts, symbol, interval, source, open, high, low, close, volume)
VALUES ($1, 'ZZB01', '1Day', 'tiingo', 1, 2, 0.5, $2, 100)`, day(d), float64(d)); err != nil {
			t.Fatal(err)
		}
	}
	// A lower-ranked source on the same session must lose the tie.
	if _, err := tx.Exec(ctx, `
INSERT INTO equity_ohlcv (ts, symbol, interval, source, open, high, low, close, volume)
VALUES ($1, 'ZZB01', '1Day', 'yahoo_finance', 1, 2, 0.5, 999, 100)`, day(4)); err != nil {
		t.Fatal(err)
	}

	latest, ok, err := LatestDailyBarTS(ctx, tx, "zzb01")
	if err != nil || !ok || !latest.Equal(day(6)) {
		t.Fatalf("LatestDailyBarTS = %v %v %v", latest, ok, err)
	}
	bars, err := DailyBars(ctx, tx, "ZZB01", day(3))
	if err != nil || len(bars) != 4 {
		t.Fatalf("DailyBars from day 3 = %d bars, err %v; want 4 (one per session)", len(bars), err)
	}
	if bars[1].Close != 4 {
		t.Errorf("session 4 close = %v, want the tiingo bar (4), not yahoo's 999", bars[1].Close)
	}

	// Two New York sessions of 5-minute bars; ask for the latest one only.
	for _, ts := range []time.Time{
		time.Date(2099, 3, 5, 14, 30, 0, 0, time.UTC), time.Date(2099, 3, 5, 14, 35, 0, 0, time.UTC),
		time.Date(2099, 3, 6, 14, 30, 0, 0, time.UTC), time.Date(2099, 3, 6, 20, 55, 0, 0, time.UTC),
	} {
		if _, err := tx.Exec(ctx, `
INSERT INTO equity_ohlcv (ts, symbol, interval, source, open, high, low, close, volume)
VALUES ($1, 'ZZB01', '5Min', 'yahoo_finance', 1, 2, 0.5, 1.5, 10)`, ts); err != nil {
			t.Fatal(err)
		}
	}
	one, err := IntradayBars(ctx, tx, "ZZB01", "5Min", 1)
	if err != nil || len(one) != 2 || one[0].TS.Day() != 6 {
		t.Errorf("IntradayBars(1 session) = %d bars %v, err %v; want the 2 bars of Mar 6", len(one), one, err)
	}
	two, _ := IntradayBars(ctx, tx, "ZZB01", "5Min", 5)
	if len(two) != 4 {
		t.Errorf("IntradayBars(5 sessions) = %d bars, want all 4 across the 2 stored sessions", len(two))
	}
	none, err := IntradayBars(ctx, tx, "ZZB01", "15Min", 1)
	if err != nil || len(none) != 0 {
		t.Errorf("no 15Min bars stored: got %d, err %v", len(none), err)
	}
}
