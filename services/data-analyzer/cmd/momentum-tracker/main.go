// Command momentum-tracker is Step 10: §5's daily exit evaluation.
//
// Two passes per run:
//
//  1. OPEN — every §3.2 gate pass gets a momentum_tracked row, idempotently
//     (-open-mode=gates, the default: the same criterion the bot's screener
//     alerts use, so "tracked" means "alerted"). -open-mode=score restores the
//     retired per-bucket score thresholds; Phase 1 §10.1.9's replay figures
//     were produced in that mode.
//  2. EVALUATE — every active row is checked against §5's five exit conditions
//     and closed on the first match, recording the realized outcome.
//
// §5 is explicit that the exit rules are "a starting default, not a validated
// strategy ... deliberately mechanical so the backtest can evaluate and replace
// them." Step 7 makes that more pressing rather than less: `breakout_failed`
// keys on resistance_20_at_alert, and breakout geometry measured INVERTED for
// predicting +100% moves. An exit signal and an entry signal are different
// questions — and §4.1 v2's drawdown finding suggests breakout geometry does
// carry information about how a move resolves — but the rules have not been
// validated and this command does not pretend otherwise.
//
// What it does instead is record what §5 asks for: max_gain_pct and exit_pct per
// closed row, plus EVERY condition that matched rather than only the one acted
// on, so both the choice of conditions and their ordering become measurable.
//
//	DATABASE_URL=... go run ./cmd/momentum-tracker
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/konsbe/trading-agent/services/data-analyzer/internal/compute"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/momentum"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/reportscope"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/store"
)

func main() {
	source := flag.String("source", "tiingo", "equity_ohlcv.source to read")
	interval := flag.String("interval", "1Day", "equity_ohlcv.interval to read")
	openModeFlag := flag.String("open-mode", string(openOnGates), "which candidates open a tracked row: gates (every gate pass, matches screener alerts) or score (retired per-bucket thresholds)")
	minMarket := flag.Int("min-score-market", 65, "§4.4 market-bucket alert threshold (open-mode=score only)")
	minPenny := flag.Int("min-score-penny", 72, "§4.4 penny-bucket alert threshold (open-mode=score only)")
	dryRun := flag.Bool("dry-run", false, "evaluate and report without writing")
	// Walks the stored history bar by bar, opening positions when a candidate
	// clears its threshold and evaluating exits on every subsequent session.
	//
	// This is how §5's rules stop being unmeasured. The live daily path produces
	// one session of data per day; a replay over three years of bars produces the
	// whole dataset now, using exactly the same EvaluateExit function so a replay
	// result and a live result are comparable rather than two implementations.
	//
	// In-memory and read-only: it writes nothing, so a replay cannot be confused
	// later with real tracked positions.
	replay := flag.Bool("replay", false, "replay §5 over stored history and report outcomes (writes nothing)")
	minBars := flag.Int("min-bars", 252, "§3.1 history minimum")
	scopeFlag := flag.String("scope", "eligible", "symbol scope: eligible (full §3.1 universe) or pilot (the frozen 450, in-sample for v2)")
	flag.Parse()

	mode, err := parseOpenMode(*openModeFlag)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	rule := openRule{mode: mode, minMarket: *minMarket, minPenny: *minPenny}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "db connect:", err)
		os.Exit(1)
	}
	defer pool.Close()

	sc, err := reportscope.Parse(*scopeFlag)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	bars, err := loadBars(ctx, pool, *interval, *source, sc)
	if err != nil {
		fmt.Fprintln(os.Stderr, "load bars:", err)
		os.Exit(1)
	}
	if len(bars) == 0 {
		fmt.Printf("no bars for source=%s interval=%s\n", *source, *interval)
		return
	}

	// DENOMINATOR GUARD, before any position is opened. A replay that silently
	// covers the pilot subset produces a plausible exit table over the wrong
	// population, and an unopened position leaves no trace to notice.
	expected, err := sc.ExpectedBarSymbols(ctx, pool, *interval, *source)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	loadedSet := map[string]struct{}{}
	totalBars := 0
	for sym, series := range bars {
		loadedSet[sym] = struct{}{}
		totalBars += len(series)
	}
	if err := reportscope.VerifySet(sc, expected, loadedSet); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	reportscope.Denominators{
		Scope: sc, Expected: len(expected), Loaded: len(bars),
		Scored: len(bars), Bars: totalBars,
	}.Print()

	fcfg := momentum.DefaultConfig()
	ecfg := momentum.DefaultExitConfig()

	if *replay {
		replayHistory(ctx, pool, bars, fcfg, ecfg, *minBars, rule, sc)
		return
	}

	// ── Pass 2 first, deliberately ──
	//
	// Evaluating exits BEFORE opening new positions means a symbol that alerts
	// again today is not immediately evaluated against a reference price set
	// moments ago. Opening first would give every new row one spurious session
	// of elapsed time.
	closed, advanced := evaluateExits(ctx, pool, bars, fcfg, ecfg, *dryRun)

	opened := openNewPositions(ctx, pool, bars, fcfg, rule, *dryRun, sc)

	fmt.Printf("\n§5 tracker: %d opened, %d advanced, %d closed", opened, advanced, len(closed))
	if *dryRun {
		fmt.Print("  (DRY RUN — nothing written)")
	}
	fmt.Println()

	for _, c := range closed {
		fmt.Printf("  closed %-7s %-18s exit %+7.2f%%  peak %+7.2f%%  matched=%s\n",
			c.symbol, c.decision.Reason, c.decision.ExitPct, c.decision.MaxGainPct,
			c.decision.MatchedString())
	}

	reportOutcomes(ctx, pool)
}

