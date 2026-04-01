package config

import (
	"fmt"
	"os"
	"strconv"
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

// defaultPoll returns DATA_POLL_INTERVAL; used when a per-source interval env is unset.
func defaultPoll() time.Duration {
	return durationEnv("DATA_POLL_INTERVAL", 60*time.Second)
}

// pollFor returns duration from key, or fallback when key is empty or invalid.
func pollFor(key string, fallback time.Duration) time.Duration {
	s := strings.TrimSpace(os.Getenv(key))
	if s == "" {
		return fallback
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return fallback
	}
	return d
}

func intEnv(key string, def int) int {
	s := strings.TrimSpace(os.Getenv(key))
	if s == "" {
		return def
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return v
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
	DatabaseURL string
	LogLevel    string
}

func LoadBase() Base {
	return Base{
		DatabaseURL: env("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/trading?sslmode=disable"),
		LogLevel:    env("LOG_LEVEL", "info"),
	}
}

type Crypto struct {
	Base
	PollBinanceREST time.Duration
	PollCoinGecko   time.Duration
	BinanceSymbols  []string
	BinanceInterval string
	CoinGeckoGlobal bool
}

func LoadCrypto() (Crypto, error) {
	b := LoadBase()
	if b.DatabaseURL == "" {
		return Crypto{}, fmt.Errorf("DATABASE_URL is required")
	}
	def := defaultPoll()
	syms := splitCSV("BINANCE_SYMBOLS")
	if len(syms) == 0 {
		syms = []string{"BTCUSDT", "ETHUSDT"}
	}
	iv := env("BINANCE_INTERVAL", "1h")
	return Crypto{
		Base:            b,
		PollBinanceREST: pollFor("DATA_CRYPTO_BINANCE_REST_POLL_INTERVAL", def),
		PollCoinGecko:   pollFor("DATA_CRYPTO_COINGECKO_POLL_INTERVAL", def),
		BinanceSymbols:  syms,
		BinanceInterval: iv,
		CoinGeckoGlobal: env("COINGECKO_POLL_GLOBAL", "true") == "true",
	}, nil
}

type Equity struct {
	Base
	PollAlpaca  time.Duration
	PollFinnhub time.Duration
	PollFred    time.Duration
	AlpacaKey   string
	AlpacaSecret string
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
	def := defaultPoll()
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
		PollAlpaca:    pollFor("DATA_EQUITY_ALPACA_POLL_INTERVAL", def),
		PollFinnhub:   pollFor("DATA_EQUITY_FINNHUB_POLL_INTERVAL", def),
		PollFred:      pollFor("DATA_EQUITY_FRED_POLL_INTERVAL", def),
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
	PollGlassnode time.Duration
	PollEtherscan time.Duration
	GlassnodeKey  string
	EtherscanKey  string
}

func LoadOnchain() (Onchain, error) {
	b := LoadBase()
	if b.DatabaseURL == "" {
		return Onchain{}, fmt.Errorf("DATABASE_URL is required")
	}
	def := defaultPoll()
	return Onchain{
		Base:          b,
		PollGlassnode: pollFor("DATA_ONCHAIN_GLASSNODE_POLL_INTERVAL", def),
		PollEtherscan: pollFor("DATA_ONCHAIN_ETHERSCAN_POLL_INTERVAL", def),
		GlassnodeKey:  os.Getenv("GLASSNODE_API_KEY"),
		EtherscanKey:  os.Getenv("ETHERSCAN_API_KEY"),
	}, nil
}

type Sentiment struct {
	Base
	PollLunarCrush    time.Duration
	PollFinnhubNews   time.Duration
	LunarCrushKey     string
	FinnhubKey        string
	NewsSymbols       []string // crypto symbols for LunarCrush
	EquityNewsSymbols []string // equity tickers for Finnhub company-news
}

func LoadSentiment() (Sentiment, error) {
	b := LoadBase()
	if b.DatabaseURL == "" {
		return Sentiment{}, fmt.Errorf("DATABASE_URL is required")
	}
	def := defaultPoll()
	newsSyms := splitCSV("FINNHUB_SYMBOLS_FOR_NEWS")
	if len(newsSyms) == 0 {
		newsSyms = []string{"BTC", "ETH"}
	}
	equitySyms := splitCSV("FINNHUB_EQUITY_NEWS_SYMBOLS")
	if len(equitySyms) == 0 {
		equitySyms = splitCSV("EQUITY_SYMBOLS")
	}
	return Sentiment{
		Base:              b,
		PollLunarCrush:   pollFor("DATA_SENTIMENT_LUNARCRUSH_POLL_INTERVAL", def),
		PollFinnhubNews:  pollFor("DATA_SENTIMENT_FINNHUB_NEWS_POLL_INTERVAL", def),
		LunarCrushKey:    os.Getenv("LUNARCRUSH_API_KEY"),
		FinnhubKey:       os.Getenv("FINNHUB_API_KEY"),
		NewsSymbols:      newsSyms,
		EquityNewsSymbols: equitySyms,
	}, nil
}
