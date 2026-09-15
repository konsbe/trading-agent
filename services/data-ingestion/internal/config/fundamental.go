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

	// Alpha Vantage API key — free at https://www.alphavantage.co/support/#api-key
	// Provides forward P/E, sector, beta, PEG ratio (not available on Finnhub free tier).
	AlphaVantageKey string

	// Equity symbols to fetch fundamentals for.
	Symbols []string

	// Poll intervals.
	PollMetrics        time.Duration // /stock/metric — TTM ratios updated daily
	PollFinancials     time.Duration // /stock/financials-reported — quarterly cadence
	PollEarnings       time.Duration // /stock/earnings — updated after each report
	PollOverview       time.Duration // Alpha Vantage OVERVIEW — weekly is sufficient
	PollRecommendation time.Duration // /stock/recommendation — analyst buy/hold/sell monthly

	// Feature toggles.
	EnableMetrics        bool // Tier 1: EPS, revenue, P/E, FCF, margins (TTM ratios)
	EnableFinancials     bool // Detailed quarterly/annual income + cash-flow statements
	EnableEarnings       bool // Historical EPS actuals vs estimates (earnings surprises)
	EnableOverview       bool // Alpha Vantage: forward P/E, sector, beta (requires ALPHA_VANTAGE_API_KEY)
	EnableRecommendation bool // Analyst consensus recommendation trend (free tier)

	// Frequency for financials-reported endpoint ("quarterly" or "annual").
	FinancialsFreq string

	// Number of historical quarterly reports to store (default 8 = 2 years).
	// Enables 8-quarter margin trend analysis in data-analyzer.
	FinancialsLimit int

	// Also fetch annual (10-K) reports in a separate API call alongside quarterly.
	// Enables year-over-year revenue/income tracking from SEC 10-K filings.
	EnableAnnualFinancials bool

	// Number of annual reports to store (default 5 = 5 years of 10-Ks).
	AnnualFinancialsLimit int

	// ── Qualitative data ingestion ────────────────────────────────────────────
	// Insider transactions (Finnhub /stock/insider-transactions, SEC Form 4).
	// Cluster buying (3+ insiders in 90 days) is a high-conviction bullish signal.
	EnableInsiderTransactions bool
	PollInsiderTransactions   time.Duration // default 24h

	// News sentiment via Alpha Vantage NEWS_SENTIMENT endpoint.
	// Populates news_headlines.sentiment with per-article numeric scores (-1 to +1).
	EnableNewsSentiment bool
	PollNewsSentiment   time.Duration // default 24h

	// Institutional ownership (Finnhub /stock/investor-ownership).
	// Tracks top-holder share changes quarter-over-quarter.
	EnableInstitutionalOwnership bool
	PollInstitutionalOwnership   time.Duration // default 7×24h (quarterly data)
	InstitutionalOwnershipLimit  int           // number of top holders to fetch (default 25)

	// ── Universe-wide metrics pass (spec §8.4, build-order step 3b) ──────────
	//
	// Widens ONLY runMetrics to the eligible universe, because market_cap and
	// shares_outstanding — the two fields the scanner's §3.2 gate and §3.9 proxy
	// need — come from that one sub-task. The other seven keep iterating the
	// static Symbols list untouched.

	// MetricsUseUniverse switches the metrics symbol source from the static list
	// to the UNION of that list and universe_symbols. Default false: this worker
	// has live consumers, so widening is opt-in.
	MetricsUseUniverse bool

	// MetricsUniverseCheckpointed runs the universe pass through
	// fundamental_fetch_state so a multi-hour pass survives a restart. Without it
	// the widened pass re-walks the whole universe on every tick.
	MetricsUniverseCheckpointed bool

	// MetricsUniversePoll is the widened pass's own cadence, default weekly.
	// Deliberately NOT inherited from PollMetrics (24h): at ~3.3 hours per pass
	// a daily tick would spend ~23 hours of API budget per week, against §2.3
	// and §8.1.2 which both specify weekly.
	MetricsUniversePoll time.Duration

	// MetricsUniverseRefreshInterval is how stale a symbol's last success may be
	// before it is claimable again. This, not a reset job, is what makes the
	// cadence emergent — see 009_fundamental_fetch_state.sql.
	MetricsUniverseRefreshInterval time.Duration

	MetricsUniverseBatchSize   int
	MetricsUniverseClaimLease  time.Duration
	MetricsUniverseMaxAttempts int

	// MetricsUniverseIdleInterval is how long to wait before re-checking for
	// claimable work once a round finds none.
	MetricsUniverseIdleInterval time.Duration
}

