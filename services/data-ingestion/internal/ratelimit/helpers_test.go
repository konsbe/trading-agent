package ratelimit

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// nonNilPool returns a pool value that is non-nil but never dialled, so
// constructor validation can be exercised without a database.
func nonNilPool() *pgxpool.Pool {
	cfg, err := pgxpool.ParseConfig("postgres://unused:unused@127.0.0.1:1/unused?sslmode=disable")
	if err != nil {
		panic(err)
	}
	// LazyConnect semantics: pgxpool.NewWithConfig does not dial on construction,
	// so this yields a usable handle that fails on first query.
	p, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		panic(err)
	}
	return p
}

// unreachablePool returns a pool pointed at a closed port. Queries fail fast,
// which is what the degradation tests need: a coordination outage that is
// detected rather than one that hangs.
func unreachablePool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	p := nonNilPool()
	t.Cleanup(p.Close)
	return p
}
