// data-technical fetches daily and weekly OHLCV bars that the other ingestion
// workers do not cover:
//
//   - data-equity fetches intraday (hourly) equity bars via Alpaca.
//   - data-crypto fetches a single configurable interval via Binance.
//   - data-technical fills the gap with daily/weekly bars for both asset classes.
//
// Sources:
//
//	Equity — Yahoo Finance (primary, free), Alpaca Data (fallback).
//	Crypto — Binance REST.
//
// All bars are written to equity_ohlcv / crypto_ohlcv.
// Indicator computation has been moved to services/data-analyzer/cmd/technical-analysis.
package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/konsbe/trading-agent/services/data-ingestion/internal/config"
	"github.com/konsbe/trading-agent/services/data-ingestion/internal/db"
	"github.com/konsbe/trading-agent/services/data-ingestion/internal/fetch/alpacadata"
	"github.com/konsbe/trading-agent/services/data-ingestion/internal/fetch/binance"
	"github.com/konsbe/trading-agent/services/data-ingestion/internal/fetch/yahoo"
	"github.com/konsbe/trading-agent/services/data-ingestion/internal/logx"
	"github.com/konsbe/trading-agent/services/data-ingestion/internal/store"
	"github.com/konsbe/trading-agent/services/data-ingestion/internal/symbols"
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

	// Manual "Compute" requests poll on their own goroutine so their bars land
	// in minutes even while a backfill / latest-bars pass is running.
	go func() {
		manual := time.NewTicker(cfg.ManualQueuePoll)
		defer manual.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-manual.C:
				w.runManualQueue(ctx)
			}
		}
	}()

	log.Info("backfilling historical bars")
	eq, cr := w.equitySymbols(ctx), w.cryptoSymbols(ctx)
	w.backfill(ctx, eq, cr)

	ticker := time.NewTicker(cfg.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Info("shutdown")
			return
		case <-ticker.C:
			// Re-resolved every pass: a symbol that joined the sets since the last
			// pass is backfilled (backfill skips symbols that already have enough
			// bars), then every symbol gets its latest bars.
			eq, cr := w.equitySymbols(ctx), w.cryptoSymbols(ctx)
			w.backfill(ctx, eq, cr)
			w.fetchLatest(ctx, eq, cr)
		}
	}
}

// equitySymbols: followed equities and funds ∪ every one with an open
// computation reason (watchlist, today's candidates, manual Compute).
func (w *worker) equitySymbols(ctx context.Context) []string {
	return symbols.Computation(ctx, w.pool, w.log, "data-technical", "equity bars (TECHNICAL_EQUITY_SYMBOLS)",
		[]string{"equity", "etf"}, w.cfg.EquitySymbols)
}

func (w *worker) cryptoSymbols(ctx context.Context) []string {
	return symbols.Computation(ctx, w.pool, w.log, "data-technical", "crypto bars (TECHNICAL_CRYPTO_SYMBOLS)",
		[]string{"crypto"}, w.cfg.CryptoSymbols)
}

// runManualQueue backfills bars for Compute requests not fetched since they
// were made, and records the fetch so the request can move on to computing.
func (w *worker) runManualQueue(ctx context.Context) {
	for _, kind := range []struct {
		types  []string
		crypto bool
	}{{[]string{"equity", "etf"}, false}, {[]string{"crypto"}, true}} {
		pending, err := store.PendingManual(ctx, w.pool, "bars", kind.types)
		if err != nil {
			w.log.Warn("manual compute queue", "err", err)
			return
		}
		for _, sym := range pending {
			var ferr error
			if kind.crypto {
				ferr = w.backfillCrypto(ctx, sym)
			} else {
				ferr = w.backfillEquity(ctx, sym)
			}
			if err := store.MarkFetched(ctx, w.pool, sym, "bars", ferr); err != nil {
				w.log.Warn("mark bars fetched", "symbol", sym, "err", err)
			}
			w.log.Info("manual compute: bars fetched", "symbol", sym, "err", ferr)
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
func (w *worker) backfill(ctx context.Context, equities, cryptos []string) {
	for _, sym := range equities {
		_ = w.backfillEquity(ctx, sym)
	}
	for _, sym := range cryptos {
		_ = w.backfillCrypto(ctx, sym)
	}
}

// backfillEquity tops one symbol up to BackfillBars per interval; the error is
// the last fetch/store failure, nil when it has bars (or already had enough).
func (w *worker) backfillEquity(ctx context.Context, sym string) error {
	var lastErr error
	{
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
				lastErr = errNoBars
				continue
			}
			if err := store.UpsertEquityOHLCV(ctx, w.pool, bars); err != nil {
				w.log.Error("equity backfill upsert", "symbol", sym, "err", err)
				lastErr = err
			} else {
				w.log.Info("equity backfill done", "symbol", sym, "interval", iv, "bars", len(bars))
			}
		}
	}
	return lastErr
}

// backfillCrypto is backfillEquity for a Binance pair.
func (w *worker) backfillCrypto(ctx context.Context, sym string) error {
	var lastErr error
	{
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
				lastErr = err
				continue
			}
			if err := store.UpsertCryptoOHLCV(ctx, w.pool, bars); err != nil {
				w.log.Error("binance backfill upsert", "symbol", sym, "err", err)
				lastErr = err
			} else {
				w.log.Info("crypto backfill done", "symbol", sym, "interval", iv, "bars", len(bars))
			}
		}
	}
	return lastErr
}

var errNoBars = errors.New("no bars from any source")

// fetchLatest pulls the most recent bars to keep the DB current between polls.
func (w *worker) fetchLatest(ctx context.Context, equities, cryptos []string) {
	const latestN = 20

	for _, sym := range equities {
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

	for _, sym := range cryptos {
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
