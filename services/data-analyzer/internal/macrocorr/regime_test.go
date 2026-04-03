package macrocorr

import "testing"

func TestBuild_RecessionPipeline(t *testing.T) {
	r := Build(Inputs{
		YieldCurve:   "inverted",
		CreditRegime: "crisis",
		GDPRegime:    "recession",
		GCStance:     "contraction",
	})
	if r.Regime != "recession_pipeline" {
		t.Fatalf("regime = %q", r.Regime)
	}
}

func TestBuild_Stagflation(t *testing.T) {
	r := Build(Inputs{
		InfStance: "hot",
		GCStance:  "slowdown",
	})
	if r.Regime != "stagflation_risk" {
		t.Fatalf("regime = %q", r.Regime)
	}
}

func TestBuild_Goldilocks(t *testing.T) {
	r := Build(Inputs{
		InfStance:    "moderate",
		MPStance:     "neutral",
		GCStance:     "expansion",
		CreditRegime: "benign",
	})
	if r.Regime != "goldilocks_light" {
		t.Fatalf("regime = %q", r.Regime)
	}
}
