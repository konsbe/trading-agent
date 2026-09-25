package momentumapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/konsbe/trading-agent/services/data-analyzer/internal/store"
)

var reportNow = time.Date(2026, 9, 25, 5, 0, 0, 0, time.UTC) // 01:00 New York, Friday

const automationStatus = `{"factor":{"hint":"Quality/value overlap","status":"partial"},"sentiment":{"hint":"PCR, IV skew","status":"needs_data"},"alternative_data":{"hint":"Vendor datasets only","status":"not_automated"}}`

func rbar(day string, close float64, hourUTC int) store.EquityOHLCVBar {
	d, _ := time.Parse(time.DateOnly, day)
	return store.EquityOHLCVBar{TS: d.Add(time.Duration(hourUTC) * time.Hour), Close: close}
}

func reportFixture() store.MarketReportInputs {
	gen := time.Date(2026, 9, 24, 22, 58, 0, 0, time.UTC)
	row := func(metric string, value float64, payload string) store.MacroRow {
		v := value
		return store.MacroRow{Metric: metric, TS: gen, Value: &v, Payload: json.RawMessage(payload)}
	}
	return store.MarketReportInputs{
		Macro: map[string]store.MacroRow{
			"mp_stance":            row("mp_stance", 0.4, `{"stance":"neutral","score":"0.40"}`),
			"mp_yield_curve":       row("mp_yield_curve", 0.31, `{"regime":"normal"}`),
			"gc_stance":            row("gc_stance", 0.425, `{"stance":"expansion"}`),
			"mc_macro_correlation": row("mc_macro_correlation", -0.52, `{"regime":"global_liquidity_stress","flags":["inflation_hot"]}`),
			"mc_market_cycle":      row("mc_market_cycle", 0.15, `{"symbol":"SPY","composite_phase":"late_cycle_stretched"}`),
			"aa_reference_snapshot": row("aa_reference_snapshot", 0,
				`{"seasonality":{"month":9,"bias":"weak_bear"},"presidential_cycle":{"cycle_year":2},"intermarket":{"bond_equity_60d":{"rho":-0.53}},"reference_modules":`+automationStatus+`}`),
			"mc_price_phase:SPY":   row("mc_price_phase:SPY", -1.56, `{"price_phase":"bull_extended","drawdown_pct":-1.56,"pct_vs_sma200":6.85,"crash_warning":false,"as_of":"2026-09-24","as_of_basis":"session close","windows":{"sma_period":200}}`),
			"mc_price_phase:DGS10": row("mc_price_phase:DGS10", 0, `{"price_phase":"bull"}`), // must never surface
			"mc_price_phase:INFQ":  row("mc_price_phase:INFQ", 0, `{"price_phase":"insufficient_data","unavailable_reason":"153 1Day bars stored for INFQ; need at least 200"}`),
		},
		Fred: map[string]store.FredPoint{
			"VIXCLS": {Value: 14.2, TS: time.Date(2026, 9, 22, 0, 0, 0, 0, time.UTC)},
			"DGS10":  {Value: 5.11, TS: time.Date(2026, 9, 23, 0, 0, 0, 0, time.UTC)},
			"DGS2":   {Value: 4.85, TS: time.Date(2026, 9, 23, 0, 0, 0, 0, time.UTC)},
		},
		Closes: map[string][]store.EquityOHLCVBar{
			"SPY":     {rbar("2026-09-23", 767.8, 13), rbar("2026-09-24", 767.18, 13)},
			"BTCUSDT": {rbar("2026-09-22", 80000, 0), rbar("2026-09-23", 84000, 0)},
			"INFQ":    {rbar("2026-09-24", 10, 13)},
		},
		Economic:  json.RawMessage(`[]`),
		Earnings:  json.RawMessage(`[{"symbol":"SHEL","date":"2026-10-30","period":"3"}]`),
		Headlines: json.RawMessage(`[{"ts":"2026-09-24T20:00:00Z","source":"finnhub_macro_general","headline":"H","url":"https://example.com/x"}]`),
		Watchlist: []store.WatchlistItem{{Symbol: "INFQ"}, {Symbol: "SHEL"}, {Symbol: "NVDA", CompanyName: ptr("NVIDIA CORP")}},
	}
}

