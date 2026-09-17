// Package barsource defines the contract every daily-bar provider implements,
// so the momentum scanner's backfill and daily refresh are provider-agnostic.
//
// Phase 1 uses two providers deliberately (see data_ingestion.md): Tiingo for the
// pilot subset, Twelve Data for the full-universe sweep. Their quotas fail in
// opposite directions, so the choice is configuration rather than a rewrite — and
// that only works if the callers never name a provider.
package barsource

import (
	"context"
	"errors"
	"time"

	"github.com/konsbe/trading-agent/services/data-ingestion/internal/store"
)

// ErrNoBars reports that a provider answered successfully with an empty series.
//
// Distinguished from a failure so callers can tell "this symbol has no bars in
// this window" — a delisting, a bad ticker, a listing younger than the window —
// from "the request broke". The backfill records the former as a completed
// symbol with zero bars rather than retrying it every round and spending quota
// on a certain answer.
//
// Provider packages alias their own sentinel to this one, so
// errors.Is(err, barsource.ErrNoBars) works regardless of which adapter ran.
var ErrNoBars = errors.New("barsource: no bars in range")

// Fetcher fetches daily bars for one symbol over an explicit window.
//
// Implementations must:
//   - return bars oldest-first, with Source set to their own constant so rows
//     are attributable and the scanner can filter by provider;
//   - use SPLIT AND DIVIDEND ADJUSTED prices and volume, per §3's opening line.
//     Mixing adjusted and unadjusted bars across providers would make windowed
//     features discontinuous at every corporate action, which looks like a real
//     price move rather than an artefact;
//   - wrap ErrNoBars on an empty-but-successful response;
//   - treat a not-found symbol as permanent, not retryable.
type Fetcher interface {
	FetchBarsRange(ctx context.Context, symbol, alpacaInterval string, from, to time.Time) ([]store.EquityBar, error)

	// SourceName is the equity_ohlcv.source value this fetcher stamps on every
	// bar. Exposed so callers filter on the provider's own constant rather than
	// a hand-typed literal — the spec once named a source value that existed
	// nowhere and would have matched zero rows.
	SourceName() string
}

// Limiter is the pacing contract, matching internal/ratelimit.Limiter and
// *rate.Limiter. Declared here so provider packages need not import either.
type Limiter interface {
	Wait(ctx context.Context) error
}
