package momentumapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/konsbe/trading-agent/services/data-analyzer/internal/store"
)

// Daily Market Report (docs/MOMENTUM_SCANNER_API_DAILY_MARKET_REPORT.md).
//
//	GET /api/v1/market-report/today
//
// A JSON reshaping of what macro-analysis, data-macro-intel and data-technical
// already stored for the Discord report. It computes no new analysis: payloads
// are passed through as stored. The only derived values are an instrument's
// day change (from its last two daily closes, via the same readers the market
// cycle uses) and the flags that say how fresh something is.

const marketReportCacheKey = "market-report"

// reportInstrument is one entry of the fixed list.
type reportInstrument struct {
	key, label, symbol, typ string
}

// fixedInstruments is the report's fixed list, in display order. Yields carry
// no price phase: a yield series has no drawdown or 200DMA (addendum §2.2).
var fixedInstruments = []reportInstrument{
	{"sp500", "S&P 500", "SPY", "etf_index"},
	{"gold", "Gold", "GLD", "etf"},
	{"oil", "Oil (WTI)", "USO", "etf"},
	{"us10y", "US 10-Year Treasury", "DGS10", "treasury_yield"},
	{"us5y", "US 5-Year Treasury", "DGS5", "treasury_yield"},
	{"us2y", "US 2-Year Treasury", "DGS2", "treasury_yield"},
	{"shell", "Shell", "SHEL", "equity"},
	{"bitcoin", "Bitcoin", "BTCUSDT", "crypto"},
}

// macroSeries are the FRED series the report reads: the header strip plus
// the three yield instruments.
var macroSeries = []string{"VIXCLS", "DGS10", "DEXUSEU", "DGS5", "DGS2"}

// stanceSections map a macro_derived prefix to its report section.
var stanceSections = []struct{ key, prefix, stance string }{
	{"monetary_policy", "mp_", "mp_stance"},
	{"growth_cycle", "gc_", "gc_stance"},
	{"inflation", "inf_", "inf_stance"},
	{"global_geopolitical", "gg_", "gg_stance"},
}

// reportStaleAfter: macro-analysis runs every 6h, so a report older than two
// runs means the pipeline has stopped, not that it is between runs.
const reportStaleAfter = 12 * time.Hour

type datedValue struct {
	Value float64 `json:"value"`
	AsOf  string  `json:"as_of"`
}

type priceInfo struct {
	Close     float64  `json:"close"`
	ChangePct *float64 `json:"change_pct"`
	AsOf      string   `json:"as_of"`
	// SessionClosed is false for an equity's bar dated today before the 16:00
	// New York close: Yahoo's newest daily bar is still forming during the
	// session, so its "close" is a live price. Crypto is always true — only
	// closed candles are read.
	SessionClosed bool `json:"session_closed"`
}

type instrumentOut struct {
	Key               string          `json:"key"`
	Label             string          `json:"label"`
	Symbol            string          `json:"symbol"`
	Type              string          `json:"type"`
	Source            string          `json:"source"`
	Price             *priceInfo      `json:"price,omitempty"`
	Yield             *datedValue     `json:"yield,omitempty"`
	MarketCycle       json.RawMessage `json:"market_cycle"`
	UnavailableReason *string         `json:"unavailable_reason,omitempty"`
}

type stanceSection struct {
	Score   *float64             `json:"score"`
	Label   *string              `json:"label"`
	AsOf    string               `json:"as_of"`
	Stance  json.RawMessage      `json:"stance"`
	Signals map[string]signalOut `json:"signals"`
}

type signalOut struct {
	Value   *float64        `json:"value"`
	AsOf    string          `json:"as_of"`
	Payload json.RawMessage `json:"payload"`
}

