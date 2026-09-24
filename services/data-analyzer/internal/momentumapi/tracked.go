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
	Tracked []trackedPosition `json:"tracked"`
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
	// EvaluationBehind is true for an active row the tracker has not yet
	// evaluated through the latest scan, so the count may lag.
	SessionsElapsed   int     `json:"sessions_elapsed"`
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

	resp := trackedResponse{
		Summary: trackedSummary{ActiveCount: counts.Active, ClosedCount: counts.Closed},
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
		SessionsElapsed:   row.SessionsElapsed,
		LastEvaluatedDate: dateOnly(row.LastEvaluatedTS),
		ReferencePrice:    row.ReferencePrice,
		MaxGainPct:        finite(row.MaxGainPct),
		ExitReason:        row.ExitReason,
	}

	if row.Status == "active" {
		// unrealized_pct and exit_pct are mutually exclusive by status.
		if price := finite(row.LatestClose); price != nil && latest != nil {
			p.CurrentPrice = price
			p.CurrentPriceDate = dateOnly(latest)
			if row.ReferencePrice > 0 {
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
