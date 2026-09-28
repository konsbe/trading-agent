//go:build integration

package store

import (
	"context"
	"fmt"
	"slices"
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
INSERT INTO fired_alerts (symbol, exchange_type, alert_type, interval, value, severity, message, fired_at, bar_date)
VALUES ($1, $2, $3, '1Day', $4, 'notice', $3 || ' msg', $5, ($5::timestamptz AT TIME ZONE 'UTC')::date)`, a.symbol, a.exchange, a.kind, a.value, a.at); err != nil {
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
INSERT INTO fired_alerts (symbol, exchange_type, alert_type, interval, value, severity, message, fired_at, bar_date)
VALUES ($1, 'equity', $2, '1Day', $3::numeric, $4, $2 || ' ' || $3::numeric::text, $5, ($5::timestamptz AT TIME ZONE 'UTC')::date) RETURNING id`, a.symbol, a.kind, a.value, a.severity, a.at).Scan(&id); err != nil {
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

// Migration 031: one row per symbol, alert type and bar; a new row needs a
// bar_date; OnsetsSince is the first row that has one.
func TestAlerts_OneRowPerOnsetBar(t *testing.T) {
	ctx := context.Background()
	tx := fixtureTx(t)
	ins := func(bar any, at time.Time) error {
		_, err := tx.Exec(ctx, `SAVEPOINT s1`)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `
INSERT INTO fired_alerts (symbol, exchange_type, alert_type, interval, severity, message, fired_at, bar_date)
VALUES ('ZZOB1', 'equity', 'bb_squeeze', '1Day', 'info', 'Bollinger Squeeze began', $1, $2)`, at, bar)
		if err != nil {
			_, _ = tx.Exec(ctx, `ROLLBACK TO SAVEPOINT s1`)
		}
		return err
	}
	bar := time.Date(2099, 5, 4, 0, 0, 0, 0, time.UTC)
	at := time.Date(2099, 5, 4, 23, 0, 0, 0, time.UTC)
	if err := ins(bar, at); err != nil {
		t.Fatal(err)
	}
	if err := ins(bar, at.Add(4*time.Hour)); err == nil {
		t.Error("a second row for the same symbol, alert type and bar must be rejected")
	}
	if err := ins(nil, at.Add(8*time.Hour)); err == nil {
		t.Error("a new row without bar_date must be rejected")
	}
	rows, err := ListAlerts(ctx, tx, AlertFilter{Symbol: strp("ZZOB1"), Limit: 5})
	if err != nil || len(rows) != 1 || rows[0].BarDate == nil || !rows[0].BarDate.Equal(bar) {
		t.Fatalf("rows = %+v, %v", rows, err)
	}
	meta, err := LoadAlertMeta(ctx, tx)
	if err != nil || meta.OnsetsSince == nil {
		t.Fatalf("meta = %+v, %v", meta, err)
	}
}

// Sorted and searched views over real SQL: severity by rank, ties by symbol,
// the search across symbol / message / label-resolved types, offset pages,
// and grouped count ordering.
func TestAlerts_SortSearchOffset(t *testing.T) {
	ctx := context.Background()
	tx := fixtureTx(t)
	base := time.Date(2099, 4, 1, 12, 0, 0, 0, time.UTC)
	for i, a := range []struct{ sym, kind, sev, msg string }{
		{"ZZSB", "liquidity_sweep", "notice", "New liquidity sweep"},
		{"ZZSA", "bb_squeeze", "info", "Bollinger Squeeze began"},
		{"ZZSC", "vix_elevated", "warning", "VIX crossed above 25"},
		{"ZZSA", "liquidity_sweep", "notice", "New liquidity sweep again"},
	} {
		at := base.Add(time.Duration(i) * time.Hour)
		if _, err := tx.Exec(ctx, `INSERT INTO fired_alerts (symbol, exchange_type, alert_type, interval, severity, message, fired_at, bar_date)
			VALUES ($1, 'equity', $2, '1Day', $3, $4, $5, ($5::timestamptz AT TIME ZONE 'UTC')::date + $6::int)`, a.sym, a.kind, a.sev, a.msg, at, i); err != nil {
			t.Fatal(err)
		}
	}
	since := base.Add(-time.Hour)
	syms := func(rows []AlertRow) (out []string) {
		for _, r := range rows {
			out = append(out, r.Symbol+":"+r.Severity)
		}
		return
	}
	rows, err := ListAlerts(ctx, tx, AlertFilter{Since: &since, Sort: AlertSortSeverity, Asc: false, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if got := syms(rows); len(got) != 4 || got[0] != "ZZSC:warning" || got[1] != "ZZSA:notice" || got[2] != "ZZSB:notice" || got[3] != "ZZSA:info" {
		t.Errorf("severity desc, ties by symbol = %v", got)
	}
	rows, _ = ListAlerts(ctx, tx, AlertFilter{Since: &since, Sort: AlertSortSymbol, Asc: true, Limit: 2, Offset: 2})
	if got := syms(rows); len(got) != 2 || got[0] != "ZZSB:notice" || got[1] != "ZZSC:warning" {
		t.Errorf("symbol asc, offset 2 = %v", got)
	}
	rows, _ = ListAlerts(ctx, tx, AlertFilter{Since: &since, Query: "squeeze", Limit: 10})
	if got := syms(rows); len(got) != 1 || got[0] != "ZZSA:info" {
		t.Errorf("message search = %v", got)
	}
	rows, _ = ListAlerts(ctx, tx, AlertFilter{Since: &since, Query: "zzsc", Limit: 10})
	if len(rows) != 1 {
		t.Errorf("symbol search (case-insensitive) = %v", syms(rows))
	}
	rows, _ = ListAlerts(ctx, tx, AlertFilter{Since: &since, Query: "VIX elev", QueryTypes: []string{"vix_elevated"}, Limit: 10})
	if len(rows) != 1 || rows[0].AlertType != "vix_elevated" {
		t.Errorf("label-resolved type search = %v", syms(rows))
	}
	rows, _ = ListAlerts(ctx, tx, AlertFilter{Since: &since, Query: "100%", Limit: 10})
	if len(rows) != 0 {
		t.Errorf("%% is literal, not a wildcard: %v", syms(rows))
	}
	if _, err := tx.Exec(ctx, `INSERT INTO fired_alerts (symbol, exchange_type, alert_type, interval, severity, message, fired_at, bar_date)
		VALUES ('ZZSB', 'equity', 'liquidity_sweep', '1Day', 'notice', 'New liquidity sweep', $1, '2099-04-10')`, base.Add(5*time.Hour)); err != nil {
		t.Fatal(err)
	}
	groups, err := ListAlertGroups(ctx, tx, AlertFilter{Since: &since, Sort: AlertSortCount, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, g := range groups {
		got = append(got, fmt.Sprintf("%s:%s:%d", g.Symbol, g.AlertType, g.Count))
	}
	want := []string{"ZZSB:liquidity_sweep:2", "ZZSA:liquidity_sweep:1", "ZZSA:bb_squeeze:1", "ZZSC:vix_elevated:1"}
	if !slices.Equal(got, want) {
		t.Errorf("groups by count desc, ties by symbol then newest = %v, want %v", got, want)
	}
}
