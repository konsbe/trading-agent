package store

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/konsbe/trading-agent/services/data-analyzer/internal/momentum"
)

// Durable progress of the daily chain (migration 025). Nothing here relies on a
// process staying up: each step's completion is a row, written where the step's
// own writes commit, so a killed step leaves no marker and is simply run again.

// TxBeginner is satisfied by *pgxpool.Pool and by pgx.Tx (where Begin opens a
// savepoint, which is what lets the integration tests run inside a fixture).
type TxBeginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

// ChainRun is one session's row in momentum_chain_runs.
type ChainRun struct {
	Session            time.Time
	Attempts           int
	ScannerCompletedAt *time.Time
	TrackerCompletedAt *time.Time
	GaveUpAt           *time.Time
	LastError          *string
}

// ScanWrite is one symbol's scanner output. Score is nil when the symbol did
// not pass the gates (or could not be scored) this run.
type ScanWrite struct {
	Feature FeatureRow
	Score   *momentum.Score
}

// WriteScan commits a whole scan atomically, together with its completion
// marker. Written row by row on the pool, a scanner killed part-way used to
// leave a partial scan that already looked fresh — max(momentum_features.ts)
// moved on the first write — which the bot would have alerted on and then
// marked as alerted. In one transaction a kill rolls everything back.
//
// A symbol that does not pass this run has any earlier score for the same bar
// deleted, so re-running a session replaces its candidate set instead of
// accumulating one (market caps can change between runs).
func WriteScan(ctx context.Context, db TxBeginner, session time.Time, rows []ScanWrite) error {
	tx, err := db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("write scan: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }() // no-op after Commit

	for _, r := range rows {
		if err := UpsertFeatures(ctx, tx, r.Feature); err != nil {
			return fmt.Errorf("write scan: %w", err)
		}
		if r.Score != nil {
			if err := UpsertScore(ctx, tx, r.Feature.TS, r.Feature.Symbol, *r.Score); err != nil {
				return fmt.Errorf("write scan: %w", err)
			}
			continue
		}
		if _, err := tx.Exec(ctx, `DELETE FROM momentum_scores WHERE ts = $1 AND symbol = $2`,
			r.Feature.TS, r.Feature.Symbol); err != nil {
			return fmt.Errorf("write scan: clear stale score %s: %w", r.Feature.Symbol, err)
		}
	}
	if err := MarkScannerCompleted(ctx, tx, session); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("write scan: commit: %w", err)
	}
	return nil
}

const upsertChainRunSQL = `
INSERT INTO momentum_chain_runs (session, %[1]s, updated_at)
VALUES ($1, %[2]s, now())
ON CONFLICT (session) DO UPDATE SET %[1]s = EXCLUDED.%[1]s, updated_at = now()`

// MarkScannerCompleted records that the session's scan committed. Call it
// inside the scan's transaction (WriteScan does).
func MarkScannerCompleted(ctx context.Context, ex Execer, session time.Time) error {
	if _, err := ex.Exec(ctx, fmt.Sprintf(upsertChainRunSQL, "scanner_completed_at", "now()"), session); err != nil {
		return fmt.Errorf("mark scanner completed %s: %w", session.Format(time.DateOnly), err)
	}
	return nil
}

// MarkTrackerCompleted records that every active row was evaluated and every
// candidate opened for the session.
func MarkTrackerCompleted(ctx context.Context, ex Execer, session time.Time) error {
	if _, err := ex.Exec(ctx, fmt.Sprintf(upsertChainRunSQL, "tracker_completed_at", "now()"), session); err != nil {
		return fmt.Errorf("mark tracker completed %s: %w", session.Format(time.DateOnly), err)
	}
	return nil
}

// BeginChainAttempt counts one more attempt at the session and returns the new
// total, so momentum-daily's retry limit holds across restarts.
func BeginChainAttempt(ctx context.Context, q Querier, session time.Time) (int, error) {
	var n int
	err := q.QueryRow(ctx, `
INSERT INTO momentum_chain_runs (session, attempts, updated_at) VALUES ($1, 1, now())
ON CONFLICT (session) DO UPDATE SET attempts = momentum_chain_runs.attempts + 1, updated_at = now()
RETURNING attempts`, session).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("begin chain attempt %s: %w", session.Format(time.DateOnly), err)
	}
	return n, nil
}

// RecordChainError stores why the last attempt failed.
func RecordChainError(ctx context.Context, ex Execer, session time.Time, msg string) error {
	if _, err := ex.Exec(ctx, fmt.Sprintf(upsertChainRunSQL, "last_error", "$2"), session, msg); err != nil {
		return fmt.Errorf("record chain error %s: %w", session.Format(time.DateOnly), err)
	}
	return nil
}

// MarkChainGaveUp records a give-up and its reason; it is final for the session.
func MarkChainGaveUp(ctx context.Context, ex Execer, session time.Time, reason string) error {
	_, err := ex.Exec(ctx, `
INSERT INTO momentum_chain_runs (session, gave_up_at, last_error, updated_at) VALUES ($1, now(), $2, now())
ON CONFLICT (session) DO UPDATE SET gave_up_at = now(), last_error = EXCLUDED.last_error, updated_at = now()`,
		session, reason)
	if err != nil {
		return fmt.Errorf("mark chain gave up %s: %w", session.Format(time.DateOnly), err)
	}
	return nil
}

// LoadChainRun returns the session's row; ok=false when none exists yet.
func LoadChainRun(ctx context.Context, q Querier, session time.Time) (ChainRun, bool, error) {
	var r ChainRun
	err := q.QueryRow(ctx, `
SELECT session, attempts, scanner_completed_at, tracker_completed_at, gave_up_at, last_error
FROM momentum_chain_runs WHERE session = $1`, session).
		Scan(&r.Session, &r.Attempts, &r.ScannerCompletedAt, &r.TrackerCompletedAt, &r.GaveUpAt, &r.LastError)
	if errors.Is(err, pgx.ErrNoRows) {
		return ChainRun{}, false, nil
	}
	if err != nil {
		return ChainRun{}, false, fmt.Errorf("load chain run %s: %w", session.Format(time.DateOnly), err)
	}
	return r, true, nil
}

// ScanSession is the session a scan or tracker run covers: the most common
// latest-bar date across symbols (latest date on a tie). Not the maximum — a
// handful of freshly backfilled symbols can carry a newer bar than the rest of
// the universe, as 19 did on 2026-09-24 while everything else stopped at 09-21.
func ScanSession(latest []time.Time) (time.Time, bool) {
	if len(latest) == 0 {
		return time.Time{}, false
	}
	counts := map[time.Time]int{}
	for _, ts := range latest {
		y, m, d := ts.UTC().Date()
		counts[time.Date(y, m, d, 0, 0, 0, 0, time.UTC)]++
	}
	days := make([]time.Time, 0, len(counts))
	for d := range counts {
		days = append(days, d)
	}
	sort.Slice(days, func(i, j int) bool {
		if counts[days[i]] != counts[days[j]] {
			return counts[days[i]] > counts[days[j]]
		}
		return days[i].After(days[j])
	})
	return days[0], true
}
