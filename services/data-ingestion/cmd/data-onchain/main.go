package main

import (
	"context"
	"log/slog"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"github.com/konsbe/trading-agent/services/data-ingestion/internal/config"
	"github.com/konsbe/trading-agent/services/data-ingestion/internal/db"
	"github.com/konsbe/trading-agent/services/data-ingestion/internal/fetch/etherscan"
	"github.com/konsbe/trading-agent/services/data-ingestion/internal/fetch/glassnode"
	"github.com/konsbe/trading-agent/services/data-ingestion/internal/logx"
	"github.com/konsbe/trading-agent/services/data-ingestion/internal/store"
)

func main() {
	_ = godotenv.Load()
	cfg, err := config.LoadOnchain()
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

	gn := glassnode.New(cfg.GlassnodeKey)
	es := etherscan.New(cfg.EtherscanKey)

	tGlass := time.NewTicker(cfg.PollGlassnode)
	tEther := time.NewTicker(cfg.PollEtherscan)
	defer tGlass.Stop()
	defer tEther.Stop()

	specs := []struct {
		asset  string
		metric string
		path   string
		q      url.Values
	}{
		{"BTC", "addresses_active_count", "addresses/active_count", url.Values{"a": {"BTC"}, "i": {"24h"}}},
		{"ETH", "addresses_active_count", "addresses/active_count", url.Values{"a": {"ETH"}, "i": {"24h"}}},
	}

	runGlassnode := func() {
		if !gn.HasKey() {
			log.Debug("GLASSNODE_API_KEY missing; skipping glassnode")
			return
		}
		for _, spec := range specs {
			rows, err := gn.FetchMetric(ctx, spec.path, spec.q)
			if err != nil {
				log.Error("glassnode", "metric", spec.metric, "err", err)
				continue
			}
			if len(rows) == 0 {
				continue
			}
			last := rows[len(rows)-1]
			ts, v, ok := glassnodePoint(last)
			if !ok {
				continue
			}
			if err := store.UpsertOnchain(ctx, pool, ts, spec.asset, spec.metric, &v, last, "glassnode"); err != nil {
				log.Error("upsert onchain", "asset", spec.asset, "err", err)
			} else {
				log.Info("onchain metric stored", "asset", spec.asset, "metric", spec.metric)
			}
		}
	}

	runEtherscan := func() {
		if !es.HasKey() {
			log.Debug("ETHERSCAN_API_KEY missing; skipping etherscan")
			return
		}
		supply, err := es.EthSupply(ctx)
		if err != nil {
			log.Error("etherscan eth supply", "err", err)
			return
		}
		v := supply
		payload := map[string]any{"eth_supply": supply}
		if err := store.UpsertOnchain(ctx, pool, time.Now().UTC(), "ETH", "eth_supply_etherscan", &v, payload, "etherscan"); err != nil {
			log.Error("upsert etherscan supply", "err", err)
		} else {
			log.Info("etherscan eth supply stored")
		}
	}

	runGlassnode()
	runEtherscan()
	for {
		select {
		case <-ctx.Done():
			log.Info("shutdown")
			return
		case <-tGlass.C:
			runGlassnode()
		case <-tEther.C:
			runEtherscan()
		}
	}
}

func glassnodePoint(row map[string]any) (time.Time, float64, bool) {
	var tu int64
	switch t := row["t"].(type) {
	case float64:
		tu = int64(t)
	default:
		return time.Time{}, 0, false
	}
	var v float64
	switch x := row["v"].(type) {
	case float64:
		v = x
	case string:
		f, err := strconv.ParseFloat(x, 64)
		if err != nil {
			return time.Time{}, 0, false
		}
		v = f
	default:
		return time.Time{}, 0, false
	}
	return time.Unix(tu, 0).UTC(), v, true
}
