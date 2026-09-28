package momentumapi

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/konsbe/trading-agent/services/data-analyzer/internal/store"
)

func (f *fakeStore) matchAlerts(fl store.AlertFilter) []store.AlertRow {
	out := []store.AlertRow{}
	for _, a := range f.alerts { // fixture is newest first
		if (fl.Symbol == nil || a.Symbol == *fl.Symbol) &&
			(len(fl.AlertTypes) == 0 || slices.Contains(fl.AlertTypes, a.AlertType)) &&
			(len(fl.Severities) == 0 || slices.Contains(fl.Severities, a.Severity)) &&
			(fl.Since == nil || !a.FiredAt.Before(*fl.Since)) &&
			(fl.Until == nil || a.FiredAt.Before(*fl.Until)) &&
			(fl.Query == "" || strings.Contains(strings.ToLower(a.Symbol+" "+a.Message+" "+a.Severity), strings.ToLower(fl.Query)) ||
				slices.Contains(fl.QueryTypes, a.AlertType)) {
			out = append(out, a)
		}
	}
	if fl.Offset > 0 {
		if fl.Offset >= len(out) {
			return []store.AlertRow{}
		}
		out = out[fl.Offset:]
	}
	return out
}

func (f *fakeStore) ListAlerts(_ context.Context, fl store.AlertFilter) ([]store.AlertRow, error) {
	f.alertFilters = append(f.alertFilters, fl)
	var cursor *store.AlertRow
	for i := range f.alerts {
		if fl.Before != nil && f.alerts[i].ID == *fl.Before {
			cursor = &f.alerts[i]
		}
	}
	out := []store.AlertRow{}
	for _, a := range f.matchAlerts(fl) {
		if fl.Before != nil && (cursor == nil || !(a.FiredAt.Before(cursor.FiredAt) || a.FiredAt.Equal(cursor.FiredAt) && a.ID < cursor.ID)) {
			continue
		}
		if len(out) < fl.Limit {
			out = append(out, a)
		}
	}
	return out, f.queryErr
}

func (f *fakeStore) ListAlertGroups(_ context.Context, fl store.AlertFilter) ([]store.AlertGroup, error) {
	f.alertFilters = append(f.alertFilters, fl)
	idx := map[string]int{}
	var groups []store.AlertGroup
	for _, a := range f.matchAlerts(fl) { // newest first: the first row of a group is its latest
		k := a.Symbol + "|" + a.AlertType
		i, ok := idx[k]
		if !ok {
			idx[k] = len(groups)
			groups = append(groups, store.AlertGroup{Symbol: a.Symbol, AlertType: a.AlertType, LastFiredAt: a.FiredAt, Latest: a})
			i = len(groups) - 1
		}
		groups[i].Count++
		groups[i].FirstFiredAt = a.FiredAt
	}
	var cursor *store.AlertRow
	for i := range f.alerts {
		if fl.Before != nil && f.alerts[i].ID == *fl.Before {
			cursor = &f.alerts[i]
		}
	}
	out := []store.AlertGroup{}
	for _, g := range groups {
		if fl.Before != nil && (cursor == nil || !(g.Latest.FiredAt.Before(cursor.FiredAt) || g.Latest.FiredAt.Equal(cursor.FiredAt) && g.Latest.ID < cursor.ID)) {
			continue
		}
		if len(out) < fl.Limit {
			out = append(out, g)
		}
	}
	return out, f.queryErr
}

