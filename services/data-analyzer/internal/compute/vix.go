package compute

// VIXThresholds are the TECHNICAL_VIX_*_THRESHOLD bands. technical-analysis
// (per-symbol vix_regime) and macro-analysis (market-wide mc_vix_regime) both
// classify through ClassifyVIX so the two can never disagree.
type VIXThresholds struct {
	Fear        float64 // > this = extreme_fear
	Elevated    float64 // > this = elevated
	Complacency float64 // < this = complacency
}

// ClassifyVIX returns extreme_fear, elevated, complacency or normal.
func ClassifyVIX(vix float64, th VIXThresholds) string {
	switch {
	case vix > th.Fear:
		return "extreme_fear"
	case vix > th.Elevated:
		return "elevated"
	case vix < th.Complacency:
		return "complacency"
	}
	return "normal"
}
