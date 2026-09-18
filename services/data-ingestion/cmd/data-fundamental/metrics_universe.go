package main

import (
	"context"
	"time"

	"github.com/konsbe/trading-agent/services/data-ingestion/internal/store"
)

// runMetricsUniverse is the checkpointed, universe-wide TTM metrics pass
// (spec §8.4, build-order step 3b).
//
// It exists because the momentum scanner's §3.2 market-cap gate and §3.9
// market_cap_est proxy need `market_cap` and `shares_outstanding` for the whole
// eligible universe, and those two fields come from this one sub-task. Without
// them, candidate counts collapse and §6's base rate is computed over a tiny
// non-representative slice — which would make step 5 look healthy for entirely
// the wrong reason.
//
// Three differences from runMetrics, and only three:
//
//   - Symbols are the UNION of the configured list and the eligible universe,
//     never a replacement for it (see store.ResolveMetricsSymbols).
//   - Work is claimed in batches against fundamental_fetch_state, so a ~3.3-hour
//     pass survives a restart instead of starting over.
//   - Per-symbol outcomes are recorded, so a persistently broken symbol stops
//     consuming the shared Finnhub budget after maxAttempts.
//
// Returns the number of symbols processed. Zero means no claimable work, which
// the caller uses to back off.
func (w *worker) runMetricsUniverse(ctx context.Context) int {
	started := time.Now()
	cfg := w.cfg

	// Seeding is idempotent and cheap, and running it every round is what makes a
	// newly-listed symbol appear without any cycle coordination.
	seeded, err := store.SeedFundamentalFetchState(ctx, w.pool, store.TaskMetrics,
		store.MetricsScope(w.cfg.MetricsScope))
	if err != nil {
		w.log.Error("seed fundamental fetch state", "err", err)
		return 0
	}
	if seeded > 0 {
		w.log.Info("seeded new symbols into the metrics fetch rotation", "count", seeded)
	}

	claims, err := store.ClaimFundamentalFetchBatch(ctx, w.pool, store.TaskMetrics,
		cfg.MetricsUniverseBatchSize,
		cfg.MetricsUniverseRefreshInterval,
		cfg.MetricsUniverseClaimLease,
		cfg.MetricsUniverseMaxAttempts)
	if err != nil {
		w.log.Error("claim metrics batch", "err", err)
		return 0
	}
	if len(claims) == 0 {
		return 0
	}

	// Sequential on purpose. The Finnhub budget is shared across every worker
	// (migration 008), so parallelism here would not raise throughput — it would
	// only deepen the queue in front of the shared limiter and starve the other
	// four workers' short, latency-sensitive polls.
	var ok, failed int
	for _, c := range claims {
		if ctx.Err() != nil {
			break
		}
		ts := time.Now().UTC()
		if err := w.fetchMetricsForSymbol(ctx, c.Symbol, ts); err != nil {
			failed++
			if e := store.MarkFundamentalFetchFailed(ctx, w.pool, c.Symbol, store.TaskMetrics, err.Error()); e != nil {
				w.log.Warn("mark metrics fetch failed", "symbol", c.Symbol, "err", e)
			}
			w.log.Debug("metrics fetch failed", "symbol", c.Symbol, "attempt", c.Attempts+1, "err", err)
			continue
		}
		ok++
		if e := store.MarkFundamentalFetchDone(ctx, w.pool, c.Symbol, store.TaskMetrics); e != nil {
			w.log.Warn("mark metrics fetch done", "symbol", c.Symbol, "err", e)
		}
	}

	prog, err := store.LoadFundamentalFetchProgress(ctx, w.pool, store.TaskMetrics,
		cfg.MetricsUniverseMaxAttempts, cfg.MetricsUniverseRefreshInterval)
	if err != nil {
		w.log.Warn("metrics fetch progress", "err", err)
	}

	w.log.Info("universe metrics batch complete",
		"claimed", len(claims),
		"ok", ok,
		"failed", failed,
		"total", prog.Total,
		"fresh", prog.Fresh,
		"pending", prog.Pending,
		"failed_total", prog.Failed,
		"in_progress", prog.InProgress,
		"took", time.Since(started).Round(time.Second).String())

	if prog.Exhausted > 0 {
		w.log.Warn("symbols exhausted their metrics fetch attempts and will no longer be claimed; inspect fundamental_fetch_state.last_error",
			"exhausted", prog.Exhausted, "max_attempts", cfg.MetricsUniverseMaxAttempts)
	}

	// Coverage is the number step 5 depends on. 'fresh' rather than 'done' is the
	// honest measure: 'done' only says the last attempt worked, while 'fresh'
	// says the data is inside the refresh window.
	if prog.Total > 0 && prog.Fresh*2 < prog.Total {
		w.log.Warn("fewer than half of universe symbols have fresh fundamentals; the §3.2 market-cap gate will exclude the rest, so candidate counts and §6 base rates are not yet meaningful",
			"fresh", prog.Fresh, "total", prog.Total)
	}
	return len(claims)
}

// metricsSymbolsForStaticPass resolves the symbol list for the non-checkpointed
// path, applying the union when the universe source is enabled.
//
// Kept separate from runMetricsUniverse because the two paths answer different
// questions: this one is "what should the static pass iterate", that one is
// "what work is outstanding". A failure to reach the universe degrades to the
// configured list rather than to an empty pass — never fail closed into
// fetching nothing.
func (w *worker) metricsSymbolsForStaticPass(ctx context.Context) []string {
	if !w.cfg.MetricsUseUniverse {
		return w.cfg.Symbols
	}
	scope := store.MetricsScope(w.cfg.MetricsScope)
	syms, err := store.ResolveMetricsSymbols(ctx, w.pool, w.cfg.Symbols, scope)
	if err != nil {
		// Includes an unrecognised scope. Degrading to the configured list is the
		// right failure here for the same reason as a database error: the pass
		// keeps working on the symbols it is sure about instead of fetching
		// nothing, and the warning names the cause.
		w.log.Warn("could not resolve the universe symbol list; falling back to the configured symbol list",
			"scope", w.cfg.MetricsScope, "configured", len(w.cfg.Symbols), "err", err)
		return w.cfg.Symbols
	}
	w.log.Info("metrics symbol source resolved",
		"scope", scope, "configured", len(w.cfg.Symbols), "total", len(syms),
		"note", "scope=selected covers the pilot draw only; scope=eligible covers the full §3.1 universe")
	if len(syms) == 0 {
		w.log.Warn("symbol resolution produced nothing; falling back to the configured list")
		return w.cfg.Symbols
	}
	return syms
}

// metricsUniverseCheckpointed reports whether the widened, checkpointed metrics
// pass is active. Both flags must be on: widening the symbol source without
// checkpointing would re-walk several thousand symbols on every tick.
func (w *worker) metricsUniverseCheckpointed() bool {
	return w.cfg.MetricsUseUniverse && w.cfg.MetricsUniverseCheckpointed
}
