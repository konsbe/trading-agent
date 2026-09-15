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

	// EnableSymbols / EnableFundamentals / EnableBackfill / EnableDailyBars allow
	// each pass to be turned off independently, following the
	// FUNDAMENTAL_ENABLE_* convention.
	EnableSymbols      bool
	EnableFundamentals bool
	EnableBackfill     bool
	EnableDailyBars    bool

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
	RequestTimeout    time.Duration
	RequestMaxRetries int
	BackoffBase       time.Duration
	BackoffMax        time.Duration

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
		BarSource:   env("UNIVERSE_BAR_SOURCE", "yahoo_finance"),

		EnableSymbols:      env("UNIVERSE_ENABLE_SYMBOLS", "true") == "true",
		EnableFundamentals: env("UNIVERSE_ENABLE_FUNDAMENTALS", "true") == "true",
		EnableBackfill:     env("UNIVERSE_ENABLE_BACKFILL", "true") == "true",
		EnableDailyBars:    env("UNIVERSE_ENABLE_DAILY_BARS", "true") == "true",

		BackfillYears:        intEnv("UNIVERSE_BACKFILL_YEARS", 3),
		BackfillBatchSize:    intEnv("UNIVERSE_BACKFILL_BATCH_SIZE", 200),
		BackfillConcurrency:  intEnv("UNIVERSE_BACKFILL_CONCURRENCY", 4),
		BackfillClaimLease:   durationEnv("UNIVERSE_BACKFILL_CLAIM_LEASE", 15*time.Minute),
		BackfillMaxAttempts:  intEnv("UNIVERSE_BACKFILL_MAX_ATTEMPTS", 3),
		BackfillIdleInterval: durationEnv("UNIVERSE_BACKFILL_IDLE_INTERVAL", time.Hour),

		RequestsPerSecond: floatEnv("UNIVERSE_YAHOO_REQUESTS_PER_SEC", 2.0),
		RequestBurst:      intEnv("UNIVERSE_YAHOO_BURST", 1),
		RequestTimeout:    durationEnv("UNIVERSE_YAHOO_TIMEOUT", 30*time.Second),
		RequestMaxRetries: intEnv("UNIVERSE_YAHOO_MAX_RETRIES", 3),
		BackoffBase:       durationEnv("UNIVERSE_YAHOO_BACKOFF_BASE", 2*time.Second),
		BackoffMax:        durationEnv("UNIVERSE_YAHOO_BACKOFF_MAX", 60*time.Second),

		DailyBarsInterval:     durationEnv("UNIVERSE_DAILY_BARS_INTERVAL", 24*time.Hour),
		DailyBarsLookbackDays: intEnv("UNIVERSE_DAILY_BARS_LOOKBACK_DAYS", 7),
	}, nil
}
