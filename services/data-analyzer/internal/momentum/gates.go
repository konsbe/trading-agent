package momentum

import (
	"fmt"
	"sort"
	"strings"
)

// §3.2 hard gates: the candidate filter.
//
// Scoring only RANKS WITHIN the candidate set. A gate failure means excluded,
// not low-scored — so this file decides membership and scores.go decides order,
// and the two must never be conflated. A "gate" implemented as a score penalty
// would let a symbol that fails a liquidity floor still surface at rank 3.
//
// Every threshold is configuration, not a constant (§3.2), because the pilot's
// whole purpose is to measure whether these numbers are right.

// Bucket is the §3.2 price bucket a candidate is judged in. The two buckets
// differ ONLY in thresholds, never in logic.
type Bucket string

const (
	BucketMarket Bucket = "market"
	BucketPenny  Bucket = "penny"
)

// BucketThresholds is one bucket's gate configuration.
type BucketThresholds struct {
	MinClose float64
	// MaxClose bounds the bucket from above. 0 means unbounded, which is how the
	// market bucket is expressed.
	MaxClose float64

	// MinMarketCap / MaxMarketCap in dollars. 0 means unbounded on that side;
	// the penny bucket has no lower bound (§3.2).
	MinMarketCap float64
	MaxMarketCap float64

	MinChangePct float64
	// MaxChangePct is load-bearing, not a safety rail. §3.2: "A stock up +60%
	// today is not a Phase 1 candidate — it is already gone. Do not remove this
	// bound." The thesis is entry at +8-15% on a confirmed move.
	MaxChangePct float64

	MinRVol20      float64
	MinDollarVolPS float64 // dollars
}

// GateConfig holds both buckets plus the shared history requirement.
type GateConfig struct {
	Market BucketThresholds
	Penny  BucketThresholds

	// MinBars is §3.1's 252-bar history requirement, re-checked here. It is a
	// universe filter at symbol-list load AND a scan-time gate, because a symbol
	// can be eligible while its stored history is still short.
	MinBars int
}

// DefaultGateConfig returns §3.2's table verbatim.
func DefaultGateConfig() GateConfig {
	return GateConfig{
		Market: BucketThresholds{
			MinClose:       2.00,
			MaxClose:       0, // unbounded
			MinMarketCap:   300e6,
			MaxMarketCap:   10e9,
			MinChangePct:   8,
			MaxChangePct:   25,
			MinRVol20:      3.0,
			MinDollarVolPS: 5e6,
		},
		Penny: BucketThresholds{
			MinClose: 0.30, // §3.2: sub-$0.30 is tick artefacts and reverse-split noise
			MaxClose: 2.00,
			// No lower market-cap bound for penny names (§3.2).
			MinMarketCap:   0,
			MaxMarketCap:   300e6,
			MinChangePct:   10,
			MaxChangePct:   40,
			MinRVol20:      4.0,
			MinDollarVolPS: 2e6,
		},
		MinBars: 252,
	}
}

// Thresholds returns the configuration for a bucket.
func (c GateConfig) Thresholds(b Bucket) BucketThresholds {
	if b == BucketPenny {
		return c.Penny
	}
	return c.Market
}

// Gate failure reasons. Stable strings: they are persisted in
// momentum_features.gate_failures and are the only record of why the candidate
// set is the size it is. Renaming one silently breaks every historical
// comparison, so treat them as a schema.
const (
	GateNoClose         = "close_null"
	GateCloseBelowFloor = "close_below_bucket_floor"
	GateCloseAboveCeil  = "close_above_bucket_ceiling"
	GateShortHistory    = "insufficient_history"
	GateNoChangePct     = "change_pct_null"
	GateChangeTooLow    = "change_pct_below_min"
	GateChangeTooHigh   = "change_pct_above_max"
	GateNoRVol          = "rvol_20_null"
	GateRVolTooLow      = "rvol_20_below_min"
	GateNoDollarVol     = "dollar_volume_null"
	GateDollarVolTooLow = "dollar_volume_below_min"
	// GateMarketCapNull is PROVENANCE, not a rejection: it records that the
	// market cap came from §3.9's proxy rather than Finnhub, which §3.2 requires
	// so the proxy's contribution to the candidate set stays measurable. A symbol
	// can pass carrying this reason.
	GateMarketCapNull = "market_cap_null"

	// GateMarketCapUnavailable is the rejection: neither Finnhub nor the proxy
	// produced a value, so the gate cannot be evaluated and therefore cannot
	// pass. Kept separate from GateMarketCapNull because an earlier version used
	// one string for both and a symbol with NO market cap passed the gate — the
	// provenance marker was excluded from the real-failure check, and the
	// absence inherited that exemption.
	GateMarketCapUnavailable = "market_cap_unavailable"
	GateMarketCapTooLow      = "market_cap_below_min"
	GateMarketCapTooHigh     = "market_cap_above_max"
	GateUnbucketable         = "no_bucket_for_price"
)

