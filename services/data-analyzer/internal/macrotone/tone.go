// Package macrotone assigns each stored macro classification (a stance word or a
// signal's regime label) one of four display tones, and each signal its tier
// within its section. It is the single place that
// decides whether a label reads as constructive, neutral or stressed; readers
// (momentum-api, the web report) consume the stored "tone" field and never
// re-derive it from the label or the raw number.
//
// The tables are ported from the analyst-bot Discord formatter
// (services/analyst-bot/notifier/discord/formatter.py) so web and Discord agree,
// with these deliberate differences:
//   - four tones only: the bot's orange ("flat" curve, "elevated" credit) is neutral;
//   - labels the bot leaves uncoloured but which are real classifications
//     (mp_m2_supply "normal") are neutral, not no-data;
//   - stance tones follow the composite bands: inflation "moderate" is neutral
//     (the band between deflationary and hot), inflation "deflationary" is stressed.
package macrotone

import "strings"

type Tone string

const (
	Constructive Tone = "constructive"
	Neutral      Tone = "neutral"
	Stressed     Tone = "stressed"
	NoData       Tone = "no_data"
	// DisplayOnly marks rows that carry levels, not a classification
	// (e.g. mp_treasury_yields); they get no status indicator.
	DisplayOnly Tone = "display_only"
)

// Key is the payload field the tone is stored under.
const Key = "tone"

var (
	c = Constructive
	n = Neutral
	s = Stressed
)

// stances maps a *_stance metric's "stance" word to its tone.
var stances = map[string]map[string]Tone{
	"mp_stance":  {"accommodative": c, "neutral": n, "restrictive": s},
	"gc_stance":  {"expansion": c, "slowdown": n, "contraction": s},
	"inf_stance": {"moderate": n, "hot": s, "deflationary": s},
	"gg_stance":  {"benign": c, "moderate": n, "elevated_stress": s},
}

// signals maps a signal metric's "regime" label to its tone. The same word can
// mean different things in different signals, so tables are per metric.
var signals = map[string]map[string]Tone{
	// Monetary policy
	"mp_rate":                {"cutting": c, "neutral": n, "hiking": s},
	"mp_yield_curve":         {"steep": c, "normal": n, "flat": n, "inverted": s, "re_steepening": s},
	"mp_real_rate":           {"deeply_negative": c, "balanced": n, "headwind": s},
	"mp_balance_sheet":       {"qe": c, "neutral": n, "qt": s},
	"mp_credit_spread":       {"benign": c, "elevated": n, "crisis": s},
	"mp_breakeven_inflation": {"anchored": c, "rising": n, "unanchored": s},
	"mp_m2_supply":           {"deflationary": c, "normal": n, "slow": n, "inflationary": s},

	// Growth cycle
	"gc_pmi":                {"strong_expansion": c, "expansion": c, "slowing": n, "contraction": s, "severe_contraction": s},
	"gc_lei":                {"expanding": c, "stable": n, "slowing": n},
	"gc_claims":             {"tight_labor": c, "normal": n, "normalizing": n, "crisis": s},
	"gc_housing":            {"strong": c, "moderate": n, "weak": s},
	"gc_gdp":                {"strong": c, "moderate": n, "stall_speed": n, "recession": s},
	"gc_employment":         {"strong": c, "moderate": n, "slowing": n, "contraction": s, "recession_confirmed": s},
	"gc_consumer":           {"healthy": c, "slowing": n, "contraction": s},
	"gc_consumer_sentiment": {"near_bottom": c, "normal": n, "pessimistic": n, "complacency": s},
	"gc_capex":              {"expanding": c, "stable": n, "slowing": n, "warning": s},

	// Inflation
	"inf_core_pce": {"at_target": c, "below_target": c, "hawkish_bias": n, "aggressive_tightening": s},
	"inf_cpi":      {"goldilocks": c, "below_target": c, "rising": n, "above_target": n, "hot": s, "deflation_risk": s},
	"inf_core_cpi": {"at_target": c, "below_target": c, "above_target": n, "hot": s},
	"inf_shelter":  {"normalizing": c, "moderating": n, "elevated": n, "hot": s},
	"inf_ppi":      {"deflationary": c, "stable": c, "moderate": n, "elevated": n, "surge": s},
	"inf_oil":      {"goldilocks": c, "low": c, "elevated": n, "inflationary_risk": s, "energy_sector_stress": s},
	"inf_wages":    {"target_consistent": c, "soft": c, "above_target": n, "elevated": n, "spiral_risk": s},
	"inf_copper":   {"global_expansion": c, "stable": c, "slowing": n, "global_contraction": s},

	// Global / geopolitical
	"gg_broad_dollar": {"dollar_weak_risk_on": c, "supportive_equities": c, "neutral": n, "em_commodity_headwind": n, "major_global_stress": s},
	"gg_usdjpy":       {"carry_intact": c, "early_carry_unwind": n, "systemic_carry_unwind": s},
	"gg_china_gdp":    {"expansion": c, "stable": n, "slowing": s, "contraction_risk": s},
	"gg_fiscal":       {"manageable": c, "elevated_supply_risk": n, "high_deficit_stress": s},

	// Macro correlations regime
	"mc_macro_correlation": {
		"goldilocks_light": c, "disinflation_soft_landing": c,
		"neutral_mixed":      n,
		"recession_pipeline": s, "stagflation_risk": s, "rising_inflation_tight_policy": s,
		"global_liquidity_stress": s, "deflation_risk": s,
	},
}