func (f *fakeStore) AlertMeta(context.Context) (store.AlertMeta, error) {
	m := store.AlertMeta{Types: []string{}}
	seen := map[string]bool{}
	for _, a := range f.alerts {
		if !seen[a.AlertType] {
			seen[a.AlertType] = true
			m.Types = append(m.Types, a.AlertType)
		}
		if m.EarliestFired == nil || a.FiredAt.Before(*m.EarliestFired) {
			t := a.FiredAt.UTC()
			m.EarliestFired = &t
		}
		if a.BarDate != nil && (m.OnsetsSince == nil || a.FiredAt.Before(*m.OnsetsSince)) {
			t := a.FiredAt.UTC()
			m.OnsetsSince = &t
		}
	}
	slices.Sort(m.Types)
	return m, f.queryErr
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
		"fired_at": "2026-09-25T20:14:00Z", "bar_date": nil}
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
		"/api/v1/alerts?since=2026-13-01":                  "invalid_since",
		"/api/v1/alerts?since=yesterday":                   "invalid_since",
		"/api/v1/alerts?until=soon":                        "invalid_until",
		"/api/v1/alerts?since=2026-09-26&until=2026-09-25": "invalid_range",
		"/api/v1/alerts?alert_type=Bad-Type":               "invalid_alert_type",
		"/api/v1/alerts?severity=critical":                 "invalid_severity",
		"/api/v1/alerts?before=0":                          "invalid_before",
		"/api/v1/alerts?before=x":                          "invalid_before",
		"/api/v1/alerts?mode=flat":                         "invalid_mode",
		"/api/v1/alerts?sort=count":                        "invalid_sort",
		"/api/v1/alerts?sort=price":                        "invalid_sort",
		"/api/v1/alerts?dir=up":                            "invalid_dir",
		"/api/v1/alerts?offset=-1":                         "invalid_offset",
		"/api/v1/alerts?sort=symbol&before=3":              "invalid_before",
		"/api/v1/alerts?q=sweep&before=3":                  "invalid_before",
		"/api/v1/alerts?limit=0":                           "invalid_limit",
		"/api/v1/alerts?limit=501":                         "invalid_limit",
		"/api/v1/alerts?limit=ten":                         "invalid_limit",
		"/api/v1/alerts?symbol=BAD$":                       "invalid_symbol",
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

// Alarm History: every response carries the table-wide types, the earliest
// record and the verbatim heuristic caveat from the shared file.
func TestAlerts_MetaAndCaveat(t *testing.T) {
	body := decode(t, get(t, newTestServer(t, alertsFixture(), freshNow), "/api/v1/alerts?symbol=BTCUSDT"))
	if !slices.Equal(toStrings(body["types"]), []string{"bb_squeeze", "liquidity_sweep", "rsi_overbought"}) {
		t.Errorf("types = %v (table-wide, not narrowed by filters)", body["types"])
	}
	if body["records_start"] != "2026-09-23T20:14:00Z" {
		t.Errorf("records_start = %v", body["records_start"])
	}
	if c := body["caveat"].(string); c != loadSharedCaveats(t).HeuristicTA || c == "" {
		t.Errorf("caveat must be the shared heuristic_ta_caveat verbatim, got %q", c)
	}
	if body["mode"] != "raw" || len(body["groups"].([]any)) != 0 {
		t.Errorf("default mode = %v groups %v", body["mode"], body["groups"])
	}
}

