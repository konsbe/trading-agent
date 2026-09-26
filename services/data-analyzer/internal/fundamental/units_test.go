package fundamental

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"math"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/konsbe/trading-agent/services/data-analyzer/internal/config"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/store"
)

// recordingDB keeps every derived upsert and answers no query.
type recordingDB struct {
	values   map[string]*float64
	payloads map[string]map[string]any
}

func (r *recordingDB) Exec(_ context.Context, _ string, args ...any) (pgconn.CommandTag, error) {
	metric := args[3].(string)
	r.values[metric] = args[4].(*float64)
	var p map[string]any
	if b, ok := args[5].([]byte); ok && b != nil {
		_ = json.Unmarshal(b, &p)
	}
	r.payloads[metric] = p
	return pgconn.CommandTag{}, nil
}

func (r *recordingDB) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, errors.New("no queries in this test")
}

func (r *recordingDB) QueryRow(context.Context, string, ...any) pgx.Row { return errRow{} }

type errRow struct{}

func (errRow) Scan(...any) error { return errors.New("no queries in this test") }

func raw(metric string, v float64) store.FundamentalRow {
	return store.FundamentalRow{TS: time.Date(2026, 9, 22, 0, 0, 0, 0, time.UTC), Period: "ttm", Metric: metric, Value: &v}
}

// market_cap is stored in USD; every FCF and revenue figure is in millions.
// MSFT-like inputs must give a percent-scale FCF yield and a single-digit-tens
// P/S, not values 1e6 off (the bug logged in data_analyzer.md, 2026-09-25).
func TestMarketCapIsReadInMillions(t *testing.T) {
	cfg, err := config.LoadFundamentalAnalysis()
	if err != nil {
		t.Fatal(err)
	}
	db := &recordingDB{values: map[string]*float64{}, payloads: map[string]map[string]any{}}
	w := &analyzer{cfg: cfg, pool: db, log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	rows := []store.FundamentalRow{
		raw("market_cap", 3_663_170_000_000), // USD
		raw("revenue_ttm", 168_090),          // millions
		raw("eps_growth_5y", 14.57),
		raw("revenue_growth_5y", 15),
	}
	rows = append(rows, filingRows("annual_2021", "10-K", "2020-07-01", "2021-06-30",
		map[string]float64{"fcf_reported": 56_118})...) // millions

	w.score(context.Background(), "MSFT", rows)
	w.scoreTier3(context.Background(), "MSFT", rows)

	fy := db.values["fcf_yield"]
	if fy == nil || math.Abs(*fy-1.532) > 0.01 {
		t.Errorf("fcf_yield = %v, want ≈ 1.532 (%%)", deref(fy))
	}
	ps := db.values["t3_ps_ratio"]
	if ps == nil || math.Abs(*ps-21.79) > 0.05 {
		t.Errorf("t3_ps_ratio = %v, want ≈ 21.79", deref(ps))
	}
	dcf := db.values["t3_dcf"]
	if dcf == nil || *dcf < 10 || *dcf > 10_000 {
		t.Errorf("t3_dcf market_cap_vs_dcf_pct = %v, want a percent in [10, 10000]", deref(dcf))
	}
	if got := db.payloads["t3_dcf"]["market_cap_millions"]; got != 3_663_170.0 {
		t.Errorf("t3_dcf payload market_cap_millions = %v, want 3663170", got)
	}
}

func TestMarketCapMillionsRejectsMissingOrNonPositive(t *testing.T) {
	for _, m := range []map[string]float64{{}, {"market_cap": 0}, {"market_cap": -5}} {
		if _, ok := marketCapMillions(m); ok {
			t.Errorf("marketCapMillions(%v) ok = true", m)
		}
	}
}

// countRow answers every QueryRow with one integer (the insider coverage and
// buyer/seller counts all read as n).
type countRow struct{ n int }

func (c countRow) Scan(dest ...any) error {
	for _, d := range dest {
		if p, ok := d.(*int); ok {
			*p = c.n
		}
	}
	return nil
}

type countingRowsDB struct {
	recordingDB
	row pgx.Row
}

func (c *countingRowsDB) QueryRow(context.Context, string, ...any) pgx.Row { return c.row }

func insiderSignal(t *testing.T, row pgx.Row) (*float64, map[string]any) {
	t.Helper()
	cfg, err := config.LoadFundamentalAnalysis()
	if err != nil {
		t.Fatal(err)
	}
	db := &countingRowsDB{recordingDB: recordingDB{values: map[string]*float64{}, payloads: map[string]map[string]any{}}, row: row}
	w := &analyzer{cfg: cfg, pool: db, log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	w.scoreQualitative(context.Background(), "MSFT", []store.FundamentalRow{raw("roe_ttm", 30)})
	if _, ok := db.payloads["qual_insider_signal"]; !ok {
		t.Fatal("qual_insider_signal not written")
	}
	return db.values["qual_insider_signal"], db.payloads["qual_insider_signal"]
}

// With no Form 4 rows for the symbol, insider activity is missing data, not
// "neutral" (Stock Detail showed "0.00 (neutral)" while the table was empty).
func TestInsiderSignal_NoCoverageIsInsufficientData(t *testing.T) {
	v, p := insiderSignal(t, countRow{n: 0})
	if v != nil || p["tier"] != "insufficient_data" {
		t.Errorf("no coverage: value %v tier %v, want nil / insufficient_data", deref(v), p["tier"])
	}
}

func TestInsiderSignal_QueryFailureIsInsufficientData(t *testing.T) {
	v, p := insiderSignal(t, errRow{})
	if v != nil || p["tier"] != "insufficient_data" {
		t.Errorf("query error: value %v tier %v, want nil / insufficient_data", deref(v), p["tier"])
	}
}

// A symbol with coverage is classified as before. The fake answers every count
// with 1 (one row of coverage, one buyer), so the tier is a real one.
func TestInsiderSignal_CoveredSymbolIsClassified(t *testing.T) {
	v, p := insiderSignal(t, countRow{n: 1})
	if v == nil || p["tier"] == "insufficient_data" {
		t.Errorf("covered symbol: value %v tier %v, want a computed classification", deref(v), p["tier"])
	}
}

func deref(p *float64) any {
	if p == nil {
		return nil
	}
	return *p
}
