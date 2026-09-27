package momentumapi

import "github.com/konsbe/trading-agent/services/data-analyzer/internal/store"

// Stock Detail's cash-flow card: the latest annual cash-flow statement — a 10-K
// via Finnhub, else a 20-F filer's IFRS statement via SEC EDGAR — never a
// single quarter (a 10-Q's cash-flow figures are fiscal-year-to-date). Values
// are in the filing's reporting currency, named, never converted. A symbol with
// no statement gets a reason, never an empty card.

type cashFlowLine struct {
	Key   string   `json:"key"`
	Label string   `json:"label"`
	Value *float64 `json:"value"` // in Currency, as filed; payments are positive amounts paid
	// MissingNote says why Value is null: "not in filing" (10-K) or "not
	// reported as a comparable line" (20-F: the filer reports it under its own
	// extension concept, which companyfacts does not carry, or folded into a
	// broader standard line — BP's capex includes intangibles and other assets).
	MissingNote *string `json:"missing_note"`
}

// newerFiling: a newer 20-F than the one shown is on EDGAR's filing index but
// not in companyfacts yet.
type newerFiling struct {
	Form      string `json:"form"`
	Filed     string `json:"filed"`
	PeriodEnd string `json:"period_end"`
}

type cashFlowOut struct {
	Available bool    `json:"available"`
	Form      *string `json:"form"`
	PeriodEnd *string `json:"period_end"`
	Filed     *string `json:"filed"`
	// FiscalYear is the year of PeriodEnd; Currency the reporting currency
	// (ISO code); Source "10-K via Finnhub" or "20-F via SEC EDGAR".
	FiscalYear  *int           `json:"fiscal_year"`
	Currency    *string        `json:"currency"`
	Source      *string        `json:"source"`
	NewerFiling *newerFiling   `json:"newer_filing"`
	Lines       []cashFlowLine `json:"lines"`
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

const (
	notInFiling       = "not in filing"
	notComparableLine = "not reported as a comparable line"
	source10KFinnhub  = "10-K via Finnhub"
	source20FEdgar    = "20-F via SEC EDGAR"
)

func buildCashFlow(in store.AnalysisInputs) cashFlowOut {
	out := cashFlowOut{Lines: []cashFlowLine{}}
	if cf := in.CashFlow; cf != nil {
		out.Available = true
		out.Form, out.PeriodEnd, out.Filed = strPtr(cf.Form), strPtr(cf.EndDate), strPtr(cf.FiledDate)
		out.FiscalYear, out.Currency, out.Source = yearOf(cf.EndDate), strPtr("USD"), strPtr(source10KFinnhub)
		for _, l := range cashFlowLines {
			line := cashFlowLine{Key: l.key, Label: l.label}
			for _, c := range l.concepts {
				if v, ok := cf.Lines[c]; ok {
					line.Value = finite(&v)
					break
				}
			}
			if line.Value == nil {
				line.MissingNote = strPtr(notInFiling)
			}
			out.Lines = append(out.Lines, line)
		}
		return out
	}
	if cf := in.CashFlow20F; cf != nil {
		out.Available = true
		out.Form, out.PeriodEnd, out.Filed = strPtr(cf.Form), strPtr(cf.PeriodEnd), strPtr(cf.Filed)
		fy := cf.FiscalYear
		out.FiscalYear, out.Currency, out.Source = &fy, strPtr(cf.Currency), strPtr(source20FEdgar)
		if l := cf.Latest20F; l != nil && l.PeriodEnd > cf.PeriodEnd {
			out.NewerFiling = &newerFiling{Form: "20-F", Filed: l.Filed, PeriodEnd: l.PeriodEnd}
		}
		for _, l := range cashFlowLines {
			line := cashFlowLine{Key: l.key, Label: l.label}
			if v, ok := cf.Lines[l.key]; ok {
				line.Value = finite(&v)
			} else {
				line.MissingNote = strPtr(notComparableLine)
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
		return "No cash-flow statement available: a foreign listing, which the app's filings sources (10-K via Finnhub, 20-F via SEC EDGAR) do not cover."
	case c.Status != nil && *c.Status == "none_returned":
		reason := "the filings source returned no 10-K/10-Q filings for this company"
		if c.Reason != nil && *c.Reason != "" {
			reason = *c.Reason
		}
		return "No cash-flow statement available: " + reason + ", and SEC EDGAR has no IFRS (20-F) cash-flow statement for it either."
	}
	return "No cash-flow statement available: no filings are stored for this symbol yet. They are fetched once it is followed, on the watchlist, a candidate, or computed."
}

func yearOf(date string) *int {
	if len(date) < 4 {
		return nil
	}
	y := 0
	for _, c := range date[:4] {
		if c < '0' || c > '9' {
			return nil
		}
		y = y*10 + int(c-'0')
	}
	return &y
}
