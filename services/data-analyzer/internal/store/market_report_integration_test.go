//go:build integration

package store

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

// Every report query runs against the real schema, and the watchlist merges
// into the instrument and earnings lists.
func TestLoadMarketReport_RealSchema(t *testing.T) {
	ctx := context.Background()
	tx := fixtureTx(t)
	if _, err := tx.Exec(ctx, `INSERT INTO universe_symbols (symbol, exchange, name) VALUES ('ZZMR1','NASDAQ','Report One')`); err != nil {
		t.Fatal(err)
	}
	if _, err := AddToWatchlist(ctx, tx, nil, "ZZMR1"); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO earnings_calendar_events (earnings_date, symbol, quarter, source, external_id)
VALUES (current_date + 3, 'ZZMR1', '3', 'finnhub', 'zzmr1-test')`); err != nil {
		t.Fatal(err)
	}
	fixed := []InstrumentRef{{"SPY", "equity"}, {"BTCUSDT", "crypto"}}
	in, err := LoadMarketReport(ctx, tx, fixed, []string{"SHEL"}, []string{"VIXCLS", "DGS10"}, time.Now().UTC())
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if _, ok := in.Closes["ZZMR1"]; !ok {
		t.Error("the watchlist symbol was not added to the instrument list")
	}
	var earnings []map[string]any
	if err := json.Unmarshal(in.Earnings, &earnings); err != nil {
		t.Fatalf("earnings json: %v (%s)", err, in.Earnings)
	}
	found := false
	for _, e := range earnings {
		if e["symbol"] == "ZZMR1" {
			found = true
		}
	}
	if !found {
		t.Errorf("watchlist symbol's earnings missing: %s", in.Earnings)
	}
	for name, raw := range map[string]json.RawMessage{"economic": in.Economic, "headlines": in.Headlines} {
		if !json.Valid(raw) || raw[0] != '[' {
			t.Errorf("%s is not a JSON array: %s", name, raw)
		}
	}
}

// Live 2026-09-27: the report's 8 headlines were 4 links, each stored again
// on every re-fetch (377 rows, 180 distinct urls), and React warned about
// duplicate keys. One row per link, the newest.
func TestMacroHeadlines_OneRowPerLink(t *testing.T) {
	ctx := context.Background()
	tx := fixtureTx(t)
	for _, r := range []struct {
		hoursAhead int
		url, head  string
	}{
		{3, "https://zz.example/a", "A, newest copy"}, {2, "https://zz.example/a", "A, older copy"},
		{2, "https://zz.example/b", "B"}, {1, "https://zz.example/a", "A, oldest copy"},
	} {
		if _, err := tx.Exec(ctx, `INSERT INTO news_headlines (ts, source, headline, url)
			VALUES (now() + make_interval(hours => $1), 'finnhub_macro_general', $2, $3)`, r.hoursAhead, r.head, r.url); err != nil {
			t.Fatal(err)
		}
	}
	raw, err := MacroHeadlines(ctx, tx, 8)
	if err != nil {
		t.Fatal(err)
	}
	var got []struct{ Headline, URL string }
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, h := range got {
		if seen[h.URL] {
			t.Errorf("url %s listed twice: %s", h.URL, raw)
		}
		seen[h.URL] = true
	}
	if len(got) < 2 || got[0].Headline != "A, newest copy" || got[1].Headline != "B" {
		t.Errorf("first rows = %+v, want A's newest copy then B", got[:min(len(got), 2)])
	}
}
