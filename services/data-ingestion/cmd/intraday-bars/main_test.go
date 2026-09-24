package main

import (
	"reflect"
	"testing"
	"time"

	"github.com/konsbe/trading-agent/services/data-ingestion/internal/store"
)

func TestSelectSymbols(t *testing.T) {
	got, dropped := selectSymbols([]string{"VGZ", "aaa", "GDC"}, []string{"gdc", "ZZZ", " "}, 0)
	want := []string{"AAA", "GDC", "VGZ", "ZZZ"}
	if !reflect.DeepEqual(got, want) || dropped != 0 {
		t.Errorf("selectSymbols = %v (dropped %d), want %v", got, dropped, want)
	}
}

func TestSelectSymbols_CapDropsWatchlistExtrasBeforeCandidates(t *testing.T) {
	got, dropped := selectSymbols([]string{"B", "A"}, []string{"Z", "Y"}, 3)
	if !reflect.DeepEqual(got, []string{"A", "B", "Y"}) || dropped != 1 {
		t.Errorf("selectSymbols = %v (dropped %d)", got, dropped)
	}
}

func TestTrimToLookback(t *testing.T) {
	cutoff := time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC)
	bars := []store.EquityBar{
		{TS: cutoff.Add(-time.Minute)},
		{TS: cutoff},
		{TS: cutoff.Add(5 * time.Minute)},
	}
	got := trimToLookback(bars, cutoff)
	if len(got) != 2 || !got[0].TS.Equal(cutoff) {
		t.Errorf("trimToLookback kept %v", got)
	}
	if len(bars) != 3 {
		t.Error("trimToLookback modified its input")
	}
}