func TestAlerts_TypeSeverityAndBounds(t *testing.T) {
	st := alertsFixture()
	srv := newTestServer(t, st, freshNow)
	body := decode(t, get(t, srv, "/api/v1/alerts?alert_type=liquidity_sweep&alert_type=bb_squeeze&severity=notice,info"))
	if !slices.Equal(toStrings(body["alert_types"]), []string{"liquidity_sweep", "bb_squeeze"}) || !slices.Equal(toStrings(body["severities"]), []string{"notice", "info"}) {
		t.Errorf("echo = %v %v", body["alert_types"], body["severities"])
	}
	if ids := alertIDs(body); !slices.Equal(ids, []float64{3, 1}) {
		t.Errorf("ids = %v", ids)
	}
	// A date until covers that whole UTC day; a timestamp is exclusive as given.
	body = decode(t, get(t, srv, "/api/v1/alerts?since=2026-09-25&until=2026-09-25"))
	if ids := alertIDs(body); !slices.Equal(ids, []float64{3, 2}) || body["until"] != "2026-09-25" {
		t.Errorf("date range ids = %v until %v", ids, body["until"])
	}
	f := st.alertFilters[len(st.alertFilters)-1]
	if !f.Until.Equal(time.Date(2026, 9, 26, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("until date = %v, want the next midnight (exclusive)", f.Until)
	}
	body = decode(t, get(t, srv, "/api/v1/alerts?since=2026-09-25T19:30:00%2B03:00&until=2026-09-25T23:14:00%2B03:00"))
	if ids := alertIDs(body); !slices.Equal(ids, []float64{2}) || body["since"] != "2026-09-25T16:30:00Z" {
		t.Errorf("local-day bounds ids = %v since %v", ids, body["since"])
	}
}

func TestAlerts_BeforeCursorPages(t *testing.T) {
	srv := newTestServer(t, alertsFixture(), freshNow)
	body := decode(t, get(t, srv, "/api/v1/alerts?limit=2"))
	if body["has_more"] != true || body["next_before"] != 2.0 {
		t.Fatalf("page 1 = %v", body)
	}
	body = decode(t, get(t, srv, "/api/v1/alerts?limit=2&before=2"))
	if ids := alertIDs(body); !slices.Equal(ids, []float64{1}) || body["has_more"] != false || body["next_before"] != nil || body["before"] != 2.0 {
		t.Errorf("page 2 = %v", body)
	}
}

func TestAlerts_GroupedMode(t *testing.T) {
	st := alertsFixture()
	at := time.Date(2026, 9, 25, 16, 14, 0, 0, time.UTC)
	st.alerts = append([]store.AlertRow{
		{ID: 5, Symbol: "TSM", ExchangeType: "equity", AlertType: "liquidity_sweep", Interval: "1Day", Value: ptr(6.0),
			Severity: "notice", Message: "Liquidity sweep detected (6 sweeps)", FiredAt: at.Add(24 * time.Hour)},
	}, st.alerts...)
	srv := newTestServer(t, st, freshNow)
	body := decode(t, get(t, srv, "/api/v1/alerts?mode=grouped"))
	groups := body["groups"].([]any)
	if body["mode"] != "grouped" || len(groups) != 3 || len(body["alerts"].([]any)) != 0 {
		t.Fatalf("grouped = %v", body)
	}
	g := groups[0].(map[string]any)
	latest := g["latest"].(map[string]any)
	if g["symbol"] != "TSM" || g["alert_type"] != "liquidity_sweep" || g["count"] != 2.0 || g["exchange_type"] != "equity" ||
		g["first_fired_at"] != "2026-09-25T20:14:00Z" || g["last_fired_at"] != "2026-09-26T16:14:00Z" ||
		latest["id"] != 5.0 || latest["message"] != "Liquidity sweep detected (6 sweeps)" || latest["value"] != 6.0 {
		t.Errorf("group = %v", g)
	}
	body = decode(t, get(t, srv, "/api/v1/alerts?mode=grouped&limit=2"))
	if body["has_more"] != true || body["next_before"] != 2.0 {
		t.Fatalf("grouped page 1 = %v", body)
	}
	body = decode(t, get(t, srv, "/api/v1/alerts?mode=grouped&limit=2&before=2"))
	if gs := body["groups"].([]any); len(gs) != 1 || gs[0].(map[string]any)["alert_type"] != "bb_squeeze" || body["has_more"] != false {
		t.Errorf("grouped page 2 = %v", body)
	}
}

func alertIDs(body map[string]any) []float64 {
	var ids []float64
	for _, a := range body["alerts"].([]any) {
		ids = append(ids, a.(map[string]any)["id"].(float64))
	}
	return ids
}

// Labels come from shared/content/alert_messages.json; onsets_since is the
// first row with a bar_date, and each alert carries its bar_date.
func TestAlerts_TypeLabelsAndOnsets(t *testing.T) {
	st := alertsFixture()
	body := decode(t, get(t, newTestServer(t, st, freshNow), "/api/v1/alerts"))
	labels := body["type_labels"].(map[string]any)
	if labels["liquidity_sweep"] != "Liquidity sweep" || labels["bb_squeeze"] != "Bollinger squeeze" || len(labels) != 6 {
		t.Errorf("type_labels = %v", labels)
	}
	if body["onsets_since"] != nil || body["alerts"].([]any)[0].(map[string]any)["bar_date"] != nil {
		t.Errorf("no onset rows yet: onsets_since %v", body["onsets_since"])
	}
	bar := time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)
	st.alerts = append([]store.AlertRow{{ID: 9, Symbol: "XOM", ExchangeType: "equity", AlertType: "bb_squeeze", Interval: "1Day",
		Severity: "info", Message: "Bollinger Squeeze began", FiredAt: bar.Add(30 * time.Hour), BarDate: &bar}}, st.alerts...)
	body = decode(t, get(t, newTestServer(t, st, freshNow), "/api/v1/alerts"))
	if body["onsets_since"] != "2026-09-29T06:00:00Z" || body["alerts"].([]any)[0].(map[string]any)["bar_date"] != "2026-09-28" {
		t.Errorf("onsets_since %v first %v", body["onsets_since"], body["alerts"].([]any)[0])
	}
}

