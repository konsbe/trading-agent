package main

import (
	"math"
	"testing"
)

// Observed stored values (equity_fundamentals.metrics_raw, 2026-09-21):
//
//	TSM   marketCapitalization 62885996  (TWD millions; served as $62.9T)
//	AAPL  marketCapitalization 4939254   (USD millions; $4.94T)
func tsmMetrics() map[string]any  { return map[string]any{"marketCapitalization": 62885996.0} }
func aaplMetrics() map[string]any { return map[string]any{"marketCapitalization": 4939254.0} }

func TestMarketCapRow_NonUSDReporterIsNullNotALocalCurrencyFigure(t *testing.T) {
	cur := reportingCurrencyFrom(map[string]any{"currency": "TWD", "estimateCurrency": "TWD"})
	v, payload := marketCapRow(tsmMetrics(), cur)
	if v != nil {
		t.Fatalf("TSM market_cap = %g, want nil: a TWD figure must not be stored as USD", *v)
	}
	if payload["currency"] != "TWD" {
		t.Errorf("payload currency = %v, want TWD", payload["currency"])
	}
	if payload["market_cap_millions_local"] != 62885996.0 {
		t.Errorf("payload local figure = %v, want the raw 62885996", payload["market_cap_millions_local"])
	}
}

// Which profile2 field governs marketCapitalization is undocumented on the free
// tier, so any foreign code among them is enough.
func TestMarketCapRow_AnyForeignCurrencyFieldNullsTheValue(t *testing.T) {
	for _, prof := range []map[string]any{
		{"currency": "USD", "estimateCurrency": "TWD"},
		{"currency": "ILS"},
		{"currency": "USD", "marketCapCurrency": "GBP"},
		{"currency": " twd "},
	} {
		if v, _ := marketCapRow(tsmMetrics(), reportingCurrencyFrom(prof)); v != nil {
			t.Errorf("profile %v: market_cap = %g, want nil", prof, *v)
		}
	}
}

func TestMarketCapRow_USDReporterIsConvertedFromMillions(t *testing.T) {
	cur := reportingCurrencyFrom(map[string]any{"currency": "USD", "estimateCurrency": "USD"})
	v, payload := marketCapRow(aaplMetrics(), cur)
	if v == nil || math.Abs(*v-4.939254e12) > 1 {
		t.Fatalf("AAPL market_cap = %v, want 4.939254e12", v)
	}
	if payload["currency"] != "USD" {
		t.Errorf("payload currency = %v, want USD", payload["currency"])
	}
}

func TestMarketCapRow_UnknownCurrencyKeepsTheUSDAssumptionButSaysSo(t *testing.T) {
	v, payload := marketCapRow(aaplMetrics(), reportingCurrency{})
	if v == nil || math.Abs(*v-4.939254e12) > 1 {
		t.Fatalf("market_cap = %v, want 4.939254e12", v)
	}
	if payload["currency_basis"] == nil {
		t.Error("an assumed currency must be recorded as assumed")
	}
}

func TestMarketCapRow_MissingFigureWritesNothing(t *testing.T) {
	v, payload := marketCapRow(map[string]any{}, reportingCurrencyFrom(map[string]any{"currency": "TWD"}))
	if v != nil || payload != nil {
		t.Errorf("got (%v, %v), want (nil, nil)", v, payload)
	}
}
