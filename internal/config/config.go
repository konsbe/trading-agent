package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

func env(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func durationEnv(key string, def time.Duration) time.Duration {
	s := strings.TrimSpace(os.Getenv(key))
	if s == "" {
		return def
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return def
	}
	return d
}

func splitCSV(key string) []string {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

type Base struct {
	DatabaseURL  string
	PollInterval time.Duration
	LogLevel     string
}

func LoadBase() Base {
	return Base{
		DatabaseURL:  env("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/trading?sslmode=disable"),
		PollInterval: durationEnv("DATA_POLL_INTERVAL", 60*time.Second),
		LogLevel:     env("LOG_LEVEL", "info"),
	}
}

type Crypto struct {
	Base
	BinanceSymbols []string
	BinanceInterval string
	CoinGeckoGlobal bool
}

func LoadCrypto() (Crypto, error) {
	b := LoadBase()
	if b.DatabaseURL == "" {
		return Crypto{}, fmt.Errorf("DATABASE_URL is required")
	}
	syms := splitCSV("BINANCE_SYMBOLS")
	if len(syms) == 0 {
		syms = []string{"BTCUSDT", "ETHUSDT"}
	}
	iv := env("BINANCE_INTERVAL", "1h")
	return Crypto{
		Base:            b,
		BinanceSymbols:  syms,
		BinanceInterval: iv,
		CoinGeckoGlobal: env("COINGECKO_POLL_GLOBAL", "true") == "true",
	}, nil
}

type Equity struct {
	Base
	AlpacaKey     string
	AlpacaSecret  string
	AlpacaBaseURL string
	Symbols       []string
	FredAPIKey    string
	FredSeries    []string
	FinnhubKey    string
}

func LoadEquity() (Equity, error) {
	b := LoadBase()
	if b.DatabaseURL == "" {
		return Equity{}, fmt.Errorf("DATABASE_URL is required")
	}
	syms := splitCSV("ALPACA_DATA_SYMBOLS")
	if len(syms) == 0 {
		syms = []string{"SPY", "QQQ"}
	}
	fred := splitCSV("FRED_SERIES_IDS")
	if len(fred) == 0 {
		fred = []string{"DGS10", "VIXCLS"}
	}
	return Equity{
		Base:          b,
		AlpacaKey:     os.Getenv("APCA_API_KEY_ID"),
		AlpacaSecret:  os.Getenv("APCA_API_SECRET_KEY"),
		AlpacaBaseURL: env("APCA_API_BASE_URL", "https://paper-api.alpaca.markets"),
		Symbols:       syms,
		FredAPIKey:    os.Getenv("FRED_API_KEY"),
		FredSeries:    fred,
		FinnhubKey:    os.Getenv("FINNHUB_API_KEY"),
	}, nil
}

type Onchain struct {
	Base
	GlassnodeKey string
	EtherscanKey string
}

func LoadOnchain() (Onchain, error) {
	b := LoadBase()
	if b.DatabaseURL == "" {
		return Onchain{}, fmt.Errorf("DATABASE_URL is required")
	}
	return Onchain{
		Base:         b,
		GlassnodeKey: os.Getenv("GLASSNODE_API_KEY"),
		EtherscanKey: os.Getenv("ETHERSCAN_API_KEY"),
	}, nil
}

type Sentiment struct {
	Base
	LunarCrushKey string
	FinnhubKey    string
	NewsSymbols   []string
}

func LoadSentiment() (Sentiment, error) {
	b := LoadBase()
	if b.DatabaseURL == "" {
		return Sentiment{}, fmt.Errorf("DATABASE_URL is required")
	}
	newsSyms := splitCSV("FINNHUB_SYMBOLS_FOR_NEWS")
	if len(newsSyms) == 0 {
		newsSyms = []string{"BTC", "ETH"}
	}
	return Sentiment{
		Base:          b,
		LunarCrushKey: os.Getenv("LUNARCRUSH_API_KEY"),
		FinnhubKey:    os.Getenv("FINNHUB_API_KEY"),
		NewsSymbols:   newsSyms,
	}, nil
}
