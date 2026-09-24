package momentumapi

import (
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/momentum"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/store"
)

// The detail view's explanations: which gate thresholds a symbol was held to,
// each component's weight, and every penalty rule. All of it comes from the
// momentum package the scanner itself runs — pass/fail is read from the stored
// gate_failures, never re-evaluated here.

// gateCheck groups the persisted failure codes that belong to one §3.2 gate.
type gateCheckDef struct {
	key      string
	label    string
	failures []string
}

var gateCheckDefs = []gateCheckDef{
	{"price", "Price within bucket range", []string{momentum.GateNoClose, momentum.GateCloseBelowFloor, momentum.GateCloseAboveCeil, momentum.GateUnbucketable}},
	{"history", "Enough price history", []string{momentum.GateShortHistory}},
	{"change_pct", "Day change within range", []string{momentum.GateNoChangePct, momentum.GateChangeTooLow, momentum.GateChangeTooHigh}},
	{"rvol_20", "Relative volume (20-day)", []string{momentum.GateNoRVol, momentum.GateRVolTooLow}},
	{"dollar_volume", "Dollar volume (liquidity)", []string{momentum.GateNoDollarVol, momentum.GateDollarVolTooLow}},
	{"market_cap", "Market cap within bucket range", []string{momentum.GateMarketCapUnavailable, momentum.GateMarketCapTooLow, momentum.GateMarketCapTooHigh, momentum.GateMarketCapPITUnavailable}},
}

// optionalBound turns the config's "0 means unbounded" into a nil.
func optionalBound(v float64) *float64 {
	if v == 0 {
		return nil
	}
	return &v
}

func buildGateChecks(row store.SymbolDetailRow) gatesDetail {
	failed := map[string]bool{}
	for _, f := range row.GateFailures {
		failed[f] = true
	}

	var th *momentum.BucketThresholds
	if row.Bucket != nil {
		t := momentum.DefaultGateConfig().Thresholds(momentum.Bucket(*row.Bucket))
		th = &t
	}
	minBars := float64(momentum.DefaultGateConfig().MinBars)
	f := row.Facts
	marketCap := f.MarketCap
	if marketCap == nil {
		marketCap = f.MarketCapEst
	}

	out := gatesDetail{Checks: []gateCheck{}}
	known := map[string]bool{momentum.GateMarketCapNull: true} // provenance marker, not a failure
	for _, def := range gateCheckDefs {
		c := gateCheck{Key: def.key, Label: def.label, Passed: true, Failures: []string{}}
		for _, code := range def.failures {
			known[code] = true
			if failed[code] {
				c.Passed = false
				c.Failures = append(c.Failures, code)
			}
		}
		switch def.key {
		case "price":
			c.Value = finite(f.Close)
			if th != nil {
				c.Min, c.Max = optionalBound(th.MinClose), optionalBound(th.MaxClose)
			}
		case "history":
			c.Min = &minBars
		case "change_pct":
			c.Value = finite(f.ChangePct)
			if th != nil {
				c.Min, c.Max = optionalBound(th.MinChangePct), optionalBound(th.MaxChangePct)
			}
		case "rvol_20":
			c.Value = finite(f.RVol20)
			if th != nil {
				c.Min = optionalBound(th.MinRVol20)
			}
		case "dollar_volume":
			c.Value = finite(f.DollarVolume)
			if th != nil {
				c.Min = optionalBound(th.MinDollarVolPS)
			}
		case "market_cap":
			c.Value = finite(marketCap)
			c.ValueIsProxy = f.MarketCapIsProxy || failed[momentum.GateMarketCapNull]
			if th != nil {
				c.Min, c.Max = optionalBound(th.MinMarketCap), optionalBound(th.MaxMarketCap)
			}
		}
		if c.Passed {
			out.PassedCount++
		}
		out.Checks = append(out.Checks, c)
	}
	out.Total = len(out.Checks)
	for _, code := range row.GateFailures {
		if !known[code] {
			out.Unmapped = append(out.Unmapped, code)
		}
	}
	return out
}

// componentWeights are §4.1 v2's per-component maxima, keyed like sub_scores.
func componentWeights() map[string]int {
	return map[string]int{
		"rvol":      momentum.WeightRVol,
		"vol_accel": momentum.WeightVolAccel,
		"catalyst":  momentum.WeightCatalyst,
		"float":     momentum.WeightFloat,
		"vwap":      momentum.WeightVWAP,
		"breakout":  momentum.WeightBreakout,
		"high52w":   momentum.WeightHigh52w,
	}
}

var penaltyRuleDefs = []struct {
	code   string
	points int
}{
	{momentum.ReasonExhaustedRSI, momentum.PenaltyExhaustedRSI},
	{momentum.ReasonAlreadyExtended, momentum.PenaltyAlreadyExtended},
	{momentum.ReasonVolumeDecaying, momentum.PenaltyVolumeDecaying},
}

// buildPenaltyRules lists every §4.3 penalty with whether it was applied, so
// "0 pts" is shown as a checked rule rather than silently absent.
func buildPenaltyRules(applied []string) []penaltyRule {
	isApplied := map[string]bool{}
	for _, p := range applied {
		isApplied[p] = true
	}
	out := make([]penaltyRule, 0, len(penaltyRuleDefs))
	known := map[string]bool{}
	for _, d := range penaltyRuleDefs {
		known[d.code] = true
		out = append(out, penaltyRule{Code: d.code, Points: d.points, Applied: isApplied[d.code]})
	}
	for _, p := range applied {
		if !known[p] {
			out = append(out, penaltyRule{Code: p, Applied: true})
		}
	}
	return out
}

func buildFacts(f store.DetailFacts) factsDetail {
	var changeAbs *float64
	if f.Close != nil && f.PriorClose != nil {
		v := *f.Close - *f.PriorClose
		changeAbs = finite(&v)
	}
	return factsDetail{
		Close:            finite(f.Close),
		PriorClose:       finite(f.PriorClose),
		ChangePct:        finite(f.ChangePct),
		ChangeAbs:        changeAbs,
		GapPct:           finite(f.GapPct),
		Volume:           finite(f.Volume),
		AvgVolume20:      finite(f.AvgVol20),
		DollarVolume:     finite(f.DollarVolume),
		RVol20:           finite(f.RVol20),
		VolAccel:         finite(f.VolAccel),
		ATRPct:           finite(f.ATRPct),
		RSI14:            finite(f.RSI14),
		High52w:          finite(f.High52w),
		PctOf52wHigh:     finite(f.PctOf52wHigh),
		Resistance20:     finite(f.Resistance20),
		BreakoutState:    f.BreakoutState,
		WasConsolidating: f.WasConsolidating,
		VWAP20:           finite(f.VWAP20),
		AboveVWAP:        f.AboveVWAP,
		VWAPDistPct:      finite(f.VWAPDistPct),
		FloatSharesEst:   finite(f.FloatSharesEst),
		FloatIsProxy:     f.FloatIsProxy,
		MarketCap:        finite(f.MarketCap),
		MarketCapEst:     finite(f.MarketCapEst),
		MarketCapIsProxy: f.MarketCapIsProxy,
		CatalystTier:     f.CatalystTier,
		CatalystHeadline: f.CatalystHeadline,
		ComputedAt:       f.ComputedAt.Format(timeRFC3339),
	}
}
