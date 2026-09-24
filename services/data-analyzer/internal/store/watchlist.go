package store

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

// Watchlist storage (migration 024). The only table momentum-api writes.
//
// owner is the identity provider's subject for a signed-in user, or nil for the
// shared unauthenticated list. There is no auth yet, so every caller passes nil.

// Execer is the write half of pgxpool.Pool / pgx.Tx.
type Execer interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// ReadWriter can both query and execute.
type ReadWriter interface {
	Querier
	Execer
}

type WatchlistItem struct {
	Symbol      string
	CompanyName *string
	Exchange    *string
	AddedAt     time.Time

	// Latest scanner facts for the symbol, from its most recent
	// momentum_features row — with no gates_passed filter, since a watched
	// symbol is usually not a candidate. All nil when the scanner has never
	// written a row for it (e.g. not in the eligible universe). AsOf is that
	// row's date, so an old price is never shown as current.
	AsOf      *time.Time
	Close     *float64
	ChangePct *float64
	RVol20    *float64
}

// ownerKey is the value the unique index compares on: ” for the
// unauthenticated list.
func ownerArg(owner *string) any {
	if owner == nil {
		return nil
	}
	return *owner
}

func ListWatchlist(ctx context.Context, q Querier, owner *string) ([]WatchlistItem, error) {
	rows, err := q.Query(ctx, `
SELECT w.symbol, u.name, u.exchange, w.added_at,
       mf.ts, mf.close, mf.change_pct, mf.rvol_20
FROM watchlist_items w
LEFT JOIN universe_symbols u ON u.symbol = w.symbol
LEFT JOIN LATERAL (
    SELECT f.ts, f.close, f.change_pct, f.rvol_20
    FROM momentum_features f
    WHERE f.symbol = w.symbol
    ORDER BY f.ts DESC
    LIMIT 1
) mf ON true
WHERE w.owner_sub IS NOT DISTINCT FROM $1
ORDER BY w.added_at DESC, w.symbol`, ownerArg(owner))
	if err != nil {
		return nil, fmt.Errorf("list watchlist: %w", err)
	}
	defer rows.Close()
	out := []WatchlistItem{}
	for rows.Next() {
		var it WatchlistItem
		if err := rows.Scan(&it.Symbol, &it.CompanyName, &it.Exchange, &it.AddedAt,
			&it.AsOf, &it.Close, &it.ChangePct, &it.RVol20); err != nil {
			return nil, fmt.Errorf("scan watchlist item: %w", err)
		}
		it.AddedAt = it.AddedAt.UTC()
		if it.AsOf != nil {
			d := it.AsOf.UTC()
			it.AsOf = &d
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

// SymbolKnown reports whether the symbol exists in universe_symbols (eligible
// or not): the watchlist only accepts symbols the system has heard of.
func SymbolKnown(ctx context.Context, q Querier, symbol string) (bool, error) {
	var ok bool
	if err := q.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM universe_symbols WHERE symbol = upper($1))`, symbol).Scan(&ok); err != nil {
		return false, fmt.Errorf("symbol known %s: %w", symbol, err)
	}
	return ok, nil
}

// AddToWatchlist is idempotent; added reports whether a new row was written.
func AddToWatchlist(ctx context.Context, db Execer, owner *string, symbol string) (added bool, err error) {
	tag, err := db.Exec(ctx, `
INSERT INTO watchlist_items (owner_sub, symbol) VALUES ($1, upper($2))
ON CONFLICT (COALESCE(owner_sub, ''), symbol) DO NOTHING`, ownerArg(owner), strings.TrimSpace(symbol))
	if err != nil {
		return false, fmt.Errorf("add to watchlist %s: %w", symbol, err)
	}
	return tag.RowsAffected() == 1, nil
}

// RemoveFromWatchlist is idempotent; removed reports whether a row existed.
func RemoveFromWatchlist(ctx context.Context, db Execer, owner *string, symbol string) (removed bool, err error) {
	tag, err := db.Exec(ctx, `
DELETE FROM watchlist_items WHERE owner_sub IS NOT DISTINCT FROM $1 AND symbol = upper($2)`,
		ownerArg(owner), strings.TrimSpace(symbol))
	if err != nil {
		return false, fmt.Errorf("remove from watchlist %s: %w", symbol, err)
	}
	return tag.RowsAffected() > 0, nil
}

// WatchlistSymbols returns every symbol on any owner's list — the intraday
// ingestion job fetches bars for these alongside the day's candidates.
func WatchlistSymbols(ctx context.Context, q Querier) ([]string, error) {
	rows, err := q.Query(ctx, `SELECT DISTINCT symbol FROM watchlist_items ORDER BY symbol`)
	if err != nil {
		return nil, fmt.Errorf("watchlist symbols: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// SymbolMatch is one symbol-search result.
type SymbolMatch struct {
	Symbol      string
	CompanyName *string
	Exchange    *string
	IsEligible  bool
}

// likeEscaper makes user input literal inside LIKE / ILIKE patterns.
var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

// SearchSymbols finds universe_symbols rows whose ticker starts with q, or
// whose company name starts with q or has a word starting with q. It searches
// every row, eligible or not — the same set PUT /watchlist accepts — so a
// result can always be added. Exact ticker first, then eligible, then shorter
// tickers.
func SearchSymbols(ctx context.Context, q Querier, query string, limit int) ([]SymbolMatch, error) {
	query = strings.TrimSpace(query)
	esc := likeEscaper.Replace(query)
	rows, err := q.Query(ctx, `
SELECT symbol, name, exchange, is_eligible
FROM universe_symbols
WHERE symbol LIKE upper($1) || '%'
   OR name ILIKE $1 || '%'
   OR name ILIKE '% ' || $1 || '%'
ORDER BY (symbol = upper($2)) DESC, is_eligible DESC, (symbol LIKE upper($1) || '%') DESC,
         length(symbol), symbol
LIMIT $3`, esc, query, limit)
	if err != nil {
		return nil, fmt.Errorf("search symbols %q: %w", query, err)
	}
	defer rows.Close()
	out := []SymbolMatch{}
	for rows.Next() {
		var m SymbolMatch
		if err := rows.Scan(&m.Symbol, &m.CompanyName, &m.Exchange, &m.IsEligible); err != nil {
			return nil, fmt.Errorf("scan symbol match: %w", err)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
