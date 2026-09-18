package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// CatalystEvent is one §3.11 keyword match on one headline.
//
// The grain is deliberately (symbol, headline, keyword) rather than
// (symbol, tier): §3.11 requires storing EVERY matched event, not just the
// winning tier, because "Phase 2's most valuable analysis is which specific
// keywords preceded real runners, and that requires the raw matches retained."
// A row per keyword is what makes that query possible.
type CatalystEvent struct {
	// TS is the candidate's bar date — the moment the catalyst is being
	// attributed to, not the article's publication time. Keeping it aligned with
	// the feature row is what lets a join answer "what was the catalyst for this
	// candidate".
	TS time.Time

	Symbol   string
	Source   string
	Headline string
	URL      string
	Keyword  string
	Tier     string

	// ScanTS is when the classification ran. Separate from TS so a re-run with a
	// revised vocabulary is distinguishable from the original pass.
	ScanTS time.Time
}

// headlineHash identifies an article without storing it twice.
//
// Hashing headline+URL rather than the headline alone: syndicated stories share
// a headline across outlets, and those are genuinely separate coverage events
// worth counting, while the same outlet re-serving one article is not.
func headlineHash(headline, url string) string {
	h := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(headline)) + "\x00" + strings.TrimSpace(url)))
	return hex.EncodeToString(h[:16])
}

const upsertCatalystSQL = `
INSERT INTO catalyst_events
    (ts, symbol, source, headline_hash, matched_keyword, tier, headline, url, scan_ts, ingested_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, now())
ON CONFLICT (symbol, ts, source, headline_hash, matched_keyword) DO UPDATE SET
    tier    = EXCLUDED.tier,
    scan_ts = EXCLUDED.scan_ts`

// UpsertCatalystEvent persists one keyword match, idempotently.
//
// Idempotent on the table's primary key (symbol, ts, source, headline_hash,
// matched_keyword) so re-running the classifier over the same window — which
// happens every time the vocabulary is tuned — updates the tier instead of
// multiplying rows. Without that, a keyword list edited three times would triple
// every candidate's apparent coverage.
//
// `source` is part of the key because syndicated coverage of one event across
// several outlets is genuinely several coverage events, and collapsing them
// would understate how widely a catalyst was reported.
func UpsertCatalystEvent(ctx context.Context, pool *pgxpool.Pool, e CatalystEvent) error {
	if e.Symbol == "" || e.Keyword == "" {
		return fmt.Errorf("catalyst event needs a symbol and a matched keyword")
	}
	scan := e.ScanTS
	if scan.IsZero() {
		scan = time.Now().UTC()
	}
	_, err := pool.Exec(ctx, upsertCatalystSQL,
		e.TS, e.Symbol, e.Source, headlineHash(e.Headline, e.URL),
		e.Keyword, e.Tier, e.Headline, e.URL, scan)
	if err != nil {
		return fmt.Errorf("upsert catalyst event %s/%s: %w", e.Symbol, e.Keyword, err)
	}
	return nil
}

// CatalystTierAt returns the highest tier recorded for a symbol on a date, or
// "none" when nothing was matched.
//
// Highest rather than most recent: §3.11 classifies a window into its strongest
// signal, so a window containing both an upgrade and an acquisition is tier A.
// Ordering by tier text happens to work because 'A' < 'B' lexically, but that is
// a coincidence not worth relying on, hence the explicit CASE.
func CatalystTierAt(ctx context.Context, pool *pgxpool.Pool, symbol string, ts time.Time) (string, error) {
	const q = `
SELECT tier FROM catalyst_events
WHERE symbol = $1 AND ts::date = $2::date
ORDER BY CASE tier WHEN 'A' THEN 2 WHEN 'B' THEN 1 ELSE 0 END DESC
LIMIT 1`
	var tier string
	err := pool.QueryRow(ctx, q, symbol, ts).Scan(&tier)
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			return "none", nil
		}
		return "", fmt.Errorf("catalyst tier %s: %w", symbol, err)
	}
	return tier, nil
}
