package main

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	appconfig "github.com/konsbe/trading-agent/services/data-analyzer/internal/config"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/fundamental"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/momentumapi"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/store"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/technical/runner"
)

// computer runs the full analysis for the computation set (migration 030):
// watchlist ∪ today's candidates ∪ followed symbols ∪ manual Compute requests.
// It calls exactly what Stock Detail's on-demand path calls
// (runner.ComputeAndStore, fundamental.AnalyzeSymbol), so there is one
// computation path. It never fetches: the ingestion workers fetch the same set
// on their own cadence (and manual requests on a short poll).
type computer struct {
	pool *pgxpool.Pool
	ta   appconfig.TechnicalAnalysis
	fa   appconfig.FundamentalAnalysis
	log  *slog.Logger
}

// computeOne: technicals for every asset type (crypto on its Binance
// intervals), fundamentals for equities and funds. The outcome is recorded in
// symbol_data_status either way.
func (c *computer) computeOne(ctx context.Context, s store.InterestSymbol) error {
	var errs []error
	if s.AssetType == "crypto" {
		for _, iv := range c.ta.CryptoIntervals {
			if _, err := runner.ComputeAndStore(ctx, c.pool, s.Symbol, "binance", iv, c.ta, c.log); err != nil {
				errs = append(errs, err)
			}
		}
	} else {
		if _, err := runner.ComputeAndStore(ctx, c.pool, s.Symbol, "equity", momentumapi.AnalysisInterval, c.ta, c.log); err != nil {
			errs = append(errs, err)
		}
		if _, err := fundamental.AnalyzeSymbol(ctx, c.pool, s.Symbol, c.fa, c.log); err != nil {
			errs = append(errs, err)
		}
	}
	err := errors.Join(errs...)
	if merr := store.MarkComputed(ctx, c.pool, s.Symbol, err); merr != nil {
		c.log.Warn("momentum-daily: mark computed", "symbol", s.Symbol, "err", merr)
	}
	return err
}

// dailyPass runs once per session after the chain completes: reconcile the
// candidate / watchlist / followed reasons, then compute every symbol with an
// open reason. A failure is logged and retried at the next poll; it never
// touches the chain's own completion.
func (c *computer) dailyPass(ctx context.Context) {
	session, ok, err := store.PendingComputationPass(ctx, c.pool)
	if err != nil {
		c.log.Error("momentum-daily: computation pass", "err", err)
		return
	}
	if !ok {
		return
	}
	opened, closed, err := store.ReconcileInterest(ctx, c.pool)
	if err != nil {
		c.log.Error("momentum-daily: reconcile computation interest", "err", err)
		return
	}
	set, err := store.ComputationSet(ctx, c.pool)
	if err != nil {
		c.log.Error("momentum-daily: computation set", "err", err)
		return
	}
	started, failed := time.Now(), 0
	for _, s := range set {
		if ctx.Err() != nil {
			return
		}
		if err := c.computeOne(ctx, s); err != nil {
			failed++
			c.log.Warn("momentum-daily: compute", "symbol", s.Symbol, "asset_type", s.AssetType, "err", err)
		}
	}
	if err := store.MarkComputationPass(ctx, c.pool, session); err != nil {
		c.log.Error("momentum-daily: mark computation pass", "err", err)
		return
	}
	c.log.Info("momentum-daily: computation pass complete", "session", session.Format(time.DateOnly),
		"reasons_opened", opened, "reasons_closed", closed, "symbols", len(set), "failed", failed,
		"took", time.Since(started).Round(time.Second).String())
}

// manualLoop computes Compute requests as soon as their data has landed.
func (c *computer) manualLoop(ctx context.Context, every time.Duration) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			pending, err := store.PendingManualCompute(ctx, c.pool)
			if err != nil {
				c.log.Warn("momentum-daily: manual compute queue", "err", err)
				continue
			}
			for _, s := range pending {
				err := c.computeOne(ctx, s)
				c.log.Info("momentum-daily: manual compute", "symbol", s.Symbol, "asset_type", s.AssetType, "err", err)
			}
		}
	}
}
