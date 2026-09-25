//go:build integration

package store

import (
	"context"
	"testing"
	"time"
)

func TestLoadAnalysis_LatestRowsHeadlinesMacroAndVIX(t *testing.T) {
	ctx := context.Background()
	tx := fixtureTx(t)
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := tx.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}
	d1 := time.Date(2098, 3, 2, 13, 30, 0, 0, time.UTC)
	d2 := d1.AddDate(0, 0, 1)
	exec(`INSERT INTO technical_indicators (ts, symbol, exchange, interval, indicator, value, payload) VALUES
		($1, 'ZZLA01', 'equity', '1Day', 'rsi_14', 40, NULL),
		($2, 'ZZLA01', 'equity', '1Day', 'rsi_14', 41, NULL),
		($2, 'ZZLA01', 'binance', '1Day', 'rsi_14', 99, NULL),
		($1, 'ZZLA01', 'equity', '1Day', 'trend', 0.1, '{"direction": "up"}')`, d1, d2)
	exec(`INSERT INTO equity_fundamentals (ts, symbol, period, metric, value, payload, source) VALUES
		($1, 'ZZLA01', 'derived', 'eps_strength', 0, '{"tier": "neutral"}', 'fundamental_analysis'),
		($2, 'ZZLA01', 'derived', 'eps_strength', 1, '{"tier": "strong"}', 'fundamental_analysis'),
		($1, 'ZZLA01', 'ttm', 'market_cap', 1e9, NULL, 'finnhub_metric'),
		($2, 'ZZLA01', 'ttm', 'market_cap', 2e9, NULL, 'finnhub_metric')`, d1, d2)
	exec(`INSERT INTO news_headlines (ts, source, symbol, headline, url, sentiment) VALUES
		($1, 'zz', 'ZZLA01', 'older', NULL, NULL), ($2, 'zz', 'ZZLA01', 'newer', 'https://x.test', 0.25)`, d1, d2)
	exec(`INSERT INTO macro_derived (ts, metric, value, payload, source) VALUES
		($1, 'mc_price_phase:ZZLA01', NULL, '{"price_phase": "bear"}', 'macro_analysis'),
		($1, 'mc_price_phase:ZZLA01X', NULL, '{"price_phase": "bull"}', 'macro_analysis')`, d2)
	// Only an old VIX: the recent-chunks read is empty, the fallback finds it.
	exec(`DELETE FROM macro_fred WHERE series_id = 'VIXCLS'`)
	exec(`INSERT INTO macro_fred (ts, series_id, value) VALUES (now() - interval '100 days', 'VIXCLS', 33.5)`)

	in, err := LoadAnalysis(ctx, tx, "ZZLA01", "1Day", 10)
	if err != nil {
		t.Fatal(err)
	}
	if r := in.Indicators["rsi_14"]; r.Value == nil || *r.Value != 41 || !r.TS.Equal(d2) {
		t.Errorf("rsi_14 = %+v; want the newest equity row", r)
	}
	if string(in.Indicators["trend"].Payload) != `{"direction": "up"}` {
		t.Errorf("trend payload = %s", in.Indicators["trend"].Payload)
	}
	if r := in.Derived["eps_strength"]; r.Value == nil || *r.Value != 1 {
		t.Errorf("eps_strength = %+v", r)
	}
	if in.MarketCap == nil || *in.MarketCap != 2e9 {
		t.Errorf("market cap = %v", in.MarketCap)
	}
	if in.VIX == nil || *in.VIX != 33.5 {
		t.Errorf("vix = %v; want the newest VIXCLS even outside the recent window", in.VIX)
	}
	if len(in.Headlines) != 2 || in.Headlines[0].Title != "newer" || *in.Headlines[0].Sentiment != 0.25 || in.Headlines[1].URL != nil {
		t.Errorf("headlines = %+v", in.Headlines)
	}
	if _, ok := in.Macro["mc_price_phase:ZZLA01"]; !ok || len(in.Macro) > 3 {
		t.Errorf("macro keys = %v", in.Macro)
	}
	if _, ok := in.Macro["mc_price_phase:ZZLA01X"]; ok {
		t.Error("a prefix match on another symbol leaked in")
	}

	fr, err := AnalysisFreshnessFor(ctx, tx, "ZZLA01", "1Day")
	if err != nil {
		t.Fatal(err)
	}
	if fr.LatestBarTS != nil || fr.TechnicalTS == nil || !fr.TechnicalTS.Equal(d2) || !fr.DerivedTS.Equal(d2) || !fr.RawTS.Equal(d2) {
		t.Errorf("freshness = %+v", fr)
	}
}

// One bar per session: when two sources wrote the same session at different
// timestamps, the latest bar is the ranked one, the ts the worker writes under.
func TestAnalysisFreshness_LatestBarIsTheWorkersAnchor(t *testing.T) {
	ctx := context.Background()
	tx := fixtureTx(t)
	day := time.Date(2098, 4, 1, 0, 0, 0, 0, time.UTC)
	if _, err := tx.Exec(ctx, `INSERT INTO equity_ohlcv (ts, symbol, interval, open, high, low, close, volume, source) VALUES
		($1, 'ZZLA02', '1Day', 1, 1, 1, 1, 1, 'tiingo'),
		($2, 'ZZLA02', '1Day', 1, 1, 1, 1, 1, 'yahoo_finance')`, day, day.Add(13*time.Hour+30*time.Minute)); err != nil {
		t.Fatal(err)
	}
	fr, err := AnalysisFreshnessFor(ctx, tx, "ZZLA02", "1Day")
	if err != nil {
		t.Fatal(err)
	}
	bars, _ := QueryEquityBars(ctx, tx, "ZZLA02", "1Day", 500)
	if fr.LatestBarTS == nil || len(bars) != 1 || !fr.LatestBarTS.Equal(bars[0].TS) || !fr.LatestBarTS.Equal(day) {
		t.Errorf("latest bar = %v, loader = %v", fr.LatestBarTS, bars)
	}
}
