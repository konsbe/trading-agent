// data-universe maintains universe_symbols: the eligible US common-stock
// universe the momentum scanner operates on.
//
// Spec: docs/MOMENTUM_SCANNER_PHASE1.md §8.1. Build-order step 2 covers the two
// weekly passes implemented here:
//
//	runSymbols       refresh the symbol list and apply §3.1 eligibility
//	runFundamentals  refresh sector / shares outstanding / market cap
//
// The resumable 3-year bar backfill (§8.1.3) and the daily incremental bar
// refresh (§8.1.4) are later build-order steps and are not wired up yet.
//
// Every write is an upsert, so the worker is safe to re-run and safe to kill.
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
	"github.com/konsbe/trading-agent/services/data-ingestion/internal/fetch/finnhub"
	"github.com/konsbe/trading-agent/services/data-ingestion/internal/logx"
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

	if !cfg.EnableSymbols && !cfg.EnableFundamentals {
		log.Info("both universe passes disabled; exiting")
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

	fh := finnhub.New(cfg.FinnhubKey)
	if cfg.EnableSymbols && !fh.HasToken() {
		// Consistent with the other workers: a missing key disables the pass and
		// logs, it never crashes the service.
		log.Warn("FINNHUB_API_KEY not set; symbol-list refresh disabled")
		cfg.EnableSymbols = false
	}
	if !cfg.EnableSymbols && !cfg.EnableFundamentals {
		log.Info("no usable universe passes after config checks; exiting")
		return
	}

	w := &worker{
		cfg:   cfg,
		pool:  pool,
		fh:    fh,
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

	tSymbols := time.NewTicker(cfg.PollSymbols)
	defer tSymbols.Stop()
	tFundamentals := time.NewTicker(cfg.PollFundamentals)
	defer tFundamentals.Stop()

	log.Info("data-universe running",
		"symbols_every", cfg.PollSymbols.String(),
		"fundamentals_every", cfg.PollFundamentals.String())

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
		}
	}
}

type worker struct {
	cfg   config.Universe
	pool  *pgxpool.Pool
	fh    *finnhub.Client
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
