package main

import (
	"testing"
	"time"
)

func TestDecide(t *testing.T) {
	session := time.Date(2026, 9, 22, 0, 0, 0, 0, time.UTC)
	close := sessionClose(session) // 16:00 New York
	const giveUp = 14 * time.Hour
	cases := []struct {
		name      string
		completed string
		coverage  float64
		attempts  int
		now       time.Time
		want      decision
	}{
		{"already handled", "2026-09-22", 0.99, 0, close.Add(3 * time.Hour), decideDone},
		{"bars landed → run", "2026-09-21", 0.97, 0, close.Add(3 * time.Hour), decideRun},
		{"partial day → wait, never run", "", 0.60, 0, close.Add(3 * time.Hour), decideWait},
		{"exactly the threshold runs", "", 0.95, 0, close.Add(3 * time.Hour), decideRun},
		{"bars never landed → give up", "", 0.60, 0, close.Add(giveUp), decideGiveUp},
		{"repeated failures → give up", "", 0.99, 3, close.Add(3 * time.Hour), decideGiveUp},
		{"a failure below the cap retries", "", 0.99, 2, close.Add(3 * time.Hour), decideRun},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := decide(session, c.completed, c.coverage, 0.95, c.attempts, 3, c.now, giveUp); got != c.want {
				t.Errorf("decide = %v, want %v", got, c.want)
			}
		})
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
