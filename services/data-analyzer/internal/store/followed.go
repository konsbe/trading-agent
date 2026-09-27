package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// Followed symbols, the symbol directory and manual Compute requests
// (migration 030), for momentum-api's followed-symbols / computed-symbols
// endpoints. The ingestion services read followed_symbols directly.

// FollowedRow is one followed_symbols row, with a display name when the
// directory or the universe knows the symbol.
type FollowedRow struct {
	Symbol    string
	AssetType string
	Listing   string
	NewsAlias *string
	Source    string
	AddedAt   time.Time
	Name      *string
}

const symbolNameSQL = `COALESCE(
    (SELECT name FROM universe_symbols u WHERE u.symbol = %[1]s LIMIT 1),
    (SELECT name FROM symbol_directory d WHERE d.symbol = %[1]s ORDER BY source LIMIT 1))`

func ListFollowed(ctx context.Context, q Querier) ([]FollowedRow, error) {
	rows, err := q.Query(ctx, `
SELECT f.symbol, f.asset_type, f.listing, f.news_alias, f.source, f.added_at, `+fmt.Sprintf(symbolNameSQL, "f.symbol")+`
FROM followed_symbols f
ORDER BY f.added_at DESC, f.symbol`)
	if err != nil {
		return nil, fmt.Errorf("list followed: %w", err)
	}
	defer rows.Close()
	out := []FollowedRow{}
	for rows.Next() {
		var r FollowedRow
		if err := rows.Scan(&r.Symbol, &r.AssetType, &r.Listing, &r.NewsAlias, &r.Source, &r.AddedAt, &r.Name); err != nil {
			return nil, fmt.Errorf("scan followed: %w", err)
		}
		r.AddedAt = r.AddedAt.UTC()
		out = append(out, r)
	}
	return out, rows.Err()
}

// SymbolClass is what a symbol is, for following or computing it.
type SymbolClass struct {
	Symbol    string
	AssetType string // equity | etf | crypto
	Listing   string // us | foreign | crypto
	Name      *string
}

// ErrNotComputable: the symbol is listed, but as a type no pipeline computes
// (warrants, units, rights, preferreds).
var ErrNotComputable = errors.New("symbol type is not computed")

// ClassifySymbol finds a symbol in followed_symbols, then universe_symbols,
// then symbol_directory. found is false for a symbol none of them know; err is
// ErrNotComputable for a directory entry of asset type other.
func ClassifySymbol(ctx context.Context, q Querier, symbol string) (SymbolClass, bool, error) {
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	c := SymbolClass{Symbol: sym}
	var assetType, listing *string
	err := q.QueryRow(ctx, `
SELECT COALESCE(
         (SELECT asset_type FROM followed_symbols WHERE symbol = $1),
         (SELECT CASE WHEN type IN ('ETP', 'ETF', 'Closed-End Fund') THEN 'etf' ELSE 'equity' END
            FROM universe_symbols WHERE symbol = $1 LIMIT 1),
         (SELECT asset_type FROM symbol_directory WHERE symbol = $1 ORDER BY source LIMIT 1)),
       COALESCE(
         (SELECT listing FROM followed_symbols WHERE symbol = $1),
         (SELECT 'us' FROM universe_symbols WHERE symbol = $1 LIMIT 1),
         (SELECT CASE WHEN source = 'binance_spot' THEN 'crypto' ELSE 'us' END
            FROM symbol_directory WHERE symbol = $1 ORDER BY source LIMIT 1)),
       `+fmt.Sprintf(symbolNameSQL, "$1"), sym).Scan(&assetType, &listing, &c.Name)
	if err != nil {
		return c, false, fmt.Errorf("classify %s: %w", sym, err)
	}
	if assetType == nil {
		return c, false, nil
	}
	c.AssetType, c.Listing = *assetType, *listing
	if c.AssetType == "other" {
		return c, true, ErrNotComputable
	}
	return c, true, nil
}

// AddFollowed is idempotent; it also opens the symbol's "followed" reason, so
// the ingestion workers pick it up without waiting for the daily reconcile.
func AddFollowed(ctx context.Context, db TxBeginner, c SymbolClass) (added bool, err error) {
	tx, err := db.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	tag, err := tx.Exec(ctx, `
INSERT INTO followed_symbols (symbol, asset_type, listing, source) VALUES ($1, $2, $3, 'user')
ON CONFLICT (symbol) DO NOTHING`, c.Symbol, c.AssetType, c.Listing)
	if err != nil {
		return false, fmt.Errorf("add followed %s: %w", c.Symbol, err)
	}
	if err := openReason(ctx, tx, c.Symbol, c.AssetType, "followed"); err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, tx.Commit(ctx)
}

// RemoveFollowed is idempotent; it closes the "followed" reason (the row is
// kept, and so is everything computed for the symbol).
func RemoveFollowed(ctx context.Context, db TxBeginner, symbol string) (removed bool, err error) {
	tx, err := db.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	tag, err := tx.Exec(ctx, `DELETE FROM followed_symbols WHERE symbol = upper($1)`, symbol)
	if err != nil {
		return false, fmt.Errorf("remove followed %s: %w", symbol, err)
	}
	if err := closeReason(ctx, tx, symbol, "followed"); err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, tx.Commit(ctx)
}

// RequestCompute opens a manual reason (idempotent): the ingestion workers
// fetch the symbol's data on their short poll, then momentum-daily computes it.
func RequestCompute(ctx context.Context, db Execer, c SymbolClass) (opened bool, err error) {
	tag, err := db.Exec(ctx, `
INSERT INTO computation_interest (symbol, asset_type, reason) VALUES ($1, $2, 'manual')
ON CONFLICT (symbol, reason) WHERE active_until IS NULL DO NOTHING`, c.Symbol, c.AssetType)
	if err != nil {
		return false, fmt.Errorf("request compute %s: %w", c.Symbol, err)
	}
	return tag.RowsAffected() == 1, nil
}

