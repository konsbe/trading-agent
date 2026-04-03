// market-operations writes mo_reference_snapshot to macro_derived (source = market_operations).
// It encodes global vol regime from VIXCLS and static reference-module coverage for
// market_operations_reference.html. Per-symbol execution context (ATR%, volume vs median)
// is computed in analyst-bot at read time.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"github.com/konsbe/trading-agent/services/data-analyzer/internal/config"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/db"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/logx"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/marketops"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/store"
)

func main() {
	_ = godotenv.Load()
	cfg, err := config.LoadMarketOperations()
	if err != nil {
		slog.Error("config", "err", err)
		os.Exit(1)
	}
	log := logx.New(cfg.Base.LogLevel)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := db.Connect(ctx, cfg.Base.DatabaseURL)
	if err != nil {
		log.Error("db", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	if cfg.StartupDelay > 0 {
		log.Info("market-ops startup delay", "secs", cfg.StartupDelay.Seconds())
		select {
		case <-ctx.Done():
			return
		case <-time.After(cfg.StartupDelay):
		}
	}

	run := func() {
		if !cfg.Enabled {
			log.Debug("MARKET_OPS_ENABLE=false — skipping snapshot")
			return
		}
		ts := time.Now().UTC()
		payload, vixPtr := marketops.BuildSnapshot(ctx, pool, cfg)
		if err := store.UpsertMacroDerivedSource(ctx, pool, ts, marketops.MoMetric, vixPtr, payload, marketops.MoSource); err != nil {
			log.Error("upsert mo_reference_snapshot", "err", err)
			return
		}
		log.Info("market operations snapshot written", "metric", marketops.MoMetric)
	}

	run()

	ticker := time.NewTicker(cfg.PollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			log.Info("shutdown")
			return
		case <-ticker.C:
			run()
		}
	}
}
