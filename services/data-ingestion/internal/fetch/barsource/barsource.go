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
	"strings"
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

// RedactSecrets removes credential query-parameter values from a string.
//
// This exists because of a real leak, not as a precaution. Go's *url.Error —
// returned by http.Client.Do for every transport failure — embeds the full
// request URL, and providers that authenticate by query parameter therefore put
// the API key inside the error text. That error then travels wherever errors go:
// this repo wrote 23 of them into universe_symbols.backfill_last_error, and the
// same string is logged.
//
// Every wrapper around a transport error must pass through here. Redacting at
// the point of use rather than trusting call sites is deliberate: the leak is
// invisible in review because the format string mentions only %w.
func RedactSecrets(s string) string {
	for _, k := range secretParams {
		s = redactParam(s, k)
	}
	return s
}

// secretParams are the query keys whose values must never appear in an error.
// Add to this list when a provider is added, not after its key shows up in a log.
var secretParams = []string{"apikey", "api_key", "token", "apiKey", "key", "access_key"}

// redactParam replaces `name=<value>` with `name=REDACTED`, matching the key
// case-insensitively and stopping at the first URL or whitespace delimiter.
func redactParam(s, name string) string {
	lower := strings.ToLower(s)
	needle := strings.ToLower(name) + "="
	var b strings.Builder
	i := 0
	for {
		j := strings.Index(lower[i:], needle)
		if j < 0 {
			b.WriteString(s[i:])
			return b.String()
		}
		start := i + j
		// Only treat it as a parameter when preceded by a delimiter, so
		// "mytoken=" does not match "token=".
		if start > 0 {
			switch s[start-1] {
			case '?', '&', ';', ' ', '"', '\'', '/':
			default:
				b.WriteString(s[i : start+len(needle)])
				i = start + len(needle)
				continue
			}
		}
		b.WriteString(s[i:start])
		b.WriteString(s[start : start+len(needle)])
		b.WriteString("REDACTED")
		end := start + len(needle)
		for end < len(s) && !strings.ContainsRune("&\" '\n\t", rune(s[end])) {
			end++
		}
		i = end
	}
}