// GateInput is everything §3.2 needs beyond the computed Features: the
// fundamentals that do not come from bars.
type GateInput struct {
	// MarketCap in dollars, nil when unavailable from both Finnhub and the §3.9
	// proxy.
	MarketCap *float64

	// MarketCapIsProxy records that MarketCap came from §3.9's
	// shares_outstanding x close estimate rather than from Finnhub.
	//
	// The gate treats the estimate as authoritative (§3.2) so micro-caps with a
	// Finnhub gap are not silently dropped, but the provenance is carried through
	// so the proxy's contribution to the candidate set stays measurable. Without
	// this the pilot could not answer "how many candidates exist only because we
	// guessed their market cap".
	MarketCapIsProxy bool
}

// GateResult is the verdict for one symbol on one day.
type GateResult struct {
	// Bucket is the bucket the symbol was JUDGED IN, recorded even on failure so
	// a rejection can be read against the right thresholds.
	Bucket Bucket

	// Bucketed is false when the close fits neither bucket (below $0.30, or
	// null). Failures is then the only meaningful field.
	Bucketed bool

	Passed bool

	// Failures lists every gate that failed, not just the first.
	//
	// Short-circuiting on the first failure would make the pilot's diagnostics
	// useless: "how many candidates does the dollar-volume floor cost us" is
	// unanswerable if a symbol that fails both RVOL and liquidity is recorded
	// only against RVOL. Sorted for stable persistence and comparison.
	Failures []string

	// MarketCapWasProxy mirrors the input, so a stored row is self-describing.
	MarketCapWasProxy bool
}

// FailureString renders the failures for logging and for the gate_failures
// column.
func (r GateResult) FailureString() string {
	if len(r.Failures) == 0 {
		return ""
	}
	return strings.Join(r.Failures, ",")
}

// AssignBucket maps a close to its §3.2 bucket.
//
// Returns false when the price fits neither, which is a real outcome rather than
// an error: sub-$0.30 names are deliberately out of scope, and conflating them
// with a missing price would hide how much of the universe the floor removes.
func AssignBucket(close *float64, cfg GateConfig) (Bucket, bool) {
	if close == nil {
		return "", false
	}
	c := *close
	if c >= cfg.Penny.MinClose && c < cfg.Penny.MaxClose {
		return BucketPenny, true
	}
	if c >= cfg.Market.MinClose {
		return BucketMarket, true
	}
	return "", false
}

