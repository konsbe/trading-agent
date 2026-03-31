// data-technical fetches daily and weekly OHLCV bars that the other ingestion
// workers do not cover:
//
//   - data-equity fetches intraday (hourly) equity bars via Alpaca.
//   - data-crypto fetches a single configurable interval via Binance.
//   - data-technical fills the gap with daily/weekly bars for both asset classes.
//
// Sources:
//   Equity — Yahoo Finance (primary, free), Alpaca Data (fallback).
//   Crypto — Binance REST.
//
// All bars are written to equity_ohlcv / crypto_ohlcv.
// Indicator computation has been moved to services/data-analyzer/cmd/technical-analysis.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"github.com/berdelis/trading-agent/services/data-ingestion/internal/config"
	"github.com/berdelis/trading-agent/services/data-ingestion/internal/db"
	"github.com/berdelis/trading-agent/services/data-ingestion/internal/fetch/alpacadata"
	"github.com/berdelis/trading-agent/services/data-ingestion/internal/fetch/binance"
	"github.com/berdelis/trading-agent/services/data-ingestion/internal/fetch/yahoo"
	"github.com/berdelis/trading-agent/services/data-ingestion/internal/logx"
	"github.com/berdelis/trading-agent/services/data-ingestion/internal/store"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	_ = godotenv.Load()
	cfg, err := config.LoadOHLCVBars()
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

	w := &worker{
		cfg:   cfg,
		pool:  pool,
		yahoo: yahoo.New(),
		alp:   alpacadata.New(cfg.AlpacaKey, cfg.AlpacaSecret),
		bin:   binance.NewREST(),
		log:   log,
	}

	log.Info("backfilling historical bars")
	w.backfill(ctx)

	ticker := time.NewTicker(cfg.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Info("shutdown")
			return
		case <-ticker.C:
			w.fetchLatest(ctx)
		}
	}
}

type worker struct {
	cfg   config.OHLCVBars
	pool  *pgxpool.Pool
	yahoo *yahoo.Client
	alp   *alpacadata.Client
	bin   *binance.REST
	log   *slog.Logger
}

// backfill ensures each symbol × interval has at least BackfillBars rows.
// Existing rows are preserved via ON CONFLICT DO UPDATE.
func (w *worker) backfill(ctx context.Context) {
	for _, sym := range w.cfg.EquitySymbols {
		for _, iv := range w.cfg.EquityIntervals {
			n, err := store.CountEquityBars(ctx, w.pool, sym, iv)
			if err != nil {
				w.log.Error("count equity bars", "symbol", sym, "interval", iv, "err", err)
				continue
			}
			if n >= w.cfg.BackfillBars {
				w.log.Debug("equity backfill not needed", "symbol", sym, "interval", iv, "have", n)
				continue
			}
			w.log.Info("backfilling equity via Yahoo Finance", "symbol", sym, "interval", iv, "have", n, "want", w.cfg.BackfillBars)
			bars, err := w.yahoo.FetchBars(ctx, sym, iv, w.cfg.BackfillBars)
			if err != nil {
				w.log.Error("yahoo backfill fetch", "symbol", sym, "interval", iv, "err", err)
			}
			if len(bars) == 0 && w.alp.HasCredentials() {
				w.log.Warn("Yahoo returned 0 bars, trying Alpaca fallback", "symbol", sym, "interval", iv)
				bars, err = w.alp.FetchLatestBars(ctx, sym, iv, w.cfg.BackfillBars)
				if err != nil {
					w.log.Error("alpaca backfill fetch", "symbol", sym, "interval", iv, "err", err)
					continue
				}
			}
			if len(bars) == 0 {
				w.log.Warn("equity backfill: no bars from any source", "symbol", sym, "interval", iv)
				continue
			}
			if err := store.UpsertEquityOHLCV(ctx, w.pool, bars); err != nil {
				w.log.Error("equity backfill upsert", "symbol", sym, "err", err)
			} else {
				w.log.Info("equity backfill done", "symbol", sym, "interval", iv, "bars", len(bars))
			}
		}
	}

	for _, sym := range w.cfg.CryptoSymbols {
		for _, iv := range w.cfg.CryptoIntervals {
			n, err := store.CountCryptoBars(ctx, w.pool, sym, iv)
			if err != nil {
				w.log.Error("count crypto bars", "symbol", sym, "interval", iv, "err", err)
				continue
			}
			if n >= w.cfg.BackfillBars {
				w.log.Debug("crypto backfill not needed", "symbol", sym, "interval", iv, "have", n)
				continue
			}
			limit := w.cfg.BackfillBars
			if limit > 1000 {
				limit = 1000
			}
			w.log.Info("backfilling crypto", "symbol", sym, "interval", iv, "have", n, "want", limit)
			bars, err := w.bin.FetchLatestKlines(ctx, sym, iv, limit)
			if err != nil {
				w.log.Error("binance backfill fetch", "symbol", sym, "interval", iv, "err", err)
				continue
			}
			if err := store.UpsertCryptoOHLCV(ctx, w.pool, bars); err != nil {
				w.log.Error("binance backfill upsert", "symbol", sym, "err", err)
			} else {
				w.log.Info("crypto backfill done", "symbol", sym, "interval", iv, "bars", len(bars))
			}
		}
	}
}

// fetchLatest pulls the most recent bars to keep the DB current between polls.
func (w *worker) fetchLatest(ctx context.Context) {
	const latestN = 20

	for _, sym := range w.cfg.EquitySymbols {
		for _, iv := range w.cfg.EquityIntervals {
			bars, err := w.yahoo.FetchBars(ctx, sym, iv, latestN)
			if err != nil {
				w.log.Error("yahoo latest", "symbol", sym, "interval", iv, "err", err)
			}
			if len(bars) == 0 && w.alp.HasCredentials() {
				bars, err = w.alp.FetchLatestBars(ctx, sym, iv, latestN)
				if err != nil {
					w.log.Error("alpaca latest fallback", "symbol", sym, "interval", iv, "err", err)
					continue
				}
			}
			if len(bars) > 0 {
				if err := store.UpsertEquityOHLCV(ctx, w.pool, bars); err != nil {
					w.log.Error("equity latest upsert", "symbol", sym, "err", err)
				} else {
					w.log.Info("equity bars refreshed", "symbol", sym, "interval", iv, "bars", len(bars))
				}
			}
		}
	}

	for _, sym := range w.cfg.CryptoSymbols {
		for _, iv := range w.cfg.CryptoIntervals {
			bars, err := w.bin.FetchLatestKlines(ctx, sym, iv, latestN)
			if err != nil {
				w.log.Error("binance latest", "symbol", sym, "interval", iv, "err", err)
				continue
			}
			if err := store.UpsertCryptoOHLCV(ctx, w.pool, bars); err != nil {
				w.log.Error("binance latest upsert", "symbol", sym, "err", err)
			} else {
				w.log.Info("crypto bars refreshed", "symbol", sym, "interval", iv, "bars", len(bars))
			}
		}
	}
}
