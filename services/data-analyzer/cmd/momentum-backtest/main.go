// Command momentum-backtest is Step 7: the go/no-go gate.
//
// Runs §3's feature engine and §4's scorer over the FULL backfill history rather
// than just today, computes §6's forward labels for every gated candidate, and
// reports score-decile hit rates against the base rate.
//
// §6 states the criterion plainly: "A score is only useful if high scores beat
// that base rate. Report score-decile hit rates against it. If they don't
// separate, the scoring weights are wrong and should be revised before anything
// is built on top of them."
//
// Read-only with respect to the pilot tables: it prints a report and writes
// nothing, so a run over a partially-backfilled universe cannot be mistaken
// later for the pilot's official record.
//
//	DATABASE_URL=... go run ./cmd/momentum-backtest -source tiingo
package main

import (
	"context"
	"flag"
	"fmt"
	"math"
	"os"
	"sort"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/konsbe/trading-agent/services/data-analyzer/internal/compute"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/momentum"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/reportscope"
)

type row struct {
	symbol string
	date   string
	score  momentum.Score
	label  momentum.Labels
	bucket momentum.Bucket

	// barIndex is the row's position in the symbol's own bar series. Episode
	// grouping needs a gap measured in TRADING SESSIONS, and a calendar date
	// cannot supply that: weekends, holidays and halts would all read as
	// sessions the symbol did not trade.
	barIndex int

	// feat is the full feature vector, carried so the wide dump can emit the
	// raw inputs behind each sub-score without recomputing them.
	feat momentum.Features
}

