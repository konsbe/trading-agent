package main

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/konsbe/trading-agent/services/data-ingestion/internal/fetch/yahoo"
	"github.com/konsbe/trading-agent/services/data-ingestion/internal/store"
)

// runBackfillRound claims one batch of symbols and backfills their history
// (spec §8.1.3, build-order step 3).
//
// Returns the number of symbols processed. Zero means no claimable work, which
// the caller uses to back off — the backfill is run-once in spirit, but the loop
// keeps running so newly-listed symbols are picked up on the next weekly refresh.
//
// Resumability comes from the claim/lease protocol in the store layer rather
// than from anything held in memory here: a symbol is marked in_progress before
// its fetch and only reaches 'done' after its bars are committed, so a kill at
// any point leaves the row reclaimable and no symbol is recorded as complete
// without its data.
func (w *worker) runBackfillRound(ctx context.Context) int {
	claims, err := store.ClaimBackfillBatch(ctx, w.pool,
		w.cfg.BackfillBatchSize, w.cfg.BackfillClaimLease, w.cfg.BackfillMaxAttempts)
	if err != nil {
		w.log.Error("claim backfill batch", "err", err)
		return 0
	}
	if len(claims) == 0 {
		return 0
	}

	started := time.Now()
	to := time.Now().UTC()
	from := to.AddDate(-w.cfg.BackfillYears, 0, 0)

	var (
		mu                              sync.Mutex
		okCount, noDataCount, failCount int
		barsTotal                       int64
	)

	sem := make(chan struct{}, max(1, w.cfg.BackfillConcurrency))
	var wg sync.WaitGroup

	for _, c := range claims {
		if ctx.Err() != nil {
			break
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(c store.BackfillClaim) {
			defer wg.Done()
			defer func() { <-sem }()

			bars, stored, err := w.fetchAndStore(ctx, c.Symbol, from, to)

			mu.Lock()
			defer mu.Unlock()
			switch {
			case errors.Is(err, yahoo.ErrNoData):
				// Not a failure: a delisting, a bad ticker, or a listing with no
				// history in the window. Recorded done-with-zero-bars so the rate
				// budget is not spent re-asking every round.
				noDataCount++
				if e := store.MarkBackfillDone(ctx, w.pool, c.Symbol, c.Exchange, 0, nil); e != nil {
					w.log.Warn("mark no-data symbol done", "symbol", c.Symbol, "err", e)
				}
			case err != nil:
				failCount++
				if e := store.MarkBackfillFailed(ctx, w.pool, c.Symbol, c.Exchange, err.Error()); e != nil {
					w.log.Warn("mark backfill failed", "symbol", c.Symbol, "err", e)
				}
				w.log.Debug("backfill symbol failed",
					"symbol", c.Symbol, "attempt", c.Attempts+1, "err", err)
			default:
				okCount++
				barsTotal += stored
				var cursor *time.Time
				if len(bars) > 0 {
					// Oldest stored bar, not the window start: a recent IPO's true
					// first bar is more useful than "three years ago".
					oldest := bars[0].TS
					for _, b := range bars {
						if b.TS.Before(oldest) {
							oldest = b.TS
						}
					}
					cursor = &oldest
				}
				if e := store.MarkBackfillDone(ctx, w.pool, c.Symbol, c.Exchange, len(bars), cursor); e != nil {
					w.log.Warn("mark backfill done", "symbol", c.Symbol, "err", e)
				}
			}
		}(c)
	}
	wg.Wait()

	// bar_count / first_bar_ts / last_bar_ts are refreshed after each batch so
	// progress is observable from SQL during a multi-hour run.
	if _, err := store.RefreshUniverseBarCounts(ctx, w.pool, w.cfg.BarInterval, w.cfg.BarSource); err != nil {
		w.log.Warn("refresh bar counts", "err", err)
	}

	prog, err := store.LoadBackfillProgress(ctx, w.pool, w.cfg.BackfillMaxAttempts, w.cfg.MinBarsHistory)
	if err != nil {
		w.log.Warn("backfill progress", "err", err)
	}

	w.log.Info("backfill batch complete",
		"claimed", len(claims),
		"ok", okCount,
		"no_data", noDataCount,
		"failed", failCount,
		"bars_upserted", barsTotal,
		"years", w.cfg.BackfillYears,
		"progress_done", prog.Done,
		"progress_pending", prog.Pending,
		"progress_failed", prog.Failed,
		"progress_in_progress", prog.InProgress,
		"eligible", prog.Eligible,
		"with_enough_bars", prog.WithEnoughBars,
		"took", time.Since(started).Round(time.Second).String())

	if prog.Exhausted > 0 {
		w.log.Warn("symbols exhausted their backfill attempts and will not be retried; inspect universe_symbols.backfill_last_error",
			"exhausted", prog.Exhausted, "max_attempts", w.cfg.BackfillMaxAttempts)
	}
	return len(claims)
}

// fetchAndStore fetches one symbol's window and commits it transactionally.
//
// The bars returned are the fetched set, so the caller can derive the true
// oldest bar for the checkpoint.
func (w *worker) fetchAndStore(ctx context.Context, symbol string, from, to time.Time) ([]store.EquityBar, int64, error) {
	bars, err := w.yh.FetchBarsRange(ctx, symbol, w.cfg.BarInterval, from, to)
	if err != nil {
		return nil, 0, err
	}
	stored, err := store.UpsertEquityOHLCVBatch(ctx, w.pool, bars)
	if err != nil {
		return nil, 0, err
	}
	return bars, stored, nil
}

// runDailyBars is the §8.1.4 incremental refresh: one short-window request per
// eligible symbol, run after the US close.
//
// Folded into build-order step 3 because §10 assigns it no step of its own, and
// without it equity_ohlcv goes stale the day after the backfill finishes. It
// shares the fetch and upsert path with the backfill; only the window differs.
//
// Symbols that have never been backfilled are skipped rather than given a
// 7-day window — a 7-day history would let the feature engine compute nothing
// and would mark the symbol as having bars when it does not meaningfully.
func (w *worker) runDailyBars(ctx context.Context) {
	started := time.Now()

	bounds, err := store.LoadBarBounds(ctx, w.pool, w.cfg.BarInterval, w.cfg.BarSource)
	if err != nil {
		w.log.Error("load bar bounds", "err", err)
		return
	}
	if len(bounds) == 0 {
		w.log.Info("daily bar refresh: no eligible symbols yet")
		return
	}

	to := time.Now().UTC()
	from := to.AddDate(0, 0, -max(1, w.cfg.DailyBarsLookbackDays))

	var (
		mu                                    sync.Mutex
		okCount, noDataCount, failCount, skip int
		barsTotal                             int64
	)
	sem := make(chan struct{}, max(1, w.cfg.BackfillConcurrency))
	var wg sync.WaitGroup

	for sym, b := range bounds {
		if ctx.Err() != nil {
			break
		}
		if b.Count == 0 {
			skip++
			continue
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(sym string) {
			defer wg.Done()
			defer func() { <-sem }()

			_, stored, err := w.fetchAndStore(ctx, sym, from, to)

			mu.Lock()
			defer mu.Unlock()
			switch {
			case errors.Is(err, yahoo.ErrNoData):
				noDataCount++
			case err != nil:
				failCount++
				w.log.Debug("daily bar refresh failed", "symbol", sym, "err", err)
			default:
				okCount++
				barsTotal += stored
			}
		}(sym)
	}
	wg.Wait()

	if _, err := store.RefreshUniverseBarCounts(ctx, w.pool, w.cfg.BarInterval, w.cfg.BarSource); err != nil {
		w.log.Warn("refresh bar counts", "err", err)
	}

	w.log.Info("daily bar refresh complete",
		"symbols", len(bounds),
		"ok", okCount,
		"no_data", noDataCount,
		"failed", failCount,
		"skipped_never_backfilled", skip,
		"bars_upserted", barsTotal,
		"lookback_days", w.cfg.DailyBarsLookbackDays,
		"took", time.Since(started).Round(time.Second).String())

	// A refresh that fails for most symbols means throttling or a changed
	// endpoint, not a per-symbol data gap — worth distinguishing loudly because
	// the scanner would otherwise run on yesterday's bars without complaint.
	if attempted := okCount + noDataCount + failCount; attempted > 0 && failCount*2 > attempted {
		w.log.Warn("more than half of daily bar requests failed; check Yahoo throttling (UNIVERSE_YAHOO_REQUESTS_PER_SEC) before trusting today's scan",
			"failed", failCount, "attempted", attempted)
	}
}
