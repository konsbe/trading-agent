package fundamental

import (
	"context"
	"io"
	"log/slog"
	"math"
	"sort"
	"testing"

	"github.com/konsbe/trading-agent/services/data-analyzer/internal/config"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/store"
)

// filingRows is one SEC filing as data-fundamental stores it: a report_raw row
// carrying the span, and one row per figure (millions).
func filingRows(period, form, start, end string, vals map[string]float64) []store.FundamentalRow {
	payload := []byte(`{"form":"` + form + `","startDate":"` + start + ` 00:00:00","endDate":"` + end + ` 00:00:00"}`)
	rows := []store.FundamentalRow{{Period: period, Metric: "report_raw", Payload: payload, Source: xbrlSource}}
	for m, v := range vals {
		v := v
		rows = append(rows, store.FundamentalRow{Period: period, Metric: m, Value: &v, Source: xbrlSource})
	}
	return rows
}

// queryOrder sorts rows as QueryLatestMetrics returns them: metric, then period
// label ascending, which puts a metric's OLDEST period first.
func queryOrder(rows []store.FundamentalRow) []store.FundamentalRow {
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Metric != rows[j].Metric {
			return rows[i].Metric < rows[j].Metric
		}
		return rows[i].Period < rows[j].Period
	})
	return rows
}

// INTC as stored on 2026-09-26 (live DB): FY2021 and FY2025 10-Ks and the
// Q1 FY2026 10-Q, after migration 029 relabelled the 10-Q to its period end.
func intcRows() []store.FundamentalRow {
	var rows []store.FundamentalRow
	rows = append(rows, filingRows("annual_2021", "10-K", "2020-12-27", "2021-12-25", map[string]float64{
		"operating_income_reported": 19456, "tax_expense_reported": 1835, "pretax_income_reported": 21703,
		"total_assets_reported": 168406, "current_liabilities_reported": 27462,
		"total_debt_reported": 33510, "total_equity_reported": 95391,
		"fcf_reported": 11258, "revenue_reported": 79024,
	})...)
	rows = append(rows, filingRows("annual_2025", "10-K", "2024-12-29", "2025-12-27", map[string]float64{
		"operating_income_reported": -2214, "tax_expense_reported": 1531, "pretax_income_reported": 1557,
		"total_assets_reported": 211429, "current_liabilities_reported": 31575,
		"total_debt_reported": 44086, "total_equity_reported": 114281, "cash_reported": 14265,
		"fcf_reported": -4949, "revenue_reported": 52853,
	})...)
	rows = append(rows, filingRows("q_2026-03-28", "10-Q", "2025-12-28", "2026-03-28", map[string]float64{
		"operating_income_reported": -3136, "tax_expense_reported": 335, "pretax_income_reported": -3946,
		"total_assets_reported": 205332, "current_liabilities_reported": 26885,
		"total_debt_reported": 43027, "total_equity_reported": 111394, "cash_reported": 17247,
		"fcf_reported": -2540, "revenue_reported": 13577,
	})...)
	rows = append(rows, raw("market_cap", 658_490_100_000), raw("net_margin_ttm", -19.79))
	return queryOrder(rows)
}

func newTestAnalyzer(t *testing.T) (*analyzer, *recordingDB) {
	t.Helper()
	cfg, err := config.LoadFundamentalAnalysis()
	if err != nil {
		t.Fatal(err)
	}
	db := &recordingDB{values: map[string]*float64{}, payloads: map[string]map[string]any{}}
	return &analyzer{cfg: cfg, pool: db, log: slog.New(slog.NewTextHandler(io.Discard, nil))}, db
}