type closedPosition struct {
	symbol   string
	decision momentum.ExitDecision
}

func evaluateExits(
	ctx context.Context,
	pool *pgxpool.Pool,
	bars map[string][]compute.Bar,
	fcfg momentum.Config,
	ecfg momentum.ExitConfig,
	dry bool,
) ([]closedPosition, int) {
	active, err := store.ActiveTracked(ctx, pool)
	if err != nil {
		fmt.Fprintln(os.Stderr, "active tracked:", err)
		return nil, 0
	}
	var closed []closedPosition
	advanced := 0

	for _, row := range active {
		series := bars[row.Symbol]
		if len(series) == 0 {
			// No bar today. Deliberately NOT treated as a session: an unmeasurable
			// day must not consume the timeout budget, or a data outage would close
			// positions for reasons unrelated to price.
			fmt.Printf("  %-7s no bars — session skipped, state unchanged\n", row.Symbol)
			continue
		}
		step := stepExits(row, series, fcfg, ecfg)
		if step.sessions == 0 {
			// Already evaluated against every stored bar. Re-evaluating would
			// double-count a session and halve every time-based condition's
			// effective horizon.
			continue
		}
		d := step.decision
		ts := step.ts

		if len(d.Unevaluable) > 0 {
			fmt.Printf("  %-7s unevaluable conditions: %v\n", row.Symbol, d.Unevaluable)
		}
		if step.sessions > 1 {
			fmt.Printf("  %-7s caught up %d sessions through %s\n", row.Symbol, step.sessions, ts.Format(time.DateOnly))
		}

		if !d.Exit {
			if !dry {
				if err := store.AdvanceTracked(ctx, pool, row.Symbol, row.AlertedTS, ts, d); err != nil {
					fmt.Fprintln(os.Stderr, err)
					continue
				}
			}
			advanced++
			continue
		}

		if !dry {
			if err := store.CloseTracked(ctx, pool, row.Symbol, row.AlertedTS, ts, step.close, d); err != nil {
				fmt.Fprintln(os.Stderr, err)
				continue
			}
		}
		closed = append(closed, closedPosition{row.Symbol, d})
	}
	return closed, advanced
}

