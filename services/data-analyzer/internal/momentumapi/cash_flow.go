package momentumapi

import "github.com/konsbe/trading-agent/services/data-analyzer/internal/store"

// Stock Detail's cash-flow card: the latest annual (10-K) cash-flow statement,
// never a single quarter — a 10-Q's cash-flow figures are fiscal-year-to-date.
// A symbol with no statement gets a reason, never an empty card.

type cashFlowLine struct {
	Key   string   `json:"key"`
	Label string   `json:"label"`
	Value *float64 `json:"value"` // USD as filed; payments are positive amounts paid
}

type cashFlowOut struct {
	Available bool           `json:"available"`
	Form      *string        `json:"form"`
	PeriodEnd *string        `json:"period_end"`
	Filed     *string        `json:"filed"`
	Lines     []cashFlowLine `json:"lines"`
	// UnavailableReason is set exactly when Available is false.
	UnavailableReason *string `json:"unavailable_reason"`
}

// cashFlowLines: each card line and the XBRL concepts it may be filed under,
// first match wins.
var cashFlowLines = []struct {
	key, label string
	concepts   []string
}{
	{"operating", "Net cash from operating activities", []string{"NetCashProvidedByUsedInOperatingActivities", "NetCashProvidedByUsedInOperatingActivitiesContinuingOperations"}},
	{"investing", "Net cash from investing activities", []string{"NetCashProvidedByUsedInInvestingActivities", "NetCashProvidedByUsedInInvestingActivitiesContinuingOperations"}},
	{"financing", "Net cash from financing activities", []string{"NetCashProvidedByUsedInFinancingActivities", "NetCashProvidedByUsedInFinancingActivitiesContinuingOperations"}},
	{"capex", "Capital spending (property, plant & equipment)", []string{"PaymentsToAcquirePropertyPlantAndEquipment", "PaymentsToAcquireProductiveAssets"}},
	{"buybacks", "Share buybacks", []string{"PaymentsForRepurchaseOfCommonStock"}},
	{"dividends", "Dividends paid", []string{"PaymentsOfDividendsCommonStock", "PaymentsOfDividends", "PaymentsOfOrdinaryDividends"}},
}

func buildCashFlow(in store.AnalysisInputs) cashFlowOut {
	out := cashFlowOut{Lines: []cashFlowLine{}}
	if cf := in.CashFlow; cf != nil {
		out.Available = true
		out.Form, out.PeriodEnd, out.Filed = strPtr(cf.Form), strPtr(cf.EndDate), strPtr(cf.FiledDate)
		for _, l := range cashFlowLines {
			line := cashFlowLine{Key: l.key, Label: l.label}
			for _, c := range l.concepts {
				if v, ok := cf.Lines[c]; ok {
					line.Value = finite(&v)
					break
				}
			}
			out.Lines = append(out.Lines, line)
		}
		return out
	}
	out.UnavailableReason = strPtr(cashFlowUnavailableReason(in.Coverage))
	return out
}

func cashFlowUnavailableReason(c store.StatementCoverage) string {
	switch {
	case c.AssetType != nil && *c.AssetType == "crypto":
		return "No cash-flow statement available: crypto assets file no financial statements."
	case c.AssetType != nil && *c.AssetType == "etf":
		return "No cash-flow statement available: funds file no company cash-flow statement."
	case c.Listing != nil && *c.Listing == "foreign":
		return "No cash-flow statement available: a foreign listing, which the app's filings source (US 10-K/10-Q) does not cover."
	case c.Status != nil && *c.Status == "none_returned":
		reason := "the filings source returned no 10-K/10-Q filings for this company"
		if c.Reason != nil && *c.Reason != "" {
			reason = *c.Reason
		}
		return "No cash-flow statement available: " + reason + " (companies filing 20-F annual reports, such as many foreign issuers, are not covered)."
	}
	return "No cash-flow statement available: no filings are stored for this symbol yet. They are fetched once it is followed, on the watchlist, a candidate, or computed."
}
