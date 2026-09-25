package compute

import "testing"

func TestClassifyVIX(t *testing.T) {
	th := VIXThresholds{Fear: 35, Elevated: 20, Complacency: 12}
	cases := map[float64]string{
		40: "extreme_fear", 35.01: "extreme_fear",
		35: "elevated", 20.01: "elevated",
		20: "normal", 14.21: "normal", 12: "normal",
		11.99: "complacency", 9: "complacency",
	}
	for vix, want := range cases {
		if got := ClassifyVIX(vix, th); got != want {
			t.Errorf("ClassifyVIX(%v) = %q, want %q", vix, got, want)
		}
	}
}
