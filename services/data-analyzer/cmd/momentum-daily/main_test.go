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
		{"bars never landed → deferred to catch-up, not final", runState{}, 0.60, close.Add(giveUp), decideDefer},
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

func TestNeedsCatchUp(t *testing.T) {
	due := time.Date(2026, 9, 29, 0, 0, 0, 0, time.UTC)
	older := time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		name    string
		session time.Time
		rs      runState
		want    bool
	}{
		{"due session in its window belongs to the in-window path", due, runState{}, false},
		{"due session deferred → catch-up", due, runState{gaveUp: true}, true},
		{"older session never attempted (daemon or machine down) → catch-up", older, runState{}, true},
		{"older session deferred (2026-09-28) → catch-up", older, runState{gaveUp: true}, true},
		{"older session that failed part-way → catch-up", older, runState{attempts: 1}, true},
		{"finished sessions are never re-run", older, runState{trackerDone: true, gaveUp: true}, false},
	}
	for _, c := range cases {
		if got := needsCatchUp(c.session, due, c.rs); got != c.want {
			t.Errorf("%s: needsCatchUp = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestDecideCatchUp(t *testing.T) {
	cases := []struct {
		name     string
		rs       runState
		coverage float64
		want     decision
	}{
		{"bars landed after the window → run", runState{gaveUp: true}, 0.99, decideRun},
		{"bars still missing → keep waiting, no time limit", runState{gaveUp: true}, 0.10, decideWait},
		{"a failed catch-up below the cap retries", runState{gaveUp: true, attempts: 2}, 0.99, decideRun},
		{"attempts exhausted → final", runState{gaveUp: true, attempts: 3}, 0.99, decideGiveUp},
		{"done", runState{trackerDone: true}, 0.99, decideDone},
	}
	for _, c := range cases {
		if got := decideCatchUp(c.rs, c.coverage, 0.95, 3); got != c.want {
			t.Errorf("%s: decideCatchUp = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestRecentSessionsOldestFirstSkipsNonSessions(t *testing.T) {
	due := time.Date(2026, 9, 29, 0, 0, 0, 0, time.UTC) // Tuesday
	got, err := recentSessions(due, 5)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"2026-09-23", "2026-09-24", "2026-09-25", "2026-09-28", "2026-09-29"}
	if len(got) != len(want) {
		t.Fatalf("got %d sessions, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].Format(time.DateOnly) != want[i] {
			t.Errorf("session %d = %s, want %s", i, got[i].Format(time.DateOnly), want[i])
		}
	}
}

// A one-minute check must not log "waiting for bars" every minute.
func TestLogWaitThrottles(t *testing.T) {
	st := &logState{waits: map[string]waitLogged{}}
	t0 := time.Date(2026, 9, 29, 22, 0, 0, 0, time.UTC)
	steps := []struct {
		at     time.Duration
		landed int
		want   bool
	}{
		{0, 0, true},
		{time.Minute, 0, false},
		{2 * time.Minute, 120, true}, // coverage moved
		{3 * time.Minute, 120, false},
		{17 * time.Minute, 120, true}, // 15 minutes since the last line
	}
	for _, s := range steps {
		if got := st.logWait("2026-09-29", s.landed, t0.Add(s.at), 15*time.Minute); got != s.want {
			t.Errorf("at +%v landed=%d: logWait = %v, want %v", s.at, s.landed, got, s.want)
		}
	}
}
