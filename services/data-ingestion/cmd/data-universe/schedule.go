package main

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/konsbe/trading-agent/services/data-ingestion/internal/store"
)

// The daily refresh used to be a plain 24h ticker started when the process
// came up. That had two failure modes, both seen live: the first pass only
// happened 24h after startup, so a worker restarted more often than daily never
// refreshed at all (2026-09-22 and -23 were skipped that way), and the pass
// time drifted to whenever the process was started rather than following the
// close, which could land a session's bars too late for momentum-daily.
//
// The next fix replaced "a timer until the next 18:30" with a wall-clock check
// every minute. A Go timer counts on the monotonic clock, which stops while
// the host sleeps: on 2026-09-28 the machine slept across 18:30 and the timer
// fired sixteen hours of wall time late, after momentum-daily had given up on
// the session. The check instead asks "has the latest 18:30 passed, and has
// its session been refreshed?", so a woken machine refreshes within a minute.

// dailyClock is a wall-clock time of day in the schedule's location.
type dailyClock struct{ hour, minute int }

func parseDailyClock(s string) (dailyClock, error) {
	h, m, ok := strings.Cut(strings.TrimSpace(s), ":")
	if !ok {
		return dailyClock{}, fmt.Errorf("want HH:MM, got %q", s)
	}
	hour, err1 := strconv.Atoi(h)
	minute, err2 := strconv.Atoi(m)
	if err1 != nil || err2 != nil || hour < 0 || hour > 23 || minute < 0 || minute > 59 {
		return dailyClock{}, fmt.Errorf("want HH:MM, got %q", s)
	}
	return dailyClock{hour, minute}, nil
}

func isWeekday(t time.Time) bool {
	return t.Weekday() != time.Saturday && t.Weekday() != time.Sunday
}

func (c dailyClock) on(day time.Time, loc *time.Location) time.Time {
	y, mo, d := day.In(loc).Date()
	return time.Date(y, mo, d, c.hour, c.minute, 0, 0, loc)
}

// nextDailyRun is the first weekday run time strictly after now. Weekends are
// skipped: there is no session to fetch, and a pass costs one request per
// symbol. Holidays are not modelled; a holiday pass just finds no new bar.
func nextDailyRun(now time.Time, at dailyClock, loc *time.Location) time.Time {
	for day := now.In(loc); ; day = day.AddDate(0, 0, 1) {
		if run := at.on(day, loc); isWeekday(run) && run.After(now) {
			return run
		}
	}
}

// latestDueSession is the trading date of the most recent weekday run time at
// or before now: the session whose bars should already be stored.
func latestDueSession(now time.Time, at dailyClock, loc *time.Location) time.Time {
	for day := now.In(loc); ; day = day.AddDate(0, 0, -1) {
		if run := at.on(day, loc); isWeekday(run) && !run.After(now) {
			y, mo, d := run.Date()
			return time.Date(y, mo, d, 0, 0, 0, 0, time.UTC)
		}
	}
}

// barsCurrentShare is the share of backfilled symbols whose newest bar is on or
// after session. Never-backfilled symbols are excluded: the daily refresh skips
// them too, so they cannot make it look behind.
func barsCurrentShare(bounds map[string]store.SymbolBarBounds, session time.Time) float64 {
	var backfilled, current int
	for _, b := range bounds {
		if b.Count == 0 || b.LastTS == nil {
			continue
		}
		backfilled++
		y, mo, d := b.LastTS.UTC().Date()
		if !time.Date(y, mo, d, 0, 0, 0, 0, time.UTC).Before(session) {
			current++
		}
	}
	if backfilled == 0 {
		return 1
	}
	return float64(current) / float64(backfilled)
}

var newYork = mustLoadLocation("America/New_York")

func mustLoadLocation(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}
	return loc
}

// dailyRefresh decides, on each wall-clock check, whether the refresh for the
// latest due session still has to run. It holds only which session was last
// refreshed; the startup check re-derives that from the stored bars.
type dailyRefresh struct {
	at       dailyClock
	loc      *time.Location
	done     time.Time // latest session refreshed, or abandoned after maxTries
	tries    int       // failed passes for triesFor
	triesFor time.Time
	retryAt  time.Time
	retry    time.Duration
	maxTries int
}

