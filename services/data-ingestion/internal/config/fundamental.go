package config

import (
	"fmt"
	"os"
	"time"
)

// Fundamental holds configuration for the data-fundamental service.
type Fundamental struct {
	Base

	// Finnhub API key (reused from FINNHUB_API_KEY).
	FinnhubKey string

	// Equity symbols to fetch fundamentals for.
	Symbols []string

	// Poll intervals.
	PollMetrics     time.Duration // /stock/metric — TTM ratios updated daily
	PollFinancials  time.Duration // /stock/financials-reported — quarterly cadence
	PollEarnings    time.Duration // /stock/earnings — updated after each report

	// Feature toggles.
	EnableMetrics     bool // Tier 1: EPS, revenue, P/E, FCF, margins (TTM ratios)
	EnableFinancials  bool // Detailed quarterly/annual income + cash-flow statements
	EnableEarnings    bool // Historical EPS actuals vs estimates (earnings surprises)

	// Frequency for financials-reported endpoint ("quarterly" or "annual").
	FinancialsFreq string
}

func LoadFundamental() (Fundamental, error) {
	b := LoadBase()
	if b.DatabaseURL == "" {
		return Fundamental{}, fmt.Errorf("DATABASE_URL is required")
	}

	syms := splitCSV("FUNDAMENTAL_SYMBOLS")
	if len(syms) == 0 {
		// Fall back to the equity symbol list so a single env variable covers both.
		syms = splitCSV("ALPACA_DATA_SYMBOLS")
	}
	if len(syms) == 0 {
		syms = []string{"AAPL", "MSFT", "SPY"}
	}

	defaultMetrics := 24 * time.Hour
	defaultFinancials := 7 * 24 * time.Hour // weekly — financials change at most quarterly
	defaultEarnings := 24 * time.Hour

	freq := "quarterly"
	if v := os.Getenv("FUNDAMENTAL_FINANCIALS_FREQ"); v == "annual" {
		freq = "annual"
	}

	return Fundamental{
		Base:             b,
		FinnhubKey:       os.Getenv("FINNHUB_API_KEY"),
		Symbols:          syms,
		PollMetrics:      pollFor("DATA_FUNDAMENTAL_METRICS_POLL_INTERVAL", defaultMetrics),
		PollFinancials:   pollFor("DATA_FUNDAMENTAL_FINANCIALS_POLL_INTERVAL", defaultFinancials),
		PollEarnings:     pollFor("DATA_FUNDAMENTAL_EARNINGS_POLL_INTERVAL", defaultEarnings),
		EnableMetrics:    env("FUNDAMENTAL_ENABLE_METRICS", "true") == "true",
		EnableFinancials: env("FUNDAMENTAL_ENABLE_FINANCIALS", "true") == "true",
		EnableEarnings:   env("FUNDAMENTAL_ENABLE_EARNINGS", "true") == "true",
		FinancialsFreq:   freq,
	}, nil
}