func main() {
	source := flag.String("source", "tiingo", "equity_ohlcv.source to read")
	interval := flag.String("interval", "1Day", "equity_ohlcv.interval to read")
	horizon := flag.Int("horizon", momentum.DefaultHorizon, "§6 forward window H, in trading days")
	threshold := flag.Float64("threshold", 100, "primary hit threshold in percent")
	minBars := flag.Int("min-bars", 252, "§3.1 history minimum; bars before this index are not scored")
	dump := flag.String("dump-candidates", "", "write evaluable candidates as CSV to this path (symbol,date,score,bucket,fwd_max_gain_pct,hit100)")
	// DIAGNOSTIC ONLY. Re-ranks the same candidates using a single component's
	// sub-score instead of the total, to ask whether isolating the one proven
	// component sharpens separation or whether the unresolved components are not
	// diluting much. This is NOT a weight proposal: adopting a single-component
	// score chosen on the same 218 candidates that identified it would be a third
	// round of in-sample fitting.
	isolate := flag.String("isolate", "", "diagnostic: rank by one component only (rvol|vol_accel|float|vwap|breakout|high52w)")
	dumpWide := flag.String("dump-full", "", "write every gate-passing complete-label candidate as CSV with sub-scores, features and the episode flag")
	episodeGap := flag.Int("episode-gap", 5, "sessions without a gate pass that start a new episode; matches the alert cooldown")
	gateVersion := flag.Int("gate-version", 1, "§3.2 market-cap definition: 1 = today's cap (LOOKAHEAD, ablation only), 2 = point-in-time")
	scope := flag.String("scope", "eligible", "symbol scope: eligible (full §3.1 universe) or pilot (the frozen 450, in-sample for v2)")
	flag.Parse()

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "db connect:", err)
		os.Exit(1)
	}
	defer pool.Close()

	sc, err := reportscope.Parse(*scope)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	bySymbol, err := loadBars(ctx, pool, *interval, *source, sc)
	if err != nil {
		fmt.Fprintln(os.Stderr, "load bars:", err)
		os.Exit(1)
	}

	// DENOMINATOR GUARD. Runs before any scoring, so a mis-scoped run dies
	// instead of publishing a confident number about a fraction of the data.
	expected, err := sc.ExpectedBarSymbols(ctx, pool, *interval, *source)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	loadedSet := map[string]struct{}{}
	for sym := range bySymbol {
		loadedSet[sym] = struct{}{}
	}
	if err := reportscope.VerifySet(sc, expected, loadedSet); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	totalBars, scorable := 0, 0
	for _, series := range bySymbol {
		totalBars += len(series)
		if len(series) > *minBars {
			scorable++
		}
	}
	denom := reportscope.Denominators{
		Scope: sc, Expected: len(expected), Loaded: len(bySymbol),
		Scored: scorable, Bars: totalBars,
	}
	if len(bySymbol) == 0 {
		fmt.Printf("no bars for source=%s interval=%s\n", *source, *interval)
		return
	}
	marketCaps := loadMetric(ctx, pool, "market_cap", sc)
	sharesOut := loadMetric(ctx, pool, "shares_outstanding", sc)
	var sharesPIT map[string]*pitSeries
	if *gateVersion == 2 {
		sharesPIT = loadSharesPIT(ctx, pool)
		fmt.Printf("  point-in-time share series: %d symbols\n", len(sharesPIT))
	}
	denom.Print()
	reportscope.ReportMetricCoverage("market_cap", len(marketCaps), denom.Loaded)
	reportscope.ReportMetricCoverage("shares_outstanding", len(sharesOut), denom.Loaded)

	fcfg := momentum.DefaultConfig()
	gcfg := momentum.DefaultGateConfig()
	gcfg.MinBars = *minBars
	gcfg.Version = momentum.GateVersion(*gateVersion)

	var rows []row
	stats := momentum.NewGateStats()
	var evaluated, incomplete int

	for sym, bs := range bySymbol {
		cb := bs
		// Market cap and shares outstanding are point-in-time TODAY, not as of
		// each historical bar — free sources do not provide a history of either.
		//
		// This is a real limitation and it is applied consistently rather than
		// hidden: the §3.2 market-cap gate is evaluated against today's value for
		// every historical row. It biases toward today's survivors and today's
		// size, which compounds the survivorship bias §6 already requires
		// documenting. It is recorded in the report rather than worked around,
		// because the alternative — skipping the gate historically — would change
		// which rows are candidates and make the base rate incomparable to live
		// scanning.
		gi := momentum.GateInput{}
		if mc, ok := marketCaps[sym]; ok && mc > 0 {
			gi.MarketCap = &mc
		} else if so, ok := sharesOut[sym]; ok && so > 0 {
			gi.MarketCapIsProxy = true
			_ = so // filled per-bar below, since the proxy needs close[t]
		}
		si := momentum.ScoreInput{}
		if so, ok := sharesOut[sym]; ok && so > 0 {
			shares := so
			si.FloatSharesEst = &shares
		}

		for i := *minBars; i < len(cb); i++ {
			f := momentum.ComputeAt(cb, i, fcfg)
			if f.Close == nil {
				continue
			}

			g := gi
			if g.MarketCap == nil && g.MarketCapIsProxy {
				est := sharesOut[sym] * *f.Close
				g.MarketCap = &est
			}
			if *gateVersion == 2 {
				// raw_close[t] x shares(filed <= t). BOTH factors unadjusted:
				// an adjusted price against an unadjusted share count is wrong
				// by the cumulative split factor, 10-100x for reverse-split
				// penny names, which is enough to cross a band edge.
				if ps := sharesPIT[sym]; ps != nil && cb[i].RawClose != nil {
					if sh, ok := ps.asOf(cb[i].TS.Format("2006-01-02")); ok {
						pit := *cb[i].RawClose * sh
						g.MarketCapPIT = &pit
						g.MarketCapPITMultiClass = ps.multiClass
					}
				}
			}

			res := momentum.EvaluateGates(&f, g, gcfg)
			stats.Add(res)
			evaluated++
			if !res.Passed {
				continue
			}
			s, ok := momentum.ScoreCandidate(&f, si, res)
			if !ok {
				continue
			}
			l := momentum.LabelAt(cb, i, *horizon)
			if l == nil {
				continue
			}
			if !l.Complete {
				// §6: excluded from evaluation. Counted so the exclusion is
				// visible rather than silently shrinking the sample.
				incomplete++
				continue
			}
			if *isolate != "" {
				// Replace the total with the isolated component's own points,
				// leaving every sub-score intact so the report still decomposes.
				v, ok := componentValue(s, *isolate)
				if !ok {
					fmt.Fprintf(os.Stderr, "unknown component %q\n", *isolate)
					os.Exit(2)
				}
				s.Raw = v
				s.Total = int(math.Round(v))
			}
			rows = append(rows, row{
				symbol: sym, date: bs[i].TS.Format("2006-01-02"),
				score: s, label: *l, bucket: res.Bucket,
				barIndex: i, feat: f,
			})
		}
	}

	if *dumpWide != "" {
		if err := dumpFull(*dumpWide, rows, *episodeGap); err != nil {
			fmt.Fprintln(os.Stderr, "dump-full:", err)
			os.Exit(1)
		}
		fmt.Printf("wrote %d candidates (episode gap %d sessions) to %s\n", len(rows), *episodeGap, *dumpWide)
	}

	if *dump != "" {
		if err := dumpCandidates(*dump, rows, *threshold); err != nil {
			fmt.Fprintln(os.Stderr, "dump:", err)
		} else {
			fmt.Printf("wrote %d candidates to %s\n", len(rows), *dump)
		}
	}

	if *isolate != "" {
		fmt.Printf("\n  ⚑ DIAGNOSTIC MODE: candidates ranked by %q alone, not by momentum_score_100.\n", *isolate)
		fmt.Println("    Not a weight proposal — selecting a single-component score on the same")
		fmt.Println("    candidates that identified that component would be in-sample fitting again.")
	}

	report(rows, stats, evaluated, incomplete, *horizon, *threshold, len(bySymbol))
}

