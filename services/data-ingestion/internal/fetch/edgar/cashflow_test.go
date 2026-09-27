package edgar

import (
	"os"
	"testing"
)

// Fixtures are trimmed copies of SEC's live companyfacts (2026-09-27); the
// expected values were checked against each 20-F's rendered cash-flow statement.
func TestParseCashFlow20F(t *testing.T) {
	cases := []struct {
		ticker, currency, periodEnd, filed string
		fy                                 int
		lines                              map[string]float64
		missing                            []string
	}{
		{"TSM", "TWD", "2024-12-31", "2025-04-17", 2024, map[string]float64{
			"operating": 1826177100000, "investing": -864842800000, "financing": -346301000000,
			"capex": 956006500000, "dividends": 363055200000,
		}, []string{"buybacks"}},
		{"SHEL", "USD", "2025-12-31", "2026-03-12", 2025, map[string]float64{
			"operating": 42863000000, "investing": -16811000000, "financing": -35812000000,
			"buybacks": 13879000000, "dividends": 8472000000,
		}, []string{"capex"}},
		{"TTE", "USD", "2025-12-31", "2026-03-27", 2025, map[string]float64{
			"operating": 27343000000, "investing": -18131000000, "financing": -9934000000,
			"buybacks": 7714000000, "dividends": 8121000000,
		}, []string{"capex"}},
	}
	for _, c := range cases {
		body, err := os.ReadFile("testdata/companyfacts_" + c.ticker + ".json")
		if err != nil {
			t.Fatal(err)
		}
		cf, err := ParseCashFlow20F(body)
		if err != nil || cf == nil {
			t.Fatalf("%s: %v, %v", c.ticker, cf, err)
		}
		if cf.Currency != c.currency || cf.PeriodEnd != c.periodEnd || cf.Filed != c.filed || cf.FiscalYear != c.fy || cf.Form != "20-F" {
			t.Errorf("%s: %s %s filed %s FY%d %s", c.ticker, cf.Currency, cf.PeriodEnd, cf.Filed, cf.FiscalYear, cf.Form)
		}
		for k, want := range c.lines {
			if got, ok := cf.Lines[k]; !ok || got != want {
				t.Errorf("%s %s = %v (present %v), want %v", c.ticker, k, got, ok, want)
			}
		}
		for _, k := range c.missing {
			if v, ok := cf.Lines[k]; ok {
				t.Errorf("%s %s = %v, want missing (the filer uses its own tag)", c.ticker, k, v)
			}
		}
	}
}

// TSM's USD figures are a convenience translation of the current year only;
// the parser must never pick them.
func TestParseCashFlow20F_ExcludesConvenienceTranslation(t *testing.T) {
	body, _ := os.ReadFile("testdata/companyfacts_TSM.json")
	cf, err := ParseCashFlow20F(body)
	if err != nil || cf.Currency != "TWD" || cf.Lines["operating"] == 55693000000 {
		t.Fatalf("currency %v operating %v err %v", cf.Currency, cf.Lines["operating"], err)
	}
}

func TestParseCashFlow20F_NoIFRSFacts(t *testing.T) {
	cf, err := ParseCashFlow20F([]byte(`{"facts":{"us-gaap":{}}}`))
	if err != nil || cf != nil {
		t.Errorf("= %v, %v; want nil, nil", cf, err)
	}
}

func TestLatest20F(t *testing.T) {
	body, _ := os.ReadFile("testdata/submissions_TSM.json")
	f, err := latest20F(body)
	if err != nil || f == nil || f.Filed != "2026-04-16" || f.PeriodEnd != "2025-12-31" || f.Accession != "0001628280-26-025362" {
		t.Errorf("latest 20-F = %+v, %v", f, err)
	}
}
