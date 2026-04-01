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

	"github.com/konsbe/trading-agent/services/data-ingestion/internal/config"
	"github.com/konsbe/trading-agent/services/data-ingestion/internal/db"
	"github.com/konsbe/trading-agent/services/data-ingestion/internal/fetch/finnhub"
	"github.com/konsbe/trading-agent/services/data-ingestion/internal/fetch/lunarcrush"
	"github.com/konsbe/trading-agent/services/data-ingestion/internal/logx"
	"github.com/konsbe/trading-agent/services/data-ingestion/internal/store"
)

func main() {
	_ = godotenv.Load()
	cfg, err := config.LoadSentiment()
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

	lc := lunarcrush.New(cfg.LunarCrushKey)
	fh := finnhub.New(cfg.FinnhubKey)

	tLunar := time.NewTicker(cfg.PollLunarCrush)
	tNews := time.NewTicker(cfg.PollFinnhubNews)
	defer tLunar.Stop()
	defer tNews.Stop()

	runLunarCrush := func() {
		if !lc.HasKey() {
			log.Debug("LUNARCRUSH_API_KEY missing; skipping lunarcrush")
			return
		}
		for _, sym := range cfg.NewsSymbols {
			raw, err := lc.FetchCoin(ctx, strings.ToUpper(sym))
			if err != nil {
				log.Error("lunarcrush", "symbol", sym, "err", err)
				continue
			}
			score := extractGalaxyScore(raw)
			if err := store.UpsertSentiment(ctx, pool, time.Now().UTC(), "lunarcrush", strings.ToUpper(sym), score, raw); err != nil {
				log.Error("upsert lunarcrush sentiment", "symbol", sym, "err", err)
			} else {
				log.Info("lunarcrush snapshot stored", "symbol", sym)
			}
		}
	}

	runFinnhubNews := func() {
		if !fh.HasToken() {
			log.Debug("FINNHUB_API_KEY missing; skipping news")
			return
		}
		news, err := fh.CryptoNews(ctx)
		if err != nil {
			log.Error("finnhub crypto news", "err", err)
			return
		}
		for _, item := range news {
			headline, _ := item["headline"].(string)
			if headline == "" {
				continue
			}
			urlStr, _ := item["url"].(string)
			ts := time.Now().UTC()
			if ds, ok := item["datetime"].(float64); ok {
				ts = time.Unix(int64(ds), 0).UTC()
			}
			if err := store.InsertNews(ctx, pool, ts, "finnhub_crypto", "", headline, urlStr, nil, item); err != nil {
				log.Error("insert news", "err", err)
			}
		}
		log.Info("finnhub crypto news ingested", "n", len(news))
	}

	runLunarCrush()
	runFinnhubNews()
	for {
		select {
		case <-ctx.Done():
			log.Info("shutdown")
			return
		case <-tLunar.C:
			runLunarCrush()
		case <-tNews.C:
			runFinnhubNews()
		}
	}
}

func extractGalaxyScore(m map[string]any) *float64 {
	for _, k := range []string{"galaxy_score", "galaxyScore", "score"} {
		if x, ok := m[k]; ok {
			switch v := x.(type) {
			case float64:
				return ptrf(v)
			case string:
				f, err := strconv.ParseFloat(v, 64)
				if err == nil {
					return ptrf(f)
				}
			}
		}
	}
	return nil
}

func ptrf(f float64) *float64 {
	return &f
}
