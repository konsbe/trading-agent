package ratelimit

import (
	"context"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/time/rate"

	"github.com/konsbe/trading-agent/services/data-ingestion/internal/fetch/finnhub"
)

// FinnhubBudgetKey names the Finnhub free-tier quota. Every worker using
// FINNHUB_API_KEY must pass this same key or they are not sharing anything.
const FinnhubBudgetKey = "finnhub"

// SharedFinnhub builds the cross-process limiter for the Finnhub budget.
//
// Intended use is a single line at each worker's client construction:
//
//	fh := finnhub.NewWithLimiter(cfg.FinnhubKey, ratelimit.SharedFinnhub(ctx, pool, log))
//
// It returns nil — meaning "use the client's own in-process bucket" — when the
// feature is disabled or cannot be set up. Returning nil rather than an error is
// deliberate: coordination is an optimisation over the status quo, and a worker
// that cannot coordinate must still run at its old rate rather than fail to
// start. Every outcome is logged so the choice is never silent.
func SharedFinnhub(ctx context.Context, pool *pgxpool.Pool, log *slog.Logger) finnhub.Limiter {
	if log == nil {
		log = slog.Default()
	}
	if !boolEnv("FINNHUB_SHARED_RATE_ENABLE", true) {
		log.Info("shared Finnhub rate limiting disabled; using this process's own bucket",
			"env", "FINNHUB_SHARED_RATE_ENABLE=false")
		return nil
	}
	if pool == nil {
		log.Warn("no database pool available for shared Finnhub rate limiting; using this process's own bucket")
		return nil
	}

	perSec := floatEnv("FINNHUB_RATE_PER_SEC", 1.0) // free tier: 60 req/min
	burst := floatEnv("FINNHUB_RATE_BURST", 2.0)

	s, err := NewShared(pool,
		Budget{Key: FinnhubBudgetKey, RefillPerSec: perSec, Burst: burst},
		// The fallback is the very bucket this client would otherwise have used,
		// so degradation lands on known-good behaviour rather than something new.
		finnhub.DefaultLimiter(),
		Options{
			AcquireTimeout: durationEnv("SHARED_RATE_ACQUIRE_TIMEOUT", 250*time.Millisecond),
			MaxSleep:       durationEnv("SHARED_RATE_MAX_SLEEP", 5*time.Second),
			WarnEvery:      durationEnv("SHARED_RATE_WARN_EVERY", 30*time.Second),
		},
		log)
	if err != nil {
		log.Warn("could not construct the shared Finnhub limiter; using this process's own bucket",
			"err", err)
		return nil
	}

	// Register the budget so whichever worker starts first creates the row and
	// the rest reconcile. Failure here is not fatal: the limiter degrades to the
	// fallback per request and says so.
	ectx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := s.EnsureBudget(ectx); err != nil {
		log.Warn("could not register the shared Finnhub budget; the limiter will degrade to this process's own bucket until the database is reachable",
			"err", err)
		return s
	}

	log.Info("shared Finnhub rate limiting active",
		"budget", FinnhubBudgetKey,
		"per_sec", perSec,
		"burst", burst,
		"note", "this budget is shared by every worker using FINNHUB_API_KEY")
	return s
}

// Local env helpers. Duplicated rather than imported from internal/config
// because config imports nothing and this package must not create a cycle
// through it.

func boolEnv(key string, def bool) bool {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}

func floatEnv(key string, def float64) float64 {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return def
	}
	return f
}

func durationEnv(key string, def time.Duration) time.Duration {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return def
	}
	return d
}

// TiingoBudgetKey names the Tiingo quota.
const TiingoBudgetKey = "tiingo"

// SharedTiingo builds the cross-process limiter for Tiingo.
//
// CORRECTED after a 429 storm. The previous version of this function paced at
// 1.5 req/sec and described itself as "politeness only", on the reasoning that
// Tiingo's real constraint is a monthly unique-symbol count that a rate limiter
// cannot express. Two of those three claims were wrong:
//
//	50 requests / hour   HARD, resets on the clock hour (measured: blocked
//	                     05:54 UTC, recovered 06:01 UTC — fixed clock, not a
//	                     rolling window)
//	1,000 requests / day resets at midnight EST — perfectly expressible
//	500 unique symbols / month   this is the part api_rate_budget cannot express
//
// Pacing at 1.5/sec spent the hourly allowance in under a minute and then
// failed 69 symbols, each burning 4 attempts on a refusal that could not clear
// for the rest of the hour.
//
// So the limiter now carries BOTH an hourly-equivalent rate and the daily
// ceiling. 50/hour is 0.0139 req/sec, which is the true throughput: a
// 450-symbol backfill takes ~9 hours and no setting here can shorten it.
//
// Only the unique-symbol allowance still lives elsewhere, in the
// subset-selection size assertion (store.SubsetSizeError).
func SharedTiingo(ctx context.Context, pool *pgxpool.Pool, log *slog.Logger) barsourceLimiter {
	if log == nil {
		log = slog.Default()
	}
	if !boolEnv("TIINGO_SHARED_RATE_ENABLE", true) {
		log.Info("shared Tiingo rate limiting disabled; using this process's own bucket")
		return nil
	}
	if pool == nil {
		log.Warn("no database pool for shared Tiingo rate limiting; using this process's own bucket")
		return nil
	}

	// 50/hour = 0.013889/sec. The default trims to 0.0138 so clock skew against
	// Tiingo's own hour boundary cannot push the last request of an hour over.
	perSec := floatEnv("TIINGO_RATE_PER_SEC", 0.0138)
	burst := floatEnv("TIINGO_RATE_BURST", 1.0)
	daily := floatEnv("TIINGO_RATE_DAILY_LIMIT", 1000)

	// Tiingo's spec: "daily requests (reset at midnight EST)".
	b := Budget{Key: TiingoBudgetKey, RefillPerSec: perSec, Burst: burst,
		DailyResetTZ: env("TIINGO_RATE_RESET_TZ", "EST")}
	if daily > 0 {
		b.DailyLimit = &daily
	}

	s, err := NewShared(pool, b,
		localBucket(perSec, burst),
		Options{
			AcquireTimeout: durationEnv("SHARED_RATE_ACQUIRE_TIMEOUT", 250*time.Millisecond),
			MaxSleep:       durationEnv("SHARED_RATE_MAX_SLEEP", 5*time.Second),
			WarnEvery:      durationEnv("SHARED_RATE_WARN_EVERY", 30*time.Second),
		}, log)
	if err != nil {
		log.Warn("could not construct the shared Tiingo limiter; using this process's own bucket", "err", err)
		return nil
	}
	ectx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := s.EnsureBudget(ectx); err != nil {
		log.Warn("could not register the shared Tiingo budget; the limiter will degrade to a local bucket until the database is reachable", "err", err)
		return s
	}
	log.Info("shared Tiingo rate limiting active",
		"budget", TiingoBudgetKey, "per_sec", perSec, "burst", burst, "daily_limit", daily,
		"effective_per_hour", perSec*3600,
		"note", "50 req/clock-hour and 1,000/day are enforced here; the 500 unique-symbols/month cap is enforced by the subset size assertion")
	return s
}

