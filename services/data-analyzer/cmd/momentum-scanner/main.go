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
// Gate version: v2 (point-in-time market cap, raw_close[t] x shares filed <= t)
// since 2026-09-26, with a 15-month maximum filing age. Before that the live
// scan ran v1 (today's Finnhub cap) because DefaultGateConfig's zero Version
// behaves as v1 and this command never opted in. See Phase 1 §3.2.
//
//	DATABASE_URL=... go run ./cmd/momentum-scanner [-gate-version 2] [-pit-max-age-months 15]
//
// Flags fall back to MOMENTUM_GATE_VERSION / MOMENTUM_PIT_MAX_FILING_AGE_MONTHS
// (momentum-daily runs this with no arguments); an explicit flag wins.
//
// -session YYYY-MM-DD scans a past session point-in-time (momentum-daily's
// catch-up): bars, fundamentals and catalysts stamped before the next UTC
// midnight, share filings filed by the session. Universe eligibility is read as
// it is now. Without the flag every query is the unattended one, unchanged.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/konsbe/trading-agent/services/data-analyzer/internal/compute"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/momentum"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/store"
)

func main() {
	source := flag.String("source", "tiingo", "equity_ohlcv.source to read")
	interval := flag.String("interval", "1Day", "equity_ohlcv.interval to read")
	dryRun := flag.Bool("dry-run", false, "compute and report without writing")
	gateVersion := flag.Int("gate-version", envInt("MOMENTUM_GATE_VERSION", int(momentum.GateV2)),
		"§3.2 market-cap definition: 2 = point-in-time (default), 1 = today's Finnhub cap. Env MOMENTUM_GATE_VERSION")
	pitMaxAge := flag.Int("pit-max-age-months", envInt("MOMENTUM_PIT_MAX_FILING_AGE_MONTHS", momentum.DefaultGateConfig().PITMaxFilingAgeMonths),
		"gate v2: a share count filed more than N months before the session counts as unavailable (market_cap_pit_unavailable); 0 = no limit. Env MOMENTUM_PIT_MAX_FILING_AGE_MONTHS")
	explain := flag.String("explain", "", "comma-separated symbols whose gate verdict and market-cap inputs are printed")
	sessionFlag := flag.String("session", "", "scan this past NYSE session (YYYY-MM-DD), point-in-time: bars, share filings, fundamentals and catalysts as of that date. Empty = the newest session in the bars")
	flag.Parse()
	asOf, err := store.ParseSessionFlag(*sessionFlag)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	explainSet := map[string]bool{}
	for _, s := range strings.Split(*explain, ",") {
		if s = strings.TrimSpace(strings.ToUpper(s)); s != "" {
			explainSet[s] = true
		}
	}
	if *gateVersion != int(momentum.GateV1) && *gateVersion != int(momentum.GateV2) {
		fmt.Fprintf(os.Stderr, "-gate-version must be 1 or 2, got %d\n", *gateVersion)
		os.Exit(2)
	}
	if *pitMaxAge < 0 {
		fmt.Fprintf(os.Stderr, "-pit-max-age-months must be >= 0, got %d\n", *pitMaxAge)
		os.Exit(2)
	}

	fcfg := momentum.DefaultConfig()
	gcfg := momentum.DefaultGateConfig()
	gcfg.Version = momentum.GateVersion(*gateVersion)
	gcfg.PITMaxFilingAgeMonths = *pitMaxAge
	gateDesc := describeGate(gcfg)
	fmt.Println("gate:", gateDesc)
	if asOf != nil {
		fmt.Println("point-in-time session:", asOf.Format(time.DateOnly))
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "db connect:", err)
		os.Exit(1)
	}
	defer pool.Close()

	bars, err := loadBars(ctx, pool, *interval, *source, asOf)
	if err != nil {
		fmt.Fprintln(os.Stderr, "load bars:", err)
		os.Exit(1)
	}
	if len(bars) == 0 {
		fmt.Printf("no bars for source=%s interval=%s\n", *source, *interval)
		return
	}

	marketCaps := loadMetric(ctx, pool, "market_cap", asOf)
	sharesOut := loadMetric(ctx, pool, "shares_outstanding", asOf)
	catalysts := loadCatalystTiers(ctx, pool, asOf)

	var sharesPIT map[string]*store.SharesPIT
	if gcfg.Version == momentum.GateV2 {
		// Fatal, unlike the backtest: without the series every symbol fails
		// market_cap_pit_unavailable and the scan would commit as a valid day
		// with zero candidates.
		sharesPIT, err = store.LoadSharesPIT(ctx, pool)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}

	stats := momentum.NewGateStats()

	var skipped, pitUnavailable int
	var writes []store.ScanWrite
	var latest []time.Time
	var candidates []string

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
		if gcfg.Version == momentum.GateV2 {
			// raw_close[t] x shares(filed <= t), both UNADJUSTED, exactly as the
			// backtest computes it. v2 has no proxy path, so MarketCapEst stays
			// nil and MarketCap records the PIT value — the value the gate was
			// evaluated against, which is what that column means.
			gi.SessionDate = ts
			row.MarketCap, row.MarketCapEst = nil, nil
			if raw := series[last].RawClose; raw != nil {
				if sh, filed, ok := sharesPIT[sym].AsOf(ts); ok {
					pit := *raw * sh
					gi.MarketCapPIT = &pit
					gi.MarketCapPITFiled = filed
					gi.MarketCapPITMultiClass = sharesPIT[sym].MultiClass
					row.MarketCap = &pit
				}
			}
		}
		if t, ok := catalysts[sym]; ok && t != "" {
			tier := t
			row.CatalystTier = &tier
		}

		g := momentum.EvaluateGates(&f, gi, gcfg)
		row.Gate = g
		if gcfg.Version == momentum.GateV2 && hasFailure(g, momentum.GateMarketCapPITUnavailable) {
			// Stale filing: the gate did not use the value, so it is not stored
			// as if it had.
			row.MarketCap = nil
			pitUnavailable++
		}
		stats.Add(g)

		if explainSet[sym] {
			fmt.Printf("  explain %-7s %s passed=%v failures=[%s] %s\n",
				sym, g.Bucket, g.Passed, g.FailureString(), describeCap(gi, gcfg))
			delete(explainSet, sym)
		}

		w := store.ScanWrite{Feature: row}
		latest = append(latest, ts)
		if !g.Passed {
			writes = append(writes, w)
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
		if ok {
			w.Score = &s
			candidates = append(candidates, fmt.Sprintf("  candidate %-7s %-6s score=%d/%d %s",
				sym, s.Bucket, s.Total, momentum.WeightAllocated, describeCap(gi, gcfg)))
		}
		writes = append(writes, w)
	}

	session, ok := store.ScanSession(latest)
	if !ok {
		fmt.Println("no symbol had a usable close; nothing to write")
		return
	}
	if asOf != nil && !session.Equal(*asOf) {
		fmt.Fprintf(os.Stderr, "-session %s: the bars up to that date make session %s (the requested session's bars have not landed); nothing written\n",
			asOf.Format(time.DateOnly), session.Format(time.DateOnly))
		os.Exit(1)
	}
	// One transaction for the whole scan, marker included: a scanner killed
	// part-way leaves nothing, never a partial scan that looks fresh.
	if !*dryRun {
		if err := store.WriteScan(ctx, pool, session, writes); err != nil {
			fmt.Fprintln(os.Stderr, err, "— nothing was written; the scan is all-or-nothing")
			os.Exit(1)
		}
	}
	for _, c := range candidates {
		fmt.Println(c)
	}
	for sym := range explainSet {
		fmt.Printf("  explain %-7s not scanned (no eligible bars or no usable close)\n", sym)
	}

	fmt.Println("\n§8.2 scanner:")
	fmt.Printf("  gate:                 %s\n", gateDesc)
	if gcfg.Version == momentum.GateV2 {
		fmt.Printf("  PIT share series:     %d symbols; %d scanned symbols failed market_cap_pit_unavailable\n",
			len(sharesPIT), pitUnavailable)
	}
	fmt.Printf("  symbols with bars:    %d\n", len(bars))
	fmt.Printf("  no usable close:      %d\n", skipped)
	fmt.Printf("  session:              %s\n", session.Format(time.DateOnly))
	fmt.Printf("  feature rows written: %d\n", len(writes))
	fmt.Printf("  score rows written:   %d (gate-passing only)\n", len(candidates))
	if !*dryRun {
		fmt.Printf("  committed:            one transaction, with the momentum_chain_runs marker\n")
	}
	fmt.Printf("  passed by bucket:     market=%d penny=%d\n",
		stats.PassedByBkt[momentum.BucketMarket], stats.PassedByBkt[momentum.BucketPenny])
	fmt.Printf("  gate rejections:      %v\n", stats.TopFailures(8))
	if *dryRun {
		fmt.Println("  DRY RUN — nothing written")
	}
}