func (s *Server) handleMarketReport(w http.ResponseWriter, r *http.Request) {
	if body, ok := s.cache.get(marketReportCacheKey); ok {
		writeBody(w, http.StatusOK, body)
		return
	}
	now := s.cfg.Now().UTC()
	var fixed []store.InstrumentRef
	var earnings []string
	for _, in := range fixedInstruments {
		switch in.typ {
		case "treasury_yield":
			continue
		case "crypto":
			fixed = append(fixed, store.InstrumentRef{Symbol: in.symbol, Kind: "crypto"})
		default:
			fixed = append(fixed, store.InstrumentRef{Symbol: in.symbol, Kind: "equity"})
			if in.typ == "equity" {
				earnings = append(earnings, in.symbol) // ETFs report no earnings
			}
		}
	}
	in, err := s.cfg.Store.MarketReport(r.Context(), fixed, earnings, macroSeries, now)
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	body, err := json.Marshal(buildMarketReport(in, now, reportOpts{
		earningsCovered: toSet(s.cfg.EarningsCoveredSymbols),
		gprConfigured:   s.cfg.GPRSourceConfigured,
	}))
	if err != nil {
		s.cfg.Log.Error("momentum-api: encode market report", "err", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "internal_error"})
		return
	}
	s.cache.setFor(marketReportCacheKey, body, s.cfg.MarketReportCacheTTL)
	writeBody(w, http.StatusOK, body)
}

// reportOpts carries the configuration facts the report states about coverage.
type reportOpts struct {
	earningsCovered map[string]bool
	gprConfigured   bool
}

func toSet(xs []string) map[string]bool {
	m := map[string]bool{}
	for _, x := range xs {
		m[strings.ToUpper(strings.TrimSpace(x))] = true
	}
	return m
}

type earningsCoverage struct {
	Symbol string `json:"symbol"`
	// Status: upcoming (dates in the window), none_in_window (fetched, nothing
	// in the next 14 days) or not_ingested (data-macro-intel never fetches it).
	Status string `json:"status"`
	Note   string `json:"note,omitempty"`
}

type dataGap struct {
	Key  string `json:"key"`
	Note string `json:"note"`
}

