package momentum

import (
	"strings"
	"testing"
)

// gateFeatures builds a Features that passes every gate in the given bucket, so
// each test can break exactly one thing. A helper that starts from a PASSING
// baseline is deliberate: starting from zero values would make every test pass
// for the wrong reason.
func gateFeatures(bucket Bucket) *Features {
	f := &Features{BarsAvailable: 300}
	switch bucket {
	case BucketPenny:
		f.Close = ptr(1.00)
		f.ChangePct = ptr(15.0)   // inside 10..40
		f.RVol20 = ptr(5.0)       // >= 4.0
		f.DollarVolume = ptr(3e6) // >= 2M
	default:
		f.Close = ptr(50.0)
		f.ChangePct = ptr(12.0)    // inside 8..25
		f.RVol20 = ptr(4.0)        // >= 3.0
		f.DollarVolume = ptr(10e6) // >= 5M
	}
	return f
}

func marketCap(v float64) GateInput { return GateInput{MarketCap: ptr(v)} }

func hasFailure(r GateResult, want string) bool {
	for _, f := range r.Failures {
		if f == want {
			return true
		}
	}
	return false
}

// ─── Bucketing ─────────────────────────────────────────────────────────────────

func TestAssignBucket_ImplementsTheSection32Bands(t *testing.T) {
	cfg := DefaultGateConfig()
	cases := []struct {
		close  float64
		bucket Bucket
		ok     bool
		why    string
	}{
		{0.29, "", false, "below the $0.30 floor: tick artefacts and reverse-split noise"},
		{0.30, BucketPenny, true, "the floor itself is inclusive"},
		{1.99, BucketPenny, true, "just under the $2 boundary"},
		{2.00, BucketMarket, true, "$2.00 is market, not penny — the boundary belongs to exactly one bucket"},
		{5000, BucketMarket, true, "market bucket is unbounded above"},
	}
	for _, c := range cases {
		got, ok := AssignBucket(ptr(c.close), cfg)
		if ok != c.ok || got != c.bucket {
			t.Errorf("close %.2f -> (%q,%v), want (%q,%v): %s", c.close, got, ok, c.bucket, c.ok, c.why)
		}
	}
	if _, ok := AssignBucket(nil, cfg); ok {
		t.Error("a nil close must not be bucketed")
	}
}

// A price in neither bucket is a distinct outcome from a missing price: one is a
// deliberate scope decision, the other an ingestion failure, and collapsing them
// would hide how much of the universe the $0.30 floor removes.
func TestEvaluateGates_DistinguishesUnbucketableFromMissingPrice(t *testing.T) {
	cfg := DefaultGateConfig()

	sub30 := gateFeatures(BucketPenny)
	sub30.Close = ptr(0.15)
	r := EvaluateGates(sub30, marketCap(50e6), cfg)
	if r.Passed || r.Bucketed {
		t.Error("a sub-$0.30 name must not pass or be bucketed")
	}
	if !hasFailure(r, GateCloseBelowFloor) {
		t.Errorf("want %s, got %v", GateCloseBelowFloor, r.Failures)
	}
	if hasFailure(r, GateNoClose) {
		t.Error("a present-but-low price must not be recorded as a null price")
	}

	noPrice := gateFeatures(BucketPenny)
	noPrice.Close = nil
	r = EvaluateGates(noPrice, marketCap(50e6), cfg)
	if !hasFailure(r, GateNoClose) {
		t.Errorf("want %s, got %v", GateNoClose, r.Failures)
	}
}

// ─── The change_pct ceiling, which is the strategy ─────────────────────────────

// §3.2: "A stock up +60% today is not a Phase 1 candidate — it is already gone.
// Do not remove this bound." This test exists to make removing it fail loudly.
func TestEvaluateGates_RejectsAlreadyExtendedMoves(t *testing.T) {
	cfg := DefaultGateConfig()

	for _, c := range []struct {
		bucket Bucket
		change float64
		pass   bool
	}{
		{BucketMarket, 7.9, false},  // below the +8% floor
		{BucketMarket, 8.0, true},   // the floor is inclusive
		{BucketMarket, 25.0, true},  // the ceiling is inclusive
		{BucketMarket, 25.1, false}, // over the ceiling
		{BucketMarket, 60.0, false}, // the case the spec calls out by name
		{BucketPenny, 9.9, false},
		{BucketPenny, 10.0, true},
		{BucketPenny, 40.0, true},
		{BucketPenny, 40.1, false},
	} {
		f := gateFeatures(c.bucket)
		f.ChangePct = ptr(c.change)
		cap := 5e9
		if c.bucket == BucketPenny {
			cap = 50e6
		}
		r := EvaluateGates(f, marketCap(cap), cfg)
		if r.Passed != c.pass {
			t.Errorf("%s bucket, change_pct %.1f: passed=%v want %v (failures %v)",
				c.bucket, c.change, r.Passed, c.pass, r.Failures)
		}
		if c.change > 25 && c.bucket == BucketMarket && !hasFailure(r, GateChangeTooHigh) {
			t.Errorf("change_pct %.1f should record %s", c.change, GateChangeTooHigh)
		}
	}
}