func loadBars(ctx context.Context, pool *pgxpool.Pool, interval, source string, asOf *time.Time) (map[string][]compute.Bar, error) {
	args := []any{interval, source}
	cutoff := ""
	if asOf != nil {
		cutoff = " AND o.ts < $3"
		args = append(args, store.PITCutoff(*asOf))
	}
	rows, err := pool.Query(ctx, `
SELECT o.symbol, o.ts, o.open, o.high, o.low, o.close, o.volume, o.raw_close
FROM equity_ohlcv o
-- data_unavailable_reason excludes symbols whose history the provider no
-- longer serves. Their stored bars are a frozen remnant, and the feature
-- engine would compute a 20-day RVOL and a 252-bar high from it and emit a
-- candidate indistinguishable from a live one. See migration 022.
JOIN universe_symbols u ON u.symbol = o.symbol AND u.is_eligible
                       AND u.data_unavailable_reason IS NULL
WHERE o.interval = $1 AND o.source = $2 AND o.close > 0`+cutoff+`
ORDER BY o.symbol, o.ts`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string][]compute.Bar{}
	for rows.Next() {
		var sym string
		var b compute.Bar
		if err := rows.Scan(&sym, &b.TS, &b.Open, &b.High, &b.Low, &b.Close, &b.Volume, &b.RawClose); err != nil {
			return nil, err
		}
		out[sym] = append(out[sym], b)
	}
	return out, rows.Err()
}

