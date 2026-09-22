package main

import (
	"context"
	"errors"
	"math"
	"sync"
	"time"

	"github.com/konsbe/trading-agent/services/data-ingestion/internal/fetch/barsource"
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
		w.cfg.BackfillBatchSize, w.cfg.BackfillClaimLease, w.cfg.BackfillMaxAttempts,
		w.cfg.SubsetEnable)
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
			case errors.Is(err, barsource.ErrNoBars):
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
	bars, err := w.bars.FetchBarsRange(ctx, symbol, w.cfg.BarInterval, from, to)
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
	if w.cfg.SubsetEnable {
		// Refresh only the pilot subset. Refreshing the full universe on the
		// pilot provider would spend a quota sized for a few hundred symbols on
		// several thousand.
		sel, err := store.SelectedSubset(ctx, w.pool)
		if err != nil {
			w.log.Error("load selected subset", "err", err)
			return
		}
		keep := make(map[string]struct{}, len(sel))
		for _, m := range sel {
			keep[m.Symbol] = struct{}{}
		}
		for sym := range bounds {
			if _, ok := keep[sym]; !ok {
				delete(bounds, sym)
			}
		}
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
		repairedCount                         int
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

			bars, stored, err := w.fetchAndStore(ctx, sym, from, to)

			// CORPORATE-ACTION RE-FETCH.
			//
			// Adjusted prices are rewritten BACKWARDS on every split and every
			// dividend. This refresh fetches only a short recent window, so the
			// moment a symbol has a corporate action after its backfill, the
			// stored history is on the OLD adjustment basis and the bars just
			// written are on the NEW one. Windowed features then read a step
			// that no market participant experienced — the same defect that
			// disqualified Twelve Data, except self-inflicted and invisible,
			// because each individual refresh looks correct in isolation.
			//
			// So the action is the trigger: re-fetch the symbol's FULL history
			// onto one basis. Checked BOTH on splitFactor and divCash, because
			// either alone leaves the other's seams behind.
			var repaired bool
			if err == nil && hasCorporateAction(bars) {
				if n, rerr := w.refetchFullHistory(ctx, sym); rerr != nil {
					w.log.Error("corporate action detected but full re-fetch FAILED; "+
						"this symbol's history is now on two adjustment bases and its "+
						"windowed features are wrong until repaired",
						"symbol", sym, "err", rerr)
				} else {
					repaired = true
					stored += n
					w.log.Info("corporate action: re-fetched full history onto one adjustment basis",
						"symbol", sym, "bars", n)
				}
			}

			mu.Lock()
			defer mu.Unlock()
			switch {
			case errors.Is(err, barsource.ErrNoBars):
				noDataCount++
			case err != nil:
				failCount++
				w.log.Debug("daily bar refresh failed", "symbol", sym, "err", err)
			default:
				okCount++
				barsTotal += stored
				if repaired {
					repairedCount++
				}
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
		"corporate_action_refetches", repairedCount,
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

// reportSubsetConsistency re-reads the subset stats once real bars exist.
//
// Run after the backfill rather than after selection because the bucket-drift
// comparison needs both sides: at selection time only the Finnhub quote
// snapshot exists, so the check would trivially report zero and prove nothing.
func (w *worker) reportSubsetConsistency(ctx context.Context) {
	st, err := store.LoadSubsetStats(ctx, w.pool,
		w.cfg.PriceInterval, w.cfg.PriceSource, w.cfg.BarInterval, w.cfg.BarSource, w.cfg.SubsetPennyMaxPrice)
	if err != nil {
		w.log.Warn("subset consistency", "err", err)
		return
	}

	w.log.Info("subset backfill coverage",
		"selected", st.Selected,
		"with_bars", st.WithBars,
		"missing_bars", st.Selected-st.WithBars,
		"bucket_checked", st.BucketChecked,
		"bucket_drifted", st.BucketDrifted)

	if st.BucketDrifted > 0 {
		// Expected in small numbers and accepted — the selection price is a live
		// quote, the bar price is an adjusted close. Warned rather than ignored
		// so a drifted symbol is known to be scored against the other bucket's
		// thresholds instead of looking like a merely strange candidate.
		w.log.Warn("symbols changed price bucket between selection and backfill; they will be scored against the bucket their adjusted close implies",
			"drifted", st.BucketDrifted,
			"of_checked", st.BucketChecked,
			"penny_boundary", w.cfg.SubsetPennyMaxPrice,
			"examples", st.BucketDriftExamples)
	}
}

// hasCorporateAction reports whether any bar in the window carries a split or a
// dividend, either of which rewrites the adjusted series backwards.
//
// A nil field means the provider did not report it, which is NOT the same as
// "no action" — but it also cannot be used as evidence of one, so it is
// ignored here and surfaces instead as the NULL columns the seam audit reads.
func hasCorporateAction(bars []store.EquityBar) bool {
	for _, b := range bars {
		if b.SplitFactor != nil && math.Abs(*b.SplitFactor-1.0) > 1e-9 {
			return true
		}
		if b.DivCash != nil && *b.DivCash > 0 {
			return true
		}
	}
	return false
}

// refetchFullHistory re-fetches a symbol's whole STORED range and upserts it,
// putting every bar on the current adjustment basis.
//
// The start date comes from the earliest bar we already hold, NOT from
// now-BackfillYears. That distinction is not pedantic — getting it wrong
// created a real seam:
//
//	backfill run on 2026-09-21 stored bars from 2016-09-21
//	DTE went ex-dividend on 2026-09-21, rewriting its adjusted history
//	re-fetch run on 2026-09-22 used now-10y = 2016-09-22 as its start
//	-> the 2016-09-21 bar fell OUTSIDE the new window and kept the
//	   pre-dividend basis, while every later bar moved to the new one
//
// A rolling window advances by a day on every run, so it orphans the oldest
// bar of the previous run every time. Anchoring to the stored minimum makes
// the repair cover exactly what needs repairing, which is everything we hold.
//
// A one-week buffer is subtracted so a boundary bar cannot be missed to
// timezone or half-open-interval effects.
func (w *worker) refetchFullHistory(ctx context.Context, sym string) (int64, error) {
	var earliest time.Time
	err := w.pool.QueryRow(ctx, `
SELECT min(ts) FROM equity_ohlcv
WHERE symbol = $1 AND interval = $2 AND source = $3`,
		sym, w.cfg.BarInterval, w.cfg.BarSource).Scan(&earliest)

	to := time.Now().UTC()
	from := to.AddDate(-max(1, w.cfg.BackfillYears), 0, 0)
	if err == nil && !earliest.IsZero() && earliest.Before(from) {
		from = earliest.AddDate(0, 0, -7)
	}
	_, stored, ferr := w.fetchAndStore(ctx, sym, from, to)
	return stored, ferr
}
