// Command momentum-dryrun runs §3's features, §3.2's gates and §4's scorer over
// whatever real bars are already stored, and prints the gate funnel.
//
// It exists so Step 5/6 can be developed and sanity-checked against real data
// while the backfill is still running: the pipeline logic is not blocked by
// backfill completion, only the pilot's real base-rate numbers are.
//
// Deliberately read-only. It writes nothing to momentum_features or
// momentum_scores, so it cannot contaminate the pilot's record with results
// computed over a partial universe.
//
//	DATABASE_URL=... go run ./cmd/momentum-dryrun -source tiingo
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"sort"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/konsbe/trading-agent/services/data-analyzer/internal/compute"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/momentum"
)

type bar struct {
	ts                          string
	open, high, low, close, vol float64
}

func main() {
	source := flag.String("source", "tiingo", "equity_ohlcv.source to read")
	interval := flag.String("interval", "1Day", "equity_ohlcv.interval to read")
	top := flag.Int("top", 10, "how many top-scoring candidates to print")
	relaxed := flag.Bool("relaxed", false, "also report how the funnel looks with change_pct gates removed, to see whether the day simply had no movers")
	flag.Parse()

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "db connect:", err)
		os.Exit(1)
	}
	defer pool.Close()

	rows, err := pool.Query(ctx, `
SELECT o.symbol, o.ts::date::text, o.open, o.high, o.low, o.close, o.volume
FROM equity_ohlcv o
WHERE o.interval = $1 AND o.source = $2 AND o.close > 0
ORDER BY o.symbol, o.ts`, *interval, *source)
	if err != nil {
		fmt.Fprintln(os.Stderr, "query:", err)
		os.Exit(1)
	}
	defer rows.Close()

	bySymbol := map[string][]bar{}
	for rows.Next() {
		var sym string
		var b bar
		if err := rows.Scan(&sym, &b.ts, &b.open, &b.high, &b.low, &b.close, &b.vol); err != nil {
			fmt.Fprintln(os.Stderr, "scan:", err)
			os.Exit(1)
		}
		bySymbol[sym] = append(bySymbol[sym], b)
	}
	if len(bySymbol) == 0 {
		fmt.Printf("no bars for source=%s interval=%s\n", *source, *interval)
		return
	}

	// §3.2's market-cap gate, plus §3.9's proxy input. Latest value per symbol.
	//
	// market_cap is stored USD ABSOLUTE: Finnhub reports millions and the writer
	// multiplies by 1e6. Reading it as millions here would re-create the gate
	// inversion the writer fix removed, from the other end.
	marketCaps := loadMetric(ctx, pool, "market_cap")
	sharesOut := loadMetric(ctx, pool, "shares_outstanding")
	fmt.Printf("  market_cap available:    %d symbols\n", len(marketCaps))
	fmt.Printf("  shares_outstanding:      %d symbols (§3.9 proxy input)\n", len(sharesOut))

	fcfg := momentum.DefaultConfig()
	gcfg := momentum.DefaultGateConfig()
	stats := momentum.NewGateStats()

	type scored struct {
		symbol string
		score  momentum.Score
	}
	var candidates []scored
	var featureFailures int
	var blockedOnFundamentalsOnly int
	var proxied int

	for sym, bs := range bySymbol {
		f := momentum.Compute(toComputeBars(bs), fcfg)
		if f.Close == nil {
			// No usable close at t: nothing downstream can be evaluated, and
			// counting it separately keeps it out of the gate funnel where it
			// would masquerade as a gate rejection.
			featureFailures++
			continue
		}

		gi := momentum.GateInput{}
		if mc, ok := marketCaps[sym]; ok && mc > 0 {
			gi.MarketCap = &mc
		} else if so, ok := sharesOut[sym]; ok && so > 0 && f.Close != nil {
			// §3.9's documented fallback: shares_outstanding x close[t]. Flagged as
			// a proxy so the gate records market_cap_null as provenance and the
			// estimate's contribution to the candidate set stays measurable.
			est := so * *f.Close
			gi.MarketCap = &est
			gi.MarketCapIsProxy = true
			proxied++
		}
		g := momentum.EvaluateGates(&f, gi, gcfg)
		stats.Add(g)
		if !g.Passed {
			// "Blocked only by the missing fundamentals" is the actionable
			// number: it separates "the price/volume gates rejected this" from
			// "we simply have not run the fundamentals pass for it yet", which a
			// funnel of reason counts cannot distinguish because failures are
			// collected rather than short-circuited.
			if onlyMarketCapMissing(g.Failures) {
				blockedOnFundamentalsOnly++
			}
			continue
		}
		s, ok := momentum.ScoreCandidate(&f, momentum.ScoreInput{}, g)
		if ok {
			candidates = append(candidates, scored{sym, s})
		}
	}

	fmt.Printf("momentum dry run: source=%s interval=%s\n", *source, *interval)
	fmt.Printf("  symbols with bars:    %d\n", len(bySymbol))
	fmt.Printf("  feature compute nil:  %d\n", featureFailures)
	fmt.Printf("  gates evaluated:      %d\n", stats.Evaluated)
	fmt.Printf("  unbucketable (<$0.30 or null close): %d\n", stats.Unbucketed)
	fmt.Printf("  passed gates:         %d  (market=%d penny=%d)\n",
		stats.Passed, stats.PassedByBkt[momentum.BucketMarket], stats.PassedByBkt[momentum.BucketPenny])
	fmt.Printf("  gate failure reasons: %v\n", stats.TopFailures(12))
	fmt.Printf("  gated on a §3.9 market-cap proxy: %d\n", proxied)
	fmt.Printf("  would pass but for the missing market cap: %d\n", blockedOnFundamentalsOnly)

	if *relaxed {
		// The change_pct gates are the strategy, so on a quiet day they legitimately
		// empty the candidate set. Reporting the counterfactual separates "the
		// gates are miscoded" from "nothing moved today", which otherwise look
		// identical from a zero.
		rcfg := gcfg
		rcfg.Market.MinChangePct, rcfg.Market.MaxChangePct = -1e9, 1e9
		rcfg.Penny.MinChangePct, rcfg.Penny.MaxChangePct = -1e9, 1e9
		rstats := momentum.NewGateStats()
		for sym, bs := range bySymbol {
			f := momentum.Compute(toComputeBars(bs), fcfg)
			if f.Close == nil {
				continue
			}
			gi := momentum.GateInput{}
			if mc, ok := marketCaps[sym]; ok && mc > 0 {
				gi.MarketCap = &mc
			}
			rstats.Add(momentum.EvaluateGates(&f, gi, rcfg))
		}
		fmt.Printf("  [counterfactual] passed with change_pct gates removed: %d\n", rstats.Passed)
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].score.Total != candidates[j].score.Total {
			return candidates[i].score.Total > candidates[j].score.Total
		}
		return candidates[i].symbol < candidates[j].symbol
	})
	n := *top
	if n > len(candidates) {
		n = len(candidates)
	}
	if n > 0 {
		fmt.Println("  top candidates:")
		for _, c := range candidates[:n] {
			fmt.Printf("    %-7s %3d  accel=%.1f rvol=%.1f brk=%.0f cat=%.0f flt=%.0f vwap=%.0f h52=%.0f  penalties=%v nulls=%v\n",
				c.symbol, c.score.Total, c.score.Sub.VolAccel, c.score.Sub.RVol, c.score.Sub.Breakout,
				c.score.Sub.Catalyst, c.score.Sub.Float, c.score.Sub.VWAP, c.score.Sub.High52w,
				c.score.Penalties, c.score.NullInputs)
		}
	}
}

