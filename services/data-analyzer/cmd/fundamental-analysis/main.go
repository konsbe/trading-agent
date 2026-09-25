// fundamental-analysis reads raw fundamental metrics stored by data-ingestion/data-fundamental
// and derives the FA signals (Tier 1–3, qualitative, correlations) that the
// analyst-bot and momentum-api read.
//
// Data flow:
//
//	data-fundamental (data-ingestion)
//	  → equity_fundamentals (TimescaleDB, source = finnhub_*)
//	  → fundamental-analysis (this binary, data-analyzer)
//	  → equity_fundamentals (TimescaleDB, source = "fundamental_analysis")
//
// The per-symbol scoring lives in internal/fundamental (AnalyzeSymbol), the
// same code momentum-api runs on demand; this binary only loops it over
// FUNDAMENTAL_SYMBOLS on a ticker.
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

	"github.com/konsbe/trading-agent/services/data-analyzer/internal/config"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/db"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/fundamental"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/logx"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/store"
)

func main() {
	_ = godotenv.Load()
	cfg, err := config.LoadFundamentalAnalysis()
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

	// Allow data-fundamental time to complete its initial ingestion pass before
	// the first scoring run. Controlled by FUNDAMENTAL_STARTUP_DELAY_SECS
	// (default 30). Set to 0 when running against a pre-populated database.
	if delaySecs := fundamentalStartupDelay(); delaySecs > 0 {
		log.Info("waiting for data-fundamental backfill", "delay_secs", delaySecs)
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Duration(delaySecs) * time.Second):
		}
	}

	log.Info("running initial fundamental analysis")
	w.analyzeAll(ctx)

	ticker := time.NewTicker(cfg.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Info("shutdown")
			return
		case <-ticker.C:
			w.analyzeAll(ctx)
		}
	}
}

type worker struct {
	cfg  config.FundamentalAnalysis
	pool store.ReadWriter
	log  *slog.Logger
}

func (w *worker) analyzeAll(ctx context.Context) {
	for _, sym := range w.cfg.Symbols {
		res, err := fundamental.AnalyzeSymbol(ctx, w.pool, sym, w.cfg, w.log)
		if err != nil && !res.Scored {
			w.log.Error("query metrics", "symbol", sym, "err", err)
			continue
		}
		if !res.Scored {
			w.log.Debug("not enough metrics to score", "symbol", sym, "have", res.RawMetrics)
		}
	}
}

// fundamentalStartupDelay reads FUNDAMENTAL_STARTUP_DELAY_SECS from the
// environment. Defaults to 30 seconds — enough for data-fundamental to finish
// its first metrics + financials pass before the scoring worker runs.
// Set to 0 to skip the delay (useful when the DB is already populated).
func fundamentalStartupDelay() int {
	s := strings.TrimSpace(os.Getenv("FUNDAMENTAL_STARTUP_DELAY_SECS"))
	if s == "" {
		return 30
	}
	v, err := strconv.Atoi(s)
	if err != nil || v < 0 {
		return 30
	}
	return v
}
