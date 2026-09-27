package momentumapi

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strconv"
	"strings"
	"testing"
)

func parseGo(t *testing.T, path string) *ast.File {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

// fieldLiterals returns every string literal assigned to `field:` in a
// composite literal of the file.
func fieldLiterals(t *testing.T, path, field string) map[string]bool {
	out := map[string]bool{}
	ast.Inspect(parseGo(t, path), func(n ast.Node) bool {
		kv, ok := n.(*ast.KeyValueExpr)
		if !ok {
			return true
		}
		if k, ok := kv.Key.(*ast.Ident); ok && k.Name == field {
			if lit, ok := kv.Value.(*ast.BasicLit); ok && lit.Kind == token.STRING {
				s, _ := strconv.Unquote(lit.Value)
				out[s] = true
			}
		}
		return true
	})
	return out
}

// firstReturnLiterals returns the first result of every return in function fn.
func firstReturnLiterals(t *testing.T, path, fn string) map[string]bool {
	out := map[string]bool{}
	for _, d := range parseGo(t, path).Decls {
		f, ok := d.(*ast.FuncDecl)
		if !ok || f.Name.Name != fn {
			continue
		}
		ast.Inspect(f, func(n ast.Node) bool {
			r, ok := n.(*ast.ReturnStmt)
			if ok && len(r.Results) > 0 {
				if lit, ok := r.Results[0].(*ast.BasicLit); ok {
					s, _ := strconv.Unquote(lit.Value)
					out[s] = true
				}
			}
			return true
		})
	}
	return out
}

// sharedText loads the real shared/content/market_report_descriptions.json —
// the file analyst-bot reads too.
func sharedText(t *testing.T) MarketReportText {
	t.Helper()
	txt, err := LoadMarketReportText(sharedContentPath(t, "market_report_descriptions.json"))
	if err != nil {
		t.Fatal(err)
	}
	return txt
}

func TestMarketReportTextCoversEveryCode(t *testing.T) {
	txt := sharedText(t)
	cycleText, regimeText, intermarketText := txt.MarketCycle, txt.MacroRegime, txt.Intermarket
	check := func(what string, codes map[string]bool, table map[string]string) {
		if len(codes) < 3 {
			t.Fatalf("%s: found %d codes; the parser no longer finds them", what, len(codes))
		}
		for c := range codes {
			if _, ok := table[c]; !ok {
				t.Errorf("%s code %q has no display text", what, c)
			}
		}
		for c := range table {
			if !codes[c] {
				t.Errorf("%s display text for %q, which the producer no longer emits", what, c)
			}
		}
	}
	check("market cycle", fieldLiterals(t, "../marketcycle/composite.go", "Phase"), cycleText)
	check("macro regime", fieldLiterals(t, "../macrocorr/regime.go", "Regime"), regimeText)
	for pair, fn := range map[string]string{"bond_equity_60d": "regimeBondEquity", "oil_equity_60d": "regimeOilEquity", "vix_equity_60d": "regimeVIXEquity"} {
		check(pair, firstReturnLiterals(t, "../additional/intermarket.go", fn), intermarketText[pair])
	}
	for _, texts := range []map[string]string{cycleText, regimeText} {
		for code, text := range texts {
			for _, bad := range []string{"tighten stops", "Reduce risk", "hedge", "size dips", "wait for", "watch ", "playbook"} {
				if strings.Contains(text, bad) {
					t.Errorf("%s display text still instructs the reader (%q): %s", code, bad, text)
				}
			}
		}
	}
}

// Live payloads on 2026-09-27 (trimmed): the stored lines are replaced by the
// description of the code; other fields and unknown codes are untouched.
func TestMarketReportTextRewritesStoredLines(t *testing.T) {
	txt := sharedText(t)
	cycleText, intermarketText := txt.MarketCycle, txt.Intermarket
	cycle := rewriteText(json.RawMessage(`{"composite_phase": "late_cycle_stretched", "composite_label": "Price extended vs 200DMA with tight macro (policy/inflation) — late-cycle playbook; tighten stops.", "tone": "neutral"}`),
		"composite_phase", "composite_label", cycleText)
	var c map[string]any
	_ = json.Unmarshal(cycle, &c)
	if c["composite_label"] != cycleText["late_cycle_stretched"] || c["tone"] != "neutral" {
		t.Errorf("cycle = %s", cycle)
	}
	unknown := json.RawMessage(`{"composite_phase": "zz_new", "composite_label": "as stored"}`)
	if got := rewriteText(unknown, "composite_phase", "composite_label", cycleText); string(got) != string(unknown) {
		t.Errorf("unknown code rewritten: %s", got)
	}
	im := rewriteIntermarket(json.RawMessage(`{"bond_equity_60d": {"regime": "inflationary_positive", "label": "Positive correlation — bonds may not hedge equity drawdowns (inflation / rates shock pattern).", "correlation_60d": 0.528}, "oil_equity_60d": {"regime": "decoupled", "label": "Negative correlation — possible supply-shock or defensive equity phase vs energy."}}`), intermarketText)
	var p map[string]map[string]any
	_ = json.Unmarshal(im, &p)
	if p["bond_equity_60d"]["label"] != intermarketText["bond_equity_60d"]["inflationary_positive"] || p["bond_equity_60d"]["correlation_60d"] != 0.528 {
		t.Errorf("bond pair = %v", p["bond_equity_60d"])
	}
	if p["oil_equity_60d"]["label"] != intermarketText["oil_equity_60d"]["decoupled"] {
		t.Errorf("oil pair = %v", p["oil_equity_60d"])
	}
	s := rewriteSeasonality(json.RawMessage(`{"month": 9, "disclaimer": "Static almanac from reference doc — tie-breaker only; do not trade in isolation."}`), txt.SeasonalityDisclaimer)
	if !strings.Contains(string(s), txt.SeasonalityDisclaimer) || strings.Contains(string(s), "do not trade") {
		t.Errorf("seasonality = %s", s)
	}
}

func TestLoadMarketReportText_RefusesMissingOrIncomplete(t *testing.T) {
	if _, err := LoadMarketReportText(t.TempDir() + "/absent.json"); err == nil {
		t.Error("a missing file loaded")
	}
	p := t.TempDir() + "/partial.json"
	if err := os.WriteFile(p, []byte(`{"market_cycle": {"x": "y"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadMarketReportText(p); err == nil {
		t.Error("a file without the regime, intermarket and disclaimer sections loaded")
	}
}