// TwelveDataBudgetKey names the Twelve Data quota.
const TwelveDataBudgetKey = "twelve_data"

// SharedTwelveData builds the cross-process limiter for Twelve Data.
//
// Unlike SharedTiingo this budget DOES carry a daily ceiling, because Twelve
// Data's limits are both expressible as rate plus daily count:
//
//	8 credits / minute   measured on this account; the 429 names the count
//	800 requests / day    published free-tier limit
//	no unique-symbol cap for US equities
//
// That is the whole reason this provider replaced Tiingo as primary. Tiingo's
// binding constraint is 50 requests per clock hour, which makes a 450-symbol
// backfill a ~9-hour job; 8/minute makes the same job ~56 minutes, and the
// absence of a unique-symbol meter removes the "have we burned the month's
// allowance on retries" question entirely.
//
// The daily ceiling matters more here than the rate: exceeding 8/minute yields a
// 429 that clears by itself within a minute, whereas exhausting 800/day strands
// the backfill until midnight. Routing that through api_rate_budget means the
// limiter reports ErrDailyQuotaExhausted — a distinguishable "come back
// tomorrow" — instead of the caller seeing a wall of per-symbol failures.
func SharedTwelveData(ctx context.Context, pool *pgxpool.Pool, log *slog.Logger) barsourceLimiter {
	if log == nil {
		log = slog.Default()
	}
	if !boolEnv("TWELVE_DATA_SHARED_RATE_ENABLE", true) {
		log.Info("shared Twelve Data rate limiting disabled; using this process's own bucket")
		return nil
	}
	if pool == nil {
		log.Warn("no database pool for shared Twelve Data rate limiting; using this process's own bucket")
		return nil
	}

	// 0.1333/s is 8/minute exactly; the default backs off to 0.125/s so clock
	// skew against Twelve Data's own minute boundary cannot push a burst over.
	perSec := floatEnv("TWELVE_DATA_RATE_PER_SEC", 0.125)
	burst := floatEnv("TWELVE_DATA_RATE_BURST", 1.0)
	daily := floatEnv("TWELVE_DATA_RATE_DAILY_LIMIT", 800)

	// Twelve Data does not document its daily reset and it has not been measured
	// here. EST is the conservative guess: if the real boundary is UTC midnight
	// we wait five extra hours before reusing the allowance, whereas the opposite
	// error overspends it.
	b := Budget{Key: TwelveDataBudgetKey, RefillPerSec: perSec, Burst: burst,
		DailyResetTZ: env("TWELVE_DATA_RATE_RESET_TZ", "EST")}
	if daily > 0 {
		b.DailyLimit = &daily
	}

	s, err := NewShared(pool, b, localBucket(perSec, burst),
		Options{
			AcquireTimeout: durationEnv("SHARED_RATE_ACQUIRE_TIMEOUT", 250*time.Millisecond),
			MaxSleep:       durationEnv("SHARED_RATE_MAX_SLEEP", 5*time.Second),
			WarnEvery:      durationEnv("SHARED_RATE_WARN_EVERY", 30*time.Second),
		}, log)
	if err != nil {
		log.Warn("could not construct the shared Twelve Data limiter; using this process's own bucket", "err", err)
		return nil
	}
	ectx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := s.EnsureBudget(ectx); err != nil {
		log.Warn("could not register the shared Twelve Data budget; the limiter will degrade to a local bucket until the database is reachable", "err", err)
		return s
	}
	log.Info("shared Twelve Data rate limiting active",
		"budget", TwelveDataBudgetKey, "per_sec", perSec, "burst", burst, "daily_limit", daily,
		"note", "8 credits/min measured, 800/day published; no unique-symbol cap")
	return s
}

// barsourceLimiter is the shape the bar fetchers accept. Declared locally so this
// package does not import fetch/barsource (which would invert the dependency).
type barsourceLimiter interface {
	Wait(ctx context.Context) error
}

// localBucket returns an in-process limiter at the given pace, used as the
// fallback so a coordination outage degrades to a known rate rather than to
// unlimited.
func localBucket(perSec float64, burst float64) barsourceLimiter {
	b := int(burst)
	if b < 1 {
		b = 1
	}
	return rate.NewLimiter(rate.Limit(perSec), b)
}

// env reads a string setting with a default.
func env(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}
