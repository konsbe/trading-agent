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
	"github.com/berdelis/trading-agent/internal/fetch/binance"
	"github.com/berdelis/trading-agent/internal/fetch/coingecko"
	"github.com/berdelis/trading-agent/internal/logx"
	"github.com/berdelis/trading-agent/internal/store"
)

func main() {
	_ = godotenv.Load()
	cfg, err := config.LoadCrypto()
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

	rest := binance.NewREST()
	cg := coingecko.New()

	if os.Getenv("BINANCE_ENABLE_WS") == "true" {
		ch := make(chan store.CryptoBar, 256)
		go func() {
			for {
				err := binance.StreamKlines(ctx, cfg.BinanceSymbols, cfg.BinanceInterval, ch)
				if ctx.Err() != nil {
					return
				}
				log.Warn("binance ws disconnected, reconnecting", "err", err)
				time.Sleep(3 * time.Second)
			}
		}()
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case bar := <-ch:
					if err := store.UpsertCryptoOHLCV(ctx, pool, []store.CryptoBar{bar}); err != nil {
						log.Error("upsert ws bar", "err", err)
					}
				}
			}
		}()
	}

	t := time.NewTicker(cfg.PollInterval)
	defer t.Stop()

	runPoll := func() {
		for _, sym := range cfg.BinanceSymbols {
			bars, err := rest.FetchLatestKlines(ctx, sym, cfg.BinanceInterval, 200)
			if err != nil {
				log.Error("binance rest", "symbol", sym, "err", err)
				continue
			}
			if err := store.UpsertCryptoOHLCV(ctx, pool, bars); err != nil {
				log.Error("upsert crypto ohlcv", "symbol", sym, "err", err)
			} else {
				log.Info("crypto ohlcv upserted", "symbol", sym, "bars", len(bars))
			}
		}
		if cfg.CoinGeckoGlobal {
			g, err := cg.FetchGlobal(ctx)
			if err != nil {
				log.Error("coingecko global", "err", err)
			} else if err := store.InsertCryptoGlobal(ctx, pool, time.Now().UTC(), "coingecko", g); err != nil {
				log.Error("insert coingecko global", "err", err)
			} else {
				log.Info("coingecko global stored")
			}
		}
	}

	runPoll()
	for {
		select {
		case <-ctx.Done():
			log.Info("shutdown")
			return
		case <-t.C:
			runPoll()
		}
	}
}
