package store

import "testing"

func TestParsePenalties(t *testing.T) {
	cases := []struct {
		raw     string
		want    int
		wantErr bool
	}{
		{`["exhausted_momentum_rsi_gt_85","already_extended_change_gt_20"]`, 2, false},
		{`[]`, 0, false},
		{`{}`, 0, false},
		{` { } `, 0, true}, // only the exact column default is accepted
		{``, 0, false},
		{`null`, 0, false},
		{`{"exhausted_momentum_rsi_gt_85": 5}`, 0, true},
		{`"oops"`, 0, true},
	}
	for _, c := range cases {
		got, err := parsePenalties([]byte(c.raw))
		if (err != nil) != c.wantErr {
			t.Errorf("parsePenalties(%q) err = %v, wantErr %v", c.raw, err, c.wantErr)
		}
		if err == nil && len(got) != c.want {
			t.Errorf("parsePenalties(%q) = %v, want %d names", c.raw, got, c.want)
		}
	}
}
