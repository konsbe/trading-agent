package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"github.com/konsbe/trading-agent/services/data-ingestion/internal/config"
	"github.com/konsbe/trading-agent/services/data-ingestion/internal/db"
	"github.com/konsbe/trading-agent/services/data-ingestion/internal/fetch/alpacadata"
	"github.com/konsbe/trading-agent/services/data-ingestion/internal/fetch/finnhub"
	"github.com/konsbe/trading-agent/services/data-ingestion/internal/fetch/fred"
	"github.com/konsbe/trading-agent/services/data-ingestion/internal/logx"
	"github.com/konsbe/trading-agent/services/data-ingestion/internal/store"
)

func main() {
	_ = godotenv.Load()
	cfg, err := config.LoadEquity()
	if err != nil {
		slog.Error("config", "err", err)
		os.Exit(1)
	}
	log := logx.New(cfg.LogLevel)
	log.Info("alpaca trading API base (for future execution service)", "url", cfg.AlpacaBaseURL)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("db", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	alp := alpacadata.New(cfg.AlpacaKey, cfg.AlpacaSecret)
	fh := finnhub.New(cfg.FinnhubKey)
	fr := fred.New(cfg.FredAPIKey)

	tAlpaca := time.NewTicker(cfg.PollAlpaca)
	tFinnhub := time.NewTicker(cfg.PollFinnhub)
	tFred := time.NewTicker(cfg.PollFred)
	defer tAlpaca.Stop()
	defer tFinnhub.Stop()
	defer tFred.Stop()

	runAlpaca := func() {
		if !alp.HasCredentials() {
			log.Debug("alpaca market data keys missing; skipping bars")
			return
		}
		for _, sym := range cfg.Symbols {
			bars, err := alp.FetchLatestBars(ctx, sym, "1Hour", 200)
			if err != nil {
				log.Error("alpaca bars", "symbol", sym, "err", err)
				continue
			}
			if err := store.UpsertEquityOHLCV(ctx, pool, bars); err != nil {
				log.Error("upsert equity bars", "symbol", sym, "err", err)
			} else {
				log.Info("equity bars upserted", "symbol", sym, "n", len(bars))
			}
		}
	}

	runFinnhub := func() {
		if !fh.HasToken() {
			log.Debug("finnhub token missing; skipping quotes")
			return
		}
		for _, sym := range cfg.Symbols {
			q, err := fh.Quote(ctx, sym)
			if err != nil {
				log.Error("finnhub quote", "symbol", sym, "err", err)
				continue
			}
			bar, ok := finnhub.StoreQuoteAsEquityBar(sym, q)
			if ok {
				if err := store.UpsertEquityOHLCV(ctx, pool, []store.EquityBar{bar}); err != nil {
					log.Error("upsert finnhub quote bar", "symbol", sym, "err", err)
				} else {
					log.Info("finnhub quote upserted", "symbol", sym)
				}
			}
		}
	}

	runFred := func() {
		if cfg.FredAPIKey == "" {
			log.Debug("FRED_API_KEY missing; skipping macro")
			return
		}
		for _, sid := range cfg.FredSeries {
			obs, err := fr.FetchSeries(ctx, sid)
			if err != nil {
				log.Error("fred series", "id", sid, "err", err)
				continue
			}
			rows := make([]store.MacroFredRow, 0, len(obs))
			for _, o := range obs {
				if !o.Valid {
					continue
				}
				d, err := time.Parse("2006-01-02", o.Date)
				if err != nil {
					continue
				}
				rows = append(rows, store.MacroFredRow{
					TS:    time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.UTC),
					Value: o.Value,
				})
			}
			if err := store.UpsertMacroFredBatch(ctx, pool, sid, rows); err != nil {
				log.Error("upsert fred batch", "series", sid, "err", err)
			} else {
				log.Info("fred series refreshed", "id", sid, "points", len(rows))
			}
		}
	}

	runAlpaca()
	runFinnhub()
	runFred()
	for {
		select {
		case <-ctx.Done():
			log.Info("shutdown")
			return
		case <-tAlpaca.C:
			runAlpaca()
		case <-tFinnhub.C:
			runFinnhub()
		case <-tFred.C:
			runFred()
		}
	}
}
