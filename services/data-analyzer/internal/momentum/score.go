package momentum

import (
	"math"
	"sort"
	"strings"
)

// §4 scoring: momentum_score_100.
//
// Computed ONLY for symbols that passed §3.2's gates. Scoring ranks within the
// candidate set; it never admits anything. Score() enforces that by requiring a
// passing GateResult.
//
// Every sub-score is piecewise-linear and deterministic (§4.2), and a null input
// scores 0 for its component while being recorded in NullInputs — because §4.4
// requires the breakdown to be persisted, and "a score with no visible breakdown
// is undebuggable".

// CatalystTier is §3.11's classification.
type CatalystTier string

const (
	CatalystA    CatalystTier = "A"
	CatalystB    CatalystTier = "B"
	CatalystNone CatalystTier = "none"
)

// Weights are §4.1's corrected table. The original listed both "Volume +25" and
// "Relative Volume +20", double-counting the same quantity; §4.1 resolves that
// to volume ACCELERATION 25 plus relative volume 20, preserving the total of
// 100 while removing the overlap.
const (
	WeightVolAccel = 25
	WeightRVol     = 20
	WeightBreakout = 20
	WeightCatalyst = 15
	WeightFloat    = 10
	WeightVWAP     = 5
	WeightHigh52w  = 5
	WeightTotal    = WeightVolAccel + WeightRVol + WeightBreakout + WeightCatalyst + WeightFloat + WeightVWAP + WeightHigh52w
	MaxScore       = 100
	MinScore       = 0
)

// Penalty point values (§4.3), applied after summing, then clamped to [0,100].
const (
	PenaltyExhaustedRSI    = 5
	PenaltyAlreadyExtended = 10
	PenaltyVolumeDecaying  = 5
)

// Penalty reason strings. Persisted, so they are a schema: renaming one breaks
// historical comparison.
const (
	ReasonExhaustedRSI    = "exhausted_momentum_rsi_gt_85"
	ReasonAlreadyExtended = "already_extended_change_gt_20"
	ReasonVolumeDecaying  = "volume_decaying_accel_lt_1_rvol_ge_3"
)

// Null-input markers, recorded per §4.4 so a low score is attributable.
const (
	NullVolAccel = "vol_accel"
	NullRVol     = "rvol_20"
	NullBreakout = "breakout_state"
	NullCatalyst = "catalyst_tier"
	NullFloat    = "float_shares_est"
	NullVWAP     = "above_vwap"
	NullHigh52w  = "pct_of_52w_high"
	NullRSI      = "rsi_14"
)

// ScoreInput carries the §4 inputs that do not come from bars.
type ScoreInput struct {
	// CatalystTier from §3.11. Empty string is treated as null rather than as
	// "none": "we never fetched news" and "we fetched and found nothing" score
	// the same 0 but must be distinguishable in the record, since the first is an
	// ingestion gap and the second is a fact about the symbol.
	CatalystTier CatalystTier

	// FloatSharesEst from §3.9, in shares.
	FloatSharesEst *float64
}

// SubScores is §4.4's required breakdown: every component separately.
type SubScores struct {
	VolAccel float64
	RVol     float64
	Breakout float64
	Catalyst float64
	Float    float64
	VWAP     float64
	High52w  float64
}

func (s SubScores) Sum() float64 {
	return s.VolAccel + s.RVol + s.Breakout + s.Catalyst + s.Float + s.VWAP + s.High52w
}

// Score is the full §4.4 output contract for one symbol on one day.
type Score struct {
	Total int

	// Raw is the pre-penalty, pre-clamp sum, kept because a 0 could otherwise
	// mean "scored nothing" or "scored 8 and lost 15 to penalties".
	Raw float64

	Sub SubScores

	// PenaltyPoints is the total deducted, and Penalties names each one applied.
	PenaltyPoints float64
	Penalties     []string

	// NullInputs lists components whose input was absent and therefore scored 0.
	NullInputs []string

	Bucket Bucket
}

// PenaltyString and NullString render for persistence.
func (s Score) PenaltyString() string { return strings.Join(s.Penalties, ",") }
func (s Score) NullString() string    { return strings.Join(s.NullInputs, ",") }

