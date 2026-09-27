package store

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DirectoryRow is one provider directory entry (migration 030 symbol_directory):
// every symbol a provider lists, kept for the "all symbols" search, whether or
// not the scanner's universe includes it.
type DirectoryRow struct {
	Symbol        string
	DisplaySymbol string
	Name          string
	Type          string
	MIC           string
	Currency      string
	AssetType     string // equity | etf | crypto | other
}

// DirectoryAssetType maps a Finnhub /stock/symbol type to the pipeline asset
// type. Funds (ETP, closed-end fund) are etf: bars, technicals and Finnhub
// metrics, no financial statements. Warrants, units, rights, preferreds
// (Finnhub "PUBLIC") and blank types are other: listed, never computed.
func DirectoryAssetType(providerType string) string {
	switch strings.TrimSpace(providerType) {
	case "Common Stock", "ADR", "GDR", "REIT", "NY Reg Shrs", "MLP", "Ltd Part", "Royalty Trst":
		return "equity"
	case "ETP", "ETF", "Closed-End Fund":
		return "etf"
	}
	return "other"
}

const upsertDirectorySQL = `
INSERT INTO symbol_directory (source, symbol, display_symbol, name, type, mic, currency, asset_type, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, now())
ON CONFLICT (source, symbol) DO UPDATE SET
    display_symbol = EXCLUDED.display_symbol, name = EXCLUDED.name, type = EXCLUDED.type,
    mic = EXCLUDED.mic, currency = EXCLUDED.currency, asset_type = EXCLUDED.asset_type,
    updated_at = now()`

// UpsertDirectory writes one provider's directory in a single transaction.
// Rows are upserted, never deleted: a symbol the provider stops listing keeps
// its last entry (its updated_at shows when it was last seen).
func UpsertDirectory(ctx context.Context, pool *pgxpool.Pool, source string, rows []DirectoryRow) (int, error) {
	if len(rows) == 0 {
		return 0, nil
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin directory tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck — no-op after Commit
	batch := &pgx.Batch{}
	seen := map[string]bool{}
	n := 0
	for _, r := range rows {
		sym := strings.ToUpper(strings.TrimSpace(r.Symbol))
		if sym == "" || seen[sym] {
			continue
		}
		seen[sym] = true
		batch.Queue(upsertDirectorySQL, source, sym, nullable(r.DisplaySymbol), nullable(r.Name),
			nullable(r.Type), nullable(r.MIC), nullable(r.Currency), r.AssetType)
		n++
	}
	br := tx.SendBatch(ctx, batch)
	for i := 0; i < n; i++ {
		if _, err := br.Exec(); err != nil {
			br.Close() //nolint:errcheck
			return 0, fmt.Errorf("upsert directory %s row %d: %w", source, i, err)
		}
	}
	if err := br.Close(); err != nil {
		return 0, fmt.Errorf("close directory batch: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit directory: %w", err)
	}
	return n, nil
}
