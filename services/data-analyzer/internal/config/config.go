package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// ── shared helpers ────────────────────────────────────────────────────────────

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

func pollFor(key string, fallback time.Duration) time.Duration {
	return durationEnv(key, fallback)
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

func floatEnv(key string, def float64) float64 {
	s := strings.TrimSpace(os.Getenv(key))
	if s == "" {
		return def
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return def
	}
	return v
}

func boolEnv(key string, def bool) bool {
	s := strings.TrimSpace(os.Getenv(key))
	if s == "" {
		return def
	}
	return s == "true" || s == "1" || s == "yes"
}

func intSliceEnv(key string, def []int) []int {
	parts := splitCSV(key)
	if len(parts) == 0 {
		return def
	}
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		v, err := strconv.Atoi(strings.TrimSpace(p))
		if err == nil && v > 0 {
			out = append(out, v)
		}
	}
	if len(out) == 0 {
		return def
	}
	return out
}

// ── Base ──────────────────────────────────────────────────────────────────────

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

// ── TechnicalAnalysis ─────────────────────────────────────────────────────────

// TechnicalAnalysis holds all configuration for the technical-analysis worker.
// Env-var names are intentionally identical to the old data-technical worker so
// existing .env files work without changes.
//
// TODO: once compute/ is migrated to Python, this struct can be replaced with a
// simple YAML/TOML config file loaded by the Python service.
type TechnicalAnalysis struct {
	Base

	EquitySymbols   []string
	EquityIntervals []string
	CryptoSymbols   []string
	CryptoIntervals []string

	ComputeLookback int
	PollInterval    time.Duration

	SMAPeriods []int
	EMAPeriods []int
	RSIPeriod  int
	VolSMAPeriod int

	SRSwingStrength int
	SRLevels        int
	SRClusterPct    float64

	TrendLookback int

	MACDFast   int
	MACDSlow   int
	MACDSignal int

	BBPeriod int
	BBStd    float64

	FibSwingStrength    int
	FibExtensions       bool
	RSIDivSwingStrength int

	VolProfileBins         int
	VolProfileTypical      bool
	VolProfileValueAreaPct float64

	RSIHiddenMinPivotSep  int
	RSIHiddenRequireTrend bool

	StochKPeriod    int
	StochDSmooth    int
	StochDSignal    int
	ATRPeriod       int
	ADXPeriod       int
	WilliamsRPeriod int

	IchimokuTenkan   int
	IchimokuKijun    int
	IchimokuSpanB    int
	IchimokuDisplace int

	VWAPMode       string
	VWAPRollingN   int
	VWAPUseTypical bool

	RibbonPeriods          []int
	MACrossFast            int
	MACrossSlow            int
	ChartPatternClusterPct float64

	CMFPeriod        int
	KeltnerEMAPeriod int
	KeltnerATRPeriod int
	KeltnerMult      float64
	DonchianPeriod   int
	TrendlinePivots  int
	CCIPeriod        int
	ROCPeriod        int
	ParabolicStep    float64
	ParabolicMaxAF   float64
	MFIPeriod        int
	GannLookback     int

	RSBenchmarkEquity     string
	RSBenchmarkCrypto     string
	RSBenchmarkMinAligned int

	MTFEquitySecondary []string
	MTFCryptoSecondary []string

	// Feature toggles.
	EnableMA              bool
	EnableRSI             bool
	EnableVolume          bool
	EnableSR              bool
	EnableTrend           bool
	EnableCandles         bool
	EnableMACD            bool
	EnableOBV             bool
	EnableBollinger       bool
	EnableFib             bool
	EnableRSIDivergence   bool
	EnableVolProfileProxy bool
	EnableRSIHidden       bool
	EnableStochastic      bool
	EnableATR             bool
	EnableIchimoku        bool
	EnableADLine          bool
	EnableADX             bool
	EnablePivots          bool
	EnableWilliamsR       bool
	EnableVWAP            bool
	EnableMARibbon        bool
	EnableChartPatterns   bool
	EnableCMF             bool
	EnableKeltner         bool
	EnableDonchian        bool
	EnableTrendlineBreak  bool
	EnableCCI             bool
	EnableROC             bool
	EnableParabolicSAR    bool
	EnableMFI             bool
	EnableMarketStructure bool
	EnableElliottHint     bool
	EnableGannHint        bool
	EnableOpenInterestInfo bool
	EnableRSBenchmark     bool
	EnableMTFConfluence   bool
}

