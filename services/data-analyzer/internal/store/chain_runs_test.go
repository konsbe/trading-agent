package store

import (
	"testing"
	"time"
)

func TestScanSessionIsTheModalDateNotTheMax(t *testing.T) {
	d := func(day int) time.Time { return time.Date(2026, 9, day, 0, 0, 0, 0, time.UTC) }
	// 2026-09-24 morning: 19 backfilled symbols had 09-23 while the universe stopped at 09-21.
	latest := []time.Time{}
	for i := 0; i < 19; i++ {
		latest = append(latest, d(23))
	}
	for i := 0; i < 4950; i++ {
		latest = append(latest, d(21))
	}
	if got, ok := ScanSession(latest); !ok || !got.Equal(d(21)) {
		t.Fatalf("got %s, want 2026-09-21", got.Format(time.DateOnly))
	}
	if got, _ := ScanSession([]time.Time{d(22), d(23)}); !got.Equal(d(23)) {
		t.Fatalf("tie: got %s, want the later date", got.Format(time.DateOnly))
	}
	if _, ok := ScanSession(nil); ok {
		t.Fatal("empty input must report ok=false")
	}
}
