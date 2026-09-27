//go:build integration

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/konsbe/trading-agent/services/data-analyzer/internal/config"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/testdb"
)

var updateGolden = flag.Bool("update", false, "rewrite testdata/derived_golden.json from the current worker")

// Fixture symbols (testdata/fa_fixture.json): a fully covered watchlist symbol
// and a scanner candidate with finnhub_metric / finnhub_profile2 rows only.
var goldenSymbols = []string{"ZZFAFULL", "ZZFATHIN"}

type fixture struct {
	Fundamentals []struct {
		Symbol  string          `json:"symbol"`
		TS      string          `json:"ts"`
		Period  string          `json:"period"`
		Metric  string          `json:"metric"`
		Value   *float64        `json:"value"`
		Payload json.RawMessage `json:"payload"`
		Source  string          `json:"source"`
	} `json:"fundamentals"`
	Closes []struct {
		Symbol string  `json:"symbol"`
		TS     string  `json:"ts"`
		Close  float64 `json:"close"`
	} `json:"closes"`
}

func seedFixture(t *testing.T, ctx context.Context, tx pgx.Tx) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "fa_fixture.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fx fixture
	if err := json.Unmarshal(raw, &fx); err != nil {
		t.Fatal(err)
	}
	for _, r := range fx.Fundamentals {
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
	// Windowed inputs are placed relative to now() (the transaction start),
	// which is what the worker's NOW() windows read.
	for _, q := range []string{
		`INSERT INTO insider_transactions (ts, symbol, insider_name, transaction_code, shares)
		 VALUES (now() - interval '5 days', 'ZZFAFULL', 'Insider A', 'P', 100),
		        (now() - interval '6 days', 'ZZFAFULL', 'Insider B', 'P', 200),
		        (now() - interval '7 days', 'ZZFAFULL', 'Insider C', 'P', 300),
		        (now() - interval '8 days', 'ZZFAFULL', 'Insider D', 'S', 400),
		        (now() - interval '200 days', 'ZZFAFULL', 'Insider E', 'P', 500)`,
		// A derived row from an earlier run that this run cannot recompute
		// (ZZFATHIN has no filings): it must be superseded, not left latest.
		`INSERT INTO equity_fundamentals (ts, symbol, period, metric, value, payload, source)
		 VALUES ('2026-09-25 22:19:29+00', 'ZZFATHIN', 'derived', 't3_interest_coverage', -5.94,
		         '{"tier": "high_risk", "coverage_ratio": -5.94}', 'fundamental_analysis')`,
		`INSERT INTO news_headlines (ts, source, symbol, headline, sentiment)
		 VALUES (now() - interval '2 days', 'zz_fixture', 'ZZFAFULL', 'zz one', 0.40),
		        (now() - interval '3 days', 'zz_fixture', 'ZZFAFULL', 'zz two', 0.10),
		        (now() - interval '20 days', 'zz_fixture', 'ZZFAFULL', 'zz three', -0.30)`,
	} {
		if _, err := tx.Exec(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
}

type derivedRow struct {
	Symbol  string          `json:"symbol"`
	Metric  string          `json:"metric"`
	Period  string          `json:"period"`
	Value   *float64        `json:"value"`
	Payload json.RawMessage `json:"payload"`
}

// The worker's derived rows (metric names, values, payloads) are a live
// contract read by the analyst bot and momentum-api. This pins all of them for
// a fixed raw input, so moving the scoring code cannot change what it writes.
// ts is excluded: it is the wall clock at write time.
func TestAnalyzeMatchesGolden(t *testing.T) {
	ctx := context.Background()
	t.Setenv("DATABASE_URL", "postgres://unused")
	cfg, err := config.LoadFundamentalAnalysis()
	if err != nil {
		t.Fatal(err)
	}
	cfg.Symbols = goldenSymbols

	tx := testdb.Tx(t)
	seedFixture(t, ctx, tx)
	w := &worker{cfg: cfg, pool: tx, log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	w.analyzeAll(ctx, cfg.Symbols)

	rows, err := tx.Query(ctx, `SELECT symbol, metric, period, value, payload::text
		FROM equity_fundamentals
		WHERE symbol = ANY($1) AND source = 'fundamental_analysis'
		ORDER BY symbol, metric, period, ts`, goldenSymbols)
	if err != nil {
		t.Fatal(err)
	}
	var got []derivedRow
	for rows.Next() {
		var r derivedRow
		var payload *string
		if err := rows.Scan(&r.Symbol, &r.Metric, &r.Period, &r.Value, &payload); err != nil {
			t.Fatal(err)
		}
		if payload != nil {
			r.Payload = json.RawMessage(*payload)
		}
		got = append(got, r)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	perSymbol := map[string]int{}
	for _, r := range got {
		perSymbol[r.Symbol]++
	}
	if perSymbol["ZZFAFULL"] < 30 || perSymbol["ZZFATHIN"] < 5 {
		t.Fatalf("fixture exercises too little of the worker: %v", perSymbol)
	}

	body, err := json.MarshalIndent(got, "", " ")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join("testdata", "derived_golden.json")
	if *updateGolden {
		if err := os.WriteFile(path, body, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden (run with -update once to create it): %v", err)
	}
	if !bytes.Equal(body, want) {
		t.Fatalf("worker derived output changed; diff testdata/derived_golden.json against:\n%s", firstDiff(want, body))
	}
}

// ZZFAFULL is MSFT: fcf_eps_divergence stores "warning_eps_growing_fcf_low"
// under "quality" (EPS growth 31.56%, FCF yield 1.95%). scoreCorrelations read
// "tier" and matched values the worker never writes ("accruals_concern"), so
// live MSFT's earnings quality ran 1 check and read "healthy". The low FCF
// yield is MSFT's price (FCF conversion 0.70), so the deterioration warning's
// FCF condition, which needs cash weak against earnings, stays unmet.
func TestFCFEPSDivergenceReachesCorrelations(t *testing.T) {
	ctx := context.Background()
	t.Setenv("DATABASE_URL", "postgres://unused")
	cfg, err := config.LoadFundamentalAnalysis()
	if err != nil {
		t.Fatal(err)
	}
	cfg.Symbols = []string{"ZZFAFULL"}
	tx := testdb.Tx(t)
	seedFixture(t, ctx, tx)
	w := &worker{cfg: cfg, pool: tx, log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	w.analyzeAll(ctx, cfg.Symbols)

	payload := func(metric string) map[string]any {
		var raw []byte
		if err := tx.QueryRow(ctx, `SELECT payload FROM equity_fundamentals
			WHERE symbol = 'ZZFAFULL' AND source = 'fundamental_analysis' AND metric = $1
			ORDER BY ts DESC LIMIT 1`, metric).Scan(&raw); err != nil {
			t.Fatalf("%s: %v", metric, err)
		}
		var p map[string]any
		_ = json.Unmarshal(raw, &p)
		return p
	}
	if q := payload("fcf_eps_divergence")["quality"]; q != "warning_eps_growing_fcf_low" {
		t.Fatalf("precondition: fcf_eps_divergence quality = %v", q)
	}
	eq := payload("corr_earnings_quality")
	if eq["checks_run"] != 2.0 || len(eq["warnings"].([]any)) != 1 {
		t.Errorf("corr_earnings_quality checks_run %v warnings %v, want 2 checks and the FCF/EPS warning (live bug: 1, none)", eq["checks_run"], eq["warnings"])
	}
	det, _ := payload("corr_master_signals")["deterioration_warning"].(map[string]any)
	conds, _ := det["conditions_met"].([]any)
	for _, c := range conds {
		if c == "fcf_accruals_concern" {
			t.Errorf("deterioration_warning conditions %v: fcf_accruals_concern met on FCF yield alone (conversion 0.70)", conds)
		}
	}
}

func firstDiff(a, b []byte) string {
	la, lb := bytes.Split(a, []byte("\n")), bytes.Split(b, []byte("\n"))
	for i := 0; i < len(la) && i < len(lb); i++ {
		if !bytes.Equal(la[i], lb[i]) {
			return "line " + strconv.Itoa(i+1) + ":\n  want " + string(la[i]) + "\n  got  " + string(lb[i])
		}
	}
	return "length differs"
}