func LoadTechnicalAnalysis() (TechnicalAnalysis, error) {
	b := LoadBase()
	if b.DatabaseURL == "" {
		return TechnicalAnalysis{}, fmt.Errorf("DATABASE_URL is required")
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

	return TechnicalAnalysis{
		Base:            b,
		EquitySymbols:   equitySyms,
		EquityIntervals: equityIvs,
		CryptoSymbols:   cryptoSyms,
		CryptoIntervals: cryptoIvs,
		ComputeLookback: intEnv("TECHNICAL_COMPUTE_LOOKBACK", 500),
		PollInterval:    pollFor("DATA_TECHNICAL_POLL_INTERVAL", 6*time.Hour),

		SMAPeriods:   intSliceEnv("TECHNICAL_SMA_PERIODS", []int{20, 50, 100, 200}),
		EMAPeriods:   intSliceEnv("TECHNICAL_EMA_PERIODS", []int{9, 21, 50, 200}),
		RSIPeriod:    intEnv("TECHNICAL_RSI_PERIOD", 14),
		VolSMAPeriod: intEnv("TECHNICAL_VOL_SMA_PERIOD", 20),

		SRSwingStrength: intEnv("TECHNICAL_SR_SWING_STRENGTH", 5),
		SRLevels:        intEnv("TECHNICAL_SR_LEVELS", 3),
		SRClusterPct:    floatEnv("TECHNICAL_SR_CLUSTER_PCT", 0.5),
		TrendLookback:   intEnv("TECHNICAL_TREND_LOOKBACK", 60),

		MACDFast:   intEnv("TECHNICAL_MACD_FAST", 12),
		MACDSlow:   intEnv("TECHNICAL_MACD_SLOW", 26),
		MACDSignal: intEnv("TECHNICAL_MACD_SIGNAL", 9),

		BBPeriod:            intEnv("TECHNICAL_BB_PERIOD", 20),
		BBStd:               floatEnv("TECHNICAL_BB_STD", 2),
		FibSwingStrength:    intEnv("TECHNICAL_FIB_SWING_STRENGTH", 0),
		FibExtensions:       boolEnv("TECHNICAL_FIB_EXTENSIONS", true),
		RSIDivSwingStrength: intEnv("TECHNICAL_RSI_DIV_SWING_STRENGTH", 0),

		VolProfileBins:         intEnv("TECHNICAL_VOL_PROFILE_BINS", 48),
		VolProfileTypical:      boolEnv("TECHNICAL_VOL_PROFILE_TYPICAL", true),
		VolProfileValueAreaPct: floatEnv("TECHNICAL_VOL_PROFILE_VALUE_AREA_PCT", 0.70),

		RSIHiddenMinPivotSep:  intEnv("TECHNICAL_RSI_HIDDEN_MIN_PIVOT_SEP", 3),
		RSIHiddenRequireTrend: boolEnv("TECHNICAL_RSI_HIDDEN_REQUIRE_TREND", true),

		StochKPeriod:    intEnv("TECHNICAL_STOCH_K", 14),
		StochDSmooth:    intEnv("TECHNICAL_STOCH_D_SMOOTH", 3),
		StochDSignal:    intEnv("TECHNICAL_STOCH_D_SIGNAL", 3),
		ATRPeriod:       intEnv("TECHNICAL_ATR_PERIOD", 14),
		ADXPeriod:       intEnv("TECHNICAL_ADX_PERIOD", 14),
		WilliamsRPeriod: intEnv("TECHNICAL_WILLIAMS_R_PERIOD", 14),

		IchimokuTenkan:   intEnv("TECHNICAL_ICHIMOKU_TENKAN", 9),
		IchimokuKijun:    intEnv("TECHNICAL_ICHIMOKU_KIJUN", 26),
		IchimokuSpanB:    intEnv("TECHNICAL_ICHIMOKU_SPAN_B", 52),
		IchimokuDisplace: intEnv("TECHNICAL_ICHIMOKU_DISPLACE", 0),

		VWAPMode:       env("TECHNICAL_VWAP_MODE", "rolling"),
		VWAPRollingN:   intEnv("TECHNICAL_VWAP_ROLLING_N", 20),
		VWAPUseTypical: boolEnv("TECHNICAL_VWAP_USE_TYPICAL", true),

		RibbonPeriods:          intSliceEnv("TECHNICAL_RIBBON_PERIODS", []int{10, 20, 50, 200}),
		MACrossFast:            intEnv("TECHNICAL_MA_CROSS_FAST", 50),
		MACrossSlow:            intEnv("TECHNICAL_MA_CROSS_SLOW", 200),
		ChartPatternClusterPct: floatEnv("TECHNICAL_CHART_PATTERN_CLUSTER_PCT", 0.5),

		CMFPeriod:        intEnv("TECHNICAL_CMF_PERIOD", 21),
		KeltnerEMAPeriod: intEnv("TECHNICAL_KELTNER_EMA", 20),
		KeltnerATRPeriod: intEnv("TECHNICAL_KELTNER_ATR", 10),
		KeltnerMult:      floatEnv("TECHNICAL_KELTNER_MULT", 2),
		DonchianPeriod:   intEnv("TECHNICAL_DONCHIAN_PERIOD", 20),
		TrendlinePivots:  intEnv("TECHNICAL_TRENDLINE_PIVOTS", 3),
		CCIPeriod:        intEnv("TECHNICAL_CCI_PERIOD", 20),
		ROCPeriod:        intEnv("TECHNICAL_ROC_PERIOD", 12),
		ParabolicStep:    floatEnv("TECHNICAL_PARABOLIC_STEP", 0.02),
		ParabolicMaxAF:   floatEnv("TECHNICAL_PARABOLIC_MAX_AF", 0.2),
		MFIPeriod:        intEnv("TECHNICAL_MFI_PERIOD", 14),
		GannLookback:     intEnv("TECHNICAL_GANN_LOOKBACK", 60),

		RSBenchmarkEquity:     strings.TrimSpace(os.Getenv("TECHNICAL_RS_BENCHMARK_EQUITY")),
		RSBenchmarkCrypto:     strings.TrimSpace(os.Getenv("TECHNICAL_RS_BENCHMARK_CRYPTO")),
		RSBenchmarkMinAligned: intEnv("TECHNICAL_RS_MIN_ALIGNED_BARS", 30),

		MTFEquitySecondary: splitCSV("TECHNICAL_MTF_EQUITY_INTERVALS"),
		MTFCryptoSecondary: splitCSV("TECHNICAL_MTF_CRYPTO_INTERVALS"),

		EnableMA:              boolEnv("TECHNICAL_ENABLE_MA", true),
		EnableRSI:             boolEnv("TECHNICAL_ENABLE_RSI", true),
		EnableVolume:          boolEnv("TECHNICAL_ENABLE_VOLUME", true),
		EnableSR:              boolEnv("TECHNICAL_ENABLE_SR", true),
		EnableTrend:           boolEnv("TECHNICAL_ENABLE_TREND", true),
		EnableCandles:         boolEnv("TECHNICAL_ENABLE_CANDLES", true),
		EnableMACD:            boolEnv("TECHNICAL_ENABLE_MACD", true),
		EnableOBV:             boolEnv("TECHNICAL_ENABLE_OBV", true),
		EnableBollinger:       boolEnv("TECHNICAL_ENABLE_BOLLINGER", true),
		EnableFib:             boolEnv("TECHNICAL_ENABLE_FIB", true),
		EnableRSIDivergence:   boolEnv("TECHNICAL_ENABLE_RSI_DIVERGENCE", true),
		EnableVolProfileProxy: boolEnv("TECHNICAL_ENABLE_VOL_PROFILE_PROXY", true),
		EnableRSIHidden:       boolEnv("TECHNICAL_ENABLE_RSI_HIDDEN", true),
		EnableStochastic:      boolEnv("TECHNICAL_ENABLE_STOCHASTIC", true),
		EnableATR:             boolEnv("TECHNICAL_ENABLE_ATR", true),
		EnableIchimoku:        boolEnv("TECHNICAL_ENABLE_ICHIMOKU", true),
		EnableADLine:          boolEnv("TECHNICAL_ENABLE_AD_LINE", true),
		EnableADX:             boolEnv("TECHNICAL_ENABLE_ADX", true),
		EnablePivots:          boolEnv("TECHNICAL_ENABLE_PIVOTS", true),
		EnableWilliamsR:       boolEnv("TECHNICAL_ENABLE_WILLIAMS_R", true),
		EnableVWAP:            boolEnv("TECHNICAL_ENABLE_VWAP", true),
		EnableMARibbon:        boolEnv("TECHNICAL_ENABLE_MA_RIBBON", true),
		EnableChartPatterns:   boolEnv("TECHNICAL_ENABLE_CHART_PATTERNS", true),
		EnableCMF:             boolEnv("TECHNICAL_ENABLE_CMF", true),
		EnableKeltner:         boolEnv("TECHNICAL_ENABLE_KELTNER", true),
		EnableDonchian:        boolEnv("TECHNICAL_ENABLE_DONCHIAN", true),
		EnableTrendlineBreak:  boolEnv("TECHNICAL_ENABLE_TRENDLINE_BREAK", true),
		EnableCCI:             boolEnv("TECHNICAL_ENABLE_CCI", true),
		EnableROC:             boolEnv("TECHNICAL_ENABLE_ROC", true),
		EnableParabolicSAR:    boolEnv("TECHNICAL_ENABLE_PARABOLIC_SAR", true),
		EnableMFI:             boolEnv("TECHNICAL_ENABLE_MFI", true),
		EnableMarketStructure: boolEnv("TECHNICAL_ENABLE_MARKET_STRUCTURE", true),
		EnableElliottHint:     boolEnv("TECHNICAL_ENABLE_ELLIOTT_HINT", true),
		EnableGannHint:        boolEnv("TECHNICAL_ENABLE_GANN_HINT", true),
		EnableOpenInterestInfo: boolEnv("TECHNICAL_ENABLE_OPEN_INTEREST_INFO", false),
		EnableRSBenchmark:     boolEnv("TECHNICAL_ENABLE_RS_BENCHMARK", false),
		EnableMTFConfluence:   boolEnv("TECHNICAL_ENABLE_MTF_CONFLUENCE", false),
	}, nil
}

// FibSwing returns pivot strength for Fibonacci (falls back to SR strength).
func (t TechnicalAnalysis) FibSwing() int {
	if t.FibSwingStrength > 0 {
		return t.FibSwingStrength
	}
	return t.SRSwingStrength
}

// RSIDivSwing returns pivot strength for RSI divergence (falls back to SR strength).
func (t TechnicalAnalysis) RSIDivSwing() int {
	if t.RSIDivSwingStrength > 0 {
		return t.RSIDivSwingStrength
	}
	return t.SRSwingStrength
}

// ── FundamentalAnalysis ───────────────────────────────────────────────────────

// FundamentalAnalysis holds configuration for the fundamental-analysis worker.
//
// TODO: migrate fundamental scoring logic to Python. The asyncpg query and upsert
// structure will be simpler — one pandas DataFrame per symbol, vectorised ratio math.
type FundamentalAnalysis struct {
	Base
	Symbols      []string
	PollInterval time.Duration

	// Minimum number of raw metrics required before scoring a symbol.
	MinMetrics int
}

func LoadFundamentalAnalysis() (FundamentalAnalysis, error) {
	b := LoadBase()
	if b.DatabaseURL == "" {
		return FundamentalAnalysis{}, fmt.Errorf("DATABASE_URL is required")
	}
	syms := splitCSV("FUNDAMENTAL_SYMBOLS")
	if len(syms) == 0 {
		syms = splitCSV("ALPACA_DATA_SYMBOLS")
	}
	if len(syms) == 0 {
		syms = []string{"AAPL", "MSFT", "SPY"}
	}
	return FundamentalAnalysis{
		Base:         b,
		Symbols:      syms,
		PollInterval: pollFor("DATA_FUNDAMENTAL_ANALYSIS_POLL_INTERVAL", 24*time.Hour),
		MinMetrics:   intEnv("FUNDAMENTAL_ANALYSIS_MIN_METRICS", 5),
	}, nil
}