// Every pass kept the first value seen per metric, i.e. the oldest period:
// live INTC scored ROIC 50.5% "moat_quality" from FY2021 operating income
// (19,456, then ×4 as if quarterly) while its net margin was −19.8%. The old
// code gives exactly the live values on these rows: ROIC 50.55, D/E 0.3513,
// FCF yield 1.71%. Resolving oldest-first alone (×4 gone) still fails every
// assertion: D/E 0.3513, FCF yield 1.71%, ROIC 12.6% "adequate" from FY2021.
func TestINTCReadsNewestFilingNotOldest(t *testing.T) {
	w, db := newTestAnalyzer(t)
	rows := intcRows()
	w.score(context.Background(), "INTC", rows)
	w.scoreTier2(context.Background(), "INTC", rows)

	// ROIC from FY2025, one filing, no ×4: tax rate 1531/1557 is out of bounds
	// so the 21% default applies; −2214×0.79 / (211429−31575) = −0.97%.
	roic := db.values["t2_roic"]
	if roic == nil || math.Abs(*roic-(-0.9725)) > 0.01 {
		t.Errorf("t2_roic = %v, want ≈ -0.97 (live bug: 50.55 from FY2021 ×4)", deref(roic))
	}
	if tier := db.payloads["t2_roic"]["tier"]; tier != "low_roic" {
		t.Errorf("t2_roic tier = %v, want low_roic", tier)
	}
	if got := db.payloads["t2_roic"]["operating_income"]; got != -2214.0 {
		t.Errorf("t2_roic operating_income = %v, want -2214 (FY2025 10-K)", got)
	}

	// D/E from the newest balance sheet, the Q1 FY2026 10-Q: 43027/111394.
	de := db.values["t2_leverage"]
	if de == nil || math.Abs(*de-0.3863) > 0.001 {
		t.Errorf("t2_leverage = %v, want ≈ 0.3863 (live bug: 0.3513 from FY2021)", deref(de))
	}

	// FY2025 operating income is negative: Net Debt/EBITDA is not meaningful.
	if v := db.values["t2_net_debt_ebitda"]; v != nil {
		t.Errorf("t2_net_debt_ebitda = %v, want nil with operating income ≤ 0", *v)
	}
	if tier := db.payloads["t2_net_debt_ebitda"]["tier"]; tier != "negative_ebitda" {
		t.Errorf("t2_net_debt_ebitda tier = %v, want negative_ebitda", tier)
	}

	// FCF yield from FY2025 FCF: −4949 / 658,490.1 × 100.
	fy := db.values["fcf_yield"]
	if fy == nil || math.Abs(*fy-(-0.7516)) > 0.001 {
		t.Errorf("fcf_yield = %v, want ≈ -0.7516 (live bug: 1.7097 from FY2021)", deref(fy))
	}
}

// With no 10-K, a 10-Q is annualised by the days it covers, not ×4: Finnhub's
// 10-Q figures are fiscal-year-to-date (INTC's Q3 2023 revenue, 38,822, is
// nine months).
func TestYearFlowsAnnualisesAYTD10QBySpan(t *testing.T) {
	fs := buildFilings(filingRows("q_2023-09-30", "10-Q", "2023-01-01", "2023-09-30",
		map[string]float64{"revenue_reported": 38822}))
	v, basis, ok := annualFlow(fs, "revenue_reported")
	if !ok || math.Abs(v-38822*365.25/273) > 1 {
		t.Errorf("annualFlow = %v (%s), want 38822×365.25/273 ≈ 51940, not ×4 = 155288", v, basis)
	}
}

// A newer 10-Q does not displace the 10-K for flows, but does for the balance sheet.
func TestFilingChoiceByKind(t *testing.T) {
	fs := buildFilings(intcRows())
	if f, factor, _, ok := fs.yearFlows(); !ok || f.period != "annual_2025" || factor != 1 {
		t.Errorf("yearFlows = %s ×%v, want annual_2025 ×1", f.period, factor)
	}
	if f, ok := fs.balanceSheet(); !ok || f.period != "q_2026-03-28" {
		t.Errorf("balanceSheet = %s, want q_2026-03-28", f.period)
	}
}

// finnhub_earnings periods are "q_<date>": the surprise average must take the
// newest quarters, and latestValues the newest quarter's value.
func TestEarningsResolveNewestQuarter(t *testing.T) {
	var rows []store.FundamentalRow
	for i, p := range []string{"q_2025-03-31", "q_2025-06-30", "q_2025-09-30", "q_2025-12-31", "q_2026-03-31"} {
		v := float64(i + 1)
		rows = append(rows, store.FundamentalRow{Period: p, Metric: "eps_surprise_pct", Value: &v, Source: "finnhub_earnings"})
	}
	if got := latestValues(rows)["eps_surprise_pct"]; got != 5 {
		t.Errorf("latest eps_surprise_pct = %v, want 5 (q_2026-03-31)", got)
	}
	if got := chronological(rows, "eps_surprise_pct"); len(got) != 5 || got[0] != 1 || got[4] != 5 {
		t.Errorf("chronological = %v, want [1 2 3 4 5]", got)
	}
	w, db := newTestAnalyzer(t)
	w.cfg.SurpriseQuarters = 4
	w.score(context.Background(), "ZZ", queryOrder(rows))
	if avg := db.values["earnings_surprise_avg"]; avg == nil || *avg != 3.5 {
		t.Errorf("earnings_surprise_avg = %v, want 3.5 (newest 4; the oldest 4 give 2.5)", deref(avg))
	}
}
