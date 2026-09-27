//go:build integration

package fundamental

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"slices"
	"testing"

	"github.com/konsbe/trading-agent/services/data-analyzer/internal/config"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/store"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/testdb"
)

// The deterioration warning's FCF condition read only fcf_eps_divergence: EPS
// growth > 10% with FCF yield < 2%. FCF yield divides by market value, so on
// 2026-09-26 MSFT (conversion 0.70) and COHR (3.9) met it on price alone. It
// now also needs cash weak against earnings. Rows are the live payloads.
func TestDeteriorationFCFConditionNeedsWeakCash(t *testing.T) {
	warning := `{"quality": "warning_eps_growing_fcf_low"}`
	for _, tc := range []struct {
		name       string
		conversion *string // t3_fcf_conversion payload; nil = no row
		convValue  *float64
		fcfYield   float64
		want       bool
	}{
		// GOOGL: FCF 73,266 vs net income 132,170.
		{"GOOGL conversion 0.55", ptrS(`{"tier": "accrual_concern"}`), ptrF(0.5543), 1.6814, true},
		// MSFT: FCF 71,611 vs net income 101,832 — the low yield is price.
		{"MSFT conversion 0.70", ptrS(`{"tier": "moderate"}`), ptrF(0.7032), 1.8797, false},
		// Negative FCF: conversion is not computed (INTC's FCF yield −0.75).
		{"negative FCF", ptrS(`{"status": "not_computable"}`), nil, -0.7516, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			t.Setenv("DATABASE_URL", "postgres://unused")
			cfg, err := config.LoadFundamentalAnalysis()
			if err != nil {
				t.Fatal(err)
			}
			tx := testdb.Tx(t)
			const sym = "ZZDETER"
			seed := func(metric string, value *float64, payload string) {
				if _, err := tx.Exec(ctx, `INSERT INTO equity_fundamentals (ts, symbol, period, metric, value, payload, source)
					VALUES (now(), $1, 'derived', $2, $3, $4::jsonb, 'fundamental_analysis')`, sym, metric, value, payload); err != nil {
					t.Fatal(err)
				}
			}
			seed("eps_strength", ptrF(1), `{"tier": "strong"}`)
			seed("fcf_eps_divergence", ptrF(-1), warning)
			seed("fcf_yield", ptrF(tc.fcfYield), `{}`)
			if tc.conversion != nil {
				seed("t3_fcf_conversion", tc.convValue, *tc.conversion)
			}
			w := &analyzer{cfg: cfg, pool: tx, log: slog.New(slog.NewTextHandler(io.Discard, nil))}
			w.scoreCorrelations(ctx, sym, nil)

			rows, err := store.QueryLatestDerived(ctx, tx, sym)
			if err != nil {
				t.Fatal(err)
			}
			var master derivedLatest
			for _, r := range rows {
				if r.Metric == "corr_master_signals" {
					master = derivedLatest{Value: r.Value}
					_ = json.Unmarshal(r.Payload, &master.Payload)
				}
			}
			got := slices.Contains(master.conditions("deterioration_warning"), "fcf_accruals_concern")
			if got != tc.want {
				t.Errorf("fcf_accruals_concern met = %v, want %v (conditions %v)", got, tc.want, master.conditions("deterioration_warning"))
			}
		})
	}
}

func ptrS(s string) *string   { return &s }
func ptrF(f float64) *float64 { return &f }
