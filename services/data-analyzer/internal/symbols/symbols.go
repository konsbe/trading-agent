// Package symbols resolves which symbols a data-analyzer worker processes, from
// Postgres (migration 030), each time it runs a pass:
//
//   - Followed: followed_symbols — what the user chose to follow.
//   - Computation: followed_symbols ∪ every symbol with an open
//     computation_interest reason (watchlist, today's candidates, manual
//     "Compute" requests) — what gets fetched so it can be analysed.
//
// Both are filtered by asset type (equity, etf, crypto). The worker's .env list
// is used ONLY when the table is unreachable or the result is empty, and every
// such use is logged at ERROR / WARN naming the service and the list, so a
// fallback is never silent.
package symbols

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/jackc/pgx/v5"
)

// Querier is the read half of pgxpool.Pool / pgx.Tx.
type Querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

const followedSQL = `
SELECT symbol FROM followed_symbols WHERE asset_type = ANY($1)
ORDER BY added_at, symbol`

const computationSQL = `
SELECT symbol FROM followed_symbols WHERE asset_type = ANY($1)
UNION
SELECT symbol FROM computation_interest WHERE active_until IS NULL AND asset_type = ANY($1)
ORDER BY 1`

// newsAliasSQL: the coin symbol news feeds use for followed crypto pairs
// (RENDERUSDT → RNDR), the pair itself when no alias is set.
const newsAliasSQL = `
SELECT COALESCE(news_alias, symbol) FROM followed_symbols WHERE asset_type = 'crypto'
ORDER BY added_at, symbol`

// Followed returns followed_symbols of the given asset types.
func Followed(ctx context.Context, q Querier, log *slog.Logger, service, list string, assetTypes []string, fallback []string) []string {
	return resolve(ctx, q, log, service, list, followedSQL, []any{assetTypes}, assetTypes, fallback)
}

// Computation returns followed_symbols ∪ open computation_interest symbols of
// the given asset types.
func Computation(ctx context.Context, q Querier, log *slog.Logger, service, list string, assetTypes []string, fallback []string) []string {
	return resolve(ctx, q, log, service, list, computationSQL, []any{assetTypes}, assetTypes, fallback)
}

// CryptoNewsAliases returns the news/sentiment coin symbols of followed crypto.
func CryptoNewsAliases(ctx context.Context, q Querier, log *slog.Logger, service, list string, fallback []string) []string {
	return resolve(ctx, q, log, service, list, newsAliasSQL, nil, []string{"crypto"}, fallback)
}

func resolve(ctx context.Context, q Querier, log *slog.Logger, service, list, sql string, args []any, assetTypes, fallback []string) []string {
	out, err := query(ctx, q, sql, args)
	switch {
	case err != nil:
		log.Error("SYMBOL LIST FALLBACK: followed_symbols unreachable — using the .env list instead",
			"service", service, "list", list, "err", err, "fallback_symbols", len(fallback))
		return fallback
	case len(out) == 0:
		log.Warn("SYMBOL LIST FALLBACK: followed_symbols has no rows for this list — using the .env list instead",
			"service", service, "list", list, "asset_types", assetTypes, "fallback_symbols", len(fallback))
		return fallback
	}
	return out
}

func query(ctx context.Context, q Querier, sql string, args []any) ([]string, error) {
	if q == nil {
		return nil, errors.New("no database connection")
	}
	rows, err := q.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		out = append(out, strings.ToUpper(strings.TrimSpace(s)))
	}
	return out, rows.Err()
}