// ─── Null inputs never pass ────────────────────────────────────────────────────

// §3.4 forbids substituting 0 or 1 for a missing rvol_20, and the same logic
// applies to every gate input: an absent measurement is not a passing one. Each
// null gets its own reason so "we lack data" stays separable from "the data says
// no" — which is what decides whether a thin candidate set is a market condition
// or an ingestion bug.
func TestEvaluateGates_NullInputsFailWithTheirOwnReason(t *testing.T) {
	cfg := DefaultGateConfig()
	cases := []struct {
		name   string
		mutate func(*Features)
		reason string
	}{
		{"change_pct", func(f *Features) { f.ChangePct = nil }, GateNoChangePct},
		{"rvol_20", func(f *Features) { f.RVol20 = nil }, GateNoRVol},
		{"dollar_volume", func(f *Features) { f.DollarVolume = nil }, GateNoDollarVol},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := gateFeatures(BucketMarket)
			c.mutate(f)
			r := EvaluateGates(f, marketCap(5e9), cfg)
			if r.Passed {
				t.Errorf("a null %s must not pass", c.name)
			}
			if !hasFailure(r, c.reason) {
				t.Errorf("want reason %s, got %v", c.reason, r.Failures)
			}
		})
	}

	// A missing market cap with no proxy cannot be evaluated, so it cannot pass.
	f := gateFeatures(BucketMarket)
	r := EvaluateGates(f, GateInput{MarketCap: nil}, cfg)
	if r.Passed {
		t.Error("a null market cap with no proxy must not pass")
	}
	if !hasFailure(r, GateMarketCapUnavailable) {
		t.Errorf("want %s, got %v", GateMarketCapUnavailable, r.Failures)
	}
	if hasFailure(r, GateMarketCapNull) {
		t.Error("a total absence must not be recorded as proxy provenance — that is what let it pass before")
	}
}

// §3.2 routes market cap through §3.9's proxy when Finnhub is null, so a
// micro-cap with a data gap is not silently dropped — but market_cap_null stays
// recorded so the proxy's contribution to the candidate set is measurable.
func TestEvaluateGates_ProxiedMarketCapPassesButStaysRecorded(t *testing.T) {
	cfg := DefaultGateConfig()
	f := gateFeatures(BucketPenny)
	r := EvaluateGates(f, GateInput{MarketCap: ptr(80e6), MarketCapIsProxy: true}, cfg)

	if !r.Passed {
		t.Errorf("a proxied market cap inside the band must pass: %v", r.Failures)
	}
	if !hasFailure(r, GateMarketCapNull) {
		t.Errorf("provenance lost: want %s recorded even on a pass, got %v", GateMarketCapNull, r.Failures)
	}
	if !r.MarketCapWasProxy {
		t.Error("MarketCapWasProxy should make the stored row self-describing")
	}
}

// ─── Every failure is collected, not just the first ────────────────────────────

// Short-circuiting would make the pilot's diagnostics useless: "what would
// relaxing the dollar-volume floor buy us" is unanswerable if a symbol failing
// both RVOL and liquidity is only recorded against RVOL.
func TestEvaluateGates_CollectsAllFailuresNotJustTheFirst(t *testing.T) {
	cfg := DefaultGateConfig()
	f := gateFeatures(BucketMarket)
	f.ChangePct = ptr(2.0)    // too low
	f.RVol20 = ptr(1.0)       // too low
	f.DollarVolume = ptr(1e6) // too low
	f.BarsAvailable = 100     // too short

	r := EvaluateGates(f, marketCap(50e9), cfg) // cap also too high
	for _, want := range []string{
		GateChangeTooLow, GateRVolTooLow, GateDollarVolTooLow, GateShortHistory, GateMarketCapTooHigh,
	} {
		if !hasFailure(r, want) {
			t.Errorf("missing %s from %v", want, r.Failures)
		}
	}
	if len(r.Failures) < 5 {
		t.Errorf("got %d failures, want >=5: %v", len(r.Failures), r.Failures)
	}
	// Sorted, because the string is persisted and compared across runs.
	if got := r.FailureString(); got != strings.Join(r.Failures, ",") {
		t.Errorf("FailureString mismatch: %q", got)
	}
	for i := 1; i < len(r.Failures); i++ {
		if r.Failures[i-1] > r.Failures[i] {
			t.Errorf("failures not sorted: %v", r.Failures)
			break
		}
	}
}