// componentValue extracts one sub-score by name, for -isolate.
func componentValue(s momentum.Score, name string) (float64, bool) {
	switch name {
	case "rvol":
		return s.Sub.RVol, true
	case "vol_accel":
		return s.Sub.VolAccel, true
	case "float":
		return s.Sub.Float, true
	case "vwap":
		return s.Sub.VWAP, true
	case "breakout":
		return s.Sub.Breakout, true
	case "high52w":
		return s.Sub.High52w, true
	case "catalyst":
		return s.Sub.Catalyst, true
	}
	return 0, false
}

func report(rows []row, stats *momentum.GateStats, evaluated, incomplete, horizon int, threshold float64, symbols int) {
	fmt.Println("═══ Step 7: momentum score validation ═══")
	fmt.Printf("  weighting: §4 v2 — rvol=%d vol_accel=%d catalyst=%d float=%d vwap=%d breakout=%d high52w=%d\n",
		momentum.WeightRVol, momentum.WeightVolAccel, momentum.WeightCatalyst,
		momentum.WeightFloat, momentum.WeightVWAP, momentum.WeightBreakout, momentum.WeightHigh52w)
	fmt.Printf("  allocated=%d reserved=%d (max attainable %d; %d with catalyst null)\n",
		momentum.WeightAllocated, momentum.WeightReserved, momentum.WeightAllocated,
		momentum.WeightAllocated-momentum.WeightCatalyst)
	fmt.Println()
	fmt.Println("  ⚠ IN-SAMPLE. The v2 weights were chosen using these same candidates, so any")
	fmt.Println("    improvement below is a consistency check on the logic, NOT evidence the")
	fmt.Println("    revision generalises — selecting components on this data then scoring it")
	fmt.Println("    again shows improvement almost by construction. Out-of-sample confirmation")
	fmt.Println("    needs the full-universe backfill or fresh forward sessions this 450-symbol")
	fmt.Println("    set did not inform.")
	fmt.Println()
	fmt.Printf("  symbols:                 %d\n", symbols)
	fmt.Printf("  symbol-days evaluated:   %d\n", evaluated)
	fmt.Printf("  passed §3.2 gates:       %d\n", stats.Passed)
	fmt.Printf("  excluded, label incomplete (inside the last %d sessions): %d\n", horizon, incomplete)
	fmt.Printf("  evaluable candidates:    %d\n", len(rows))
	fmt.Printf("  top gate rejections:     %v\n", stats.TopFailures(6))

	if len(rows) == 0 {
		fmt.Println("\n  NO EVALUABLE CANDIDATES — the base rate is undefined.")
		fmt.Println("  This is a result, not a crash: with this universe and these thresholds the")
		fmt.Println("  scanner produced nothing to evaluate, so the scoring weights cannot be")
		fmt.Println("  validated or refuted yet. Widening the universe is the next lever, not")
		fmt.Println("  relaxing §3.2 — the gates are the strategy.")
		return
	}

	// ── Base rate ──
	hits := 0
	for _, r := range rows {
		if r.label.FwdMaxGainPct >= threshold {
			hits++
		}
	}
	base := 100 * float64(hits) / float64(len(rows))
	fmt.Printf("\n  BASE RATE: %d/%d candidates reached +%.0f%% within %d sessions = %.2f%%\n",
		hits, len(rows), threshold, horizon, base)

	// ── Score deciles ──
	//
	// §6's actual test: do high scores beat the base rate? Sorted ascending and
	// cut into ten equal-count buckets, so each decile is comparable in size.
	sorted := make([]row, len(rows))
	copy(sorted, rows)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].score.Total < sorted[j].score.Total })

	fmt.Printf("\n  %-8s %-9s %-7s %-9s %-9s %-10s %s\n",
		"decile", "score", "n", "hit%", "lift", "med gain%", "med drawdown%")
	for d := 0; d < 10; d++ {
		lo := d * len(sorted) / 10
		hi := (d + 1) * len(sorted) / 10
		if hi > len(sorted) {
			hi = len(sorted)
		}
		if lo >= hi {
			continue
		}
		seg := sorted[lo:hi]
		h := 0
		gains := make([]float64, 0, len(seg))
		dds := make([]float64, 0, len(seg))
		for _, r := range seg {
			if r.label.FwdMaxGainPct >= threshold {
				h++
			}
			gains = append(gains, r.label.FwdMaxGainPct)
			dds = append(dds, r.label.FwdMaxDrawdownPct)
		}
		hitPct := 100 * float64(h) / float64(len(seg))
		lift := math.NaN()
		if base > 0 {
			lift = hitPct / base
		}
		fmt.Printf("  %-8d %3d-%-5d %-7d %-9.2f %-9s %-10.1f %.1f\n",
			d+1, seg[0].score.Total, seg[len(seg)-1].score.Total, len(seg), hitPct,
			fmtLift(lift), median(gains), median(dds))
	}

	// ── Verdict ──
	//
	// Stated mechanically rather than narratively: §6 says if the deciles do not
	// separate, the weights are wrong. Comparing the top and bottom thirds is a
	// coarser, more robust read than the top decile alone, which on a small
	// sample is mostly noise.
	third := len(sorted) / 3
	if third > 0 {
		lowHits, highHits := 0, 0
		for _, r := range sorted[:third] {
			if r.label.FwdMaxGainPct >= threshold {
				lowHits++
			}
		}
		for _, r := range sorted[len(sorted)-third:] {
			if r.label.FwdMaxGainPct >= threshold {
				highHits++
			}
		}
		lowPct := 100 * float64(lowHits) / float64(third)
		highPct := 100 * float64(highHits) / float64(third)
		fmt.Printf("\n  bottom third (n=%d): %.2f%%   top third (n=%d): %.2f%%\n", third, lowPct, third, highPct)
		// A direction alone is not a result. Two-proportion z-test on the thirds:
		// at n=72 per third with ~13 hits each, a few points of difference is
		// entirely compatible with chance. An earlier version called a 2.78-point
		// gap "SEPARATES", which claimed far more than the data supports.
		z, pval := twoProp(lowPct/100, highPct/100, third)
		fmt.Printf("  difference: %+.2f points (%.2fx), z=%+.2f, p=%.3f\n",
			highPct-lowPct, highPct/math.Max(lowPct, 1e-9), z, pval)
		switch {
		case len(rows) < 100:
			fmt.Printf("  VERDICT: INCONCLUSIVE — %d candidates is too few to separate signal from noise.\n", len(rows))
			fmt.Println("  Not a refutation of the weights; an insufficient sample to test them.")
		case pval >= 0.05 && highPct > lowPct:
			fmt.Println("  VERDICT: INCONCLUSIVE — the score points the right way, but the gap is not")
			fmt.Println("  distinguishable from noise at this sample size. Direction is not evidence.")
		case pval >= 0.05:
			fmt.Println("  VERDICT: INCONCLUSIVE — no measurable relationship either way at this size.")
		case highPct > lowPct:
			fmt.Printf("  VERDICT: the score SEPARATES — top third beats bottom by %.2f points (p=%.4f).\n",
				highPct-lowPct, pval)
		default:
			fmt.Println("  VERDICT: the score separates BACKWARDS, significantly. Per §6 the weights are")
			fmt.Println("  wrong and must be revised before anything is built on top of them.")
		}
	}

	// ── Bucket split ──
	fmt.Println("\n  by bucket:")
	for _, b := range []momentum.Bucket{momentum.BucketMarket, momentum.BucketPenny} {
		var n, h int
		for _, r := range rows {
			if r.bucket != b {
				continue
			}
			n++
			if r.label.FwdMaxGainPct >= threshold {
				h++
			}
		}
		if n == 0 {
			fmt.Printf("    %-7s no evaluable candidates\n", b)
			continue
		}
		fmt.Printf("    %-7s n=%-6d hit%%=%.2f\n", b, n, 100*float64(h)/float64(n))
	}

	// ── Within-bucket deciles: the confound check ──
	//
	// The bucket effect is far stronger than the score effect here (penny
	// candidates hit +100% roughly 3x as often as market ones), so a score that
	// merely correlates with bucket membership would show up as score
	// separation — or, if it correlates negatively, as score INVERSION — without
	// any of it being about the score.
	//
	// That is Simpson's paradox, and on a sample this size it is the first thing
	// to rule out before concluding anything about the weights. Quintiles rather
	// than deciles inside each bucket, because the penny bucket has too few rows
	// to cut ten ways.
	fmt.Println("\n  within-bucket quintiles (confound check — does the score work INSIDE a bucket?):")
	for _, b := range []momentum.Bucket{momentum.BucketMarket, momentum.BucketPenny} {
		var seg []row
		for _, r := range rows {
			if r.bucket == b {
				seg = append(seg, r)
			}
		}
		if len(seg) < 10 {
			fmt.Printf("    %-7s n=%d — too few to stratify\n", b, len(seg))
			continue
		}
		sort.Slice(seg, func(i, j int) bool { return seg[i].score.Total < seg[j].score.Total })
		fmt.Printf("    %s (n=%d):\n", b, len(seg))
		for q := 0; q < 5; q++ {
			lo, hi := q*len(seg)/5, (q+1)*len(seg)/5
			if lo >= hi {
				continue
			}
			part := seg[lo:hi]
			h := 0
			dds := make([]float64, 0, len(part))
			for _, r := range part {
				if r.label.FwdMaxGainPct >= threshold {
					h++
				}
				dds = append(dds, r.label.FwdMaxDrawdownPct)
			}
			fmt.Printf("      Q%d score %3d-%-3d n=%-4d hit%%=%-7.2f med drawdown%%=%.1f\n",
				q+1, part[0].score.Total, part[len(part)-1].score.Total, len(part),
				100*float64(h)/float64(len(part)), median(dds))
		}
	}

	// ── Drawdown monotonicity ──
	//
	// Reported separately because hit rate is not the only thing a score can be
	// good at. A signal that does not improve the chance of a +100% move but does
	// reliably reduce how far underwater the position goes is still information,
	// and §6 only tests the former.
	{
		sortedByScore := make([]row, len(rows))
		copy(sortedByScore, rows)
		sort.Slice(sortedByScore, func(i, j int) bool {
			return sortedByScore[i].score.Total < sortedByScore[j].score.Total
		})
		half := len(sortedByScore) / 2
		lowDD := make([]float64, 0, half)
		highDD := make([]float64, 0, half)
		for _, r := range sortedByScore[:half] {
			lowDD = append(lowDD, r.label.FwdMaxDrawdownPct)
		}
		for _, r := range sortedByScore[len(sortedByScore)-half:] {
			highDD = append(highDD, r.label.FwdMaxDrawdownPct)
		}
		lo, hi := median(lowDD), median(highDD)
		fmt.Printf("\n  drawdown: low-score half median %.1f%% vs high-score half median %.1f%%",
			lo, hi)
		if hi < lo {
			fmt.Printf("  -> high scores took %.1f points LESS drawdown\n", lo-hi)
		} else {
			fmt.Println("  -> no drawdown advantage")
		}
	}

	// ── All thresholds ──
	fmt.Println("\n  hit rates at every §6 threshold:")
	for _, th := range momentum.HitThresholds {
		h := 0
		for _, r := range rows {
			if r.label.FwdMaxGainPct >= th {
				h++
			}
		}
		fmt.Printf("    +%-6.0f%% %d/%d = %.2f%%\n", th, h, len(rows), 100*float64(h)/float64(len(rows)))
	}

	// ── Per-component diagnostic ──
	//
	// §6's instruction when the deciles fail is "the weights are wrong and should
	// be revised", which needs to know WHICH weight and in which direction. A
	// total score cannot answer that; each component's own relationship to the
	// outcome can.
	//
	// Read as: for each sub-score, the +100% hit rate among candidates scoring
	// LOW on it versus HIGH on it. A component carrying real signal should show
	// high > low. One showing the reverse is actively costing accuracy at its
	// current weight, and one showing no difference is contributing noise in
	// proportion to its weight.
	fmt.Println("\n  per-component signal (hit% among low vs high scorers on each component):")
	fmt.Printf("    %-12s %-6s %-14s %-14s %s\n", "component", "weight", "below median", "at/above median", "reading")
	comps := []struct {
		name   string
		weight int
		get    func(row) float64
	}{
		{"vol_accel", momentum.WeightVolAccel, func(r row) float64 { return r.score.Sub.VolAccel }},
		{"rvol", momentum.WeightRVol, func(r row) float64 { return r.score.Sub.RVol }},
		{"breakout", momentum.WeightBreakout, func(r row) float64 { return r.score.Sub.Breakout }},
		{"float", momentum.WeightFloat, func(r row) float64 { return r.score.Sub.Float }},
		{"vwap", momentum.WeightVWAP, func(r row) float64 { return r.score.Sub.VWAP }},
		{"high52w", momentum.WeightHigh52w, func(r row) float64 { return r.score.Sub.High52w }},
	}
	for _, c := range comps {
		seg := make([]row, len(rows))
		copy(seg, rows)
		sort.Slice(seg, func(i, j int) bool { return c.get(seg[i]) < c.get(seg[j]) })
		half := len(seg) / 2
		if half == 0 {
			continue
		}

		// A component every candidate scores identically on — which is what a
		// zeroed weight produces — has no low half and no high half. Splitting it
		// anyway compares two arbitrary slices of tied values and prints a
		// difference that is an artifact of sort order. An earlier version of this
		// report did exactly that and showed breakout, high52w and vwap with
		// identical "INVERTED" readings: a reporting bug, not a finding.
		if c.get(seg[0]) == c.get(seg[len(seg)-1]) {
			fmt.Printf("    %-12s %-6d %-14s %-14s no variance — every candidate scores the same\n",
				c.name, c.weight, "—", "—")
			continue
		}
		// Split at the MEDIAN VALUE with ties kept whole, not at the midpoint of
		// the sorted slice. A component with few distinct values — vwap is
		// binary, float has five steps — otherwise has its boundary drawn through
		// a group of identical scores, and which side each tied candidate lands
		// on is decided by sort order rather than by anything real.
		//
		// This mattered: under equal-halves, vwap read "INVERTED" purely because
		// the cut fell inside its tie group.
		cut := c.get(seg[half])
		var lowG, highG []row
		for _, r := range seg {
			if c.get(r) < cut {
				lowG = append(lowG, r)
			} else {
				highG = append(highG, r)
			}
		}
		if len(lowG) == 0 || len(highG) == 0 {
			// Every candidate on one side of the median: no contrast available.
			fmt.Printf("    %-12s %-6d %-14s %-14s no contrast — median splits nothing\n",
				c.name, c.weight, "—", "—")
			continue
		}
		countHits := func(g []row) float64 {
			h := 0
			for _, r := range g {
				if r.label.FwdMaxGainPct >= threshold {
					h++
				}
			}
			return 100 * float64(h) / float64(len(g))
		}
		lo, hi := countHits(lowG), countHits(highG)

		// Significance, so a reading is not asserted on a direction alone. Uses
		// the smaller group's size, which is the conservative choice when the
		// split is uneven.
		nEff := len(lowG)
		if len(highG) < nEff {
			nEff = len(highG)
		}
		_, pval := twoProp(lo/100, hi/100, nEff)
		reading := "no signal"
		switch {
		case pval >= 0.05:
			reading = fmt.Sprintf("not distinguishable (p=%.2f)", pval)
		case hi > lo:
			reading = fmt.Sprintf("PREDICTIVE (p=%.4f)", pval)
		default:
			reading = fmt.Sprintf("INVERTED (p=%.4f)", pval)
		}
		fmt.Printf("    %-12s %-6d %-14s %-14s %s\n", c.name, c.weight,
			fmt.Sprintf("%.2f (n=%d)", lo, len(lowG)),
			fmt.Sprintf("%.2f (n=%d)", hi, len(highG)), reading)
	}

	fmt.Println("\n  LIMITATIONS, per §6 — these are not caveats to skim:")
	fmt.Println("   • Survivorship bias is PRESENT. The universe was pulled today, so delisted")
	fmt.Println("     tickers are absent, and they are disproportionately failures. Hit rates here")
	fmt.Println("     are therefore optimistic by an unmeasured amount. This backtest is not unbiased.")
	fmt.Println("   • market_cap and shares_outstanding are point-in-time TODAY, applied to every")
	fmt.Println("     historical row. Both the §3.2 gate and the float sub-score are biased toward")
	fmt.Println("     today's size rather than the size at the time.")
	fmt.Println("   • catalyst_tier is null throughout (§3.11 is Step 8), so 15 of the 100 points")
	fmt.Println("     are unavailable and the achievable score ceiling is 85.")
	fmt.Println("   • The pilot is 450 stratified symbols, not the full 4,975-symbol universe.")
}