// Score computes §4's momentum_score_100.
//
// gate must be a PASSING GateResult. Scoring a gate-failed symbol is a
// programming error, not a runtime condition: §3.2 is explicit that a gate
// failure means excluded rather than low-scored, and returning a score for one
// would let it appear in a ranked list. The second return value is false in that
// case rather than producing a plausible-looking number.
func ScoreCandidate(f *Features, in ScoreInput, gate GateResult) (Score, bool) {
	if f == nil || !gate.Passed {
		return Score{}, false
	}

	s := Score{Bucket: gate.Bucket}

	// ── Volume acceleration, 0-25 (§4.2) ──
	if f.VolAccel == nil {
		s.NullInputs = append(s.NullInputs, NullVolAccel)
	} else {
		s.Sub.VolAccel = piecewise(*f.VolAccel, []band{
			{lo: 0, hi: 1.0, pLo: 0, pHi: 0},
			{lo: 1.0, hi: 1.5, pLo: 0, pHi: 10},
			{lo: 1.5, hi: 2.5, pLo: 10, pHi: 20},
			{lo: 2.5, hi: 4.0, pLo: 20, pHi: 25},
		}, WeightVolAccel)
	}

	// ── Relative volume, 0-20 ──
	if f.RVol20 == nil {
		s.NullInputs = append(s.NullInputs, NullRVol)
	} else {
		s.Sub.RVol = piecewise(*f.RVol20, []band{
			{lo: 0, hi: 1.5, pLo: 0, pHi: 0},
			{lo: 1.5, hi: 3.0, pLo: 0, pHi: 12},
			{lo: 3.0, hi: 5.0, pLo: 12, pHi: 18},
			{lo: 5.0, hi: 10.0, pLo: 18, pHi: 20},
		}, WeightRVol)
	}

	// ── Breakout, 0-20 — discrete, not interpolated ──
	if f.BreakoutState == nil {
		s.NullInputs = append(s.NullInputs, NullBreakout)
	} else {
		s.Sub.Breakout = breakoutPoints(*f.BreakoutState)
	}

	// ── Catalyst, 0-15 ──
	if in.CatalystTier == "" {
		s.NullInputs = append(s.NullInputs, NullCatalyst)
	} else {
		switch in.CatalystTier {
		case CatalystA:
			s.Sub.Catalyst = 15
		case CatalystB:
			s.Sub.Catalyst = 8
		default:
			s.Sub.Catalyst = 0
		}
	}

	// ── Float, 0-10 — step bands, NOT interpolated ──
	//
	// §4.2 gives float as discrete values per band rather than a ramp, unlike
	// the volume components. Interpolating here would invent a precision the
	// estimate does not have: float_shares_est is itself a §3.9 approximation.
	if in.FloatSharesEst == nil {
		s.NullInputs = append(s.NullInputs, NullFloat)
	} else {
		s.Sub.Float = floatPoints(*in.FloatSharesEst)
	}

	// ── Above VWAP, 0-5 ──
	if f.AboveVWAP == nil {
		s.NullInputs = append(s.NullInputs, NullVWAP)
	} else if *f.AboveVWAP {
		s.Sub.VWAP = WeightVWAP
	}

	// ── Near 52-week high, 0-5 — step bands ──
	if f.PctOf52wHigh == nil {
		s.NullInputs = append(s.NullInputs, NullHigh52w)
	} else {
		s.Sub.High52w = high52wPoints(*f.PctOf52wHigh)
	}

	s.Raw = s.Sub.Sum()

	// ── Penalties (§4.3), after summing ──
	//
	// RSI is the only input whose absence is NOT penalised: §3.10 makes RSI a
	// penalty input only, so a missing RSI means "no evidence of exhaustion",
	// not "assume exhausted". It is still recorded as null.
	if f.RSI14 == nil {
		s.NullInputs = append(s.NullInputs, NullRSI)
	} else if *f.RSI14 > 85 {
		s.PenaltyPoints += PenaltyExhaustedRSI
		s.Penalties = append(s.Penalties, ReasonExhaustedRSI)
	}

	// "Already extended" enforces the core thesis inside the score: the system
	// fires at +8-15% on a confirmed move, so a same-day +22% is later in the
	// sequence than the intended entry (§4.3).
	if f.ChangePct != nil && *f.ChangePct > 20 {
		s.PenaltyPoints += PenaltyAlreadyExtended
		s.Penalties = append(s.Penalties, ReasonAlreadyExtended)
	}

	// High volume that is NOT building should be visibly marked down rather than
	// silently scoring well (§4.3). Requires both conditions: decaying volume on
	// its own already scores 0 on the acceleration component.
	if f.VolAccel != nil && f.RVol20 != nil && *f.VolAccel < 1.0 && *f.RVol20 >= 3 {
		s.PenaltyPoints += PenaltyVolumeDecaying
		s.Penalties = append(s.Penalties, ReasonVolumeDecaying)
	}

	total := s.Raw - s.PenaltyPoints
	if total < MinScore {
		total = MinScore
	}
	if total > MaxScore {
		total = MaxScore
	}
	// Rounded rather than truncated: momentum_score_100 is an integer, and
	// truncation would bias every score downward by up to a point, which matters
	// at a threshold of exactly 60.
	s.Total = int(math.Round(total))

	sort.Strings(s.NullInputs)
	sort.Strings(s.Penalties)
	return s, true
}

// band is one piecewise-linear segment: value in [lo,hi) maps to [pLo,pHi].
type band struct {
	lo, hi   float64
	pLo, pHi float64
}

// piecewise interpolates v through the bands, clamping below the first band's lo
// to its pLo and above the last band's hi to maxPoints (§4.2: "clamp at band
// edges").
func piecewise(v float64, bands []band, maxPoints float64) float64 {
	if len(bands) == 0 {
		return 0
	}
	if v <= bands[0].lo {
		return bands[0].pLo
	}
	for _, b := range bands {
		if v >= b.lo && v < b.hi {
			if b.pHi == b.pLo {
				return b.pLo
			}
			frac := (v - b.lo) / (b.hi - b.lo)
			return b.pLo + frac*(b.pHi-b.pLo)
		}
	}
	return maxPoints
}

func breakoutPoints(st BreakoutState) float64 {
	switch st {
	case BreakoutFromConsolidation:
		return 20
	case Breakout:
		return 14
	case BreakoutApproaching:
		return 8
	default:
		return 0
	}
}

// floatPoints implements §4.2's float bands. Lower float scores higher: a small
// float means the same buying pressure moves the price further.
func floatPoints(shares float64) float64 {
	switch {
	case shares < 20e6:
		return 10
	case shares < 50e6:
		return 7
	case shares < 100e6:
		return 4
	case shares < 300e6:
		return 2
	default:
		return 0
	}
}

// high52wPoints implements §4.2's proximity bands.
//
// The >= 1.00 band IS the new-52-week-high case and is reachable because §3.7's
// window excludes today; values above 1.0 clamp to 5 rather than extrapolating.
func high52wPoints(ratio float64) float64 {
	switch {
	case ratio >= 1.00:
		return 5
	case ratio >= 0.95:
		return 4
	case ratio >= 0.90:
		return 2
	default:
		return 0
	}
}