func TestLoadAlertMessages(t *testing.T) {
	if _, err := LoadAlertMessages(filepath.Join(t.TempDir(), "missing.json")); err == nil {
		t.Error("a missing file must fail: there is no fallback")
	}
	p := filepath.Join(t.TempDir(), "m.json")
	_ = os.WriteFile(p, []byte(`{"alert_types":{"bb_squeeze":{"label":"","message":"x"}}}`), 0o644)
	if _, err := LoadAlertMessages(p); err == nil {
		t.Error("an entry without a label must fail")
	}
}

// Sorted or searched views page by offset; the search also matches the
// displayed type label (alert_messages.json), not only the stored id.
func TestAlerts_SortSearchOffset(t *testing.T) {
	st := alertsFixture()
	srv := newTestServer(t, st, freshNow)
	body := decode(t, get(t, srv, "/api/v1/alerts?sort=symbol&dir=asc&limit=2"))
	f := st.alertFilters[len(st.alertFilters)-1]
	if f.Sort != "symbol" || !f.Asc || body["sort"] != "symbol" || body["dir"] != "asc" ||
		body["has_more"] != true || body["next_offset"] != 2.0 || body["next_before"] != nil {
		t.Fatalf("page 1 = %v (filter %+v)", body, f)
	}
	body = decode(t, get(t, srv, "/api/v1/alerts?sort=symbol&dir=asc&limit=2&offset=2"))
	if len(body["alerts"].([]any)) != 1 || body["has_more"] != false || body["next_offset"] != nil {
		t.Errorf("page 2 = %v", body)
	}
	body = decode(t, get(t, srv, "/api/v1/alerts?q=Bollinger"))
	f = st.alertFilters[len(st.alertFilters)-1]
	if !slices.Equal(f.QueryTypes, []string{"bb_squeeze"}) || body["q"] != "Bollinger" {
		t.Errorf("label search: types %v body %v", f.QueryTypes, body["q"])
	}
	if ids := alertIDs(body); !slices.Equal(ids, []float64{1}) {
		t.Errorf("Bollinger matches the bb_squeeze row by its label: ids %v", ids)
	}
	if body := decode(t, get(t, srv, "/api/v1/alerts?mode=grouped&sort=count&dir=desc")); body["sort"] != "count" {
		t.Errorf("grouped count sort = %v", body["sort"])
	}
	if body := decode(t, get(t, srv, "/api/v1/alerts?sort=fired&dir=desc&limit=1")); body["next_before"] == nil || body["next_offset"] != nil {
		t.Errorf("newest-first view must keep next_before: %v", body)
	}
}
