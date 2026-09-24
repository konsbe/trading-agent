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
SELECT w.symbol, u.name, u.exchange, w.added_at
FROM watchlist_items w
LEFT JOIN universe_symbols u ON u.symbol = w.symbol
WHERE w.owner_sub IS NOT DISTINCT FROM $1
ORDER BY w.added_at DESC, w.symbol`, ownerArg(owner))
	if err != nil {
		return nil, fmt.Errorf("list watchlist: %w", err)
	}
	defer rows.Close()
	out := []WatchlistItem{}
	for rows.Next() {
		var it WatchlistItem
		if err := rows.Scan(&it.Symbol, &it.CompanyName, &it.Exchange, &it.AddedAt); err != nil {
			return nil, fmt.Errorf("scan watchlist item: %w", err)
		}
		it.AddedAt = it.AddedAt.UTC()
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
