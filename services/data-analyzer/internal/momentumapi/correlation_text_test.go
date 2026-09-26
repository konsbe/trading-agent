package momentumapi

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// analyzerSentences returns every format string appended to a *Warnings or
// *Positives slice in internal/fundamental/analyze.go, literal or via
// fmt.Sprintf.
func analyzerSentences(t *testing.T) []string {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), "../fundamental/analyze.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || len(call.Args) != 2 || call.Ellipsis.IsValid() {
			return true
		}
		if fn, ok := call.Fun.(*ast.Ident); !ok || fn.Name != "append" {
			return true
		}
		slice, ok := call.Args[0].(*ast.Ident)
		if !ok || !(strings.HasSuffix(slice.Name, "Warnings") || strings.HasSuffix(slice.Name, "Positives")) {
			return true
		}
		arg := call.Args[1]
		if c, ok := arg.(*ast.CallExpr); ok && len(c.Args) > 0 {
			arg = c.Args[0] // fmt.Sprintf(format, ...)
		}
		lit, ok := arg.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			t.Errorf("append to %s with a non-literal sentence; the display table cannot match it", slice.Name)
			return true
		}
		s, err := strconv.Unquote(lit.Value)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, s)
		return true
	})
	return out
}

func TestCorrelationSentencesCoverAnalyzer(t *testing.T) {
	source := analyzerSentences(t)
	if len(source) < 30 {
		t.Fatalf("found %d sentences in analyze.go; the parser no longer finds them", len(source))
	}
	table := map[string]bool{}
	for _, s := range correlationSentences {
		if table[s.stored] {
			t.Errorf("duplicate table entry %q", s.stored)
		}
		table[s.stored] = true
		if want, got := strings.Count(s.stored, "%.1f"), strings.Count(s.display, "%s"); want != got {
			t.Errorf("%q: %d numbers stored, %d in the display text", s.stored, want, got)
		}
	}
	inSource := map[string]bool{}
	for _, s := range source {
		inSource[s] = true
		if !table[s] {
			t.Errorf("analyze.go sentence has no display text: %q", s)
		}
	}
	var stale []string
	for s := range table {
		if !inSource[s] {
			stale = append(stale, s)
		}
	}
	sort.Strings(stale)
	for _, s := range stale {
		t.Errorf("display table entry no longer in analyze.go: %q", s)
	}
}

// Rendered as analyze.go renders them, including live values (COHR's P/E
// warning, NVDA's revenue growth 83.4%), every stored sentence maps.
func TestCorrelationDisplayText(t *testing.T) {
	for _, tc := range []struct{ stored, want string }{
		{"High D/E + thin net margin 3.1% — any revenue shortfall can cascade to insolvency risk",
			"High debt / equity with a net margin of 3.1%"},
		{"High ROIC + strong revenue growth — compounding machine, every reinvested dollar earns above cost of capital",
			"High ROIC with strong revenue growth"},
		{"Fast revenue growth (83.4%) with compressing gross margin — scaling without pricing power",
			"Revenue growth of 83.4% with a compressing gross margin trend"},
		{"High P/E 41.2× + low EPS growth -3.5% — valuation trap risk, paying premium for deteriorating earnings",
			"High P/E (41.2×) with low EPS growth (-3.5%)"},
		{"Dividend yield 4.2% > FCF yield 1.9% — dividend not covered by free cash flow, cut risk elevated",
			"Dividend yield (4.2%) above FCF yield (1.9%)"},
		{"a sentence the table does not know", "a sentence the table does not know"},
	} {
		if got := correlationDisplayText(tc.stored); got != tc.want {
			t.Errorf("correlationDisplayText(%q) = %q, want %q", tc.stored, got, tc.want)
		}
	}
	for _, s := range correlationSentences {
		rendered := s.stored
		if n := strings.Count(s.stored, "%.1f"); n > 0 {
			args := make([]any, n)
			for i := range args {
				args[i] = 12.5
			}
			rendered = fmt.Sprintf(s.stored, args...)
		}
		if got := correlationDisplayText(rendered); got == rendered || strings.Contains(got, "%!") {
			t.Errorf("rendered %q maps to %q", rendered, got)
		}
	}
}
