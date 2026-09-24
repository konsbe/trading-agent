package momentumapi

import (
	"errors"
	"testing"
	"time"
)

func date(s string) time.Time {
	d, err := time.Parse(time.DateOnly, s)
	if err != nil {
		panic(err)
	}
	return d
}

// ny builds an instant in New York local time.
func ny(s string) time.Time {
	t, err := time.ParseInLocation("2006-01-02 15:04", s, newYork)
	if err != nil {
		panic(err)
	}
	return t
}

// easter returns Easter Sunday (Gregorian, anonymous computus).
func easter(year int) time.Time {
	a := year % 19
	b, c := year/100, year%100
	d, e := b/4, b%4
	f := (b + 8) / 25
	g := (b - f + 1) / 3
	h := (19*a + b - d - g + 15) % 30
	i, k := c/4, c%4
	l := (32 + 2*e + 2*i - h - k) % 7
	m := (a + 11*h + 22*l) / 451
	month := (h + l - 7*m + 114) / 31
	day := (h+l-7*m+114)%31 + 1
	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
}

func nthWeekday(year int, month time.Month, wd time.Weekday, n int) time.Time {
	d := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
	for d.Weekday() != wd {
		d = d.AddDate(0, 0, 1)
	}
	return d.AddDate(0, 0, 7*(n-1))
}

func lastWeekday(year int, month time.Month, wd time.Weekday) time.Time {
	d := time.Date(year, month+1, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, -1)
	for d.Weekday() != wd {
		d = d.AddDate(0, 0, -1)
	}
	return d
}

// observed applies NYSE's weekend rule: Saturday moves to Friday, Sunday to
// Monday — except New Year's Day on a Saturday, which is not observed.
func observed(d time.Time, isNewYear bool) (time.Time, bool) {
	switch d.Weekday() {
	case time.Saturday:
		if isNewYear {
			return time.Time{}, false
		}
		return d.AddDate(0, 0, -1), true
	case time.Sunday:
		return d.AddDate(0, 0, 1), true
	}
	return d, true
}

// ruleHolidays re-derives the regular NYSE holidays from their rules, as an
// independent check on the hard-coded list.
func ruleHolidays(year int) map[string]bool {
	out := map[string]bool{}
	add := func(d time.Time, ok bool) {
		if ok {
			out[d.Format(time.DateOnly)] = true
		}
	}
	add(observed(time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC), true))
	add(nthWeekday(year, time.January, time.Monday, 3), true)
	add(nthWeekday(year, time.February, time.Monday, 3), true)
	add(easter(year).AddDate(0, 0, -2), true)
	add(lastWeekday(year, time.May, time.Monday), true)
	add(observed(time.Date(year, 6, 19, 0, 0, 0, 0, time.UTC), false))
	add(observed(time.Date(year, 7, 4, 0, 0, 0, 0, time.UTC), false))
	add(nthWeekday(year, time.September, time.Monday, 1), true)
	add(nthWeekday(year, time.November, time.Thursday, 4), true)
	add(observed(time.Date(year, 12, 25, 0, 0, 0, 0, time.UTC), false))
	return out
}

// One-off closures that no rule produces.
var adHocClosures = map[string]bool{"2025-01-09": true}

func TestHolidayListMatchesTheObservanceRules(t *testing.T) {
	for year := calendarFirstYear; year <= calendarLastYear; year++ {
		want := ruleHolidays(year)
		for d := range want {
			if _, ok := nyseHolidays[d]; !ok {
				t.Errorf("rule-derived holiday %s missing from nyseHolidays", d)
			}
		}
		for d := range nyseHolidays {
			if date(d).Year() != year || adHocClosures[d] {
				continue
			}
			if !want[d] {
				t.Errorf("nyseHolidays has %s (%s), which no observance rule produces", d, nyseHolidays[d])
			}
		}
	}
}

func TestHolidaysAreWeekdays(t *testing.T) {
	for d, name := range nyseHolidays {
		if wd := date(d).Weekday(); wd == time.Saturday || wd == time.Sunday {
			t.Errorf("%s (%s) falls on %s; list the observed weekday instead", d, name, wd)
		}
	}
}

