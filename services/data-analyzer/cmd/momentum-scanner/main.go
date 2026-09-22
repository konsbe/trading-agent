// Command momentum-scanner is §8.2's daily pass: compute, gate, score, PERSIST.
//
// This is the step the live path was missing. momentum-dryrun, momentum-backtest
// and the §5 replay are all read-only — they compute in memory and report — so
// momentum_features and momentum_scores stayed empty while analyst-bot queried
// them every scan. Both sides' unit tests passed; nothing flowed between them.
//
// §8.2 responsibilities implemented here:
//  1. compute §3 features for every eligible symbol  -> momentum_features
//  2. apply §3.2 gates and assign a bucket           -> momentum_features
//  3. compute momentum_score_100 (§4)                -> momentum_scores
//
// Features and gate results are written for EVERY symbol, not just candidates.
// A gate failure is the answer to "why is this not a candidate" (§3.2 keeps the
// reasons for exactly that), and /score reads it to distinguish a thin-liquidity
// rejection from a missing-data one. Scores are written only for symbols that
// passed, because §3.2 is explicit that a gate failure means excluded rather
// than low-scored — persisting a score for a failed symbol would let it appear
// in a ranked query.
//
//	DATABASE_URL=... go run ./cmd/momentum-scanner
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/konsbe/trading-agent/services/data-analyzer/internal/compute"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/momentum"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/store"
)

func main() {
	source := flag.String("source", "tiingo", "equity_ohlcv.source to read")
	interval := flag.String("interval", "1Day", "equity_ohlcv.interval to read")
	dryRun := flag.Bool("dry-run", false, "compute and report without writing")
	flag.Parse()

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "db connect:", err)
		os.Exit(1)
	}
	defer pool.Close()

	bars, err := loadBars(ctx, pool, *interval, *source)
	if err != nil {
		fmt.Fprintln(os.Stderr, "load bars:", err)
		os.Exit(1)
	}
	if len(bars) == 0 {
		fmt.Printf("no bars for source=%s interval=%s\n", *source, *interval)
		return
	}

	marketCaps := loadMetric(ctx, pool, "market_cap")
	sharesOut := loadMetric(ctx, pool, "shares_outstanding")
	catalysts := loadCatalystTiers(ctx, pool)

	fcfg := momentum.DefaultConfig()
	gcfg := momentum.DefaultGateConfig()
	stats := momentum.NewGateStats()

	var featuresWritten, scoresWritten, skipped int
	var errs int

	for sym, series := range bars {
		if len(series) == 0 {
			continue
		}
		last := len(series) - 1
		f := momentum.ComputeAt(series, last, fcfg)
		ts := series[last].TS

		if f.Close == nil {
			// Nothing downstream can be evaluated without a close. Counted
			// separately so it does not masquerade as a gate rejection.
			skipped++
			continue
		}

		row := store.FeatureRow{TS: ts, Symbol: sym, Features: &f}

		gi := momentum.GateInput{}
		if mc, ok := marketCaps[sym]; ok && mc > 0 {
			gi.MarketCap = &mc
			row.MarketCap = &mc
		} else if so, ok := sharesOut[sym]; ok && so > 0 {
			// §3.9's documented fallback, flagged so the proxy's contribution to
			// the candidate set stays measurable.
			est := so * *f.Close
			gi.MarketCap = &est
			gi.MarketCapIsProxy = true
			row.MarketCapEst = &est
		}
		if so, ok := sharesOut[sym]; ok && so > 0 {
			shares := so
			row.FloatSharesEst = &shares
		}
		if t, ok := catalysts[sym]; ok && t != "" {
			tier := t
			row.CatalystTier = &tier
		}

		g := momentum.EvaluateGates(&f, gi, gcfg)
		row.Gate = g
		stats.Add(g)

		if !*dryRun {
			if err := store.UpsertFeatures(ctx, pool, row); err != nil {
				fmt.Fprintln(os.Stderr, err)
				errs++
				continue
			}
		}
		featuresWritten++

		if !g.Passed {
			continue
		}

		si := momentum.ScoreInput{}
		if row.FloatSharesEst != nil {
			si.FloatSharesEst = row.FloatSharesEst
		}
		if row.CatalystTier != nil {
			si.CatalystTier = momentum.CatalystTier(*row.CatalystTier)
		}
		s, ok := momentum.ScoreCandidate(&f, si, g)
		if !ok {
			continue
		}
		if !*dryRun {
			if err := store.UpsertScore(ctx, pool, ts, sym, s); err != nil {
				fmt.Fprintln(os.Stderr, err)
				errs++
				continue
			}
		}
		scoresWritten++
		fmt.Printf("  candidate %-7s %s score=%d/%d\n", sym, s.Bucket, s.Total, momentum.WeightAllocated)
	}

	fmt.Println("\n§8.2 scanner:")
	fmt.Printf("  symbols with bars:    %d\n", len(bars))
	fmt.Printf("  no usable close:      %d\n", skipped)
	fmt.Printf("  feature rows written: %d\n", featuresWritten)
	fmt.Printf("  score rows written:   %d (gate-passing only)\n", scoresWritten)
	fmt.Printf("  write errors:         %d\n", errs)
	fmt.Printf("  gate rejections:      %v\n", stats.TopFailures(6))
	if *dryRun {
		fmt.Println("  DRY RUN — nothing written")
	}
}

