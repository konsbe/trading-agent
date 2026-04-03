package marketops

// ReferenceModules mirrors the five tabs in market_operations_reference.html
// and documents automation status for the bot /marketops embed.
func ReferenceModules() map[string]any {
	return map[string]any{
		"positioning": map[string]any{
			"status": "not_automated",
			"hint":   "COT / CTA / risk-parity aggregates — needs dedicated ingestion; see HTML Tier 1.",
		},
		"volatility_regimes": map[string]any{
			"status": "partial_live",
			"hint":   "VIX level + regime from macro_fred in mo_reference_snapshot; per-symbol ATR% in analyst-bot.",
		},
		"liquidity_flows": map[string]any{
			"status": "needs_data",
			"hint":   "ETF / fund flows — paid or manual; not wired.",
		},
		"risk_execution": map[string]any{
			"status": "partial_live",
			"hint":   "Volume vs median + ATR% flags on /analyze — not position sizing or stop prices.",
		},
		"market_structure": map[string]any{
			"status": "not_automated",
			"hint":   "Session / auction / microstructure — reference doc; US equity-centric.",
		},
	}
}
