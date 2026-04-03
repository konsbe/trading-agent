package store

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// EquityOHLCVBar is one daily (or other interval) row from equity_ohlcv.
type EquityOHLCVBar struct {
	TS     time.Time
	Open   float64
	High   float64
	Low    float64
	Close  float64
	Volume float64
}

// QueryEquityOHLCVAsc returns the last `limit` bars for symbol×interval in ascending time order.
func QueryEquityOHLCVAsc(ctx context.Context, pool *pgxpool.Pool, symbol, interval string, limit int) ([]EquityOHLCVBar, error) {
	rows, err := pool.Query(ctx,
		`SELECT ts, open, high, low, close, volume
		 FROM equity_ohlcv
		 WHERE symbol = $1 AND interval = $2
		 ORDER BY ts DESC
		 LIMIT $3`,
		symbol, interval, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var desc []EquityOHLCVBar
	for rows.Next() {
		var b EquityOHLCVBar
		if err := rows.Scan(&b.TS, &b.Open, &b.High, &b.Low, &b.Close, &b.Volume); err != nil {
			return nil, err
		}
		desc = append(desc, b)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// reverse to ascending
	for i, j := 0, len(desc)-1; i < j; i, j = i+1, j-1 {
		desc[i], desc[j] = desc[j], desc[i]
	}
	return desc, nil
}
