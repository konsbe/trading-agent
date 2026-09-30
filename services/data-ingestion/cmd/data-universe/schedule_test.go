package main

import (
	"testing"
	"time"

	"github.com/konsbe/trading-agent/services/data-ingestion/internal/store"
)

func ny(t *testing.T, s string) time.Time {
	t.Helper()
	ts, err := time.ParseInLocation("2006-01-02 15:04", s, newYork)
	if err != nil {
		t.Fatal(err)
	}
	return ts
}

func day(s string) time.Time {
	d, _ := time.Parse(time.DateOnly, s)
	return d
}

var at1830 = dailyClock{18, 30}

func TestParseDailyClock(t *testing.T) {
	if c, err := parseDailyClock("18:30"); err != nil || c != at1830 {
		t.Fatalf("got %v, %v", c, err)
	}
	for _, bad := range []string{"", "18", "24:00", "18:60", "x:30"} {
		if _, err := parseDailyClock(bad); err == nil {
			t.Errorf("%q: want error", bad)
		}
	}
}

func TestNextDailyRun(t *testing.T) {
	cases := []struct{ now, want string }{
		{"2026-09-24 12:00", "2026-09-24 18:30"}, // Thursday, before the run
		{"2026-09-24 18:30", "2026-09-25 18:30"}, // exactly at it: strictly after
		{"2026-09-25 19:00", "2026-09-28 18:30"}, // Friday evening -> Monday
		{"2026-09-26 10:00", "2026-09-28 18:30"}, // Saturday
	}
	for _, c := range cases {
		if got := nextDailyRun(ny(t, c.now), at1830, newYork); !got.Equal(ny(t, c.want)) {
			t.Errorf("now %s: got %s, want %s", c.now, got.In(newYork), c.want)
		}
	}
}

func TestNextDailyRunFollowsNewYorkAcrossDST(t *testing.T) {
	// DST ends 2026-11-01: the run stays at 18:30 local, so its UTC shifts.
	got := nextDailyRun(ny(t, "2026-10-30 19:00"), at1830, newYork)
	if want := time.Date(2026, 11, 2, 23, 30, 0, 0, time.UTC); !got.Equal(want) {
		t.Fatalf("got %s, want %s", got.UTC(), want)
	}
}

func TestLatestDueSession(t *testing.T) {
	cases := []struct{ now, want string }{
		{"2026-09-24 13:07", "2026-09-23"}, // today's run not yet due
		{"2026-09-24 18:30", "2026-09-24"},
		{"2026-09-27 09:00", "2026-09-25"}, // Sunday -> Friday
		{"2026-09-28 08:00", "2026-09-25"}, // Monday morning -> Friday
	}
	for _, c := range cases {
		if got := latestDueSession(ny(t, c.now), at1830, newYork); !got.Equal(day(c.want)) {
			t.Errorf("now %s: got %s, want %s", c.now, got.Format(time.DateOnly), c.want)
		}
	}
}

func TestBarsCurrentShare(t *testing.T) {
	last := func(s string) *time.Time { d := day(s); return &d }
	bounds := map[string]store.SymbolBarBounds{
		"A": {Count: 300, LastTS: last("2026-09-23")},
		"B": {Count: 300, LastTS: last("2026-09-24")},
		"C": {Count: 300, LastTS: last("2026-09-21")}, // stale
		"D": {Count: 300, LastTS: last("2026-09-21")}, // stale
		"N": {Count: 0},                               // never backfilled: ignored
	}
	if got := barsCurrentShare(bounds, day("2026-09-23")); got != 0.5 {
		t.Fatalf("got %v, want 0.5", got)
	}
	if got := barsCurrentShare(map[string]store.SymbolBarBounds{}, day("2026-09-23")); got != 1 {
		t.Fatalf("empty universe: got %v, want 1 (nothing to catch up)", got)
	}
}

func newRefresh(done time.Time) *dailyRefresh {
	return &dailyRefresh{at: at1830, loc: newYork, done: done, retry: 30 * time.Minute, maxTries: 3}
}

// 2026-09-28: the machine slept from 13:55 to 05:38 New York, across 18:30.
// The check after waking must see the session as due at once.
func TestDailyRefreshDueAfterSleepingThroughTheRunTime(t *testing.T) {
	d := newRefresh(day("2026-09-25"))
	if _, run := d.due(ny(t, "2026-09-28 13:55")); run {
		t.Fatal("due before 18:30")
	}
	session, run := d.due(ny(t, "2026-09-29 05:38"))
	if !run || !session.Equal(day("2026-09-28")) {
		t.Fatalf("after waking: session %s run=%v, want 2026-09-28 true", session.Format(time.DateOnly), run)
	}
	d.finished(session, ny(t, "2026-09-29 06:20"), true)
	if _, run := d.due(ny(t, "2026-09-29 06:21")); run {
		t.Error("ran again for a session already refreshed")
	}
	if s, run := d.due(ny(t, "2026-09-29 18:30")); !run || !s.Equal(day("2026-09-29")) {
		t.Errorf("next session not due at 18:30: %s %v", s.Format(time.DateOnly), run)
	}
}

func TestDailyRefreshWeekendIsNotDue(t *testing.T) {
	d := newRefresh(day("2026-09-25"))
	for _, at := range []string{"2026-09-26 18:31", "2026-09-27 23:00", "2026-09-28 18:29"} {
		if s, run := d.due(ny(t, at)); run {
			t.Errorf("%s: due for %s", at, s.Format(time.DateOnly))
		}
	}
}

func TestDailyRefreshRetriesThenAbandons(t *testing.T) {
	d := newRefresh(day("2026-09-28"))
	now := ny(t, "2026-09-29 18:30")
	for try := 1; try <= 3; try++ {
		session, run := d.due(now)
		if !run {
			t.Fatalf("try %d not due at %s", try, now.Format(time.Kitchen))
		}
		abandoned := d.finished(session, now.Add(5*time.Minute), false)
		if abandoned != (try == 3) {
			t.Fatalf("try %d: abandoned = %v", try, abandoned)
		}
		if try < 3 {
			if _, run := d.due(now.Add(10 * time.Minute)); run {
				t.Fatalf("try %d: retried before the retry delay", try)
			}
			now = now.Add(35 * time.Minute)
		}
	}
	if _, run := d.due(now.Add(time.Hour)); run {
		t.Error("abandoned session retried")
	}
	if s, run := d.due(ny(t, "2026-09-30 18:30")); !run || !s.Equal(day("2026-09-30")) {
		t.Errorf("next session not due after an abandoned one: %s %v", s.Format(time.DateOnly), run)
	}
}

// A new session becoming due resets the failed-pass count of the previous one.
func TestDailyRefreshTriesResetPerSession(t *testing.T) {
	d := newRefresh(day("2026-09-28"))
	s, _ := d.due(ny(t, "2026-09-29 18:30"))
	d.finished(s, ny(t, "2026-09-29 18:40"), false)
	s, run := d.due(ny(t, "2026-09-30 18:30"))
	if !run || d.tries != 0 || !s.Equal(day("2026-09-30")) {
		t.Fatalf("session %s run=%v tries=%d, want 2026-09-30 true 0", s.Format(time.DateOnly), run, d.tries)
	}
}
