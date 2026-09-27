//go:build integration

package runner

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/konsbe/trading-agent/services/data-analyzer/internal/compute"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/config"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/heuristics"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/technical"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/testdb"
)

func walk(n int) []compute.Bar {
	bars := make([]compute.Bar, n)
	price := 50.0
	t0 := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)
	for i := range bars {
		c := price * (1 + 0.02*math.Sin(float64(i)/5) + 0.004*math.Cos(float64(i)/2))
		bars[i] = compute.Bar{TS: t0.AddDate(0, 0, i), Open: price, High: math.Max(price, c) * 1.01,
			Low: math.Min(price, c) * 0.99, Close: c, Volume: 1e5 + float64(i%7)*1e4}
		price = c
	}
	return bars
}

type row struct {
	Value   *float64
	Payload any
}

func normalise(t *testing.T, payload []byte) any {
	t.Helper()
	if payload == nil {
		return nil
	}
	var v any
	if err := json.Unmarshal(payload, &v); err != nil {
		t.Fatal(err)
	}
	return v
}

// What ComputeAndStore writes for a symbol is exactly what the pinned emitter
// produces for the same bars, at the last bar's ts, plus the VIX regime row
// read from macro_fred.
func TestComputeAndStoreWritesTheEmitterOutput(t *testing.T) {
	ctx := context.Background()
	t.Setenv("DATABASE_URL", "postgres://unused")
	cfg, err := config.LoadTechnicalAnalysis()
	if err != nil {
		t.Fatal(err)
	}
	tx := testdb.Tx(t)
	bars := walk(300)
	for _, b := range bars {
		if _, err := tx.Exec(ctx, `INSERT INTO equity_ohlcv (ts, symbol, interval, open, high, low, close, volume, source)
			VALUES ($1, 'ZZTA01', '1Day', $2, $3, $4, $5, $6, 'tiingo')`, b.TS, b.Open, b.High, b.Low, b.Close, b.Volume); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := tx.Exec(ctx, `INSERT INTO macro_fred (ts, series_id, value) VALUES (now() + interval '1 day', 'VIXCLS', 27.5)`); err != nil {
		t.Fatal(err)
	}

	res, err := ComputeAndStore(ctx, tx, "ZZTA01", "equity", "1Day", cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil || !res.Computed || res.Bars != 300 || !res.TS.Equal(bars[299].TS) {
		t.Fatalf("result %+v, err %v", res, err)
	}

	want := map[string]row{}
	technical.Emitter{Cfg: cfg}.Emit(bars, func(name string, v *float64, payload any) {
		var jb []byte
		if payload != nil {
			jb, _ = json.Marshal(payload)
		}
		want[name] = row{v, normalise(t, jb)}
	})

	rows, err := tx.Query(ctx, `SELECT indicator, value, payload::text, ts FROM technical_indicators
		WHERE symbol = 'ZZTA01' AND exchange = 'equity' AND interval = '1Day'`)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]row{}
	for rows.Next() {
		var name string
		var v *float64
		var payload *string
		var ts time.Time
		if err := rows.Scan(&name, &v, &payload, &ts); err != nil {
			t.Fatal(err)
		}
		if !ts.Equal(bars[299].TS) {
			t.Errorf("%s written at %v, want the last bar's ts", name, ts)
		}
		var jb []byte
		if payload != nil {
			jb = []byte(*payload)
		}
		got[name] = row{v, normalise(t, jb)}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}

	vix, ok := got["vix_regime"]
	if !ok || vix.Value == nil || *vix.Value != 27.5 || vix.Payload.(map[string]any)["regime"] != "elevated" {
		t.Errorf("vix_regime = %+v", vix)
	}
	delete(got, "vix_regime")

	// alert_onsets: the bot's onset flags for the last bar, from heuristics.OnsetsAt.
	onsets, ok := got["alert_onsets"]
	wantOn := heuristics.OnsetsAt(bars, len(bars)-1, heuristics.Config{Lookback: cfg.ComputeLookback, Params: technical.ParamsFrom(cfg)})
	if !ok || onsets.Value == nil || int(*onsets.Value) != wantOn.Count() {
		t.Errorf("alert_onsets = %+v, want count %d", onsets, wantOn.Count())
	} else {
		pl := onsets.Payload.(map[string]any)
		if pl["bar_date"] != bars[299].TS.UTC().Format(time.DateOnly) || pl["gap_sessions"] != 5.0 || pl["judged"] != true ||
			pl["bb_squeeze_onset"] != wantOn.BBSqueezeOnset || pl["sweep_onset"] != wantOn.SweepOnset ||
			pl["rsi_overbought_onset"] != wantOn.RSIOverbought || pl["rsi_oversold_onset"] != wantOn.RSIOversold {
			t.Errorf("alert_onsets payload = %v, want %+v", pl, wantOn)
		}
	}
	delete(got, "alert_onsets")
	delete(got, "pivots_weekly") // no 1Week bars seeded
	if len(want) < 40 {
		t.Fatalf("emitter produced only %d indicators", len(want))
	}
	for name, w := range want {
		g, ok := got[name]
		if !ok {
			t.Errorf("%s: emitted but not stored", name)
			continue
		}
		if (w.Value == nil) != (g.Value == nil) || (w.Value != nil && *w.Value != *g.Value) || !reflect.DeepEqual(w.Payload, g.Payload) {
			t.Errorf("%s: stored %+v, emitter %+v", name, g, w)
		}
		delete(got, name)
	}
	for name := range got {
		t.Errorf("%s: stored but not emitted", name)
	}
}

func TestComputeAndStoreTooFewBarsIsNotAnError(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://unused")
	cfg, err := config.LoadTechnicalAnalysis()
	if err != nil {
		t.Fatal(err)
	}
	res, err := ComputeAndStore(context.Background(), testdb.Tx(t), "ZZTA02", "equity", "1Day", cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil || res.Computed || res.Bars != 0 {
		t.Errorf("no bars: %+v %v", res, err)
	}
}
