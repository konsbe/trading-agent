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

// dailyBarsSchedule returns when the next refresh should fire. On the first
// call (startup) it fires immediately if stored bars are behind the latest due
// session; after that it follows the schedule.
func (w *worker) dailyBarsSchedule(ctx context.Context) (func(now time.Time, startup bool) time.Time, error) {
	if strings.TrimSpace(w.cfg.DailyBarsAt) == "interval" {
		w.log.Info("daily bars: legacy interval schedule", "every", w.cfg.DailyBarsInterval.String())
		return func(now time.Time, _ bool) time.Time { return now.Add(w.cfg.DailyBarsInterval) }, nil
	}
	at, err := parseDailyClock(w.cfg.DailyBarsAt)
	if err != nil {
		return nil, fmt.Errorf("UNIVERSE_DAILY_BARS_AT: %w", err)
	}
	return func(now time.Time, startup bool) time.Time {
		next := nextDailyRun(now, at, newYork)
		if !startup || !w.cfg.EnableDailyBars {
			w.log.Info("daily bars: next refresh", "at", next.Format(time.RFC3339))
			return next
		}
		session := latestDueSession(now, at, newYork)
		bounds, err := store.LoadBarBounds(ctx, w.pool, w.cfg.BarInterval, w.cfg.BarSource)
		if err != nil {
			w.log.Warn("daily bars: could not check coverage; catching up to be safe", "err", err)
			return now
		}
		share := barsCurrentShare(bounds, session)
		if share < w.cfg.DailyBarsCatchUpShare {
			w.log.Info("daily bars: behind at startup — refreshing now",
				"session", session.Format(time.DateOnly), "current_share", fmt.Sprintf("%.3f", share),
				"then_at", next.Format(time.RFC3339))
			return now
		}
		w.log.Info("daily bars: current at startup",
			"session", session.Format(time.DateOnly), "current_share", fmt.Sprintf("%.3f", share),
			"next_refresh", next.Format(time.RFC3339))
		return next
	}, nil
}