// buildMarketReport is pure: every section's shape is testable from inputs.
func buildMarketReport(in store.MarketReportInputs, now time.Time, opts reportOpts) map[string]any {
	var generated time.Time
	for _, row := range in.Macro {
		if row.TS.After(generated) {
			generated = row.TS
		}
	}
	out := map[string]any{}
	if generated.IsZero() {
		out["report_date"], out["generated_at"] = nil, nil
		out["is_stale"] = true
	} else {
		out["report_date"] = generated.Format(time.DateOnly)
		out["generated_at"] = generated.Format(time.RFC3339)
		out["is_stale"] = now.Sub(generated) > reportStaleAfter
	}

	fred := func(id string) *datedValue {
		p, ok := in.Fred[id]
		if !ok {
			return nil
		}
		return &datedValue{Value: p.Value, AsOf: p.TS.Format(time.DateOnly)}
	}
	global := map[string]any{
		"macro": map[string]any{"vix": fred("VIXCLS"), "us10y_pct": fred("DGS10"), "eur_usd": fred("DEXUSEU")},
	}
	for _, sec := range stanceSections {
		global[sec.key] = buildStance(in.Macro, sec.prefix, sec.stance)
	}
	global["macro_correlations_regime"] = macroPayload(in.Macro, "mc_macro_correlation")
	global["market_cycle_composite"] = macroPayload(in.Macro, "mc_market_cycle")
	aa, hasAA := in.Macro["aa_reference_snapshot"]
	for _, k := range []string{"seasonality", "presidential_cycle", "intermarket"} {
		global[k] = pick(aa.Payload, k, hasAA)
	}
	// automation_status is the snapshot's reference_modules list, unchanged —
	// including every not_automated / needs_data / partial entry.
	global["automation_status"] = pick(aa.Payload, "reference_modules", hasAA)
	global["calendars"] = map[string]any{"economic": orEmpty(in.Economic)}
	global["news"] = orEmpty(in.Headlines)
	global["geopolitical_intel"] = map[string]any{"gpr": nullIfNil(in.GPR), "gdelt": nullIfNil(in.GDELT)}
	out["global"] = global

	instruments := []instrumentOut{}
	for _, fi := range fixedInstruments {
		instruments = append(instruments, buildInstrument(in, fi, "fixed_list", now))
	}
	fixedSyms := map[string]bool{}
	for _, fi := range fixedInstruments {
		fixedSyms[fi.symbol] = true
	}
	for _, wl := range in.Watchlist {
		if fixedSyms[wl.Symbol] {
			continue
		}
		label := wl.Symbol
		if wl.CompanyName != nil && *wl.CompanyName != "" {
			label = *wl.CompanyName
		}
		instruments = append(instruments, buildInstrument(in,
			reportInstrument{key: wl.Symbol, label: label, symbol: wl.Symbol, typ: "equity"}, "watchlist", now))
	}
	out["instruments"] = instruments
	out["earnings_calendar"] = orEmpty(in.Earnings)

	// Earnings coverage per equity instrument (ETFs, yields and crypto have no
	// earnings), so a symbol without dates is never silently omitted.
	var dated []struct {
		Symbol string `json:"symbol"`
	}
	_ = json.Unmarshal(orEmpty(in.Earnings), &dated)
	hasDate := map[string]bool{}
	for _, d := range dated {
		hasDate[d.Symbol] = true
	}
	coverage := []earningsCoverage{}
	for _, inst := range instruments {
		if inst.Type != "equity" {
			continue
		}
		switch {
		case hasDate[inst.Symbol]:
			coverage = append(coverage, earningsCoverage{Symbol: inst.Symbol, Status: "upcoming"})
		case opts.earningsCovered[inst.Symbol]:
			coverage = append(coverage, earningsCoverage{Symbol: inst.Symbol, Status: "none_in_window",
				Note: "No earnings date in the next 14 days."})
		default:
			coverage = append(coverage, earningsCoverage{Symbol: inst.Symbol, Status: "not_ingested",
				Note: "Earnings data not available for this symbol — data-macro-intel does not fetch it."})
		}
	}
	out["earnings_coverage"] = coverage

	// Known data gaps, each stated with its condition and cause.
	gaps := []dataGap{}
	if len(in.GPR) == 0 {
		if !opts.gprConfigured {
			gaps = append(gaps, dataGap{"gpr", "Geopolitical risk index (GPR): not ingested — GPR_CSV_URL is not configured for data-macro-intel."})
		} else {
			gaps = append(gaps, dataGap{"gpr", "Geopolitical risk index (GPR): configured, but no readings stored yet."})
		}
	}
	if in.EconomicStored30d == 0 {
		gaps = append(gaps, dataGap{"economic_calendar", "Economic calendar: no events stored in the last 30 days. " +
			"Last observed cause (2026-09-24): Finnhub's economic-calendar endpoint answered 403 (no access on the current plan)."})
	}
	if len(in.GDELT) == 0 {
		gaps = append(gaps, dataGap{"gdelt", "GDELT news tone: no readings stored."})
	}
	out["data_gaps"] = gaps
	return out
}

func buildStance(macro map[string]store.MacroRow, prefix, stanceMetric string) any {
	st, ok := macro[stanceMetric]
	if !ok {
		return nil
	}
	sec := stanceSection{Score: st.Value, AsOf: st.TS.Format(time.DateOnly), Stance: st.Payload, Signals: map[string]signalOut{}}
	var p struct {
		Stance string `json:"stance"`
	}
	if json.Unmarshal(st.Payload, &p) == nil && p.Stance != "" {
		sec.Label = &p.Stance
	}
	for name, row := range macro {
		if strings.HasPrefix(name, prefix) && name != stanceMetric {
			sec.Signals[name] = signalOut{Value: row.Value, AsOf: row.TS.Format(time.DateOnly), Payload: row.Payload}
		}
	}
	return sec
}

func macroPayload(macro map[string]store.MacroRow, metric string) any {
	row, ok := macro[metric]
	if !ok {
		return nil
	}
	return map[string]any{"score": row.Value, "as_of": row.TS.Format(time.DateOnly), "payload": row.Payload}
}

