// Package barquality detects corporate-action adjustment defects in stored bars.
//
// # Why this exists
//
// It was written after a provider shipped a series that alternated, bar by bar,
// between split-adjusted and unadjusted values inside a single response. The
// result was fabricated one-day moves of +1382%, -94%, +796% and +925% across
// 75 of 446 backfilled symbols — 41% of the penny-price bucket.
//
// For a momentum scanner that defect is maximally harmful: a fabricated +796%
// day is precisely the signal the scanner exists to find, so every corrupted
// symbol sorts to the top of the scan looking like a perfect breakout. No unit
// test caught it, because each individual bar was internally consistent; only
// comparing against a second provider revealed it.
//
// This package turns that one-off comparison into a permanent gate that needs no
// second provider.
//
// # How an artifact is distinguished from a real move
//
// Penny stocks genuinely do move 300% in a day, so price alone cannot decide.
// Two further signals separate the cases, and the second is the decisive one:
//
//  1. The price ratio sits near a plausible corporate-action factor (2, 3, 5,
//     10, 15, 20...). Suggestive, not conclusive — a real move can land on 10x
//     by coincidence.
//
//  2. DOLLAR VOLUME IS CONTINUOUS ACROSS THE JUMP. When a provider switches
//     between adjusted and unadjusted output, price and volume are rescaled in
//     OPPOSITE directions by the same factor, so their product is preserved. A
//     real 15x price move comes with a volume surge, so dollar volume explodes.
//
//     Observed on ABTS, 2025-02-27 to 2025-02-28 (1-for-15 reverse split):
//
//     close   0.421 ->  6.240   = 14.82x
//     volume  6600  ->  778     =  0.12x
//     close x volume  2779 -> 4855 = 1.75x   <- barely moved
//
//  3. THE WHOLE STEP IS AN OVERNIGHT GAP that lands on a plausible factor.
//
//     Signal 2 is necessary but not sufficient, and the ABTS fixture is what
//     proved it: on 2025-03-04 a real selloff coincided with a seam, pushing
//     dollar volume to 9.33x and hiding the seam from signal 2 entirely.
//
//     prev close  6.2250
//     cur OPEN    0.3230   <- gap ratio 1/19.27, on a reverse-split factor
//     cur close   0.3520
//
//     A rescaling seam puts the entire move between the previous close and the
//     open, because both bars are internally consistent and only the scale
//     changed. A real selloff opens near the prior close and falls intraday.
//     Checking the gap is independent of volume, so it survives a coinciding
//     real move.
//
// Either signal 2 or signal 3 is enough to report, which is why a synthetic
// fixture would have been inadequate: it would have exercised only the clean
// case that signal 2 already handles.
//
// # Measured false-positive rate: ~8% of symbols on a penny-heavy universe
//
// CORRECTION to an earlier estimate of ~3%, which was taken from a 64-symbol
// sample and did not survive the full 450. On the completed pilot this flags 38
// of 450 (8.4%, 51 breaks) against Tiingo — a source since verified to apply ONE
// consistent factor per series — so effectively all of those are false.
//
// The cause is structural, not a tuning error. In an illiquid micro-cap a real
// -79% day arrives with volume merely doubling, so dollar volume lands at 0.43x,
// inside signal 2's "continuous" band, and the price ratio can sit near a round
// factor by coincidence. EDHL on 2025-07-14 fired BOTH signals and is a genuine
// crash: Tiingo reports splitFactor=1.0 throughout with a uniform 16x adjustment,
// and the RAW closes show the same move (4.30 -> 0.9129).
//
// That last point is the discriminator this package cannot use: for a real move
// the UNADJUSTED series moves too, while for a seam only the adjusted series
// does. Checking it needs raw and adjusted side by side, which Tiingo provides
// (close + adjClose + splitFactor) and Twelve Data does not — so it cannot be
// the general check, and is instead the recommended follow-up when triaging a
// specific flagged symbol on a provider that exposes both.
//
// None of this weakens the original purpose. Twelve Data's defect presented at
// 12-17% of symbols with seams whose raw series showed NO move at all, which is
// categorically different from these. The output is a review list; treat a flag
// as "go look", never as "reject".
package barquality

import (
	"fmt"
	"math"
	"sort"
	"time"
)

// Bar is the minimum a check needs. Declared locally so this package does not
// depend on the store's row type.
type Bar struct {
	TS     time.Time
	Open   float64
	High   float64
	Low    float64
	Close  float64
	Volume float64
}

// Config tunes detection.
type Config struct {
	// MinPriceRatio is how far a close-to-close ratio must be from 1 before the
	// bar is examined at all, expressed as a multiple. 2.5 means "at least
	// 2.5x up or down".
	//
	// Not lower: real penny-stock days of +100% are common and are not defects,
	// so a tighter threshold would bury the signal in true positives.
	MinPriceRatio float64

	// MaxDollarVolumeRatio is the band within which dollar volume counts as
	// "continuous" across the jump, and therefore as evidence of a rescaling
	// seam rather than a real move.
	//
	// 4.0 is deliberately loose: a genuine 2.5x+ price move essentially always
	// takes dollar volume far beyond 4x, while an adjustment seam holds it near
	// 1x. The gap between the two populations is wide, so precision here buys
	// little and risks missing seams on thinly traded names.
	MaxDollarVolumeRatio float64

	// SplitFactorTolerance is the fractional distance at which a ratio counts as
	// "near" a plausible corporate-action factor.
	//
	// 8% rather than something tighter because the underlying price also moves
	// on the seam day: ABTS's 1-for-15 shows as 14.82x (1.2% off) on one seam and
	// 19.27x against a 20x factor (3.7% off) on another. Much looser than this
	// and the factor list starts matching arbitrary ratios, which would make
	// NearFactor meaningless as corroboration.
	SplitFactorTolerance float64
}

