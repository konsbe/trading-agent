//go:build integration

package store

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func lbPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	p, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(p.Close)
	return p
}

// The lockbox exclusion must be evaluated as a REGION, not against the frozen
// row list in phase2_lockbox.
//
// The row list was reserved under gate v1. Gate v2 changes which symbol-days
// pass §3.2 — 42.3% of the candidate union changes membership — so a query that
// excludes only those 1,783 rows lets NEW gate-v2 candidates inside the held-out
// window through into training. That is a silent leak of exactly the data the
// lockbox exists to protect, and it leaves no trace in any count.
func TestLockboxExclusionIsByRegionNotRowList(t *testing.T) {
	ctx := context.Background()
	pool := lbPool(t)

	var start, end, cohort string
	err := pool.QueryRow(ctx, `
SELECT start_date::text, end_date::text, exclude_cohort
FROM phase2_lockbox_region WHERE region_key = 'phase2_lockbox_v2'`).
		Scan(&start, &end, &cohort)
	if err != nil {
		t.Fatalf("the region definition must exist before any dataset query can exclude it: %v", err)
	}
	if start != "2025-03-28" || end != "2026-03-27" {
		t.Errorf("region window = %s..%s, want 2025-03-28..2026-03-27 — the boundaries were reserved and must not move", start, end)
	}
	if cohort != "phase1_pilot_450" {
		t.Errorf("exclude_cohort = %q, want phase1_pilot_450; without the in-sample exclusion the lockbox contains symbols score v2 was fitted on", cohort)
	}

	// The region must cover strictly more than the old row list within its own
	// window, under ANY gate version. If it did not, the redefinition would
	// have narrowed the held-out data rather than made it gate-independent.
	var rowListInWindow, regionSymbols int
	if err := pool.QueryRow(ctx, `
SELECT count(*) FROM phase2_lockbox
WHERE ts BETWEEN $1::date AND $2::date`, start, end).Scan(&rowListInWindow); err != nil {
		t.Fatalf("count row list: %v", err)
	}
	if err := pool.QueryRow(ctx, `
SELECT count(*) FROM universe_symbols u
WHERE u.is_eligible
  AND NOT EXISTS (SELECT 1 FROM momentum_pilot_cohort c WHERE c.symbol = u.symbol)`).
		Scan(&regionSymbols); err != nil {
		t.Fatalf("count region symbols: %v", err)
	}
	if regionSymbols == 0 {
		t.Fatal("region covers no symbols; the pilot-cohort exclusion has swallowed the universe")
	}
	t.Logf("region covers %d non-pilot eligible symbols; the superseded row list held %d rows in the same window",
		regionSymbols, rowListInWindow)
}

// The pilot cohort and the lockbox region must not overlap. A lockbox
// containing in-sample symbols is not held-out data, whatever it is called.
func TestLockboxRegionExcludesTheInSampleCohort(t *testing.T) {
	ctx := context.Background()
	pool := lbPool(t)

	var overlap int
	if err := pool.QueryRow(ctx, `
SELECT count(*)
FROM momentum_pilot_cohort c
JOIN universe_symbols u ON u.symbol = c.symbol
WHERE u.is_eligible
  AND NOT EXISTS (SELECT 1 FROM momentum_pilot_cohort p WHERE p.symbol = u.symbol)`).
		Scan(&overlap); err != nil {
		t.Fatalf("query: %v", err)
	}
	if overlap != 0 {
		t.Errorf("the region's symbol predicate admits %d pilot symbols; those are in-sample for score v2", overlap)
	}
}

// The superseded row list must still exist, and must be marked as superseded.
// Deleting it would destroy the audit trail for the original reservation;
// leaving it unmarked would invite a future query from using it for exclusion.
func TestSupersededRowListIsRetainedAndMarked(t *testing.T) {
	ctx := context.Background()
	pool := lbPool(t)

	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM phase2_lockbox`).Scan(&n); err != nil {
		t.Fatalf("query: %v", err)
	}
	if n != 1783 {
		t.Errorf("phase2_lockbox holds %d rows, want the original 1,783 — it is the audit record of the first reservation and must not be rewritten", n)
	}

	var comment string
	if err := pool.QueryRow(ctx,
		`SELECT obj_description('phase2_lockbox'::regclass, 'pg_class')`).Scan(&comment); err != nil {
		t.Fatalf("read comment: %v", err)
	}
	if comment == "" || !contains(comment, "SUPERSEDED") {
		t.Errorf("phase2_lockbox is not marked SUPERSEDED; an unmarked row list invites a future exclusion query to use it and leak gate-v2 candidates")
	}

	var supersedes string
	if err := pool.QueryRow(ctx,
		`SELECT supersedes FROM phase2_lockbox_region WHERE region_key='phase2_lockbox_v2'`).
		Scan(&supersedes); err != nil {
		t.Fatalf("read supersedes: %v", err)
	}
	// The OLD hash must be recorded, so the original reservation stays provable.
	if !contains(supersedes, "104cce0836a12af00242b31de3df5b53175febf8b5cf1024e40f74792feb495c") {
		t.Errorf("the superseded row list's content hash is not recorded in phase2_lockbox_region.supersedes; without it the original reservation cannot be verified after the fact: %q", supersedes)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}