func loadMetric(ctx context.Context, pool *pgxpool.Pool, metric string, asOf *time.Time) map[string]float64 {
	out := map[string]float64{}
	args := []any{metric}
	cutoff := ""
	if asOf != nil {
		cutoff = " AND ts < $2"
		args = append(args, store.PITCutoff(*asOf))
	}
	rows, err := pool.Query(ctx, `
SELECT DISTINCT ON (symbol) symbol, value
FROM equity_fundamentals
WHERE metric = $1 AND value IS NOT NULL AND value > 0`+cutoff+`
-- The NOT NULL filter already prevents this picking a NULL, which is what
-- saved the live scanner from the universe-loader bug. The source rank is
-- still required for DETERMINISM: two sources can both report a non-null
-- market_cap at the same ts, and without a tiebreak the gate input would
-- depend on physical row order.
ORDER BY symbol, ts DESC, fundamental_source_rank(source) DESC`, args...)
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
func loadCatalystTiers(ctx context.Context, pool *pgxpool.Pool, asOf *time.Time) map[string]string {
	out := map[string]string{}
	// The unattended scan runs the evening of the session, so its window is
	// the session and the two days before it; a past session gets that same
	// window, ending at the session's cutoff.
	window := "ts::date >= (now() - interval '2 days')::date"
	var args []any
	if asOf != nil {
		window = "ts::date >= ($1::timestamptz - interval '3 days')::date AND ts < $1"
		args = append(args, store.PITCutoff(*asOf))
	}
	rows, err := pool.Query(ctx, `
SELECT DISTINCT ON (symbol) symbol, tier
FROM catalyst_events
WHERE `+window+`
-- Tier first (§3.11 takes the window's strongest signal), then ts and
-- source so that two providers reporting the same tier resolve the same way
-- on every run. catalyst_events has 4 writers and 3 colliding groups.
ORDER BY symbol, CASE tier WHEN 'A' THEN 2 WHEN 'B' THEN 1 ELSE 0 END DESC,
         ts DESC, source`, args...)
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

// envInt is the flag default: the env value when set and numeric, else def.
// A malformed value is fatal rather than silently falling back, so a typo in
// the deployment cannot quietly change which gate runs.
func envInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s=%q is not an integer\n", key, v)
		os.Exit(2)
	}
	return n
}

func describeGate(c momentum.GateConfig) string {
	if c.Version != momentum.GateV2 {
		return "v1 (today's market cap; LOOKAHEAD in any historical use)"
	}
	if c.PITMaxFilingAgeMonths <= 0 {
		return "v2 (point-in-time market cap), max filing age: none"
	}
	return fmt.Sprintf("v2 (point-in-time market cap), max filing age: %d months", c.PITMaxFilingAgeMonths)
}

func hasFailure(g momentum.GateResult, reason string) bool {
	for _, f := range g.Failures {
		if f == reason {
			return true
		}
	}
	return false
}

// describeCap renders the market-cap input the gate used, for candidate and
// -explain lines.
func describeCap(gi momentum.GateInput, c momentum.GateConfig) string {
	if c.Version != momentum.GateV2 {
		if gi.MarketCap == nil {
			return "mcap=none"
		}
		return fmt.Sprintf("mcap=$%.0fM proxy=%v", *gi.MarketCap/1e6, gi.MarketCapIsProxy)
	}
	if gi.MarketCapPIT == nil {
		return "mcap_pit=none"
	}
	stale := ""
	if c.PITFilingTooOld(gi.SessionDate, gi.MarketCapPITFiled) {
		stale = " STALE"
	}
	return fmt.Sprintf("mcap_pit=$%.0fM filed=%s%s", *gi.MarketCapPIT/1e6, gi.MarketCapPITFiled.Format(time.DateOnly), stale)
}
