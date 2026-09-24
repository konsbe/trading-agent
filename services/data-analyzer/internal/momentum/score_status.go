package momentum

// What the score claims about itself, for every surface that serves it.
//
// Both are build-level facts about the scoring system, not per-symbol
// properties, and they live next to the weights so that a future model version
// changes them in exactly one place.
const (
	// ScoreModelVersion names the weight table above (§4.1 v2).
	ScoreModelVersion = "v2"

	// ScoreStatus is "unvalidated" per Phase 1 §10.1.0: no component separates
	// outcomes within a bucket out-of-sample. It travels with the score on every
	// read path so a sorted or bare score can never appear without it.
	ScoreStatus = "unvalidated"
)

// nullInputWeights maps a NullInputs marker to the weight its component could
// have earned. rsi_14 is only a penalty input, so a null RSI costs no headroom.
var nullInputWeights = map[string]int{
	NullVolAccel: WeightVolAccel,
	NullRVol:     WeightRVol,
	NullBreakout: WeightBreakout,
	NullCatalyst: WeightCatalyst,
	NullFloat:    WeightFloat,
	NullVWAP:     WeightVWAP,
	NullHigh52w:  WeightHigh52w,
	NullRSI:      0,
}

// Attainable is the ceiling a score could have reached given its null inputs:
// WeightAllocated minus the weight of every component whose input was absent.
// With catalyst_tier null (the norm until §3.11 ships) that is 90 - 15 = 75,
// the "practical ceiling" §4.4 describes — derived per row rather than
// hard-coded, so it stays right once catalyst data starts arriving.
func Attainable(nullInputs []string) int {
	attainable := WeightAllocated
	seen := make(map[string]bool, len(nullInputs))
	for _, input := range nullInputs {
		if seen[input] {
			continue
		}
		seen[input] = true
		attainable -= nullInputWeights[input]
	}
	return attainable
}