func DefaultConfig() Config {
	return Config{
		MinPriceRatio:        2.5,
		MaxDollarVolumeRatio: 4.0,
		SplitFactorTolerance: 0.08,
	}
}

// plausibleFactors are the corporate-action ratios seen in US equities. Reverse
// splits in micro-caps reach 1-for-20 and beyond, which is why the list runs
// well past the 2:1 and 4:1 of large caps.
var plausibleFactors = []float64{2, 3, 4, 5, 6, 7, 8, 10, 12, 15, 20, 25, 30, 40, 50, 100}

// Break is one suspected adjustment discontinuity.
type Break struct {
	TS        time.Time
	PrevClose float64
	Close     float64

	// PriceRatio is Close/PrevClose. >1 means the series steps up.
	PriceRatio float64

	// DollarVolumeRatio is (Close*Volume)/(PrevClose*PrevVolume). Near 1 is the
	// signature of a rescaling seam.
	DollarVolumeRatio float64

	// GapRatio is cur.Open/prev.Close — the overnight step. Near a split factor
	// means the whole move happened between sessions, which is what a rescaling
	// seam looks like. Zero when the open is unavailable.
	GapRatio float64

	// NearFactor is the plausible corporate-action factor the price or gap ratio
	// sits on, or 0 if none.
	NearFactor float64

	// Signals names which checks fired, so a reviewer can see WHY a bar was
	// reported rather than having to re-derive it.
	Signals []string
}

// Reason renders the evidence, so a warning is actionable without a query.
func (b Break) Reason() string {
	f := "none"
	if b.NearFactor != 0 {
		f = fmt.Sprintf("%.0fx", b.NearFactor)
	}
	return fmt.Sprintf("%s close %.4f->%.4f (%.2fx); dollar volume %.2fx; overnight gap %.4fx; near split factor %s; signals %v",
		b.TS.Format(time.DateOnly), b.PrevClose, b.Close, b.PriceRatio,
		b.DollarVolumeRatio, b.GapRatio, f, b.Signals)
}

// DetectAdjustmentBreaks returns suspected adjustment seams in a chronological
// series.
//
// Bars must be oldest-first; the function sorts defensively because a caller
// passing them backwards would otherwise get a reversed-but-plausible answer
// rather than an error.
//
// A bar is reported only when the price step is large AND dollar volume stayed
// continuous. Requiring both is what keeps real penny-stock moves out: those
// have the large step and a dollar-volume explosion.
func DetectAdjustmentBreaks(bars []Bar, cfg Config) []Break {
	d := DefaultConfig()
	if cfg.MinPriceRatio <= 1 {
		cfg.MinPriceRatio = d.MinPriceRatio
	}
	if cfg.MaxDollarVolumeRatio <= 0 {
		cfg.MaxDollarVolumeRatio = d.MaxDollarVolumeRatio
	}
	if cfg.SplitFactorTolerance <= 0 {
		cfg.SplitFactorTolerance = d.SplitFactorTolerance
	}
	if len(bars) < 2 {
		return nil
	}

	s := make([]Bar, len(bars))
	copy(s, bars)
	sort.Slice(s, func(i, j int) bool { return s[i].TS.Before(s[j].TS) })

	var out []Break
	for i := 1; i < len(s); i++ {
		prev, cur := s[i-1], s[i]
		if prev.Close <= 0 || cur.Close <= 0 || prev.Volume <= 0 || cur.Volume <= 0 {
			continue
		}
		pr := cur.Close / prev.Close
		if pr < cfg.MinPriceRatio && pr > 1/cfg.MinPriceRatio {
			continue
		}
		var signals []string

		// Signal 2: dollar volume continuous in BOTH directions — a seam
		// preserves the product, so 0.25x is as diagnostic as 4x.
		dv := (cur.Close * cur.Volume) / (prev.Close * prev.Volume)
		if dv <= cfg.MaxDollarVolumeRatio && dv >= 1/cfg.MaxDollarVolumeRatio {
			signals = append(signals, "dollar_volume_continuous")
		}

		// Signal 3: the entire step is an overnight gap sitting on a plausible
		// corporate-action factor. Independent of volume, so it still fires when
		// a real move coincides with the seam and inflates dollar volume.
		gapRatio, gapFactor := 0.0, 0.0
		if cur.Open > 0 {
			gapRatio = cur.Open / prev.Close
			if gapFactor = nearestFactor(gapRatio, cfg.SplitFactorTolerance); gapFactor != 0 {
				signals = append(signals, "overnight_gap_on_split_factor")
			}
		}

		if len(signals) == 0 {
			continue
		}

		factor := nearestFactor(pr, cfg.SplitFactorTolerance)
		if factor == 0 {
			factor = gapFactor
		}
		out = append(out, Break{
			TS:                cur.TS,
			PrevClose:         prev.Close,
			Close:             cur.Close,
			PriceRatio:        pr,
			DollarVolumeRatio: dv,
			GapRatio:          gapRatio,
			NearFactor:        factor,
			Signals:           signals,
		})
	}
	return out
}

// nearestFactor returns the plausible corporate-action factor a ratio sits on,
// checking the ratio and its reciprocal so forward and reverse splits are both
// recognised. Returns 0 when none is close.
func nearestFactor(ratio, tol float64) float64 {
	cands := []float64{ratio, 1 / ratio}
	best, bestErr := 0.0, math.Inf(1)
	for _, c := range cands {
		for _, f := range plausibleFactors {
			e := math.Abs(c-f) / f
			if e <= tol && e < bestErr {
				best, bestErr = f, e
			}
		}
	}
	return best
}
