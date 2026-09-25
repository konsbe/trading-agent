package main

import (
	"strings"
	"testing"
	"time"

	"github.com/konsbe/trading-agent/services/data-analyzer/internal/marketcycle"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/store"
)

func TestInstrumentList_FixedThenCryptoThenWatchlistWithoutDuplicates(t *testing.T) {
	got := instrumentList([]string{"SPY", "GLD", "USO", "SHEL"}, []string{"BTCUSDT"}, []string{"nvda", "SPY", " SHEL "})
	var parts []string
	for _, in := range got {
		parts = append(parts, in.symbol+"/"+in.kind)
	}
	want := "SPY/equity GLD/equity USO/equity SHEL/equity BTCUSDT/crypto NVDA/equity"
	if strings.Join(parts, " ") != want {
		t.Fatalf("got %v\nwant %s", parts, want)
	}
}

var th = marketcycle.Thresholds{
	PullbackPct: -0.03, CorrectionPct: -0.10, BearPct: -0.20,
	CrashVs10DHighPct: -0.12, CrashVs5BarPct: -0.10, BullExtendedSMAPct: 0.05,
	PeakLookback: 252, SMAPeriod: 200,
}

func bars(n int) []store.EquityOHLCVBar {
	out := make([]store.EquityOHLCVBar, n)
	for i := range out {
		c := 100 + float64(i)*0.1
		out[i] = store.EquityOHLCVBar{TS: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, i), High: c, Low: c, Close: c}
	}
	return out
}

func TestInstrumentPayload_TooFewBarsSaysWhyAndComputesNothing(t *testing.T) {
	p := instrumentPayload(instrument{"NEWCO", "equity"}, "1Day", "session close", bars(40), th, 200)
	if p["price_phase"] != "insufficient_data" || !strings.Contains(p["unavailable_reason"].(string), "need at least 200") {
		t.Fatalf("payload = %v", p)
	}
	for _, k := range []string{"drawdown_pct", "pct_vs_sma200", "sma200", "peak_high"} {
		if _, ok := p[k]; ok {
			t.Errorf("%s present on an insufficient-data instrument; it must not be computed from 40 bars", k)
		}
	}
}

func TestInstrumentPayload_CryptoReportsItsCalendarDayWindows(t *testing.T) {
	crypto := th
	crypto.PeakLookback, crypto.CrashHighWindow, crypto.CrashCloseBars = 365, 14, 7
	p := instrumentPayload(instrument{"BTCUSDT", "crypto"}, "1d", "00:00 UTC daily close (closed candles only)", bars(400), crypto, 200)
	w := p["windows"].(map[string]any)
	if p["type"] != "crypto" || w["peak_lookback"] != 365 || w["crash_high_window"] != 14 || w["crash_close_bars"] != 7 || w["sma_period"] != 200 {
		t.Fatalf("payload = %v", p)
	}
	if p["as_of_basis"] != "00:00 UTC daily close (closed candles only)" || p["price_phase"] == "insufficient_data" {
		t.Fatalf("payload = %v", p)
	}
	eq := instrumentPayload(instrument{"SPY", "equity"}, "1Day", "session close", bars(320), th, 200)
	if ew := eq["windows"].(map[string]any); ew["crash_high_window"] != 10 || ew["crash_close_bars"] != 5 {
		t.Errorf("equity windows = %v, want the original 10 / 5", ew)
	}
}