func loadBars(ctx context.Context, pool *pgxpool.Pool, interval, source string) (map[string][]compute.Bar, error) {
	rows, err := pool.Query(ctx, `
SELECT o.symbol, o.ts, o.open, o.high, o.low, o.close, o.volume
FROM equity_ohlcv o
-- data_unavailable_reason excludes symbols whose history the provider no
-- longer serves. Their stored bars are a frozen remnant, and the feature
-- engine would compute a 20-day RVOL and a 252-bar high from it and emit a
-- candidate indistinguishable from a live one. See migration 022.
JOIN universe_symbols u ON u.symbol = o.symbol AND u.is_eligible
                       AND u.data_unavailable_reason IS NULL
WHERE o.interval = $1 AND o.source = $2 AND o.close > 0
ORDER BY o.symbol, o.ts`, interval, source)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string][]compute.Bar{}
	for rows.Next() {
		var sym string
		var b compute.Bar
		if err := rows.Scan(&sym, &b.TS, &b.Open, &b.High, &b.Low, &b.Close, &b.Volume); err != nil {
			return nil, err
		}
		out[sym] = append(out[sym], b)
	}
	return out, rows.Err()
}

func loadMetric(ctx context.Context, pool *pgxpool.Pool, metric string) map[string]float64 {
	out := map[string]float64{}
	rows, err := pool.Query(ctx, `
SELECT DISTINCT ON (symbol) symbol, value
FROM equity_fundamentals
WHERE metric = $1 AND value IS NOT NULL AND value > 0
-- The NOT NULL filter already prevents this picking a NULL, which is what
-- saved the live scanner from the universe-loader bug. The source rank is
-- still required for DETERMINISM: two sources can both report a non-null
-- market_cap at the same ts, and without a tiebreak the gate input would
-- depend on physical row order.
ORDER BY symbol, ts DESC, fundamental_source_rank(source) DESC`, metric)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var sym string
		var v float64
		if err := rows.Scan(&sym, &v); err != nil {
			return out
		}
		out[sym] = v
	}
	return out
}

// loadCatalystTiers reads the highest tier recorded per symbol.
//
// Highest rather than most recent: §3.11 classifies a window into its strongest
// signal, so a window holding both an upgrade and an acquisition is tier A.
func loadCatalystTiers(ctx context.Context, pool *pgxpool.Pool) map[string]string {
	out := map[string]string{}
	rows, err := pool.Query(ctx, `
SELECT DISTINCT ON (symbol) symbol, tier
FROM catalyst_events
WHERE ts::date >= (now() - interval '2 days')::date
-- Tier first (§3.11 takes the window's strongest signal), then ts and
-- source so that two providers reporting the same tier resolve the same way
-- on every run. catalyst_events has 4 writers and 3 colliding groups.
ORDER BY symbol, CASE tier WHEN 'A' THEN 2 WHEN 'B' THEN 1 ELSE 0 END DESC,
         ts DESC, source`)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var sym, tier string
		if err := rows.Scan(&sym, &tier); err != nil {
			return out
		}
		out[sym] = tier
	}
	return out
}
