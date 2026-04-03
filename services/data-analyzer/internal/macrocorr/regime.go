// Package macrocorr derives a compact macro “transmission / regime” label from the
// latest stance and regime strings already stored in macro_derived (see
// macro_analysis_reference.html — Macro Correlations panel).
package macrocorr

import "strings"

// Inputs are string fields read from macro_derived payloads (empty = unknown).
type Inputs struct {
	GCStance, MPStance, InfStance, GGStance string
	YieldCurve, RealRate, CreditRegime    string
	GDPRegime                             string
	OilRegime                             string
	DollarRegime, JPYRegime               string
}

// Result is persisted as mc_macro_correlation in macro_derived.
type Result struct {
	Regime string
	Score  float64
	Label  string
	Flags  []string
}

func addFlag(flags *[]string, seen map[string]struct{}, f string) {
	f = strings.TrimSpace(f)
	if f == "" {
		return
	}
	if _, ok := seen[f]; ok {
		return
	}
	seen[f] = struct{}{}
	*flags = append(*flags, f)
}

// Build maps cross-metric stances into one regime bucket and a bounded flag list.
func Build(in Inputs) Result {
	flags := []string{}
	seen := map[string]struct{}{}

	inf := in.InfStance
	gc := in.GCStance
	mp := in.MPStance
	gg := in.GGStance
	yc := in.YieldCurve
	cr := in.CreditRegime
	rr := in.RealRate
	gdp := in.GDPRegime
	usd := in.DollarRegime
	jpy := in.JPYRegime
	oil := in.OilRegime

	infHot := inf == "hot"
	infMod := inf == "moderate"
	infDef := inf == "deflationary"

	gcWeak := gc == "contraction" || gc == "slowdown"
	gcStr := gc == "expansion"

	curveInv := yc == "inverted" || yc == "re_steepening"
	curveTight := curveInv || yc == "flat"

	credStress := cr == "elevated" || cr == "crisis"
	gdpWeak := gdp == "stall_speed" || gdp == "recession"

	usdStrong := usd == "em_commodity_headwind" || usd == "major_global_stress"
	jpyStress := jpy == "systemic_carry_unwind"

	if rr == "headwind" {
		addFlag(&flags, seen, "real_rates_headwind")
	}
	if curveTight {
		addFlag(&flags, seen, "curve_flat_or_inverted")
	}
	if credStress {
		addFlag(&flags, seen, "credit_stress")
	}
	if infHot {
		addFlag(&flags, seen, "inflation_hot")
	}
	if gcWeak {
		addFlag(&flags, seen, "growth_soft")
	}
	if usdStrong {
		addFlag(&flags, seen, "usd_strong_em_headwind")
	}
	if gg == "elevated_stress" {
		addFlag(&flags, seen, "global_stress")
	}
	if jpyStress {
		addFlag(&flags, seen, "jpy_carry_stress")
	}
	if oil == "inflationary_risk" || oil == "energy_sector_stress" || oil == "elevated" {
		addFlag(&flags, seen, "energy_price_pressure")
	}

	// Ordered regime assignment (first match wins).
	switch {
	case curveInv && credStress && (gdpWeak || gc == "contraction"):
		return Result{
			Regime: "recession_pipeline",
			Score:  -0.82,
			Label:  "Inverted curve with tight credit and weak growth — classic late-cycle / hard-landing pipeline.",
			Flags:  flags,
		}
	case infHot && gcWeak:
		return Result{
			Regime: "stagflation_risk",
			Score:  -0.58,
			Label:  "Inflation stance hot while growth is rolling over — stagflation-style pressure on risk assets.",
			Flags:  flags,
		}
	case infHot && mp == "restrictive" && curveTight:
		return Result{
			Regime: "rising_inflation_tight_policy",
			Score:  -0.48,
			Label:  "Hot inflation met with restrictive policy and a flat/inverted curve — rate and duration headwinds.",
			Flags:  flags,
		}
	case gg == "elevated_stress" && (usdStrong || jpyStress):
		return Result{
			Regime: "global_liquidity_stress",
			Score:  -0.52,
			Label:  "Elevated global stress with USD or JPY stress — tighter financial conditions for EM and commodities.",
			Flags:  flags,
		}
	case infDef:
		return Result{
			Regime: "deflation_risk",
			Score:  -0.18,
			Label:  "Inflation composite skews deflationary — watch growth, credit spreads, and policy response.",
			Flags:  flags,
		}
	case infMod && (mp == "accommodative" || mp == "neutral") && gcStr && !credStress:
		return Result{
			Regime: "goldilocks_light",
			Score:  0.48,
			Label:  "Moderate inflation, constructive growth, benign credit — supportive backdrop for equities.",
			Flags:  flags,
		}
	case !infHot && gcStr && mp != "restrictive" && !credStress:
		return Result{
			Regime: "disinflation_soft_landing",
			Score:  0.32,
			Label:  "Growth holding with inflation not in “hot” territory and spreads contained — soft-landing friendly.",
			Flags:  flags,
		}
	default:
		return Result{
			Regime: "neutral_mixed",
			Score:  0.0,
			Label:  "Macro inputs do not line up into a single high-conviction regime — treat as mixed / data-dependent.",
			Flags:  flags,
		}
	}
}