func TestIsTradingDay(t *testing.T) {
	cases := map[string]bool{
		"2026-09-17": true,  // Thursday
		"2026-09-19": false, // Saturday
		"2026-09-20": false, // Sunday
		"2026-09-07": false, // Labor Day
		"2026-04-03": false, // Good Friday
		"2025-01-09": false, // Carter day of mourning
		"2027-12-24": false, // Christmas observed
	}
	for d, want := range cases {
		got, err := IsTradingDay(date(d))
		if err != nil {
			t.Fatalf("%s: %v", d, err)
		}
		if got != want {
			t.Errorf("IsTradingDay(%s) = %v, want %v", d, got, want)
		}
	}
}

func TestExpectedSession(t *testing.T) {
	const grace = 6 * time.Hour
	cases := []struct {
		name string
		now  time.Time
		want string
	}{
		{"before today's close uses the previous session", ny("2026-09-23 10:00"), "2026-09-22"},
		{"after close but inside the grace window", ny("2026-09-23 18:00"), "2026-09-22"},
		{"after close plus grace uses today", ny("2026-09-23 22:30"), "2026-09-23"},
		{"monday morning falls back to friday", ny("2026-09-21 09:00"), "2026-09-18"},
		{"weekend uses friday", ny("2026-09-20 12:00"), "2026-09-18"},
		{"day after a holiday skips it", ny("2026-09-08 09:00"), "2026-09-04"},
		{"holiday itself uses the prior session", ny("2026-09-07 23:59"), "2026-09-04"},
		{"good friday weekend uses thursday", ny("2026-04-05 12:00"), "2026-04-02"},
		{"utc midnight is still the prior new york evening", time.Date(2026, 9, 24, 3, 0, 0, 0, time.UTC), "2026-09-23"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := ExpectedSession(c.now, grace)
			if err != nil {
				t.Fatal(err)
			}
			if g := got.Format(time.DateOnly); g != c.want {
				t.Errorf("ExpectedSession(%s) = %s, want %s", c.now.Format(time.RFC3339), g, c.want)
			}
		})
	}
}

func TestIsStale(t *testing.T) {
	const grace = 6 * time.Hour
	cases := []struct {
		name     string
		scanDate string
		now      time.Time
		want     bool
	}{
		{"scan of the expected session is fresh", "2026-09-17", ny("2026-09-18 09:00"), false},
		{"one session behind is stale", "2026-09-17", ny("2026-09-19 12:00"), true},
		{"friday scan is fresh all weekend and monday morning", "2026-09-18", ny("2026-09-21 09:00"), false},
		{"friday scan is fresh across a monday holiday", "2026-09-04", ny("2026-09-08 09:00"), false},
		{"thursday scan is fresh across good friday", "2026-04-02", ny("2026-04-06 09:00"), false},
		{"a scan newer than expected is not stale", "2026-09-23", ny("2026-09-23 17:00"), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := IsStale(date(c.scanDate), c.now, grace)
			if err != nil {
				t.Fatal(err)
			}
			if got != c.want {
				t.Errorf("IsStale(%s, %s) = %v, want %v", c.scanDate, c.now.Format(time.RFC3339), got, c.want)
			}
		})
	}
}

func TestExpectedSessionOutsideCoverageIsAnError(t *testing.T) {
	_, err := ExpectedSession(ny("2028-01-05 12:00"), 0)
	var notCovered ErrCalendarNotCovered
	if !errors.As(err, &notCovered) {
		t.Fatalf("err = %v, want ErrCalendarNotCovered", err)
	}
}

// Tripwire: fails six months before the holiday list runs out, so extending it
// happens on a schedule rather than during an outage.
func TestHolidayCalendarHasSixMonthsOfRunway(t *testing.T) {
	end := time.Date(calendarLastYear, 12, 31, 0, 0, 0, 0, time.UTC)
	if time.Now().AddDate(0, 6, 0).After(end) {
		t.Errorf("nyseHolidays ends %s; add the next year's NYSE holidays", end.Format(time.DateOnly))
	}
}
