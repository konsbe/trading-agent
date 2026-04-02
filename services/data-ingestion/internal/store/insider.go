package store

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const upsertInsiderSQL = `
INSERT INTO insider_transactions
    (ts, symbol, insider_name, transaction_code, shares, price_per_share, filing_date)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (symbol, ts, insider_name, transaction_code, shares) DO NOTHING`

// UpsertInsiderTransaction inserts a single insider transaction row.
// Conflicts (duplicate filings) are silently ignored.
//
//   - ts:              timestamp of the transaction (transactionDate from Finnhub)
//   - symbol:          equity ticker
//   - insiderName:     SEC filer display name
//   - code:            transaction code — P = purchase, S = sale, A = award, F = tax, M = exercise
//   - shares:          number of shares; may be nil if Finnhub omits the field
//   - pricePerShare:   transaction price; nil when not reported
//   - filingDate:      date the Form 4 was filed with the SEC; nil if unknown
func UpsertInsiderTransaction(
	ctx context.Context,
	pool *pgxpool.Pool,
	ts time.Time,
	symbol, insiderName, code string,
	shares, pricePerShare *float64,
	filingDate *time.Time,
) error {
	var fd any
	if filingDate != nil {
		fd = filingDate.Format("2006-01-02")
	}
	_, err := pool.Exec(ctx, upsertInsiderSQL,
		ts, symbol, nullable(insiderName), nullable(code), shares, pricePerShare, fd,
	)
	return err
}
