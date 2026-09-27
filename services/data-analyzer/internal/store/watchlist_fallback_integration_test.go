//go:build integration

package store

import (
	"context"
	"testing"
	"time"
)

// Adding opens the watchlist reason in the same transaction (with the fund
// asset type for an ETP); removing the last list's row closes it, kept as history.
func TestWatchlist_AddOpensReasonRemoveClosesIt(t *testing.T) {
	ctx := context.Background()
	tx := fixtureTx(t)
	if _, err := tx.Exec(ctx, `INSERT INTO universe_symbols (symbol, exchange, name, type) VALUES
		('ZZWR1','NYSE','Fund','ETP'), ('ZZWR2','NYSE','Adr','ADR')`); err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{"zzwr1", "ZZWR2"} {
		if _, err := AddToWatchlist(ctx, tx, nil, s); err != nil {
			t.Fatal(err)
		}
	}
	alice := "alice"
	if _, err := AddToWatchlist(ctx, tx, &alice, "ZZWR2"); err != nil {
		t.Fatal(err)
	}
	open := func(sym string) (assetType string, n int) {
		_ = tx.QueryRow(ctx, `SELECT min(asset_type), count(*) FROM computation_interest
			WHERE symbol = $1 AND reason = 'watchlist' AND active_until IS NULL`, sym).Scan(&assetType, &n)
		return
	}
	if at, n := open("ZZWR1"); at != "etf" || n != 1 {
		t.Errorf("ZZWR1 open reason = %q x%d, want etf x1", at, n)
	}
	if at, n := open("ZZWR2"); at != "equity" || n != 1 {
		t.Errorf("ZZWR2 open reason = %q x%d, want equity x1 (a second list must not open a second row)", at, n)
	}
	if _, err := RemoveFromWatchlist(ctx, tx, nil, "ZZWR2"); err != nil {
		t.Fatal(err)
	}
	if _, n := open("ZZWR2"); n != 1 {
		t.Errorf("ZZWR2 is still on alice's list: reason must stay open, got %d", n)
	}
	if _, err := RemoveFromWatchlist(ctx, tx, &alice, "ZZWR2"); err != nil {
		t.Fatal(err)
	}
	var openN, closedN int
	_ = tx.QueryRow(ctx, `SELECT count(*) FILTER (WHERE active_until IS NULL), count(*) FILTER (WHERE active_until IS NOT NULL)
		FROM computation_interest WHERE symbol = 'ZZWR2' AND reason = 'watchlist'`).Scan(&openN, &closedN)
	if openN != 0 || closedN != 1 {
		t.Errorf("after the last removal: open %d closed %d, want 0 open, 1 kept", openN, closedN)
	}
}

// The fallback reads one bar per session (Tiingo over Yahoo), runs the
// scanner's feature code on it, and takes the newest market cap row even when
// the non-USD guard nulled it.
func TestLoadBarFallback(t *testing.T) {
	ctx := context.Background()
	tx := fixtureTx(t)
	start := time.Date(2098, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 30; i++ {
		day := start.AddDate(0, 0, i)
		c := 100 + float64(i)
		if _, err := tx.Exec(ctx, `INSERT INTO equity_ohlcv (symbol, interval, ts, open, high, low, close, volume, source)
			VALUES ('ZZFB', '1Day', $1, $2, $2, $2, $2, 1000, 'yahoo_finance')`, day.Add(13*time.Hour+30*time.Minute), c); err != nil {
			t.Fatal(err)
		}
	}
	last := start.AddDate(0, 0, 29)
	// Same session from Tiingo: it must win over Yahoo's bar.
	if _, err := tx.Exec(ctx, `INSERT INTO equity_ohlcv (symbol, interval, ts, open, high, low, close, volume, source)
		VALUES ('ZZFB', '1Day', $1, 140, 140, 140, 140, 3000, 'tiingo')`, last); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO equity_fundamentals (ts, symbol, period, metric, value, payload, source) VALUES
		($1, 'ZZFB', 'ttm', 'market_cap', 5e9, '{}', 'finnhub_metric'),
		($2, 'ZZFB', 'ttm', 'market_cap', NULL, '{"note":"not in USD (TWD)"}', 'finnhub_metric')`,
		start, last); err != nil {
		t.Fatal(err)
	}
	fb, err := LoadBarFallback(ctx, tx, "zzfb")
	if err != nil || fb == nil {
		t.Fatalf("fallback = %v, %v", fb, err)
	}
	f := fb.Features
	if fb.Source != "tiingo" || !fb.AsOf.Equal(last) || f.BarsAvailable != 30 || *f.Close != 140 || *f.Volume != 3000 {
		t.Errorf("latest bar: source %s as_of %v bars %d close %v volume %v", fb.Source, fb.AsOf, f.BarsAvailable, *f.Close, *f.Volume)
	}
	if f.ChangePct == nil || *f.ChangePct != (140.0/128-1)*100 {
		t.Errorf("change_pct = %v, want from the prior session's close 128", f.ChangePct)
	}
	if f.RVol20 == nil || *f.RVol20 != 3 || f.RSI14 == nil || f.BreakoutState == nil || f.PctOf52wHigh != nil {
		t.Errorf("rvol %v rsi %v breakout %v pct52w %v (30 bars: no 52-week window)", f.RVol20, f.RSI14, f.BreakoutState, f.PctOf52wHigh)
	}
	if fb.MarketCap != nil || fb.MarketCapNote == nil || *fb.MarketCapNote != "not in USD (TWD)" {
		t.Errorf("market cap = %v note %v, want the newest (nulled) row, never the older 5e9", fb.MarketCap, fb.MarketCapNote)
	}
	if none, err := LoadBarFallback(ctx, tx, "ZZFBNONE"); err != nil || none != nil {
		t.Errorf("no bars = %v, %v", none, err)
	}
}
