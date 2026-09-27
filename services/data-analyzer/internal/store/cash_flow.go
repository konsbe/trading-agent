package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

// CashFlowStatement is the cash-flow section of a symbol's latest stored
// annual report (10-K) from Finnhub /stock/financials-reported: every line as
// filed, keyed by its XBRL concept without the taxonomy prefix. Only annual
// reports: a 10-Q's cash-flow figures are fiscal-year-to-date, not a quarter.
type CashFlowStatement struct {
	Period    string // annual_<year>
	Form      string
	EndDate   string
	FiledDate string
	Lines     map[string]float64
}

// StatementCoverage is what the ingestion recorded about a symbol's filings
// (symbol_data_status) and what the symbol is (followed_symbols), for saying
// why there is no statement.
type StatementCoverage struct {
	AssetType *string
	Listing   *string
	Status    *string
	Reason    *string
}

func LatestAnnualCashFlow(ctx context.Context, q Querier, symbol string) (*CashFlowStatement, error) {
	var period string
	var payload []byte
	err := q.QueryRow(ctx, `
SELECT period, payload FROM equity_fundamentals
WHERE symbol = $1 AND source = 'finnhub_financials_reported' AND metric = 'report_raw' AND period LIKE 'annual\_%'
ORDER BY period DESC, ts DESC LIMIT 1`, symbol).Scan(&period, &payload)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("cash flow %s: %w", symbol, err)
	}
	var raw struct {
		Form      string `json:"form"`
		EndDate   string `json:"endDate"`
		FiledDate string `json:"filedDate"`
		Report    struct {
			CF []struct {
				Concept string   `json:"concept"`
				Value   *float64 `json:"value"`
			} `json:"cf"`
		} `json:"report"`
	}
	if err := json.Unmarshal(payload, &raw); err != nil {
		return nil, fmt.Errorf("cash flow %s: payload: %w", symbol, err)
	}
	if len(raw.Report.CF) == 0 {
		return nil, nil
	}
	cf := &CashFlowStatement{Period: period, Form: raw.Form, EndDate: dateOnly(raw.EndDate), FiledDate: dateOnly(raw.FiledDate), Lines: map[string]float64{}}
	for _, l := range raw.Report.CF {
		if l.Value == nil {
			continue
		}
		concept := l.Concept
		if i := strings.IndexByte(concept, '_'); i >= 0 {
			concept = concept[i+1:] // us-gaap_NetCash… / msft_… → NetCash…
		}
		if _, dup := cf.Lines[concept]; !dup {
			cf.Lines[concept] = *l.Value
		}
	}
	return cf, nil
}

func dateOnly(s string) string {
	if len(s) >= 10 {
		return s[:10]
	}
	return s
}

func LoadStatementCoverage(ctx context.Context, q Querier, symbol string) (StatementCoverage, error) {
	var c StatementCoverage
	err := q.QueryRow(ctx, `
SELECT COALESCE(
           (SELECT asset_type FROM followed_symbols WHERE symbol = $1),
           (SELECT asset_type FROM computation_interest WHERE symbol = $1 AND active_until IS NULL LIMIT 1),
           (SELECT asset_type FROM symbol_directory WHERE symbol = $1 AND asset_type <> 'other' LIMIT 1)),
       COALESCE(
           (SELECT listing FROM followed_symbols WHERE symbol = $1),
           (SELECT CASE WHEN source = 'binance_spot' THEN 'crypto' ELSE 'us' END
              FROM symbol_directory WHERE symbol = $1 LIMIT 1)),
       (SELECT statements_status FROM symbol_data_status WHERE symbol = $1),
       (SELECT statements_reason FROM symbol_data_status WHERE symbol = $1)`, symbol).
		Scan(&c.AssetType, &c.Listing, &c.Status, &c.Reason)
	if err != nil {
		return c, fmt.Errorf("statement coverage %s: %w", symbol, err)
	}
	return c, nil
}

// IFRSCashFlow is a 20-F filer's cash-flow statement as data-fundamental
// stored it from SEC EDGAR companyfacts (source sec_edgar_20f): the card's
// lines keyed operating / investing / financing / capex / buybacks /
// dividends, in the filing's reporting currency, never converted. A line the
// filer tags with its own extension concept is absent from Lines.
type IFRSCashFlow struct {
	Form       string             `json:"form"`
	Accession  string             `json:"accession"`
	Filed      string             `json:"filed"`
	PeriodEnd  string             `json:"period_end"`
	FiscalYear int                `json:"fiscal_year"`
	Currency   string             `json:"currency"`
	Lines      map[string]float64 `json:"lines"`
	// Latest20F is the newest 20-F on EDGAR's filing index when the row was
	// stored; newer than PeriodEnd means companyfacts lags that filing.
	Latest20F *struct {
		Filed     string `json:"filed"`
		PeriodEnd string `json:"period_end"`
	} `json:"latest_20f"`
}

func LatestIFRSCashFlow(ctx context.Context, q Querier, symbol string) (*IFRSCashFlow, error) {
	var payload []byte
	err := q.QueryRow(ctx, `
SELECT payload FROM equity_fundamentals
WHERE symbol = $1 AND source = 'sec_edgar_20f' AND metric = 'cash_flow_statement' AND period LIKE 'annual\_%'
ORDER BY period DESC, ts DESC LIMIT 1`, symbol).Scan(&payload)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("20-F cash flow %s: %w", symbol, err)
	}
	var cf IFRSCashFlow
	if err := json.Unmarshal(payload, &cf); err != nil {
		return nil, fmt.Errorf("20-F cash flow %s: payload: %w", symbol, err)
	}
	if cf.Currency == "" || len(cf.Lines) == 0 {
		return nil, nil
	}
	return &cf, nil
}
