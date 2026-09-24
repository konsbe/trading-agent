package momentumapi

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"time"

	"github.com/konsbe/trading-agent/services/data-analyzer/internal/store"
)

// Data Source status (docs/MOMENTUM_SCANNER_API.md, Addendum: Data Source).
//
//	GET /api/v1/data-sources/status
//
// The one page whose purpose IS current operational health, so it reports live
// state — but only when asked: no polling, no push, a short cache. Read-only.

// dataProviders are the budgets this page reports, in display order. An
// explicit list, never "every api_rate_budget row": the table also holds a
// retired provider and rows left behind by the limiter's integration tests.
var dataProviders = []struct{ key, role string }{
	{"tiingo", "primary bar provider"},
	{"finnhub", "universe, quotes, fundamentals"},
}

const (
	statusClean               = "clean"
	statusCompletedAfterRetry = "completed_after_retry"
	statusPending             = "pending"
	statusFailed              = "failed"
	statusNotRun              = "not_run"
	statusNotRecorded         = "not_recorded"

	sectionUnavailable = "unavailable"
	secondsPerDay      = 86400
)

type dataSourcesResponse struct {
	CheckedAt string `json:"checked_at"`
	// Providers is map[string]providerStatus, or the string "unavailable" when
	// its query failed; DailyChain likewise. One failing section never hides
	// the other.
	Providers      any      `json:"providers"`
	DailyChain     any      `json:"daily_chain"`
	Overall        string   `json:"overall"`
	OverallReasons []string `json:"overall_reasons"`
}

type providerStatus struct {
	Role string `json:"role"`
	// DailyUsed is today's count in the provider's own reset window.
	DailyUsed        float64 `json:"daily_used"`
	DailyWindowStart *string `json:"daily_window_start"`
	DailyResetTZ     string  `json:"daily_reset_tz"`
	// DailyLimit / DailyUsedPct are null when the provider has no daily cap.
	DailyLimit   *float64 `json:"daily_limit"`
	DailyUsedPct *float64 `json:"daily_used_pct"`
	// RatePerSec is the enforced constraint for a rate-limited provider;
	// TheoreticalDailyCapacity is rate × 86,400 — what could be spent in a day,
	// NOT a quota, and never used as a denominator.
	RatePerSec               float64 `json:"rate_per_sec"`
	TheoreticalDailyCapacity float64 `json:"theoretical_daily_capacity"`
	// DegradedCount24h is always null: the limiter counts degradations in each
	// ingestion process's memory and never persists them.
	DegradedCount24h *int `json:"degraded_count_24h"`
}

type dailyChainStatus struct {
	LastCleanSession *string         `json:"last_clean_session"`
	Sessions         []sessionStatus `json:"sessions"`
	SessionsShown    int             `json:"sessions_shown"`
}

type sessionStatus struct {
	Session string `json:"session"`
	// BarsCoverageNowPct is computed at request time with momentum-daily's
	// definition. It can differ from the coverage that gated the run.
	BarsCoverageNowPct *float64 `json:"bars_coverage_now_pct"`
	Attempts           int      `json:"attempts"`
	ScannerCompleted   bool     `json:"scanner_completed"`
	TrackerCompleted   bool     `json:"tracker_completed"`
	GaveUpReason       *string  `json:"gave_up_reason"`
	LastError          *string  `json:"last_error"`
	Status             string   `json:"status"`
	// Note explains a status that needs one (not_run, not_recorded, a failure
	// without a recorded give-up).
	Note *string `json:"note,omitempty"`
}

// sessionStatusOf is the status rule, pure so every case is testable.
//
// Sessions come from the NYSE calendar, not from momentum_chain_runs rows, so a
// session the daemon never attempted is visible as not_run instead of absent.
func sessionStatusOf(session time.Time, run store.ChainRun, found bool,
	firstRecorded time.Time, hasFirst bool, now time.Time, giveUpAfter time.Duration) (string, *string) {
	note := func(s string) *string { return &s }
	if found && run.TrackerCompletedAt != nil && run.ScannerCompletedAt != nil {
		if run.Attempts == 1 && run.LastError == nil && run.GaveUpAt == nil {
			return statusClean, nil
		}
		return statusCompletedAfterRetry, nil
	}
	if found && run.GaveUpAt != nil {
		return statusFailed, nil
	}
	closeAt := time.Date(session.Year(), session.Month(), session.Day(), sessionCloseHour, 0, 0, 0, newYork)
	if now.Before(closeAt.Add(giveUpAfter)) {
		return statusPending, nil
	}
	if found {
		return statusFailed, note("did not finish and no give-up was recorded — momentum-daily was probably not running")
	}
	if hasFirst && !session.Before(firstRecorded) {
		return statusNotRun, note("no attempt was recorded — momentum-daily was not running, or the machine was off")
	}
	return statusNotRecorded, note("before the chain recorded its runs (momentum_chain_runs)")
}

// recentClosedSessions are the last n NYSE sessions whose 16:00 close has
// passed, newest first.
func recentClosedSessions(now time.Time, n int) ([]time.Time, error) {
	latest, err := ExpectedSession(now, 0)
	if err != nil {
		return nil, err
	}
	out := []time.Time{latest}
	day := latest
	for len(out) < n {
		day = day.AddDate(0, 0, -1)
		ok, err := IsTradingDay(day)
		if err != nil {
			return out, nil // calendar ends: show what it covers
		}
		if ok {
			out = append(out, day)
		}
	}
	return out, nil
}

func round1(v float64) float64 { return math.Round(v*10) / 10 }

