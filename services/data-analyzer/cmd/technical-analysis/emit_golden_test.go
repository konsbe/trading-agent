package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/konsbe/trading-agent/services/data-analyzer/internal/compute"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/config"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/technical"
)

var updateGolden = flag.Bool("update", false, "rewrite testdata/indicators_golden.json from the current emitter")

// liveTechnicalConfig is the worker's config under the repo .env values that
// differ from the code defaults (or that the heuristics replay pins).
func liveTechnicalConfig(t *testing.T) config.TechnicalAnalysis {
	t.Helper()
	for k, v := range map[string]string{
		"DATABASE_URL":                       "postgres://unused",
		"TECHNICAL_COMPUTE_LOOKBACK":         "500",
		"TECHNICAL_RSI_PERIOD":               "14",
		"TECHNICAL_TREND_LOOKBACK":           "60",
		"TECHNICAL_MACD_FAST":                "12",
		"TECHNICAL_MACD_SLOW":                "26",
		"TECHNICAL_MACD_SIGNAL":              "9",
		"TECHNICAL_BB_PERIOD":                "20",
		"TECHNICAL_BB_STD":                   "2",
		"TECHNICAL_ATR_PERIOD":               "14",
		"TECHNICAL_KELTNER_EMA":              "20",
		"TECHNICAL_KELTNER_ATR":              "10",
		"TECHNICAL_KELTNER_MULT":             "2",
		"TECHNICAL_OB_SWING_STRENGTH":        "3",
		"TECHNICAL_OB_IMPULSE_MIN_PCT":       "1.5",
		"TECHNICAL_OB_LOOKBACK":              "100",
		"TECHNICAL_LIQUIDITY_SWING_STRENGTH": "3",
		"TECHNICAL_LIQUIDITY_LOOKBACK":       "50",
		"TECHNICAL_HS_SWING_STRENGTH":        "5",
		"TECHNICAL_HS_TOLERANCE_PCT":         "5.0",
		"TECHNICAL_HS_LOOKBACK":              "100",
		"TECHNICAL_FLAG_POLE_PCT":            "5.0",
		"TECHNICAL_FLAG_MAX_RETRACEMENT_PCT": "50.0",
		"TECHNICAL_FLAG_POLE_LEN":            "5",
		"TECHNICAL_FLAG_LEN":                 "10",
	} {
		t.Setenv(k, v)
	}
	cfg, err := config.LoadTechnicalAnalysis()
	if err != nil {
		t.Fatalf("config: %v", err)
	}
	return cfg
}

// goldenBars is a deterministic, regime-switching random walk: long enough to
// fill the 500-bar compute window, volatile enough to produce sweeps, order
// blocks, flags and H&S pivots.
func goldenBars(n int) []compute.Bar {
	bars := make([]compute.Bar, n)
	seed := uint64(0x9E3779B97F4A7C15)
	rnd := func() float64 {
		seed ^= seed << 13
		seed ^= seed >> 7
		seed ^= seed << 17
		return float64(seed%1_000_000)/1_000_000*2 - 1
	}
	price := 40.0
	t0 := time.Date(2020, 1, 2, 0, 0, 0, 0, time.UTC)
	for i := 0; i < n; i++ {
		vol := 0.01 + 0.02*math.Abs(math.Sin(float64(i)/37))
		drift := 0.002 * math.Sin(float64(i)/53)
		o := price
		c := price * (1 + drift + vol*rnd())
		h := math.Max(o, c) * (1 + vol*math.Abs(rnd()))
		l := math.Min(o, c) * (1 - vol*math.Abs(rnd()))
		bars[i] = compute.Bar{TS: t0.AddDate(0, 0, i), Open: o, High: h, Low: l, Close: c, Volume: 1e5 * (1 + math.Abs(rnd()))}
		price = c
	}
	return bars
}

type emitted struct {
	Bars      int             `json:"bars"`
	Indicator string          `json:"indicator"`
	Value     *float64        `json:"value"`
	Payload   json.RawMessage `json:"payload"`
}

func emitAll(t *testing.T, cfg config.TechnicalAnalysis, bars []compute.Bar) []emitted {
	t.Helper()
	var out []emitted
	for _, n := range []int{3, 30, 61, 120, 260, 499, 500} {
		window := bars[len(bars)-n:]
		technical.Emitter{Cfg: cfg}.Emit(window, func(indicator string, value *float64, payload any) {
			var jb []byte
			if payload != nil {
				var err error
				// store.UpsertIndicator marshals exactly this way.
				if jb, err = json.Marshal(payload); err != nil {
					t.Fatalf("marshal %s: %v", indicator, err)
				}
			}
			out = append(out, emitted{Bars: n, Indicator: indicator, Value: value, Payload: jb})
		})
	}
	return out
}

// The worker's stored indicator names, values and payloads are a live
// contract (the analyst bot reads them). This pins every one of them, byte for
// byte, to the output captured before the heuristics extraction.
func TestEmitMatchesGolden(t *testing.T) {
	cfg := liveTechnicalConfig(t)
	got, err := json.MarshalIndent(emitAll(t, cfg, goldenBars(600)), "", " ")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join("testdata", "indicators_golden.json")
	if *updateGolden {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden (run with -update once to create it): %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("worker indicator output changed; diff testdata/indicators_golden.json against:\n%s", firstDiff(want, got))
	}
}

func firstDiff(a, b []byte) string {
	la, lb := bytes.Split(a, []byte("\n")), bytes.Split(b, []byte("\n"))
	for i := 0; i < len(la) && i < len(lb); i++ {
		if !bytes.Equal(la[i], lb[i]) {
			return "line " + strconv.Itoa(i+1) + ":\n  want " + string(la[i]) + "\n  got  " + string(lb[i])
		}
	}
	return "length differs"
}
