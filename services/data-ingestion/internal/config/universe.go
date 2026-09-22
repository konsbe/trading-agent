package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// floatEnv reads a float env var, falling back to def when unset or malformed.
// Lives here rather than in config.go to keep this file's additions self-
// contained; the sibling intEnv/durationEnv helpers are in config.go.
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

// Universe configures the data-universe worker (spec §8.1).
//
// Phase 1 scope for this worker is the weekly symbol-list refresh with §3.1
// eligibility, plus the weekly sector/shares/market-cap refresh. The resumable
// 3-year bar backfill and the daily incremental refresh are separate build-order
// steps and are not wired here yet.
type Universe struct {
	Base

	FinnhubKey string

	// Exchange code passed to Finnhub /stock/symbol. "US" returns the whole US
	// listing in one request.
	Exchange string

	// PollSymbols is how often the symbol list is re-fetched. Weekly by
	// default: the listed universe changes slowly and the free Finnhub quota is
	// the binding constraint (§2.3).
	PollSymbols time.Duration

	// PollFundamentals is how often sector / shares outstanding / market cap are
	// refreshed. Weekly, not daily — these do not move intraday (§2.3).
	PollFundamentals time.Duration

	// StartupDelay lets the database and any co-scheduled ingestion settle
	// before the first pass, matching the other workers' behaviour.
	StartupDelay time.Duration

	// §3.1 eligibility policy. Empty values fall back to the spec defaults in
	// internal/universe; they never mean "allow nothing".
	AllowedTypes     []string
	AllowedMICs      []string
	ExcludedSuffixes []string
	AllowEmptyMIC    bool

	// MinBarsHistory is §3.1's / §3.2's bar minimum, 252 by default: §3.7's
	// 52-week window reads high[t-251], so a complete window needs 252 bars
	// including today. Lowering it reintroduces silently truncated windows.
	//
	// Recorded and reported here but NOT used to set is_eligible — see the note
	// on store.EligibleUniverse for why that would be circular.
	MinBarsHistory int

	// BarInterval / BarSource identify the rows in equity_ohlcv that count as
	// this scanner's daily bars (§7: reuse equity_ohlcv, do not create a
	// parallel bar table).
	//
	// The source is 'yahoo_finance', which is what internal/fetch/yahoo actually
	// writes and what data-analyzer's bar reader prefers. §7 originally said
	// 'yahoo'; that value exists nowhere and would match zero rows.
	BarInterval string
	BarSource   string

	// ── Pilot subset (§2.2, migration 010) ─────────────────────────────────

	// SubsetEnable runs the selection pass and restricts the backfill and daily
	// refresh to the selected subset.
	//
	// The pilot exists because free-tier quotas cannot cover ~4,978 symbols, and
	// because §6's base rate should be read cheaply before paying to backfill a
	// universe on a score with no demonstrated edge.
	SubsetEnable bool

	// SubsetStrategy: "stratified" (default), "random", or "explicit".
	SubsetStrategy string

	// SubsetSize is how many symbols to select.
	SubsetSize int

	// SubsetMaxSize is the hard cap the selection asserts before writing.
	//
	// Was 450, which was Tiingo's free tier metering 500 unique symbols per
	// month — a billing cap wearing the costume of a design decision. The Power
	// upgrade (2026-09-21) allows ~108,980, so the number that set 450 is gone.
	//
	// The assertion is KEPT, not removed. It no longer guards a bill, but it
	// still catches an order-of-magnitude error: a selection query that tried to
	// claim 50,000 symbols would be a bug rather than a plan, and this is where
	// that surfaces instead of at the provider. Default set above the current
	// eligible count (~4,975) so ordinary universe growth does not trip it.
	SubsetMaxSize int

	// SubsetPennyFloorPct is the minimum share drawn from §3.2's penny bucket.
	// Without a floor, a ~9% sampling rate leaves the penny bucket's thresholds
	// unexercised whenever the universe's natural proportion is small.
	SubsetPennyFloorPct float64

	SubsetPennyMinPrice float64
	SubsetPennyMaxPrice float64

	// PricingEnable runs the Finnhub /quote pass that supplies the bucketing
	// signal for stratification.
	PricingEnable bool

	// SubsetReselect permits replacing an existing pilot draw. Off by default
	// because a redraw spends a second batch of unique symbols on a metered
	// provider and, with an empty seed, changes the sample the pilot's numbers
	// were measured on.
	SubsetReselect bool

	// PriceInterval / PriceSource are the equity_ohlcv rows holding that signal.
	// Separate from BarInterval/BarSource so pricing the universe costs no bar
	// provider quota.
	PriceInterval string
	PriceSource   string

	// SubsetSeed makes a draw reproducible. Empty means a different sample per
	// run — set it for anything whose numbers will be cited.
	SubsetSeed string

	// SubsetSymbols is the verbatim list for the "explicit" strategy.
	SubsetSymbols []string

	// EnableSymbols / EnableBackfill / EnableDailyBars allow
	// each pass to be turned off independently, following the
	// FUNDAMENTAL_ENABLE_* convention.
	EnableSymbols   bool
	EnableBackfill  bool
	EnableDailyBars bool

	// ── §8.1.3 resumable bar backfill ──────────────────────────────────────

	// BackfillYears is the history depth. 3 per §2.3.
	BackfillYears int

	// BackfillBatchSize is how many symbols are claimed per round. Bounded so a
	// crash loses at most one batch's worth of in-flight claims, and so progress
	// is logged regularly across a multi-hour run.
	BackfillBatchSize int

	// BackfillConcurrency is how many symbols are fetched in parallel. The rate
	// limiter is shared, so this controls pipelining rather than throughput —
	// raising it will not exceed RequestsPerSecond.
	BackfillConcurrency int

	// BackfillClaimLease is how long an in_progress claim is honoured before
	// another round may reclaim it. This is what makes a killed worker's rows
	// recoverable instead of stranded.
	BackfillClaimLease time.Duration

	// BackfillMaxAttempts caps retries per symbol across runs. Beyond this the
	// symbol stays 'failed' and is reported rather than retried forever.
	BackfillMaxAttempts int

	// BackfillIdleInterval is how long to wait before re-checking for claimable
	// work once a round finds none. The backfill is run-once in spirit, but the
	// loop keeps running so newly-listed symbols get picked up.
	BackfillIdleInterval time.Duration

	// ── Yahoo request behaviour (§2.3: rate, concurrency, retry configurable) ──

	// RequestsPerSecond throttles Yahoo requests. §2.3 budgets ~5/s, but the
	// endpoint is unofficial and §2.2 says to be polite; 2/s is the default
	// compromise and sustains a ~6k-symbol pass in under an hour.
	RequestsPerSecond float64
	RequestBurst      int

	// TwelveDataToken / TwelveDataRequestsPerSecond apply when
	// UNIVERSE_BAR_SOURCE is twelve_data, the Phase 1 primary provider.
	TwelveDataToken             string
	TwelveDataRequestsPerSecond float64

	// TiingoToken / TiingoRequestsPerSecond apply when UNIVERSE_BAR_SOURCE is
	// "tiingo". The rate is politeness only — Tiingo's real constraint is
	// monthly unique symbols, guarded by the subset size cap.
	TiingoToken             string
	TiingoRequestsPerSecond float64
	RequestTimeout          time.Duration
	RequestMaxRetries       int
	BackoffBase             time.Duration
	BackoffMax              time.Duration

	// ── §8.1.4 daily incremental refresh ───────────────────────────────────

	// DailyBarsInterval is how often the incremental refresh runs. Daily after
	// the US close; the worker does not implement a market calendar, so this is
	// a plain interval and a run on a holiday simply returns no new bars.
	DailyBarsInterval time.Duration

	// DailyBarsLookbackDays is the window requested per symbol. Wider than one
	// day on purpose: it repairs gaps from a missed run or a late provider
	// correction, and upserts make the overlap free.
	DailyBarsLookbackDays int
}

