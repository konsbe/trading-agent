package store

import (
	"testing"
	"time"
)

func pitDate(y int, m time.Month, day int) time.Time {
	return time.Date(y, m, day, 0, 0, 0, 0, time.UTC)
}

func TestSharesPIT_AsOfByFiledDate(t *testing.T) {
	p := &SharesPIT{}
	p.add(pitDate(2024, 3, 1), 100, false)
	p.add(pitDate(2024, 8, 1), 200, true)

	cases := []struct {
		at     time.Time
		ok     bool
		shares float64
		filed  time.Time
	}{
		{pitDate(2024, 2, 29), false, 0, time.Time{}},         // before the first filing
		{pitDate(2024, 3, 1), true, 100, pitDate(2024, 3, 1)}, // filed that day counts
		{pitDate(2024, 7, 31), true, 100, pitDate(2024, 3, 1)},
		{pitDate(2024, 8, 1), true, 200, pitDate(2024, 8, 1)},
		{pitDate(2026, 9, 25), true, 200, pitDate(2024, 8, 1)}, // no staleness judgement here
		// A bar timestamp late in the day in another zone still reads as its
		// own printed date.
		{time.Date(2024, 7, 31, 23, 0, 0, 0, time.FixedZone("EDT", -4*3600)), true, 100, pitDate(2024, 3, 1)},
	}
	for _, c := range cases {
		sh, filed, ok := p.AsOf(c.at)
		if ok != c.ok || sh != c.shares || !filed.Equal(c.filed) {
			t.Errorf("AsOf(%s) = (%v, %s, %v), want (%v, %s, %v)",
				c.at, sh, filed.Format(time.DateOnly), ok, c.shares, c.filed.Format(time.DateOnly), c.ok)
		}
	}
	if !p.MultiClass {
		t.Error("MultiClass should be set when any filing is multi-class")
	}
}

func TestSharesPIT_NilSeries(t *testing.T) {
	var p *SharesPIT
	if _, _, ok := p.AsOf(pitDate(2026, 1, 1)); ok {
		t.Error("nil series returned a share count")
	}
}
