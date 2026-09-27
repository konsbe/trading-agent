//go:build integration

package store

import (
	"context"
	"testing"
	"time"
)

// The newest annual statement wins, quarters are never read, and the taxonomy
// prefix is stripped from concepts.
func TestLatestAnnualCashFlow_NewestAnnualOnly(t *testing.T) {
	ctx := context.Background()
	tx := fixtureTx(t)
	ts := time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC)
	if _, err := tx.Exec(ctx, `INSERT INTO equity_fundamentals (ts, symbol, period, metric, payload, source) VALUES
		($1, 'ZZCF', 'annual_2097', 'report_raw', '{"form":"10-K","endDate":"2097-12-31 00:00:00","report":{"cf":[{"concept":"us-gaap_NetCashProvidedByUsedInOperatingActivities","value":1}]}}', 'finnhub_financials_reported'),
		($1, 'ZZCF', 'annual_2098', 'report_raw', '{"form":"10-K","endDate":"2098-12-31 00:00:00","filedDate":"2099-02-01 00:00:00","report":{"cf":[{"concept":"us-gaap_NetCashProvidedByUsedInOperatingActivities","value":2},{"concept":"zzcf_PaymentsForRepurchaseOfCommonStock","value":3}]}}', 'finnhub_financials_reported'),
		($1, 'ZZCF', 'quarterly_2099Q1', 'report_raw', '{"form":"10-Q","report":{"cf":[{"concept":"us-gaap_NetCashProvidedByUsedInOperatingActivities","value":9}]}}', 'finnhub_financials_reported')`, ts); err != nil {
		t.Fatal(err)
	}
	cf, err := LatestAnnualCashFlow(ctx, tx, "ZZCF")
	if err != nil || cf == nil {
		t.Fatalf("cash flow = %v, %v", cf, err)
	}
	if cf.Period != "annual_2098" || cf.EndDate != "2098-12-31" || cf.FiledDate != "2099-02-01" {
		t.Errorf("statement = %+v", cf)
	}
	if cf.Lines["NetCashProvidedByUsedInOperatingActivities"] != 2 || cf.Lines["PaymentsForRepurchaseOfCommonStock"] != 3 {
		t.Errorf("lines = %v", cf.Lines)
	}
	if none, err := LatestAnnualCashFlow(ctx, tx, "ZZCFNONE"); err != nil || none != nil {
		t.Errorf("no statement = %v, %v; want nil, nil", none, err)
	}
}

// Asset type comes from followed_symbols, else an open computation reason,
// else the directory — so an unfollowed fund still reads as a fund.
func TestLoadStatementCoverage_AssetTypeFallbacks(t *testing.T) {
	ctx := context.Background()
	tx := fixtureTx(t)
	if _, err := tx.Exec(ctx, `
		INSERT INTO followed_symbols (symbol, asset_type, listing) VALUES ('ZZCV1', 'equity', 'foreign');
		INSERT INTO computation_interest (symbol, asset_type, reason) VALUES ('ZZCV2', 'etf', 'manual');
		INSERT INTO symbol_directory (source, symbol, asset_type) VALUES ('finnhub_us', 'ZZCV3', 'etf');
		INSERT INTO symbol_data_status (symbol, statements_status, statements_reason) VALUES ('ZZCV3', 'none_returned', 'r')`); err != nil {
		t.Fatal(err)
	}
	for sym, want := range map[string][2]string{
		"ZZCV1": {"equity", "foreign"},
		"ZZCV2": {"etf", ""},
		"ZZCV3": {"etf", "us"},
		"ZZCV4": {"", ""},
	} {
		c, err := LoadStatementCoverage(ctx, tx, sym)
		if err != nil {
			t.Fatal(err)
		}
		deref := func(p *string) string {
			if p == nil {
				return ""
			}
			return *p
		}
		if deref(c.AssetType) != want[0] || deref(c.Listing) != want[1] {
			t.Errorf("%s: asset_type %q listing %q, want %v", sym, deref(c.AssetType), deref(c.Listing), want)
		}
	}
}

// The newest stored 20-F row is read, and a 10-K statement is never shadowed:
// LoadAnalysis only asks for it when there is none.
func TestLatestIFRSCashFlow(t *testing.T) {
	ctx := context.Background()
	tx := fixtureTx(t)
	ts := time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC)
	if _, err := tx.Exec(ctx, `INSERT INTO equity_fundamentals (ts, symbol, period, metric, payload, source) VALUES
		($1, 'ZZ20F', 'annual_2023', 'cash_flow_statement', '{"form":"20-F","period_end":"2023-12-31","fiscal_year":2023,"currency":"TWD","lines":{"operating":1}}', 'sec_edgar_20f'),
		($1, 'ZZ20F', 'annual_2024', 'cash_flow_statement', '{"form":"20-F","filed":"2025-04-17","period_end":"2024-12-31","fiscal_year":2024,"currency":"TWD","lines":{"operating":2},"latest_20f":{"filed":"2026-04-16","period_end":"2025-12-31"}}', 'sec_edgar_20f')`, ts); err != nil {
		t.Fatal(err)
	}
	cf, err := LatestIFRSCashFlow(ctx, tx, "ZZ20F")
	if err != nil || cf == nil {
		t.Fatalf("= %v, %v", cf, err)
	}
	if cf.FiscalYear != 2024 || cf.Currency != "TWD" || cf.Lines["operating"] != 2 || cf.Latest20F == nil || cf.Latest20F.Filed != "2026-04-16" {
		t.Errorf("statement = %+v latest %+v", cf, cf.Latest20F)
	}
	if none, err := LatestIFRSCashFlow(ctx, tx, "ZZ20FNONE"); err != nil || none != nil {
		t.Errorf("no row = %v, %v", none, err)
	}
}
