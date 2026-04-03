package additional

// ReferenceModules documents every tab in additional_analysis_reference.html vs automation status.
func ReferenceModules() map[string]any {
	return map[string]any{
		"sentiment": map[string]any{
			"status": "needs_data",
			"hint":   "PCR, IV skew, short interest, margin — need options/FINRA feeds (see HTML data sources).",
		},
		"factor": map[string]any{
			"status": "partial",
			"hint":   "Quality/value overlap fundamental-analysis; momentum/vol from technical-analysis OHLCV — no separate factor ETF pipeline yet.",
		},
		"intermarket": map[string]any{
			"status": "partial_live",
			"hint":   "Live rolling ρ in aa_reference_snapshot (bond, oil, VIX vs benchmark). Other pairs in HTML are regime guides.",
		},
		"flow_microstructure": map[string]any{
			"status": "needs_data",
			"hint":   "UOA, GEX, dark pool, max pain — paid options / FINRA OTC APIs not wired.",
		},
		"seasonality": map[string]any{
			"status": "live_static",
			"hint":   "Static month almanac + presidential cycle in payload — not backtested on your DB history.",
		},
		"alternative_data": map[string]any{
			"status": "not_automated",
			"hint":   "Vendor datasets only — no ingestion.",
		},
		"event_driven": map[string]any{
			"status": "not_automated",
			"hint":   "M&A, spinoffs, index adds — would need SEC/index parsers + event tables.",
		},
		"relative_value": map[string]any{
			"status": "not_automated",
			"hint":   "Pairs / sector ratios — configure ETF universe + second worker pass (future).",
		},
	}
}