func pick(payload json.RawMessage, key string, ok bool) json.RawMessage {
	if !ok {
		return json.RawMessage("null")
	}
	var m map[string]json.RawMessage
	if json.Unmarshal(payload, &m) != nil {
		return json.RawMessage("null")
	}
	if v, ok := m[key]; ok {
		return v
	}
	return json.RawMessage("null")
}

func orEmpty(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage("[]")
	}
	return raw
}

func nullIfNil(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage("null")
	}
	return raw
}

func buildInstrument(in store.MarketReportInputs, fi reportInstrument, source string, now time.Time) instrumentOut {
	o := instrumentOut{Key: fi.key, Label: fi.label, Symbol: fi.symbol, Type: fi.typ, Source: source,
		MarketCycle: json.RawMessage("null")}
	reason := func(s string) { o.UnavailableReason = &s }

	if fi.typ == "treasury_yield" {
		if p, ok := in.Fred[fi.symbol]; ok {
			o.Yield = &datedValue{Value: p.Value, AsOf: p.TS.Format(time.DateOnly)}
		} else {
			reason(fmt.Sprintf("no %s observations in macro_fred", fi.symbol))
		}
		return o // market_cycle stays null: a yield has no price phase
	}

	bars := in.Closes[fi.symbol]
	if len(bars) == 0 {
		reason(fmt.Sprintf("no daily bars stored for %s", fi.symbol))
		return o
	}
	last := bars[len(bars)-1]
	// The session date is the bar's UTC calendar date for every source (Tiingo
	// stamps 00:00 UTC, Yahoo 13:30 UTC); converting to New York first would
	// move a Tiingo bar to the previous day.
	p := &priceInfo{Close: roundTo(last.Close, 4), AsOf: last.TS.UTC().Format(time.DateOnly), SessionClosed: true}
	if fi.typ != "crypto" {
		p.SessionClosed = sessionClosed(last.TS, now)
	}
	if len(bars) == 2 && bars[0].Close > 0 {
		c := roundTo((last.Close/bars[0].Close-1)*100, 2)
		p.ChangePct = &c
	}
	o.Price = p

	row, ok := in.Macro["mc_price_phase:"+fi.symbol]
	if !ok {
		reason(fmt.Sprintf("no market-cycle reading for %s yet (macro-analysis runs every 6h)", fi.symbol))
		return o
	}
	var mc map[string]json.RawMessage
	if json.Unmarshal(row.Payload, &mc) != nil {
		reason("market-cycle payload unreadable")
		return o
	}
	if ur, ok := mc["unavailable_reason"]; ok {
		var s string
		_ = json.Unmarshal(ur, &s)
		reason(s)
		return o
	}
	cycle := map[string]json.RawMessage{
		"phase":                  mc["price_phase"],
		"drawdown_from_peak_pct": mc["drawdown_pct"],
		"vs_200dma_pct":          mc["pct_vs_sma200"],
		"crash_velocity_flag":    mc["crash_warning"],
		"as_of":                  mc["as_of"],
		"as_of_basis":            mc["as_of_basis"],
		"windows":                mc["windows"],
	}
	b, _ := json.Marshal(cycle)
	o.MarketCycle = b
	return o
}

// sessionClosed: a bar for today's New York session is still forming until the
// close. The bar's session is its UTC calendar date (see buildInstrument).
func sessionClosed(barTS, now time.Time) bool {
	bar := barTS.UTC()
	n := now.In(newYork)
	if bar.Year() != n.Year() || bar.Month() != n.Month() || bar.Day() != n.Day() {
		return true
	}
	closeAt := time.Date(n.Year(), n.Month(), n.Day(), sessionCloseHour, 0, 0, 0, newYork)
	return !n.Before(closeAt)
}

func roundTo(v float64, places int) float64 {
	p := 1.0
	for i := 0; i < places; i++ {
		p *= 10
	}
	return float64(int64(v*p+0.5*sign(v))) / p
}

func sign(v float64) float64 {
	if v < 0 {
		return -1
	}
	return 1
}
