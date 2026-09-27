package main

import (
	"time"

	"github.com/konsbe/trading-agent/services/data-analyzer/internal/config"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/store"
)

// observationStale reports whether the newest observation (obs is newest
// first) is more than maxAgeDays old at now. FRED dates a monthly or
// quarterly observation by the start of its period, so the limit must allow
// for the period itself plus the publication lag. maxAgeDays ≤ 0 disables it.
func observationStale(obs []store.MacroObs, now time.Time, maxAgeDays int) bool {
	return maxAgeDays > 0 && len(obs) > 0 && now.Sub(obs[0].TS) > time.Duration(maxAgeDays)*24*time.Hour
}

// staleRowPayload is stored, with a nil value, for a row whose series stopped
// updating: the row reads no_recent_data and is left out of its stance.
func staleRowPayload(series string, obs []store.MacroObs, maxAgeDays int) map[string]any {
	return map[string]any{
		"regime":           "no_recent_data",
		"series":           series,
		"last_observation": obs[0].TS.UTC().Format(time.DateOnly),
		"last_value":       obs[0].Value,
		"max_age_days":     maxAgeDays,
		"note":             "newest observation is older than the age limit; not scored",
	}
}

// growthStanceFor maps the Growth composite score to its stance.
func growthStanceFor(score float64, gc config.GrowthCycle) string {
	switch {
	case score >= gc.GrowthExpansionScore:
		return "expansion"
	case score <= gc.GrowthContractionScore:
		return "contraction"
	}
	return "slowdown"
}
