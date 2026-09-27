//go:build integration

package store

import (
	"context"
	"testing"
	"time"
)

// ListAlerts over real SQL, fixtures in a rolled-back transaction. The fixture
// dates are in 2099 so they are the newest rows in the table.
func TestAlerts_ListAlerts(t *testing.T) {
	ctx := context.Background()
	tx := fixtureTx(t)
	base := time.Date(2099, 1, 2, 12, 0, 0, 0, time.UTC)
	for _, a := range []struct {
		symbol, exchange, kind string
		value                  *float64
		at                     time.Time
	}{
		{"ZZAL1", "equity", "bb_squeeze", nil, base.Add(-26 * time.Hour)},
		{"ZZAL1", "equity", "liquidity_sweep", f64p(4), base},
		{"ZZAL2USDT", "crypto", "rsi_overbought", f64p(71.5), base.Add(-time.Hour)},
		{"ZZAL1", "equity", "rsi_overbought", f64p(75.1), base.Add(time.Hour)},
	} {
		if _, err := tx.Exec(ctx, `
INSERT INTO fired_alerts (symbol, exchange_type, alert_type, interval, value, severity, message, fired_at)
VALUES ($1, $2, $3, '1Day', $4, 'notice', $3 || ' msg', $5)`, a.symbol, a.exchange, a.kind, a.value, a.at); err != nil {
			t.Fatal(err)
		}
	}
	symbols := func(rows []AlertRow) (out []string) {
		for _, r := range rows {
			out = append(out, r.Symbol+":"+r.AlertType)
		}
		return out
	}

	sym := "ZZAL1"
	rows, err := ListAlerts(ctx, tx, AlertFilter{Symbol: &sym, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if got := symbols(rows); len(got) != 3 || got[0] != "ZZAL1:rsi_overbought" || got[2] != "ZZAL1:bb_squeeze" {
		t.Errorf("symbol filter = %v, want ZZAL1's three, newest first", got)
	}
	if r := rows[0]; r.ExchangeType != "equity" || r.Interval != "1Day" || r.Value == nil || *r.Value != 75.1 ||
		r.Severity != "notice" || r.Message != "rsi_overbought msg" || !r.FiredAt.Equal(base.Add(time.Hour)) || r.FiredAt.Location() != time.UTC {
		t.Errorf("row = %+v", r)
	}
	if rows[2].Value != nil {
		t.Errorf("null value = %v, want nil", *rows[2].Value)
	}

	since := time.Date(2099, 1, 2, 0, 0, 0, 0, time.UTC)
	rows, err = ListAlerts(ctx, tx, AlertFilter{Since: &since, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if got := symbols(rows); len(got) != 3 || got[0] != "ZZAL1:rsi_overbought" || got[1] != "ZZAL1:liquidity_sweep" || got[2] != "ZZAL2USDT:rsi_overbought" {
		t.Errorf("since filter = %v, want the three since the day's start, every exchange, newest first", got)
	}

	rows, err = ListAlerts(ctx, tx, AlertFilter{Symbol: &sym, Since: &since, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if got := symbols(rows); len(got) != 1 || got[0] != "ZZAL1:rsi_overbought" {
		t.Errorf("combined with limit 1 = %v", got)
	}

	if _, err := ListAlerts(ctx, tx, AlertFilter{}); err == nil {
		t.Error("an unbounded read must be refused")
	}
}

// Type, severity, until and the (fired_at, id) cursor over real SQL; grouping
// by symbol + alert type with count, span and the latest row; the table-wide
// meta. Fixture dates in 2099: the newest rows in the table.
func TestAlerts_FiltersGroupsCursorAndMeta(t *testing.T) {
	ctx := context.Background()
	tx := fixtureTx(t)
	base := time.Date(2099, 3, 1, 12, 0, 0, 0, time.UTC)
	ids := map[string]int64{}
	for _, a := range []struct {
		key, symbol, kind, severity string
		value                       float64
		at                          time.Time
	}{
		{"s1", "ZZAG1", "liquidity_sweep", "notice", 2, base.Add(-48 * time.Hour)},
		{"s2", "ZZAG1", "liquidity_sweep", "notice", 3, base.Add(-24 * time.Hour)},
		{"s3", "ZZAG1", "liquidity_sweep", "notice", 5, base},
		{"b1", "ZZAG1", "bb_squeeze", "info", 1, base.Add(-time.Hour)},
		{"f1", "ZZAG2", "fa_tier_flip", "warning", 0, base.Add(-2 * time.Hour)},
	} {
		var id int64
		if err := tx.QueryRow(ctx, `
INSERT INTO fired_alerts (symbol, exchange_type, alert_type, interval, value, severity, message, fired_at)
VALUES ($1, 'equity', $2, '1Day', $3::numeric, $4, $2 || ' ' || $3::numeric::text, $5) RETURNING id`, a.symbol, a.kind, a.value, a.severity, a.at).Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids[a.key] = id
	}
	since := base.Add(-72 * time.Hour)
	f := AlertFilter{Since: &since, Limit: 10}

	rows, err := ListAlerts(ctx, tx, AlertFilter{Since: &since, AlertTypes: []string{"bb_squeeze", "fa_tier_flip"}, Limit: 10})
	if err != nil || len(rows) != 2 || rows[0].ID != ids["b1"] || rows[1].ID != ids["f1"] {
		t.Errorf("alert_type filter = %+v, %v", rows, err)
	}
	rows, _ = ListAlerts(ctx, tx, AlertFilter{Since: &since, Severities: []string{"warning"}, Limit: 10})
	if len(rows) != 1 || rows[0].ID != ids["f1"] {
		t.Errorf("severity filter = %+v", rows)
	}
	until := base.Add(-12 * time.Hour)
	rows, _ = ListAlerts(ctx, tx, AlertFilter{Since: &since, Until: &until, Limit: 10})
	if len(rows) != 2 || rows[0].ID != ids["s2"] || rows[1].ID != ids["s1"] {
		t.Errorf("until (exclusive) = %+v", rows)
	}
	before := ids["b1"]
	rows, _ = ListAlerts(ctx, tx, AlertFilter{Since: &since, Before: &before, Limit: 10})
	if len(rows) != 3 || rows[0].ID != ids["f1"] || rows[2].ID != ids["s1"] {
		t.Errorf("cursor = %+v, want the rows older than b1", rows)
	}

	groups, err := ListAlertGroups(ctx, tx, f)
	if err != nil || len(groups) != 3 {
		t.Fatalf("groups = %+v, %v", groups, err)
	}
	g := groups[0]
	if g.Symbol != "ZZAG1" || g.AlertType != "liquidity_sweep" || g.Count != 3 || !g.FirstFiredAt.Equal(base.Add(-48*time.Hour)) ||
		!g.LastFiredAt.Equal(base) || g.Latest.ID != ids["s3"] || g.Latest.Value == nil || *g.Latest.Value != 5 ||
		g.Latest.Message != "liquidity_sweep 5" || g.Latest.Symbol != "ZZAG1" || g.Latest.ExchangeType != "equity" {
		t.Errorf("sweep group = %+v", g)
	}
	if groups[1].AlertType != "bb_squeeze" || groups[2].AlertType != "fa_tier_flip" {
		t.Errorf("order = %s, %s (newest latest row first)", groups[1].AlertType, groups[2].AlertType)
	}
	cursor := groups[0].Latest.ID
	page2, _ := ListAlertGroups(ctx, tx, AlertFilter{Since: &since, Before: &cursor, Limit: 10})
	if len(page2) != 2 || page2[0].AlertType != "bb_squeeze" {
		t.Errorf("grouped cursor page = %+v", page2)
	}
	// The range bounds the group: a count is of the alerts in range only.
	narrow := base.Add(-30 * time.Hour)
	gs, _ := ListAlertGroups(ctx, tx, AlertFilter{Since: &narrow, Symbol: strp("ZZAG1"), AlertTypes: []string{"liquidity_sweep"}, Limit: 10})
	if len(gs) != 1 || gs[0].Count != 2 || !gs[0].FirstFiredAt.Equal(base.Add(-24*time.Hour)) {
		t.Errorf("range-bounded group = %+v", gs)
	}

	meta, err := LoadAlertMeta(ctx, tx)
	if err != nil || meta.EarliestFired == nil || len(meta.Types) == 0 {
		t.Fatalf("meta = %+v, %v", meta, err)
	}
	for _, want := range []string{"bb_squeeze", "fa_tier_flip", "liquidity_sweep"} {
		found := false
		for _, t := range meta.Types {
			found = found || t == want
		}
		if !found {
			t.Errorf("meta types %v missing %s", meta.Types, want)
		}
	}
}

