package momentumapi

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/konsbe/trading-agent/services/data-analyzer/internal/store"
)

func (f *fakeStore) ListAlerts(_ context.Context, fl store.AlertFilter) ([]store.AlertRow, error) {
	f.alertFilters = append(f.alertFilters, fl)
	out := []store.AlertRow{}
	for _, a := range f.alerts {
		if (fl.Symbol == nil || a.Symbol == *fl.Symbol) && (fl.Since == nil || !a.FiredAt.Before(*fl.Since)) && len(out) < fl.Limit {
			out = append(out, a)
		}
	}
	return out, f.queryErr
}

func alertsFixture() *fakeStore {
	st := fixtureStore()
	at := time.Date(2026, 9, 25, 20, 14, 0, 0, time.UTC)
	st.alerts = []store.AlertRow{
		{ID: 3, Symbol: "TSM", ExchangeType: "equity", AlertType: "liquidity_sweep", Interval: "1Day", Value: ptr(4.0),
			Severity: "notice", Message: "Liquidity sweep detected (4 sweeps)", FiredAt: at.In(time.FixedZone("EEST", 3*3600))},
		{ID: 2, Symbol: "BTCUSDT", ExchangeType: "crypto", AlertType: "rsi_overbought", Interval: "1Day", Value: ptr(71.0),
			Severity: "notice", Message: "RSI 71.0 — overbought (>70.0)", FiredAt: at.Add(-time.Hour)},
		{ID: 1, Symbol: "TSM", ExchangeType: "equity", AlertType: "bb_squeeze", Interval: "1Day",
			Severity: "info", Message: "BB squeeze", FiredAt: at.AddDate(0, 0, -2)},
	}
	return st
}

func TestAlerts_DefaultIsTheNewestBoundedFeed(t *testing.T) {
	st := alertsFixture()
	rec := get(t, newTestServer(t, st, freshNow), "/api/v1/alerts", "Origin", "http://localhost:3000")
	if rec.Code != http.StatusOK {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	if rec.Header().Get("Access-Control-Allow-Origin") != "http://localhost:3000" {
		t.Error("missing CORS header")
	}
	body := decode(t, rec)
	if body["symbol"] != nil || body["since"] != nil || body["limit"] != 100.0 || body["has_more"] != false {
		t.Errorf("envelope = %v", body)
	}
	if f := st.alertFilters[0]; f.Symbol != nil || f.Since != nil || f.Limit != 101 {
		t.Errorf("filter = %+v, want no filters and limit+1", f)
	}
	alerts := body["alerts"].([]any)
	if len(alerts) != 3 {
		t.Fatalf("alerts = %d", len(alerts))
	}
	first := alerts[0].(map[string]any)
	want := map[string]any{"id": 3.0, "symbol": "TSM", "exchange_type": "equity", "alert_type": "liquidity_sweep",
		"interval": "1Day", "value": 4.0, "severity": "notice", "message": "Liquidity sweep detected (4 sweeps)",
		"fired_at": "2026-09-25T20:14:00Z"}
	if len(first) != len(want) {
		t.Errorf("alert = %v, want exactly %v", first, want)
	}
	for k, v := range want {
		if first[k] != v {
			t.Errorf("alert.%s = %v, want %v", k, first[k], v)
		}
	}
	if v, present := alerts[2].(map[string]any)["value"]; !present || v != nil {
		t.Errorf("null value = %v (present=%v), want explicit null", v, present)
	}
}

func TestAlerts_SymbolAndSinceCombine(t *testing.T) {
	st := alertsFixture()
	srv := newTestServer(t, st, freshNow)
	body := decode(t, get(t, srv, "/api/v1/alerts?symbol=tsm&since=2026-09-25"))
	if body["symbol"] != "TSM" || body["since"] != "2026-09-25" {
		t.Errorf("envelope = %v", body)
	}
	f := st.alertFilters[0]
	if f.Symbol == nil || *f.Symbol != "TSM" || f.Since == nil || !f.Since.Equal(time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("filter = %+v, want TSM since 2026-09-25T00:00Z", f)
	}
	if a := body["alerts"].([]any); len(a) != 1 || a[0].(map[string]any)["id"] != 3.0 {
		t.Errorf("alerts = %v", a)
	}

	var ids []float64
	for _, a := range decode(t, get(t, srv, "/api/v1/alerts?symbol=TSM"))["alerts"].([]any) {
		ids = append(ids, a.(map[string]any)["id"].(float64))
	}
	if !slices.Equal(ids, []float64{3, 1}) {
		t.Errorf("symbol only: ids = %v, want newest first", ids)
	}
	if a := decode(t, get(t, srv, "/api/v1/alerts?since=2099-01-01"))["alerts"].([]any); len(a) != 0 {
		t.Errorf("no rows = %v, want []", a)
	}
}

func TestAlerts_LimitAndHasMore(t *testing.T) {
	st := alertsFixture()
	srv := newTestServer(t, st, freshNow)
	body := decode(t, get(t, srv, "/api/v1/alerts?limit=2"))
	if body["limit"] != 2.0 || body["has_more"] != true || len(body["alerts"].([]any)) != 2 {
		t.Errorf("limit=2: %v", body)
	}
	body = decode(t, get(t, srv, "/api/v1/alerts?limit=3"))
	if body["has_more"] != false || len(body["alerts"].([]any)) != 3 {
		t.Errorf("limit=3: %v", body)
	}
	if rec := get(t, srv, "/api/v1/alerts?limit=500"); rec.Code != http.StatusOK {
		t.Errorf("limit=500 (the max) = %d", rec.Code)
	}
}

func TestAlerts_Errors(t *testing.T) {
	srv := newTestServer(t, alertsFixture(), freshNow)
	for path, want := range map[string]string{
		"/api/v1/alerts?since=2026-13-01":           "invalid_since",
		"/api/v1/alerts?since=yesterday":            "invalid_since",
		"/api/v1/alerts?since=2026-09-25T00:00:00Z": "invalid_since",
		"/api/v1/alerts?limit=0":                    "invalid_limit",
		"/api/v1/alerts?limit=501":                  "invalid_limit",
		"/api/v1/alerts?limit=ten":                  "invalid_limit",
		"/api/v1/alerts?symbol=BAD$":                "invalid_symbol",
	} {
		if rec := get(t, srv, path); rec.Code != http.StatusBadRequest || decode(t, rec)["error"] != want {
			t.Errorf("%s: %d %s, want 400 %s", path, rec.Code, rec.Body, want)
		}
	}

	st := alertsFixture()
	st.queryErr = errors.New("relation does not exist")
	if rec := get(t, newTestServer(t, st, freshNow), "/api/v1/alerts"); rec.Code != http.StatusInternalServerError || decode(t, rec)["error"] != "internal_error" {
		t.Errorf("failing query: %d %s", rec.Code, rec.Body)
	}
	st.pingErr = errors.New("down")
	if rec := get(t, newTestServer(t, st, freshNow), "/api/v1/alerts"); rec.Code != http.StatusServiceUnavailable || decode(t, rec)["error"] != "database_unavailable" {
		t.Errorf("db down: %d %s", rec.Code, rec.Body)
	}
	// No scan is not a precondition: alerts exist independently of the scanner.
	st = alertsFixture()
	st.hasScan = false
	if rec := get(t, newTestServer(t, st, freshNow), "/api/v1/alerts"); rec.Code != http.StatusOK {
		t.Errorf("no scan: %d", rec.Code)
	}
}
