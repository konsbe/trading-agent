package store

import (
	"fmt"
	"strings"
	"time"
)

// ParseSessionFlag reads the scanner's and tracker's -session value. Empty
// means "the newest session in the bars" (nil), which is the unattended path.
func ParseSessionFlag(s string) (*time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	d, err := time.Parse(time.DateOnly, s)
	if err != nil {
		return nil, fmt.Errorf("-session wants YYYY-MM-DD, got %q", s)
	}
	return &d, nil
}

// PITCutoff is the exclusive upper bound of a point-in-time read for session:
// rows stamped before the next UTC midnight. The unattended chain runs the
// evening of the session (UTC), so a caught-up session reads what its on-time
// run could have read, minus anything written between that run and midnight.
func PITCutoff(session time.Time) time.Time {
	y, m, d := session.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC).AddDate(0, 0, 1)
}
