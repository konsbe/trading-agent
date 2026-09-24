package main

import (
	"testing"
	"time"

	"github.com/konsbe/trading-agent/services/data-analyzer/internal/store"
)

func TestDecide(t *testing.T) {
	session := time.Date(2026, 9, 22, 0, 0, 0, 0, time.UTC)
	close := sessionClose(session) // 16:00 New York
	const giveUp = 14 * time.Hour
	cases := []struct {
		name     string
		rs       runState
		coverage float64
		now      time.Time
		want     decision
	}{
		{"chain complete", runState{trackerDone: true}, 0.99, close.Add(3 * time.Hour), decideDone},
		{"gave up earlier (persisted) stays given up", runState{gaveUp: true}, 0.99, close.Add(3 * time.Hour), decideDone},
		{"bars landed → run", runState{}, 0.97, close.Add(3 * time.Hour), decideRun},
		{"scanned but tracker not done → run again", runState{attempts: 1}, 0.97, close.Add(3 * time.Hour), decideRun},
		{"partial day → wait, never run", runState{}, 0.60, close.Add(3 * time.Hour), decideWait},
		{"exactly the threshold runs", runState{}, 0.95, close.Add(3 * time.Hour), decideRun},
		{"bars never landed → give up", runState{}, 0.60, close.Add(giveUp), decideGiveUp},
		{"attempts exhausted (survives restarts) → give up", runState{attempts: 3}, 0.99, close.Add(3 * time.Hour), decideGiveUp},
		{"a failure below the cap retries", runState{attempts: 2}, 0.99, close.Add(3 * time.Hour), decideRun},
		{"bars landed late, before the cap, still run", runState{}, 0.99, close.Add(20 * time.Hour), decideRun},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := decide(session, c.rs, c.coverage, 0.95, 3, c.now, giveUp); got != c.want {
				t.Errorf("decide = %v, want %v", got, c.want)
			}
		})
	}
}

// A killed run leaves either no row or a row without tracker_completed_at —
// both must read as "not done", never as finished.
func TestRunStateOfKilledRunIsNotDone(t *testing.T) {
	scanned := time.Now()
	for _, c := range []struct {
		name  string
		row   store.ChainRun
		found bool
	}{
		{"no row: killed before the scan committed", store.ChainRun{}, false},
		{"attempt counted, nothing committed", store.ChainRun{Attempts: 1}, true},
		{"scan committed, tracker killed", store.ChainRun{Attempts: 1, ScannerCompletedAt: &scanned}, true},
	} {
		if rs := runStateOf(c.row, c.found); rs.trackerDone || rs.gaveUp {
			t.Errorf("%s: read as handled (%+v)", c.name, rs)
		}
	}
}

func TestSessionCloseIsNewYorkFourPM(t *testing.T) {
	got := sessionClose(time.Date(2026, 9, 22, 0, 0, 0, 0, time.UTC)).UTC()
	if want := time.Date(2026, 9, 22, 20, 0, 0, 0, time.UTC); !got.Equal(want) { // EDT = UTC-4
		t.Errorf("close = %v, want %v", got, want)
	}
	winter := sessionClose(time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)).UTC()
	if want := time.Date(2026, 12, 1, 21, 0, 0, 0, time.UTC); !winter.Equal(want) { // EST = UTC-5
		t.Errorf("winter close = %v, want %v", winter, want)
	}
}
