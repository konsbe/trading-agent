package main

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/konsbe/trading-agent/services/data-ingestion/internal/config"
	"github.com/konsbe/trading-agent/services/data-ingestion/internal/fetch/barsource"
	"github.com/konsbe/trading-agent/services/data-ingestion/internal/fetch/tiingo"
	"github.com/konsbe/trading-agent/services/data-ingestion/internal/fetch/yahoo"
	"github.com/konsbe/trading-agent/services/data-ingestion/internal/ratelimit"
	"github.com/konsbe/trading-agent/services/data-ingestion/internal/store"
)

// buildBarFetcher selects the bar provider from UNIVERSE_BAR_SOURCE.
//
// The value doubles as equity_ohlcv.source, so it decides both who is called and
// how the rows are labelled — and the scanner reads bars back by that same
// label. Keeping them one setting removes the class of bug where a provider is
// swapped but the reader still filters on the old source and finds nothing.
//
// Unknown values fail at startup rather than defaulting. A silent fallback would
// mean writing rows under a source nobody reads, which presents as an empty
// backfill rather than as a configuration error.
func buildBarFetcher(ctx context.Context, cfg config.Universe, pool *pgxpool.Pool, log *slog.Logger) (barsource.Fetcher, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.BarSource)) {
	case tiingo.SourceName:
		if cfg.TiingoToken == "" {
			return nil, fmt.Errorf("UNIVERSE_BAR_SOURCE=%s requires TIINGO_API_KEY", tiingo.SourceName)
		}
		// No daily ceiling on this budget: Tiingo meters unique symbols per
		// month, which a rate limiter cannot express. The subset size assertion
		// is the real guard.
		lim := ratelimit.SharedTiingo(ctx, pool, log)
		log.Info("bar source: tiingo",
			"adjusted", "split+dividend (adj* fields)",
			"quota", "500 unique symbols/month — enforced by the subset size cap, not by the limiter")
		return tiingo.NewWithLimiter(cfg.TiingoToken, lim, tiingo.Options{
			RequestsPerSecond: cfg.TiingoRequestsPerSecond,
			Burst:             cfg.RequestBurst,
			Timeout:           cfg.RequestTimeout,
			MaxRetries:        cfg.RequestMaxRetries,
			BackoffBase:       cfg.BackoffBase,
			BackoffMax:        cfg.BackoffMax,
		}), nil

	case yahoo.SourceName, "yahoo":
		// Retained because data-technical already writes rows under this source.
		// Two caveats, both documented in data_ingestion.md: it returns a blanket
		// 429 from some corporate networks (IP reputation, not rate), and it
		// reads indicators.quote only, so its bars carry NO dividend adjustment
		// and do not satisfy §3's "split/dividend-adjusted" requirement.
		log.Warn("bar source: yahoo_finance — bars are NOT dividend-adjusted (indicators.adjclose is never decoded), so §3's adjustment requirement is unmet on this path; see data_ingestion.md")
		return yahoo.NewWithOptions(yahoo.Options{
			RequestsPerSecond: cfg.RequestsPerSecond,
			Burst:             cfg.RequestBurst,
			Timeout:           cfg.RequestTimeout,
			MaxRetries:        cfg.RequestMaxRetries,
			BackoffBase:       cfg.BackoffBase,
			BackoffMax:        cfg.BackoffMax,
		}), nil

	default:
		return nil, fmt.Errorf(
			"unknown UNIVERSE_BAR_SOURCE %q; expected %q or %q — an unrecognised value would write rows under a source the scanner never reads",
			cfg.BarSource, tiingo.SourceName, yahoo.SourceName)
	}
}

