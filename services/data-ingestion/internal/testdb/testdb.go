// Package testdb is the single entry point integration tests in this module use
// to reach Postgres. It exists so the scratch-database guard cannot be applied
// in one package and forgotten in another: the guard used to live only inside
// internal/store's clearUniverse, and internal/ratelimit's tests — which had
// their own pool helper — ran against the live database and left 15 test_*
// budget rows behind in api_rate_budget (found 2026-09-24).
//
// Import it only from _test.go files.
package testdb

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Pool connects to TEST_DATABASE_URL, skipping the test when it is unset, and
// fails the test unless the database is a scratch one (RequireScratch). Every
// integration test in this module gets its pool here, so none can reach a
// populated database by accident.
func Pool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	RequireScratch(t, pool)
	return pool
}

// RequireScratch refuses to continue unless the connected database's NAME
// contains "test". Integration fixtures here truncate and seed real tables
// (universe_symbols, equity_ohlcv, api_rate_budget, ...), which is harmless on
// a scratch database and silently corrupting anywhere else.
//
// The check is on the name, not on row counts or fixture shapes: neither can
// separate the two cases (fixtures legitimately create thousands of rows and
// use bare tickers like AAPL). It is deliberately not an opt-in env flag — a
// flag gets set once and forgotten, at which point it protects nothing.
func RequireScratch(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	var name string
	if err := pool.QueryRow(context.Background(), `SELECT current_database()`).Scan(&name); err != nil {
		t.Fatalf("scratch-db guard: %v", err)
	}
	if !strings.Contains(strings.ToLower(name), "test") {
		t.Fatalf("refusing to run integration fixtures against database %q: they write to and "+
			"truncate real tables. Point TEST_DATABASE_URL at a database whose name contains "+
			"\"test\" (e.g. trading_test).", name)
	}
}
