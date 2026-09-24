package momentum

import "testing"

func TestAttainable(t *testing.T) {
	cases := []struct {
		name       string
		nullInputs []string
		want       int
	}{
		{"no nulls reaches the allocated weight", nil, WeightAllocated},
		{"null catalyst is the 75 practical ceiling", []string{NullCatalyst}, 75},
		{"rsi is a penalty input and costs no headroom", []string{NullRSI}, WeightAllocated},
		{"zero-weight components cost nothing", []string{NullBreakout, NullHigh52w}, WeightAllocated},
		{"several nulls subtract each weight", []string{NullCatalyst, NullFloat, NullVWAP}, 90 - 15 - 10 - 5},
		{"duplicates are counted once", []string{NullCatalyst, NullCatalyst}, 75},
		{"unknown markers cost nothing", []string{"not_a_component"}, WeightAllocated},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Attainable(c.nullInputs); got != c.want {
				t.Errorf("Attainable(%v) = %d, want %d", c.nullInputs, got, c.want)
			}
		})
	}
}

// Every scored component must have a null marker with a weight, or Attainable
// would silently overstate the ceiling for that component's missing input.
func TestNullInputWeightsCoverEveryComponent(t *testing.T) {
	sum := 0
	for marker, w := range nullInputWeights {
		if marker == NullRSI {
			continue
		}
		sum += w
	}
	if sum != WeightAllocated {
		t.Errorf("null-input weights sum to %d, want WeightAllocated %d", sum, WeightAllocated)
	}
}
