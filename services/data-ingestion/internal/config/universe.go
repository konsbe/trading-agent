package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

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

	// MinBarsHistory is §3.1's / §3.2's 250-bar minimum. It is recorded and
	// reported here but NOT used to set is_eligible — see the note on
	// store.EligibleUniverse for why that would be circular.
	MinBarsHistory int

	// BarInterval / BarSource identify the rows in equity_ohlcv that count as
	// this scanner's daily bars (§7: reuse equity_ohlcv, source='yahoo').
	BarInterval string
	BarSource   string

	// EnableSymbols / EnableFundamentals allow either pass to be turned off
	// independently, following the FUNDAMENTAL_ENABLE_* convention.
	EnableSymbols      bool
	EnableFundamentals bool
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

		MinBarsHistory: intEnv("UNIVERSE_MIN_BARS_HISTORY", 250),

		BarInterval: env("UNIVERSE_BAR_INTERVAL", "1Day"),
		BarSource:   env("UNIVERSE_BAR_SOURCE", "yahoo"),

		EnableSymbols:      env("UNIVERSE_ENABLE_SYMBOLS", "true") == "true",
		EnableFundamentals: env("UNIVERSE_ENABLE_FUNDAMENTALS", "true") == "true",
	}, nil
}
