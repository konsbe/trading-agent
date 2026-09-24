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
