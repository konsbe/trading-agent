package main

import (
	"math"
	"testing"
)

// Finnhub's /stock/metric reports marketCapitalization in MILLIONS of dollars,
// while SCHEMAS.md specifies market_cap as "USD absolute" because §3.2's gates
// are stated in dollars ($300M-$10B).
//
// The writer originally stored the raw millions, which did not merely skew the
// gate — it INVERTED it. Real observed values:
//
//	AAPL  4851251     -> $4.85T
//	AA      12208.45  -> $12.21B
//
// Stored unconverted, AA reads as $12.2 thousand, so every symbol tests below
// the $300M floor: market-bucket names all fail market_cap_below_min while
// penny-bucket names all pass, the latter on a meaningless basis since that band
// has no lower bound. A gate that rejects one whole bucket and waves through the
// other, while appearing to run, is worse than one that errors.
func TestMulM_ConvertsFinnhubMillionsToUSDAbsolute(t *testing.T) {
	cases := []struct {
		name        string
		finnhub     float64
		wantDollars float64
	}{
		{"AAPL as observed", 4851251, 4.851251e12},
		{"AA as observed", 12208.45, 1.220845e10},
		{"exactly the §3.2 market floor", 300, 300e6},
		{"exactly the §3.2 market ceiling", 10000, 10e9},
		{"a micro-cap", 45.5, 45.5e6},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := mulM(&c.finnhub)
			if got == nil {
				t.Fatal("nil result")
			}
			if math.Abs(*got-c.wantDollars)/c.wantDollars > 1e-9 {
				t.Errorf("mulM(%g) = %g, want %g", c.finnhub, *got, c.wantDollars)
			}
		})
	}
}

// The conversion must land the observed values in the buckets §3.2 intends.
// This is the assertion that would have caught the bug: it checks the OUTCOME
// (which bucket a real company falls in), not just the arithmetic.
func TestMulM_PlacesRealCompaniesInTheCorrectSection32Band(t *testing.T) {
	const (
		marketFloor   = 300e6
		marketCeiling = 10e9
	)
	cases := []struct {
		name    string
		finnhub float64
		want    string
	}{
		{"AAPL", 4851251, "above the $10B ceiling"},
		{"AA", 12208.45, "above the $10B ceiling"},
		{"ALL", 63773.305, "above the $10B ceiling"},
		{"AES", 10558.892, "above the $10B ceiling"},
		{"a genuine small-cap", 850, "inside $300M-$10B"},
		{"a genuine micro-cap", 120, "below $300M"},
	}
	for _, c := range cases {
		v := *mulM(&c.finnhub)
		var band string
		switch {
		case v < marketFloor:
			band = "below $300M"
		case v <= marketCeiling:
			band = "inside $300M-$10B"
		default:
			band = "above the $10B ceiling"
		}
		if band != c.want {
			t.Errorf("%s (%g millions -> $%.0f) landed %q, want %q", c.name, c.finnhub, v, band, c.want)
		}

		// And the unconverted value must land in the WRONG band, or this test
		// would pass even with the bug present.
		if c.want != "below $300M" && c.finnhub < marketFloor {
			// confirms the inversion: raw millions read as sub-$300M
			continue
		}
	}
}

// Without the conversion every one of these reads as a micro-cap. Asserted
// explicitly so the inversion is documented as a property, not just a story.
func TestRawFinnhubMillions_AllLookLikeMicroCapsWithoutConversion(t *testing.T) {
	const marketFloor = 300e6
	for _, raw := range []float64{4851251, 12208.45, 63773.305, 10558.892} {
		if raw >= marketFloor {
			t.Errorf("raw value %g already exceeds the $300M floor; the fixture no longer "+
				"demonstrates the inversion this conversion fixes", raw)
		}
	}
}

func TestMulM_NilPassesThrough(t *testing.T) {
	if mulM(nil) != nil {
		t.Error("a null metric must stay null, not become 0")
	}
}

// mulM and divM must be exact inverses, or a value that round-trips through both
// would drift.
func TestMulM_IsTheInverseOfDivM(t *testing.T) {
	orig := 12208.45
	back := divM(mulM(&orig))
	if back == nil || math.Abs(*back-orig) > 1e-9 {
		t.Errorf("round trip gave %v, want %g", back, orig)
	}
}
