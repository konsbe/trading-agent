package main

import (
	"context"
	"strings"

	"github.com/konsbe/trading-agent/services/data-ingestion/internal/store"
)

// marketCapBasis is what can be established about the currency Finnhub's
// marketCapitalization is denominated in.
//
// /stock/metric carries no currency field, and its marketCapitalization is in
// millions of the currency of the LISTING Finnhub resolved the symbol to — not
// USD, and not necessarily the filing currency either:
//
//	TSM   TAIWAN STOCK EXCHANGE     currency TWD  62,885,996 = TWD 62.9T, served as $62.9T
//	TEVA  TEL AVIV STOCK EXCHANGE   currency USD  140,418    = ILS, about 3.08x its USD cap
//	HAFN  OSLO BORS ASA             currency USD  40,997     = NOK, about 9.2x
//	AZN   LONDON STOCK EXCHANGE     currency USD  192,311    = GBP, about 0.74x
//
// profile2's "currency" is the filing currency, so it catches TSM but reports
// USD for the other three. The listing is what matters, so a non-US exchange
// is only trusted when the figure reconciles with the stored USD close.
type marketCapBasis struct {
	Currency          string // profile2 "currency": currency of company filings
	EstimateCurrency  string // profile2 "estimateCurrency"
	MarketCapCurrency string // profile2 "marketCapCurrency" (not on every tier)
	Exchange          string // profile2 "exchange": the listing Finnhub priced

	// USDPriceRatio is profile2 marketCapitalization / (shareOutstanding × the
	// latest stored USD close): the listing's implied share price over the US
	// price. ~1 for a USD figure; the FX rate (times any ADR ratio) otherwise.
	// nil when any input is missing.
	USDPriceRatio *float64
}

// A USD figure reconciles within share-count staleness and a day's price move;
// every foreign-currency case observed sits well outside (GBP 0.74, CAD ~1.4,
// ILS ~3.1, TWD-per-ADR 5.5, NOK ~9).
const (
	usdRatioMin = 0.87
	usdRatioMax = 1.15
)

func marketCapBasisFrom(prof map[string]any, usdClose *float64) marketCapBasis {
	s := func(k string) string {
		v, _ := prof[k].(string)
		return strings.ToUpper(strings.TrimSpace(v))
	}
	b := marketCapBasis{
		Currency:          s("currency"),
		EstimateCurrency:  s("estimateCurrency"),
		MarketCapCurrency: s("marketCapCurrency"),
		Exchange:          s("exchange"),
	}
	mc, sh := floatPtr(prof, "marketCapitalization"), floatPtr(prof, "shareOutstanding")
	if mc != nil && sh != nil && usdClose != nil && *mc > 0 && *sh > 0 && *usdClose > 0 {
		r := *mc / (*sh * *usdClose)
		b.USDPriceRatio = &r
	}
	return b
}

func (b marketCapBasis) known() bool {
	return b.Currency != "" || b.EstimateCurrency != "" || b.MarketCapCurrency != "" || b.Exchange != ""
}

// nonUSD returns the first non-USD code among the currency fields present.
func (b marketCapBasis) nonUSD() string {
	for _, v := range []string{b.MarketCapCurrency, b.Currency, b.EstimateCurrency} {
		if v != "" && v != "USD" {
			return v
		}
	}
	return ""
}

func isUSExchange(ex string) bool {
	for _, m := range []string{"NASDAQ", "NEW YORK STOCK EXCHANGE", "NYSE", "OTC", "CBOE", "BATS", "IEX"} {
		if strings.Contains(ex, m) {
			return true
		}
	}
	return false
}

// untrusted returns why the figure cannot be taken as USD, or "".
func (b marketCapBasis) untrusted() string {
	if code := b.nonUSD(); code != "" {
		return "reporting currency " + code
	}
	if b.Exchange == "" || isUSExchange(b.Exchange) {
		return ""
	}
	if b.USDPriceRatio != nil && *b.USDPriceRatio >= usdRatioMin && *b.USDPriceRatio <= usdRatioMax {
		return ""
	}
	return "priced on a non-US listing and does not reconcile with the USD close"
}

// payload records the basis on every market_cap row.
func (b marketCapBasis) payload() map[string]any {
	p := map[string]any{}
	if b.Currency != "" {
		p["currency"] = b.Currency
	}
	if b.Exchange != "" {
		p["listing_exchange"] = b.Exchange
	}
	if b.USDPriceRatio != nil {
		p["usd_price_ratio"] = *b.USDPriceRatio
	}
	return p
}

// marketCapRow decides the stored market_cap value and payload.
//
// A trusted figure is converted from millions as before. An untrusted one is
// stored as NULL with the local figure and the reason in the payload: the only
// FX series in the store are JPY and EUR (DEXJPUS, DEXUSEU), so a conversion
// would be unavailable for most currencies seen, and a wrong number is worse
// than an honest absence — every consumer already handles a null market cap.
// No profile at all keeps the historical USD assumption but says so.
func marketCapRow(metricMap map[string]any, b marketCapBasis) (*float64, map[string]any) {
	raw := floatPtr(metricMap, "marketCapitalization")
	if raw == nil {
		return nil, nil
	}
	p := b.payload()
	if why := b.untrusted(); why != "" {
		p["market_cap_millions_local"] = *raw
		p["note"] = "market_cap is null: Finnhub marketCapitalization is not in USD (" + why + ") and no FX conversion is available"
		return nil, p
	}
	if !b.known() {
		p["currency_basis"] = "assumed USD: no /stock/profile2 data available"
	}
	return mulM(raw), p
}

// marketCapBasisFor resolves the basis from this pass's profile2 response,
// falling back to the last stored profile when this pass's request failed —
// otherwise a single transient profile failure would write a local-currency
// figure as USD again and it would become the newest row.
func (w *worker) marketCapBasisFor(ctx context.Context, sym string, prof map[string]any) marketCapBasis {
	if len(prof) == 0 {
		stored, err := store.LatestFundamentalPayload(ctx, w.pool, sym, "profile_raw")
		if err != nil {
			w.log.Warn("stored profile lookup failed; market_cap basis unknown", "symbol", sym, "err", err)
		}
		prof = stored
	}
	usdClose, err := store.LatestDailyClose(ctx, w.pool, sym)
	if err != nil {
		w.log.Warn("latest close lookup failed; market_cap USD reconciliation unavailable", "symbol", sym, "err", err)
	}
	return marketCapBasisFrom(prof, usdClose)
}

// writeMarketCap stores market_cap and, when the figure is not USD, repairs the
// rows written before the basis was checked.
func (w *worker) writeMarketCap(ctx context.Context, upsert func(string, *float64, any), sym string, metricMap map[string]any, b marketCapBasis) {
	value, payload := marketCapRow(metricMap, b)
	if payload == nil {
		upsert("market_cap", value, nil)
		return
	}
	upsert("market_cap", value, payload)
	if value != nil {
		return
	}
	reason := b.payload()
	reason["note"] = "nulled: Finnhub marketCapitalization is not in USD (" + b.untrusted() + ")"
	n, err := store.NullNonUSDMarketCapHistory(ctx, w.pool, sym, reason)
	if err != nil {
		w.log.Error("repair non-USD market_cap history", "symbol", sym, "err", err)
		return
	}
	if n > 0 {
		w.log.Info("nulled non-USD market_cap rows previously stored as USD",
			"symbol", sym, "reason", b.untrusted(), "rows", n)
	}
}
