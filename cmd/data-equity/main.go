package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"github.com/berdelis/trading-agent/internal/config"
	"github.com/berdelis/trading-agent/internal/db"
	"github.com/berdelis/trading-agent/internal/fetch/alpacadata"
	"github.com/berdelis/trading-agent/internal/fetch/finnhub"
	"github.com/berdelis/trading-agent/internal/fetch/fred"
	"github.com/berdelis/trading-agent/internal/logx"
	"github.com/berdelis/trading-agent/internal/store"
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

	t := time.NewTicker(cfg.PollInterval)
	defer t.Stop()

	run := func() {
		if alp.HasCredentials() {
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
		} else {
			log.Debug("alpaca market data keys missing; skipping bars")
		}

		if fh.HasToken() {
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
					}
				}
			}
		} else {
			log.Debug("finnhub token missing; skipping quotes")
		}

		if cfg.FredAPIKey != "" {
			for _, sid := range cfg.FredSeries {
				obs, err := fr.FetchSeries(ctx, sid)
				if err != nil {
					log.Error("fred series", "id", sid, "err", err)
					continue
				}
				for _, o := range obs {
					if !o.Valid {
						continue
					}
					d, err := time.Parse("2006-01-02", o.Date)
					if err != nil {
						continue
					}
					ts := time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.UTC)
					if err := store.UpsertMacroFred(ctx, pool, sid, ts, o.Value); err != nil {
						log.Error("upsert fred", "series", sid, "err", err)
					}
				}
				log.Info("fred series refreshed", "id", sid, "points", len(obs))
			}
		} else {
			log.Debug("FRED_API_KEY missing; skipping macro")
		}
	}

	run()
	for {
		select {
		case <-ctx.Done():
			log.Info("shutdown")
			return
		case <-t.C:
			run()
		}
	}
}
