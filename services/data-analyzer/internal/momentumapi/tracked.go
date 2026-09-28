package momentumapi

import (
	"net/http"
	"strings"
	"time"

	"github.com/konsbe/trading-agent/services/data-analyzer/internal/store"
)

// GET /api/v1/scanner/tracked?status=active|closed|all — the Tracked Positions
// addendum. A read-only view of momentum_tracked: research instrumentation for
// evaluating §5's exit rules, not a portfolio. Nothing here computes more than
// /tracked in Discord already does.

var trackedStatuses = map[string]store.TrackedStatusFilter{
	"active": "active", "closed": "closed", "all": "all",
}

type trackedResponse struct {
	Summary trackedSummary    `json:"summary"`
	Chain   trackedChain      `json:"chain"`
	Tracked []trackedPosition `json:"tracked"`
}

// trackedChain is the system-level freshness of the daily chain, for the one
// banner above the tabs (addendum §6.1): computed here from the NYSE calendar
// and scan-grace rule behind scan.is_stale, so the UI never derives staleness
// from row dates. Counts are trading sessions, so weekends and holidays never
// raise them.
type trackedChain struct {
	// ExpectedSession: the latest session whose scan should exist by now.
	ExpectedSession string `json:"expected_session"`
	// LastScanDate: max(momentum_features.ts); null when there is no scan.
	LastScanDate *string `json:"last_scan_date"`
	// LastTrackedSession: the latest session the tracker completed.
	LastTrackedSession *string `json:"last_tracked_session"`
	// SessionsBehind: trading sessions after LastScanDate up to and including
	// ExpectedSession (0 = current); null when there is no scan.
	SessionsBehind *int `json:"sessions_behind"`
	// TrackerBehind: the latest scanned session's chain row has its scan
	// marker more than trackerGrace ago and no tracker marker, or gave up.
	TrackerBehind bool `json:"tracker_behind"`
}

// trackerGrace: the tracker runs right after the scanner; a missing tracker
// marker inside this window is the chain still running, not a lag.
const trackerGrace = 15 * time.Minute

// sessionsAfter counts NYSE sessions d with from < d <= through (civil dates).
func sessionsAfter(from, through time.Time) (int, error) {
	n := 0
	for d := civilDate(from).AddDate(0, 0, 1); !d.After(civilDate(through)); d = d.AddDate(0, 0, 1) {
		ok, err := IsTradingDay(d)
		if err != nil {
			return 0, err
		}
		if ok {
			n++
		}
	}
	return n, nil
}

func buildTrackedChain(expected time.Time, latest *time.Time, c store.TrackerChain, now time.Time) (trackedChain, error) {
	out := trackedChain{
		ExpectedSession:    expected.Format(time.DateOnly),
		LastScanDate:       dateOnly(latest),
		LastTrackedSession: dateOnly(c.LastTrackedSession),
	}
	if latest != nil {
		n, err := sessionsAfter(*latest, expected)
		if err != nil {
			return out, err
		}
		out.SessionsBehind = &n
	}
	if r := c.LatestScanRun; r != nil {
		switch {
		case r.GaveUpAt != nil:
			out.TrackerBehind = r.TrackerCompletedAt == nil
		case r.ScannerCompletedAt != nil && r.TrackerCompletedAt == nil:
			out.TrackerBehind = now.Sub(*r.ScannerCompletedAt) > trackerGrace
		}
	}
	return out, nil
}

type trackedSummary struct {
	ActiveCount int `json:"active_count"`
	ClosedCount int `json:"closed_count"`
}

type trackedPosition struct {
	Symbol      string  `json:"symbol"`
	Exchange    *string `json:"exchange"`
	CompanyName *string `json:"company_name"`
	Bucket      string  `json:"bucket"`
	Status      string  `json:"status"`
	AlertedDate string  `json:"alerted_date"`

	// SessionsElapsed is the tracker's stored count as of LastEvaluatedDate;
	// null (as is UnrealizedPct) for a row the tracker has never evaluated
	// (alerted on the latest session): a count of 0 would read as a reading.
	// EvaluationBehind is true for an active row the tracker has not yet
	// evaluated through the latest scan, so the count may lag.
	SessionsElapsed   *int    `json:"sessions_elapsed"`
	LastEvaluatedDate *string `json:"last_evaluated_date"`
	EvaluationBehind  bool    `json:"evaluation_behind"`

	ReferencePrice float64 `json:"reference_price"`
	// CurrentPrice is the latest scan's as-traded close, active rows only;
	// CurrentPriceDate is that scan's date.
	CurrentPrice     *float64 `json:"current_price"`
	CurrentPriceDate *string  `json:"current_price_date"`
	UnrealizedPct    *float64 `json:"unrealized_pct"`
	MaxGainPct       *float64 `json:"max_gain_pct"`

	ExitReason     *string  `json:"exit_reason"`
	ExitReasonNote *string  `json:"exit_reason_note"`
	ExitPrice      *float64 `json:"exit_price"`
	ExitPct        *float64 `json:"exit_pct"`
	ClosedDate     *string  `json:"closed_date"`
}

