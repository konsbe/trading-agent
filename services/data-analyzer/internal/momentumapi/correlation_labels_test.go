package momentumapi

import (
	"go/ast"
	"go/token"
	"os"
	"sort"
	"strconv"
	"testing"
)

func loadSharedCorrelationLabels(t *testing.T) CorrelationLabels {
	t.Helper()
	l, err := LoadCorrelationLabels(sharedContentPath(t, "correlation_labels.json"))
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func keys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func sortedSet(s map[string]bool) []string {
	out := make([]string, 0, len(s))
	for k := range s {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// analyzerCorrelationCodes parses internal/fundamental/analyze.go for every
// code the correlation pass can store: corrTier's tiers, netLabel's values and
// the pattern keys of the corr_master_signals payload.
func analyzerCorrelationCodes(t *testing.T) (tiers, net, patterns map[string]bool) {
	t.Helper()
	tiers = firstReturnLiterals(t, "../fundamental/analyze.go", "corrTier")
	net, patterns = map[string]bool{}, map[string]bool{}
	ast.Inspect(parseGo(t, "../fundamental/analyze.go"), func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.AssignStmt:
			if id, ok := x.Lhs[0].(*ast.Ident); ok && id.Name == "netLabel" {
				if lit, ok := x.Rhs[0].(*ast.BasicLit); ok && lit.Kind == token.STRING {
					v, _ := strconv.Unquote(lit.Value)
					net[v] = true
				}
			}
		case *ast.CallExpr:
			if len(x.Args) != 3 {
				return true
			}
			if lit, ok := x.Args[0].(*ast.BasicLit); !ok || lit.Value != `"corr_master_signals"` {
				return true
			}
			cl, ok := x.Args[2].(*ast.CompositeLit)
			if !ok {
				return true
			}
			for _, e := range cl.Elts {
				kv := e.(*ast.KeyValueExpr)
				k, _ := strconv.Unquote(kv.Key.(*ast.BasicLit).Value)
				// Patterns are the masterSig values m1…m5 ("bullish_convergence": m1);
				// the counts and net_signal are other idents.
				if id, ok := kv.Value.(*ast.Ident); ok && len(id.Name) == 2 && id.Name[0] == 'm' {
					patterns[k] = true
				}
			}
		}
		return true
	})
	return tiers, net, patterns
}

func TestCorrelationLabelsCoverEveryCode(t *testing.T) {
	l := loadSharedCorrelationLabels(t)
	tiers, net, patterns := analyzerCorrelationCodes(t)
	if len(tiers) < 4 || len(net) < 5 || len(patterns) < 5 {
		t.Fatalf("parser found tiers %v, net %v, patterns %v", tiers, net, patterns)
	}
	check := func(what string, want map[string]bool, got map[string]string) {
		if w, g := sortedSet(want), keys(got); !equalStrings(w, g) {
			t.Errorf("%s: analyze.go emits %v, correlation_labels.json has %v", what, w, g)
		}
	}
	check("cluster tiers", tiers, l.ClusterTiers)
	check("net signal", net, l.NetSignal)
	check("patterns", patterns, l.Patterns)
	clusters, masters := map[string]bool{}, map[string]bool{}
	for _, c := range correlationClusters {
		clusters[c] = true
	}
	for _, m := range masterSignals {
		masters[m] = true
	}
	check("clusters", clusters, l.Clusters)
	check("served patterns", masters, l.Patterns)
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestLoadCorrelationLabels_RefusesMissingOrIncomplete(t *testing.T) {
	if _, err := LoadCorrelationLabels(t.TempDir() + "/absent.json"); err == nil {
		t.Error("a missing file loaded")
	}
	p := t.TempDir() + "/partial.json"
	if err := os.WriteFile(p, []byte(`{"cluster_tiers": {"healthy": "x"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadCorrelationLabels(p); err == nil {
		t.Error("an incomplete file loaded")
	}
}