// StopCompute closes the manual reason ("Stop computing"). Other reasons, and
// every computed row, are untouched.
func StopCompute(ctx context.Context, db Execer, symbol string) (closed bool, err error) {
	tag, err := db.Exec(ctx, `
UPDATE computation_interest SET active_until = now()
WHERE symbol = upper($1) AND reason = 'manual' AND active_until IS NULL`, symbol)
	if err != nil {
		return false, fmt.Errorf("stop compute %s: %w", symbol, err)
	}
	return tag.RowsAffected() > 0, nil
}

func openReason(ctx context.Context, tx pgx.Tx, symbol, assetType, reason string) error {
	if _, err := tx.Exec(ctx, `
INSERT INTO computation_interest (symbol, asset_type, reason) VALUES (upper($1), $2, $3)
ON CONFLICT (symbol, reason) WHERE active_until IS NULL DO NOTHING`, symbol, assetType, reason); err != nil {
		return fmt.Errorf("open %s reason %s: %w", reason, symbol, err)
	}
	return nil
}

func closeReason(ctx context.Context, tx pgx.Tx, symbol, reason string) error {
	if _, err := tx.Exec(ctx, `
UPDATE computation_interest SET active_until = now()
WHERE symbol = upper($1) AND reason = $2 AND active_until IS NULL`, symbol, reason); err != nil {
		return fmt.Errorf("close %s reason %s: %w", reason, symbol, err)
	}
	return nil
}

// DirectoryMatch is one "all symbols" search result.
type DirectoryMatch struct {
	Symbol     string
	Name       *string
	Type       *string
	MIC        *string
	AssetType  string
	Source     string
	InUniverse bool // an eligible scanner symbol (the other search covers it)
	Followed   bool
}

// SearchDirectory searches symbol_directory by ticker prefix or name word
// prefix, skipping asset type other (never computed). Exact ticker first.
func SearchDirectory(ctx context.Context, q Querier, query string, limit int) ([]DirectoryMatch, error) {
	query = strings.TrimSpace(query)
	esc := likeEscaper.Replace(query)
	rows, err := q.Query(ctx, `
SELECT d.symbol, d.name, d.type, d.mic, d.asset_type, d.source,
       EXISTS (SELECT 1 FROM universe_symbols u WHERE u.symbol = d.symbol AND u.is_eligible),
       EXISTS (SELECT 1 FROM followed_symbols f WHERE f.symbol = d.symbol)
FROM symbol_directory d
WHERE d.asset_type <> 'other'
  AND (d.symbol LIKE upper($1) || '%' OR lower(d.name) LIKE lower($1) || '%' OR lower(d.name) LIKE '% ' || lower($1) || '%')
ORDER BY (d.symbol = upper($2)) DESC, (d.symbol LIKE upper($1) || '%') DESC, length(d.symbol), d.symbol
LIMIT $3`, esc, query, limit)
	if err != nil {
		return nil, fmt.Errorf("search directory %q: %w", query, err)
	}
	defer rows.Close()
	out := []DirectoryMatch{}
	for rows.Next() {
		var m DirectoryMatch
		if err := rows.Scan(&m.Symbol, &m.Name, &m.Type, &m.MIC, &m.AssetType, &m.Source, &m.InUniverse, &m.Followed); err != nil {
			return nil, fmt.Errorf("scan directory match: %w", err)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// ComputedRow is one symbol with at least one open computation reason.
type ComputedRow struct {
	Symbol      string
	AssetType   string
	Reasons     []string
	ManualSince *time.Time // open manual request, if any
	// QueuedSince is the newest open manual or watchlist reason: both queue a
	// fetch and a computation (data-technical / data-fundamental, momentum-daily).
	QueuedSince           *time.Time
	BarsFetchedAt         *time.Time
	FundamentalsFetchedAt *time.Time
	ComputedAt            *time.Time
	LastError             *string
	StatementsStatus      *string
	StatementsReason      *string
	Name                  *string
}

func ComputedSymbols(ctx context.Context, q Querier) ([]ComputedRow, error) {
	rows, err := q.Query(ctx, `
SELECT ci.symbol, min(ci.asset_type), array_agg(DISTINCT ci.reason ORDER BY ci.reason),
       max(ci.active_from) FILTER (WHERE ci.reason = 'manual'),
       max(ci.active_from) FILTER (WHERE ci.reason IN ('manual', 'watchlist')),
       s.bars_fetched_at, s.fundamentals_fetched_at, s.computed_at, s.last_error,
       s.statements_status, s.statements_reason, `+fmt.Sprintf(symbolNameSQL, "ci.symbol")+`
FROM computation_interest ci
LEFT JOIN symbol_data_status s ON s.symbol = ci.symbol
WHERE ci.active_until IS NULL
GROUP BY ci.symbol, s.bars_fetched_at, s.fundamentals_fetched_at, s.computed_at, s.last_error,
         s.statements_status, s.statements_reason
ORDER BY ci.symbol`)
	if err != nil {
		return nil, fmt.Errorf("computed symbols: %w", err)
	}
	defer rows.Close()
	out := []ComputedRow{}
	for rows.Next() {
		var r ComputedRow
		if err := rows.Scan(&r.Symbol, &r.AssetType, &r.Reasons, &r.ManualSince, &r.QueuedSince, &r.BarsFetchedAt,
			&r.FundamentalsFetchedAt, &r.ComputedAt, &r.LastError, &r.StatementsStatus, &r.StatementsReason, &r.Name); err != nil {
			return nil, fmt.Errorf("scan computed symbol: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