func toComputeBars(bs []bar) []compute.Bar {
	out := make([]compute.Bar, len(bs))
	for i, b := range bs {
		out[i] = compute.Bar{Open: b.open, High: b.high, Low: b.low, Close: b.close, Volume: b.vol}
	}
	return out
}

// onlyMarketCapMissing reports whether the sole reason a symbol failed is that
// no market cap was available.
func onlyMarketCapMissing(failures []string) bool {
	if len(failures) == 0 {
		return false
	}
	for _, f := range failures {
		if f != momentum.GateMarketCapUnavailable && f != momentum.GateMarketCapNull {
			return false
		}
	}
	return true
}

// loadMetric returns the latest value per symbol for one equity_fundamentals
// metric, restricted to the pilot draw.
func loadMetric(ctx context.Context, pool *pgxpool.Pool, metric string) map[string]float64 {
	out := map[string]float64{}
	rows, err := pool.Query(ctx, `
SELECT DISTINCT ON (f.symbol) f.symbol, f.value
FROM equity_fundamentals f
JOIN universe_symbols u ON u.symbol = f.symbol AND u.backfill_selected
WHERE f.metric = $1 AND f.value IS NOT NULL
ORDER BY f.symbol, f.ts DESC`, metric)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load %s: %v\n", metric, err)
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