func LoadFundamental() (Fundamental, error) {
	b := LoadBase()
	if b.DatabaseURL == "" {
		return Fundamental{}, fmt.Errorf("DATABASE_URL is required")
	}

	syms := splitCSV("FUNDAMENTAL_SYMBOLS")
	if len(syms) == 0 {
		syms = MergedAlpacaEquitySymbols()
	}
	if len(syms) == 0 {
		syms = []string{"AAPL", "MSFT", "SPY"}
	}

	defaultMetrics := 24 * time.Hour
	defaultFinancials := 7 * 24 * time.Hour
	defaultEarnings := 24 * time.Hour
	defaultOverview := 7 * 24 * time.Hour   // weekly — free tier is 25 calls/day
	defaultRecommendation := 24 * time.Hour // analyst rec changes monthly; daily poll is sufficient

	freq := "quarterly"
	if v := os.Getenv("FUNDAMENTAL_FINANCIALS_FREQ"); v == "annual" {
		freq = "annual"
	}

	limit := intEnv("FUNDAMENTAL_FINANCIALS_LIMIT", 8)
	if limit < 1 {
		limit = 8
	}

	annualLimit := intEnv("FUNDAMENTAL_ANNUAL_FINANCIALS_LIMIT", 5)
	if annualLimit < 1 {
		annualLimit = 5
	}

	instLimit := intEnv("FUNDAMENTAL_INSTITUTIONAL_OWNERSHIP_LIMIT", 25)
	if instLimit < 1 {
		instLimit = 25
	}

	return Fundamental{
		Base:                         b,
		FinnhubKey:                   os.Getenv("FINNHUB_API_KEY"),
		AlphaVantageKey:              os.Getenv("ALPHA_VANTAGE_API_KEY"),
		Symbols:                      syms,
		PollMetrics:                  pollFor("DATA_FUNDAMENTAL_METRICS_POLL_INTERVAL", defaultMetrics),
		PollFinancials:               pollFor("DATA_FUNDAMENTAL_FINANCIALS_POLL_INTERVAL", defaultFinancials),
		PollEarnings:                 pollFor("DATA_FUNDAMENTAL_EARNINGS_POLL_INTERVAL", defaultEarnings),
		PollOverview:                 pollFor("DATA_FUNDAMENTAL_OVERVIEW_POLL_INTERVAL", defaultOverview),
		PollRecommendation:           pollFor("DATA_FUNDAMENTAL_RECOMMENDATION_POLL_INTERVAL", defaultRecommendation),
		EnableMetrics:                env("FUNDAMENTAL_ENABLE_METRICS", "true") == "true",
		EnableFinancials:             env("FUNDAMENTAL_ENABLE_FINANCIALS", "true") == "true",
		EnableEarnings:               env("FUNDAMENTAL_ENABLE_EARNINGS", "true") == "true",
		EnableOverview:               env("FUNDAMENTAL_ENABLE_OVERVIEW", "true") == "true",
		EnableRecommendation:         env("FUNDAMENTAL_ENABLE_RECOMMENDATION", "true") == "true",
		EnableAnnualFinancials:       env("FUNDAMENTAL_ENABLE_ANNUAL_FINANCIALS", "true") == "true",
		FinancialsFreq:               freq,
		FinancialsLimit:              limit,
		AnnualFinancialsLimit:        annualLimit,
		EnableInsiderTransactions:    env("FUNDAMENTAL_ENABLE_INSIDER_TRANSACTIONS", "true") == "true",
		PollInsiderTransactions:      pollFor("DATA_FUNDAMENTAL_INSIDER_POLL_INTERVAL", 24*time.Hour),
		EnableNewsSentiment:          env("FUNDAMENTAL_ENABLE_NEWS_SENTIMENT", "true") == "true",
		PollNewsSentiment:            pollFor("DATA_FUNDAMENTAL_NEWS_SENTIMENT_POLL_INTERVAL", 24*time.Hour),
		EnableInstitutionalOwnership: env("FUNDAMENTAL_ENABLE_INSTITUTIONAL_OWNERSHIP", "true") == "true",
		PollInstitutionalOwnership:   pollFor("DATA_FUNDAMENTAL_INSTITUTIONAL_POLL_INTERVAL", 7*24*time.Hour),
		InstitutionalOwnershipLimit:  instLimit,

		MetricsUseUniverse:          env("FUNDAMENTAL_METRICS_SYMBOL_SOURCE", "env") == "env+universe",
		MetricsUniverseCheckpointed: env("FUNDAMENTAL_METRICS_UNIVERSE_CHECKPOINTED", "true") == "true",

		MetricsUniversePoll:            pollFor("FUNDAMENTAL_METRICS_UNIVERSE_POLL_INTERVAL", 168*time.Hour),
		MetricsUniverseRefreshInterval: pollFor("FUNDAMENTAL_METRICS_UNIVERSE_REFRESH_INTERVAL", 168*time.Hour),
		MetricsUniverseBatchSize:       intEnv("FUNDAMENTAL_METRICS_UNIVERSE_BATCH_SIZE", 250),
		MetricsUniverseClaimLease:      pollFor("FUNDAMENTAL_METRICS_UNIVERSE_CLAIM_LEASE", 30*time.Minute),
		MetricsUniverseMaxAttempts:     intEnv("FUNDAMENTAL_METRICS_UNIVERSE_MAX_ATTEMPTS", 3),
		MetricsUniverseIdleInterval:    pollFor("FUNDAMENTAL_METRICS_UNIVERSE_IDLE_INTERVAL", time.Hour),
	}, nil
}
