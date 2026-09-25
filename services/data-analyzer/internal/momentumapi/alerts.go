package momentumapi

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/konsbe/trading-agent/services/data-analyzer/internal/store"
)

// GET /api/v1/alerts — full-stock-analysis addendum §3.2. Read-only over
// fired_alerts, newest first. symbol and since are optional and combinable;
// with neither, the newest alertsDefaultLimit rows are the feed. Not cached:
// the bot's scan adds rows every 5 minutes and the query is index-bounded.

const (
	alertsDefaultLimit = 100
	alertsMaxLimit     = 500
)

type alertsResponse struct {
	// Symbol and Since echo the filters applied (null when not given).
	Symbol *string `json:"symbol"`
	Since  *string `json:"since"`
	Limit  int     `json:"limit"`
	// HasMore: more rows match than Limit; narrow with since or raise limit.
	HasMore bool       `json:"has_more"`
	Alerts  []alertOut `json:"alerts"`
}

type alertOut struct {
	ID           int64    `json:"id"`
	Symbol       string   `json:"symbol"`
	ExchangeType string   `json:"exchange_type"`
	AlertType    string   `json:"alert_type"`
	Interval     string   `json:"interval"`
	Value        *float64 `json:"value"`
	Severity     string   `json:"severity"`
	Message      string   `json:"message"`
	FiredAt      string   `json:"fired_at"` // RFC3339, UTC
}

func (s *Server) handleAlerts(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	var f store.AlertFilter
	resp := alertsResponse{Alerts: []alertOut{}}

	if raw := strings.TrimSpace(q.Get("symbol")); raw != "" {
		sym := strings.ToUpper(raw)
		if !symbolPattern(sym) {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid_symbol"})
			return
		}
		f.Symbol, resp.Symbol = &sym, &sym
	}
	if raw := strings.TrimSpace(q.Get("since")); raw != "" {
		d, err := time.Parse(time.DateOnly, raw)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid_since"})
			return
		}
		day := d.Format(time.DateOnly)
		f.Since, resp.Since = &d, &day
	}
	resp.Limit = alertsDefaultLimit
	if raw := strings.TrimSpace(q.Get("limit")); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > alertsMaxLimit {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid_limit"})
			return
		}
		resp.Limit = n
	}

	// One extra row tells has_more without a count query.
	f.Limit = resp.Limit + 1
	rows, err := s.cfg.Store.ListAlerts(r.Context(), f)
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	if len(rows) > resp.Limit {
		rows, resp.HasMore = rows[:resp.Limit], true
	}
	for _, a := range rows {
		resp.Alerts = append(resp.Alerts, alertOut{
			ID: a.ID, Symbol: a.Symbol, ExchangeType: a.ExchangeType, AlertType: a.AlertType,
			Interval: a.Interval, Value: finite(a.Value), Severity: a.Severity, Message: a.Message,
			FiredAt: a.FiredAt.UTC().Format(time.RFC3339),
		})
	}
	writeJSON(w, http.StatusOK, resp)
}
