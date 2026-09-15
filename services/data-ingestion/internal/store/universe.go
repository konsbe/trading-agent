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

// UniverseFundamentals carries the weekly sector/shares/cap refresh (§8.1.2).
//
// Units are ABSOLUTE, already converted from the provider's millions — see the
// unit conventions at the top of 007_momentum.sql. Nil means "not available",
// which is distinct from zero and must stay nil so the §3.2 gate and the §3.9
// market-cap proxy can tell them apart.
type UniverseFundamentals struct {
	Symbol            string
	Exchange          string
	Sector            *string
	Industry          *string
	SharesOutstanding *float64
	MarketCap         *float64
}

const updateUniverseFundamentalsSQL = `
UPDATE universe_symbols SET
    sector             = COALESCE($3, sector),
    industry           = COALESCE($4, industry),
    shares_outstanding = COALESCE($5, shares_outstanding),
    market_cap         = COALESCE($6, market_cap),
    fundamentals_ts    = now(),
    updated_at         = now()
WHERE symbol = $1 AND exchange = $2`

// UpdateUniverseFundamentals applies the weekly fundamentals refresh.
//
// COALESCE means a provider returning null for one field preserves the previous
// value rather than erasing it: a transient gap in coverage should not drop a
// symbol out of the §3.2 market-cap gate. fundamentals_ts still advances, so
// staleness remains visible even when nothing changed.
func UpdateUniverseFundamentals(ctx context.Context, pool *pgxpool.Pool, rows []UniverseFundamentals) (int64, error) {
	if len(rows) == 0 {
		return 0, nil
	}
	batch := &pgx.Batch{}
	for _, r := range rows {
		batch.Queue(updateUniverseFundamentalsSQL,
			r.Symbol, r.Exchange, r.Sector, r.Industry, r.SharesOutstanding, r.MarketCap)
	}
	br := pool.SendBatch(ctx, batch)
	var n int64
	for i := 0; i < len(rows); i++ {
		ct, err := br.Exec()
		if err != nil {
			br.Close() //nolint:errcheck
			return 0, fmt.Errorf("update universe fundamentals %s: %w", rows[i].Symbol, err)
		}
		n += ct.RowsAffected()
	}
	if err := br.Close(); err != nil {
		return 0, fmt.Errorf("close fundamentals batch: %w", err)
	}
	return n, nil
}

// UniverseMember identifies an eligible symbol for downstream jobs.
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

// LoadFundamentalsFromEquityFundamentals reads the latest sector, industry,
// shares outstanding and market cap already ingested into equity_fundamentals,
// for the eligible universe only.
//
// §8.1.2 specifies refreshing these "from existing fundamentals ingestion", so
// this reads the table rather than issuing new API calls. That is also the only
// affordable option: /stock/metric is rate-limited to one request per two
// seconds, which is over four hours for a 7,500-symbol universe (§2.3 names the
// free quota as the binding constraint).
//
// UNIT CONVERSION HAPPENS HERE. equity_fundamentals stores market_cap in $
// millions and shares_outstanding in millions, as documented in SCHEMAS.md,
// while universe_symbols and the §3.2 gates are in absolute dollars and shares.
// Both are multiplied by 1e6 on the way out.
//
// Coverage is limited to whatever data-fundamental was configured to fetch
// (FUNDAMENTAL_SYMBOLS), which is typically far narrower than the universe. The
// caller reports the shortfall rather than hiding it.
func LoadFundamentalsFromEquityFundamentals(ctx context.Context, pool *pgxpool.Pool) ([]UniverseFundamentals, error) {
	const q = `
WITH latest AS (
    SELECT DISTINCT ON (ef.symbol, ef.metric)
           ef.symbol, ef.metric, ef.value, ef.payload
    FROM equity_fundamentals ef
    JOIN universe_symbols u ON u.symbol = ef.symbol AND u.is_eligible
    WHERE ef.period = 'ttm'
      AND ef.metric IN ('market_cap', 'shares_outstanding', 'sector_profile')
    ORDER BY ef.symbol, ef.metric, ef.ts DESC
)
SELECT u.symbol,
       u.exchange,
       max(l.payload ->> 'sector')   FILTER (WHERE l.metric = 'sector_profile')   AS sector,
       max(l.payload ->> 'industry') FILTER (WHERE l.metric = 'sector_profile')   AS industry,
       max(l.value)                  FILTER (WHERE l.metric = 'shares_outstanding') AS shares_m,
       max(l.value)                  FILTER (WHERE l.metric = 'market_cap')         AS market_cap_m
FROM latest l
JOIN universe_symbols u ON u.symbol = l.symbol
GROUP BY u.symbol, u.exchange`

	rows, err := pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("load fundamentals for universe: %w", err)
	}
	defer rows.Close()

	const millions = 1e6
	var out []UniverseFundamentals
	for rows.Next() {
		var r UniverseFundamentals
		var sharesM, capM *float64
		if err := rows.Scan(&r.Symbol, &r.Exchange, &r.Sector, &r.Industry, &sharesM, &capM); err != nil {
			return nil, fmt.Errorf("scan universe fundamentals: %w", err)
		}
		if sharesM != nil {
			v := *sharesM * millions
			r.SharesOutstanding = &v
		}
		if capM != nil {
			v := *capM * millions
			r.MarketCap = &v
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// FundamentalsCoverage counts how much of the eligible universe actually has
// each field, so a thin provider list is visible as a number rather than as an
// unexplained empty penny bucket at scan time.
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
