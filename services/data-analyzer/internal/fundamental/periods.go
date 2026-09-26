package fundamental

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/konsbe/trading-agent/services/data-analyzer/internal/store"
)

// xbrlSource is where data-fundamental stores the figures of SEC filings
// (Finnhub /stock/financials-reported): one period per filing, "annual_<fiscal
// year>" for a 10-K and "q_<period end>" for a 10-Q.
const xbrlSource = "finnhub_financials_reported"

// periodEnd is the date a period label ends on. "q_2026-03-28" (a 10-Q) and
// finnhub_earnings' "q_2026-03-31" carry it; "annual_2025" is read as the
// calendar year end, a fallback only (filings take the exact end from
// report_raw). ok is false for undated labels such as "ttm".
func periodEnd(label string) (time.Time, bool) {
	if s, ok := strings.CutPrefix(label, "q_"); ok {
		t, err := time.Parse("2006-01-02", s)
		return t, err == nil
	}
	if s, ok := strings.CutPrefix(label, "annual_"); ok {
		y, err := strconv.Atoi(s)
		if err != nil {
			return time.Time{}, false
		}
		return time.Date(y, 12, 31, 0, 0, 0, 0, time.UTC), true
	}
	return time.Time{}, false
}

// latestValues maps each metric to the value of its most recent period.
//
// QueryLatestMetrics returns one row per (metric, period) in ascending label
// order, so keeping the first value seen per metric, as every pass used to,
// kept the OLDEST period (INTC's operating income resolved to FY2021).
//
// Filing metrics (source xbrlSource) are left out: an annual and a quarterly
// figure are different quantities, so they are read through filings, where
// each consumer chooses one explicitly.
func latestValues(rows []store.FundamentalRow) map[string]float64 {
	latest := make(map[string]float64, len(rows))
	ends := make(map[string]time.Time, len(rows))
	for _, r := range rows {
		if r.Value == nil || r.Source == xbrlSource {
			continue
		}
		end, _ := periodEnd(r.Period)
		if prev, seen := ends[r.Metric]; seen && !end.After(prev) {
			continue
		}
		latest[r.Metric] = *r.Value
		ends[r.Metric] = end
	}
	return latest
}

// chronological returns the values of one metric across its periods, oldest
// period end first.
func chronological(rows []store.FundamentalRow, metric string) []float64 {
	type point struct {
		end time.Time
		v   float64
	}
	var pts []point
	for _, r := range rows {
		if r.Metric != metric || r.Value == nil {
			continue
		}
		end, _ := periodEnd(r.Period)
		pts = append(pts, point{end, *r.Value})
	}
	sort.SliceStable(pts, func(i, j int) bool { return pts[i].end.Before(pts[j].end) })
	out := make([]float64, len(pts))
	for i, p := range pts {
		out[i] = p.v
	}
	return out
}

// filing is every figure data-fundamental stored for one SEC filing.
type filing struct {
	period     string
	annual     bool      // 10-K; otherwise a 10-Q
	start, end time.Time // from report_raw; start is zero without it
	vals       map[string]float64
}

func (f filing) get(metric string) (float64, bool) {
	v, ok := f.vals[metric]
	return v, ok
}

// filings are a symbol's filings, newest period end first.
type filings []filing

func buildFilings(rows []store.FundamentalRow) filings {
	byPeriod := map[string]*filing{}
	for _, r := range rows {
		if r.Source != xbrlSource {
			continue
		}
		annual := strings.HasPrefix(r.Period, "annual_")
		if !annual && !strings.HasPrefix(r.Period, "q_") {
			continue
		}
		f := byPeriod[r.Period]
		if f == nil {
			f = &filing{period: r.Period, annual: annual, vals: map[string]float64{}}
			f.end, _ = periodEnd(r.Period)
			byPeriod[r.Period] = f
		}
		if r.Metric == "report_raw" {
			var meta struct {
				StartDate string `json:"startDate"`
				EndDate   string `json:"endDate"`
			}
			if json.Unmarshal(r.Payload, &meta) == nil {
				f.start = parseDay(meta.StartDate)
				if end := parseDay(meta.EndDate); !end.IsZero() {
					f.end = end
				}
			}
			continue
		}
		if r.Value != nil {
			f.vals[r.Metric] = *r.Value
		}
	}
	var out filings
	for _, f := range byPeriod {
		if len(f.vals) > 0 {
			out = append(out, *f)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].end.Equal(out[j].end) {
			return out[i].end.After(out[j].end)
		}
		return out[i].period > out[j].period
	})
	return out
}

// parseDay reads Finnhub's "2025-12-27 00:00:00" (or a bare date).
func parseDay(s string) time.Time {
	if len(s) < 10 {
		return time.Time{}
	}
	t, err := time.Parse("2006-01-02", s[:10])
	if err != nil {
		return time.Time{}
	}
	return t
}

// balanceSheet is the newest filing of either kind: balance-sheet figures are
// point-in-time, so a 10-Q's are as valid as a 10-K's and more recent.
func (fs filings) balanceSheet() (filing, bool) {
	if len(fs) == 0 {
		return filing{}, false
	}
	return fs[0], true
}

// yearFlows is the filing income-statement and cash-flow figures are read
// from, with the factor that scales its figures to one year:
//
//   - the newest 10-K, a full fiscal year: factor 1;
//   - with no 10-K, the newest 10-Q scaled by 365.25 / the days it covers.
//     Finnhub's 10-Q figures run from the fiscal-year start (INTC Q3 2023
//     revenue 38,822 is nine months; the quarter alone was ~14,158), so a flat
//     ×4 overstates every 10-Q after Q1; the span is report_raw's
//     startDate..endDate, and without it there is no annual figure.
//
// A newer 10-Q does not displace the 10-K: a partial year scaled up carries
// the seasonality of the months it covers. basis records the choice.
func (fs filings) yearFlows() (f filing, factor float64, basis string, ok bool) {
	for _, f := range fs {
		if f.annual {
			return f, 1, "10-K " + f.period, true
		}
	}
	for _, f := range fs {
		if f.start.IsZero() {
			continue
		}
		days := f.end.Sub(f.start).Hours()/24 + 1
		if days < 80 {
			continue
		}
		factor := 365.25 / days
		return f, factor, fmt.Sprintf("10-Q %s fiscal YTD (%.0f days) ×%.2f", f.period, days, factor), true
	}
	return filing{}, 0, "", false
}

// annualFlow is one income-statement or cash-flow figure over one year, from
// yearFlows' filing.
func annualFlow(fs filings, metric string) (v float64, basis string, ok bool) {
	f, factor, basis, ok := fs.yearFlows()
	if !ok {
		return 0, "", false
	}
	x, ok := f.get(metric)
	return x * factor, basis, ok
}
