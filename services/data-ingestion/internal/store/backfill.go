package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/konsbe/trading-agent/services/data-ingestion/internal/buildinfo"
)

// Backfill status values for universe_symbols.backfill_status.
const (
	BackfillPending    = "pending"
	BackfillInProgress = "in_progress"
	BackfillDone       = "done"
	BackfillFailed     = "failed"
)

// BackfillClaim is one symbol leased to this worker for bar backfill.
type BackfillClaim struct {
	Symbol   string
	Exchange string
	Attempts int
}

// ClaimBackfillBatch leases up to `limit` eligible symbols for backfill and
// marks them in_progress.
//
// Claimable rows, in priority order:
//
//  1. 'pending'     — never attempted
//  2. 'failed'      — attempted and failed, under maxAttempts
//  3. 'in_progress' — claimed longer ago than `lease`, i.e. abandoned by a
//     worker that was killed mid-symbol
//
// Case 3 is what makes the job resumable rather than merely restartable: a
// crash leaves rows in_progress, and without a lease those rows would never be
// picked up again and the backfill would silently finish incomplete.
//
// FOR UPDATE SKIP LOCKED makes concurrent claims safe. The design assumes one
// worker, but two would now split the work instead of duplicating it.
//
// selectedOnly restricts claims to the pilot subset (backfill_selected). The
// pilot and the full universe use different providers with different quotas, so
// running the backfill over the wrong population would spend the wrong budget.
func ClaimBackfillBatch(
	ctx context.Context,
	pool *pgxpool.Pool,
	limit int,
	lease time.Duration,
	maxAttempts int,
	selectedOnly bool,
) ([]BackfillClaim, error) {
	if limit <= 0 {
		return nil, nil
	}
	const q = `
WITH claimable AS (
    SELECT symbol, exchange
    FROM universe_symbols
    WHERE is_eligible
      -- Never claim a symbol the provider no longer serves: the fetch
      -- returns nothing, the row churns through attempts, and its stored
      -- bars stay a frozen remnant regardless. See migration 022.
      AND data_unavailable_reason IS NULL
      AND (NOT $4 OR backfill_selected)
      AND (
            backfill_status = 'pending'
         OR (backfill_status = 'failed'      AND backfill_attempts < $3)
         OR (backfill_status = 'in_progress' AND backfill_claimed_at < now() - $2::interval)
      )
    ORDER BY
        CASE backfill_status
            WHEN 'pending'     THEN 0
            WHEN 'in_progress' THEN 1
            WHEN 'failed'      THEN 2
            ELSE 3
        END,
        backfill_attempts,
        symbol
    LIMIT $1
    FOR UPDATE SKIP LOCKED
)
UPDATE universe_symbols u SET
    backfill_status         = 'in_progress',
    backfill_claimed_at     = now(),
    -- Stamped on CLAIM, not on completion: the point is to see which build is
    -- working the queue right now, including a build that claims rows and then
    -- dies. See migration 022 and internal/buildinfo.
    backfill_worker_version = $5,
    updated_at              = now()
FROM claimable c
WHERE u.symbol = c.symbol AND u.exchange = c.exchange
RETURNING u.symbol, u.exchange, u.backfill_attempts`

	rows, err := pool.Query(ctx, q, limit, lease, maxAttempts, selectedOnly, buildinfo.Version())
	if err != nil {
		return nil, fmt.Errorf("claim backfill batch: %w", err)
	}
	defer rows.Close()
	var out []BackfillClaim
	for rows.Next() {
		var c BackfillClaim
		if err := rows.Scan(&c.Symbol, &c.Exchange, &c.Attempts); err != nil {
			return nil, fmt.Errorf("scan backfill claim: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// MarkBackfillDone records a successful backfill.
//
// cursorTS is the oldest bar actually stored, so the recorded checkpoint
// reflects the data rather than the request window — a symbol that IPO'd 8
// months ago gets its true first bar, not the 3-years-ago window start.
//
// A symbol Yahoo has no data for is also 'done' with zero bars: it is a
// delisting or a bad ticker, and retrying it forever would waste the rate
// budget that the rest of the universe needs.
func MarkBackfillDone(
	ctx context.Context,
	pool *pgxpool.Pool,
	symbol, exchange string,
	barsStored int,
	cursorTS *time.Time,
) error {
	const q = `
UPDATE universe_symbols SET
    backfill_status       = 'done',
    backfill_cursor_ts    = COALESCE($3, backfill_cursor_ts),
    backfill_completed_at = now(),
    backfill_last_error   = CASE WHEN $4 = 0 THEN 'no_bars_returned' ELSE NULL END,
    backfill_claimed_at   = NULL,
    updated_at            = now()
WHERE symbol = $1 AND exchange = $2`
	_, err := pool.Exec(ctx, q, symbol, exchange, cursorTS, barsStored)
	if err != nil {
		return fmt.Errorf("mark backfill done %s: %w", symbol, err)
	}
	return nil
}

// MarkBackfillFailed records a failed attempt and increments the attempt count.
//
// The error text is persisted so a stalled backfill is diagnosable from SQL
// alone rather than by correlating logs. Attempts increment so
// ClaimBackfillBatch can stop retrying a permanently broken symbol.
func MarkBackfillFailed(ctx context.Context, pool *pgxpool.Pool, symbol, exchange, reason string) error {
	const q = `
UPDATE universe_symbols SET
    backfill_status     = 'failed',
    backfill_attempts   = backfill_attempts + 1,
    backfill_last_error = $3,
    backfill_claimed_at = NULL,
    updated_at          = now()
WHERE symbol = $1 AND exchange = $2`
	if len(reason) > 500 {
		reason = reason[:500]
	}
	_, err := pool.Exec(ctx, q, symbol, exchange, reason)
	if err != nil {
		return fmt.Errorf("mark backfill failed %s: %w", symbol, err)
	}
	return nil
}

// BackfillProgress is the state of the backfill across the eligible universe.
type BackfillProgress struct {
	Eligible   int
	Pending    int
	InProgress int
	Done       int
	Failed     int

	// Exhausted counts failed symbols that have hit maxAttempts and will no
	// longer be claimed. These are the ones needing human attention.
	Exhausted int

	// WithEnoughBars counts symbols meeting the §3.2 bar minimum — the number
	// that actually matters, since 'done' only means "we asked", not "there was
	// enough history".
	WithEnoughBars int
}

func LoadBackfillProgress(ctx context.Context, pool *pgxpool.Pool, maxAttempts, minBars int) (BackfillProgress, error) {
	var p BackfillProgress
	const q = `
SELECT count(*),
       count(*) FILTER (WHERE backfill_status = 'pending'),
       count(*) FILTER (WHERE backfill_status = 'in_progress'),
       count(*) FILTER (WHERE backfill_status = 'done'),
       count(*) FILTER (WHERE backfill_status = 'failed'),
       count(*) FILTER (WHERE backfill_status = 'failed' AND backfill_attempts >= $1),
       count(*) FILTER (WHERE bar_count >= $2)
FROM universe_symbols WHERE is_eligible`
	err := pool.QueryRow(ctx, q, maxAttempts, minBars).Scan(
		&p.Eligible, &p.Pending, &p.InProgress, &p.Done, &p.Failed, &p.Exhausted, &p.WithEnoughBars)
	if err != nil {
		return p, fmt.Errorf("backfill progress: %w", err)
	}
	return p, nil
}

// ResetBackfill puts eligible symbols back to 'pending'.
//
// Used to force a re-backfill (e.g. after widening the date window). Not called
// on the normal path — the weekly symbol refresh deliberately preserves backfill
// state so it cannot accidentally restart a multi-hour job.
func ResetBackfill(ctx context.Context, pool *pgxpool.Pool) (int64, error) {
	const q = `
UPDATE universe_symbols SET
    backfill_status       = 'pending',
    backfill_claimed_at   = NULL,
    backfill_attempts     = 0,
    backfill_last_error   = NULL,
    backfill_completed_at = NULL,
    updated_at            = now()
WHERE is_eligible`
	ct, err := pool.Exec(ctx, q)
	if err != nil {
		return 0, fmt.Errorf("reset backfill: %w", err)
	}
	return ct.RowsAffected(), nil
}

// UpsertEquityOHLCVBatch upserts bars for one symbol in a single transaction.
//
// Transactional per symbol so a partially-written history is never visible: the
// feature engine computes windowed features (20-bar RVOL, 252-bar highs) and a
// half-loaded symbol would produce features that are wrong rather than absent,
// which is far harder to notice.
func UpsertEquityOHLCVBatch(ctx context.Context, pool *pgxpool.Pool, rows []EquityBar) (int64, error) {
	if len(rows) == 0 {
		return 0, nil
	}
	// COALESCE on the conflict path so a provider that does not report the
	// unadjusted fields cannot erase values another provider already captured.
	// Plain EXCLUDED.raw_close would overwrite a real number with NULL on every
	// refresh from a non-Tiingo source.
	const q = `
INSERT INTO equity_ohlcv (ts, symbol, interval, open, high, low, close, volume, source, raw_close, split_factor, div_cash)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
ON CONFLICT (symbol, interval, ts, source) DO UPDATE SET
  open = EXCLUDED.open, high = EXCLUDED.high, low = EXCLUDED.low,
  close = EXCLUDED.close, volume = EXCLUDED.volume,
  raw_close = COALESCE(EXCLUDED.raw_close, equity_ohlcv.raw_close),
  split_factor = COALESCE(EXCLUDED.split_factor, equity_ohlcv.split_factor),
  div_cash = COALESCE(EXCLUDED.div_cash, equity_ohlcv.div_cash)`

	tx, err := pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin bars tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck — no-op after Commit

	batch := &pgx.Batch{}
	for _, r := range rows {
		batch.Queue(q, r.TS, r.Symbol, r.Interval, r.Open, r.High, r.Low, r.Close, r.Volume, r.Source, r.RawClose, r.SplitFactor, r.DivCash)
	}
	br := tx.SendBatch(ctx, batch)
	var n int64
	for i := 0; i < len(rows); i++ {
		ct, err := br.Exec()
		if err != nil {
			br.Close() //nolint:errcheck
			return 0, fmt.Errorf("upsert bar %s @ %s: %w", rows[i].Symbol, rows[i].TS.Format(time.DateOnly), err)
		}
		n += ct.RowsAffected()
	}
	if err := br.Close(); err != nil {
		return 0, fmt.Errorf("close bars batch: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit bars tx: %w", err)
	}
	return n, nil
}

// SymbolBarBounds reports what history a symbol already has, so the daily
// incremental refresh can request only the missing tail.
type SymbolBarBounds struct {
	Symbol  string
	Count   int
	FirstTS *time.Time
	LastTS  *time.Time
}

// LoadBarBounds returns per-symbol bar coverage for the eligible universe.
func LoadBarBounds(ctx context.Context, pool *pgxpool.Pool, interval, source string) (map[string]SymbolBarBounds, error) {
	const q = `
SELECT u.symbol,
       COALESCE(b.n, 0),
       b.first_ts,
       b.last_ts
FROM universe_symbols u
LEFT JOIN (
    SELECT symbol, count(*) AS n, min(ts) AS first_ts, max(ts) AS last_ts
    FROM equity_ohlcv
    WHERE interval = $1 AND source = $2
    GROUP BY symbol
) b ON b.symbol = u.symbol
WHERE u.is_eligible`
	rows, err := pool.Query(ctx, q, interval, source)
	if err != nil {
		return nil, fmt.Errorf("load bar bounds: %w", err)
	}
	defer rows.Close()
	out := make(map[string]SymbolBarBounds)
	for rows.Next() {
		var b SymbolBarBounds
		if err := rows.Scan(&b.Symbol, &b.Count, &b.FirstTS, &b.LastTS); err != nil {
			return nil, fmt.Errorf("scan bar bounds: %w", err)
		}
		out[b.Symbol] = b
	}
	return out, rows.Err()
}

// QualityBar is one bar for the adjustment-quality audit.
type QualityBar struct {
	TS     time.Time
	Open   float64
	High   float64
	Low    float64
	Close  float64
	Volume float64
}

// LoadBarsBySource returns every stored bar for one source and interval, grouped
// by symbol and ordered chronologically, for the barquality audit.
//
// Loads whole series rather than streaming because the adjustment checks are
// close-to-close comparisons that need neighbouring bars; 450 symbols x ~750
// bars is a few hundred thousand rows, which is comfortable in memory and far
// cheaper than 450 round trips.
func LoadBarsBySource(ctx context.Context, pool *pgxpool.Pool, interval, source string, selectedOnly bool) (map[string][]QualityBar, error) {
	const q = `
SELECT o.symbol, o.ts, o.open, o.high, o.low, o.close, o.volume
FROM equity_ohlcv o
JOIN universe_symbols u ON u.symbol = o.symbol
WHERE o.interval = $1 AND o.source = $2 AND (NOT $3 OR u.backfill_selected)
ORDER BY o.symbol, o.ts`
	rows, err := pool.Query(ctx, q, interval, source, selectedOnly)
	if err != nil {
		return nil, fmt.Errorf("load bars by source: %w", err)
	}
	defer rows.Close()

	out := make(map[string][]QualityBar)
	for rows.Next() {
		var sym string
		var b QualityBar
		if err := rows.Scan(&sym, &b.TS, &b.Open, &b.High, &b.Low, &b.Close, &b.Volume); err != nil {
			return nil, fmt.Errorf("scan quality bar: %w", err)
		}
		out[sym] = append(out[sym], b)
	}
	return out, rows.Err()
}
