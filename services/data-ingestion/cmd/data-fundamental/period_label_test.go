package main

import "testing"

// Report items from /stock/financials-reported have form, year, quarter and
// startDate/endDate, but no "freq". Shapes below are INTC's stored report_raw
// payloads. The old label read "freq", so every one of them came out
// annual_<year>: the Q1 FY2026 10-Q as annual_2026, and the FY2024 10-K and a
// 10-Q of 2024 both as annual_2024.
func TestFinancialPeriodLabel(t *testing.T) {
	cases := []struct {
		name    string
		report  map[string]any
		reqFreq string
		want    string
	}{
		{"INTC Q1 FY2026 10-Q", map[string]any{"form": "10-Q", "year": 2026.0, "quarter": 1.0,
			"startDate": "2025-12-28 00:00:00", "endDate": "2026-03-28 00:00:00"}, "quarterly", "q_2026-03-28"},
		{"INTC Q3 FY2023 10-Q (nine-month YTD)", map[string]any{"form": "10-Q", "year": 2023.0, "quarter": 3.0,
			"startDate": "2023-01-01 00:00:00", "endDate": "2023-09-30 00:00:00"}, "quarterly", "q_2023-09-30"},
		{"INTC FY2024 10-K", map[string]any{"form": "10-K", "year": 2024.0, "quarter": 0.0,
			"startDate": "2023-12-31 00:00:00", "endDate": "2024-12-28 00:00:00"}, "annual", "annual_2024"},
		{"10-K returned to a quarterly request", map[string]any{"form": "10-K", "year": 2024.0,
			"endDate": "2024-12-28 00:00:00"}, "quarterly", "annual_2024"},
		{"10-Q returned to an annual request", map[string]any{"form": "10-Q", "year": 2024.0,
			"endDate": "2024-03-30 00:00:00"}, "annual", "q_2024-03-30"},
		{"amended 10-K", map[string]any{"form": "10-K/A", "year": 2022.0}, "quarterly", "annual_2022"},
		{"20-F", map[string]any{"form": "20-F", "year": 2025.0}, "annual", "annual_2025"},
		{"no form: request freq decides", map[string]any{"year": 2025.0,
			"endDate": "2025-06-28 00:00:00"}, "quarterly", "q_2025-06-28"},
		{"10-Q without an end date", map[string]any{"form": "10-Q", "year": 2025.0}, "quarterly", "unknown"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := financialPeriodLabel(c.report, c.reqFreq); got != c.want {
				t.Errorf("financialPeriodLabel = %q, want %q", got, c.want)
			}
		})
	}
}
