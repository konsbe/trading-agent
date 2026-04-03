package marketcycle

import "strings"

// Composite blends price phase with latest macro_derived stances (same run order).
type Composite struct {
	Phase string  // machine id
	Label string  // Discord-friendly one-liner
	Score float64 // −1 stress … +1 constructive
}

// BuildComposite maps price + macro regimes to a headline cycle phase (reference HTML matrix).
func BuildComposite(p PriceResult, gcStance, mpStance, infStance, ggStance string) Composite {
	gc := strings.ToLower(strings.TrimSpace(gcStance))
	mp := strings.ToLower(strings.TrimSpace(mpStance))
	inf := strings.ToLower(strings.TrimSpace(infStance))
	gg := strings.ToLower(strings.TrimSpace(ggStance))

	if p.Phase == "insufficient_data" {
		return Composite{
			Phase: "insufficient_equity_data",
			Label: "Need ≥200 daily bars for " + p.Symbol + " in equity_ohlcv (1Day). Ingest SPY/INDEX.",
			Score: 0,
		}
	}

	// 1) Price stress dominates
	switch p.Phase {
	case "crash":
		return Composite{
			Phase: "crash_panic",
			Label: "Crash-style velocity vs recent highs — liquidity/event risk; flat risk until stabilises.",
			Score: -0.95,
		}
	case "bear":
		l := "Bear market drawdown (≤−20% from peak). Capital preservation; wait for macro + price repair."
		if gc == "contraction" {
			l += " Growth cycle confirms contraction."
		}
		return Composite{Phase: "bear_structural", Label: l, Score: -0.78}
	case "correction":
		sc := -0.45
		l := "Correction zone (−10–20%). Reduce risk / hedge; watch credit + VIX."
		if inf == "hot" {
			l += " Hot inflation backdrop adds volatility."
			sc -= 0.1
		}
		if mp == "restrictive" {
			sc -= 0.08
		}
		return Composite{Phase: "correction_risk", Label: l, Score: sc}
	case "pullback":
		sc := -0.12
		l := "Pullback (−3–10%) within uptrend — usually healthy if macro intact."
		if gc == "expansion" || gc == "slowdown" {
			sc += 0.08
		}
		if inf == "hot" {
			l += " Inflation still hot — size dips smaller."
			sc -= 0.05
		}
		return Composite{Phase: "pullback_healthy", Label: l, Score: sc}
	}

	// 2) Upside / late-cycle (price at or above trend)
	if p.Phase == "bull_extended" {
		if inf == "hot" && (mp == "restrictive" || mp == "neutral") {
			return Composite{
				Phase: "late_cycle_stretched",
				Label: "Price extended vs 200DMA with tight macro (policy/inflation) — late-cycle playbook; tighten stops.",
				Score: 0.15,
			}
		}
		if gg == "elevated_stress" || gg == "moderate" {
			return Composite{
				Phase: "bull_fragile_global",
				Label: "Strong price trend but global stress elevated — geopolitical/USD channel can snap leaders.",
				Score: 0.22,
			}
		}
		return Composite{
			Phase: "bull_overextended",
			Label: "Extended above 200DMA — trend strong; add mean-reversion awareness only.",
			Score: 0.48,
		}
	}

	if p.Phase == "below_sma" {
		l := "Below 200DMA but shallow drawdown — trend damage; confirm with growth/credit."
		sc := -0.18
		if gc == "contraction" {
			sc -= 0.2
			l += " Growth in contraction."
		}
		return Composite{Phase: "trend_soft", Label: l, Score: sc}
	}

	// bull (above SMA, not extended) — align with macro table “expansion / bull”
	sc := 0.35
	l := "Above 200DMA with mild drawdown — core bull structure."
	if gc == "expansion" {
		sc += 0.12
		l = "Macro growth expansion + price above 200DMA — classic bull alignment."
	} else if gc == "slowdown" {
		sc += 0.02
		l += " Growth slowing — watch LEI/yield curve."
	} else if gc == "contraction" {
		sc -= 0.25
		l = "Price still above 200DMA but growth cycle weak — bear market rally risk."
	}
	if inf == "hot" {
		sc -= 0.08
		l += " Inflation hot."
	}
	if mp == "accommodative" {
		sc += 0.05
	}
	if mp == "restrictive" {
		sc -= 0.06
	}
	if sc > 0.55 {
		return Composite{Phase: "bull_macro_aligned", Label: l, Score: 0.55}
	}
	if sc < -0.35 {
		return Composite{Phase: "bull_macro_divergent", Label: l, Score: sc}
	}
	return Composite{Phase: "neutral_mixed", Label: l, Score: sc}
}