// runSubsetSelection marks the pilot subset (§2.2).
//
// Runs after the symbol refresh so a newly-listed symbol can be drawn, and
// before the backfill claims anything. Reselection replaces rather than
// accumulates, so a repeated run is safe.
func (w *worker) runSubsetSelection(ctx context.Context) {
	started := time.Now()
	cfg := w.cfg

	// A committed draw is not redrawn on restart unless asked for explicitly.
	//
	// UNIVERSE_SUBSET_ENABLE means "restrict work to the pilot subset", and the
	// backfill needs it set for the whole pilot. Letting it also trigger a draw
	// meant every restart reselected. With a fixed seed that is merely wasted
	// work — the hash-based ordering reproduces the same 450 — but with an empty
	// seed it silently draws a DIFFERENT 450 and spends another 450 of Tiingo's
	// 500-unique-symbols/month allowance, blowing the cap and invalidating the
	// base rate that the pilot exists to measure.
	//
	// Neither symptom is visible at a glance: the row count is 450 either way.
	selected, err := store.CountSelected(ctx, w.pool)
	if err != nil {
		w.log.Error("count existing selection", "err", err)
		return
	}
	if selected > 0 && !cfg.SubsetReselect {
		w.log.Info("pilot subset already selected; keeping it",
			"selected", selected,
			"note", "set UNIVERSE_SUBSET_RESELECT=true to draw a new sample — on a metered provider that spends a second batch of unique symbols")
		return
	}
	if cfg.SubsetReselect && cfg.SubsetSeed == "" {
		// Reproducibility is the difference between a citable pilot and a
		// number nobody can regenerate.
		w.log.Warn("reselecting with an empty UNIVERSE_SUBSET_SEED; this sample cannot be reproduced",
			"previous_selected", selected)
	}

	n, err := store.SelectPilotSubset(ctx, w.pool, store.SelectSubsetParams{
		Strategy:      store.SubsetStrategy(cfg.SubsetStrategy),
		Size:          cfg.SubsetSize,
		MaxSize:       cfg.SubsetMaxSize,
		Symbols:       cfg.SubsetSymbols,
		Seed:          cfg.SubsetSeed,
		PennyFloorPct: cfg.SubsetPennyFloorPct,
		PennyMinPrice: cfg.SubsetPennyMinPrice,
		PennyMaxPrice: cfg.SubsetPennyMaxPrice,
		PriceInterval: cfg.PriceInterval,
		PriceSource:   cfg.PriceSource,
	})
	if err != nil {
		// Both failure modes are actionable and self-describing: the size cap
		// explains Tiingo's monthly allowance, and a stratification failure
		// explains the bootstrap order. Neither writes anything, so the previous
		// selection is intact and the backfill keeps working on it.
		w.log.Error("pilot subset selection failed; the previous selection is unchanged",
			"strategy", cfg.SubsetStrategy, "err", err)
		return
	}

	st, err := store.LoadSubsetStats(ctx, w.pool,
		cfg.PriceInterval, cfg.PriceSource, cfg.BarInterval, cfg.BarSource, cfg.SubsetPennyMaxPrice)
	if err != nil {
		w.log.Warn("subset stats", "err", err)
		return
	}

	w.log.Info("pilot subset selected",
		"strategy", cfg.SubsetStrategy,
		"selected", n,
		"cap", cfg.SubsetMaxSize,
		"seed", seedLabel(cfg.SubsetSeed),
		"under_2_dollars", st.UnderTwoDollars,
		"with_bars", st.WithBars,
		"took", time.Since(started).Round(time.Millisecond).String())

	// Post-hoc invariant on top of the stratification, cheap and worth checking
	// even when the upstream logic is believed to guarantee it: a subset with no
	// penny-bucket members leaves §3.2's penny thresholds, its separate
	// market-cap band, and the documented RSI-action unreachability entirely
	// unexercised — and §6's base rate would then only be computable for one
	// bucket.
	if st.Selected > 0 && st.UnderTwoDollars == 0 && st.WithBars > 0 {
		w.log.Warn("the selected subset contains no sub-$2 symbols; §3.2's penny bucket will be completely unexercised and §6's base rate computable for the market bucket only",
			"selected", st.Selected, "with_bars", st.WithBars)
	}
	if cfg.SubsetMaxSize > 0 && n > cfg.SubsetMaxSize {
		w.log.Error("selected subset exceeds the cap after writing — Tiingo's monthly unique-symbol allowance may be overspent",
			"selected", n, "cap", cfg.SubsetMaxSize)
	}
}

func seedLabel(s string) string {
	if s == "" {
		return "(unseeded — this draw is not reproducible)"
	}
	return s
}
