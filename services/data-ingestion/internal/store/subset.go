package store

import (
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SubsetStrategy selects which eligible symbols form the pilot subset.
type SubsetStrategy string

const (
	// SubsetRandom samples uniformly at random from the eligible universe.
	//
	// The default, and the only strategy that keeps §6's base rate meaningful.
	// A random sample of N from the universe is statistically unbiased, so the
	// measured base rate estimates the real one with wider confidence intervals
	// rather than a shifted centre — and it keeps both §3.2 buckets represented
	// in proportion.
	SubsetRandom SubsetStrategy = "random"

	// SubsetStratified samples randomly WITHIN each §3.2 price bucket, with a
	// floor on the penny bucket.
	//
	// The default for the pilot, and the only strategy that makes §6's base rate
	// computable for both buckets. Plain SubsetRandom has the same failure mode
	// as a curated list, just by accident instead of by construction: at a ~9 %
	// sampling rate (450 of 4,978), whatever the universe's natural penny
	// proportion happens to be decides how many sub-$2 symbols land in the
	// sample, and a thin draw leaves §3.2's penny thresholds, its separate
	// market-cap band, and the documented RSI-action unreachability entirely
	// unexercised — discovered only after the fact.
	//
	// Stratifying makes the composition a decision rather than an outcome.
	SubsetStratified SubsetStrategy = "stratified"

	// SubsetExplicit uses a configured symbol list verbatim.
	//
	// Useful for reproducing a specific run or debugging named symbols. NOT
	// suitable for §6's base rate: hand-picking by familiarity or liquidity
	// biases the sample in an unknown direction, and a list of well-known
	// large caps contains essentially no §3.2 penny-bucket members, leaving
	// half the scanner untested.
	SubsetExplicit SubsetStrategy = "explicit"
)

// SubsetSizeError reports that a requested subset exceeds the configured cap.
//
// This is the guard that keeps Tiingo's monthly allowance intact. Its allowance
// is 500 UNIQUE SYMBOLS PER MONTH — not a rate and not a daily count, so
// api_rate_budget structurally cannot express it and no limiter will refuse the
// 501st symbol. Exceeding it is only discovered when Tiingo starts rejecting
// requests, weeks later, for reasons that look unrelated to the change that
// caused them. Failing loudly here is the only cheap place to catch it.
type SubsetSizeError struct {
	Requested int
	Max       int
}

func (e *SubsetSizeError) Error() string {
	return fmt.Sprintf(
		"pilot subset of %d exceeds the cap of %d: Tiingo's free allowance is 500 unique symbols per month "+
			"and nothing else enforces it — widening the subset silently spends the month, and the failure "+
			"surfaces later as unexplained rejections. Raise the cap deliberately or shrink the subset",
		e.Requested, e.Max)
}

// SelectSubsetParams configures one selection run.
type SelectSubsetParams struct {
	Strategy SubsetStrategy

	// Size is how many symbols to mark. Ignored by SubsetExplicit.
	Size int

	// MaxSize is the hard cap asserted before anything is written.
	MaxSize int

	// Symbols is the verbatim list for SubsetExplicit.
	Symbols []string

	// Seed makes SubsetRandom reproducible. Empty means unseeded — a different
	// sample per run, fine for exploration but meaning a re-run cannot be
	// compared against the previous one. Set it to anything stable (a date, a
	// run label) for numbers you intend to cite.
	//
	// Seeding hashes the seed with each symbol rather than calling setseed():
	// PostgreSQL's setseed fixes the SEQUENCE of random() values, not which row
	// receives which value, so any change in physical scan order — and marking
	// rows changes it — reshuffles the sample. Hashing is stable against table
	// state, row order, and parallel plans.
	Seed string

	// MinBars, when > 0, restricts selection to symbols that already have this
	// much history. Zero selects regardless, which is correct on a fresh
	// install where no bars exist yet — the whole point of the subset is to
	// decide what to fetch.
	MinBars int

	// ── SubsetStratified only ────────────────────────────────────────────

	// PennyFloorPct is the minimum share of the subset drawn from the penny
	// bucket, e.g. 0.20 for 20 %. Below this the selection fails rather than
	// silently returning a market-heavy sample.
	PennyFloorPct float64

	// PennyMinPrice / PennyMaxPrice are §3.2's penny price band ($0.30–$2.00).
	// A symbol at or above PennyMaxPrice is market-bucket.
	PennyMinPrice float64
	PennyMaxPrice float64

	// PriceInterval / PriceSource identify which equity_ohlcv rows supply the
	// price used to bucket a symbol.
	//
	// Deliberately separate from the bar source. Bucketing uses Finnhub /quote
	// snapshots (interval='quote_snapshot', source='finnhub_quote') because they
	// can be gathered for the whole universe without spending the bar provider's
	// quota — see cmd/data-universe/pricing.go. The backfill then writes real
	// adjusted daily bars under a different interval and source.
	PriceInterval string
	PriceSource   string
}

// StratificationError reports that the universe cannot supply the requested
// bucket composition.
//
// Kept distinct from a generic failure because the two causes need different
// fixes, and both are invisible from a row count alone.
type StratificationError struct {
	Reason     string
	WantPenny  int
	HavePenny  int
	WantMarket int
	HaveMarket int
	Priced     int
}

func (e *StratificationError) Error() string {
	return fmt.Sprintf(
		"cannot stratify the subset: %s (penny want %d have %d, market want %d have %d, symbols with a known price %d). "+
			"Stratifying needs a last close per symbol from equity_ohlcv, which the backfill produces — so on a fresh "+
			"install run an unstratified draw first, backfill it, then re-stratify (reselection replaces, it does not accumulate)",
		e.Reason, e.WantPenny, e.HavePenny, e.WantMarket, e.HaveMarket, e.Priced)
}

// SelectPilotSubset marks the pilot subset, replacing any previous selection.
//
// The size assertion runs BEFORE any write, so an over-large request changes
// nothing. The clear-then-mark runs in one transaction, so a concurrent reader
// never sees a partially-reselected subset — which would otherwise show up as a
// backfill covering some symbols from the old set and some from the new.
func SelectPilotSubset(ctx context.Context, pool *pgxpool.Pool, p SelectSubsetParams) (int, error) {
	if p.Strategy == "" {
		p.Strategy = SubsetRandom
	}

	// Determine the intended size first and assert the cap before writing.
	want := p.Size
	if p.Strategy == SubsetExplicit {
		want = len(dedupeUpper(p.Symbols))
	}
	if want <= 0 {
		return 0, fmt.Errorf("pilot subset size must be positive, got %d", want)
	}
	if p.MaxSize > 0 && want > p.MaxSize {
		return 0, &SubsetSizeError{Requested: want, Max: p.MaxSize}
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin subset tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck — no-op after Commit

	if _, err := tx.Exec(ctx,
		`UPDATE universe_symbols SET backfill_selected = false, updated_at = now() WHERE backfill_selected`,
	); err != nil {
		return 0, fmt.Errorf("clear previous subset: %w", err)
	}

	var n int
	switch p.Strategy {
	case SubsetRandom:
		// Two query shapes rather than one with a CASE: the seeded form must
		// order by a deterministic hash of (seed, symbol), which is stable
		// regardless of scan order, while the unseeded form uses random().
		var (
			ct  interface{ RowsAffected() int64 }
			err error
		)
		if p.Seed != "" {
			const seeded = `
UPDATE universe_symbols u SET backfill_selected = true, updated_at = now()
WHERE (u.symbol, u.exchange) IN (
    SELECT symbol, exchange FROM universe_symbols
    WHERE is_eligible AND ($2 = 0 OR COALESCE(bar_count, 0) >= $2)
    ORDER BY md5($3 || symbol)
    LIMIT $1
)`
			ct, err = tx.Exec(ctx, seeded, p.Size, p.MinBars, p.Seed)
		} else {
			const unseeded = `
UPDATE universe_symbols u SET backfill_selected = true, updated_at = now()
WHERE (u.symbol, u.exchange) IN (
    SELECT symbol, exchange FROM universe_symbols
    WHERE is_eligible AND ($2 = 0 OR COALESCE(bar_count, 0) >= $2)
    ORDER BY random()
    LIMIT $1
)`
			ct, err = tx.Exec(ctx, unseeded, p.Size, p.MinBars)
		}
		if err != nil {
			return 0, fmt.Errorf("select random subset: %w", err)
		}
		n = int(ct.RowsAffected())

	case SubsetStratified:
		got, serr := selectStratified(ctx, tx, p)
		if serr != nil {
			return 0, serr
		}
		n = got

	case SubsetExplicit:
		syms := dedupeUpper(p.Symbols)
		const q = `
UPDATE universe_symbols SET backfill_selected = true, updated_at = now()
WHERE is_eligible AND symbol = ANY($1)`
		ct, err := tx.Exec(ctx, q, syms)
		if err != nil {
			return 0, fmt.Errorf("select explicit subset: %w", err)
		}
		n = int(ct.RowsAffected())

	default:
		return 0, fmt.Errorf("unknown subset strategy %q", p.Strategy)
	}

	// Re-assert against what was actually written. The pre-check used the
	// intended size; this catches the case where a symbol appears on more than
	// one exchange and one requested entry marks two rows — which would spend
	// two of Tiingo's unique-symbol slots.
	if p.MaxSize > 0 && n > p.MaxSize {
		return 0, &SubsetSizeError{Requested: n, Max: p.MaxSize}
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit subset tx: %w", err)
	}
	return n, nil
}

// SubsetStats describes the current selection, for logging and for the
// coverage and consistency checks the pilot needs.
type SubsetStats struct {
	Selected int

	// UnderTwoDollars counts selected symbols priced below $2, the §3.2
	// penny-bucket band, using the selection-time price. Reported because a
	// subset with none of them leaves half the scanner's gate logic unexercised,
	// and that is invisible in a run that otherwise looks healthy.
	UnderTwoDollars int

	// WithBars counts selected symbols that already have real daily bars. Zero
	// immediately after selection — the backfill has not run yet — and expected
	// to converge on Selected afterwards.
	WithBars int

	// BucketChecked and BucketDrifted compare each symbol's selection-time
	// bucket against its bucket once real bars have landed.
	//
	// Some disagreement is inherent and accepted, not a bug: selection prices
	// come from Finnhub's live /quote snapshot, while the backfill stores
	// Tiingo's split- and dividend-adjusted close. A symbol quoted at $2.02 and
	// adjusted to $1.98 was drawn as market-bucket and is really penny-bucket.
	//
	// It is flagged rather than ignored because a drifted symbol is scored
	// against the wrong bucket's thresholds for the whole pilot, and that
	// surfaces as a merely odd-looking candidate rather than as a
	// classification error. Same instinct as UnderTwoDollars: state the
	// discrepancy while the numbers are in front of someone.
	BucketChecked int
	BucketDrifted int

	// BucketDriftExamples holds up to 10 drifted symbols as
	// "SYM sel=1.98 bar=2.02", so the warning is actionable without a query.
	BucketDriftExamples []string
}

// LoadSubsetStats reads the selection-time price from (selInterval, selSource)
// and the real bars from (barInterval, barSource). The two are separate because
// bucketing is decided from Finnhub quote snapshots before any bar provider
// quota is spent; see cmd/data-universe/pricing.go.
func LoadSubsetStats(
	ctx context.Context,
	pool *pgxpool.Pool,
	selInterval, selSource, barInterval, barSource string,
	pennyMax float64,
) (SubsetStats, error) {
	var s SubsetStats
	const q = `
WITH sel AS (
    SELECT u.symbol
    FROM universe_symbols u
    WHERE u.is_eligible AND u.backfill_selected
), sel_price AS (
    SELECT DISTINCT ON (o.symbol) o.symbol, o.close
    FROM equity_ohlcv o
    JOIN sel ON sel.symbol = o.symbol
    WHERE o.interval = $1 AND ($2 = '' OR o.source = $2) AND o.close > 0
    ORDER BY o.symbol, o.ts DESC
), bar_price AS (
    SELECT DISTINCT ON (o.symbol) o.symbol, o.close
    FROM equity_ohlcv o
    JOIN sel ON sel.symbol = o.symbol
    WHERE o.interval = $3 AND ($4 = '' OR o.source = $4) AND o.close > 0
    ORDER BY o.symbol, o.ts DESC
), cmp AS (
    SELECT s.symbol, s.close AS sel_close, b.close AS bar_close
    FROM sel_price s JOIN bar_price b ON b.symbol = s.symbol
    WHERE (s.close < $5) <> (b.close < $5)
)
SELECT (SELECT count(*) FROM sel),
       (SELECT count(*) FROM sel_price WHERE close < $5),
       (SELECT count(*) FROM bar_price),
       (SELECT count(*) FROM sel_price s JOIN bar_price b ON b.symbol = s.symbol),
       (SELECT count(*) FROM cmp),
       COALESCE((SELECT array_agg(symbol || ' sel=' || round(sel_close::numeric, 2) || ' bar=' || round(bar_close::numeric, 2))
                 FROM (SELECT * FROM cmp ORDER BY symbol LIMIT 10) x), '{}')`
	if err := pool.QueryRow(ctx, q, selInterval, selSource, barInterval, barSource, pennyMax).Scan(
		&s.Selected, &s.UnderTwoDollars, &s.WithBars,
		&s.BucketChecked, &s.BucketDrifted, &s.BucketDriftExamples); err != nil {
		return s, fmt.Errorf("subset stats: %w", err)
	}
	return s, nil
}

// SelectedSubset lists the pilot symbols, for the backfill and daily refresh to
// iterate instead of the full universe.
func SelectedSubset(ctx context.Context, pool *pgxpool.Pool) ([]UniverseMember, error) {
	const q = `
SELECT symbol, exchange FROM universe_symbols
WHERE is_eligible AND backfill_selected ORDER BY symbol`
	rows, err := pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("query selected subset: %w", err)
	}
	defer rows.Close()
	var out []UniverseMember
	for rows.Next() {
		var m UniverseMember
		if err := rows.Scan(&m.Symbol, &m.Exchange); err != nil {
			return nil, fmt.Errorf("scan selected subset: %w", err)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func dedupeUpper(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.ToUpper(strings.TrimSpace(s))
		if s == "" {
			continue
		}
		if _, dup := seen[s]; dup {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

// selectStratified draws the subset per §3.2's price buckets.
//
// Bucketing uses each symbol's most recent close from equity_ohlcv, which
// creates a bootstrap dependency worth naming: stratifying by price needs
// prices, and prices come from the backfill that this selection chooses. On a
// fresh install there is no price data, so this fails loudly with instructions
// rather than silently degrading to an unstratified draw — which would look
// identical in the row count and only reveal itself as a missing bucket at
// Step 7.
//
// Draw order within each bucket is md5(seed || symbol), so a seeded run is
// reproducible independently of scan order (PostgreSQL's setseed fixes the
// sequence of random() values, not which row receives which, and marking rows
// changes physical order).
func selectStratified(ctx context.Context, tx pgx.Tx, p SelectSubsetParams) (int, error) {
	minP, maxP := p.PennyMinPrice, p.PennyMaxPrice
	if minP <= 0 {
		minP = 0.30 // §3.2 penny floor: below this, tick artefacts dominate
	}
	if maxP <= 0 {
		maxP = 2.00 // §3.2 boundary between penny and market buckets
	}
	interval, source := p.PriceInterval, p.PriceSource
	if interval == "" {
		interval = "quote_snapshot"
	}

	wantPenny := int(math.Ceil(float64(p.Size) * p.PennyFloorPct))
	if wantPenny > p.Size {
		wantPenny = p.Size
	}
	wantMarket := p.Size - wantPenny

	// How many candidates each bucket can actually supply.
	const countSQL = `
WITH last_close AS (
    SELECT DISTINCT ON (o.symbol) o.symbol, o.close
    FROM equity_ohlcv o
    JOIN universe_symbols u ON u.symbol = o.symbol AND u.is_eligible
    WHERE o.interval = $1 AND ($2 = '' OR o.source = $2)
      AND ($5 = 0 OR COALESCE(u.bar_count, 0) >= $5)
    ORDER BY o.symbol, o.ts DESC
)
SELECT count(*) FILTER (WHERE close >= $3 AND close < $4) AS penny,
       count(*) FILTER (WHERE close >= $4)                AS market,
       count(*)                                           AS priced
FROM last_close`
	var havePenny, haveMarket, priced int
	if err := tx.QueryRow(ctx, countSQL, interval, source, minP, maxP, p.MinBars).
		Scan(&havePenny, &haveMarket, &priced); err != nil {
		return 0, fmt.Errorf("count stratification buckets: %w", err)
	}

	if priced == 0 {
		return 0, &StratificationError{
			Reason: "no symbol has a known last close", WantPenny: wantPenny,
			WantMarket: wantMarket, Priced: 0,
		}
	}
	if havePenny < wantPenny {
		return 0, &StratificationError{
			Reason:    "the penny bucket cannot supply its floor",
			WantPenny: wantPenny, HavePenny: havePenny,
			WantMarket: wantMarket, HaveMarket: haveMarket, Priced: priced,
		}
	}
	if haveMarket < wantMarket {
		return 0, &StratificationError{
			Reason:    "the market bucket cannot supply its share",
			WantPenny: wantPenny, HavePenny: havePenny,
			WantMarket: wantMarket, HaveMarket: haveMarket, Priced: priced,
		}
	}

	// One statement per bucket, ordered by the seeded hash when a seed is given.
	order := "random()"
	if p.Seed != "" {
		order = "md5($6 || symbol)"
	}
	markSQL := fmt.Sprintf(`
WITH last_close AS (
    SELECT DISTINCT ON (o.symbol) o.symbol, o.close
    FROM equity_ohlcv o
    JOIN universe_symbols u ON u.symbol = o.symbol AND u.is_eligible
    WHERE o.interval = $1 AND ($2 = '' OR o.source = $2)
      AND ($5 = 0 OR COALESCE(u.bar_count, 0) >= $5)
    ORDER BY o.symbol, o.ts DESC
), picked AS (
    SELECT symbol FROM last_close
    WHERE close >= $3 AND close < $4
    ORDER BY %s
    LIMIT $7
)
UPDATE universe_symbols u SET backfill_selected = true, updated_at = now()
FROM picked WHERE u.symbol = picked.symbol AND u.is_eligible`, order)

	args := func(lo, hi float64, limit int) []any {
		a := []any{interval, source, lo, hi, p.MinBars}
		if p.Seed != "" {
			a = append(a, p.Seed)
		} else {
			a = append(a, nil)
		}
		return append(a, limit)
	}

	var total int
	if wantPenny > 0 {
		ct, err := tx.Exec(ctx, markSQL, args(minP, maxP, wantPenny)...)
		if err != nil {
			return 0, fmt.Errorf("mark penny stratum: %w", err)
		}
		total += int(ct.RowsAffected())
	}
	if wantMarket > 0 {
		// Upper bound large enough to include every market-bucket price.
		ct, err := tx.Exec(ctx, markSQL, args(maxP, math.MaxFloat64, wantMarket)...)
		if err != nil {
			return 0, fmt.Errorf("mark market stratum: %w", err)
		}
		total += int(ct.RowsAffected())
	}
	return total, nil
}

// PriceCoverage reports how many symbols have a usable selection-time price and
// how they split across §3.2's buckets.
type PriceCoverage struct {
	Priced int
	Penny  int
	Market int

	// BelowFloor counts symbols priced under §3.2's $0.30 penny floor. They are
	// in neither bucket — sub-$0.30 names are dominated by tick artefacts and
	// reverse-split noise, which is why the floor exists — so they are reported
	// rather than silently folded into the penny stratum.
	BelowFloor int
}

func LoadPriceCoverage(ctx context.Context, pool *pgxpool.Pool, interval, source string, pennyMin, pennyMax float64) (PriceCoverage, error) {
	var c PriceCoverage
	const q = `
WITH last_price AS (
    SELECT DISTINCT ON (o.symbol) o.symbol, o.close
    FROM equity_ohlcv o
    JOIN universe_symbols u ON u.symbol = o.symbol AND u.is_eligible
    WHERE o.interval = $1 AND ($2 = '' OR o.source = $2) AND o.close > 0
    ORDER BY o.symbol, o.ts DESC
)
SELECT count(*),
       count(*) FILTER (WHERE close >= $3 AND close < $4),
       count(*) FILTER (WHERE close >= $4),
       count(*) FILTER (WHERE close < $3)
FROM last_price`
	if err := pool.QueryRow(ctx, q, interval, source, pennyMin, pennyMax).
		Scan(&c.Priced, &c.Penny, &c.Market, &c.BelowFloor); err != nil {
		return c, fmt.Errorf("price coverage: %w", err)
	}
	return c, nil
}

// CountSelected reports how many symbols are currently marked as the pilot
// subset, so a committed draw is not silently replaced on restart.
func CountSelected(ctx context.Context, pool *pgxpool.Pool) (int, error) {
	var n int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM universe_symbols WHERE backfill_selected`).Scan(&n); err != nil {
		return 0, fmt.Errorf("count selected: %w", err)
	}
	return n, nil
}
