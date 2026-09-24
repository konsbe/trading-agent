// Package testdb is the single entry point this module's integration tests use
// to reach Postgres. Import it only from _test.go files.
//
// These tests deliberately run against the populated database
// (`make test-integration` defaults TEST_DATABASE_URL to DATABASE_URL), because
// their rule is that fixtures live in a transaction that is always rolled back.
// That rule used to be a convention, and one file broke it: the equity_ohlcv
// tests inserted and deleted committed rows on the pool, against live data
// (found 2026-09-24). Here it is enforced instead:
//
//   - Pool opens every session read-only (default_transaction_read_only), so a
//     write outside Tx fails with "read-only transaction" instead of landing.
//   - Tx is the only way to write: read-write, and rolled back when the test
//     ends. Nothing a test does can commit.
package testdb

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Pool connects to TEST_DATABASE_URL (skipping the test when unset) with every
// session read-only by default.
func Pool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("parse TEST_DATABASE_URL: %v", err)
	}
	cfg.ConnConfig.RuntimeParams["default_transaction_read_only"] = "on"
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// Tx opens a read-write transaction on a fresh Pool and rolls it back when the
// test ends. Fixtures go here and nowhere else.
func Tx(t *testing.T) pgx.Tx {
	t.Helper()
	ctx := context.Background()
	tx, err := Pool(t).BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadWrite})
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
	return tx
}