// A symbol can be universe-eligible while its backfill is incomplete — exactly
// the state a partially drained backfill leaves — so history is re-checked here
// rather than trusted from §3.1.
func TestEvaluateGates_RechecksHistoryAtScanTime(t *testing.T) {
	cfg := DefaultGateConfig()
	f := gateFeatures(BucketMarket)
	f.BarsAvailable = 251
	r := EvaluateGates(f, marketCap(5e9), cfg)
	if r.Passed {
		t.Error("251 bars must fail the 252-bar gate")
	}
	if !hasFailure(r, GateShortHistory) {
		t.Errorf("want %s, got %v", GateShortHistory, r.Failures)
	}

	f.BarsAvailable = 252
	if r := EvaluateGates(f, marketCap(5e9), cfg); !r.Passed {
		t.Errorf("252 bars is exactly the minimum and must pass: %v", r.Failures)
	}
}

// ─── Bucket thresholds genuinely differ ────────────────────────────────────────

// The two buckets differ only in thresholds, and this checks that the
// difference is real: the same RVOL and liquidity that pass in the market bucket
// must fail in the penny bucket, which demands more of both.
func TestEvaluateGates_PennyBucketIsStricterOnVolumeAndLiquidity(t *testing.T) {
	cfg := DefaultGateConfig()

	f := gateFeatures(BucketPenny)
	f.RVol20 = ptr(3.5) // passes market's 3.0, fails penny's 4.0
	r := EvaluateGates(f, marketCap(50e6), cfg)
	if r.Passed {
		t.Error("rvol 3.5 must fail the penny bucket's 4.0 floor")
	}
	if !hasFailure(r, GateRVolTooLow) {
		t.Errorf("want %s, got %v", GateRVolTooLow, r.Failures)
	}

	// And the market bucket accepts it at a market price.
	g := gateFeatures(BucketMarket)
	g.RVol20 = ptr(3.5)
	if r := EvaluateGates(g, marketCap(5e9), cfg); !r.Passed {
		t.Errorf("rvol 3.5 should pass the market bucket's 3.0 floor: %v", r.Failures)
	}
}

func TestEvaluateGates_MarketCapBandsPerBucket(t *testing.T) {
	cfg := DefaultGateConfig()

	// Market bucket: $300M - $10B.
	for _, c := range []struct {
		cap  float64
		pass bool
	}{{299e6, false}, {300e6, true}, {10e9, true}, {10.1e9, false}} {
		f := gateFeatures(BucketMarket)
		if r := EvaluateGates(f, marketCap(c.cap), cfg); r.Passed != c.pass {
			t.Errorf("market bucket cap %.0f: passed=%v want %v (%v)", c.cap, r.Passed, c.pass, r.Failures)
		}
	}

	// Penny bucket has NO lower bound (§3.2) and a $300M ceiling.
	for _, c := range []struct {
		cap  float64
		pass bool
	}{{1e6, true}, {300e6, true}, {301e6, false}} {
		f := gateFeatures(BucketPenny)
		if r := EvaluateGates(f, marketCap(c.cap), cfg); r.Passed != c.pass {
			t.Errorf("penny bucket cap %.0f: passed=%v want %v (%v)", c.cap, r.Passed, c.pass, r.Failures)
		}
	}
}

func TestEvaluateGates_NilFeaturesNeverPasses(t *testing.T) {
	if r := EvaluateGates(nil, marketCap(1e9), DefaultGateConfig()); r.Passed {
		t.Error("nil features must not pass")
	}
}

// ─── Stats ─────────────────────────────────────────────────────────────────────

// The stats exist so the candidate count is explainable without a query. Because
// failures are collected rather than short-circuited, counts intentionally sum
// to more than the number of rejected symbols.
func TestGateStats_ExplainsTheCandidateCount(t *testing.T) {
	cfg := DefaultGateConfig()
	st := NewGateStats()

	st.Add(EvaluateGates(gateFeatures(BucketMarket), marketCap(5e9), cfg)) // pass
	st.Add(EvaluateGates(gateFeatures(BucketPenny), marketCap(50e6), cfg)) // pass

	low := gateFeatures(BucketMarket)
	low.RVol20 = ptr(1.0)
	low.DollarVolume = ptr(1e3)
	st.Add(EvaluateGates(low, marketCap(5e9), cfg))

	sub30 := gateFeatures(BucketPenny)
	sub30.Close = ptr(0.10)
	st.Add(EvaluateGates(sub30, marketCap(5e6), cfg))

	if st.Evaluated != 4 {
		t.Errorf("Evaluated = %d, want 4", st.Evaluated)
	}
	if st.Passed != 2 {
		t.Errorf("Passed = %d, want 2", st.Passed)
	}
	if st.PassedByBkt[BucketMarket] != 1 || st.PassedByBkt[BucketPenny] != 1 {
		t.Errorf("PassedByBkt = %v, want one of each", st.PassedByBkt)
	}
	if st.Unbucketed != 1 {
		t.Errorf("Unbucketed = %d, want 1", st.Unbucketed)
	}
	if st.FailureCounts[GateRVolTooLow] != 1 || st.FailureCounts[GateDollarVolTooLow] != 1 {
		t.Errorf("both failures of one symbol should be counted: %v", st.FailureCounts)
	}
	if top := st.TopFailures(3); len(top) == 0 {
		t.Error("TopFailures should summarise the reasons")
	}
}
