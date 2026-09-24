package momentumapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

func sharedReportPath(t *testing.T) string {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "shared", "content", "backtest_lab_report.json")
}

func loadSharedReport(t *testing.T) BacktestReport {
	t.Helper()
	r, err := LoadBacktestReport(sharedReportPath(t))
	if err != nil {
		t.Fatalf("load backtest lab report: %v", err)
	}
	return r
}

// readReport decodes the checked-in file directly, independently of the loader.
func readReport(t *testing.T) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(sharedReportPath(t))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

// at walks a decoded JSON document: keys for objects, ints for arrays.
func at(t *testing.T, v any, path ...any) any {
	t.Helper()
	for _, p := range path {
		switch k := p.(type) {
		case string:
			m, ok := v.(map[string]any)
			if !ok {
				t.Fatalf("path %v: not an object at %q", path, k)
			}
			v, ok = m[k]
			if !ok {
				t.Fatalf("path %v: missing %q", path, k)
			}
		case int:
			a, ok := v.([]any)
			if !ok || k >= len(a) {
				t.Fatalf("path %v: no index %d", path, k)
			}
			v = a[k]
		}
	}
	return v
}

// Drift test (addendum §3). The report is hand-transcribed, so load-bearing
// numbers are checked against constants re-read from the specs when this test
// was written — not against the JSON itself.
func TestBacktestReport_LoadBearingNumbersMatchTheSpecs(t *testing.T) {
	r := readReport(t)
	cases := []struct {
		source string
		path   []any
		want   float64
	}{
		// Required by the addendum.
		{"Phase 2 §2.1 RESULT", []any{"entry_gate", "result", "p_value"}, 0.947},
		{"Phase 2 §2.1 RESULT", []any{"entry_gate", "result", "mh_odds_ratio"}, 0.991},
		{"Phase 1 §10.1.4", []any{"v2_score_finding", "composition_share_pct"}, 97},
		{"Phase 2 §2.1 RESULT", []any{"rvol_stratification_funnel", "steps", 3, "odds_ratio"}, 0.991},
		// Further numbers a reader would quote.
		{"Phase 2 §2.1 RESULT", []any{"entry_gate", "sample", "episodes"}, 6936},
		{"Phase 1 §10.1.3", []any{"v2_score_finding", "pooled_result", "p_value"}, 0.017},
		{"Phase 1 §10.1.4", []any{"v2_score_finding", "stratified_result", "penny_bucket_p"}, 0.891},
		{"Phase 1 §10.1.4", []any{"v2_score_finding", "stratified_result", "market_bucket_p"}, 0.645},
		{"Phase 1 §10.1.5a", []any{"rvol_stratification_funnel", "steps", 0, "odds_ratio"}, 1.529},
		{"Phase 1 §10.1.5a", []any{"rvol_stratification_funnel", "steps", 1, "odds_ratio"}, 1.215},
		{"Phase 1 §10.1.5a", []any{"rvol_stratification_funnel", "steps", 2, "odds_ratio"}, 1.192},
		{"Phase 2 §2.3", []any{"research_round_1", "hypotheses", 1, "best_effect", "odds_ratio"}, 1.616},
		{"Phase 2 §2.3", []any{"sample_size", "episodes"}, 7579},
	}
	for _, c := range cases {
		got, ok := at(t, r, c.path...).(float64)
		if !ok || got != c.want {
			t.Errorf("%v = %v, want %v (%s)", c.path, at(t, r, c.path...), c.want, c.source)
		}
	}
	if at(t, r, "research_round_1", "hypotheses", 1, "id") != "b" {
		t.Fatal("hypotheses[1] is not (b); the positional checks above assume the published order a, b, d, e")
	}
}

// Addendum §6: nothing in the report may look like a live score, symbol data,
// or a way to re-run the analysis.
func TestBacktestReport_HasNoLiveScoreSymbolOrRerunFields(t *testing.T) {
	forbiddenKey := regexp.MustCompile(`(^|_)(symbol|symbols|ticker|tickers|live|rerun|refresh|query|url|href|endpoint|link)($|_)`)
	forbiddenExact := map[string]bool{"score": true, "scores": true, "momentum_score_100": true, "score_attainable": true}
	var walk func(path string, v any)
	walk = func(path string, v any) {
		switch x := v.(type) {
		case map[string]any:
			for k, child := range x {
				if forbiddenExact[k] || forbiddenKey.MatchString(k) {
					t.Errorf("%s.%s: field looks like live/symbol/re-run data", path, k)
				}
				walk(path+"."+k, child)
			}
		case []any:
			for i, child := range x {
				walk(path+"["+string(rune('0'+i))+"]", child)
			}
		case string:
			if strings.Contains(x, "/api/") || strings.HasPrefix(x, "http") {
				t.Errorf("%s: %q points at an API or URL", path, x)
			}
		}
	}
	walk("$", readReport(t))
}

