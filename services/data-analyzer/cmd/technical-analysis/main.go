// technical-analysis reads OHLCV bars that data-ingestion workers have already
// stored in TimescaleDB, runs every enabled indicator from the compute package,
// and writes results to the technical_indicators table.
//
// Data flow:
//
//	data-crypto / data-equity (data-ingestion)
//	  → equity_ohlcv / crypto_ohlcv (TimescaleDB)
//	  → technical-analysis (this binary, data-analyzer)
//	  → technical_indicators (TimescaleDB)
//
// TODO: migrate compute/ to Python (pandas-ta / ta-lib) and retire this binary.
// See internal/compute/doc.go for the migration checklist.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/config"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/db"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/logx"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/technical/runner"
)

func main() {
	_ = godotenv.Load()
	cfg, err := config.LoadTechnicalAnalysis()
	if err != nil {
		slog.Error("config", "err", err)
		os.Exit(1)
	}
	log := logx.New(cfg.LogLevel)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("db", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	w := &worker{cfg: cfg, pool: pool, log: log}

	// Allow data-ingestion workers time to complete their initial OHLCV backfill
	// before the first computation run. Controlled by ANALYZER_STARTUP_DELAY_SECS
	// (default 60). Set to 0 when running against a pre-populated database.
	if delaySecs := startupDelay(); delaySecs > 0 {
		log.Info("waiting for ingestion backfill", "delay_secs", delaySecs)
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Duration(delaySecs) * time.Second):
		}
	}

	log.Info("running initial indicator computation")
	w.computeAll(ctx)

	ticker := time.NewTicker(cfg.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Info("shutdown")
			return
		case <-ticker.C:
			w.computeAll(ctx)
		}
	}
}

type worker struct {
	cfg  config.TechnicalAnalysis
	pool *pgxpool.Pool
	log  *slog.Logger
}

// computeAll runs every enabled indicator for all configured symbols × intervals.
// It reads bars from the DB (written by data-ingestion workers) — it never calls
// any external API. The per-symbol work is runner.ComputeAndStore, the same code
// momentum-api runs on demand.
//
// TODO: each indicator block in internal/technical maps 1-to-1 to a pandas-ta
// call in Python. When migrating, keep the indicator name strings identical
// (e.g. "rsi_14", "macd_12_26_9") so the technical_indicators schema and
// downstream consumers need no changes.
func (w *worker) computeAll(ctx context.Context) {
	for _, sym := range w.cfg.EquitySymbols {
		for _, iv := range w.cfg.EquityIntervals {
			w.computeOne(ctx, sym, "equity", iv)
		}
	}
	for _, sym := range w.cfg.CryptoSymbols {
		for _, iv := range w.cfg.CryptoIntervals {
			w.computeOne(ctx, sym, "binance", iv)
		}
	}
}

func (w *worker) computeOne(ctx context.Context, sym, exchange, iv string) {
	res, err := runner.ComputeAndStore(ctx, w.pool, sym, exchange, iv, w.cfg, w.log)
	if err != nil && !res.Computed {
		w.log.Error("query bars", "symbol", sym, "exchange", exchange, "interval", iv, "err", err)
		return
	}
	if !res.Computed {
		w.log.Warn("not enough bars", "symbol", sym, "exchange", exchange, "interval", iv, "have", res.Bars)
	}
}

func startupDelay() int {
	s := strings.TrimSpace(os.Getenv("ANALYZER_STARTUP_DELAY_SECS"))
	if s == "" {
		return 60
	}
	v, err := strconv.Atoi(s)
	if err != nil || v < 0 {
		return 60
	}
	return v
}
