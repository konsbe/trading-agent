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
