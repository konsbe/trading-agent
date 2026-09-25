package severity

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"sort"
	"strconv"
	"testing"
)

func botFile(t *testing.T, rel string) string {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	b, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "..", "..", "analyst-bot", rel))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestEveryAlertKindHasASeverity(t *testing.T) {
	for _, k := range AlertKinds {
		if _, ok := ForAlert(k, "strong"); !ok {
			t.Errorf("alert kind %q has no severity", k)
		}
	}
	if _, ok := ForAlert("atr_pct_elevated", ""); ok {
		t.Error("an unknown kind must report ok=false, not a default")
	}
}

func TestEveryChartPatternHasASeverity(t *testing.T) {
	for _, p := range ChartPatterns {
		for _, confirmed := range []bool{true, false} {
			if _, ok := ForPattern(p, confirmed); !ok {
				t.Errorf("pattern %q (confirmed=%v) has no severity", p, confirmed)
			}
		}
	}
	for p := range byPattern {
		if !slices.Contains(ChartPatterns, p) {
			t.Errorf("pattern %q is mapped but not listed in ChartPatterns", p)
		}
	}
	if l, _ := ForPattern(BearFlag, true); l != Notice {
		t.Errorf("confirmed pattern = %q, want notice", l)
	}
}

func TestFATierFlipIsDirectional(t *testing.T) {
	if l, _ := ForAlert(FATierFlip, "weak"); l != Warning {
		t.Errorf("flip to weak = %q", l)
	}
	if l, _ := ForAlert(FATierFlip, "strong"); l != Notice {
		t.Errorf("flip to strong = %q", l)
	}
}

func TestSweepActionSeverity(t *testing.T) {
	cases := []struct {
		action string
		conf   int
		want   Level
	}{
		{"TRIM_WATCH", 4, Warning}, {"TRIM_WATCH", 3, Notice},
		{"BUY_WATCH", 4, Notice}, {"WATCH", 1, Notice},
	}
	for _, c := range cases {
		if got := ForSweepAction(c.action, c.conf); got != c.want {
			t.Errorf("%s %d/4 = %q, want %q", c.action, c.conf, got, c.want)
		}
	}
}

// The Python table writes fired_alerts.severity; this one serves the API. They
// must be the same table.
func TestMatchesThePythonAlertSeverityTable(t *testing.T) {
	src := botFile(t, filepath.Join("reports", "alert_severity.py"))
	body := regexp.MustCompile(`(?s)_BY_KIND: dict\[str, str\] = \{(.*?)\n\}`).FindStringSubmatch(src)
	if body == nil {
		t.Fatal("_BY_KIND not found in alert_severity.py")
	}
	py := map[string]Level{}
	for _, m := range regexp.MustCompile(`"(\w+)":\s*"(\w+)"`).FindAllStringSubmatch(body[1], -1) {
		py[m[1]] = Level(m[2])
	}
	if len(py) == 0 {
		t.Fatal("parsed no entries from _BY_KIND")
	}
	for k, want := range py {
		if got := byKind[k]; got != want {
			t.Errorf("%s: python %q, go %q", k, want, got)
		}
	}
	for k := range byKind {
		if _, ok := py[k]; !ok {
			t.Errorf("%s mapped in Go but not in alert_severity.py", k)
		}
	}
	dir := regexp.MustCompile(`_DIRECTIONAL_KINDS = frozenset\(\{([^}]*)\}\)`).FindStringSubmatch(src)
	if dir == nil || dir[1] != `"fa_tier_flip"` {
		t.Fatalf("directional kinds = %v, want exactly fa_tier_flip", dir)
	}
	if !regexp.MustCompile(`return "warning" if alert\.payload\.get\("new_tier"\) == "weak" else "notice"`).MatchString(src) {
		t.Error("fa_tier_flip direction rule changed in alert_severity.py")
	}
}

// Every kind the scan emits is in AlertKinds.
func TestAlertKindsAreWhatTheScanEmits(t *testing.T) {
	src := botFile(t, filepath.Join("reports", "builder.py"))
	seen := map[string]bool{}
	for _, m := range regexp.MustCompile(`kind="(\w+)"`).FindAllStringSubmatch(src, -1) {
		seen[m[1]] = true
	}
	var emitted []string
	for k := range seen {
		emitted = append(emitted, k)
	}
	sort.Strings(emitted)
	want := slices.Clone(AlertKinds)
	sort.Strings(want)
	if !slices.Equal(emitted, want) {
		t.Errorf("scan emits %v, Go AlertKinds %v", emitted, want)
	}
}

func TestVIXAlertThresholdMatchesTheBot(t *testing.T) {
	m := regexp.MustCompile(`bot_vix_alert_threshold:\s*float\s*=\s*([0-9.]+)`).FindStringSubmatch(botFile(t, "config.py"))
	if m == nil {
		t.Fatal("bot_vix_alert_threshold not found in config.py")
	}
	if v, _ := strconv.ParseFloat(m[1], 64); v != BotVIXAlertThreshold {
		t.Errorf("bot default %v, Go %v", v, BotVIXAlertThreshold)
	}
}
