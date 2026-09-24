package main

import (
	"testing"
	"time"

	"github.com/konsbe/trading-agent/services/data-analyzer/internal/compute"
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

func flatSeries(n int, start time.Time) []compute.Bar {
	out := make([]compute.Bar, n)
	for i := range out {
		out[i] = compute.Bar{TS: start.AddDate(0, 0, i), Open: 10, High: 10.1, Low: 9.9, Close: 10, Volume: 1e6}
	}
	return out
}

func TestStepExitsEvaluatesEverySessionSinceTheFloor(t *testing.T) {
	start := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	series := flatSeries(60, start)
	alertIdx := 57 // two unevaluated sessions follow, like the 2026-09-22/23 catch-up
	row := store.TrackedRow{
		Symbol: "X", Bucket: "market", ReferencePrice: 10,
		AlertedTS: series[alertIdx].TS, HighestCloseSince: 10,
	}
	cfg, ecfg := momentum.DefaultConfig(), momentum.DefaultExitConfig()

	step := stepExits(row, series, cfg, ecfg)
	if step.decision.Exit {
		t.Fatalf("flat series should not exit within 2 sessions: %+v", step.decision)
	}
	if step.sessions != 2 || step.decision.SessionsElapsed != 2 {
		t.Fatalf("evaluated %d sessions, elapsed %d; want 2 and 2 (only the latest bar was evaluated before)",
			step.sessions, step.decision.SessionsElapsed)
	}
	if !step.ts.Equal(series[59].TS) {
		t.Fatalf("advanced to %s, want the latest bar %s", step.ts, series[59].TS)
	}

	evaluated := series[59].TS
	row.LastEvaluatedTS = &evaluated
	if again := stepExits(row, series, cfg, ecfg); again.sessions != 0 {
		t.Fatalf("re-run evaluated %d sessions, want 0", again.sessions)
	}
}

func TestStepExitsStopsAtTheFirstExit(t *testing.T) {
	start := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	series := flatSeries(60, start)
	alertIdx := 40
	row := store.TrackedRow{
		Symbol: "X", Bucket: "market", ReferencePrice: 10,
		AlertedTS: series[alertIdx].TS, HighestCloseSince: 10,
	}
	step := stepExits(row, series, momentum.DefaultConfig(), momentum.DefaultExitConfig())
	if !step.decision.Exit {
		t.Skip("default exit config does not time out a flat series within 19 sessions")
	}
	want := series[alertIdx+step.sessions].TS
	if !step.ts.Equal(want) || step.sessions >= len(series)-1-alertIdx {
		t.Fatalf("exit at %s after %d sessions; want the fold to stop at the first exit (%s)", step.ts, step.sessions, want)
	}
}
