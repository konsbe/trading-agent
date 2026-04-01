package store

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// QueryLatestFREDValue returns the most recent value for the given FRED series_id
// from the macro_fred table. Returns (value, true, nil) when a row is found,
// or (0, false, nil) when the series has no data yet.
func QueryLatestFREDValue(ctx context.Context, pool *pgxpool.Pool, seriesID string) (float64, bool, error) {
	var value float64
	err := pool.QueryRow(ctx, `
		SELECT value
		FROM macro_fred
		WHERE series_id = $1
		ORDER BY ts DESC
		LIMIT 1`,
		seriesID,
	).Scan(&value)
	if err != nil {
		// pgx returns pgx.ErrNoRows when there is no matching row.
		if err.Error() == "no rows in result set" {
			return 0, false, nil
		}
		return 0, false, err
	}
	return value, true, nil
}
