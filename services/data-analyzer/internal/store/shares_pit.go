package store

import (
	"context"
	"fmt"
	"sort"
	"time"
)

// SharesPIT is one symbol's point-in-time share series from
// shares_outstanding_pit, oldest filing first. Shared by momentum-scanner and
// momentum-backtest so gate v2's market cap is computed one way everywhere.
type SharesPIT struct {
	filed  []time.Time // calendar dates, UTC midnight
	shares []float64

	// MultiClass is true when any filing's count sums several share classes
	// (see momentum.GateInput.MarketCapPITMultiClass).
	MultiClass bool
}

// LoadSharesPIT reads every symbol's share series.
//
// Keyed on FILED date, never period_end: the period end precedes the filing by
// weeks, and joining on it would use a share count before it was public —
// swapping one lookahead for a subtler one.
func LoadSharesPIT(ctx context.Context, q Querier) (map[string]*SharesPIT, error) {
	rows, err := q.Query(ctx, `
SELECT symbol, filed_date, shares, multi_class
FROM shares_outstanding_pit
ORDER BY symbol, filed_date`)
	if err != nil {
		return nil, fmt.Errorf("load shares_outstanding_pit: %w", err)
	}
	defer rows.Close()
	out := map[string]*SharesPIT{}
	for rows.Next() {
		var sym string
		var d time.Time
		var sh float64
		var mc bool
		if err := rows.Scan(&sym, &d, &sh, &mc); err != nil {
			return nil, fmt.Errorf("scan shares_outstanding_pit: %w", err)
		}
		ps := out[sym]
		if ps == nil {
			ps = &SharesPIT{}
			out[sym] = ps
		}
		ps.add(d, sh, mc)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read shares_outstanding_pit: %w", err)
	}
	return out, nil
}

// add appends a filing; callers must add in filed-date order.
func (p *SharesPIT) add(filed time.Time, shares float64, multiClass bool) {
	p.filed = append(p.filed, calendarDate(filed))
	p.shares = append(p.shares, shares)
	p.MultiClass = p.MultiClass || multiClass
}

// AsOf returns the most recent share count FILED on or before t's calendar
// date, and that filing's date.
//
// ok=false when no filing exists yet. That is not a gap to be patched: a
// symbol-day before the company's first filing is UNMEASURABLE, and gate v2
// rejects it with market_cap_pit_unavailable rather than substituting today's
// value, which would restore the leak exactly where it is largest. Whether the
// returned filing is too OLD is the gate's decision
// (momentum.GateConfig.PITMaxFilingAgeMonths), not this lookup's.
func (p *SharesPIT) AsOf(t time.Time) (shares float64, filed time.Time, ok bool) {
	if p == nil {
		return 0, time.Time{}, false
	}
	day := calendarDate(t)
	i := sort.Search(len(p.filed), func(i int) bool { return p.filed[i].After(day) })
	if i == 0 {
		return 0, time.Time{}, false
	}
	return p.shares[i-1], p.filed[i-1], true
}

// calendarDate keeps t's own year/month/day, so a bar timestamp and a DATE
// column compare as the dates they print as.
func calendarDate(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}
