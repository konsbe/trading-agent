//go:build integration

package twelvedata

import (
	"context"
	"os"
	"testing"
	"time"
)

// Live checks against the real Twelve Data API. Skipped unless
// TWELVE_DATA_API_KEY is set, so the default suite stays hermetic.
//
//	set -a; . ./.env; set +a
//	go test -tags=integration ./internal/fetch/twelvedata/ -run Live -v
//
// These spend real credits against the 8/minute, 800/day free tier, so they are
// deliberately few and spaced.

func liveClient(t *testing.T) *Client {
	t.Helper()
	tok := os.Getenv("TWELVE_DATA_API_KEY")
	if tok == "" {
		t.Skip("TWELVE_DATA_API_KEY not set")
	}
	return New(tok)
}

// The hermetic split fixture asserts what we believe the API returns. This
// asserts the API actually returns it — the two together are what make the
// volume-adjustment claim evidence rather than a comment.
//
// Ground truth is Tiingo's raw/adjusted pair for NVDA's last pre-split session
// before the 10:1 split of 2024-06-10:
//
//	raw close 1208.88      raw volume 41,238,580
//	adj close  120.5447    adj volume 412,385,800
func TestLive_NVDASplitVolumeIsAdjusted(t *testing.T) {
	c := liveClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	from := time.Date(2024, 6, 3, 0, 0, 0, 0, time.UTC)
	to := time.Date(2024, 6, 14, 0, 0, 0, 0, time.UTC)

	bars, err := c.FetchBarsRange(ctx, "NVDA", "1Day", from, to)
	if err != nil {
		t.Fatalf("live fetch: %v", err)
	}
	if len(bars) < 5 {
		t.Fatalf("got %d bars over the split window, want >=5", len(bars))
	}

	var found bool
	for _, b := range bars {
		if b.TS.Format(time.DateOnly) != "2024-06-07" {
			continue
		}
		found = true

		const rawVol, adjVol = 41238580.0, 412385800.0
		const rawClose, adjClose = 1208.88, 120.5447288816

		if r := b.Volume / rawVol; r < 9.9 || r > 10.1 {
			t.Errorf("live volume %.0f is %.2fx Tiingo's raw %.0f; want ~10x. "+
				"Unadjusted volume here would corrupt RVOL for every symbol that ever split",
				b.Volume, r, rawVol)
		}
		if d := absPct(b.Volume, adjVol); d > 0.1 {
			t.Errorf("live volume %.0f vs Tiingo adjVolume %.0f: %.4f%% apart, want <0.1%%", b.Volume, adjVol, d)
		}
		if d := absPct(b.Close, adjClose); d > 0.1 {
			t.Errorf("live close %.5f vs Tiingo adjClose %.5f: %.4f%% apart, want <0.1%% "+
				"(a match against the RAW close %.2f would mean adjust=all was ignored)",
				b.Close, adjClose, d, rawClose)
		}
		t.Logf("NVDA 2024-06-07 live: close=%.5f volume=%.0f (Tiingo adj: %.5f / %.0f)",
			b.Close, b.Volume, adjClose, adjVol)
	}
	if !found {
		t.Error("2024-06-07 bar absent from the live response")
	}
}

// Confirms the depth the pilot needs: 3 years of daily bars in one request.
// This is also the request shape the backfill issues, so a change in paging
// behaviour shows up here.
func TestLive_ThreeYearWindowReturnsFullHistory(t *testing.T) {
	c := liveClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	to := time.Now().UTC()
	from := to.AddDate(-3, 0, 0)

	bars, err := c.FetchBarsRange(ctx, "AAPL", "1Day", from, to)
	if err != nil {
		t.Fatalf("live fetch: %v", err)
	}
	// ~252 trading days a year; allow slack for holidays and partial edges.
	if len(bars) < 700 || len(bars) > 800 {
		t.Errorf("got %d bars over 3y, want ~750 — outside that range suggests "+
			"paging truncation or an unexpected history limit", len(bars))
	}
	if !bars[0].TS.Before(bars[len(bars)-1].TS) {
		t.Error("bars are not oldest-first; ComputeAt indexes assume chronological order")
	}
	for _, b := range bars {
		if b.Close <= 0 || b.Volume <= 0 {
			t.Fatalf("unusable bar slipped through: %+v", b)
		}
	}
	t.Logf("AAPL 3y live: %d bars, %s..%s", len(bars),
		bars[0].TS.Format(time.DateOnly), bars[len(bars)-1].TS.Format(time.DateOnly))
}
