//go:build integration

package momentumapi

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"math"
	"net/http"
	"testing"
	"time"

	"github.com/konsbe/trading-agent/services/data-analyzer/internal/config"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/fundamental"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/store"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/technical"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/technical/runner"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/testdb"
)

// End to end over real SQL: a scanner candidate with bars and finnhub_metric
// rows but no stored analysis goes computing → ready, through the workers' own
// per-symbol code, inside a transaction that is always rolled back.
func TestIntegration_AnalysisComputesOnDemandThenServesStoredRows(t *testing.T) {
	ctx := context.Background()
	pool := testdb.Pool(t)
	tx := testdb.Tx(t)
	t.Setenv("DATABASE_URL", "postgres://unused")
	taCfg, err := config.LoadTechnicalAnalysis()
	if err != nil {
		t.Fatal(err)
	}
	faCfg, err := config.LoadFundamentalAnalysis()
	if err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	day := time.Date(2099, 1, 2, 0, 0, 0, 0, time.UTC)
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := tx.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}
	exec(`INSERT INTO universe_symbols (symbol, exchange, name, is_eligible) VALUES ('ZZAN01', 'NYSE', 'Analysis One', true)`)
	exec(`INSERT INTO momentum_features (ts, symbol, close, rvol_20, bucket, gates_passed) VALUES ($1, 'ZZAN01', 10, 3, 'market', true)`, day)
	price := 20.0
	t0 := day.AddDate(0, 0, -299)
	for i := 0; i < 300; i++ {
		c := price * (1 + 0.02*math.Sin(float64(i)/6))
		exec(`INSERT INTO equity_ohlcv (ts, symbol, interval, open, high, low, close, volume, source)
			VALUES ($1, 'ZZAN01', '1Day', $2, $3, $4, $5, 1e6, 'tiingo')`,
			t0.AddDate(0, 0, i), price, math.Max(price, c)*1.01, math.Min(price, c)*0.99, c)
		price = c
	}
	for metric, v := range map[string]float64{
		"eps_growth_ttm_yoy": 22, "revenue_growth_ttm_yoy": 12, "pe_ratio_ttm": 18, "pe_ratio_5y_avg": 25,
		"gross_margin_ttm": 45, "net_margin_ttm": 12, "roe_ttm": 18, "current_ratio": 1.8, "market_cap": 2.5e9,
	} {
		exec(`INSERT INTO equity_fundamentals (ts, symbol, period, metric, value, source)
			VALUES (now() - interval '2 days', 'ZZAN01', 'ttm', $1, $2, 'finnhub_metric')`, metric, v)
	}

	srv := NewServer(Config{
		Store:           DBStore{Q: tx, PingFn: pool.Ping},
		Caveats:         loadSharedCaveats(t),
		CorrelationText: loadSharedCorrelationText(t),
		Log:             log,
		CacheTTL:        5 * time.Minute,
		AnalysisNames:   technical.NamesFor(technical.Emitter{Cfg: taCfg}),
		AnalysisCompute: func(ctx context.Context, sym string, parts AnalysisParts) error {
			var errs []error
			if parts.Technical {
				_, err := runner.ComputeAndStore(ctx, tx, sym, "equity", AnalysisInterval, taCfg, log)
				errs = append(errs, err)
			}
			if parts.Fundamentals {
				_, err := fundamental.AnalyzeSymbol(ctx, tx, sym, faCfg, log)
				errs = append(errs, err)
			}
			return errors.Join(errs...)
		},
	})

	fr, err := store.AnalysisFreshnessFor(ctx, tx, "ZZAN01", AnalysisInterval)
	if err != nil || fr.LatestBarTS == nil || fr.TechnicalTS != nil || fr.DerivedTS != nil || fr.RawTS == nil {
		t.Fatalf("before: %+v %v", fr, err)
	}

	rec := get(t, srv, "/api/v1/scanner/today/zzan01/analysis")
	if rec.Code != http.StatusAccepted || decode(t, rec)["status"] != "computing" {
		t.Fatalf("first view: %d %s", rec.Code, rec.Body)
	}
	srv.analysis.wg.Wait()

	fr, err = store.AnalysisFreshnessFor(ctx, tx, "ZZAN01", AnalysisInterval)
	if err != nil || fr.TechnicalTS == nil || !fr.TechnicalTS.Equal(*fr.LatestBarTS) || fr.DerivedTS == nil {
		t.Fatalf("after: %+v %v", fr, err)
	}
	rec = get(t, srv, "/api/v1/scanner/today/ZZAN01/analysis")
	if rec.Code != http.StatusOK {
		t.Fatalf("second view: %d %s", rec.Code, rec.Body)
	}
	body := decode(t, rec)
	if body["status"] != "ready" || body["as_of"] != fr.LatestBarTS.Format(time.DateOnly) {
		t.Errorf("status %v as_of %v", body["status"], body["as_of"])
	}
	if s := obj(t, body, "sections"); s["technical"] != "ready" || s["fundamentals"] != "ready" {
		t.Errorf("sections = %v", s)
	}
	var rsi float64
	if err := tx.QueryRow(ctx, `SELECT value FROM technical_indicators WHERE symbol = 'ZZAN01' AND indicator = 'rsi_14'`).Scan(&rsi); err != nil {
		t.Fatal(err)
	}
	if got := obj(t, body, "technical", "rsi_14")["value"]; got != rsi {
		t.Errorf("rsi_14 = %v, stored %v", got, rsi)
	}
	fa := obj(t, body, "fundamentals")
	if fa["eps_strength"] != "strong" || fa["market_cap"] != 2.5e9 || obj(t, fa, "pe_vs_5y")["band"] != "cheap_vs_history" {
		t.Errorf("fundamentals = %v", fa)
	}
	if obj(t, body, "balance_sheet", "roe")["band"] != "excellent" {
		t.Errorf("balance_sheet = %v", body["balance_sheet"])
	}
	if body["scanner_data"] != true {
		t.Errorf("scanner_data = %v for a scanned symbol", body["scanner_data"])
	}

	// Never scanned, but has daily bars: served, flagged scanner_data=false.
	// A single bar is too little to compute from, so it settles as ready/no_data.
	exec(`INSERT INTO equity_ohlcv (ts, symbol, interval, open, high, low, close, volume, source)
		VALUES ($1, 'ZZAN02', '1Day', 10, 11, 9, 10.5, 1e6, 'tiingo')`, day)
	rec = get(t, srv, "/api/v1/scanner/today/ZZAN02/analysis")
	if b := decode(t, rec); rec.Code != http.StatusAccepted || b["status"] != "computing" || b["scanner_data"] != false {
		t.Fatalf("unscanned first view: %d %s", rec.Code, rec.Body)
	}
	srv.analysis.wg.Wait()
	rec = get(t, srv, "/api/v1/scanner/today/ZZAN02/analysis")
	if b := decode(t, rec); rec.Code != http.StatusOK || b["status"] != "ready" || b["scanner_data"] != false {
		t.Fatalf("unscanned second view: %d %s", rec.Code, rec.Body)
	}
	if rec := get(t, srv, "/api/v1/scanner/today/ZZAN02"); rec.Code != http.StatusNotFound {
		t.Errorf("GET /today/ZZAN02 = %d, want 404 (no scanner row)", rec.Code)
	}
	// No daily bars at all: 404.
	rec = get(t, srv, "/api/v1/scanner/today/ZZAN03/analysis")
	if rec.Code != http.StatusNotFound || decode(t, rec)["error"] != "no_data_for_symbol" {
		t.Errorf("no bars: %d %s", rec.Code, rec.Body)
	}
}