// due returns the latest due session and whether a pass should run now.
func (d *dailyRefresh) due(now time.Time) (time.Time, bool) {
	session := latestDueSession(now, d.at, d.loc)
	if !session.After(d.done) {
		return session, false
	}
	if !session.Equal(d.triesFor) {
		d.triesFor, d.tries, d.retryAt = session, 0, time.Time{}
	}
	return session, !now.Before(d.retryAt)
}

// finished records a pass for session. A failed pass is retried after retry;
// the maxTries-th failure abandons the session until the next one is due, and
// finished reports that.
func (d *dailyRefresh) finished(session, now time.Time, ok bool) (abandoned bool) {
	if ok {
		d.done = session
		return false
	}
	d.tries++
	if d.tries >= d.maxTries {
		d.done = session
		return true
	}
	d.retryAt = now.Add(d.retry)
	return false
}

// dailyBarsSchedule builds the wall-clock schedule, nil for the legacy
// "interval" setting. At startup the latest due session counts as refreshed
// only if the stored bars already cover it; otherwise the first check runs.
func (w *worker) dailyBarsSchedule(ctx context.Context, now time.Time) (*dailyRefresh, error) {
	if strings.TrimSpace(w.cfg.DailyBarsAt) == "interval" {
		w.log.Info("daily bars: legacy interval schedule", "every", w.cfg.DailyBarsInterval.String())
		return nil, nil
	}
	at, err := parseDailyClock(w.cfg.DailyBarsAt)
	if err != nil {
		return nil, fmt.Errorf("UNIVERSE_DAILY_BARS_AT: %w", err)
	}
	d := &dailyRefresh{at: at, loc: newYork, retry: w.cfg.DailyBarsRetryAfter, maxTries: max(1, w.cfg.DailyBarsMaxTries)}
	if !w.cfg.EnableDailyBars {
		return d, nil
	}
	session := latestDueSession(now, at, newYork)
	next := nextDailyRun(now, at, newYork)
	bounds, err := store.LoadBarBounds(ctx, w.pool, w.cfg.BarInterval, w.cfg.BarSource)
	if err != nil {
		w.log.Warn("daily bars: could not check coverage; catching up to be safe", "err", err)
		return d, nil
	}
	share := barsCurrentShare(bounds, session)
	if share < w.cfg.DailyBarsCatchUpShare {
		w.log.Info("daily bars: behind at startup — refreshing now",
			"session", session.Format(time.DateOnly), "current_share", fmt.Sprintf("%.3f", share),
			"then_at", next.Format(time.RFC3339))
		return d, nil
	}
	d.done = session
	w.log.Info("daily bars: current at startup",
		"session", session.Format(time.DateOnly), "current_share", fmt.Sprintf("%.3f", share),
		"next_refresh", next.Format(time.RFC3339))
	return d, nil
}

// checkDailyBars runs the refresh when the schedule says it is due.
func (w *worker) checkDailyBars(ctx context.Context, d *dailyRefresh, now time.Time) {
	if !w.cfg.EnableDailyBars {
		return
	}
	session, run := d.due(now)
	if !run {
		return
	}
	scheduled := d.at.on(session, d.loc)
	w.log.Info("daily bars: refresh due", "session", session.Format(time.DateOnly),
		"scheduled", scheduled.Format(time.RFC3339), "late_by", now.Sub(scheduled).Round(time.Minute).String())
	ok := w.runDailyBars(ctx)
	after := time.Now()
	switch {
	case d.finished(session, after, ok):
		w.log.Error("daily bars: refresh failed on every try; not retrying until the next session is due",
			"session", session.Format(time.DateOnly), "tries", d.maxTries)
	case !ok:
		w.log.Warn("daily bars: refresh incomplete; retrying", "session", session.Format(time.DateOnly),
			"retry_at", d.retryAt.Format(time.RFC3339))
	default:
		w.log.Info("daily bars: next refresh", "at", nextDailyRun(after, d.at, d.loc).Format(time.RFC3339))
	}
}
