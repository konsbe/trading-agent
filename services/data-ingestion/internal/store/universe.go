package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// UniverseRow is one eligibility decision ready to upsert into
// universe_symbols (migration 007_momentum.sql, spec §3.1).
type UniverseRow struct {
	Symbol         string
	Exchange       string
	MIC            string
	Name           string
	Type           string
	IsEligible     bool
	ExcludedReason string // "" iff IsEligible
}

// upsertUniverseSymbolSQL writes the symbol-list half of the row and leaves the
// fundamentals and backfill columns alone.
//
// Two deliberate choices:
//   - COALESCE on backfill_status keeps an in-flight or completed backfill from
//     being reset to 'pending' by the weekly symbol refresh. The refresh knows
//     nothing about bar progress and must not clobber it.
//   - excluded_reason is overwritten every run, including to NULL, so a symbol
//     that changes type or venue stops carrying a stale reason.
const upsertUniverseSymbolSQL = `
INSERT INTO universe_symbols
    (symbol, exchange, mic, name, type, is_eligible, excluded_reason, updated_at)
VALUES ($1,$2,$3,$4,$5,$6,$7, now())
ON CONFLICT (symbol, exchange) DO UPDATE SET
    mic             = EXCLUDED.mic,
    name            = EXCLUDED.name,
    type            = EXCLUDED.type,
    is_eligible     = EXCLUDED.is_eligible,
    excluded_reason = EXCLUDED.excluded_reason,
    updated_at      = now()`