// EvaluateGates applies §3.2 to one symbol's features.
//
// A nil input NEVER passes a gate. §3.4 is explicit that a missing rvol_20 must
// not be substituted with 0 or 1, and the same reasoning applies to every gate
// input: an absent measurement is not a passing measurement. Each null is
// recorded with its own reason so "we lack the data" is distinguishable from
// "the data says no" — the distinction that decides whether a thin candidate set
// is a market condition or an ingestion bug.
func EvaluateGates(f *Features, in GateInput, cfg GateConfig) GateResult {
	res := GateResult{MarketCapWasProxy: in.MarketCapIsProxy}

	if f == nil {
		res.Failures = []string{GateNoClose, GateUnbucketable}
		return res
	}

	bucket, ok := AssignBucket(f.Close, cfg)
	res.Bucket, res.Bucketed = bucket, ok
	if !ok {
		if f.Close == nil {
			res.Failures = append(res.Failures, GateNoClose)
		} else {
			res.Failures = append(res.Failures, GateCloseBelowFloor)
		}
		res.Failures = append(res.Failures, GateUnbucketable)
		sort.Strings(res.Failures)
		return res
	}
	t := cfg.Thresholds(bucket)

	// Price bounds. Bucketing already guarantees these, but they are re-checked
	// so a future change to AssignBucket cannot silently widen a bucket.
	if *f.Close < t.MinClose {
		res.Failures = append(res.Failures, GateCloseBelowFloor)
	}
	if t.MaxClose > 0 && *f.Close >= t.MaxClose {
		res.Failures = append(res.Failures, GateCloseAboveCeil)
	}

	// History. Checked against stored bars rather than trusting universe
	// eligibility, because a symbol can be eligible while its backfill is
	// incomplete — exactly the state a partially drained backfill leaves.
	if f.BarsAvailable < cfg.MinBars {
		res.Failures = append(res.Failures, GateShortHistory)
	}

	switch {
	case f.ChangePct == nil:
		res.Failures = append(res.Failures, GateNoChangePct)
	case *f.ChangePct < t.MinChangePct:
		res.Failures = append(res.Failures, GateChangeTooLow)
	case *f.ChangePct > t.MaxChangePct:
		// The upper bound is the strategy, not a guard rail (§3.2).
		res.Failures = append(res.Failures, GateChangeTooHigh)
	}

	switch {
	case f.RVol20 == nil:
		res.Failures = append(res.Failures, GateNoRVol)
	case *f.RVol20 < t.MinRVol20:
		res.Failures = append(res.Failures, GateRVolTooLow)
	}

	switch {
	case f.DollarVolume == nil:
		res.Failures = append(res.Failures, GateNoDollarVol)
	case *f.DollarVolume < t.MinDollarVolPS:
		res.Failures = append(res.Failures, GateDollarVolTooLow)
	}

	// Market cap. §3.2 routes through §3.9's estimate when Finnhub is null, and
	// keeps market_cap_null recorded even when the proxy carries the gate, so the
	// proxy's contribution stays measurable.
	if in.MarketCapIsProxy {
		res.Failures = append(res.Failures, GateMarketCapNull)
	}
	switch {
	case in.MarketCap == nil:
		// No Finnhub value and no usable proxy: the gate cannot be evaluated, so
		// it cannot pass. A distinct reason from the provenance marker above.
		res.Failures = append(res.Failures, GateMarketCapUnavailable)
	case t.MinMarketCap > 0 && *in.MarketCap < t.MinMarketCap:
		res.Failures = append(res.Failures, GateMarketCapTooLow)
	case t.MaxMarketCap > 0 && *in.MarketCap > t.MaxMarketCap:
		res.Failures = append(res.Failures, GateMarketCapTooHigh)
	}

	// market_cap_null is provenance, not a rejection, when the proxy supplied a
	// value inside the band. Anything else in the list is a real failure.
	res.Passed = !hasRealFailure(res.Failures)
	sort.Strings(res.Failures)
	return res
}

// hasRealFailure reports whether any recorded reason actually excludes the
// symbol. GateMarketCapNull alone is provenance for a proxied value.
func hasRealFailure(failures []string) bool {
	for _, f := range failures {
		if f != GateMarketCapNull {
			return true
		}
	}
	return false
}

// GateStats summarises a scan, so the candidate set's size is explainable.
type GateStats struct {
	Evaluated   int
	Unbucketed  int
	Passed      int
	PassedByBkt map[Bucket]int

	// FailureCounts counts each reason across all symbols. Because failures are
	// collected rather than short-circuited, these sum to more than the number of
	// rejected symbols — which is the point: it answers "what would relaxing this
	// one threshold buy us".
	FailureCounts map[string]int
}

func NewGateStats() *GateStats {
	return &GateStats{
		PassedByBkt:   map[Bucket]int{},
		FailureCounts: map[string]int{},
	}
}

func (s *GateStats) Add(r GateResult) {
	s.Evaluated++
	if !r.Bucketed {
		s.Unbucketed++
	}
	if r.Passed {
		s.Passed++
		s.PassedByBkt[r.Bucket]++
	}
	for _, f := range r.Failures {
		s.FailureCounts[f]++
	}
}

// TopFailures returns the n most common reasons, for a log line that explains
// the candidate count without a query.
func (s *GateStats) TopFailures(n int) []string {
	type kv struct {
		k string
		v int
	}
	all := make([]kv, 0, len(s.FailureCounts))
	for k, v := range s.FailureCounts {
		all = append(all, kv{k, v})
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].v != all[j].v {
			return all[i].v > all[j].v
		}
		return all[i].k < all[j].k
	})
	if n > len(all) {
		n = len(all)
	}
	out := make([]string, 0, n)
	for _, e := range all[:n] {
		out = append(out, fmt.Sprintf("%s=%d", e.k, e.v))
	}
	return out
}
