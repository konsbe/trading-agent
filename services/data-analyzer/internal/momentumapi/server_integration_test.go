//go:build integration

package momentumapi

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// End to end: real SQL on a fixture scan, through the HTTP handlers, asserting
// on the JSON the web app receives. The fixture lives in a transaction that is
// always rolled back, so the live scan is never touched.

func TestIntegration_TodayAndDetailOverRealQueries(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })

	day := time.Date(2099, 1, 2, 0, 0, 0, 0, time.UTC)
	for i := 1; i <= 11; i++ {
		sym := fmt.Sprintf("ZZE%02d", i)
		if _, err := tx.Exec(ctx, `INSERT INTO universe_symbols (symbol, exchange, name, is_eligible) VALUES ($1, 'NYSE', $1, true)`, sym); err != nil {
			t.Fatal(err)
		}
		if _, err := tx.Exec(ctx, `
INSERT INTO momentum_features (ts, symbol, close, rvol_20, bucket, gates_passed)
VALUES ($1, $2, 10, $3, 'market', true)`, day, sym, float64(i)); err != nil {
			t.Fatal(err)
		}
		if i%2 == 0 { // odd-numbered candidates have no score row
			if _, err := tx.Exec(ctx, `
INSERT INTO momentum_scores (ts, symbol, bucket, momentum_score_100, null_inputs)
VALUES ($1, $2, 'market', $3, '{catalyst_tier}')`, day, sym, 50+i); err != nil {
				t.Fatal(err)
			}
		}
	}

	srv := NewServer(Config{
		Store:             DBStore{Q: tx, PingFn: pool.Ping},
		Caveats:           loadSharedCaveats(t),
		Log:               slog.New(slog.NewTextHandler(io.Discard, nil)),
		SessionReadyAfter: 6 * time.Hour,
		// A covered "now": the future fixture scan is simply newer than expected.
		Now: func() time.Time { return ny("2026-09-18 09:00") },
	})

	rec := get(t, srv, "/api/v1/scanner/today")
	if rec.Code != http.StatusOK {
		t.Fatalf("today: %d %s", rec.Code, rec.Body)
	}
	body := decode(t, rec)
	if body["scan"].(map[string]any)["date"] != "2099-01-02" {
		t.Fatalf("scan = %v", body["scan"])
	}
	market := body["buckets"].(map[string]any)["market"].(map[string]any)
	cands := market["candidates"].([]any)
	if market["total_candidates"] != 11.0 || len(cands) != 11 {
		t.Fatalf("market = %d candidates, want all 11 uncapped", len(cands))
	}
	if penny := body["buckets"].(map[string]any)["penny"].(map[string]any); penny["total_candidates"] != 0.0 {
		t.Errorf("penny = %v, want the empty state", penny)
	}
	for _, c := range cands {
		m := c.(map[string]any)
		if m["score_status"] != "unvalidated" {
			t.Errorf("%v missing score_status", m["symbol"])
		}
		score, present := m["momentum_score_100"]
		if !present {
			t.Errorf("%v: momentum_score_100 key missing", m["symbol"])
		}
		var n int
		fmt.Sscanf(m["symbol"].(string), "ZZE%d", &n)
		if n%2 == 1 && score != nil {
			t.Errorf("%v has no score row but momentum_score_100 = %v", m["symbol"], score)
		}
	}

	detailRec := get(t, srv, "/api/v1/scanner/today/zze04")
	if detailRec.Code != http.StatusOK {
		t.Fatalf("detail: %d %s", detailRec.Code, detailRec.Body)
	}
	detail := decode(t, detailRec)
	score := detail["score"].(map[string]any)
	if score["total"] != 54.0 || score["attainable"] != 75.0 || score["status"] != "unvalidated" {
		t.Errorf("detail score = %v", score)
	}
	if detail["evidence_note"] != loadSharedCaveats(t).Evidence {
		t.Error("evidence_note differs from the shared constant")
	}
}