// Addendum §6: every hypothesis has a verdict; verdict_note appears only where
// the spec requires an asterisked reading — currently (b) alone.
func TestBacktestReport_VerdictsAndTheOneVerdictNote(t *testing.T) {
	hs := at(t, readReport(t), "research_round_1", "hypotheses").([]any)
	if len(hs) != 4 {
		t.Fatalf("%d hypotheses, want 4 (a, b, d, e — c was abandoned)", len(hs))
	}
	for _, h := range hs {
		m := h.(map[string]any)
		if v, _ := m["verdict"].(string); v == "" {
			t.Errorf("hypothesis %v has no verdict", m["id"])
		}
		_, hasNote := m["verdict_note"]
		if hasNote != (m["id"] == "b") {
			t.Errorf("hypothesis %v: verdict_note present=%v; only (b) carries one", m["id"], hasNote)
		}
	}
	if at(t, readReport(t), "report", "status") != "closed" {
		t.Error("report.status must be \"closed\" in the current build")
	}
	if at(t, readReport(t), "entry_gate", "rule_committed_before_result") != true {
		t.Error("entry_gate.rule_committed_before_result must be true")
	}
}

func TestBacktestReport_ServedAsAuthoredWithALongCache(t *testing.T) {
	srv := newTestServer(t, fixtureStore(), freshNow)
	rec := get(t, srv, "/api/v1/backtest-lab/report")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d %s", rec.Code, rec.Body)
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "public, max-age=86400" {
		t.Errorf("Cache-Control = %q, want a 24h cache (not the 5-minute data TTL)", cc)
	}
	raw, _ := os.ReadFile(sharedReportPath(t))
	var want bytes.Buffer
	_ = json.Compact(&want, raw)
	if !bytes.Equal(rec.Body.Bytes(), want.Bytes()) {
		t.Error("response body differs from the checked-in report")
	}
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
		if rec := request(t, srv, method, "/api/v1/backtest-lab/report"); rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s = %d, want 405 (no write path)", method, rec.Code)
		}
	}
}

func TestLoadBacktestReport_RefusesMissingOrIncompleteFiles(t *testing.T) {
	if _, err := LoadBacktestReport(filepath.Join(t.TempDir(), "absent.json")); err == nil {
		t.Error("a missing report must fail startup")
	}
	dir := t.TempDir()
	for name, body := range map[string]string{
		"not json":        `{`,
		"no report block": `{"entry_gate":{"result":{"p_value":0.9,"mh_odds_ratio":1}},"research_round_1":{"hypotheses":[{"id":"a","verdict":"FAIL"}]},"closing_statement":"x"}`,
		"no entry result": `{"report":{"version":"v","status":"closed","closed_date":"d","headline":"h"},"research_round_1":{"hypotheses":[{"id":"a","verdict":"FAIL"}]},"closing_statement":"x"}`,
		"verdict missing": `{"report":{"version":"v","status":"closed","closed_date":"d","headline":"h"},"entry_gate":{"result":{"p_value":0.9,"mh_odds_ratio":1}},"research_round_1":{"hypotheses":[{"id":"a"}]},"closing_statement":"x"}`,
		"no closing":      `{"report":{"version":"v","status":"closed","closed_date":"d","headline":"h"},"entry_gate":{"result":{"p_value":0.9,"mh_odds_ratio":1}},"research_round_1":{"hypotheses":[{"id":"a","verdict":"FAIL"}]}}`,
	} {
		p := filepath.Join(dir, strings.ReplaceAll(name, " ", "_")+".json")
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadBacktestReport(p); err == nil {
			t.Errorf("%s: loaded, want a startup error", name)
		}
	}
	if _, err := LoadBacktestReport(sharedReportPath(t)); err != nil {
		t.Errorf("the checked-in report must load: %v", err)
	}
}