func LoadUniverse() (Universe, error) {
	b := LoadBase()
	if b.DatabaseURL == "" {
		return Universe{}, fmt.Errorf("DATABASE_URL is required")
	}
	week := 168 * time.Hour
	return Universe{
		Base:       b,
		FinnhubKey: strings.TrimSpace(os.Getenv("FINNHUB_API_KEY")),

		Exchange:         env("UNIVERSE_EXCHANGE", "US"),
		PollSymbols:      pollFor("UNIVERSE_SYMBOLS_POLL_INTERVAL", week),
		PollFundamentals: pollFor("UNIVERSE_FUNDAMENTALS_POLL_INTERVAL", week),
		StartupDelay:     time.Duration(intEnv("UNIVERSE_STARTUP_DELAY_SECS", 30)) * time.Second,

		AllowedTypes:     splitCSV("UNIVERSE_ALLOWED_TYPES"),
		AllowedMICs:      splitCSV("UNIVERSE_ALLOWED_MICS"),
		ExcludedSuffixes: splitCSV("UNIVERSE_EXCLUDED_SUFFIXES"),
		AllowEmptyMIC:    env("UNIVERSE_ALLOW_EMPTY_MIC", "false") == "true",

		MinBarsHistory: intEnv("UNIVERSE_MIN_BARS_HISTORY", 252),

		BarInterval: env("UNIVERSE_BAR_INTERVAL", "1Day"),
		// Default is tiingo: it is the only source verified to apply ONE
		// consistent adjustment factor across a series. twelve_data is faster but
		// alternates between adjusted and unadjusted bars within a single
		// response (see internal/barquality), and yahoo_finance carries no
		// dividend adjustment at all.
		BarSource: env("UNIVERSE_BAR_SOURCE", "tiingo"),

		SubsetEnable:        env("UNIVERSE_SUBSET_ENABLE", "false") == "true",
		SubsetStrategy:      env("UNIVERSE_SUBSET_STRATEGY", "stratified"),
		SubsetSize:          intEnv("UNIVERSE_SUBSET_SIZE", 450),
		SubsetMaxSize:       intEnv("TIINGO_MAX_SELECTED_SYMBOLS", 6000),
		SubsetPennyFloorPct: floatEnv("UNIVERSE_SUBSET_PENNY_FLOOR_PCT", 0.20),
		PricingEnable:       env("UNIVERSE_PRICING_ENABLE", "false") == "true",
		PriceInterval:       env("UNIVERSE_PRICE_INTERVAL", "quote_snapshot"),
		PriceSource:         env("UNIVERSE_PRICE_SOURCE", "finnhub_quote"),

		SubsetPennyMinPrice: floatEnv("UNIVERSE_SUBSET_PENNY_MIN_PRICE", 0.30),
		SubsetPennyMaxPrice: floatEnv("UNIVERSE_SUBSET_PENNY_MAX_PRICE", 2.00),
		SubsetSeed:          env("UNIVERSE_SUBSET_SEED", ""),
		SubsetSymbols:       splitCSV("UNIVERSE_SUBSET_SYMBOLS"),

		EnableSymbols:   env("UNIVERSE_ENABLE_SYMBOLS", "true") == "true",
		EnableBackfill:  env("UNIVERSE_ENABLE_BACKFILL", "true") == "true",
		EnableDailyBars: env("UNIVERSE_ENABLE_DAILY_BARS", "true") == "true",

		BackfillYears:        intEnv("UNIVERSE_BACKFILL_YEARS", 3),
		BackfillBatchSize:    intEnv("UNIVERSE_BACKFILL_BATCH_SIZE", 200),
		BackfillConcurrency:  intEnv("UNIVERSE_BACKFILL_CONCURRENCY", 4),
		BackfillClaimLease:   durationEnv("UNIVERSE_BACKFILL_CLAIM_LEASE", 15*time.Minute),
		BackfillMaxAttempts:  intEnv("UNIVERSE_BACKFILL_MAX_ATTEMPTS", 3),
		BackfillIdleInterval: durationEnv("UNIVERSE_BACKFILL_IDLE_INTERVAL", time.Hour),

		TwelveDataToken:             strings.TrimSpace(os.Getenv("TWELVE_DATA_API_KEY")),
		TwelveDataRequestsPerSecond: floatEnv("TWELVE_DATA_RATE_PER_SEC", 0.125),

		TiingoToken:             strings.TrimSpace(os.Getenv("TIINGO_API_KEY")),
		TiingoRequestsPerSecond: floatEnv("TIINGO_RATE_PER_SEC", 0.0138),

		// Generic HTTP knobs, applied to WHICHEVER provider UNIVERSE_BAR_SOURCE
		// names. They were called UNIVERSE_YAHOO_* while feeding all three
		// adapters, which is how Twelve Data silently inherited Yahoo's 30s
		// timeout and then failed 12 symbols with "Client.Timeout exceeded while
		// awaiting headers" — a 3-year daily response is ~100KB and this network
		// is lossy. The UNIVERSE_YAHOO_* names remain as fallbacks so existing
		// deployments keep working.
		RequestsPerSecond: floatEnv2("UNIVERSE_BAR_REQUESTS_PER_SEC", "UNIVERSE_YAHOO_REQUESTS_PER_SEC", 2.0),
		RequestBurst:      intEnv2("UNIVERSE_BAR_BURST", "UNIVERSE_YAHOO_BURST", 1),
		RequestTimeout:    durationEnv2("UNIVERSE_BAR_TIMEOUT", "UNIVERSE_YAHOO_TIMEOUT", 90*time.Second),
		RequestMaxRetries: intEnv2("UNIVERSE_BAR_MAX_RETRIES", "UNIVERSE_YAHOO_MAX_RETRIES", 3),
		BackoffBase:       durationEnv2("UNIVERSE_BAR_BACKOFF_BASE", "UNIVERSE_YAHOO_BACKOFF_BASE", 2*time.Second),
		BackoffMax:        durationEnv2("UNIVERSE_BAR_BACKOFF_MAX", "UNIVERSE_YAHOO_BACKOFF_MAX", 60*time.Second),

		DailyBarsInterval:     durationEnv("UNIVERSE_DAILY_BARS_INTERVAL", 24*time.Hour),
		DailyBarsLookbackDays: intEnv("UNIVERSE_DAILY_BARS_LOOKBACK_DAYS", 7),
	}, nil
}

// ─── Preferred-name-with-fallback env readers ─────────────────────────────────
//
// Each reads `preferred` and falls back to `legacy`, so renaming a variable does
// not silently reset a deployment's tuning to the default.

func floatEnv2(preferred, legacy string, def float64) float64 {
	if v := strings.TrimSpace(os.Getenv(preferred)); v != "" {
		return floatEnv(preferred, def)
	}
	return floatEnv(legacy, def)
}

func intEnv2(preferred, legacy string, def int) int {
	if v := strings.TrimSpace(os.Getenv(preferred)); v != "" {
		return intEnv(preferred, def)
	}
	return intEnv(legacy, def)
}

func durationEnv2(preferred, legacy string, def time.Duration) time.Duration {
	if v := strings.TrimSpace(os.Getenv(preferred)); v != "" {
		return durationEnv(preferred, def)
	}
	return durationEnv(legacy, def)
}
