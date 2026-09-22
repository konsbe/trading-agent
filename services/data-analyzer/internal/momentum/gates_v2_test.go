package momentum

import "testing"

// baseFeatures builds a symbol-day that passes every gate except whatever the
// individual test varies, so a failure names the thing under test.
func passingFeatures(t *testing.T, close float64) *Features {
	t.Helper()
	f := &Features{}
	chg, rvol, dv := 11.0, 3.0, 5_000_000.0
	c := close
	f.Close, f.ChangePct, f.RVol20, f.DollarVolume = &c, &chg, &rvol, &dv
	f.BarsAvailable = 300
	return f
}

// v2 must REJECT a symbol-day with no point-in-time market cap, not fall back
// to today's value. A fallback would restore the lookahead on exactly the rows
// where it is largest -- the oldest bars, furthest from today's share count --
// and those are the rows nobody can check.
func TestGateV2_NoPointInTimeCapIsRejectionNotFallback(t *testing.T) {
	f := passingFeatures(t, 5.0)
	todayCap := 1e9 // a perfectly good v1 value, which v2 must ignore

	cfg := DefaultGateConfig()
	cfg.Version = GateV2
	res := EvaluateGates(f, GateInput{MarketCap: &todayCap, MarketCapPIT: nil}, cfg)

	if res.Passed {
		t.Fatal("v2 passed a symbol-day with no point-in-time market cap — it fell back to today's value, which is the lookahead v2 exists to remove")
	}
	if !hasFailure(res, GateMarketCapPITUnavailable) {
		t.Errorf("failures = %v, want %s", res.Failures, GateMarketCapPITUnavailable)
	}
	// The v1 reason must NOT also appear: a today-cap was supplied, so
	// "market_cap_unavailable" would be false and would corrupt the rejection
	// histogram that the v1-vs-v2 comparison is read from.
	if hasFailure(res, GateMarketCapUnavailable) {
		t.Errorf("v2 recorded the v1 reason %s as well; the two must not be conflated in rejection counts: %v",
			GateMarketCapUnavailable, res.Failures)
	}
}

// The case that motivates the whole change: today's cap and the cap on the day
// fall on opposite sides of a band edge, so v1 and v2 reach different verdicts.
//
// Note what does NOT change. The BUCKET is assigned from the close PRICE
// (penny $0.30-$2.00, market >= $2.00), and a price is a price — it is the same
// under both versions. Market cap is a BAND CHECK WITHIN the price-determined
// bucket. So gate v2 changes whether a symbol-day PASSES, not which bucket it
// is judged in.
func TestGateV1AndV2DisagreeOnTheMarketCapBand(t *testing.T) {
	f := passingFeatures(t, 5.00) // $5 -> market bucket, band [300M, 10bn]
	// A company worth $250M on the day, worth $2bn today.
	today := 2e9
	pit := 250e6

	v1 := DefaultGateConfig()
	v1.Version = GateV1
	r1 := EvaluateGates(f, GateInput{MarketCap: &today, MarketCapPIT: &pit}, v1)

	v2 := DefaultGateConfig()
	v2.Version = GateV2
	r2 := EvaluateGates(f, GateInput{MarketCap: &today, MarketCapPIT: &pit}, v2)

	if r1.Bucket != r2.Bucket {
		t.Errorf("bucket differed (%s vs %s); bucket comes from price and must NOT move with the market-cap version",
			r1.Bucket, r2.Bucket)
	}
	if !r1.Passed {
		t.Fatalf("v1 should pass on today's $2bn inside the market band: %v", r1.Failures)
	}
	if r2.Passed {
		t.Fatal("v2 passed on a point-in-time cap of $250M, below the market bucket's $300M floor — " +
			"the version switch is not reaching the band check")
	}
	if !hasFailure(r2, GateMarketCapTooLow) {
		t.Errorf("v2 failures = %v, want %s", r2.Failures, GateMarketCapTooLow)
	}
}

// The reverse direction: a company that has SHRUNK or diluted since the setup
// is admitted by v1 and was never eligible on the day.
func TestGateV2_ExcludesWhatTodaysCapWronglyAdmits(t *testing.T) {
	f := passingFeatures(t, 5.00)
	today := 1e9 // inside the market band now
	pit := 20e9  // but $20bn on the day: above the $10bn ceiling

	cfg := DefaultGateConfig()
	cfg.Version = GateV2
	res := EvaluateGates(f, GateInput{MarketCap: &today, MarketCapPIT: &pit}, cfg)

	if res.Passed {
		t.Fatal("v2 passed a symbol-day that was $20bn on the day, above the market ceiling")
	}
	if !hasFailure(res, GateMarketCapTooHigh) {
		t.Errorf("failures = %v, want %s", res.Failures, GateMarketCapTooHigh)
	}
}

// v2 has no §3.9 proxy path. The proxy is today's share count times close, so
// it is the same leak wearing different clothes, and carrying it into v2 would
// quietly re-admit the rows v2 is meant to exclude.
func TestGateV2_IgnoresTheTodayCapProxyEntirely(t *testing.T) {
	f := passingFeatures(t, 5.0)
	proxy := 500e6

	cfg := DefaultGateConfig()
	cfg.Version = GateV2
	res := EvaluateGates(f, GateInput{
		MarketCap: &proxy, MarketCapIsProxy: true, MarketCapPIT: nil,
	}, cfg)

	if res.Passed {
		t.Fatal("v2 passed on §3.9's proxy; the proxy is built from TODAY's share count and is the same lookahead")
	}
	if hasFailure(res, GateMarketCapNull) {
		t.Errorf("v2 recorded the v1 proxy-provenance marker %s; v2 has no proxy path, so the marker is meaningless there: %v",
			GateMarketCapNull, res.Failures)
	}
}

// A point-in-time cap inside the band must pass, or v2 excludes everything and
// the comparison is vacuous.
func TestGateV2_PassesOnAnInBandPointInTimeCap(t *testing.T) {
	f := passingFeatures(t, 5.0)
	pit := 1e9

	cfg := DefaultGateConfig()
	cfg.Version = GateV2
	res := EvaluateGates(f, GateInput{MarketCapPIT: &pit}, cfg)

	if !res.Passed {
		t.Fatalf("v2 rejected an in-band point-in-time cap: %v", res.Failures)
	}
	if res.Bucket != BucketMarket {
		t.Errorf("bucket = %s, want market for $1bn", res.Bucket)
	}
}

// The zero value must be v1, so every caller written before v2 existed keeps
// its behaviour until it opts in. A default of v2 would silently change the
// meaning of existing reports.
func TestGateVersionZeroValueIsV1(t *testing.T) {
	f := passingFeatures(t, 5.0)
	today := 1e9

	var cfg GateConfig = DefaultGateConfig() // Version left unset
	if cfg.Version == GateV2 {
		t.Fatal("DefaultGateConfig() must not default to v2; existing callers would change behaviour without any edit")
	}
	res := EvaluateGates(f, GateInput{MarketCap: &today}, cfg)
	if !res.Passed {
		t.Errorf("unset version did not behave as v1: %v", res.Failures)
	}
}