func dateOnly(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.UTC().Format(time.DateOnly)
	return &s
}

func (s *Server) handleTracked(w http.ResponseWriter, r *http.Request) {
	raw := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("status")))
	if raw == "" {
		raw = "active"
	}
	status, ok := trackedStatuses[raw]
	if !ok {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid_status_param"})
		return
	}
	key := "tracked:" + raw
	if body, ok := s.cache.get(key); ok {
		writeBody(w, http.StatusOK, body)
		return
	}
	ctx := r.Context()

	// No scan yet is not an error here: tracked rows still show, without a
	// current price.
	var latest *time.Time
	if d, ok, err := s.cfg.Store.LatestScanDate(ctx); err != nil {
		s.storeError(w, r, err)
		return
	} else if ok {
		latest = &d
	}
	counts, err := s.cfg.Store.TrackedCounts(ctx)
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	rows, err := s.cfg.Store.TrackedPositions(ctx, status, latest)
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	now := s.cfg.Now()
	expected, err := ExpectedSession(now, s.cfg.SessionReadyAfter)
	if err != nil {
		s.cfg.Log.Error("momentum-api: tracked chain calendar", "err", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "session_calendar_unavailable"})
		return
	}
	tc, err := s.cfg.Store.TrackerChain(ctx, latest)
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	chain, err := buildTrackedChain(expected, latest, tc, now)
	if err != nil {
		s.cfg.Log.Error("momentum-api: tracked chain calendar", "err", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "session_calendar_unavailable"})
		return
	}

	resp := trackedResponse{
		Summary: trackedSummary{ActiveCount: counts.Active, ClosedCount: counts.Closed},
		Chain:   chain,
		Tracked: []trackedPosition{},
	}
	for _, row := range rows {
		resp.Tracked = append(resp.Tracked, s.toTrackedPosition(row, latest))
	}
	s.respondCached(w, key, resp)
}

func (s *Server) toTrackedPosition(row store.TrackedPositionRow, latest *time.Time) trackedPosition {
	p := trackedPosition{
		Symbol:            row.Symbol,
		Exchange:          row.Exchange,
		CompanyName:       row.CompanyName,
		Bucket:            row.Bucket,
		Status:            row.Status,
		AlertedDate:       row.AlertedTS.Format(time.DateOnly),
		LastEvaluatedDate: dateOnly(row.LastEvaluatedTS),
		ReferencePrice:    row.ReferencePrice,
		MaxGainPct:        finite(row.MaxGainPct),
		ExitReason:        row.ExitReason,
	}

	evaluated := row.LastEvaluatedTS != nil
	if evaluated {
		n := row.SessionsElapsed
		p.SessionsElapsed = &n
	}

	if row.Status == "active" {
		// unrealized_pct and exit_pct are mutually exclusive by status.
		if price := finite(row.LatestClose); price != nil && latest != nil {
			p.CurrentPrice = price
			p.CurrentPriceDate = dateOnly(latest)
			if row.ReferencePrice > 0 && evaluated {
				v := (*price/row.ReferencePrice - 1) * 100
				p.UnrealizedPct = finite(&v)
			}
		}
		evaluatedThrough := row.AlertedTS
		if row.LastEvaluatedTS != nil {
			evaluatedThrough = row.LastEvaluatedTS.UTC()
		}
		p.EvaluationBehind = latest != nil && civilDate(*latest).After(civilDate(evaluatedThrough))
	} else {
		p.ExitPrice = finite(row.ExitPrice)
		p.ExitPct = finite(row.ExitPct)
		p.ClosedDate = dateOnly(row.ExitTS)
	}

	if row.ExitReason != nil {
		if note, ok := s.cfg.Caveats.ExitReasonNotes[*row.ExitReason]; ok {
			p.ExitReasonNote = &note
		} else {
			s.cfg.Log.Error("momentum-api: exit reason without a shared note", "exit_reason", *row.ExitReason, "symbol", row.Symbol)
		}
	}
	return p
}
