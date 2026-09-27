package fundamental

import "testing"

// The summary averaged all four cluster scores, a cluster that ran no check
// counting as 0. Live rows on 2026-09-27 (score / checks run per cluster:
// earnings quality, valuation, leverage, operational).
func TestEvaluatedMeanLeavesOutUnevaluatedClusters(t *testing.T) {
	for _, tc := range []struct {
		name     string
		clusters []clusterScore
		want     *float64
		n        int
		stored   float64 // what the /4 average stored
	}{
		// IPGP: leverage ran nothing; (−0.5 − 0.667 − 0.5) / 3.
		{"IPGP", []clusterScore{{-0.5, 3}, {-0.5, 2}, {0, 0}, {-2.0 / 3, 3}}, fp(-0.56), 3, -0.42},
		// AAPL: (1 + 0) / 2.
		{"AAPL", []clusterScore{{1, 1}, {0, 1}, {0, 0}, {0, 0}}, fp(0.5), 2, 0.25},
		// WRBY: one cluster, scored 0 over 1 check — 0 either way.
		{"WRBY", []clusterScore{{0, 1}, {0, 0}, {0, 0}, {0, 0}}, fp(0), 1, 0},
		// SPY: nothing evaluated — no summary, not 0.
		{"SPY", []clusterScore{{0, 0}, {0, 0}, {0, 0}, {0, 0}}, nil, 0, 0},
	} {
		got, n := evaluatedMean(tc.clusters)
		if n != tc.n || (got == nil) != (tc.want == nil) || (got != nil && *got != *tc.want) {
			t.Errorf("%s: evaluatedMean = %v over %d, want %v over %d (the /4 average stored %v)", tc.name, deref(got), n, deref(tc.want), tc.n, tc.stored)
		}
	}
}

func fp(f float64) *float64 { return &f }
