package macrotone

import "testing"

func TestForStances(t *testing.T) {
	cases := []struct {
		metric, stance string
		want           Tone
	}{
		{"mp_stance", "accommodative", Constructive},
		{"mp_stance", "restrictive", Stressed},
		{"gc_stance", "slowdown", Neutral},
		{"inf_stance", "moderate", Neutral},
		{"inf_stance", "deflationary", Stressed},
		{"inf_stance", "hot", Stressed},
		{"gg_stance", "moderate", Neutral},
		{"gg_stance", "elevated_stress", Stressed},
		{"gc_stance", "insufficient_data", NoData},
	}
	for _, tc := range cases {
		got, ok := For(tc.metric, map[string]any{"stance": tc.stance})
		if !ok || got != tc.want {
			t.Errorf("%s %q = %q, %v; want %q", tc.metric, tc.stance, got, ok, tc.want)
		}
	}
}

func TestSameWordIsTonedPerMetric(t *testing.T) {
	if got, _ := For("gg_china_gdp", map[string]any{"regime": "slowing"}); got != Stressed {
		t.Errorf("gg_china_gdp slowing = %q, want stressed", got)
	}
	if got, _ := For("gc_lei", map[string]any{"regime": "slowing"}); got != Neutral {
		t.Errorf("gc_lei slowing = %q, want neutral", got)
	}
}

func TestSpreadUsesStoredMarginSignal(t *testing.T) {
	got, ok := For("inf_ppi_cpi_spread", map[string]any{"spread_ppt": 3.1, "margin_signal": "margin_pressure"})
	if !ok || got != Stressed {
		t.Errorf("got %q, %v", got, ok)
	}
	if _, ok := For("inf_ppi_cpi_spread", map[string]any{"spread_ppt": 3.1}); ok {
		t.Error("a spread without margin_signal must not be toned from the number")
	}
}

func TestEverySignalHasATier(t *testing.T) {
	for metric := range signals {
		if metric == "mc_macro_correlation" {
			continue
		}
		if _, ok := PlacementFor(metric); !ok {
			t.Errorf("%s has a tone table but no tier", metric)
		}
	}
	for metric := range placements {
		if !Classified(metric) {
			t.Errorf("%s has a tier but is not classified", metric)
		}
	}
}

func TestAnnotate(t *testing.T) {
	p := map[string]any{"regime": "no_data"}
	if !Annotate("gc_pmi", p) || p[Key] != "no_data" || p["tier"] != 1 || p["tier_group"] != "Leading Indicators" {
		t.Errorf("gc_pmi no_data: %v", p)
	}
	st := map[string]any{"stance": "neutral"}
	if Annotate("mp_stance", st); st["tier"] != nil {
		t.Errorf("stances carry no tier: %v", st)
	}
	y := map[string]any{"10y_pct": 4.1}
	if !Annotate("mp_treasury_yields", y) || y[Key] != "display_only" {
		t.Errorf("treasury yields: %v", y)
	}
	if Annotate("gc_pmi", map[string]any{"regime": "brand_new_label"}) {
		t.Error("unknown label must report false")
	}
	if Annotate("gc_new_signal", map[string]any{"regime": "x"}) {
		t.Error("untabled section metric must report false")
	}
	if !Annotate("mc_price_phase:SPY", map[string]any{"phase": "x"}) {
		t.Error("non-section metrics are left alone")
	}
}