func (s *Server) handleDataSourcesStatus(w http.ResponseWriter, r *http.Request) {
	const key = "data-sources"
	w.Header().Set("Cache-Control", "no-cache")
	if body, ok := s.statusCache.get(key); ok {
		writeBody(w, http.StatusOK, body)
		return
	}
	ctx := r.Context()
	now := s.cfg.Now()
	resp := dataSourcesResponse{CheckedAt: now.UTC().Format(time.RFC3339), OverallReasons: []string{}}
	var reasons []string

	// ── providers ──
	keys := make([]string, len(dataProviders))
	for i, p := range dataProviders {
		keys[i] = p.key
	}
	budgets, provErr := s.cfg.Store.ProviderBudgets(ctx, keys)
	if provErr != nil {
		s.cfg.Log.Warn("momentum-api: data sources: provider budgets", "err", provErr)
		resp.Providers = sectionUnavailable
		reasons = append(reasons, "provider budgets unavailable")
	} else {
		byKey := map[string]store.ProviderBudget{}
		for _, b := range budgets {
			byKey[b.Key] = b
		}
		providers := map[string]providerStatus{}
		for _, p := range dataProviders {
			b, ok := byKey[p.key]
			if !ok {
				reasons = append(reasons, fmt.Sprintf("no budget row for %s", p.key))
				continue
			}
			ps := providerStatus{
				Role: p.role, DailyUsed: b.DailyUsed, DailyResetTZ: b.DailyResetTZ,
				DailyLimit: b.DailyLimit, RatePerSec: b.RefillPerSec,
				TheoreticalDailyCapacity: b.RefillPerSec * secondsPerDay,
			}
			if b.DailyWindowStart != nil {
				d := b.DailyWindowStart.Format(time.DateOnly)
				ps.DailyWindowStart = &d
			}
			if b.DailyLimit != nil && *b.DailyLimit > 0 {
				pct := round1(b.DailyUsed / *b.DailyLimit * 100)
				ps.DailyUsedPct = &pct
				if pct >= s.cfg.BudgetAttentionPct {
					reasons = append(reasons, fmt.Sprintf("%s at %.1f%% of its daily budget (threshold %.0f%%)", p.key, pct, s.cfg.BudgetAttentionPct))
				}
			}
			providers[p.key] = ps
		}
		resp.Providers = providers
	}

	// ── daily chain ──
	chain, chainErr := s.dailyChain(r, now)
	if chainErr != nil {
		s.cfg.Log.Warn("momentum-api: data sources: daily chain", "err", chainErr)
		resp.DailyChain = sectionUnavailable
		reasons = append(reasons, "daily chain status unavailable")
	} else {
		resp.DailyChain = chain
		// The latest session that is no longer in progress decides health; a
		// session still inside its window is normal, not a problem.
		for _, ss := range chain.Sessions {
			if ss.Status == statusPending {
				continue
			}
			if ss.Status != statusClean {
				reasons = append(reasons, fmt.Sprintf("latest finished session %s is %s", ss.Session, ss.Status))
			}
			break
		}
	}

	if provErr != nil && chainErr != nil {
		if pingErr := s.cfg.Store.Ping(ctx); pingErr != nil {
			// The status page cannot reach the database: that IS the problem it reports.
			writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "database_unavailable"})
			return
		}
	}

	resp.Overall = "healthy"
	if len(reasons) > 0 {
		resp.Overall = "attention"
		resp.OverallReasons = reasons
	}
	body, err := jsonMarshal(resp)
	if err != nil {
		s.cfg.Log.Error("momentum-api: encode data sources", "err", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "internal_error"})
		return
	}
	s.statusCache.set(key, body)
	writeBody(w, http.StatusOK, body)
}

func (s *Server) dailyChain(r *http.Request, now time.Time) (dailyChainStatus, error) {
	ctx := r.Context()
	n := s.cfg.SessionsShown
	sessions, err := recentClosedSessions(now, n)
	if err != nil {
		return dailyChainStatus{}, err
	}
	runs, first, hasFirst, err := s.cfg.Store.ChainRunsBetween(ctx, sessions[len(sessions)-1], sessions[0])
	if err != nil {
		return dailyChainStatus{}, err
	}
	coverage, err := s.cfg.Store.SessionCoverage(ctx, sessions, s.cfg.BarSource)
	if err != nil {
		return dailyChainStatus{}, err
	}
	lastClean, err := s.cfg.Store.LastCleanSession(ctx)
	if err != nil {
		return dailyChainStatus{}, err
	}
	out := dailyChainStatus{Sessions: []sessionStatus{}, SessionsShown: n}
	if lastClean != nil {
		d := lastClean.UTC().Format(time.DateOnly)
		out.LastCleanSession = &d
	}
	for _, sess := range sessions {
		key := sess.Format(time.DateOnly)
		run, found := runs[key]
		status, note := sessionStatusOf(sess, run, found, first, hasFirst, now, s.cfg.ChainGiveUpAfter)
		ss := sessionStatus{Session: key, Status: status, Note: note, Attempts: run.Attempts,
			ScannerCompleted: found && run.ScannerCompletedAt != nil,
			TrackerCompleted: found && run.TrackerCompletedAt != nil,
			LastError:        run.LastError,
		}
		if found && run.GaveUpAt != nil {
			ss.GaveUpReason = run.LastError
		}
		if pct, ok := coverage[key]; ok {
			v := round1(pct)
			ss.BarsCoverageNowPct = &v
		}
		out.Sessions = append(out.Sessions, ss)
	}
	return out, nil
}

func jsonMarshal(v any) ([]byte, error) { return json.Marshal(v) }
