// Command intraday-bars stores 5-minute regular-session bars for the symbols
// someone is actually looking at: the latest scan's candidates plus every
// watchlist symbol. They feed the 1D and 5D ranges of the web app's detail
// chart (momentum-api GET /api/v1/scanner/symbols/{symbol}/bars).
//
// Source: Yahoo Finance's chart endpoint — free, no key, consolidated volume.
// It depends on Yahoo's goodwill (see data_ingestion.md), so the job is scoped
// to a few dozen symbols rather than the universe, and a failed symbol is
// logged and skipped rather than retried aggressively.
//
// Bars are written to equity_ohlcv with interval '5Min', source
// 'yahoo_finance'. Every daily reader filters on interval = '1Day', so these
// rows never reach the scanner.
//
//	DATABASE_URL=... go run ./cmd/intraday-bars          # loop
//	DATABASE_URL=... go run ./cmd/intraday-bars -once    # single pass
package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"github.com/konsbe/trading-agent/services/data-ingestion/internal/db"
	"github.com/konsbe/trading-agent/services/data-ingestion/internal/fetch/yahoo"
	"github.com/konsbe/trading-agent/services/data-ingestion/internal/logx"
	"github.com/konsbe/trading-agent/services/data-ingestion/internal/store"
)

const barInterval = "5Min"

type config struct {
	pollInterval time.Duration
	lookbackDays int
	maxSymbols   int
}

func main() {
	once := flag.Bool("once", false, "run a single pass and exit")
	flag.Parse()

	_ = godotenv.Load()
	_ = godotenv.Load("../../.env")
	log := logx.New(env("LOG_LEVEL", "info"))

	dsn := env("DATABASE_URL", "")
	if dsn == "" {
		log.Error("intraday-bars: DATABASE_URL is required")
		os.Exit(1)
	}
	cfg := config{
		pollInterval: durationEnv("INTRADAY_BARS_POLL_INTERVAL", 30*time.Minute),
		lookbackDays: intEnv("INTRADAY_BARS_LOOKBACK_DAYS", 8),
		maxSymbols:   intEnv("INTRADAY_BARS_MAX_SYMBOLS", 200),
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := db.Connect(ctx, dsn)
	if err != nil {
		log.Error("intraday-bars: database", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	client := yahoo.New()
	for {
		runPass(ctx, log, pool, client, cfg, time.Now())
		if *once {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(cfg.pollInterval):
		}
	}
}

func runPass(ctx context.Context, log *slog.Logger, pool *pgxpool.Pool, client *yahoo.Client, cfg config, now time.Time) {
	candidates, err := querySymbols(ctx, pool, `
SELECT symbol FROM momentum_features
WHERE gates_passed AND ts = (SELECT max(ts) FROM momentum_features)`)
	if err != nil {
		log.Error("intraday-bars: candidates", "err", err)
		return
	}
	watchlist, err := querySymbols(ctx, pool, `SELECT DISTINCT symbol FROM watchlist_items`)
	if err != nil {
		log.Error("intraday-bars: watchlist", "err", err)
		return
	}
	symbols, dropped := selectSymbols(candidates, watchlist, cfg.maxSymbols)
	if dropped > 0 {
		log.Warn("intraday-bars: symbol cap reached; some symbols skipped this pass",
			"cap", cfg.maxSymbols, "skipped", dropped)
	}

	cutoff := now.AddDate(0, 0, -cfg.lookbackDays)
	var written, failed int
	for _, sym := range symbols {
		if ctx.Err() != nil {
			return
		}
		bars, err := client.FetchBars(ctx, sym, barInterval, 0)
		if err != nil {
			failed++
			log.Warn("intraday-bars: fetch", "symbol", sym, "err", err)
			continue
		}
		bars = trimToLookback(bars, cutoff)
		n, err := store.UpsertEquityOHLCVBatch(ctx, pool, bars)
		if err != nil {
			failed++
			log.Warn("intraday-bars: upsert", "symbol", sym, "err", err)
			continue
		}
		written += int(n)
	}
	log.Info("intraday-bars: pass complete", "symbols", len(symbols), "candidates", len(candidates),
		"watchlist", len(watchlist), "bars_written", written, "failed", failed)
}

// selectSymbols merges candidates and watchlist symbols, de-duplicated and
// sorted, candidates first so a cap drops watchlist extras before the day's
// candidates. It reports how many were dropped by the cap.
func selectSymbols(candidates, watchlist []string, max int) ([]string, int) {
	seen := map[string]bool{}
	var out []string
	for _, group := range [][]string{candidates, watchlist} {
		sorted := make([]string, 0, len(group))
		for _, s := range group {
			sorted = append(sorted, strings.ToUpper(strings.TrimSpace(s)))
		}
		sort.Strings(sorted)
		for _, s := range sorted {
			if s == "" || seen[s] {
				continue
			}
			seen[s] = true
			out = append(out, s)
		}
	}
	if max > 0 && len(out) > max {
		return out[:max], len(out) - max
	}
	return out, 0
}

// trimToLookback keeps bars at or after cutoff. Yahoo returns up to 60 days of
// 5-minute bars; the chart only needs the last few sessions.
func trimToLookback(bars []store.EquityBar, cutoff time.Time) []store.EquityBar {
	out := bars[:0:0]
	for _, b := range bars {
		if !b.TS.Before(cutoff) {
			out = append(out, b)
		}
	}
	return out
}

func querySymbols(ctx context.Context, pool *pgxpool.Pool, sql string) ([]string, error) {
	rows, err := pool.Query(ctx, sql)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func env(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func durationEnv(key string, def time.Duration) time.Duration {
	if d, err := time.ParseDuration(env(key, "")); err == nil && d > 0 {
		return d
	}
	return def
}

func intEnv(key string, def int) int {
	if v, err := strconv.Atoi(env(key, "")); err == nil && v > 0 {
		return v
	}
	return def
}
