//go:build integration

package tiingo

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/konsbe/trading-agent/services/data-ingestion/internal/fetch/barsource"
)

// Live check against the real Tiingo API. Skipped unless TIINGO_API_KEY is set,
// so the default suite stays hermetic.
//
//	set -a; . ./.env; set +a
//	go test -tags=integration ./internal/fetch/tiingo/ -run Live -v
//
// The stub tests prove the adapter parses and retries correctly; this proves the
// live API still returns what the scanner needs. Costs 2 unique symbols against
// Tiingo's monthly allowance, so it is deliberately small.
func TestLive_ThreeYearWindowMeetsScannerRequirements(t *testing.T) {
	key := os.Getenv("TIINGO_API_KEY")
	if key == "" {
		t.Skip("TIINGO_API_KEY not set")
	}
	c := New(key)
	to := time.Now().UTC()
	from := to.AddDate(-3, 0, 0)

	bars, err := c.FetchBarsRange(context.Background(), "AAPL", "1Day", from, to)
	if err != nil {
		t.Fatalf("live fetch: %v", err)
	}
	last := bars[len(bars)-1]
	t.Logf("bars=%d  source=%q  first=%s  last=%s  lastClose=%.2f  lastVol=%.0f",
		len(bars), last.Source, bars[0].TS.Format(time.DateOnly),
		last.TS.Format(time.DateOnly), last.Close, last.Volume)

	// §3.7's 52-week window reads high[t-251], so a complete window needs 252
	// bars — which is why §3.1's eligibility minimum is 252.
	if len(bars) < 252 {
		t.Errorf("bars = %d, want >= 252 so the 52-week window can be computed", len(bars))
	}
	// Oldest-first is assumed by every compute function.
	for i := 1; i < len(bars); i++ {
		if !bars[i-1].TS.Before(bars[i].TS) {
			t.Fatalf("bars not strictly oldest-first at %d: %v then %v", i, bars[i-1].TS, bars[i].TS)
		}
	}
	// §2.2's actual requirement. IEX-only volume for AAPL would be low
	// single-digit millions; consolidated is tens of millions.
	if last.Volume < 10_000_000 {
		t.Errorf("last volume = %.0f; below consolidated scale — every §3 volume feature would be wrong", last.Volume)
	}
	if last.Source != SourceName {
		t.Errorf("source = %q, want %q", last.Source, SourceName)
	}
	for _, b := range bars {
		if b.High < b.Low || b.Close <= 0 || b.Volume <= 0 {
			t.Fatalf("incoherent bar at %s: %+v", b.TS.Format(time.DateOnly), b)
		}
	}
}

// A delisted or misspelled ticker must be a clean permanent error, since the
// backfill spends quota on every retry.
func TestLive_UnknownTickerIsPermanentNotRetried(t *testing.T) {
	key := os.Getenv("TIINGO_API_KEY")
	if key == "" {
		t.Skip("TIINGO_API_KEY not set")
	}
	c := New(key)
	_, err := c.FetchBarsRange(context.Background(), "ZZZZNOTREAL", "1Day",
		time.Now().AddDate(-1, 0, 0), time.Now())
	if err == nil {
		t.Fatal("expected an error for an unknown ticker")
	}
	if errors.Is(err, barsource.ErrNoBars) {
		t.Errorf("unknown ticker reported as ErrNoBars; it is a 404, which must be a permanent failure: %v", err)
	}
	t.Logf("unknown ticker -> %v", err)
}
