// Package gpr fetches GPR-style CSV/TSV from a user-configured URL (Caldara & Iacoviello data).
// TODO [SCRAPE]: policyuncertainty.com HTML if CSV URL changes; TODO [LLM]: narrative overlay on GPR spikes.
package gpr

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/konsbe/trading-agent/services/data-ingestion/internal/httpclient"
)

// MonthlyRow is one parsed monthly GPR observation.
type MonthlyRow struct {
	MonthTS   time.Time
	GPRTotal  *float64
	GPRAct    *float64
	GPRThreat *float64
	Raw       []string
}

// FetchLatestMonthly downloads CSV/TSV and returns the last parseable data row.
func FetchLatestMonthly(ctx context.Context, csvURL string) (*MonthlyRow, error) {
	if csvURL == "" {
		return nil, fmt.Errorf("gpr: empty URL")
	}
	cli := httpclient.New(60 * time.Second)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, csvURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := cli.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gpr: HTTP %s", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 20<<20))
	if err != nil {
		return nil, err
	}
	return parseGPRCSV(string(body))
}

func parseGPRCSV(text string) (*MonthlyRow, error) {
	r := csv.NewReader(strings.NewReader(text))
	r.LazyQuotes = true
	r.FieldsPerRecord = -1
	rows, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("gpr csv: %w", err)
	}
	if len(rows) < 2 {
		return nil, fmt.Errorf("gpr: no data rows")
	}
	header := rows[0]
	colMonth := -1
	colGPR := -1
	colAct := -1
	colThreat := -1
	for i, h := range header {
		l := strings.ToLower(strings.TrimSpace(h))
		switch {
		case strings.Contains(l, "month") && colMonth < 0:
			colMonth = i
		case l == "gpr" || l == "gpr_index" || strings.Contains(l, "gpr") && colGPR < 0 && !strings.Contains(l, "act") && !strings.Contains(l, "threat"):
			colGPR = i
		case strings.Contains(l, "act"):
			colAct = i
		case strings.Contains(l, "threat"):
			colThreat = i
		}
	}
	// Fallback: first column = date, second = main index
	if colGPR < 0 && len(header) >= 2 {
		colGPR = 1
	}
	if colMonth < 0 {
		colMonth = 0
	}
	var last *MonthlyRow
	for _, row := range rows[1:] {
		if len(row) <= colMonth || len(row) <= colGPR {
			continue
		}
		monthStr := strings.TrimSpace(row[colMonth])
		if monthStr == "" {
			continue
		}
		ts, err := parseMonth(monthStr)
		if err != nil {
			continue
		}
		mr := &MonthlyRow{MonthTS: ts, Raw: row}
		if v, ok := parseFloat(row[colGPR]); ok {
			mr.GPRTotal = &v
		}
		if colAct >= 0 && colAct < len(row) {
			if v, ok := parseFloat(row[colAct]); ok {
				mr.GPRAct = &v
			}
		}
		if colThreat >= 0 && colThreat < len(row) {
			if v, ok := parseFloat(row[colThreat]); ok {
				mr.GPRThreat = &v
			}
		}
		last = mr
	}
	if last == nil {
		return nil, fmt.Errorf("gpr: could not parse any row")
	}
	return last, nil
}

func parseMonth(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	layouts := []string{
		"2006-01-02", "2006-01", "01/2006", "1/2006", "200601",
		"Jan-2006", "January 2006", "2006-01-01",
	}
	for _, ly := range layouts {
		if t, err := time.Parse(ly, s); err == nil {
			return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC), nil
		}
	}
	// Excel serial or year-month compact
	if len(s) == 6 {
		if y, err := strconv.Atoi(s[:4]); err == nil {
			if m, err := strconv.Atoi(s[4:]); err == nil && m >= 1 && m <= 12 {
				return time.Date(y, time.Month(m), 1, 0, 0, 0, 0, time.UTC), nil
			}
		}
	}
	return time.Time{}, fmt.Errorf("unknown month format %q", s)
}

func parseFloat(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}
