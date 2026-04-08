package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type CryptoBar struct {
	TS       time.Time
	Exchange string
	Symbol   string
	Interval string
	Open     float64
	High     float64
	Low      float64
	Close    float64
	Volume   float64
	Source   string
}

func UpsertCryptoOHLCV(ctx context.Context, pool *pgxpool.Pool, rows []CryptoBar) error {
	if len(rows) == 0 {
		return nil
	}
	const q = `
INSERT INTO crypto_ohlcv (ts, exchange, symbol, interval, open, high, low, close, volume, source)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
ON CONFLICT (exchange, symbol, interval, ts, source) DO UPDATE SET
  open = EXCLUDED.open, high = EXCLUDED.high, low = EXCLUDED.low,
  close = EXCLUDED.close, volume = EXCLUDED.volume
`
	for _, r := range rows {
		if _, err := pool.Exec(ctx, q, r.TS, r.Exchange, r.Symbol, r.Interval, r.Open, r.High, r.Low, r.Close, r.Volume, r.Source); err != nil {
			return err
		}
	}
	return nil
}

func InsertCryptoGlobal(ctx context.Context, pool *pgxpool.Pool, ts time.Time, provider string, payload any) error {
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	const q = `
INSERT INTO crypto_global_metrics (ts, provider, payload) VALUES ($1,$2,$3)
ON CONFLICT (provider, ts) DO UPDATE SET payload = EXCLUDED.payload
`
	_, err = pool.Exec(ctx, q, ts, provider, b)
	return err
}

type EquityBar struct {
	TS       time.Time
	Symbol   string
	Interval string
	Open     float64
	High     float64
	Low      float64
	Close    float64
	Volume   float64
	Source   string
}

func UpsertEquityOHLCV(ctx context.Context, pool *pgxpool.Pool, rows []EquityBar) error {
	const q = `
INSERT INTO equity_ohlcv (ts, symbol, interval, open, high, low, close, volume, source)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
ON CONFLICT (symbol, interval, ts, source) DO UPDATE SET
  open = EXCLUDED.open, high = EXCLUDED.high, low = EXCLUDED.low,
  close = EXCLUDED.close, volume = EXCLUDED.volume
`
	for _, r := range rows {
		if _, err := pool.Exec(ctx, q, r.TS, r.Symbol, r.Interval, r.Open, r.High, r.Low, r.Close, r.Volume, r.Source); err != nil {
			return err
		}
	}
	return nil
}

func UpsertMacroFred(ctx context.Context, pool *pgxpool.Pool, seriesID string, ts time.Time, value float64) error {
	const q = `
INSERT INTO macro_fred (ts, series_id, value) VALUES ($1,$2,$3)
ON CONFLICT (series_id, ts) DO UPDATE SET value = EXCLUDED.value
`
	_, err := pool.Exec(ctx, q, ts, seriesID, value)
	return err
}

// MacroFredRow holds a single observation ready for bulk insert.
type MacroFredRow struct {
	TS    time.Time
	Value float64
}

// UpsertMacroFredBatch upserts all observations for one series inside a single
// transaction. This prevents partial-series reads that would otherwise occur
// when the analyst-bot queries mid-backfill (e.g. on first container start).
// Under READ COMMITTED isolation, readers either see the full committed state
// or the pre-transaction state — never a halfway-populated series.
func UpsertMacroFredBatch(ctx context.Context, pool *pgxpool.Pool, seriesID string, rows []MacroFredRow) error {
	if len(rows) == 0 {
		return nil
	}
	const q = `
INSERT INTO macro_fred (ts, series_id, value) VALUES ($1,$2,$3)
ON CONFLICT (series_id, ts) DO UPDATE SET value = EXCLUDED.value
`
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx for %s: %w", seriesID, err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck — no-op after Commit

	for _, r := range rows {
		if _, err := tx.Exec(ctx, q, r.TS, seriesID, r.Value); err != nil {
			return fmt.Errorf("upsert %s @ %s: %w", seriesID, r.TS.Format("2006-01-02"), err)
		}
	}
	return tx.Commit(ctx)
}

func UpsertOnchain(ctx context.Context, pool *pgxpool.Pool, ts time.Time, asset, metric string, value *float64, payload any, source string) error {
	var jb []byte
	var err error
	if payload != nil {
		jb, err = json.Marshal(payload)
		if err != nil {
			return err
		}
	}
	const q = `
INSERT INTO onchain_metrics (ts, asset, metric, value, payload, source)
VALUES ($1,$2,$3,$4,$5,$6)
ON CONFLICT (asset, metric, ts, source) DO UPDATE SET
  value = EXCLUDED.value, payload = EXCLUDED.payload
`
	_, err = pool.Exec(ctx, q, ts, asset, metric, value, jb, source)
	return err
}

func UpsertSentiment(ctx context.Context, pool *pgxpool.Pool, ts time.Time, source, symbol string, score *float64, payload any) error {
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	const q = `
INSERT INTO sentiment_snapshots (ts, source, symbol, score, payload)
VALUES ($1,$2,$3,$4,$5)
ON CONFLICT (source, symbol, ts) DO UPDATE SET score = EXCLUDED.score, payload = EXCLUDED.payload
`
	_, err = pool.Exec(ctx, q, ts, source, symbol, score, b)
	return err
}

func InsertNews(ctx context.Context, pool *pgxpool.Pool, ts time.Time, source, symbol, headline, url string, sentiment *float64, payload any) error {
	var jb []byte
	var err error
	if payload != nil {
		jb, err = json.Marshal(payload)
		if err != nil {
			return err
		}
	}
	const q = `
INSERT INTO news_headlines (ts, source, symbol, headline, url, sentiment, payload)
VALUES ($1,$2,$3,$4,$5,$6,$7)
ON CONFLICT (ts, source, headline) DO NOTHING
`
	_, err = pool.Exec(ctx, q, ts, source, nullable(symbol), headline, nullable(url), sentiment, jb)
	return err
}

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}
