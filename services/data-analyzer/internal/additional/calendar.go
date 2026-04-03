package additional

import "time"

// SeasonalityMonth is static historical bias from additional_analysis_reference.html
// (S&P 500 monthly averages — tie-breaker only, not a trade signal alone).
type SeasonalityMonth struct {
	Month   int     `json:"month"`
	Name    string  `json:"name"`
	Bias    string  `json:"bias"` // strong_bull | mild_bull | neutral | mild_bear | weak_bear
	Score   float64 `json:"score"` // -1 .. +1 for compositing
	Note    string  `json:"note"`
	AvgHist string  `json:"avg_hist,omitempty"`
}

func monthName(m int) string {
	names := []string{"", "January", "February", "March", "April", "May", "June",
		"July", "August", "September", "October", "November", "December"}
	if m < 1 || m > 12 {
		return ""
	}
	return names[m]
}

// StaticAlmanacSeasonality returns the reference card for calendar month `month` (1–12).
func StaticAlmanacSeasonality(month int) SeasonalityMonth {
	// Avg approximations from reference HTML; score is a small tilt for dashboards only.
	table := map[int]SeasonalityMonth{
		1:  {Bias: "mild_bull", Score: 0.35, Note: "January effect — small caps often lead.", AvgHist: "~+1.1%"},
		2:  {Bias: "neutral", Score: 0.0, Note: "Often choppy, no strong seasonal edge.", AvgHist: "~+0.1%"},
		3:  {Bias: "mild_bull", Score: 0.4, Note: "Quarter-end rebalancing flows.", AvgHist: "~+1.3%"},
		4:  {Bias: "strong_bull", Score: 0.7, Note: "Historically one of the strongest months.", AvgHist: "~+1.5%"},
		5:  {Bias: "neutral", Score: 0.05, Note: "Sell-in-May transition — mixed.", AvgHist: "~+0.2%"},
		6:  {Bias: "neutral", Score: 0.0, Note: "Flat on average, can be choppy.", AvgHist: "~0%"},
		7:  {Bias: "mild_bull", Score: 0.4, Note: "Mid-year rally common.", AvgHist: "~+1.3%"},
		8:  {Bias: "mild_bear", Score: -0.15, Note: "Low liquidity, volatile.", AvgHist: "~−0.1%"},
		9:  {Bias: "weak_bear", Score: -0.5, Note: "September effect — worst month on average.", AvgHist: "~−0.7%"},
		10: {Bias: "mild_bull", Score: 0.25, Note: "Reversal month — crashes and recoveries both.", AvgHist: "~+0.9%"},
		11: {Bias: "strong_bull", Score: 0.65, Note: "Strong seasonal bid (Santa run-up starts).", AvgHist: "~+1.7%"},
		12: {Bias: "strong_bull", Score: 0.55, Note: "Santa rally, window dressing.", AvgHist: "~+1.5%"},
	}
	s, ok := table[month]
	if !ok {
		return SeasonalityMonth{Month: month, Name: monthName(month), Bias: "neutral", Score: 0, Note: "—"}
	}
	s.Month = month
	s.Name = monthName(month)
	return s
}

// PresidentialCycle describes the 4-year US election cycle phase (reference averages).
type PresidentialCycle struct {
	CycleYear int    `json:"cycle_year"` // 1–4, year 1 = first year after election
	Label     string `json:"label"`
	Bias      string `json:"bias"`
	Note      string `json:"note"`
}

// ComputePresidentialCycle uses a fixed 4-year anchor from 2021 = cycle year 1 (post-2020 election term).
func ComputePresidentialCycle(now time.Time) PresidentialCycle {
	y := now.UTC().Year()
	// 2021 -> 1, 2022 -> 2, 2023 -> 3, 2024 -> 4, 2025 -> 1, ...
	const base = 2021
	cy := ((y-base)%4 + 4) % 4
	year := cy + 1

	switch year {
	case 1:
		return PresidentialCycle{
			CycleYear: year,
			Label:     "post_election",
			Bias:      "moderate_constructive",
			Note:      "Year 1 — policy honeymoon; reference ~+6–7% avg historically (not a forecast).",
		}
	case 2:
		return PresidentialCycle{
			CycleYear: year,
			Label:     "midterm",
			Bias:      "choppy",
			Note:      "Year 2 — midterm year; often volatile, corrections common before midterms.",
		}
	case 3:
		return PresidentialCycle{
			CycleYear: year,
			Label:     "pre_election",
			Bias:      "strong_historical",
			Note:      "Year 3 — historically strongest of the four (reference only; macro can dominate).",
		}
	default: // 4
		return PresidentialCycle{
			CycleYear: year,
			Label:     "election",
			Bias:      "positive_volatile",
			Note:      "Year 4 — election year: often positive but choppy around the vote.",
		}
	}
}
