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
		ROEExcellent: floatEnv("FUNDAMENTAL_ROE_EXCELLENT", 15),
		ROEAdequate:  floatEnv("FUNDAMENTAL_ROE_ADEQUATE", 8),

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
	}, nil
}
