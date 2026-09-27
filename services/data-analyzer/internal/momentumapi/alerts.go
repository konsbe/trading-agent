package momentumapi

import (
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/konsbe/trading-agent/services/data-analyzer/internal/store"
)

// GET /api/v1/alerts — full-stock-analysis addendum §3.2, widened for Alarm
// History (2026-09-27). Read-only over fired_alerts, newest first. Filters are
// optional and combinable:
//
//	symbol        one symbol
//	alert_type    one or more (repeated or comma-separated)
//	severity      one or more of info, notice, warning
//	since, until  a date (UTC midnight; until is then the end of that day) or
//	              an RFC3339 timestamp — the page sends local-day bounds;
//	              since inclusive, until exclusive
//	before        an alert id: only rows (raw) or groups (grouped) older than it
//	limit         rows or groups per page (default 100, max 500)
//	mode          raw (default) or grouped: one row per symbol + alert type in
//	              the range, with count, first / last fired and the latest alert
//
// Every response also carries the table-wide alert types, the earliest record
// and the verbatim heuristic_ta_caveat. Not cached: the bot's scan adds rows
// every 5 minutes and the queries are index-bounded.

const (
	alertsDefaultLimit = 100
	alertsMaxLimit     = 500
)

var (
	alertTypePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,39}$`)
	alertSeverities  = map[string]bool{"info": true, "notice": true, "warning": true}
)

type alertsResponse struct {
	// Filters as applied (null / empty when not given).
	Symbol     *string  `json:"symbol"`
	AlertTypes []string `json:"alert_types"`
	Severities []string `json:"severities"`
	Since      *string  `json:"since"`
	Until      *string  `json:"until"`
	Before     *int64   `json:"before"`
	Limit      int      `json:"limit"`
	Mode       string   `json:"mode"`
	// HasMore: more rows (or groups) match; NextBefore is the cursor for the
	// next page (pass it as before), set exactly when HasMore.
	HasMore    bool         `json:"has_more"`
	NextBefore *int64       `json:"next_before"`
	Alerts     []alertOut   `json:"alerts"` // raw mode; [] in grouped mode
	Groups     []alertGroup `json:"groups"` // grouped mode; [] in raw mode

	// Types: every alert type that has fired, for the filter. RecordsStart:
	// the earliest fired_at (null when none) — earlier periods have no record.
	Types        []string `json:"types"`
	RecordsStart *string  `json:"records_start"`
	Caveat       string   `json:"caveat"`
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

type alertGroup struct {
	Symbol       string   `json:"symbol"`
	ExchangeType string   `json:"exchange_type"`
	AlertType    string   `json:"alert_type"`
	Count        int      `json:"count"`
	FirstFiredAt string   `json:"first_fired_at"`
	LastFiredAt  string   `json:"last_fired_at"`
	Latest       alertOut `json:"latest"`
}

func toAlertOut(a store.AlertRow) alertOut {
	return alertOut{
		ID: a.ID, Symbol: a.Symbol, ExchangeType: a.ExchangeType, AlertType: a.AlertType,
		Interval: a.Interval, Value: finite(a.Value), Severity: a.Severity, Message: a.Message,
		FiredAt: a.FiredAt.UTC().Format(time.RFC3339),
	}
}

// multi collects a repeated and/or comma-separated query parameter.
func multi(q map[string][]string, key string) []string {
	var out []string
	seen := map[string]bool{}
	for _, raw := range q[key] {
		for _, v := range strings.Split(raw, ",") {
			if v = strings.ToLower(strings.TrimSpace(v)); v != "" && !seen[v] {
				seen[v] = true
				out = append(out, v)
			}
		}
	}
	return out
}

// alertBound parses since / until: a date is UTC midnight (for until, the end
// of that day), anything else must be RFC3339. The echo is the normalised form.
func alertBound(raw string, endOfDay bool) (time.Time, string, bool) {
	if d, err := time.Parse(time.DateOnly, raw); err == nil {
		echo := d.Format(time.DateOnly)
		if endOfDay {
			d = d.AddDate(0, 0, 1)
		}
		return d, echo, true
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, "", false
	}
	return t.UTC(), t.UTC().Format(time.RFC3339), true
}

func (s *Server) handleAlerts(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	var f store.AlertFilter
	resp := alertsResponse{Alerts: []alertOut{}, Groups: []alertGroup{}, AlertTypes: []string{}, Severities: []string{},
		Types: []string{}, Caveat: s.cfg.Caveats.HeuristicTA}
	bad := func(code string) { writeJSON(w, http.StatusBadRequest, errorResponse{Error: code}) }

	if raw := strings.TrimSpace(q.Get("symbol")); raw != "" {
		sym := strings.ToUpper(raw)
		if !symbolPattern(sym) {
			bad("invalid_symbol")
			return
		}
		f.Symbol, resp.Symbol = &sym, &sym
	}
	for _, t := range multi(q, "alert_type") {
		if !alertTypePattern.MatchString(t) {
			bad("invalid_alert_type")
			return
		}
		f.AlertTypes = append(f.AlertTypes, t)
	}
	for _, sev := range multi(q, "severity") {
		if !alertSeverities[sev] {
			bad("invalid_severity")
			return
		}
		f.Severities = append(f.Severities, sev)
	}
	if f.AlertTypes != nil {
		resp.AlertTypes = f.AlertTypes
	}
	if f.Severities != nil {
		resp.Severities = f.Severities
	}
	if raw := strings.TrimSpace(q.Get("since")); raw != "" {
		t, echo, ok := alertBound(raw, false)
		if !ok {
			bad("invalid_since")
			return
		}
		f.Since, resp.Since = &t, &echo
	}
	if raw := strings.TrimSpace(q.Get("until")); raw != "" {
		t, echo, ok := alertBound(raw, true)
		if !ok {
			bad("invalid_until")
			return
		}
		f.Until, resp.Until = &t, &echo
	}
	if f.Since != nil && f.Until != nil && !f.Since.Before(*f.Until) {
		bad("invalid_range")
		return
	}
	if raw := strings.TrimSpace(q.Get("before")); raw != "" {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || id < 1 {
			bad("invalid_before")
			return
		}
		f.Before, resp.Before = &id, &id
	}
	resp.Limit = alertsDefaultLimit
	if raw := strings.TrimSpace(q.Get("limit")); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > alertsMaxLimit {
			bad("invalid_limit")
			return
		}
		resp.Limit = n
	}
	resp.Mode = "raw"
	switch m := strings.TrimSpace(q.Get("mode")); m {
	case "", "raw":
	case "grouped":
		resp.Mode = m
	default:
		bad("invalid_mode")
		return
	}

	ctx := r.Context()
	meta, err := s.cfg.Store.AlertMeta(ctx)
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	resp.Types = meta.Types
	if resp.Types == nil {
		resp.Types = []string{}
	}
	if meta.EarliestFired != nil {
		v := meta.EarliestFired.UTC().Format(time.RFC3339)
		resp.RecordsStart = &v
	}

	// One extra row (or group) tells has_more without a count query.
	f.Limit = resp.Limit + 1
	if resp.Mode == "grouped" {
		groups, err := s.cfg.Store.ListAlertGroups(ctx, f)
		if err != nil {
			s.storeError(w, r, err)
			return
		}
		if len(groups) > resp.Limit {
			groups, resp.HasMore = groups[:resp.Limit], true
		}
		for _, g := range groups {
			resp.Groups = append(resp.Groups, alertGroup{
				Symbol: g.Symbol, ExchangeType: g.Latest.ExchangeType, AlertType: g.AlertType, Count: g.Count,
				FirstFiredAt: g.FirstFiredAt.UTC().Format(time.RFC3339), LastFiredAt: g.LastFiredAt.UTC().Format(time.RFC3339),
				Latest: toAlertOut(g.Latest),
			})
		}
		if resp.HasMore {
			id := groups[len(groups)-1].Latest.ID
			resp.NextBefore = &id
		}
		writeJSON(w, http.StatusOK, resp)
		return
	}
	rows, err := s.cfg.Store.ListAlerts(ctx, f)
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	if len(rows) > resp.Limit {
		rows, resp.HasMore = rows[:resp.Limit], true
	}
	for _, a := range rows {
		resp.Alerts = append(resp.Alerts, toAlertOut(a))
	}
	if resp.HasMore {
		id := rows[len(rows)-1].ID
		resp.NextBefore = &id
	}
	writeJSON(w, http.StatusOK, resp)
}