func fmtLift(l float64) string {
	if math.IsNaN(l) || math.IsInf(l, 0) {
		return "—"
	}
	return fmt.Sprintf("%.2fx", l)
}

func median(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	s := make([]float64, len(xs))
	copy(s, xs)
	sort.Float64s(s)
	m := len(s) / 2
	if len(s)%2 == 1 {
		return s[m]
	}
	return (s[m-1] + s[m]) / 2
}

// loadBars reads the bar history for the requested scope.
//
// The scope USED to be hardcoded to `u.backfill_selected`, which silently
// limited every backtest to the 450-symbol pilot. That was correct while the
// pilot was the only backfilled data and invisible once it was not: after the
// full-universe backfill the query still returned 450 symbols and the report
// still looked complete, just describing a twentieth of the data.
//
// The clause now comes from internal/reportscope, and the caller verifies the
// loaded symbol set against the database before scoring anything. See that
// package for why printing the denominator is not enough on its own.
func loadBars(ctx context.Context, pool *pgxpool.Pool, interval, source string, scope reportscope.Scope) (map[string][]compute.Bar, error) {
	rows, err := pool.Query(ctx, `
SELECT o.symbol, o.ts, o.open, o.high, o.low, o.close, o.volume, o.raw_close
FROM equity_ohlcv o
`+scope.JoinOn("o")+`
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
		if err := rows.Scan(&sym, &b.TS, &b.Open, &b.High, &b.Low, &b.Close, &b.Volume, &b.RawClose); err != nil {
			return nil, err
		}
		out[sym] = append(out[sym], b)
	}
	return out, rows.Err()
}

// loadSharesPIT reads the point-in-time share series per symbol, oldest first.
//
// Returned as parallel slices of (filed_date, shares) so an as-of lookup is a
// binary search. Keyed on FILED date, never period_end: the period end precedes
// the filing by weeks, and joining on it would use a share count before it was
// public — swapping one lookahead for a subtler one.
func loadSharesPIT(ctx context.Context, pool *pgxpool.Pool) map[string]*pitSeries {
	out := map[string]*pitSeries{}
	rows, err := pool.Query(ctx, `
SELECT symbol, filed_date, shares, multi_class
FROM shares_outstanding_pit
ORDER BY symbol, filed_date`)
	if err != nil {
		fmt.Fprintln(os.Stderr, "load shares_outstanding_pit:", err)
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var sym string
		var d time.Time
		var sh float64
		var mc bool
		if err := rows.Scan(&sym, &d, &sh, &mc); err != nil {
			return out
		}
		ps := out[sym]
		if ps == nil {
			ps = &pitSeries{}
			out[sym] = ps
		}
		ps.filed = append(ps.filed, d.Format("2006-01-02"))
		ps.shares = append(ps.shares, sh)
		ps.multiClass = ps.multiClass || mc
	}
	return out
}

type pitSeries struct {
	filed      []string
	shares     []float64
	multiClass bool
}

// asOf returns the most recent share count FILED on or before date.
//
// Returns false when no filing exists yet. That is not a gap to be patched: a
// symbol-day before the company's first filing is UNMEASURABLE, and gate v2
// rejects it with market_cap_pit_unavailable rather than substituting today's
// value, which would restore the leak exactly where it is largest.
func (p *pitSeries) asOf(date string) (float64, bool) {
	i := sort.SearchStrings(p.filed, date)
	// SearchStrings gives the first index >= date; step back unless it is exact.
	if i < len(p.filed) && p.filed[i] == date {
		return p.shares[i], true
	}
	if i == 0 {
		return 0, false
	}
	return p.shares[i-1], true
}

// loadMetric reads the most recent value of one fundamental metric per symbol.
//
// Scoped the same way as loadBars, from the same Scope value, and that sharing
// is the point. This query carried its own hardcoded `backfill_selected` join,
// which was the more damaging of the two: with the bars widened but the metrics
// still pilot-only, every non-pilot symbol-day failed the §3.2 gate with
// `market_cap_unavailable` and was counted as an ordinary gate rejection. The
// backtest reported 4,971 symbols while scoring 450, and nothing errored.
func loadMetric(ctx context.Context, pool *pgxpool.Pool, metric string, scope reportscope.Scope) map[string]float64 {
	out := map[string]float64{}
	rows, err := pool.Query(ctx, `
SELECT DISTINCT ON (f.symbol) f.symbol, f.value
FROM equity_fundamentals f
`+scope.JoinOn("f")+`
WHERE f.metric = $1 AND f.value IS NOT NULL AND f.value > 0
-- Source rank for determinism: the NOT NULL filter stops this picking a
-- NULL, but two sources can report the same metric at the same ts.
ORDER BY f.symbol, f.ts DESC, fundamental_source_rank(f.source) DESC`, metric)
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

// twoProp returns the z statistic and two-sided p-value for a difference of two
// proportions measured on equal-sized samples.
//
// Exists so the verdict reports whether a gap is distinguishable from noise
// rather than only which way it points.
func twoProp(p1, p2 float64, n int) (float64, float64) {
	if n <= 0 {
		return 0, 1
	}
	pbar := (p1 + p2) / 2
	if pbar <= 0 || pbar >= 1 {
		return 0, 1
	}
	se := math.Sqrt(2 * pbar * (1 - pbar) / float64(n))
	if se == 0 {
		return 0, 1
	}
	z := (p2 - p1) / se
	return z, math.Erfc(math.Abs(z) / math.Sqrt2)
}

// dumpCandidates writes the evaluable candidate set so downstream work — §3.11's
// catalyst evaluation in particular — can be tested against exactly the same
// rows this report was computed on, rather than a re-derived approximation.
func dumpCandidates(path string, rows []row, threshold float64) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := fmt.Fprintln(f, "symbol,date,score,bucket,fwd_max_gain_pct,hit100"); err != nil {
		return err
	}
	sorted := make([]row, len(rows))
	copy(sorted, rows)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].date != sorted[j].date {
			return sorted[i].date < sorted[j].date
		}
		return sorted[i].symbol < sorted[j].symbol
	})
	for _, r := range sorted {
		if _, err := fmt.Fprintf(f, "%s,%s,%d,%s,%.4f,%t\n",
			r.symbol, r.date, r.score.Total, r.bucket, r.label.FwdMaxGainPct,
			r.label.FwdMaxGainPct >= threshold); err != nil {
			return err
		}
	}
	return nil
}
