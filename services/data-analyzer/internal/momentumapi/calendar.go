package momentumapi

import (
	"fmt"
	"time"
)

// NYSE full-day closures. Hard-coded rather than derived from ingested bars:
// is_stale exists to catch the day the pipeline did not run, and a calendar
// built from the same data cannot see a day that data never arrived for.
//
// Extend this list before calendarLastYear ends — past it, ExpectedSession
// returns an error rather than silently treating every weekday as a session,
// and a test fails six months ahead as a reminder.
var nyseHolidays = map[string]string{
	"2024-01-01": "New Year's Day",
	"2024-01-15": "Martin Luther King Jr. Day",
	"2024-02-19": "Washington's Birthday",
	"2024-03-29": "Good Friday",
	"2024-05-27": "Memorial Day",
	"2024-06-19": "Juneteenth",
	"2024-07-04": "Independence Day",
	"2024-09-02": "Labor Day",
	"2024-11-28": "Thanksgiving Day",
	"2024-12-25": "Christmas Day",

	"2025-01-01": "New Year's Day",
	"2025-01-09": "National Day of Mourning (President Carter)",
	"2025-01-20": "Martin Luther King Jr. Day",
	"2025-02-17": "Washington's Birthday",
	"2025-04-18": "Good Friday",
	"2025-05-26": "Memorial Day",
	"2025-06-19": "Juneteenth",
	"2025-07-04": "Independence Day",
	"2025-09-01": "Labor Day",
	"2025-11-27": "Thanksgiving Day",
	"2025-12-25": "Christmas Day",

	"2026-01-01": "New Year's Day",
	"2026-01-19": "Martin Luther King Jr. Day",
	"2026-02-16": "Washington's Birthday",
	"2026-04-03": "Good Friday",
	"2026-05-25": "Memorial Day",
	"2026-06-19": "Juneteenth",
	"2026-07-03": "Independence Day (observed)",
	"2026-09-07": "Labor Day",
	"2026-11-26": "Thanksgiving Day",
	"2026-12-25": "Christmas Day",

	"2027-01-01": "New Year's Day",
	"2027-01-18": "Martin Luther King Jr. Day",
	"2027-02-15": "Washington's Birthday",
	"2027-03-26": "Good Friday",
	"2027-05-31": "Memorial Day",
	"2027-06-18": "Juneteenth (observed)",
	"2027-07-05": "Independence Day (observed)",
	"2027-09-06": "Labor Day",
	"2027-11-25": "Thanksgiving Day",
	"2027-12-24": "Christmas Day (observed)",
}

const (
	calendarFirstYear = 2024
	calendarLastYear  = 2027

	// A daily session is regular-hours only; the scan reads its closing bar.
	sessionCloseHour = 16
)

var newYork = mustLoadLocation("America/New_York")

func mustLoadLocation(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic(fmt.Sprintf("load %s: %v", name, err))
	}
	return loc
}

// ErrCalendarNotCovered means the date is outside the holiday list above.
type ErrCalendarNotCovered struct{ Date string }

func (e ErrCalendarNotCovered) Error() string {
	return fmt.Sprintf("NYSE holiday calendar does not cover %s (covers %d-%d); extend nyseHolidays",
		e.Date, calendarFirstYear, calendarLastYear)
}

// civilDate is a calendar date with no time or zone, compared as YYYY-MM-DD.
func civilDate(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// IsTradingDay reports whether the NYSE held a regular session on the given
// civil date (its year, month and day; the zone is ignored).
func IsTradingDay(d time.Time) (bool, error) {
	if d.Year() < calendarFirstYear || d.Year() > calendarLastYear {
		return false, ErrCalendarNotCovered{Date: d.Format(time.DateOnly)}
	}
	if wd := d.Weekday(); wd == time.Saturday || wd == time.Sunday {
		return false, nil
	}
	_, holiday := nyseHolidays[d.Format(time.DateOnly)]
	return !holiday, nil
}

// ExpectedSession is the most recent session whose scan should exist by now: a
// trading day whose close (16:00 New York) plus readyAfter has passed. The
// grace period covers ingestion and the scanner's own run, so the payload does
// not read as stale every evening before the daily pass has had time to finish.
// The result is a civil date in UTC, matching how momentum_features.ts stores
// the trading day.
func ExpectedSession(now time.Time, readyAfter time.Duration) (time.Time, error) {
	local := now.In(newYork)
	day := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, newYork)
	for i := 0; i < 15; i++ {
		ok, err := IsTradingDay(day)
		if err != nil {
			return time.Time{}, err
		}
		ready := time.Date(day.Year(), day.Month(), day.Day(), sessionCloseHour, 0, 0, 0, newYork).Add(readyAfter)
		if ok && !now.Before(ready) {
			return civilDate(day), nil
		}
		day = day.AddDate(0, 0, -1)
	}
	return time.Time{}, fmt.Errorf("no NYSE session found in the 15 days before %s", now.Format(time.RFC3339))
}

// IsStale reports whether scanDate is older than the session that should have
// been scanned by now.
func IsStale(scanDate, now time.Time, readyAfter time.Duration) (bool, error) {
	expected, err := ExpectedSession(now, readyAfter)
	if err != nil {
		return false, err
	}
	return civilDate(scanDate.UTC()).Before(expected), nil
}