func reportBody(t *testing.T, in store.MarketReportInputs, now time.Time) map[string]any {
	t.Helper()
	raw, err := json.Marshal(buildMarketReport(in, now))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func instrumentsByKey(t *testing.T, body map[string]any) map[string]map[string]any {
	t.Helper()
	out := map[string]map[string]any{}
	for _, v := range body["instruments"].([]any) {
		m := v.(map[string]any)
		out[m["key"].(string)] = m
	}
	return out
}

// §5: treasury_yield instruments never carry a market_cycle object — even if a
// price-phase row existed for the series, it is a category error, not data.
func TestMarketReport_YieldsNeverCarryAMarketCycle(t *testing.T) {
	ins := instrumentsByKey(t, reportBody(t, reportFixture(), reportNow))
	for _, k := range []string{"us10y", "us5y", "us2y"} {
		in := ins[k]
		if in["type"] != "treasury_yield" || in["market_cycle"] != nil {
			t.Errorf("%s: type %v market_cycle %v; yields must have market_cycle null", k, in["type"], in["market_cycle"])
		}
		if _, hasPrice := in["price"]; hasPrice {
			t.Errorf("%s carries a price object; yields carry yield", k)
		}
	}
	if y := ins["us10y"]["yield"].(map[string]any); y["value"] != 5.11 || y["as_of"] != "2026-09-23" {
		t.Errorf("us10y yield = %v", y)
	}
	if ins["us5y"]["unavailable_reason"] == nil {
		t.Error("us5y has no observations in the fixture; it needs an unavailable_reason, not an empty card")
	}
}

// §5: an instrument with no data returns unavailable_reason and null prices —
// not a 500, not a silently dropped entry.
func TestMarketReport_NoDataIsAReasonNotAGap(t *testing.T) {
	body := reportBody(t, reportFixture(), reportNow)
	ins := instrumentsByKey(t, body)
	if len(body["instruments"].([]any)) != len(fixedInstruments)+2 {
		t.Fatalf("%d instruments; want the fixed %d plus INFQ and NVDA (SHEL deduplicated)", len(body["instruments"].([]any)), len(fixedInstruments))
	}
	gld := ins["gold"]
	if gld["price"] != nil || gld["market_cycle"] != nil || gld["unavailable_reason"] != "no daily bars stored for GLD" {
		t.Errorf("gold (no bars) = %v", gld)
	}
	infq := ins["INFQ"]
	if infq["price"] == nil || infq["market_cycle"] != nil || infq["unavailable_reason"] != "153 1Day bars stored for INFQ; need at least 200" {
		t.Errorf("INFQ (price but too few bars for a phase) = %v", infq)
	}
	if nv := ins["NVDA"]; nv["unavailable_reason"] == nil || nv["label"] != "NVIDIA CORP" {
		t.Errorf("NVDA = %v", nv)
	}
}

// §5: watchlist instruments merge with the fixed list and are labelled as such;
// a watched symbol already in the fixed list is not repeated.
func TestMarketReport_WatchlistMergesAndIsLabelled(t *testing.T) {
	body := reportBody(t, reportFixture(), reportNow)
	var shel, wl int
	for _, v := range body["instruments"].([]any) {
		m := v.(map[string]any)
		if m["symbol"] == "SHEL" {
			shel++
			if m["source"] != "fixed_list" {
				t.Errorf("SHEL source = %v, want fixed_list", m["source"])
			}
		}
		if m["source"] == "watchlist" {
			wl++
		}
	}
	if shel != 1 || wl != 2 {
		t.Errorf("SHEL appears %d time(s), %d watchlist entries; want 1 and 2", shel, wl)
	}
}

// §5: automation_status is the snapshot's list, unchanged — every
// not_automated / needs_data / partial entry included.
func TestMarketReport_AutomationStatusPassesThroughUnchanged(t *testing.T) {
	raw, _ := json.Marshal(buildMarketReport(reportFixture(), reportNow))
	var top struct {
		Global struct {
			AutomationStatus json.RawMessage `json:"automation_status"`
		} `json:"global"`
	}
	if err := json.Unmarshal(raw, &top); err != nil {
		t.Fatal(err)
	}
	var want, got bytes.Buffer
	_ = json.Compact(&want, []byte(automationStatus))
	_ = json.Compact(&got, top.Global.AutomationStatus)
	if want.String() != got.String() {
		t.Errorf("automation_status changed in transit:\n got %s\nwant %s", got.String(), want.String())
	}
}

func TestMarketReport_GlobalSectionsAndFreshness(t *testing.T) {
	body := reportBody(t, reportFixture(), reportNow)
	if body["report_date"] != "2026-09-24" || body["is_stale"] != false {
		t.Errorf("report_date %v is_stale %v", body["report_date"], body["is_stale"])
	}
	g := body["global"].(map[string]any)
	mp := g["monetary_policy"].(map[string]any)
	if mp["label"] != "neutral" || mp["score"] != 0.4 || mp["signals"].(map[string]any)["mp_yield_curve"] == nil {
		t.Errorf("monetary_policy = %v", mp)
	}
	if _, stanceIsASignal := mp["signals"].(map[string]any)["mp_stance"]; stanceIsASignal {
		t.Error("the stance row must not also appear as one of its own signals")
	}
	if g["inflation"] != nil {
		t.Errorf("inflation has no rows in the fixture; want null, got %v", g["inflation"])
	}
	if vix := g["macro"].(map[string]any)["vix"].(map[string]any); vix["value"] != 14.2 || vix["as_of"] != "2026-09-22" {
		t.Errorf("vix = %v; values carry their own FRED date", vix)
	}
	if g["macro"].(map[string]any)["eur_usd"] != nil {
		t.Error("eur_usd has no observation; want null")
	}
	if len(g["news"].([]any)) != 1 || g["seasonality"].(map[string]any)["bias"] != "weak_bear" {
		t.Errorf("news %v seasonality %v", g["news"], g["seasonality"])
	}
	stale := reportBody(t, reportFixture(), reportNow.Add(13*time.Hour))
	if stale["is_stale"] != true || stale["report_date"] != "2026-09-24" {
		t.Errorf("13h later: is_stale %v report_date %v; an old report is served and says so", stale["is_stale"], stale["report_date"])
	}
	empty := reportBody(t, store.MarketReportInputs{}, reportNow)
	if empty["report_date"] != nil || empty["is_stale"] != true || len(empty["instruments"].([]any)) != len(fixedInstruments) {
		t.Errorf("no report generated yet: %v", empty)
	}
}

func TestMarketReport_SessionClosedAndCryptoBasis(t *testing.T) {
	in := reportFixture()
	ins := instrumentsByKey(t, reportBody(t, in, reportNow))
	spy := ins["sp500"]["price"].(map[string]any)
	if spy["session_closed"] != true || spy["as_of"] != "2026-09-24" || spy["change_pct"] != -0.08 {
		t.Errorf("SPY after the close = %v", spy)
	}
	// Same bar read during the 2026-09-24 session: not a close yet.
	during := instrumentsByKey(t, reportBody(t, in, time.Date(2026, 9, 24, 17, 0, 0, 0, time.UTC)))
	if during["sp500"]["price"].(map[string]any)["session_closed"] != false {
		t.Error("a bar dated today before 16:00 New York must read session_closed false")
	}
	btc := ins["bitcoin"]
	if btc["type"] != "crypto" || btc["price"].(map[string]any)["session_closed"] != true || btc["price"].(map[string]any)["as_of"] != "2026-09-23" {
		t.Errorf("bitcoin = %v", btc)
	}
}

// A Tiingo-stamped bar (00:00 UTC) belongs to that UTC date's session, not to
// the previous New York evening.
func TestMarketReport_TiingoStampedBarKeepsItsSessionDate(t *testing.T) {
	in := reportFixture()
	in.Closes["SHEL"] = []store.EquityOHLCVBar{rbar("2026-09-23", 95.4, 0), rbar("2026-09-24", 95.6, 0)}
	p := instrumentsByKey(t, reportBody(t, in, reportNow))["shell"]["price"].(map[string]any)
	if p["as_of"] != "2026-09-24" || p["session_closed"] != true {
		t.Errorf("SHEL price = %v; a 00:00 UTC 09-24 bar is the 09-24 session", p)
	}
	during := instrumentsByKey(t, reportBody(t, in, time.Date(2026, 9, 24, 17, 0, 0, 0, time.UTC)))
	if during["shell"]["price"].(map[string]any)["session_closed"] != false {
		t.Error("during the 09-24 session a 09-24 bar (any stamp) is still forming")
	}
}

func TestMarketReport_CacheAndWatchlistInvalidation(t *testing.T) {
	st := fixtureStore()
	st.report = reportFixture()
	st.known = map[string]bool{"MRVL": true}
	srv := newTestServer(t, st, reportNow)
	for i := 0; i < 2; i++ {
		if rec := get(t, srv, "/api/v1/market-report/today"); rec.Code != http.StatusOK {
			t.Fatalf("status %d %s", rec.Code, rec.Body)
		}
	}
	if st.reportCalls != 1 {
		t.Fatalf("report loaded %d times; the second request must be cached", st.reportCalls)
	}
	request(t, srv, http.MethodPut, "/api/v1/watchlist/MRVL")
	get(t, srv, "/api/v1/market-report/today")
	if st.reportCalls != 2 {
		t.Errorf("a watchlist write must drop the cached report (calls=%d)", st.reportCalls)
	}
}

func TestMarketReport_DatabaseDownIs503(t *testing.T) {
	st := fixtureStore()
	st.reportErr, st.pingErr = errors.New("conn refused"), errors.New("conn refused")
	rec := get(t, newTestServer(t, st, reportNow), "/api/v1/market-report/today")
	if rec.Code != http.StatusServiceUnavailable || decode(t, rec)["error"] != "database_unavailable" {
		t.Errorf("got %d %s", rec.Code, rec.Body)
	}
}
