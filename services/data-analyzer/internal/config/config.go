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

func strEnvDefault(key, def string) string {
	s := strings.TrimSpace(os.Getenv(key))
	if s == "" {
		return def
	}
	return s
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

	// ── SMC: Order Blocks ────────────────────────────────────────────────────
	OBSwingStrength int
	OBImpulseMinPct float64
	OBLookback      int

	// ── SMC: Fair Value Gaps ─────────────────────────────────────────────────
	FVGMinGapPct float64
	FVGLookback  int

	// ── SMC: Liquidity Sweeps ────────────────────────────────────────────────
	LiquiditySwingStrength int
	LiquidityLookback      int

	// ── VIX Regime (reads VIXCLS from macro_fred) ────────────────────────────
	VIXFearThreshold        float64 // VIX > this → extreme fear
	VIXElevatedThreshold    float64 // VIX > this → elevated anxiety
	VIXComplacencyThreshold float64 // VIX < this → complacency

	// ── Multi-TF Pivot Points ─────────────────────────────────────────────────
	WeeklyPivotEquityInterval  string // e.g. "1Week"
	WeeklyPivotCryptoInterval  string // e.g. "1w"
	MonthlyPivotEquityInterval string // e.g. "1Month"
	MonthlyPivotCryptoInterval string // e.g. "1M"
	WeeklyPivotLookback        int    // how many weekly bars to query  default 10
	MonthlyPivotLookback       int    // how many monthly bars to query default 5

	// ── Candlestick Patterns ───────────────────────────────────────────────────
	CandleWindow int // last N bars to scan for candle patterns  default 3

	// ── Head & Shoulders ─────────────────────────────────────────────────────
	HSSwingStrength  int
	HSTolerancePct   float64
	HSLookback       int

	// ── Triangle Patterns ────────────────────────────────────────────────────
	TriangleSwingStrength   int
	TriangleMinPivots       int
	TriangleFlatThresholdPct float64
	TriangleLookback        int

	// ── Flag / Pennant ───────────────────────────────────────────────────────
	FlagPolePct          float64
	FlagMaxRetracePct    float64
	FlagPoleLen          int
	FlagLen              int

	// Feature toggles.
	EnableMA               bool
	EnableRSI              bool
	EnableVolume           bool
	EnableSR               bool
	EnableTrend            bool
	EnableCandles          bool
	EnableMACD             bool
	EnableOBV              bool
	EnableBollinger        bool
	EnableFib              bool
	EnableRSIDivergence    bool
	EnableVolProfileProxy  bool
	EnableRSIHidden        bool
	EnableStochastic       bool
	EnableATR              bool
	EnableIchimoku         bool
	EnableADLine           bool
	EnableADX              bool
	EnablePivots           bool
	EnableWilliamsR        bool
	EnableVWAP             bool
	EnableMARibbon         bool
	EnableChartPatterns    bool
	EnableCMF              bool
	EnableKeltner          bool
	EnableDonchian         bool
	EnableTrendlineBreak   bool
	EnableCCI              bool
	EnableROC              bool
	EnableParabolicSAR     bool
	EnableMFI              bool
	EnableMarketStructure  bool
	EnableElliottHint      bool
	EnableGannHint         bool
	EnableOpenInterestInfo bool
	EnableRSBenchmark      bool
	EnableMTFConfluence    bool
	// New SMC / pattern toggles.
	EnableOrderBlocks     bool
	EnableFVG             bool
	EnableLiquiditySweep  bool
	EnableBBSqueeze       bool
	EnableVIXRegime       bool
	EnableWeeklyPivots    bool
	EnableMonthlyPivots   bool
	EnableHSPattern       bool
	EnableTriangle        bool
	EnableFlag            bool
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

		// SMC: Order Blocks
		OBSwingStrength: intEnv("TECHNICAL_OB_SWING_STRENGTH", 3),
		OBImpulseMinPct: floatEnv("TECHNICAL_OB_IMPULSE_MIN_PCT", 1.5),
		OBLookback:      intEnv("TECHNICAL_OB_LOOKBACK", 100),

		// SMC: Fair Value Gaps
		FVGMinGapPct: floatEnv("TECHNICAL_FVG_MIN_GAP_PCT", 0.1),
		FVGLookback:  intEnv("TECHNICAL_FVG_LOOKBACK", 50),

		// SMC: Liquidity Sweeps
		LiquiditySwingStrength: intEnv("TECHNICAL_LIQUIDITY_SWING_STRENGTH", 3),
		LiquidityLookback:      intEnv("TECHNICAL_LIQUIDITY_LOOKBACK", 50),

		// VIX Regime
		VIXFearThreshold:        floatEnv("TECHNICAL_VIX_FEAR_THRESHOLD", 35),
		VIXElevatedThreshold:    floatEnv("TECHNICAL_VIX_ELEVATED_THRESHOLD", 20),
		VIXComplacencyThreshold: floatEnv("TECHNICAL_VIX_COMPLACENCY_THRESHOLD", 12),

		// Multi-TF Pivots
		WeeklyPivotEquityInterval:  env("TECHNICAL_WEEKLY_PIVOT_EQUITY_INTERVAL", "1Week"),
		WeeklyPivotCryptoInterval:  env("TECHNICAL_WEEKLY_PIVOT_CRYPTO_INTERVAL", "1w"),
		MonthlyPivotEquityInterval: env("TECHNICAL_MONTHLY_PIVOT_EQUITY_INTERVAL", "1Month"),
		MonthlyPivotCryptoInterval: env("TECHNICAL_MONTHLY_PIVOT_CRYPTO_INTERVAL", "1M"),
		WeeklyPivotLookback:        intEnv("TECHNICAL_WEEKLY_PIVOT_LOOKBACK", 10),
		MonthlyPivotLookback:       intEnv("TECHNICAL_MONTHLY_PIVOT_LOOKBACK", 5),

		CandleWindow: intEnv("TECHNICAL_CANDLE_WINDOW", 3),

		// Head & Shoulders
		HSSwingStrength: intEnv("TECHNICAL_HS_SWING_STRENGTH", 5),
		HSTolerancePct:  floatEnv("TECHNICAL_HS_TOLERANCE_PCT", 15.0),
		HSLookback:      intEnv("TECHNICAL_HS_LOOKBACK", 100),

		// Triangle Patterns
		TriangleSwingStrength:    intEnv("TECHNICAL_TRIANGLE_SWING_STRENGTH", 3),
		TriangleMinPivots:        intEnv("TECHNICAL_TRIANGLE_MIN_PIVOTS", 3),
		TriangleFlatThresholdPct: floatEnv("TECHNICAL_TRIANGLE_FLAT_THRESHOLD_PCT", 0.05),
		TriangleLookback:         intEnv("TECHNICAL_TRIANGLE_LOOKBACK", 100),

		// Flag / Pennant
		FlagPolePct:       floatEnv("TECHNICAL_FLAG_POLE_PCT", 5.0),
		FlagMaxRetracePct: floatEnv("TECHNICAL_FLAG_MAX_RETRACEMENT_PCT", 50.0),
		FlagPoleLen:       intEnv("TECHNICAL_FLAG_POLE_LEN", 5),
		FlagLen:           intEnv("TECHNICAL_FLAG_LEN", 10),

		EnableMA:               boolEnv("TECHNICAL_ENABLE_MA", true),
		EnableRSI:              boolEnv("TECHNICAL_ENABLE_RSI", true),
		EnableVolume:           boolEnv("TECHNICAL_ENABLE_VOLUME", true),
		EnableSR:               boolEnv("TECHNICAL_ENABLE_SR", true),
		EnableTrend:            boolEnv("TECHNICAL_ENABLE_TREND", true),
		EnableCandles:          boolEnv("TECHNICAL_ENABLE_CANDLES", true),
		EnableMACD:             boolEnv("TECHNICAL_ENABLE_MACD", true),
		EnableOBV:              boolEnv("TECHNICAL_ENABLE_OBV", true),
		EnableBollinger:        boolEnv("TECHNICAL_ENABLE_BOLLINGER", true),
		EnableFib:              boolEnv("TECHNICAL_ENABLE_FIB", true),
		EnableRSIDivergence:    boolEnv("TECHNICAL_ENABLE_RSI_DIVERGENCE", true),
		EnableVolProfileProxy:  boolEnv("TECHNICAL_ENABLE_VOL_PROFILE_PROXY", true),
		EnableRSIHidden:        boolEnv("TECHNICAL_ENABLE_RSI_HIDDEN", true),
		EnableStochastic:       boolEnv("TECHNICAL_ENABLE_STOCHASTIC", true),
		EnableATR:              boolEnv("TECHNICAL_ENABLE_ATR", true),
		EnableIchimoku:         boolEnv("TECHNICAL_ENABLE_ICHIMOKU", true),
		EnableADLine:           boolEnv("TECHNICAL_ENABLE_AD_LINE", true),
		EnableADX:              boolEnv("TECHNICAL_ENABLE_ADX", true),
		EnablePivots:           boolEnv("TECHNICAL_ENABLE_PIVOTS", true),
		EnableWilliamsR:        boolEnv("TECHNICAL_ENABLE_WILLIAMS_R", true),
		EnableVWAP:             boolEnv("TECHNICAL_ENABLE_VWAP", true),
		EnableMARibbon:         boolEnv("TECHNICAL_ENABLE_MA_RIBBON", true),
		EnableChartPatterns:    boolEnv("TECHNICAL_ENABLE_CHART_PATTERNS", true),
		EnableCMF:              boolEnv("TECHNICAL_ENABLE_CMF", true),
		EnableKeltner:          boolEnv("TECHNICAL_ENABLE_KELTNER", true),
		EnableDonchian:         boolEnv("TECHNICAL_ENABLE_DONCHIAN", true),
		EnableTrendlineBreak:   boolEnv("TECHNICAL_ENABLE_TRENDLINE_BREAK", true),
		EnableCCI:              boolEnv("TECHNICAL_ENABLE_CCI", true),
		EnableROC:              boolEnv("TECHNICAL_ENABLE_ROC", true),
		EnableParabolicSAR:     boolEnv("TECHNICAL_ENABLE_PARABOLIC_SAR", true),
		EnableMFI:              boolEnv("TECHNICAL_ENABLE_MFI", true),
		EnableMarketStructure:  boolEnv("TECHNICAL_ENABLE_MARKET_STRUCTURE", true),
		EnableElliottHint:      boolEnv("TECHNICAL_ENABLE_ELLIOTT_HINT", true),
		EnableGannHint:         boolEnv("TECHNICAL_ENABLE_GANN_HINT", true),
		EnableOpenInterestInfo: boolEnv("TECHNICAL_ENABLE_OPEN_INTEREST_INFO", false),
		EnableRSBenchmark:      boolEnv("TECHNICAL_ENABLE_RS_BENCHMARK", false),
		EnableMTFConfluence:    boolEnv("TECHNICAL_ENABLE_MTF_CONFLUENCE", false),
		// New SMC / pattern toggles.
		EnableOrderBlocks:    boolEnv("TECHNICAL_ENABLE_ORDER_BLOCKS", true),
		EnableFVG:            boolEnv("TECHNICAL_ENABLE_FVG", true),
		EnableLiquiditySweep: boolEnv("TECHNICAL_ENABLE_LIQUIDITY_SWEEP", true),
		EnableBBSqueeze:      boolEnv("TECHNICAL_ENABLE_BB_SQUEEZE", true),
		EnableVIXRegime:      boolEnv("TECHNICAL_ENABLE_VIX_REGIME", true),
		EnableWeeklyPivots:   boolEnv("TECHNICAL_ENABLE_WEEKLY_PIVOTS", true),
		EnableMonthlyPivots:  boolEnv("TECHNICAL_ENABLE_MONTHLY_PIVOTS", false),
		EnableHSPattern:      boolEnv("TECHNICAL_ENABLE_HS_PATTERN", true),
		EnableTriangle:       boolEnv("TECHNICAL_ENABLE_TRIANGLE", true),
		EnableFlag:           boolEnv("TECHNICAL_ENABLE_FLAG", true),
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
// All classification thresholds are exposed as env vars so the algorithm can be
// fine-tuned per market regime, sector, or from a future web-app control panel
// without recompiling the binary.
//
// TODO: migrate fundamental scoring logic to Python. The asyncpg query and upsert
// structure will be simpler — one pandas DataFrame per symbol, vectorised ratio math.
type FundamentalAnalysis struct {
	Base
	Symbols      []string
	PollInterval time.Duration

	// Minimum number of raw metrics required before scoring a symbol.
	MinMetrics int

	// ── EPS growth thresholds (%) ─────────────────────────────────────────────
	EPSGrowthStrong float64 // >this = "strong"   default 15
	EPSGrowthWeak   float64 // <this = "weak"      default 5

	// ── Revenue growth thresholds (%) ────────────────────────────────────────
	RevGrowthStrong float64 // >this = "strong"   default 10
	RevGrowthWeak   float64 // <this = "weak"      default 2

	// ── P/E vs own 5-year mean (% deviation) ─────────────────────────────────
	PEVs5YCheapPct float64 // below 5Y mean by this % = cheap   default 15
	PEVs5YExpPct   float64 // above 5Y mean by this % = expensive default 15

	// ── P/E absolute fallback (when no 5Y history) ───────────────────────────
	PEAbsValue  float64 // <this = "value"        default 15
	PEAbsGrowth float64 // <this = "growth_fair"  default 25

	// ── FCF yield thresholds (%) ──────────────────────────────────────────────
	FCFYieldAttractive float64 // >=this = attractive   default 5
	FCFYieldFair       float64 // >=this = fair          default 2

	// ── FCF/EPS divergence detection ─────────────────────────────────────────
	FCFDivEPSGrowth float64 // EPS growth threshold for divergence check   default 10
	FCFDivYieldLow  float64 // FCF yield below this = suspect earnings      default 2
	FCFDivYieldHigh float64 // FCF yield above this = high quality earnings default 5

	// ── Gross margin tiers (%) ────────────────────────────────────────────────
	GrossMarginMoat float64 // >=this = "strong_moat"  default 40
	GrossMarginAvg  float64 // >=this = "average"       default 20

	// ── Net margin tiers (%) ─────────────────────────────────────────────────
	NetMarginStrong float64 // >=this = "strong"    default 15
	NetMarginAvg    float64 // >=this = "average"   default 5

	// ── Forward P/E compression flat band (%) ────────────────────────────────
	PECompressionFlat float64 // within ±this % = "flat"  default 5

	// ── PEG ratio tiers ───────────────────────────────────────────────────────
	PEGUndervalued float64 // <this = undervalued  default 1
	PEGFair        float64 // <this = fair          default 2

	// ── Earnings surprise ────────────────────────────────────────────────────
	SurpriseBeatPct  float64 // avg surprise >= this = "beat"  default 2
	SurpriseMissPct  float64 // avg surprise <= -this = "miss" default 2
	SurpriseQuarters int     // max quarters to average         default 4

	// ── Composite score boundaries ───────────────────────────────────────────
	CompositeStrong float64 // >=this = "strong"  default 0.5
	CompositeWeak   float64 // <=-this = "weak"   default 0.5

	// ── Margin trend (percentage-point change, 8-quarter window) ─────────────
	MarginTrendStablePP float64 // within ±this pp = "stable"  default 2
	MarginTrendQuarters int     // quarters to analyse          default 8

	// ── Tier 2: ROE (Return on Equity) ───────────────────────────────────────
	// Sustained ROE >15% for 5+ years = moat. <8% = capital destruction.
	ROEExcellent float64 // >this = "excellent"  default 15
	ROEAdequate  float64 // >this = "adequate"   default 8

	// ── Tier 2: ROIC (Return on Invested Capital) ──────────────────────────
	// Finnhub provides 5-year average ROIC (roic5Y). ROIC >15% = durable moat.
	// Composite-scored alongside ROE to avoid double-weighting.
	ROICExcellent float64 // >this = "moat_quality"  default 15
	ROICAdequate  float64 // >this = "adequate_roic"  default 8

	// ── Tier 2: Debt-to-Equity leverage ──────────────────────────────────────
	// D/E <1 conservative, 1-2 manageable (monitor), >2 high risk in rate rises.
	DEConservative float64 // <this = "conservative"  default 1.0
	DEManageable   float64 // <this = "manageable"    default 2.0

	// ── Tier 2: Net Debt / EBITDA ─────────────────────────────────────────────
	// <2× conservative, 2–4× manageable, >4× high risk.
	NetDebtEBITDALow  float64 // <this = "conservative"  default 2.0
	NetDebtEBITDAHigh float64 // <this = "manageable"    default 4.0

	// ── Tier 2: EV/EBITDA (rank 08) ──────────────────────────────────────────
	// <10× value, 10–20× fair, >20× requires strong growth justification.
	EVEBITDAValue float64 // <this = "value"  default 10
	EVEBITDAFair  float64 // <this = "fair"   default 20

	// ── Tier 2: Current Ratio (rank 10) ──────────────────────────────────────
	// >1.5 safe, 1.0–1.5 monitor, <1.0 liquidity risk.
	CurrentRatioSafe    float64 // >this = "safe"     default 1.5
	CurrentRatioMonitor float64 // >this = "monitor"  default 1.0

	// ── Tier 2: Price/Book (P/B) ─────────────────────────────────────────────
	// <1.5 value (asset-heavy sectors), >5 limited margin of safety.
	PBValue     float64 // <this = "value"     default 1.5
	PBExpensive float64 // >this = "expensive" default 5.0

	// ── Tier 2: Dividend sustainability ──────────────────────────────────────
	// 3–6% yield + <60% payout = ideal sustainable income.
	DividendYieldMin   float64 // below this = "none/minimal"     default 2
	DividendYieldHigh  float64 // above this = verify payout      default 6
	PayoutRatioSafe    float64 // <this = sustainable             default 60
	PayoutRatioDanger  float64 // >this = cut risk                default 80

	// ── Tier 2: CapEx intensity ───────────────────────────────────────────────
	// <5% asset-light, 5–15% moderate, >20% capital-intensive / FCF constrained.
	CapExIntensityLow  float64 // <this = "asset_light"          default 5
	CapExIntensityHigh float64 // >this = "capital_intensive"    default 20

	// ── Tier 3: Share Count Trend — buybacks vs dilution (rank 13) ───────────
	// Declining >2%/yr = buyback program (bullish), growing >3%/yr = dilution risk.
	ShareDeclineBuyback float64 // annual decline % to qualify as buyback   default 2
	ShareGrowthDilution float64 // annual growth % to qualify as dilution   default 3
	ShareTrendYears     int     // years of history window to sample        default 2

	// ── Tier 3: DCF Intrinsic Value (simplified 5-year model) (rank 14) ──────
	// Simplified model: explicit FCF growth stage + perpetuity terminal value.
	// Use as a directional sanity check, not a precise number.
	// TODO: Python — more sophisticated inputs (sector WACC, analyst forecasts, scenario ranges).
	DCFWACCPct        float64 // discount rate / WACC                default 10
	DCFTerminalGrowth float64 // perpetual terminal growth rate (%)   default 3
	DCFGrowthYears    int     // explicit FCF growth stage years       default 5
	DCFMaxGrowthPct   float64 // cap on assumed FCF growth rate        default 20
	DCFSafetyMargin   float64 // price as % of DCF: <this = strong buy default 70
	DCFOvervalued     float64 // price as % of DCF: >this = overvalued default 110

	// ── Tier 3: Interest Coverage Ratio (rank 15) ─────────────────────────────
	// Interest Coverage = EBIT ÷ Interest Expense.
	// <2× in a rising rate environment is high risk.
	InterestCoverageSafe     float64 // >this = "very_safe"     default 5
	InterestCoverageAdequate float64 // >this = "adequate"      default 2

	// ── Tier 3: Goodwill & Intangibles % of Total Assets (rank 18) ───────────
	// Goodwill impairment is non-cash but signals an acquisition that failed.
	GoodwillLowPct  float64 // <this% = "low_risk"      default 20
	GoodwillHighPct float64 // >this% = "high_risk"     default 40

	// ── Tier 3: Price-to-Sales (P/S) Ratio (rank 19) ─────────────────────────
	// Most useful for unprofitable or early-stage growth companies.
	// Compare within sector — SaaS companies carry higher P/S than industrials.
	PSValue       float64 // <this = "value"        default 5
	PSFair        float64 // <this = "growth_fair"  default 10
	PSSpeculative float64 // >this = "speculative"  default 15

	// ── Tier 3: Analyst Target Price (rank 17) ────────────────────────────────
	// Source: Alpha Vantage AnalystTargetPrice (stored as analyst_target_price).
	// Upside = (target - price) / price × 100.
	AnalystUpsideBullish  float64 // upside % above this = bullish consensus  default 15
	AnalystDownsideBearish float64 // upside % below this = bearish consensus default -5

	// ── Tier 3: Analyst Recommendation Trend (rank 17 extended) ─────────────
	// Computed from Finnhub /stock/recommendation month-over-month delta.
	// net_buy_change > AnalystRecUpgrade = analysts upgrading consensus.
	// net_buy_change < AnalystRecDowngrade = analysts downgrading.
	AnalystRecUpgrade   float64 // delta above this = "upgrading"    default 5
	AnalystRecDowngrade float64 // delta below this = "downgrading"   default -5

	// ── Tier 3: FCF Conversion Rate ───────────────────────────────────────────
	// FCF Conversion = FCF / Net Income. >1.0 = cash-backed earnings (high quality).
	// <0.7 = aggressive accruals or large working-capital drag.
	FCFConversionHigh float64 // >=this = "high_quality_cash"  default 1.0
	FCFConversionLow  float64 // >=this = "moderate"           default 0.7

	// ── Equity interval for Tier 3 live-price lookup ─────────────────────────
	// Used when scoreTier3 queries the latest close from equity_ohlcv.
	EquityInterval string // e.g. "1Day"

	// ── Qualitative scoring ───────────────────────────────────────────────────
	// Insider cluster detection window and minimum unique-buyer threshold.
	// cluster_buy is triggered when ≥ InsiderClusterMinBuyers distinct insiders
	// submit Form 4 purchase filings within InsiderClusterWindowDays.
	InsiderClusterWindowDays int     // default 90
	InsiderClusterMinBuyers  int     // default 3

	// Number of quarterly gross-margin data points used to compute the moat
	// stability score (std dev of quarterly gross margin).
	QualMoatStabilityQuarters int // default 8

	// News sentiment thresholds. Sentiment scores are numeric Alpha Vantage values
	// in the range -1.0 to +1.0 (0 = neutral).
	QualSentimentPositive float64 // AVG sentiment > this → "positive"  default 0.15
	QualSentimentNegative float64 // AVG sentiment < this → "negative"  default -0.15

	// R&D intensity thresholds as % of revenue.
	// Sector context: tech typically 10–20%; pharma 15–25%; industrials 2–5%.
	QualRDHealthyPct  float64 // >= this → "investing_in_future"  default 10
	QualRDModeratePct float64 // >= this → "moderate"             default 3

	// Gross-margin standard deviation threshold (percentage points) for moat scoring.
	// Low std dev = stable pricing power.
	QualMoatStableStdPP float64 // std-dev < this → considered stable  default 5

	// ── Correlation analysis thresholds ───────────────────────────────────────

	// Minimum conditions met (out of 5) for the "bullish convergence" master
	// signal to fire. Higher = more conservative / fewer false positives.
	// Configurable via CORR_BULLISH_CONVERGENCE_MIN_CONDITIONS (default 3).
	CorrBullishConvergenceMin int

	// Minimum conditions met (out of 4) for the "leverage cycle warning" master
	// signal to fire. Configurable via CORR_LEVERAGE_CYCLE_MIN_CONDITIONS (default 3).
	CorrLeverageCycleMin int

	// Minimum conditions met (out of 4) for the "value trap" master signal.
	// Configurable via CORR_VALUE_TRAP_MIN_CONDITIONS (default 3).
	CorrValueTrapMin int

	// Receivables / revenue growth ratio threshold for the deterioration warning.
	// If accounts_receivable growth rate > revenue growth rate × this multiplier,
	// receivables are considered "growing faster than revenue".
	// Configurable via CORR_RECEIVABLES_GROWTH_MULTIPLIER (default 1.1).
	CorrReceivablesGrowthMultiplier float64
}

// ── MacroAnalysis ─────────────────────────────────────────────────────────────

// MacroAnalysis holds all thresholds and toggles for the macro-analysis worker.
// Every value is configurable via .env so the algorithm can be adapted to
// different rate regimes without recompiling.
//
// TODO: migrate to Python once the LLM layer (FOMC scoring, news NLP) is added.
// The TimescaleDB queries are identical; only the driver changes.
type MacroAnalysis struct {
	Base
	PollInterval time.Duration

	// ── Yield Curve (T10Y2Y — 2s10s spread, in percentage points) ─────────────
	// >1.0pp = steep (expansion). 0–1.0pp = normal. 0 to -0.5pp = flat/warning.
	// <-0.5pp = inverted (recession signal, 12–18 month lag).
	// Re-steepening after inversion = recession arriving.
	YCSteepThreshold    float64 // >this = "steep"         default 1.0
	YCFlatThreshold     float64 // <this = "flat"           default 0.0
	YCInvertedThreshold float64 // <this = "inverted"       default -0.5
	YCRestepeningBps    float64 // rise from minimum (pp) to call re-steepening default 0.5
	YCLookbackDays      int     // days of history for re-steepening detection   default 90

	// ── Real Interest Rate (DFII10 — 10Y TIPS yield, in %) ───────────────────
	// Deeply negative (<−2%) = max risk-on, drives capital into gold & growth.
	// Headwind (>+2%) = significant drag on growth stocks and gold.
	RealRateDeeplyNeg float64 // <this = "deeply_negative"  default -2.0
	RealRateHeadwind  float64 // >this = "headwind"          default 2.0

	// ── Fed Balance Sheet (WALCL — weekly, in millions USD) ──────────────────
	// WALCL is in millions; thresholds are stored/compared in billions.
	// QE/QT detection: compare latest to value 4 weeks ago.
	BSExpandThresholdBn  float64 // 4w change > +this Bn = "qe"  default 100
	BSContractThresholdBn float64 // 4w change < -this Bn = "qt"  default 100

	// ── Credit Spreads (BAMLH0A0HYM2 — HY OAS, in %, displayed in bps) ───────
	// Series value is in %; multiply × 100 for bps display.
	// <300bps benign, 300–600bps elevated, >600bps crisis.
	HYElevatedThreshold float64 // bps above which = "elevated"  default 300
	HYCrisisThreshold   float64 // bps above which = "crisis"     default 600

	// ── Breakeven Inflation (T10YIE — 10Y, in %) ─────────────────────────────
	// <2.5% anchored (Fed comfortable). >3.0% = unanchored, Fed acts aggressively.
	BreakevenRisingPct     float64 // >this = "rising"      default 2.5
	BreakevenUnanchoredPct float64 // >this = "unanchored"  default 3.0

	// ── M2 Money Supply (M2SL — monthly, in billions USD) ────────────────────
	// YoY growth rate drives inflation 12–24 months ahead.
	// >15% = inflationary surge. <0% = deflationary pressure.
	M2InflationaryPct float64 // YoY% > this = "inflationary"  default 15
	M2NormalMin       float64 // YoY% > this = "normal"         default 4

	// ── Composite MP Stance score boundaries ─────────────────────────────────
	MPAccommodativeScore float64 // weighted score > this = "accommodative"  default 0.4
	MPRestrictiveScore   float64 // weighted score < this = "restrictive"    default -0.4
}

// ── GrowthCycle ───────────────────────────────────────────────────────────────

// GrowthCycle holds thresholds for the growth-cycle analysis pass in the
// macro-analysis worker.  All values are configurable via .env — no recompile
// required to tune the algorithm.
//
// Data sources: all free FRED series.  No external APIs beyond FRED.
//
// TODO [PAID]:  Add S&P Global (Markit) PMI once a subscription is available.
// TODO [PAID]:  ISM Services PMI — requires ISM membership or paid data feed.
// TODO [PAID]:  China Caixin PMI — paid subscription.
// TODO [SCRAPE]: GDPNow (Atlanta Fed real-time GDP) — no public API.
// TODO [FUTURE]: Migrate to Python + asyncpg once LLM growth scoring is added.
type GrowthCycle struct {
	PollInterval time.Duration

	// ── PMI (NAPM — ISM Manufacturing, monthly, index 0–100) ─────────────────
	// Above 50 = expansion, below 50 = contraction.  New Orders sub-component
	// leads the headline by 1–2 months but is not a separate free FRED series.
	PMIStrong     float64 // >this = "strong_expansion"   default 55
	PMIExpansion  float64 // >this = "expansion"           default 50
	PMISlow       float64 // <this = "slowing"             default 45
	PMISevere     float64 // <this = "severe_contraction"  default 40

	// ── LEI (USSLIND — Conference Board Leading Economic Index, monthly) ──────
	// Tracks the 6-month annualized rate of change.
	// 3 consecutive monthly declines = historical recession signal ("rule of three").
	LEIExpansionRate float64 // 6m rate > this = "expanding"      default 0.0
	LEIRecessionRate float64 // 6m rate < this = "recession_risk"  default -3.0

	// ── Initial Jobless Claims (ICSA — weekly, persons) ──────────────────────
	// 4-week moving average used to smooth single-week volatility.
	ClaimsTight       float64 // 4w MA < this = "tight_labor"       default 225000
	ClaimsNormalizing float64 // 4w MA > this = "normalizing"        default 300000
	ClaimsCrisis      float64 // 4w MA > this = "crisis"             default 500000

	// ── Housing Starts (HOUST — monthly, annualized thousands of units) ───────
	// Housing accounts for 15–18% of GDP including related services.
	// When housing turns, broader economy typically follows in 6–12 months.
	HousingStrong float64 // > this thousands = "strong"  default 1500
	HousingWeak   float64 // < this thousands = "weak"    default 800

	// ── Real GDP (GDPC1 — quarterly, annualized %) ────────────────────────────
	// Computed from quarterly levels: (current/prior - 1) × 400 for annualized %.
	// Quarterly frequency means this signal is always 1–3 months stale.
	GDPStrong float64 // >this % = "strong"      default 3.0
	GDPStall  float64 // <this % = "stall_speed" default 1.0

	// ── Nonfarm Payrolls (PAYEMS — monthly, thousands net added) ─────────────
	// MoM change: >200K strong, 75–200K moderate, <0 recession signal.
	NFPStrong   float64 // >this K/month = "strong"    default 200
	NFPModerate float64 // >this K/month = "moderate"  default 75

	// ── Sahm Rule (SAHMREALTIME — monthly, pp above 12-month low) ────────────
	// >=0.5pp = recession historically already underway (not a leading indicator —
	// it confirms recession after the fact but is faster than NBER dating).
	SahmThreshold float64 // >= this = "recession_signal"  default 0.5

	// ── Real Retail Sales (RRSFS — monthly, YoY % change) ────────────────────
	// Inflation-adjusted consumption: cleanest GDP input for the consumer sector.
	RetailHealthy float64 // YoY > this = "healthy"   default 3.0

	// ── Michigan Consumer Sentiment (UMCSENT — monthly, index) ───────────────
	// Contrarian signal: <60 near market bottoms; >100 + VIX<12 = complacency.
	UMichBottom      float64 // <this = "near_bottom"    default 60
	UMichComplacency float64 // >this = "complacency"    default 100

	// ── Core Capex (NEWORDER — monthly, % 3-month trend) ─────────────────────
	// Capital Goods Nondefense Ex-Aircraft: cleanest business investment proxy.
	// 3-month rolling change vs 3 months prior used to smooth monthly volatility.
	CapexExpansion float64 // 3m trend > this = "expanding"  default 3.0
	CapexWarning   float64 // 3m trend < this = "warning"    default -3.0

	// ── Composite Growth Stance score boundaries ──────────────────────────────
	GrowthExpansionScore  float64 // weighted score > this = "expansion"   default 0.4
	GrowthContractionScore float64 // weighted score < this = "contraction" default -0.4
}

func LoadGrowthCycle() GrowthCycle {
	return GrowthCycle{
		PollInterval: pollFor("DATA_MACRO_GROWTH_POLL_INTERVAL", 12*time.Hour),

		PMIStrong:    floatEnv("GROWTH_PMI_STRONG", 55),
		PMIExpansion: floatEnv("GROWTH_PMI_EXPANSION", 50),
		PMISlow:      floatEnv("GROWTH_PMI_SLOW", 45),
		PMISevere:    floatEnv("GROWTH_PMI_SEVERE", 40),

		LEIExpansionRate: floatEnv("GROWTH_LEI_EXPANSION_RATE", 0.0),
		LEIRecessionRate: floatEnv("GROWTH_LEI_RECESSION_RATE", -3.0),

		ClaimsTight:       floatEnv("GROWTH_CLAIMS_TIGHT", 225000),
		ClaimsNormalizing: floatEnv("GROWTH_CLAIMS_NORMALIZING", 300000),
		ClaimsCrisis:      floatEnv("GROWTH_CLAIMS_CRISIS", 500000),

		HousingStrong: floatEnv("GROWTH_HOUSING_STRONG", 1500),
		HousingWeak:   floatEnv("GROWTH_HOUSING_WEAK", 800),

		GDPStrong: floatEnv("GROWTH_GDP_STRONG", 3.0),
		GDPStall:  floatEnv("GROWTH_GDP_STALL", 1.0),

		NFPStrong:   floatEnv("GROWTH_NFP_STRONG", 200),
		NFPModerate: floatEnv("GROWTH_NFP_MODERATE", 75),

		SahmThreshold: floatEnv("GROWTH_SAHM_THRESHOLD", 0.5),

		RetailHealthy: floatEnv("GROWTH_RETAIL_HEALTHY", 3.0),

		UMichBottom:      floatEnv("GROWTH_UMICH_BOTTOM", 60),
		UMichComplacency: floatEnv("GROWTH_UMICH_COMPLACENCY", 100),

		CapexExpansion: floatEnv("GROWTH_CAPEX_EXPANSION", 3.0),
		CapexWarning:   floatEnv("GROWTH_CAPEX_WARNING", -3.0),

		GrowthExpansionScore:   floatEnv("GROWTH_EXPANSION_SCORE", 0.4),
		GrowthContractionScore: floatEnv("GROWTH_CONTRACTION_SCORE", -0.4),
	}
}

func LoadMacroAnalysis() (MacroAnalysis, error) {
	b := LoadBase()
	if b.DatabaseURL == "" {
		return MacroAnalysis{}, fmt.Errorf("DATABASE_URL is required")
	}
	return MacroAnalysis{
		Base:         b,
		PollInterval: pollFor("DATA_MACRO_ANALYSIS_POLL_INTERVAL", 6*time.Hour),

		YCSteepThreshold:    floatEnv("MACRO_YC_STEEP_THRESHOLD", 1.0),
		YCFlatThreshold:     floatEnv("MACRO_YC_FLAT_THRESHOLD", 0.0),
		YCInvertedThreshold: floatEnv("MACRO_YC_INVERTED_THRESHOLD", -0.5),
		YCRestepeningBps:    floatEnv("MACRO_YC_RESTEEPENING_BPS", 0.5),
		YCLookbackDays:      intEnv("MACRO_YC_LOOKBACK_DAYS", 90),

		RealRateDeeplyNeg: floatEnv("MACRO_REAL_RATE_DEEPLY_NEGATIVE", -2.0),
		RealRateHeadwind:  floatEnv("MACRO_REAL_RATE_HEADWIND", 2.0),

		BSExpandThresholdBn:   floatEnv("MACRO_BS_EXPAND_THRESHOLD_BN", 100.0),
		BSContractThresholdBn: floatEnv("MACRO_BS_CONTRACT_THRESHOLD_BN", 100.0),

		HYElevatedThreshold: floatEnv("MACRO_HY_ELEVATED_BPS", 300),
		HYCrisisThreshold:   floatEnv("MACRO_HY_CRISIS_BPS", 600),

		BreakevenRisingPct:     floatEnv("MACRO_BREAKEVEN_RISING_PCT", 2.5),
		BreakevenUnanchoredPct: floatEnv("MACRO_BREAKEVEN_UNANCHORED_PCT", 3.0),

		M2InflationaryPct: floatEnv("MACRO_M2_INFLATIONARY_PCT", 15.0),
		M2NormalMin:       floatEnv("MACRO_M2_NORMAL_MIN_PCT", 4.0),

		MPAccommodativeScore: floatEnv("MACRO_MP_ACCOMMODATIVE_SCORE", 0.4),
		MPRestrictiveScore:   floatEnv("MACRO_MP_RESTRICTIVE_SCORE", -0.4),
	}, nil
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

		EPSGrowthStrong: floatEnv("FUNDAMENTAL_EPS_GROWTH_STRONG", 15),
		EPSGrowthWeak:   floatEnv("FUNDAMENTAL_EPS_GROWTH_WEAK", 5),

		RevGrowthStrong: floatEnv("FUNDAMENTAL_REV_GROWTH_STRONG", 10),
		RevGrowthWeak:   floatEnv("FUNDAMENTAL_REV_GROWTH_WEAK", 2),

		PEVs5YCheapPct: floatEnv("FUNDAMENTAL_PE_5Y_CHEAP_PCT", 15),
		PEVs5YExpPct:   floatEnv("FUNDAMENTAL_PE_5Y_EXPENSIVE_PCT", 15),

		PEAbsValue:  floatEnv("FUNDAMENTAL_PE_ABS_VALUE", 15),
		PEAbsGrowth: floatEnv("FUNDAMENTAL_PE_ABS_GROWTH", 25),

		FCFYieldAttractive: floatEnv("FUNDAMENTAL_FCF_YIELD_ATTRACTIVE", 5),
		FCFYieldFair:       floatEnv("FUNDAMENTAL_FCF_YIELD_FAIR", 2),

		FCFDivEPSGrowth: floatEnv("FUNDAMENTAL_FCF_DIV_EPS_GROWTH", 10),
		FCFDivYieldLow:  floatEnv("FUNDAMENTAL_FCF_DIV_YIELD_LOW", 2),
		FCFDivYieldHigh: floatEnv("FUNDAMENTAL_FCF_DIV_YIELD_HIGH", 5),

		GrossMarginMoat: floatEnv("FUNDAMENTAL_GROSS_MARGIN_MOAT", 40),
		GrossMarginAvg:  floatEnv("FUNDAMENTAL_GROSS_MARGIN_AVG", 20),

		NetMarginStrong: floatEnv("FUNDAMENTAL_NET_MARGIN_STRONG", 15),
		NetMarginAvg:    floatEnv("FUNDAMENTAL_NET_MARGIN_AVG", 5),

		PECompressionFlat: floatEnv("FUNDAMENTAL_PE_COMPRESSION_FLAT", 5),

		PEGUndervalued: floatEnv("FUNDAMENTAL_PEG_UNDERVALUED", 1),
		PEGFair:        floatEnv("FUNDAMENTAL_PEG_FAIR", 2),

		SurpriseBeatPct:  floatEnv("FUNDAMENTAL_SURPRISE_BEAT_PCT", 2),
		SurpriseMissPct:  floatEnv("FUNDAMENTAL_SURPRISE_MISS_PCT", 2),
		SurpriseQuarters: intEnv("FUNDAMENTAL_SURPRISE_QUARTERS", 4),

		CompositeStrong: floatEnv("FUNDAMENTAL_COMPOSITE_STRONG", 0.5),
		CompositeWeak:   floatEnv("FUNDAMENTAL_COMPOSITE_WEAK", 0.5),

		MarginTrendStablePP: floatEnv("FUNDAMENTAL_MARGIN_TREND_STABLE_PP", 2),
		MarginTrendQuarters: intEnv("FUNDAMENTAL_MARGIN_TREND_QUARTERS", 8),

		// Tier 2
		ROEExcellent:  floatEnv("FUNDAMENTAL_ROE_EXCELLENT", 15),
		ROEAdequate:   floatEnv("FUNDAMENTAL_ROE_ADEQUATE", 8),
		ROICExcellent: floatEnv("FUNDAMENTAL_ROIC_EXCELLENT", 15),
		ROICAdequate:  floatEnv("FUNDAMENTAL_ROIC_ADEQUATE", 8),

		DEConservative: floatEnv("FUNDAMENTAL_DE_CONSERVATIVE", 1.0),
		DEManageable:   floatEnv("FUNDAMENTAL_DE_MANAGEABLE", 2.0),

		NetDebtEBITDALow:  floatEnv("FUNDAMENTAL_NET_DEBT_EBITDA_LOW", 2.0),
		NetDebtEBITDAHigh: floatEnv("FUNDAMENTAL_NET_DEBT_EBITDA_HIGH", 4.0),

		EVEBITDAValue: floatEnv("FUNDAMENTAL_EV_EBITDA_VALUE", 10),
		EVEBITDAFair:  floatEnv("FUNDAMENTAL_EV_EBITDA_FAIR", 20),

		CurrentRatioSafe:    floatEnv("FUNDAMENTAL_CURRENT_RATIO_SAFE", 1.5),
		CurrentRatioMonitor: floatEnv("FUNDAMENTAL_CURRENT_RATIO_MONITOR", 1.0),

		PBValue:     floatEnv("FUNDAMENTAL_PB_VALUE", 1.5),
		PBExpensive: floatEnv("FUNDAMENTAL_PB_EXPENSIVE", 5.0),

		DividendYieldMin:  floatEnv("FUNDAMENTAL_DIVIDEND_YIELD_MIN", 2),
		DividendYieldHigh: floatEnv("FUNDAMENTAL_DIVIDEND_YIELD_HIGH", 6),
		PayoutRatioSafe:   floatEnv("FUNDAMENTAL_PAYOUT_RATIO_SAFE", 60),
		PayoutRatioDanger: floatEnv("FUNDAMENTAL_PAYOUT_RATIO_DANGER", 80),

		CapExIntensityLow:  floatEnv("FUNDAMENTAL_CAPEX_INTENSITY_LOW", 5),
		CapExIntensityHigh: floatEnv("FUNDAMENTAL_CAPEX_INTENSITY_HIGH", 20),

		// Tier 3
		ShareDeclineBuyback: floatEnv("FUNDAMENTAL_SHARE_DECLINE_BUYBACK", 2),
		ShareGrowthDilution: floatEnv("FUNDAMENTAL_SHARE_GROWTH_DILUTION", 3),
		ShareTrendYears:     intEnv("FUNDAMENTAL_SHARE_TREND_YEARS", 2),

		DCFWACCPct:        floatEnv("FUNDAMENTAL_DCF_WACC_PCT", 10),
		DCFTerminalGrowth: floatEnv("FUNDAMENTAL_DCF_TERMINAL_GROWTH_PCT", 3),
		DCFGrowthYears:    intEnv("FUNDAMENTAL_DCF_GROWTH_YEARS", 5),
		DCFMaxGrowthPct:   floatEnv("FUNDAMENTAL_DCF_MAX_GROWTH_PCT", 20),
		DCFSafetyMargin:   floatEnv("FUNDAMENTAL_DCF_SAFETY_MARGIN_PCT", 70),
		DCFOvervalued:     floatEnv("FUNDAMENTAL_DCF_OVERVALUED_PCT", 110),

		InterestCoverageSafe:     floatEnv("FUNDAMENTAL_INTEREST_COVERAGE_SAFE", 5),
		InterestCoverageAdequate: floatEnv("FUNDAMENTAL_INTEREST_COVERAGE_ADEQUATE", 2),

		GoodwillLowPct:  floatEnv("FUNDAMENTAL_GOODWILL_LOW_PCT", 20),
		GoodwillHighPct: floatEnv("FUNDAMENTAL_GOODWILL_HIGH_PCT", 40),

		PSValue:       floatEnv("FUNDAMENTAL_PS_VALUE", 5),
		PSFair:        floatEnv("FUNDAMENTAL_PS_FAIR", 10),
		PSSpeculative: floatEnv("FUNDAMENTAL_PS_SPECULATIVE", 15),

		AnalystUpsideBullish:   floatEnv("FUNDAMENTAL_ANALYST_UPSIDE_BULLISH", 15),
		AnalystDownsideBearish: floatEnv("FUNDAMENTAL_ANALYST_DOWNSIDE_BEARISH", -5),
		AnalystRecUpgrade:      floatEnv("FUNDAMENTAL_ANALYST_REC_UPGRADE_DELTA", 5),
		AnalystRecDowngrade:    floatEnv("FUNDAMENTAL_ANALYST_REC_DOWNGRADE_DELTA", -5),

		FCFConversionHigh: floatEnv("FUNDAMENTAL_FCF_CONVERSION_HIGH", 1.0),
		FCFConversionLow:  floatEnv("FUNDAMENTAL_FCF_CONVERSION_LOW", 0.7),

		EquityInterval: strEnvDefault("FUNDAMENTAL_EQUITY_INTERVAL", "1Day"),

		// Correlation analysis
		CorrBullishConvergenceMin:       intEnv("CORR_BULLISH_CONVERGENCE_MIN_CONDITIONS", 3),
		CorrLeverageCycleMin:            intEnv("CORR_LEVERAGE_CYCLE_MIN_CONDITIONS", 3),
		CorrValueTrapMin:                intEnv("CORR_VALUE_TRAP_MIN_CONDITIONS", 3),
		CorrReceivablesGrowthMultiplier: floatEnv("CORR_RECEIVABLES_GROWTH_MULTIPLIER", 1.1),

		// Qualitative
		InsiderClusterWindowDays:  intEnv("QUAL_INSIDER_CLUSTER_WINDOW_DAYS", 90),
		InsiderClusterMinBuyers:   intEnv("QUAL_INSIDER_CLUSTER_MIN_BUYERS", 3),
		QualMoatStabilityQuarters: intEnv("QUAL_MOAT_STABILITY_QUARTERS", 8),
		QualSentimentPositive:     floatEnv("QUAL_SENTIMENT_POSITIVE_THRESHOLD", 0.15),
		QualSentimentNegative:     floatEnv("QUAL_SENTIMENT_NEGATIVE_THRESHOLD", -0.15),
		QualRDHealthyPct:          floatEnv("QUAL_RD_HEALTHY_PCT", 10),
		QualRDModeratePct:         floatEnv("QUAL_RD_MODERATE_PCT", 3),
		QualMoatStableStdPP:       floatEnv("QUAL_MOAT_STABLE_STD_PP", 5),
	}, nil
}
