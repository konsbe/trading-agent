package main

import (
	"testing"
	"time"

	"github.com/konsbe/trading-agent/services/data-analyzer/internal/momentum"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/store"
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

func TestAlreadyEvaluated(t *testing.T) {
	alert := time.Date(2026, 9, 21, 0, 0, 0, 0, time.UTC)
	next := alert.AddDate(0, 0, 1)
	later := alert.AddDate(0, 0, 2)
	fresh := store.TrackedRow{AlertedTS: alert}
	if !alreadyEvaluated(fresh, alert) {
		t.Error("a row opened on bar T must not be evaluated against bar T (spurious session 1 on re-run)")
	}
	if alreadyEvaluated(fresh, next) {
		t.Error("the first session after the alert must be evaluated")
	}
	advanced := store.TrackedRow{AlertedTS: alert, LastEvaluatedTS: &next}
	if !alreadyEvaluated(advanced, next) {
		t.Error("re-running on an already evaluated bar must be a no-op")
	}
	if alreadyEvaluated(advanced, later) {
		t.Error("a newer bar must be evaluated")
	}
}
