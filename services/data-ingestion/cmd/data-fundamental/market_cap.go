package main

import (
	"context"
	"strings"

	"github.com/konsbe/trading-agent/services/data-ingestion/internal/store"
)

// reportingCurrency is what /stock/profile2 says about the currency Finnhub's
// figures are denominated in.
//
// /stock/metric carries no currency field at all, yet its marketCapitalization
// is in millions of the company's REPORTING currency, not USD. TSM is the case
// that exposed it: the whole payload is on the Taiwan listing's basis
// (52WeekHigh 2535, epsTTM 87.38 — TWD per ordinary share, while the NYSE ADR
// trades near $450), so 62,885,996 "millions" is TWD 62.9T, about $2T, and was
// stored and served as $62.9T.
type reportingCurrency struct {
	Currency          string // profile2 "currency": currency of company filings
	EstimateCurrency  string // profile2 "estimateCurrency"
	MarketCapCurrency string // profile2 "marketCapCurrency" (not on every tier)
}

func reportingCurrencyFrom(prof map[string]any) reportingCurrency {
	s := func(k string) string {
		v, _ := prof[k].(string)
		return strings.ToUpper(strings.TrimSpace(v))
	}
	return reportingCurrency{
		Currency:          s("currency"),
		EstimateCurrency:  s("estimateCurrency"),
		MarketCapCurrency: s("marketCapCurrency"),
	}
}

func (c reportingCurrency) known() bool {
	return c.Currency != "" || c.EstimateCurrency != "" || c.MarketCapCurrency != ""
}

// nonUSD returns the first non-USD code among the fields present, or "".
// Any one of them being foreign is enough to distrust the figure: which field
// governs marketCapitalization is undocumented for the free tier.
func (c reportingCurrency) nonUSD() string {
	for _, v := range []string{c.MarketCapCurrency, c.Currency, c.EstimateCurrency} {
		if v != "" && v != "USD" {
			return v
		}
	}
	return ""
}

// marketCapRow decides the stored market_cap value and payload.
//
// USD reporters are converted from millions as before. Non-USD reporters are
// stored as NULL with the local figure and currency in the payload: the only
// FX series in the store are JPY and EUR (DEXJPUS, DEXUSEU), so a conversion
// would be unavailable for most currencies seen (TWD, ILS, GBP, NOK, ...), and a
// wrong number is worse than an honest absence — every consumer already
// handles a null market cap. An unknown currency keeps the historical USD
// assumption but says so in the payload.
func marketCapRow(metricMap map[string]any, cur reportingCurrency) (*float64, map[string]any) {
	raw := floatPtr(metricMap, "marketCapitalization")
	if raw == nil {
		return nil, nil
	}
	if code := cur.nonUSD(); code != "" {
		return nil, map[string]any{
			"currency":                  code,
			"market_cap_millions_local": *raw,
			"note":                      "Finnhub marketCapitalization is in the reporting currency; no FX conversion to USD is available, so market_cap is null",
		}
	}
	if !cur.known() {
		return mulM(raw), map[string]any{
			"currency":       nil,
			"currency_basis": "assumed USD: no /stock/profile2 currency available",
		}
	}
	return mulM(raw), map[string]any{"currency": "USD"}
}

// currencyForMarketCap resolves the reporting currency from this pass's
// profile2 response, falling back to the last stored profile when this pass's
// request failed — otherwise a single transient profile failure would write a
// local-currency figure as USD again and it would become the newest row.
func (w *worker) currencyForMarketCap(ctx context.Context, sym string, prof map[string]any) reportingCurrency {
	if cur := reportingCurrencyFrom(prof); cur.known() {
		return cur
	}
	stored, err := store.LatestFundamentalPayload(ctx, w.pool, sym, "profile_raw")
	if err != nil {
		w.log.Warn("stored profile lookup failed; market_cap currency unknown", "symbol", sym, "err", err)
		return reportingCurrency{}
	}
	return reportingCurrencyFrom(stored)
}

// writeMarketCap stores market_cap and, for a non-USD reporter, repairs the
// rows written before the currency was checked.
func (w *worker) writeMarketCap(ctx context.Context, upsert func(string, *float64, any), sym string, metricMap map[string]any, cur reportingCurrency) {
	value, payload := marketCapRow(metricMap, cur)
	if payload == nil {
		upsert("market_cap", value, nil)
		return
	}
	upsert("market_cap", value, payload)
	code := cur.nonUSD()
	if code == "" {
		return
	}
	n, err := store.NullNonUSDMarketCapHistory(ctx, w.pool, sym, code)
	if err != nil {
		w.log.Error("repair non-USD market_cap history", "symbol", sym, "err", err)
		return
	}
	if n > 0 {
		w.log.Info("nulled local-currency market_cap rows previously stored as USD",
			"symbol", sym, "currency", code, "rows", n)
	}
}
