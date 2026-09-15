package ratelimit

import (
	"context"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

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
