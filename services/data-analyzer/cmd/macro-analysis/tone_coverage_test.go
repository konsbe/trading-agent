package main

import (
	"os"
	"regexp"
	"testing"

	"github.com/konsbe/trading-agent/services/data-analyzer/internal/macrotone"
)

// regimeVarMetric pairs each regime variable in main.go with the metric whose
// "regime" field it fills.
var regimeVarMetric = map[string]string{
	"mpRateRegime":   "mp_rate",
	"ycRegime":       "mp_yield_curve",
	"rrRegime":       "mp_real_rate",
	"bsRegime":       "mp_balance_sheet",
	"creditRegime":   "mp_credit_spread",
	"beRegime":       "mp_breakeven_inflation",
	"m2Regime":       "mp_m2_supply",
	"pmiRegime":      "gc_pmi",
	"leiRegime":      "gc_lei",
	"claimsRegime":   "gc_claims",
	"housingRegime":  "gc_housing",
	"gdpRegime":      "gc_gdp",
	"emplRegime":     "gc_employment",
	"consumerRegime": "gc_consumer",
	"umichRegime":    "gc_consumer_sentiment",
	"capexRegime":    "gc_capex",
	"corePCERegime":  "inf_core_pce",
	"cpiRegime":      "inf_cpi",
	"coreCPIRegime":  "inf_core_cpi",
	"shelterRegime":  "inf_shelter",
	"ppiRegime":      "inf_ppi",
	"oilRegime":      "inf_oil",
	"wageRegime":     "inf_wages",
	"copperRegime":   "inf_copper",
	"dollarRegime":   "gg_broad_dollar",
	"jpyRegime":      "gg_usdjpy",
	"chinaRegime":    "gg_china_gdp",
	"fiscalRegime":   "gg_fiscal",
}

var stanceVarMetric = map[string]string{
	"stance":    "mp_stance",
	"gcStance":  "gc_stance",
	"infStance": "inf_stance",
	"ggStance":  "gg_stance",
}

// TestEveryEmittedLabelHasATone fails when macro-analysis can write a regime or
// stance label that internal/macrotone does not tone, so the web report never
// receives a classification without a stored tone.
func TestEveryEmittedLabelHasATone(t *testing.T) {
	src := readFile(t, "main.go")

	regimeAssign := regexp.MustCompile(`\b(\w+Regime)\s*:?=\s*"([a-z_]+)"`)
	seen := map[string]bool{}
	for _, m := range regimeAssign.FindAllStringSubmatch(src, -1) {
		v, label := m[1], m[2]
		metric, ok := regimeVarMetric[v]
		if !ok {
			t.Errorf("regime variable %s is not paired with a metric in regimeVarMetric", v)
			continue
		}
		seen[v] = true
		assertToned(t, metric, "regime", label)
	}
	for v := range regimeVarMetric {
		if !seen[v] {
			t.Errorf("regimeVarMetric lists %s but main.go never assigns it", v)
		}
	}

	stanceAssign := regexp.MustCompile(`\b(stance|gcStance|infStance|ggStance)\s*:?=\s*"([a-z_]+)"`)
	for _, m := range stanceAssign.FindAllStringSubmatch(src, -1) {
		assertToned(t, stanceVarMetric[m[1]], "stance", m[2])
	}

	composite := readFile(t, "../../internal/marketcycle/composite.go")
	phases := regexp.MustCompile(`Phase:\s*"([a-z_]+)"`).FindAllStringSubmatch(composite, -1)
	if len(phases) == 0 {
		t.Fatal("found no composite phases in composite.go; the scan pattern is stale")
	}
	emitted := map[string]bool{}
	for _, m := range phases {
		emitted[m[1]] = true
		assertToned(t, "mc_market_cycle", "composite_phase", m[1])
	}
	for phase := range macrotone.CompositePhases() {
		if !emitted[phase] {
			t.Errorf("macrotone tones composite phase %q that composite.go never emits", phase)
		}
	}

	vix := readFile(t, "../../internal/compute/vix.go")
	for _, m := range regexp.MustCompile(`return "([a-z_]+)"`).FindAllStringSubmatch(vix, -1) {
		assertToned(t, "mc_vix_regime", "regime", m[1])
	}

	corr := readFile(t, "../../internal/macrocorr/regime.go")
	for _, m := range regexp.MustCompile(`Regime:\s*"([a-z_]+)"`).FindAllStringSubmatch(corr, -1) {
		assertToned(t, "mc_macro_correlation", "regime", m[1])
	}
}

func TestEveryWrittenSectionMetricIsClassified(t *testing.T) {
	src := readFile(t, "main.go")
	for _, m := range regexp.MustCompile(`upsert\("((?:mp|gc|inf|gg)_\w+)"`).FindAllStringSubmatch(src, -1) {
		if !macrotone.Classified(m[1]) {
			t.Errorf("%s is written but has no tone table (add one, or mark it display-only)", m[1])
		}
	}
}

func assertToned(t *testing.T, metric, key, label string) {
	t.Helper()
	if _, ok := macrotone.For(metric, map[string]any{key: label}); !ok {
		t.Errorf("%s %s %q has no tone", metric, key, label)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