func openNewPositions(
	ctx context.Context,
	pool *pgxpool.Pool,
	bars map[string][]compute.Bar,
	fcfg momentum.Config,
	rule openRule,
	dry bool,
	scope reportscope.Scope,
) int {
	gcfg := momentum.DefaultGateConfig()
	marketCaps := loadMetric(ctx, pool, "market_cap", scope)
	sharesOut := loadMetric(ctx, pool, "shares_outstanding", scope)
	reportscope.ReportMetricCoverage("market_cap", len(marketCaps), len(bars))
	reportscope.ReportMetricCoverage("shares_outstanding", len(sharesOut), len(bars))

	opened := 0
	for sym, series := range bars {
		if len(series) == 0 {
			continue
		}
		last := len(series) - 1
		f := momentum.ComputeAt(series, last, fcfg)
		if f.Close == nil {
			continue
		}

		gi := momentum.GateInput{}
		if mc, ok := marketCaps[sym]; ok && mc > 0 {
			gi.MarketCap = &mc
		} else if so, ok := sharesOut[sym]; ok && so > 0 {
			est := so * *f.Close
			gi.MarketCap = &est
			gi.MarketCapIsProxy = true
		}
		g := momentum.EvaluateGates(&f, gi, gcfg)
		if !g.Passed {
			continue
		}

		si := momentum.ScoreInput{}
		if so, ok := sharesOut[sym]; ok && so > 0 {
			shares := so
			si.FloatSharesEst = &shares
		}
		s, ok := momentum.ScoreCandidate(&f, si, g)
		if !ok {
			continue
		}

		if !rule.opens(g.Bucket, s.Total) {
			continue
		}

		row := store.TrackedRow{
			Symbol:              sym,
			AlertedTS:           series[last].TS,
			Bucket:              string(g.Bucket),
			ReferencePrice:      *f.Close,
			ScoreAtAlert:        s.Total,
			Resistance20AtAlert: f.Resistance20,
			ATR14AtAlert:        f.ATR14,
		}
		if dry {
			fmt.Printf("  would open %-7s %s score=%d ref=%.4f\n", sym, g.Bucket, s.Total, *f.Close)
			opened++
			continue
		}
		created, err := store.OpenTracked(ctx, pool, row)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			continue
		}
		if created {
			fmt.Printf("  opened %-7s %s score=%d ref=%.4f\n", sym, g.Bucket, s.Total, *f.Close)
			opened++
		}
	}
	return opened
}

// reportOutcomes prints the dataset §5's rules will eventually be judged on.
func reportOutcomes(ctx context.Context, pool *pgxpool.Pool) {
	stats, err := store.ExitOutcomes(ctx, pool)
	if err != nil {
		fmt.Fprintln(os.Stderr, "exit outcomes:", err)
		return
	}
	if len(stats) == 0 {
		fmt.Println("\n  no closed positions yet — §5's rules cannot be evaluated until there are.")
		return
	}
	fmt.Println("\n  realized outcomes by exit reason (§5's evaluation dataset):")
	fmt.Printf("    %-20s %-6s %-14s %s\n", "reason", "n", "med peak%", "med exit%")
	for _, s := range stats {
		fmt.Printf("    %-20s %-6d %-14.2f %.2f\n", s.Reason, s.N, s.MedianMaxGain, s.MedianExit)
	}
	fmt.Println("    A reason with a high median peak but a low median exit is firing LATE;")
	fmt.Println("    one with a low median peak is firing on positions that never worked.")
	fmt.Println("    Neither the five conditions nor their ordering is validated (§5).")
}

