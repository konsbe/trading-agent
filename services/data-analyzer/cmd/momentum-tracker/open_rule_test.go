package main

import (
	"testing"

	"github.com/konsbe/trading-agent/services/data-analyzer/internal/momentum"
)

func TestParseOpenMode(t *testing.T) {
	for in, want := range map[string]openMode{"gates": openOnGates, " GATES ": openOnGates, "score": openOnScore} {
		if got, err := parseOpenMode(in); err != nil || got != want {
			t.Errorf("parseOpenMode(%q) = %q, %v", in, got, err)
		}
	}
	if _, err := parseOpenMode("threshold"); err == nil {
		t.Error("an unknown mode must be rejected, not silently defaulted")
	}
}

func TestOpenRule(t *testing.T) {
	gates := openRule{mode: openOnGates, minMarket: 65, minPenny: 72}
	score := openRule{mode: openOnScore, minMarket: 65, minPenny: 72}
	cases := []struct {
		rule   openRule
		bucket momentum.Bucket
		total  int
		want   bool
	}{
		{gates, momentum.BucketMarket, 10, true},
		{gates, momentum.BucketPenny, 0, true},
		{score, momentum.BucketMarket, 65, true},
		{score, momentum.BucketMarket, 64, false},
		{score, momentum.BucketPenny, 71, false},
		{score, momentum.BucketPenny, 72, true},
	}
	for _, c := range cases {
		if got := c.rule.opens(c.bucket, c.total); got != c.want {
			t.Errorf("%s opens(%s, %d) = %v, want %v", c.rule, c.bucket, c.total, got, c.want)
		}
	}
}
