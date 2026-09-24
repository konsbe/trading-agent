//go:build integration

package testdb

import (
	"context"
	"strings"
	"testing"
)

// The rule, enforced: a write outside Tx cannot land, and Tx's writes never
// outlive the test.
func TestPoolIsReadOnlyAndTxAlwaysRollsBack(t *testing.T) {
	ctx := context.Background()
	pool := Pool(t)
	_, err := pool.Exec(ctx, `INSERT INTO watchlist_items (symbol) VALUES ('ZZRO01')`)
	if err == nil || !strings.Contains(err.Error(), "read-only") {
		t.Fatalf("a write on the pool = %v; want a read-only-transaction error", err)
	}

	t.Run("write inside Tx", func(t *testing.T) {
		tx := Tx(t)
		if _, err := tx.Exec(ctx, `INSERT INTO watchlist_items (symbol) VALUES ('ZZRO02')`); err != nil {
			t.Fatalf("write in Tx: %v", err)
		}
	})
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM watchlist_items WHERE symbol IN ('ZZRO01','ZZRO02')`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("%d fixture rows survived; Tx must roll back when its test ends", n)
	}
}