// loadBars reads bar history for the declared scope.
//
// Carried a hardcoded `u.backfill_selected` join until 2026-09-21, which meant
// the §5 exit replay silently ran over the 450-symbol pilot even after the
// full-universe backfill. The replay reported "119 positions on the widened
// data"; the widening it actually saw was ten years of history for the same
// 450 symbols. Same bug as momentum-backtest, same invisibility: a position
// that is never opened leaves no trace in the output.
func loadBars(ctx context.Context, pool *pgxpool.Pool, interval, source string, scope reportscope.Scope) (map[string][]compute.Bar, error) {
	rows, err := pool.Query(ctx, `
SELECT o.symbol, o.ts, o.open, o.high, o.low, o.close, o.volume
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
		if err := rows.Scan(&sym, &b.TS, &b.Open, &b.High, &b.Low, &b.Close, &b.Volume); err != nil {
			return nil, err
		}
		out[sym] = append(out[sym], b)
	}
	return out, rows.Err()
}

// loadMetric reads one fundamental metric per symbol, for the declared scope.
// Takes the same Scope value as loadBars so the two cannot diverge.
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

// replayOutcome is one simulated position's realized result.
type replayOutcome struct {
	symbol   string
	bucket   momentum.Bucket
	score    int
	reason   string
	matched  string
	exitPct  float64
	peakPct  float64
	sessions int
}

// replayHistory walks each symbol's bars, opening a position when a candidate
// clears its bucket threshold and running §5 forward until an exit fires.
//
// One position per symbol at a time: §5 tracks alerts, and allowing a second
// concurrent position in the same name would double-count the same move.
func replayHistory(
	ctx context.Context,
	pool *pgxpool.Pool,
	bars map[string][]compute.Bar,
	fcfg momentum.Config,
	ecfg momentum.ExitConfig,
	minBars int,
	rule openRule,
	scope reportscope.Scope,
) {
	gcfg := momentum.DefaultGateConfig()
	gcfg.MinBars = minBars
	marketCaps := loadMetric(ctx, pool, "market_cap", scope)
	sharesOut := loadMetric(ctx, pool, "shares_outstanding", scope)
	reportscope.ReportMetricCoverage("market_cap", len(marketCaps), len(bars))
	reportscope.ReportMetricCoverage("shares_outstanding", len(sharesOut), len(bars))

	var outcomes []replayOutcome
	stillOpen := 0

	for sym, series := range bars {
		mc, hasMC := marketCaps[sym]
		so, hasSO := sharesOut[sym]

		var st *momentum.TrackedState
		var openScore int

		for i := minBars; i < len(series); i++ {
			f := momentum.ComputeAt(series, i, fcfg)
			if f.Close == nil {
				continue
			}

			// ── an open position is evaluated first ──
			if st != nil {
				d := momentum.EvaluateExit(*st, &f, ecfg)
				st.HighestCloseSince = d.HighestCloseSince
				st.LowRVolStreak = d.LowRVolStreak
				st.SessionsElapsed = d.SessionsElapsed
				if d.Exit {
					outcomes = append(outcomes, replayOutcome{
						symbol: sym, bucket: st.Bucket, score: openScore,
						reason: d.Reason, matched: d.MatchedString(),
						exitPct: d.ExitPct, peakPct: d.MaxGainPct,
						sessions: d.SessionsElapsed,
					})
					st = nil
				}
				continue
			}

			// ── otherwise look for a new alert ──
			gi := momentum.GateInput{}
			if hasMC && mc > 0 {
				gi.MarketCap = &mc
			} else if hasSO && so > 0 {
				est := so * *f.Close
				gi.MarketCap = &est
				gi.MarketCapIsProxy = true
			}
			g := momentum.EvaluateGates(&f, gi, gcfg)
			if !g.Passed {
				continue
			}
			si := momentum.ScoreInput{}
			if hasSO && so > 0 {
				shares := so
				si.FloatSharesEst = &shares
			}
			sc, ok := momentum.ScoreCandidate(&f, si, g)
			if !ok {
				continue
			}
			if !rule.opens(g.Bucket, sc.Total) {
				continue
			}
			openScore = sc.Total
			st = &momentum.TrackedState{
				Symbol: sym, Bucket: g.Bucket,
				ReferencePrice:      *f.Close,
				Resistance20AtAlert: f.Resistance20,
				ATR14AtAlert:        f.ATR14,
				HighestCloseSince:   *f.Close,
			}
		}
		if st != nil {
			// Ran out of history with the position still open. Excluded from the
			// outcome stats rather than closed at the last bar: a forced exit is
			// not one of §5's five reasons, and counting it as one would invent
			// data for a rule that never fired.
			stillOpen++
		}
	}

	reportReplay(outcomes, stillOpen, rule)
}

func reportReplay(outcomes []replayOutcome, stillOpen int, rule openRule) {
	fmt.Println("═══ §5 exit-rule replay over stored history ═══")
	fmt.Printf("  open rule: %s\n", rule)
	fmt.Printf("  positions closed: %d   still open at end of history: %d (excluded)\n",
		len(outcomes), stillOpen)
	if len(outcomes) == 0 {
		fmt.Println("\n  No positions opened. Not a failure — with these thresholds over this")
		fmt.Println("  universe nothing cleared the bar, so §5 has nothing to evaluate yet.")
		return
	}

	byReason := map[string][]replayOutcome{}
	for _, o := range outcomes {
		byReason[o.reason] = append(byReason[o.reason], o)
	}

	fmt.Printf("\n  %-20s %-6s %-12s %-12s %-12s %s\n",
		"exit reason", "n", "med exit%", "med peak%", "med sessions", "gave back")
	for _, r := range []string{
		momentum.ExitBreakoutFailed, momentum.ExitLostVWAP,
		momentum.ExitMomentumStalled, momentum.ExitStopATR, momentum.ExitTimeout,
	} {
		g := byReason[r]
		if len(g) == 0 {
			fmt.Printf("  %-20s %-6d %-12s %-12s %-12s never fired\n", r, 0, "—", "—", "—")
			continue
		}
		ex := medianOf(g, func(o replayOutcome) float64 { return o.exitPct })
		pk := medianOf(g, func(o replayOutcome) float64 { return o.peakPct })
		se := medianOf(g, func(o replayOutcome) float64 { return float64(o.sessions) })
		// "Gave back" is the gap between the peak the position reached and what
		// the exit actually captured — the number that says whether a rule is
		// firing too late.
		fmt.Printf("  %-20s %-6d %-12.2f %-12.2f %-12.0f %.2f pts\n", r, len(g), ex, pk, se, pk-ex)
	}

	// Session-survival distribution. This is what separates the two
	// unreachability findings: a condition suppressed by ORDERING would surface
	// if reordered, whereas one whose trigger is never reached at all would not.
	maxSessions, reached20 := 0, 0
	for _, o := range outcomes {
		if o.sessions > maxSessions {
			maxSessions = o.sessions
		}
		if o.sessions >= 20 {
			reached20++
		}
	}
	fmt.Printf("\n  session survival: longest %d sessions; %d of %d positions reached 20+\n",
		maxSessions, reached20, len(outcomes))

	allEx := medianOf(outcomes, func(o replayOutcome) float64 { return o.exitPct })
	allPk := medianOf(outcomes, func(o replayOutcome) float64 { return o.peakPct })
	fmt.Printf("\n  overall: median exit %+.2f%%, median peak %+.2f%%, median gave back %.2f pts\n",
		allEx, allPk, allPk-allEx)

	// ── Ordering evidence ──
	//
	// This is what recording every match rather than only the acted-on one buys.
	// A condition that never fires FIRST but is frequently true SIMULTANEOUSLY is
	// not unused — it is outranked, and whether it should be is a question the
	// priority order alone cannot answer.
	fmt.Println("\n  condition co-occurrence — was a lower-priority rule also true?")
	fmt.Printf("    %-20s %-10s %-12s %s\n", "condition", "fired 1st", "also true", "note")
	for _, r := range []string{
		momentum.ExitBreakoutFailed, momentum.ExitLostVWAP,
		momentum.ExitMomentumStalled, momentum.ExitStopATR, momentum.ExitTimeout,
	} {
		first, also := 0, 0
		for _, o := range outcomes {
			if o.reason == r {
				first++
			} else if strings.Contains(o.matched, r) {
				also++
			}
		}
		note := ""
		switch {
		case first == 0 && also == 0:
			note = "never true — untested by this data"
		case first == 0 && also > 0:
			note = "ALWAYS OUTRANKED — never got to fire"
		case also > first:
			note = "outranked more often than it fires"
		}
		fmt.Printf("    %-20s %-10d %-12d %s\n", r, first, also, note)
	}

	fmt.Println("\n  READING THIS HONESTLY:")
	fmt.Println("   • §5's five conditions and their ORDERING are a starting default, not a")
	fmt.Println("     validated strategy. This is the dataset for evaluating them, not a result.")
	fmt.Println("   • A reason with a high median peak but a low median exit is firing LATE.")
	fmt.Println("     One with a low median peak fired on positions that never worked.")
	fmt.Println("   • breakout_failed keys on resistance_20_at_alert, and breakout geometry")
	fmt.Println("     measured INVERTED for predicting +100% moves (§4.1 v2). An exit signal is")
	fmt.Println("     a different question from an entry signal, but that rule in particular")
	fmt.Println("     deserves scrutiny rather than assumption.")
	fmt.Println("   • Same survivorship and point-in-time-fundamentals caveats as §6.")
}

func medianOf(xs []replayOutcome, f func(replayOutcome) float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	v := make([]float64, 0, len(xs))
	for _, x := range xs {
		v = append(v, f(x))
	}
	sort.Float64s(v)
	m := len(v) / 2
	if len(v)%2 == 1 {
		return v[m]
	}
	return (v[m-1] + v[m]) / 2
}

// openMode selects which candidates open a momentum_tracked row.
type openMode string

const (
	// openOnGates tracks every §3.2 gate pass — the same criterion the bot's
	// screener-mode alerts fire on, so the tracked set is the alerted set.
	openOnGates openMode = "gates"
	// openOnScore is the retired per-bucket score threshold (§4.4), kept so
	// Phase 1 §10.1.9's replay remains reproducible.
	openOnScore openMode = "score"
)

func parseOpenMode(s string) (openMode, error) {
	switch m := openMode(strings.ToLower(strings.TrimSpace(s))); m {
	case openOnGates, openOnScore:
		return m, nil
	default:
		return "", fmt.Errorf("-open-mode must be %q or %q, got %q", openOnGates, openOnScore, s)
	}
}

// openRule decides whether a gate-passing, scored candidate opens a row.
type openRule struct {
	mode                openMode
	minMarket, minPenny int
}

func (r openRule) opens(bucket momentum.Bucket, total int) bool {
	if r.mode != openOnScore {
		return true
	}
	if bucket == momentum.BucketPenny {
		return total >= r.minPenny
	}
	return total >= r.minMarket
}

func (r openRule) String() string {
	if r.mode == openOnScore {
		return fmt.Sprintf("score thresholds (market >=%d, penny >=%d)", r.minMarket, r.minPenny)
	}
	return "every gate pass (screener mode)"
}

// alreadyEvaluated reports whether the tracker has nothing new to evaluate for
// this row at bar ts. The alert bar itself is never an evaluation session: a
// row opened this run has no last_evaluated_ts yet, and without this floor a
// second run the same day would evaluate it against its own alert bar and
// count a spurious session 1 — the same double-count the evaluate-before-open
// ordering in main exists to prevent, now reachable because momentum-daily
// may re-run the tracker after a restart.
func alreadyEvaluated(row store.TrackedRow, ts time.Time) bool {
	floor := row.AlertedTS
	if row.LastEvaluatedTS != nil && row.LastEvaluatedTS.After(floor) {
		floor = *row.LastEvaluatedTS
	}
	return !ts.After(floor)
}

// exitStep is the outcome of evaluating one row over its unevaluated bars.
type exitStep struct {
	sessions int // bars evaluated this run; 0 = nothing new
	ts       time.Time
	close    float64
	decision momentum.ExitDecision
}

// stepExits evaluates EVERY bar after the row's floor, oldest first, carrying
// the state forward and stopping at the first exit — the same fold -replay
// does. Evaluating only the latest bar silently merged sessions whenever a run
// covered more than one (a missed day, a bars catch-up): on 2026-09-24 that
// counted 22nd+23rd as one session and judged exits on the 23rd alone.
func stepExits(row store.TrackedRow, series []compute.Bar, fcfg momentum.Config, ecfg momentum.ExitConfig) exitStep {
	st := row.ToTrackedState()
	var out exitStep
	for i := range series {
		if alreadyEvaluated(row, series[i].TS) {
			continue
		}
		f := momentum.ComputeAt(series, i, fcfg)
		d := momentum.EvaluateExit(st, &f, ecfg)
		st.HighestCloseSince = d.HighestCloseSince
		st.LowRVolStreak = d.LowRVolStreak
		st.SessionsElapsed = d.SessionsElapsed
		out.sessions++
		out.ts = series[i].TS
		out.decision = d
		if f.Close != nil {
			out.close = *f.Close
		}
		if d.Exit {
			break
		}
	}
	return out
}
