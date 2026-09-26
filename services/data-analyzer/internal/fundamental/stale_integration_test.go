//go:build integration

package fundamental

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/konsbe/trading-agent/services/data-analyzer/internal/config"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/store"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/testdb"
)

type fixtureRow struct {
	Symbol  string          `json:"symbol"`
	TS      string          `json:"ts"`
	Period  string          `json:"period"`
	Metric  string          `json:"metric"`
	Value   *float64        `json:"value"`
	Payload json.RawMessage `json:"payload"`
	Source  string          `json:"source"`
}

// seedStale loads testdata/stale_fixture.json: COHR's and INTC's raw rows and
// the derived rows that were still their latest on 2026-09-26.
func seedStale(t *testing.T, ctx context.Context, tx pgx.Tx) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "stale_fixture.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fx struct {
		Fundamentals []fixtureRow `json:"fundamentals"`
		StaleDerived []fixtureRow `json:"stale_derived"`
		Closes       []struct {
			Symbol string  `json:"symbol"`
			TS     string  `json:"ts"`
			Close  float64 `json:"close"`
		} `json:"closes"`
	}
	if err := json.Unmarshal(raw, &fx); err != nil {
		t.Fatal(err)
	}
	for _, r := range append(fx.Fundamentals, fx.StaleDerived...) {
		var payload any
		if len(r.Payload) > 0 && string(r.Payload) != "null" {
			payload = string(r.Payload)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO equity_fundamentals (ts, symbol, period, metric, value, payload, source)
			VALUES ($1::timestamptz, $2, $3, $4, $5, $6::jsonb, $7)`,
			r.TS, r.Symbol, r.Period, r.Metric, r.Value, payload, r.Source); err != nil {
			t.Fatalf("seed %s %s: %v", r.Symbol, r.Metric, err)
		}
	}
	for _, c := range fx.Closes {
		if _, err := tx.Exec(ctx, `INSERT INTO equity_ohlcv (ts, symbol, interval, open, high, low, close, volume, source)
			VALUES ($1::timestamptz, $2, '1Day', $3, $3, $3, $3, 1000, 'tiingo')`, c.TS, c.Symbol, c.Close); err != nil {
			t.Fatal(err)
		}
	}
}

// analyzeStale runs AnalyzeSymbol on the seeded symbol and returns its latest
// derived rows, payloads decoded.
func analyzeStale(t *testing.T, symbol string) map[string]derivedLatest {
	t.Helper()
	ctx := context.Background()
	t.Setenv("DATABASE_URL", "postgres://unused")
	cfg, err := config.LoadFundamentalAnalysis()
	if err != nil {
		t.Fatal(err)
	}
	tx := testdb.Tx(t)
	seedStale(t, ctx, tx)
	if _, err := AnalyzeSymbol(ctx, tx, symbol, cfg, slog.New(slog.NewTextHandler(io.Discard, nil))); err != nil {
		t.Fatal(err)
	}
	rows, err := store.QueryLatestDerived(ctx, tx, symbol)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]derivedLatest{}
	for _, r := range rows {
		d := derivedLatest{Value: r.Value}
		_ = json.Unmarshal(r.Payload, &d.Payload)
		out[r.Metric] = d
	}
	return out
}

type derivedLatest struct {
	Value   *float64
	Payload map[string]any
}

func (d derivedLatest) conditions(signal string) []string {
	var out []string
	sig, _ := d.Payload[signal].(map[string]any)
	arr, _ := sig["conditions_met"].([]any)
	for _, v := range arr {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func assertSuperseded(t *testing.T, got map[string]derivedLatest, metric, supersededTS string) {
	t.Helper()
	d, ok := got[metric]
	if !ok {
		t.Fatalf("%s: no row", metric)
	}
	if d.Value != nil || d.Payload["status"] != "not_computable" || d.Payload["tier"] != nil {
		t.Errorf("%s = %v %v, want nil value, status not_computable, no tier", metric, deref(d.Value), d.Payload)
	}
	if d.Payload["superseded_ts"] != supersededTS {
		t.Errorf("%s superseded_ts = %v, want %s", metric, d.Payload["superseded_ts"], supersededTS)
	}
}

// Live COHR on 2026-09-26: interest coverage −5.94 "high_risk" from the ×4
// code (FY2023 operating income −355.558 over FY2020 interest 59.899). Neither
// of COHR's two newest 10-Ks has an interest expense line, so the fixed code
// does not write the metric, and the old row stayed the latest: with Net
// Debt/EBITDA 16.9 "high_risk" it fired the credit-stress check and the
// leverage cycle warning's coverage condition.
func TestStaleRowIsSuperseded_COHRInterestCoverage(t *testing.T) {
	got := analyzeStale(t, "ZZCOHR")
	assertSuperseded(t, got, "t3_interest_coverage", "2026-09-25T22:19:29Z")

	if nd := got["t2_net_debt_ebitda"].Payload["tier"]; nd != "high_risk" {
		t.Fatalf("precondition: t2_net_debt_ebitda tier = %v, want high_risk", nd)
	}
	lev := got["corr_leverage_liquidity"]
	if c := lev.Payload["checks_run"]; c != 3.0 {
		t.Errorf("corr_leverage_liquidity checks_run = %v, want 3 (live bug: 4, the coverage check on the −5.94 row)", c)
	}
	if slices.Contains(got["corr_master_signals"].conditions("leverage_cycle_warning"), "interest_coverage_high_risk") {
		t.Error("leverage_cycle_warning still counts interest_coverage_high_risk from the superseded row")
	}
}

// Live INTC on 2026-09-26: FCF conversion 0.567 and DCF 452% from FY2021 FCF
// (11,258), written by pre-fix code. FY2025 FCF is −4,949, so the fixed code
// computes neither, and the FY2021 rows stayed the latest.
func TestStaleRowIsSuperseded_INTCFCFConversion(t *testing.T) {
	got := analyzeStale(t, "ZZINTC")
	assertSuperseded(t, got, "t3_fcf_conversion", "2026-09-26T20:24:19Z")
	assertSuperseded(t, got, "t3_dcf", "2026-09-26T20:24:19Z")
	if slices.Contains(got["corr_master_signals"].conditions("leverage_cycle_warning"), "fcf_poor_conversion") {
		t.Error("leverage_cycle_warning still counts fcf_poor_conversion from the FY2021 row")
	}
}

// Live INTC: all three margin trends "compressing", stored under "direction".
// scoreCorrelations read "tier", found nothing, and ran none of the three
// margin-trend checks (gross vs net, revenue growth vs gross, gross as demand).
func TestMarginTrendsReachCorrelations_INTC(t *testing.T) {
	got := analyzeStale(t, "ZZINTC")
	for _, m := range []string{"gross_margin_trend_8q", "net_margin_trend_8q"} {
		if d := got[m].Payload["direction"]; d != "compressing" {
			t.Fatalf("precondition: %s direction = %v, want compressing", m, d)
		}
	}
	// Revenue/EPS 0 (EPS weak, revenue neutral), both margins compressing −1,
	// revenue growth 7.47% vs gross trend 0: −1/3. Live: 1 check, 0.
	eq := got["corr_earnings_quality"]
	if c, s := eq.Payload["checks_run"], deref(eq.Value); c != 3.0 || s != -1.0/3 {
		t.Errorf("corr_earnings_quality = %v over %v checks, want -1/3 over 3 (live bug: 0 over 1)", s, c)
	}
	// ROIC vs revenue 0, gross margin compressing −1, capital-intensive with
	// FCF yield "avoid" −1: −2/3. Live: 2 checks, −1/2.
	op := got["corr_operational"]
	if c, s := op.Payload["checks_run"], deref(op.Value); c != 3.0 || s != -2.0/3 {
		t.Errorf("corr_operational = %v over %v checks, want -2/3 over 3 (live bug: -1/2 over 2)", s, c)
	}
}
