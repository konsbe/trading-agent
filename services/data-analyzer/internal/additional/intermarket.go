package additional

import (
	"math"
	"time"

	"github.com/konsbe/trading-agent/services/data-analyzer/internal/store"
)

// RollCorrResult is rolling Pearson correlation between benchmark log returns and
// day-over-day changes in a FRED level series (forward-filled to equity dates).
type RollCorrResult struct {
	SeriesID           string  `json:"series_id"`
	Correlation60d     float64 `json:"correlation_60d"`
	ObservationsUsed   int     `json:"observations_used"`
	Regime             string  `json:"regime"`
	Label              string  `json:"label"`
	InsufficientData   bool    `json:"insufficient_data"`
}

func pearson(x, y []float64) (float64, bool) {
	if len(x) != len(y) || len(x) < 3 {
		return 0, false
	}
	n := float64(len(x))
	var sx, sy float64
	for i := range x {
		sx += x[i]
		sy += y[i]
	}
	mx, my := sx/n, sy/n
	var num, dx, dy float64
	for i := range x {
		ax := x[i] - mx
		ay := y[i] - my
		num += ax * ay
		dx += ax * ax
		dy += ay * ay
	}
	den := math.Sqrt(dx * dy)
	if den < 1e-12 {
		return 0, false
	}
	return num / den, true
}

// dateUTC truncates to calendar date in UTC for alignment.
func dateUTC(t time.Time) time.Time {
	y, m, d := t.UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// forwardFillFred maps each equity bar date to the latest FRED value on or before that date.
func forwardFillFred(spyDates []time.Time, fred []store.MacroObs) []float64 {
	if len(fred) == 0 || len(spyDates) == 0 {
		return nil
	}
	out := make([]float64, len(spyDates))
	k := 0
	var last float64
	var has bool
	for i, ts := range spyDates {
		d := dateUTC(ts)
		for k < len(fred) && !dateUTC(fred[k].TS).After(d) {
			last = fred[k].Value
			has = true
			k++
		}
		if has {
			out[i] = last
		} else {
			out[i] = math.NaN()
		}
	}
	return out
}

type regimeFn func(corr float64) (regime, label string)

// ComputeRollCorrEquityVsFredDelta runs the shared 60d (or window) rolling correlation pipeline.
func ComputeRollCorrEquityVsFredDelta(
	spy []store.EquityOHLCVBar,
	fred []store.MacroObs,
	window, minObs int,
	seriesID string,
	regime regimeFn,
	insufficientNote string,
) RollCorrResult {
	bad := RollCorrResult{
		SeriesID:         seriesID,
		InsufficientData: true,
		Regime:           "insufficient_data",
		Label:            insufficientNote,
	}
	if window < 20 || minObs < 20 || len(spy) < window+2 || len(fred) < 5 {
		return bad
	}
	start := len(spy) - (window + 1)
	if start < 0 {
		return bad
	}
	seg := spy[start:]
	dates := make([]time.Time, len(seg))
	for i := range seg {
		dates[i] = seg[i].TS
	}
	filled := forwardFillFred(dates, fred)
	if filled == nil {
		return bad
	}

	var rets []float64
	var ychg []float64
	for i := 1; i < len(seg); i++ {
		if math.IsNaN(filled[i]) || math.IsNaN(filled[i-1]) {
			continue
		}
		if seg[i-1].Close <= 0 || seg[i].Close <= 0 {
			continue
		}
		r := math.Log(seg[i].Close / seg[i-1].Close)
		dy := filled[i] - filled[i-1]
		rets = append(rets, r)
		ychg = append(ychg, dy)
	}

	if len(rets) < minObs {
		bad.Label = "Not enough overlapping return / FRED-delta pairs after alignment."
		return bad
	}
	if len(rets) > window {
		rets = rets[len(rets)-window:]
		ychg = ychg[len(ychg)-window:]
	}

	corr, ok := pearson(rets, ychg)
	if !ok {
		return bad
	}

	rg, lb := regime(corr)
	return RollCorrResult{
		SeriesID:           seriesID,
		Correlation60d:   math.Round(corr*1000) / 1000,
		ObservationsUsed: len(rets),
		Regime:             rg,
		Label:              lb,
		InsufficientData:   false,
	}
}

func regimeBondEquity(c float64) (string, string) {
	switch {
	case c <= -0.25:
		return "deflationary_hedge", "Bonds and equities negatively correlated — classic flight-to-quality hedge (60d rolling)."
	case c >= 0.25:
		return "inflationary_positive", "Positive correlation — bonds may not hedge equity drawdowns (inflation / rates shock pattern)."
	default:
		return "transition_neutral", "Correlation near zero — mixed regime; 60/40 hedge effectiveness uncertain."
	}
}

func regimeOilEquity(c float64) (string, string) {
	switch {
	case c >= 0.25:
		return "procyclical", "SPY and WTI tend to move together — risk-on / growth tilt to crude over the window."
	case c <= -0.25:
		return "decoupled", "Negative correlation — possible supply-shock or defensive equity phase vs energy."
	default:
		return "neutral_mixed", "Weak linear SPY–oil link over 60 sessions."
	}
}

func regimeVIXEquity(c float64) (string, string) {
	switch {
	case c <= -0.25:
		return "typical_fear_greed", "Equities rise when VIX falls — usual fear gauge wiring over the window."
	case c >= 0.15:
		return "unusual_positive", "Positive SPY–ΔVIX correlation — both moving together; macro may be unusual."
	default:
		return "compressed_link", "Mild short-term coupling between equity returns and VIX changes."
	}
}

// ComputeBondEquity60d is a thin wrapper for tests and bond-specific wording.
func ComputeBondEquity60d(spy []store.EquityOHLCVBar, dgs10 []store.MacroObs, window, minObs int) BondEquityLegacy {
	r := ComputeRollCorrEquityVsFredDelta(spy, dgs10, window, minObs, "DGS10", regimeBondEquity,
		"Need more overlapping SPY bars and DGS10 observations.")
	return BondEquityLegacy{
		Correlation60d:   r.Correlation60d,
		ObservationsUsed: r.ObservationsUsed,
		Regime:           r.Regime,
		Label:            r.Label,
		InsufficientData: r.InsufficientData,
	}
}

// BondEquityLegacy matches the original test / snapshot field shape for DGS10.
type BondEquityLegacy struct {
	Correlation60d     float64
	ObservationsUsed   int
	Regime             string
	Label              string
	InsufficientData   bool
}