// margin maps inf_ppi_cpi_spread's stored "margin_signal" to its tone.
var margin = map[string]Tone{"margin_expansion": c, "neutral": n, "margin_pressure": s}

var displayOnly = map[string]bool{"mp_treasury_yields": true}

// Placement is where a signal sits in its section: tier 1 is the primary read.
// Ported from the bot's embed layout so both surfaces group signals the same way.
type Placement struct {
	Tier  int
	Group string
}

var placements = map[string]Placement{
	"mp_rate":                {1, "Monetary Policy"},
	"mp_yield_curve":         {1, "Monetary Policy"},
	"mp_real_rate":           {1, "Monetary Policy"},
	"mp_balance_sheet":       {1, "Monetary Policy"},
	"mp_credit_spread":       {1, "Monetary Policy"},
	"mp_breakeven_inflation": {2, "Bond Market"},
	"mp_treasury_yields":     {2, "Bond Market"},
	"mp_m2_supply":           {2, "Bond Market"},

	"gc_pmi":                {1, "Leading Indicators"},
	"gc_lei":                {1, "Leading Indicators"},
	"gc_claims":             {1, "Leading Indicators"},
	"gc_housing":            {1, "Leading Indicators"},
	"gc_gdp":                {2, "Coincident Indicators"},
	"gc_employment":         {2, "Coincident Indicators"},
	"gc_consumer":           {2, "Coincident Indicators"},
	"gc_consumer_sentiment": {3, "Lagging / Sentiment"},
	"gc_capex":              {3, "Lagging / Sentiment"},

	"inf_core_pce":       {1, "Core Inflation"},
	"inf_cpi":            {1, "Core Inflation"},
	"inf_core_cpi":       {1, "Core Inflation"},
	"inf_shelter":        {1, "Core Inflation"},
	"inf_ppi":            {2, "Pipeline & Energy"},
	"inf_ppi_cpi_spread": {2, "Pipeline & Energy"},
	"inf_oil":            {2, "Pipeline & Energy"},
	"inf_wages":          {3, "Wages & Commodities"},
	"inf_copper":         {3, "Wages & Commodities"},

	"gg_broad_dollar": {1, "FX & carry"},
	"gg_usdjpy":       {1, "FX & carry"},
	"gg_china_gdp":    {2, "China & fiscal"},
	"gg_fiscal":       {2, "China & fiscal"},
}

// PlacementFor returns a signal metric's tier; ok is false for stances, the
// correlation regime and unknown metrics.
func PlacementFor(metric string) (Placement, bool) {
	p, ok := placements[metric]
	return p, ok
}

// For returns the tone of a metric's stored classification. ok is false when the
// label is not in the tables (a new label nobody has toned yet).
func For(metric string, payload map[string]any) (Tone, bool) {
	if displayOnly[metric] {
		return DisplayOnly, true
	}
	if metric == "inf_ppi_cpi_spread" {
		return lookup(margin, str(payload, "margin_signal"))
	}
	if t, isStance := stances[metric]; isStance {
		return lookup(t, str(payload, "stance"))
	}
	if t, isSignal := signals[metric]; isSignal {
		return lookup(t, str(payload, "regime"))
	}
	return "", false
}

// Annotate stores the tone on payload in place when payload is a classification
// row. It reports false when a classification metric's label has no tone, or when
// a metric under a section prefix (mp_, gc_, inf_, gg_) is not in the tables, so
// callers can log it instead of shipping an untoned row silently.
func Annotate(metric string, payload any) bool {
	if !Classified(metric) {
		return !sectionMetric(metric)
	}
	m, ok := payload.(map[string]any)
	if !ok {
		return false
	}
	t, ok := For(metric, m)
	if !ok {
		return false
	}
	m[Key] = string(t)
	if p, ok := placements[metric]; ok {
		m["tier"] = p.Tier
		m["tier_group"] = p.Group
	}
	return true
}

func sectionMetric(metric string) bool {
	for _, p := range []string{"mp_", "gc_", "inf_", "gg_"} {
		if strings.HasPrefix(metric, p) {
			return true
		}
	}
	return false
}

// Classified reports whether metric is one this package tones.
func Classified(metric string) bool {
	if displayOnly[metric] || metric == "inf_ppi_cpi_spread" {
		return true
	}
	_, st := stances[metric]
	_, sg := signals[metric]
	return st || sg
}

// Labels returns the toned labels for metric, for tests that check coverage.
func Labels(metric string) map[string]Tone {
	if t, ok := stances[metric]; ok {
		return t
	}
	return signals[metric]
}

func lookup(t map[string]Tone, label string) (Tone, bool) {
	switch label {
	case "no_data", "insufficient_data":
		return NoData, true
	case "":
		return "", false
	}
	v, ok := t[label]
	return v, ok
}

func str(m map[string]any, k string) string {
	v, _ := m[k].(string)
	return v
}
