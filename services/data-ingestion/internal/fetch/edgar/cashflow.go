package edgar

import (
	"encoding/json"
	"fmt"
	"time"
)

// IFRS cash-flow statement of a 20-F filer, from companyfacts' ifrs-full facts.
// Only the standard ifrs-full concept for each card row is read. A filer that
// reports a row under its own extension tag (Shell's "Cash capital
// expenditure", which companyfacts does not carry) or folds it into a broader
// line (BP's capex includes intangibles and other non-current assets) leaves
// that row missing rather than approximated.

// CashFlowLines maps the Stock Detail card's rows to ifrs-full concepts,
// first present wins. Verified 2026-09-27 against the rendered cash-flow
// statements of TSM, SHEL and TTE: these are the lines on the face of the
// statement (PurchaseOfTreasuryShares and DividendsPaid are note figures that
// differ from it, so they are not used).
var CashFlowLines = []struct {
	Key      string
	Concepts []string
}{
	{"operating", []string{"CashFlowsFromUsedInOperatingActivities"}},
	{"investing", []string{"CashFlowsFromUsedInInvestingActivities"}},
	{"financing", []string{"CashFlowsFromUsedInFinancingActivities"}},
	{"capex", []string{"PurchaseOfPropertyPlantAndEquipmentClassifiedAsInvestingActivities"}},
	{"buybacks", []string{"PaymentsToAcquireOrRedeemEntitysShares"}},
	// Dividends to the parent's shareholders; a filer without a
	// non-controlling split tags the total paid instead.
	{"dividends", []string{"DividendsPaidToEquityHoldersOfParentClassifiedAsFinancingActivities", "DividendsPaidClassifiedAsFinancingActivities"}},
}

// CashFlow20F is one fiscal year's statement as filed.
type CashFlow20F struct {
	Form       string             `json:"form"`
	Accession  string             `json:"accession"`
	Filed      string             `json:"filed"`
	PeriodEnd  string             `json:"period_end"`
	FiscalYear int                `json:"fiscal_year"`
	Currency   string             `json:"currency"`
	Lines      map[string]float64 `json:"lines"`    // card key → value (payments positive, as tagged)
	Concepts   map[string]string  `json:"concepts"` // card key → ifrs-full concept used
}

type fact struct {
	Start string  `json:"start"`
	End   string  `json:"end"`
	Val   float64 `json:"val"`
	Accn  string  `json:"accn"`
	FY    int     `json:"fy"`
	Form  string  `json:"form"`
	Filed string  `json:"filed"`
}

type companyFacts struct {
	Facts map[string]map[string]struct {
		Units map[string][]fact `json:"units"`
	} `json:"facts"`
}

func annual(f fact) bool {
	if f.Start == "" || (f.Form != "20-F" && f.Form != "20-F/A") {
		return false
	}
	s, err1 := time.Parse(time.DateOnly, f.Start)
	e, err2 := time.Parse(time.DateOnly, f.End)
	if err1 != nil || err2 != nil {
		return false
	}
	d := e.Sub(s).Hours() / 24
	return d >= 350 && d <= 380
}

// ParseCashFlow20F returns the newest fiscal year's IFRS cash-flow statement
// from the newest 20-F that tags an operating cash-flow total, or nil when
// there is none.
//
// The reporting currency is the unit carrying the prior-year comparatives in
// that filing. A convenience translation (TSM files NT$ figures with a US$
// copy of the current year only) covers one year and is excluded: figures are
// shown as reported, never converted.
func ParseCashFlow20F(body []byte) (*CashFlow20F, error) {
	var cf companyFacts
	if err := json.Unmarshal(body, &cf); err != nil {
		return nil, fmt.Errorf("companyfacts: %w", err)
	}
	ifrs := cf.Facts["ifrs-full"]
	op, ok := ifrs["CashFlowsFromUsedInOperatingActivities"]
	if !ok {
		return nil, nil
	}
	// The newest filing (by filed date, then accession) with an annual total.
	var filing fact
	for _, facts := range op.Units {
		for _, f := range facts {
			if annual(f) && (f.Filed > filing.Filed || f.Filed == filing.Filed && f.Accn > filing.Accn) {
				filing = f
			}
		}
	}
	if filing.Accn == "" {
		return nil, nil
	}
	// Per unit, the fiscal years that filing presents and its latest period end.
	periods := map[string]map[string]bool{}
	latestEnd := ""
	for unit, facts := range op.Units {
		for _, f := range facts {
			if f.Accn == filing.Accn && annual(f) {
				if periods[unit] == nil {
					periods[unit] = map[string]bool{}
				}
				periods[unit][f.End] = true
				if f.End > latestEnd {
					latestEnd = f.End
				}
			}
		}
	}
	currency, best, tie := "", 0, false
	for unit, ends := range periods {
		switch {
		case len(ends) > best:
			currency, best, tie = unit, len(ends), false
		case len(ends) == best:
			tie = true
		}
	}
	if currency == "" || tie {
		return nil, fmt.Errorf("companyfacts: no single reporting currency in %s (units %v)", filing.Accn, keys(periods))
	}
	end, _ := time.Parse(time.DateOnly, latestEnd)
	out := &CashFlow20F{
		Form: filing.Form, Accession: filing.Accn, Filed: filing.Filed, PeriodEnd: latestEnd,
		FiscalYear: end.Year(), Currency: currency,
		Lines: map[string]float64{}, Concepts: map[string]string{},
	}
	for _, line := range CashFlowLines {
		for _, concept := range line.Concepts {
			v, ok := valueFor(ifrs[concept].Units[currency], filing.Accn, latestEnd)
			if ok {
				out.Lines[line.Key], out.Concepts[line.Key] = v, concept
				break
			}
		}
	}
	return out, nil
}

func valueFor(facts []fact, accn, end string) (float64, bool) {
	for _, f := range facts {
		if f.Accn == accn && f.End == end && annual(f) {
			return f.Val, true
		}
	}
	return 0, false
}

func keys(m map[string]map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
