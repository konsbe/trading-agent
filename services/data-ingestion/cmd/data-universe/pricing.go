package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/konsbe/trading-agent/services/data-ingestion/internal/fetch/finnhub"
	"github.com/konsbe/trading-agent/services/data-ingestion/internal/ratelimit"
	"github.com/konsbe/trading-agent/services/data-ingestion/internal/store"
)

// runUniversePricing fetches an approximate current price for every eligible
// symbol, purely so the pilot subset can be stratified by §3.2's price buckets.
//
// Why this exists, and why it uses Finnhub rather than the bar provider:
//
// Stratifying needs a price per symbol, but prices normally come from the
// backfill that the selection chooses — a bootstrap loop. Solving it with the
// bar provider costs Tiingo quota twice over: a first draw to establish prices
// and a second stratified draw would spend ~900 of a 500-unique-symbols/month
// allowance, and the two draws come from different random processes, so there is
// no guarantee the first draw's bucket boundaries generalise before the real
// selection is committed.
//
// Finnhub's /quote is already integrated, already paced through the shared
// budget, and costs zero Tiingo symbols. Pricing the whole universe once at
// ~1 req/s is roughly 83 minutes for ~4,978 symbols, and yields one clean signal
// for one clean draw.
//
// Rows are stored as equity_ohlcv with interval='quote_snapshot' and
// source='finnhub_quote' — the pattern data-equity already uses and SCHEMAS.md
// already documents, so this needs no migration and no new table. They are
// deliberately NOT daily bars: volume is 0 and the timestamp is wall-clock, so
// nothing that reads interval='1Day' can mistake them for history.
func (w *worker) runUniversePricing(ctx context.Context) {
	started := time.Now()

	members, err := store.EligibleUniverse(ctx, w.pool)
	if err != nil {
		w.log.Error("load eligible universe for pricing", "err", err)
		return
	}
	if len(members) == 0 {
		w.log.Info("universe pricing: no eligible symbols yet")
		return
	}

	var ok, failed, quotaStopped int
	batch := make([]store.EquityBar, 0, 200)

	// Progress is reported on a timer, not every N symbols. At ~1 req/sec a
	// count-based interval means minutes of total silence on a pass that runs
	// well over an hour, which is indistinguishable from a hang.
	const progressEvery = 60 * time.Second
	lastReport := started

	flush := func() {
		if len(batch) == 0 {
			return
		}
		if _, err := store.UpsertEquityOHLCVBatch(ctx, w.pool, batch); err != nil {
			w.log.Error("upsert quote snapshots", "err", err)
		}
		batch = batch[:0]
	}

	for i, m := range members {
		if ctx.Err() != nil {
			break
		}
		q, err := w.fh.Quote(ctx, m.Symbol)
		if err != nil {
			// A spent daily quota is not a per-symbol failure and must not be
			// retried symbol by symbol — stop the pass and resume next run. The
			// shared limiter reports it distinguishably precisely so callers can
			// tell "come back tomorrow" from "this symbol is broken".
			if errors.Is(err, ratelimit.ErrDailyQuotaExhausted) {
				quotaStopped = len(members) - i
				w.log.Warn("universe pricing stopped: shared daily quota exhausted; resuming on the next run",
					"priced", ok, "remaining", quotaStopped)
				break
			}
			failed++
			continue
		}
		bar, okBar := finnhub.StoreQuoteAsEquityBar(m.Symbol, q)
		if !okBar || bar.Close <= 0 {
			// No usable price. Left absent rather than stored as zero: a zero
			// close would bucket the symbol as penny and quietly corrupt the
			// stratification.
			failed++
			continue
		}
		batch = append(batch, bar)
		ok++
		if len(batch) >= 200 {
			flush()
		}
		if time.Since(lastReport) >= progressEvery {
			lastReport = time.Now()
			done := i + 1
			var eta string
			if rate := float64(done) / time.Since(started).Seconds(); rate > 0 {
				eta = time.Duration(float64(len(members)-done) / rate * float64(time.Second)).Round(time.Minute).String()
			}
			w.log.Info("universe pricing progress",
				"priced", ok, "failed", failed, "of", len(members),
				"pct", fmt.Sprintf("%.1f%%", 100*float64(done)/float64(len(members))),
				"elapsed", time.Since(started).Round(time.Second).String(),
				"eta", eta)
		}
	}
	flush()

	cov, err := store.LoadPriceCoverage(ctx, w.pool, w.cfg.PriceInterval, w.cfg.PriceSource,
		w.cfg.SubsetPennyMinPrice, w.cfg.SubsetPennyMaxPrice)
	if err != nil {
		w.log.Warn("price coverage", "err", err)
	}

	w.log.Info("universe pricing complete",
		"priced_now", ok,
		"failed", failed,
		"stopped_remaining", quotaStopped,
		"eligible", len(members),
		"with_price_total", cov.Priced,
		"penny_bucket", cov.Penny,
		"market_bucket", cov.Market,
		"below_penny_floor", cov.BelowFloor,
		"took", time.Since(started).Round(time.Second).String())

	// The stratified draw needs enough of each bucket to honour its floor. Saying
	// so here, with the numbers, beats discovering it when the selection refuses.
	wantPenny := int(float64(w.cfg.SubsetSize) * w.cfg.SubsetPennyFloorPct)
	if cov.Penny < wantPenny {
		w.log.Warn("priced penny bucket is smaller than the subset's penny floor; the stratified draw will refuse until more symbols are priced",
			"penny_available", cov.Penny, "penny_floor_needed", wantPenny)
	}
}
