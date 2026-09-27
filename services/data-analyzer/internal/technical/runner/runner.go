// Package runner computes and stores every technical indicator for one
// (symbol, exchange, interval). It is the single per-symbol code path: the
// technical-analysis worker loops it over its configured symbols, and
// momentum-api runs it on demand for a symbol outside that list.
//
// It lives apart from internal/technical because it needs internal/store,
// which imports internal/heuristics, which imports internal/technical.
package runner

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/konsbe/trading-agent/services/data-analyzer/internal/compute"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/config"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/store"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/technical"
)

// Result says what ComputeAndStore did.
type Result struct {
	// Bars is how many bars the compute window loaded.
	Bars int
	// Computed is false when fewer than 2 bars are stored: nothing is written,
	// which is a valid outcome, not an error.
	Computed bool
	// TS is the last bar's timestamp, the ts every row is written under.
	TS time.Time
}

// ComputeAndStore loads the compute window (TECHNICAL_COMPUTE_LOOKBACK bars),
// runs every enabled indicator and upserts one technical_indicators row per
// indicator at the last bar's ts, so re-running on identical bars is
// idempotent. exchange is "equity" or "binance". A failed write does not stop
// the remaining indicators (the worker has always logged and continued); it is
// reported in the returned error.
func ComputeAndStore(ctx context.Context, db store.ReadWriter, symbol, exchange, interval string, cfg config.TechnicalAnalysis, log *slog.Logger) (Result, error) {
	var (
		bars []compute.Bar
		err  error
	)
	switch exchange {
	case "equity":
		bars, err = equityBars(ctx, db, symbol, interval, cfg.ComputeLookback)
	case "binance":
		bars, err = cryptoBars(ctx, db, symbol, interval, cfg.ComputeLookback)
	default:
		return Result{}, fmt.Errorf("unknown exchange %q", exchange)
	}
	if err != nil {
		return Result{}, fmt.Errorf("query %s bars %s %s: %w", exchange, symbol, interval, err)
	}
	res := Result{Bars: len(bars)}
	if len(bars) < 2 {
		return res, nil
	}
	cdb := &countingDB{ReadWriter: db}
	w := &run{cfg: cfg, pool: cdb, log: log}
	ts := bars[len(bars)-1].TS

	technical.Emitter{Cfg: cfg}.Emit(bars, func(indicator string, value *float64, payload any) {
		if err := store.UpsertIndicator(ctx, cdb, ts, symbol, exchange, interval, indicator, value, payload); err != nil {
			log.Error("upsert indicator", "indicator", indicator, "symbol", symbol, "err", err)
		}
	})

	w.computeAlertOnsets(ctx, ts, symbol, exchange, interval, bars)
	w.computeRSBenchmark(ctx, ts, symbol, exchange, interval, bars)
	w.computeMTFConfluence(ctx, ts, symbol, exchange, interval, bars)
	w.computeVIXRegime(ctx, ts, symbol, exchange, interval)
	w.computeMultiTFPivots(ctx, ts, symbol, exchange, interval)

	log.Info("indicators computed",
		"symbol", symbol,
		"exchange", exchange,
		"interval", interval,
		"ts", ts.Format("2006-01-02"),
	)
	res.Computed, res.TS = true, ts
	if cdb.failed > 0 {
		return res, fmt.Errorf("technical analysis %s: %d of %d writes failed, first: %w", symbol, cdb.failed, cdb.writes, cdb.first)
	}
	return res, nil
}

// Bar timestamps are normalised to UTC because payloads carry them as JSON
// strings (pivots' reference_ts): pgx returns them in the process's local zone,
// which is UTC in the worker's container but not on a developer machine.
func equityBars(ctx context.Context, db store.Querier, symbol, interval string, limit int) ([]compute.Bar, error) {
	bars, err := store.QueryEquityBars(ctx, db, symbol, interval, limit)
	return inUTC(bars), err
}

func cryptoBars(ctx context.Context, db store.Querier, symbol, interval string, limit int) ([]compute.Bar, error) {
	bars, err := store.QueryCryptoBars(ctx, db, symbol, interval, limit)
	return inUTC(bars), err
}

func inUTC(bars []compute.Bar) []compute.Bar {
	for i := range bars {
		bars[i].TS = bars[i].TS.UTC()
	}
	return bars
}

// run carries what the multi-series indicators (RS benchmark, MTF confluence,
// VIX regime, weekly/monthly pivots) need: they read more bars or macro rows.
type run struct {
	cfg  config.TechnicalAnalysis
	pool store.ReadWriter
	log  *slog.Logger
}

// countingDB records write failures, which the indicator blocks log and skip.
type countingDB struct {
	store.ReadWriter
	writes, failed int
	first          error
}

func (c *countingDB) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	tag, err := c.ReadWriter.Exec(ctx, sql, args...)
	c.writes++
	if err != nil {
		c.failed++
		if c.first == nil {
			c.first = err
		}
	}
	return tag, err
}
