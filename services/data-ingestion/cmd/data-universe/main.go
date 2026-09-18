// data-universe maintains universe_symbols: the eligible US common-stock
// universe the momentum scanner operates on.
//
// Spec: docs/MOMENTUM_SCANNER_PHASE1.md §8.1. Four passes, each independently
// switchable:
//
//	runSymbols        weekly   refresh the symbol list, apply §3.1 eligibility
//	runFundamentals   weekly   refresh sector / shares outstanding / market cap
//	runBackfillRound  looping  resumable 3-year bar backfill (§8.1.3)
//	runDailyBars      daily    incremental bar refresh after the US close (§8.1.4)
//
// Every write is an upsert and the backfill checkpoints per symbol, so the
// worker is safe to re-run and safe to kill at any point.
package main

import (
	"context"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"github.com/konsbe/trading-agent/services/data-ingestion/internal/config"
	"github.com/konsbe/trading-agent/services/data-ingestion/internal/db"
	"github.com/konsbe/trading-agent/services/data-ingestion/internal/fetch/barsource"
	"github.com/konsbe/trading-agent/services/data-ingestion/internal/fetch/finnhub"
	"github.com/konsbe/trading-agent/services/data-ingestion/internal/logx"
	"github.com/konsbe/trading-agent/services/data-ingestion/internal/ratelimit"
	"github.com/konsbe/trading-agent/services/data-ingestion/internal/store"
	"github.com/konsbe/trading-agent/services/data-ingestion/internal/universe"

	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	_ = godotenv.Load()

	cfg, err := config.LoadUniverse()
	if err != nil {
		slog.Error("config", "err", err)
		os.Exit(1)
	}
	log := logx.New(cfg.LogLevel)

	// Pricing and subset selection are passes in their own right: running them
	// alone is the normal way to prepare a pilot draw without spending any bar
	// provider quota. Omitting them here made a pricing-only run exit at startup.
	if !cfg.EnableSymbols && !cfg.EnableFundamentals && !cfg.EnableBackfill &&
		!cfg.EnableDailyBars && !cfg.PricingEnable && !cfg.SubsetEnable {
		log.Info("all universe passes disabled; exiting")
		return
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("db connect", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	fh := finnhub.NewWithLimiter(cfg.FinnhubKey, ratelimit.SharedFinnhub(ctx, pool, log))
	if cfg.EnableSymbols && !fh.HasToken() {
		// Consistent with the other workers: a missing key disables the pass and
		// logs, it never crashes the service.
		log.Warn("FINNHUB_API_KEY not set; symbol-list refresh disabled")
		cfg.EnableSymbols = false
	}
	if cfg.PricingEnable && !fh.HasToken() {
		// Same contract as the symbol pass: a missing key disables the pass with a
		// warning rather than crashing. Left enabled it would fail once per
		// symbol for ~4,975 symbols and look like a provider outage.
		log.Warn("FINNHUB_API_KEY not set; universe pricing disabled — the stratified subset draw has no bucketing signal without it")
		cfg.PricingEnable = false
	}
	if !cfg.EnableSymbols && !cfg.EnableFundamentals && !cfg.EnableBackfill &&
		!cfg.EnableDailyBars && !cfg.PricingEnable && !cfg.SubsetEnable {
		log.Info("no usable universe passes after config checks; exiting")
		return
	}

	// Bar provider is chosen by UNIVERSE_BAR_SOURCE. The backfill and daily
	// refresh never name a provider — they hold a barsource.Fetcher — so adding
	// or swapping one is configuration rather than a rewrite.
	//
	// The limiter is shared across every goroutine in the client, so
	// BackfillConcurrency controls pipelining, not throughput.
	bars, err := buildBarFetcher(ctx, cfg, pool, log)
	if err != nil {
		log.Error("bar source", "source", cfg.BarSource, "err", err)
		os.Exit(1)
	}

	w := &worker{
		cfg:   cfg,
		pool:  pool,
		fh:    fh,
		bars:  bars,
		log:   log,
		rules: universe.NewRules(cfg.AllowedTypes, cfg.AllowedMICs, cfg.ExcludedSuffixes, cfg.AllowEmptyMIC),
	}

	if cfg.StartupDelay > 0 {
		log.Info("startup delay", "secs", cfg.StartupDelay.Seconds())
		if !sleepCtx(ctx, cfg.StartupDelay) {
			return
		}
	}

	// Run both passes immediately, then on their own cadences.
	if cfg.EnableSymbols {
		w.runSymbols(ctx)
	}
	if cfg.EnableFundamentals {
		w.runFundamentals(ctx)
	}
	// Pricing must precede selection: stratification buckets symbols by price.
	if cfg.PricingEnable {
		w.runUniversePricing(ctx)
	}
	if cfg.SubsetEnable {
		w.runSubsetSelection(ctx)
	}

	tSymbols := time.NewTicker(cfg.PollSymbols)
	defer tSymbols.Stop()
	tFundamentals := time.NewTicker(cfg.PollFundamentals)
	defer tFundamentals.Stop()
	tDailyBars := time.NewTicker(cfg.DailyBarsInterval)
	defer tDailyBars.Stop()

	// The backfill runs back-to-back batches while work remains, then idles. A
	// timer rather than a ticker so the interval is measured from the end of the
	// previous round, not its start — a long batch must not queue up more.
	backfillTimer := time.NewTimer(0)
	if !cfg.EnableBackfill {
		backfillTimer.Stop()
	}
	defer backfillTimer.Stop()

	log.Info("data-universe running",
		"symbols_every", cfg.PollSymbols.String(),
		"fundamentals_every", cfg.PollFundamentals.String(),
		"daily_bars_every", cfg.DailyBarsInterval.String(),
		"backfill_enabled", cfg.EnableBackfill,
		"backfill_years", cfg.BackfillYears,
		// Named after the setting, not after one provider: the field read
		// "yahoo_req_per_sec" while UNIVERSE_BAR_SOURCE was tiingo, which
		// invites exactly the wrong conclusion when someone is debugging pacing.
		"bar_source", cfg.BarSource,
		// The provider's own pace, not the generic knob: this field read
		// "bar_req_per_sec=2" while the Twelve Data limiter was actually running
		// at 0.125/s, which is the opposite of useful when debugging throughput.
		"bar_req_per_sec", barPaceFor(cfg),
		"bar_timeout", cfg.RequestTimeout.String())

	// Tracks the backfill's drained/working edge so the post-backfill consistency

	// report fires on the transition, not on every idle tick.

	backfillDrained := false

	for {
		select {
		case <-ctx.Done():
			log.Info("shutdown")
			return
		case <-tSymbols.C:
			if cfg.EnableSymbols {
				w.runSymbols(ctx)
			}
		case <-tFundamentals.C:
			if cfg.EnableFundamentals {
				w.runFundamentals(ctx)
			}
		case <-tDailyBars.C:
			if cfg.EnableDailyBars {
				w.runDailyBars(ctx)
			}
		case <-backfillTimer.C:
			processed := w.runBackfillRound(ctx)
			// Immediately continue while there is work; idle once drained so a
			// finished backfill stops polling the database every second.
			next := time.Duration(0)
			if processed == 0 {
				next = cfg.BackfillIdleInterval
				// Just drained. Report coverage and bucket drift on the
				// transition rather than every idle tick, so the check lands in
				// the log exactly when both the selection price and the real
				// bars exist to be compared.
				if cfg.SubsetEnable && !backfillDrained {
					w.reportSubsetConsistency(ctx)
				}
			}
			backfillDrained = processed == 0
			backfillTimer.Reset(next)
		}
	}
}

type worker struct {
	cfg   config.Universe
	pool  *pgxpool.Pool
	fh    *finnhub.Client
	bars  barsource.Fetcher
	log   *slog.Logger
	rules universe.Rules
}

// runSymbols fetches the exchange symbol directory, applies §3.1, and upserts
// every decision — including the exclusions, which are retained with a reason
// so the filter stays auditable (§10 step 2).
func (w *worker) runSymbols(ctx context.Context) {
	started := time.Now()

	syms, err := w.fh.StockSymbols(ctx, w.cfg.Exchange)
	if err != nil {
		w.log.Error("fetch symbol list", "exchange", w.cfg.Exchange, "err", err)
		return
	}
	if len(syms) == 0 {
		// Never write an empty universe: the transactional upsert would leave the
		// previous set in place, but an empty fetch is a provider problem worth
		// surfacing rather than a universe of zero.
		w.log.Warn("symbol list empty; leaving existing universe untouched",
			"exchange", w.cfg.Exchange)
		return
	}

	recs := make([]universe.Record, 0, len(syms))
	meta := make(map[string]finnhub.StockSymbol, len(syms))
	for _, s := range syms {
		recs = append(recs, universe.Record{
			Symbol:      s.Symbol,
			DisplayName: s.Description,
			Type:        s.Type,
			MIC:         s.MIC,
			Currency:    s.Currency,
			FIGI:        s.FIGI,
		})
		// Decisions carry the normalised symbol, so key the raw record by it to
		// recover mic/name/type when building rows.
		key := strings.ToUpper(strings.TrimSpace(s.Symbol))
		if _, exists := meta[key]; !exists {
			meta[key] = s
		}
	}

	plan := w.rules.BuildPlan(recs)
	rows := make([]store.UniverseRow, 0, len(plan.Decisions))
	for _, d := range plan.Decisions {
		s := meta[d.Symbol]
		rows = append(rows, store.UniverseRow{
			Symbol:         d.Symbol,
			Exchange:       d.Exchange,
			MIC:            s.MIC,
			Name:           s.Description,
			Type:           s.Type,
			IsEligible:     d.Eligible,
			ExcludedReason: d.Reason,
		})
	}

	affected, err := store.UpsertUniverseSymbols(ctx, w.pool, rows)
	if err != nil {
		w.log.Error("upsert universe symbols", "err", err)
		return
	}

	// bar_count is informational, and only meaningful once bars exist. Refreshing
	// it here keeps the audit view current without gating eligibility on it.
	if _, err := store.RefreshUniverseBarCounts(ctx, w.pool, w.cfg.BarInterval, w.cfg.BarSource); err != nil {
		w.log.Warn("refresh bar counts", "err", err)
	}

	counts, err := store.LoadUniverseCounts(ctx, w.pool)
	if err != nil {
		w.log.Warn("load universe counts", "err", err)
	}

	attrs := []any{
		"exchange", w.cfg.Exchange,
		"fetched", len(syms),
		"persisted", len(rows),
		"upserted", affected,
		"eligible", counts.Eligible,
		"total_rows", counts.Total,
		// Two distinct reasons a fetched record is not persisted, kept apart so
		// the filter does not look broken when it is only deduplicating.
		"skipped_off_venue", plan.SkippedOffVenue,
		"skipped_duplicate", plan.SkippedDuplicate,
		"took", time.Since(started).Round(time.Millisecond).String(),
	}
	for _, reason := range plan.Tally.ReasonsSorted() {
		attrs = append(attrs, "excl_"+reason, plan.Tally.ByReason[reason])
	}
	w.log.Info("universe symbol refresh complete", attrs...)

	for ex, n := range counts.ByExchange {
		w.log.Info("eligible by exchange", "exchange", ex, "count", n)
	}

	// §2.3 expects roughly 5,000-7,500 eligible US common stocks. A count far
	// outside that says the type or MIC allowlist is wrong, which would silently
	// shrink or inflate everything downstream.
	switch {
	case counts.Eligible < 3000:
		w.log.Warn("eligible universe smaller than the ~5-7.5k the spec expects; check UNIVERSE_ALLOWED_TYPES / UNIVERSE_ALLOWED_MICS",
			"eligible", counts.Eligible)
	case counts.Eligible > 12000:
		w.log.Warn("eligible universe larger than the ~5-7.5k the spec expects; the type allowlist may be admitting non-common instruments",
			"eligible", counts.Eligible)
	}
}

// runFundamentals copies sector, industry, shares outstanding and market cap
// from equity_fundamentals onto the universe rows, converting the provider's
// millions to absolute units.
//
// It deliberately makes no API calls — §8.1.2 says to refresh "from existing
// fundamentals ingestion", and a per-symbol metric fetch across the whole
// universe would take hours against the free rate limit (§2.3).
func (w *worker) runFundamentals(ctx context.Context) {
	started := time.Now()

	rows, err := store.LoadFundamentalsFromEquityFundamentals(ctx, w.pool)
	if err != nil {
		w.log.Error("load fundamentals for universe", "err", err)
		return
	}

	updated, err := store.UpdateUniverseFundamentals(ctx, w.pool, rows)
	if err != nil {
		w.log.Error("update universe fundamentals", "err", err)
		return
	}

	cov, err := store.LoadFundamentalsCoverage(ctx, w.pool)
	if err != nil {
		w.log.Warn("fundamentals coverage", "err", err)
		return
	}

	w.log.Info("universe fundamentals refresh complete",
		"matched", len(rows),
		"updated", updated,
		"eligible", cov.Eligible,
		"with_sector", cov.WithSector,
		"with_shares", cov.WithShares,
		"with_market_cap", cov.WithMarketCap,
		"took", time.Since(started).Round(time.Millisecond).String())

	// Coverage drives two later steps, so a shortfall is logged loudly now rather
	// than discovered as an empty candidate set at step 5.
	if cov.Eligible > 0 && cov.WithNeitherCapNor == cov.Eligible {
		w.log.Warn("no eligible symbol has market cap or shares outstanding; the §3.2 market-cap gate and the §3.9 market_cap_est proxy will both be unavailable — point FUNDAMENTAL_SYMBOLS at the universe or add a metric fetch pass",
			"eligible", cov.Eligible)
		return
	}
	if cov.WithNeitherCapNor > 0 {
		w.log.Warn("symbols with neither market cap nor shares outstanding will fail the §3.2 market-cap gate (market_cap_est cannot be computed)",
			"affected", cov.WithNeitherCapNor,
			"of_eligible", cov.Eligible)
	}
	if cov.WithSector == 0 && cov.Eligible > 0 {
		w.log.Warn("no eligible symbol has a sector; §3.12 sector_strength_pct will be null for the whole universe (sector comes from the Alpha Vantage overview pass)")
	}
}

// sleepCtx waits for d, returning false if the context is cancelled first.
func sleepCtx(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}
