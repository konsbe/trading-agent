package config

import (
	"fmt"
	"os"
	"time"
)

// OHLCVBars holds configuration for the data-technical worker, which is now
// responsible solely for fetching daily / weekly OHLCV bars.
//
// Indicator computation has been moved to services/data-analyzer/cmd/technical-analysis.
// The env-var names are kept identical so existing .env files need no changes.
type OHLCVBars struct {
	Base

	AlpacaKey    string
	AlpacaSecret string

	// Symbols and daily/weekly intervals to fetch.
	EquitySymbols   []string
	EquityIntervals []string
	CryptoSymbols   []string
	CryptoIntervals []string

	// BackfillBars is the target history depth ensured on startup.
	BackfillBars int
	// PollInterval is how often to pull the latest bars.
	PollInterval time.Duration
}

func LoadOHLCVBars() (OHLCVBars, error) {
	b := LoadBase()
	if b.DatabaseURL == "" {
		return OHLCVBars{}, fmt.Errorf("DATABASE_URL is required")
	}

	equitySyms := splitCSV("TECHNICAL_EQUITY_SYMBOLS")
	if len(equitySyms) == 0 {
		equitySyms = splitCSV("ALPACA_DATA_SYMBOLS")
	}
	if len(equitySyms) == 0 {
		equitySyms = []string{"AAPL", "MSFT", "SPY"}
	}

	equityIvs := splitCSV("TECHNICAL_EQUITY_INTERVALS")
	if len(equityIvs) == 0 {
		equityIvs = []string{"1Day"}
	}

	cryptoSyms := splitCSV("TECHNICAL_CRYPTO_SYMBOLS")
	if len(cryptoSyms) == 0 {
		cryptoSyms = splitCSV("BINANCE_SYMBOLS")
	}
	if len(cryptoSyms) == 0 {
		cryptoSyms = []string{"BTCUSDT", "ETHUSDT"}
	}

	cryptoIvs := splitCSV("TECHNICAL_CRYPTO_INTERVALS")
	if len(cryptoIvs) == 0 {
		cryptoIvs = []string{"1d"}
	}

	return OHLCVBars{
		Base:            b,
		AlpacaKey:       os.Getenv("APCA_API_KEY_ID"),
		AlpacaSecret:    os.Getenv("APCA_API_SECRET_KEY"),
		EquitySymbols:   equitySyms,
		EquityIntervals: equityIvs,
		CryptoSymbols:   cryptoSyms,
		CryptoIntervals: cryptoIvs,
		BackfillBars:    intEnv("TECHNICAL_BACKFILL_BARS", 500),
		PollInterval:    pollFor("DATA_TECHNICAL_POLL_INTERVAL", 6*time.Hour),
	}, nil
}