// UpsertUniverseSymbols writes every decision in one transaction.
//
// The batch is transactional so the scanner never reads a half-refreshed
// universe: under READ COMMITTED a concurrent reader sees either the previous
// complete universe or the new one, never a mixture. This matters because the
// daily scan iterates `WHERE is_eligible` and a partial view would silently
// shrink the candidate set.
func UpsertUniverseSymbols(ctx context.Context, pool *pgxpool.Pool, rows []UniverseRow) (int64, error) {
	if len(rows) == 0 {
		return 0, nil
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin universe tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck — no-op after Commit

	batch := &pgx.Batch{}
	for _, r := range rows {
		batch.Queue(upsertUniverseSymbolSQL,
			r.Symbol, r.Exchange, nullable(r.MIC), nullable(r.Name), nullable(r.Type),
			r.IsEligible, nullable(r.ExcludedReason),
		)
	}
	br := tx.SendBatch(ctx, batch)
	var n int64
	for i := 0; i < len(rows); i++ {
		ct, err := br.Exec()
		if err != nil {
			br.Close() //nolint:errcheck
			return 0, fmt.Errorf("upsert universe symbol %s: %w", rows[i].Symbol, err)
		}
		n += ct.RowsAffected()
	}
	if err := br.Close(); err != nil {
		return 0, fmt.Errorf("close universe batch: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit universe tx: %w", err)
	}
	return n, nil
}

type UniverseMember struct {
	Symbol   string
	Exchange string
}

// EligibleUniverse lists eligible symbols. This is the set the §8.1.3 backfill
// and the daily bar refresh iterate.
//
// Note it does NOT filter on bar_count. The 250-bar minimum cannot gate this
// query without being circular — bars only exist because the backfill ran over
// this list. History sufficiency is enforced later as a §3.2 hard gate.
func EligibleUniverse(ctx context.Context, pool *pgxpool.Pool) ([]UniverseMember, error) {
	const q = `SELECT symbol, exchange FROM universe_symbols WHERE is_eligible ORDER BY symbol`
	rows, err := pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("query eligible universe: %w", err)
	}
	defer rows.Close()
	var out []UniverseMember
	for rows.Next() {
		var m UniverseMember
		if err := rows.Scan(&m.Symbol, &m.Exchange); err != nil {
			return nil, fmt.Errorf("scan eligible universe: %w", err)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// RefreshUniverseBarCounts recomputes bar_count, first_bar_ts and last_bar_ts
// from equity_ohlcv for every eligible symbol, in a single statement.
//
// Purely for auditability: it makes "how much history does this symbol have"
// answerable without a join, so §10 step 3 can verify the backfill and step 5
// can explain an insufficient_history gate failure. It never sets
// is_eligible — see the note on EligibleUniverse.
func RefreshUniverseBarCounts(ctx context.Context, pool *pgxpool.Pool, interval, source string) (int64, error) {
	const q = `
UPDATE universe_symbols u SET
    bar_count    = b.n,
    first_bar_ts = b.first_ts,
    last_bar_ts  = b.last_ts,
    updated_at   = now()
FROM (
    SELECT symbol, count(*) AS n, min(ts) AS first_ts, max(ts) AS last_ts
    FROM equity_ohlcv
    WHERE interval = $1 AND source = $2
    GROUP BY symbol
) b
WHERE u.symbol = b.symbol`
	ct, err := pool.Exec(ctx, q, interval, source)
	if err != nil {
		return 0, fmt.Errorf("refresh universe bar counts: %w", err)
	}
	return ct.RowsAffected(), nil
}

type FundamentalsCoverage struct {
	Eligible          int
	WithSector        int
	WithShares        int
	WithMarketCap     int
	WithNeitherCapNor int // no market_cap AND no shares_outstanding: §3.9's proxy cannot help these
}

func LoadFundamentalsCoverage(ctx context.Context, pool *pgxpool.Pool) (FundamentalsCoverage, error) {
	var c FundamentalsCoverage
	const q = `
SELECT count(*),
       count(*) FILTER (WHERE sector IS NOT NULL),
       count(*) FILTER (WHERE shares_outstanding IS NOT NULL),
       count(*) FILTER (WHERE market_cap IS NOT NULL),
       count(*) FILTER (WHERE market_cap IS NULL AND shares_outstanding IS NULL)
FROM universe_symbols WHERE is_eligible`
	err := pool.QueryRow(ctx, q).Scan(
		&c.Eligible, &c.WithSector, &c.WithShares, &c.WithMarketCap, &c.WithNeitherCapNor)
	if err != nil {
		return c, fmt.Errorf("fundamentals coverage: %w", err)
	}
	return c, nil
}

// UniverseCounts is the audit summary §10 step 2 requires: eligible total plus
// a breakdown of why everything else was excluded.
type UniverseCounts struct {
	Total      int
	Eligible   int
	ByReason   map[string]int
	ByExchange map[string]int
	LastUpdate *time.Time
}

// LoadUniverseCounts reads the audit summary back out of the table, so the
// numbers reported are the persisted ones rather than in-memory tallies.
func LoadUniverseCounts(ctx context.Context, pool *pgxpool.Pool) (UniverseCounts, error) {
	c := UniverseCounts{ByReason: map[string]int{}, ByExchange: map[string]int{}}

	const totals = `SELECT count(*), count(*) FILTER (WHERE is_eligible), max(updated_at) FROM universe_symbols`
	if err := pool.QueryRow(ctx, totals).Scan(&c.Total, &c.Eligible, &c.LastUpdate); err != nil {
		return c, fmt.Errorf("universe totals: %w", err)
	}

	const byReason = `
SELECT COALESCE(excluded_reason, 'unknown'), count(*)
FROM universe_symbols WHERE NOT is_eligible GROUP BY 1 ORDER BY 2 DESC`
	rows, err := pool.Query(ctx, byReason)
	if err != nil {
		return c, fmt.Errorf("universe by reason: %w", err)
	}
	for rows.Next() {
		var reason string
		var n int
		if err := rows.Scan(&reason, &n); err != nil {
			rows.Close()
			return c, err
		}
		c.ByReason[reason] = n
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return c, err
	}

	const byExchange = `
SELECT exchange, count(*) FROM universe_symbols WHERE is_eligible GROUP BY 1 ORDER BY 2 DESC`
	rows2, err := pool.Query(ctx, byExchange)
	if err != nil {
		return c, fmt.Errorf("universe by exchange: %w", err)
	}
	defer rows2.Close()
	for rows2.Next() {
		var ex string
		var n int
		if err := rows2.Scan(&ex, &n); err != nil {
			return c, err
		}
		c.ByExchange[ex] = n
	}
	return c, rows2.Err()
}
