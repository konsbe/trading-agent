package main

import (
	"math"
	"strings"
	"testing"
)

// Fixtures are the stored /stock/profile2 responses and latest USD closes of
// 2026-09-26 (equity_fundamentals.profile_raw, equity_ohlcv 1Day).
type observed struct {
	name     string
	prof     map[string]any
	usdClose float64
	metricMC float64 // /stock/metric marketCapitalization
}

var (
	tsm = observed{"TSM", map[string]any{"currency": "TWD", "estimateCurrency": "TWD",
		"exchange": "TAIWAN STOCK EXCHANGE", "marketCapitalization": 64182613.623047, "shareOutstanding": 25932.37}, 450.61, 64182612}
	teva = observed{"TEVA", map[string]any{"currency": "USD", "estimateCurrency": "USD",
		"exchange": "TEL AVIV STOCK EXCHANGE", "marketCapitalization": 140418.185181, "shareOutstanding": 1164.6}, 39.19, 140418.2}
	hafn = observed{"HAFN", map[string]any{"currency": "USD", "estimateCurrency": "USD",
		"exchange": "OSLO BORS ASA", "marketCapitalization": 40997.260606, "shareOutstanding": 499.8}, 8.92, 40997.3}
	azn = observed{"AZN", map[string]any{"currency": "USD", "estimateCurrency": "USD",
		"exchange": "LONDON STOCK EXCHANGE", "marketCapitalization": 192311.088379, "shareOutstanding": 1551.0}, 166.58, 192311.1}
	aapl = observed{"AAPL", map[string]any{"currency": "USD", "estimateCurrency": "USD",
		"exchange": "NASDAQ NMS - GLOBAL MARKET", "marketCapitalization": 4977637.064987, "shareOutstanding": 14687.36}, 341.07, 4963043}
	// Share-class quirk: profile2 counts shares on a different basis than the
	// B-share close, so the ratio is far from 1 — but the listing is US and the
	// figure genuinely USD.
	brkb = observed{"BRK.B", map[string]any{"currency": "USD", "estimateCurrency": "USD",
		"exchange": "NEW YORK STOCK EXCHANGE, INC.", "marketCapitalization": 972269.61, "shareOutstanding": 1.44}, 505.48, 971461.4}
)

func (o observed) row() (*float64, map[string]any) {
	c := o.usdClose
	return marketCapRow(map[string]any{"marketCapitalization": o.metricMC}, marketCapBasisFrom(o.prof, &c))
}

// TSM's TWD figure was served as $62.9T.
func TestMarketCapRow_NonUSDReportingCurrencyIsNull(t *testing.T) {
	v, p := tsm.row()
	if v != nil {
		t.Fatalf("TSM market_cap = %g, want nil", *v)
	}
	if p["currency"] != "TWD" || p["market_cap_millions_local"] != tsm.metricMC {
		t.Errorf("payload = %v, want currency TWD and the local figure", p)
	}
}

// The filing currency says USD for these, yet Finnhub priced the Tel Aviv,
// Oslo and London listings: ILS, NOK and GBP figures.
func TestMarketCapRow_ForeignListingThatDoesNotReconcileIsNull(t *testing.T) {
	for _, o := range []observed{teva, hafn, azn} {
		v, p := o.row()
		if v != nil {
			t.Errorf("%s market_cap = %g, want nil: a %v figure is not USD", o.name, *v, p["listing_exchange"])
			continue
		}
		if !strings.Contains(p["note"].(string), "non-US listing") {
			t.Errorf("%s note = %q, want the listing reason", o.name, p["note"])
		}
	}
}

func TestMarketCapRow_ForeignListingThatReconcilesIsKept(t *testing.T) {
	o := azn
	o.usdClose = 192311.088379 / 1551.0 // implied price equals the USD close
	if v, _ := o.row(); v == nil {
		t.Error("a foreign-listed figure that matches the USD price must be kept")
	}
}

func TestMarketCapRow_ForeignListingWithoutAUSDCloseIsNull(t *testing.T) {
	b := marketCapBasisFrom(teva.prof, nil)
	if v, _ := marketCapRow(map[string]any{"marketCapitalization": teva.metricMC}, b); v != nil {
		t.Errorf("unverifiable foreign listing stored %g, want nil", *v)
	}
}

func TestMarketCapRow_USListingIsConvertedFromMillions(t *testing.T) {
	for _, o := range []observed{aapl, brkb} {
		v, p := o.row()
		if v == nil || math.Abs(*v-o.metricMC*1e6) > 1 {
			t.Errorf("%s market_cap = %v, want %g", o.name, v, o.metricMC*1e6)
			continue
		}
		if p["currency"] != "USD" {
			t.Errorf("%s payload currency = %v, want USD", o.name, p["currency"])
		}
	}
}

func TestMarketCapRow_NoProfileKeepsTheUSDAssumptionButSaysSo(t *testing.T) {
	v, p := marketCapRow(map[string]any{"marketCapitalization": aapl.metricMC}, marketCapBasisFrom(nil, nil))
	if v == nil {
		t.Fatal("want the historical USD conversion when nothing is known")
	}
	if p["currency_basis"] == nil {
		t.Error("an assumed currency must be recorded as assumed")
	}
}

func TestMarketCapRow_MissingFigureWritesNothing(t *testing.T) {
	c := tsm.usdClose
	v, p := marketCapRow(map[string]any{}, marketCapBasisFrom(tsm.prof, &c))
	if v != nil || p != nil {
		t.Errorf("got (%v, %v), want (nil, nil)", v, p)
	}
}

func TestUSDPriceRatio_MatchesObservedFXClusters(t *testing.T) {
	for _, c := range []struct {
		o      observed
		lo, hi float64
	}{
		{teva, 3.0, 3.2},  // ILS
		{hafn, 9.0, 9.4},  // NOK
		{azn, 0.72, 0.77}, // GBP
		{aapl, 0.97, 1.03},
	} {
		cl := c.o.usdClose
		r := marketCapBasisFrom(c.o.prof, &cl).USDPriceRatio
		if r == nil || *r < c.lo || *r > c.hi {
			t.Errorf("%s ratio = %v, want within [%g, %g]", c.o.name, r, c.lo, c.hi)
		}
	}
}
