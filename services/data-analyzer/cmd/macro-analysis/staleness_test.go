package main

import (
	"math"
	"testing"
	"time"

	"github.com/konsbe/trading-agent/services/data-analyzer/internal/config"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/macrocorr"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/store"
)

func obs(dates []string, vals []float64) []store.MacroObs {
	out := make([]store.MacroObs, len(dates))
	for i, d := range dates {
		ts, _ := time.Parse(time.DateOnly, d)
		out[i] = store.MacroObs{TS: ts, Value: vals[i]}
	}
	return out
}

// The newest stored observations on 2026-09-27: USSLIND ended 2020-02-01 and
// CHNGDPNQDSMEI 2023-07-01, and both still fed their stances as current.
var (
	runAt   = time.Date(2026, 9, 27, 9, 0, 0, 0, time.UTC)
	usslind = obs([]string{"2020-02-01", "2020-01-01", "2019-12-01", "2019-11-01", "2019-10-01", "2019-09-01"},
		[]float64{1.72, 1.57, 1.48, 1.38, 1.41, 1.5})
	chinaGDP = obs([]string{"2023-07-01", "2023-04-01", "2023-01-01", "2022-10-01", "2022-07-01"},
		[]float64{31999230000000, 30803760000000, 28499660000000, 33550790000000, 30927060000000})
)

func TestObservationStale_DifferentLimitsPerCadence(t *testing.T) {
	gc, g := config.LoadGrowthCycle(), config.LoadGlobalGeopolitical()
	if gc.LEIMaxAgeDays >= g.ChinaGDPMaxAgeDays {
		t.Errorf("monthly LEI limit %d should be shorter than quarterly China GDP's %d", gc.LEIMaxAgeDays, g.ChinaGDPMaxAgeDays)
	}
	if !observationStale(usslind, runAt, gc.LEIMaxAgeDays) {
		t.Error("USSLIND (last 2020-02-01) not stale")
	}
	if !observationStale(chinaGDP, runAt, g.ChinaGDPMaxAgeDays) {
		t.Error("CHNGDPNQDSMEI (last 2023-07-01) not stale")
	}
	// A normally published series is not: a monthly print for July read in
	// late September, and a Q1 print (dated 1 January) read in late June.
	if observationStale(obs([]string{"2026-07-01"}, []float64{1}), runAt, gc.LEIMaxAgeDays) {
		t.Error("a two-month-old monthly observation is stale")
	}
	if observationStale(obs([]string{"2026-01-01"}, []float64{1}), time.Date(2026, 6, 25, 0, 0, 0, 0, time.UTC), g.ChinaGDPMaxAgeDays) {
		t.Error("a normally published quarterly observation is stale")
	}
	if observationStale(nil, runAt, gc.LEIMaxAgeDays) {
		t.Error("no observations is no_data, not stale")
	}
}

// Live 2026-09-27: the Growth rows as scored (score, weight). LEI's 9.06 is
// the sum of the six USSLIND values above, "expanding", +0.8 at 0.12.
func TestStaleLEIFlipsGrowthAndTheRegime(t *testing.T) {
	gc := config.LoadGrowthCycle()
	rows := []struct {
		name          string
		score, weight float64
	}{
		{"capex", 0.8, 0.05}, {"claims", 0.9, 0.08}, {"consumer", 0, 0.10}, {"consumer_sentiment", 0.3, 0.05},
		{"employment", 0.3, 0.14}, {"gdp", 0.3, 0.14}, {"housing", 0.2, 0.08},
	}
	growth := func(withLEI bool) (string, float64) {
		sum, w := 0.0, 0.0
		for _, r := range rows {
			sum += r.score * r.weight
			w += r.weight
		}
		if withLEI {
			sum += 0.8 * 0.12
			w += 0.12
		}
		return growthStanceFor(sum/w, gc), sum / w
	}
	regime := func(gcStance string) string {
		return macrocorr.Build(macrocorr.Inputs{
			GCStance: gcStance, MPStance: "neutral", InfStance: "hot", GGStance: "elevated_stress",
			YieldCurve: "normal", RealRate: "headwind", CreditRegime: "benign", GDPRegime: "moderate",
			OilRegime: "elevated", DollarRegime: "major_global_stress", JPYRegime: "carry_intact",
		}).Regime
	}

	lei := !observationStale(usslind, runAt, gc.LEIMaxAgeDays)
	stance, score := growth(lei)
	if stance != "slowdown" || math.Abs(score-0.355) > 0.001 || regime(stance) != "stagflation_risk" {
		t.Errorf("growth %s %.3f, regime %s; want slowdown 0.355, stagflation_risk", stance, score, regime(stance))
	}
	// What the stale row produced live.
	if s, sc := growth(true); s != "expansion" || math.Abs(sc-0.425) > 0.001 || regime(s) != "global_liquidity_stress" {
		t.Errorf("with the stale LEI: %s %.3f %s; the live bug read expansion 0.425, global_liquidity_stress", s, sc, regime(s))
	}
}

func TestStaleRowPayloadHasNoTierAndKeepsTheEvidence(t *testing.T) {
	p := staleRowPayload("USSLIND", usslind, 120)
	if p["regime"] != "no_recent_data" || p["last_observation"] != "2020-02-01" || p["last_value"] != 1.72 {
		t.Errorf("payload = %v", p)
	}
}
