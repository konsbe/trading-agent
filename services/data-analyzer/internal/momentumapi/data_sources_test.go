package momentumapi

import (
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/konsbe/trading-agent/services/data-analyzer/internal/store"
)

func day(s string) time.Time {
	d, _ := time.Parse(time.DateOnly, s)
	return d
}

func TestSessionStatusOf(t *testing.T) {
	now := ny("2026-09-18 09:00") // Friday morning; Thursday 09-17's window closed at 06:00
	first := day("2026-09-14")
	set := time.Now()
	errMsg := "momentum-tracker: exit status 1"
	cases := []struct {
		name    string
		session string
		run     store.ChainRun
		found   bool
		now     time.Time
		want    string
	}{
		{"clean", "2026-09-17", store.ChainRun{Attempts: 1, ScannerCompletedAt: &set, TrackerCompletedAt: &set}, true, now, statusClean},
		{"finished on a retry", "2026-09-17", store.ChainRun{Attempts: 2, ScannerCompletedAt: &set, TrackerCompletedAt: &set, LastError: &errMsg}, true, now, statusCompletedAfterRetry},
		{"gave up", "2026-09-17", store.ChainRun{Attempts: 3, GaveUpAt: &set, LastError: &errMsg}, true, now, statusFailed},
		{"attempted, never finished, no give-up", "2026-09-17", store.ChainRun{Attempts: 1, ScannerCompletedAt: &set}, true, now, statusFailed},
		{"in its window, nothing yet", "2026-09-17", store.ChainRun{}, false, ny("2026-09-17 19:00"), statusPending},
		{"in its window, scan done, tracker not yet", "2026-09-17", store.ChainRun{Attempts: 1, ScannerCompletedAt: &set}, true, ny("2026-09-17 19:00"), statusPending},
		{"missed after records began", "2026-09-16", store.ChainRun{}, false, now, statusNotRun},
		{"before records began", "2026-09-11", store.ChainRun{}, false, now, statusNotRecorded},
	}
	for _, c := range cases {
		got, _ := sessionStatusOf(day(c.session), c.run, c.found, first, true, c.now, 14*time.Hour)
		if got != c.want {
			t.Errorf("%s: %s, want %s", c.name, got, c.want)
		}
	}
	if got, _ := sessionStatusOf(day("2026-09-16"), store.ChainRun{}, false, time.Time{}, false, now, 14*time.Hour); got != statusNotRecorded {
		t.Errorf("empty table: %s, want not_recorded", got)
	}
}

func TestRecentClosedSessionsSkipWeekendsAndHolidays(t *testing.T) {
	got, err := recentClosedSessions(ny("2026-09-08 09:00"), 3) // Tuesday after Labor Day (09-07)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"2026-09-04", "2026-09-03", "2026-09-02"}
	for i, d := range got {
		if d.Format(time.DateOnly) != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func statusFixture() *fakeStore {
	st := fixtureStore()
	set := time.Now()
	errMsg := "bars below 95% coverage 14h0m0s after the close"
	first := day("2026-09-15")
	today := day("2026-09-18")
	st.firstRun = &first
	st.lastClean = ptr(day("2026-09-16"))
	st.chainRuns = map[string]store.ChainRun{
		"2026-09-16": {Session: day("2026-09-16"), Attempts: 1, ScannerCompletedAt: &set, TrackerCompletedAt: &set},
		"2026-09-15": {Session: day("2026-09-15"), Attempts: 0, GaveUpAt: &set, LastError: &errMsg},
	}
	st.coverage = map[string]float64{"2026-09-17": 12.345, "2026-09-16": 99.84}
	st.budgets = []store.ProviderBudget{
		{Key: "tiingo", RefillPerSec: 2, DailyLimit: ptr(90000.0), DailyUsed: 4975, DailyWindowStart: &today, WindowIsCurrent: true, DailyResetTZ: "EST"},
		{Key: "finnhub", RefillPerSec: 1, DailyUsed: 9993, DailyWindowStart: &today, WindowIsCurrent: true, DailyResetTZ: "UTC"},
	}
	return st
}

func sessionsOf(t *testing.T, body map[string]any) map[string]map[string]any {
	t.Helper()
	out := map[string]map[string]any{}
	for _, s := range body["daily_chain"].(map[string]any)["sessions"].([]any) {
		m := s.(map[string]any)
		out[m["session"].(string)] = m
	}
	return out
}

func TestDataSources_StatusesAndCoverageNow(t *testing.T) {
	// Thursday 19:00 New York: 09-17 is inside its window.
	body := decode(t, get(t, newTestServer(t, statusFixture(), ny("2026-09-17 19:00")), "/api/v1/data-sources/status"))
	ss := sessionsOf(t, body)
	want := map[string]string{
		"2026-09-17": statusPending, "2026-09-16": statusClean, "2026-09-15": statusFailed,
		"2026-09-14": statusNotRecorded, // before the first recorded row
	}
	for d, w := range want {
		if ss[d]["status"] != w {
			t.Errorf("%s: status %v, want %s", d, ss[d]["status"], w)
		}
	}
	if ss["2026-09-15"]["gave_up_reason"] != "bars below 95% coverage 14h0m0s after the close" {
		t.Errorf("failed session gave_up_reason = %v", ss["2026-09-15"]["gave_up_reason"])
	}
	if ss["2026-09-16"]["gave_up_reason"] != nil {
		t.Errorf("clean session gave_up_reason = %v, want null", ss["2026-09-16"]["gave_up_reason"])
	}
	if ss["2026-09-17"]["bars_coverage_now_pct"] != 12.3 || ss["2026-09-16"]["bars_coverage_now_pct"] != 99.8 {
		t.Errorf("coverage now = %v / %v", ss["2026-09-17"]["bars_coverage_now_pct"], ss["2026-09-16"]["bars_coverage_now_pct"])
	}
	if v, present := ss["2026-09-14"]["bars_coverage_now_pct"]; !present || v != nil {
		t.Errorf("no coverage must be explicit null, got %v (present=%v)", v, present)
	}
	chain := body["daily_chain"].(map[string]any)
	if chain["last_clean_session"] != "2026-09-16" || chain["sessions_shown"] != 7.0 || len(ss) != 7 {
		t.Errorf("chain = last_clean %v, shown %v, %d sessions", chain["last_clean_session"], chain["sessions_shown"], len(ss))
	}
	// Pending is normal: health is decided by the latest FINISHED session (09-16, clean).
	if body["overall"] != "healthy" {
		t.Errorf("overall = %v (%v), want healthy", body["overall"], body["overall_reasons"])
	}
}

func TestDataSources_MissedSessionIsNotRunNotAbsent(t *testing.T) {
	st := statusFixture()
	body := decode(t, get(t, newTestServer(t, st, ny("2026-09-18 09:00")), "/api/v1/data-sources/status"))
	ss := sessionsOf(t, body)
	if ss["2026-09-17"]["status"] != statusNotRun {
		t.Errorf("09-17 (no row, window over) = %v, want not_run", ss["2026-09-17"]["status"])
	}
	if body["overall"] != "attention" || !strings.Contains(strings.Join(toStrings(body["overall_reasons"]), ";"), "2026-09-17 is not_run") {
		t.Errorf("overall = %v %v; a missed latest session needs attention", body["overall"], body["overall_reasons"])
	}
}

func TestDataSources_BudgetOverThresholdFlipsOverallEvenWhenChainIsClean(t *testing.T) {
	st := statusFixture()
	st.budgets[0].DailyUsed = 85000 // 94.4% of 90,000
	body := decode(t, get(t, newTestServer(t, st, ny("2026-09-17 19:00")), "/api/v1/data-sources/status"))
	if ss := sessionsOf(t, body); ss["2026-09-16"]["status"] != statusClean {
		t.Fatal("fixture: the chain must be clean for this test")
	}
	if body["overall"] != "attention" {
		t.Errorf("overall = %v; tiingo at 94%% must need attention on its own", body["overall"])
	}
	tiingo := body["providers"].(map[string]any)["tiingo"].(map[string]any)
	if tiingo["daily_used_pct"] != 94.4 {
		t.Errorf("tiingo pct = %v", tiingo["daily_used_pct"])
	}
}

func TestDataSources_FinnhubHasNoDailyLimitAndIsNeverAPercentage(t *testing.T) {
	st := statusFixture()
	st.budgets[1].DailyUsed = 86000 // near rate × 86,400: still not a percentage of anything
	body := decode(t, get(t, newTestServer(t, st, ny("2026-09-17 19:00")), "/api/v1/data-sources/status"))
	fh := body["providers"].(map[string]any)["finnhub"].(map[string]any)
	for _, k := range []string{"daily_limit", "daily_used_pct", "degraded_count_24h"} {
		if v, present := fh[k]; !present || v != nil {
			t.Errorf("finnhub %s = %v (present=%v), want explicit null", k, v, present)
		}
	}
	if fh["rate_per_sec"] != 1.0 || fh["theoretical_daily_capacity"] != 86400.0 {
		t.Errorf("finnhub rate/capacity = %v / %v", fh["rate_per_sec"], fh["theoretical_daily_capacity"])
	}
	if body["overall"] != "healthy" {
		t.Errorf("overall = %v %v; capacity must never act as a quota", body["overall"], body["overall_reasons"])
	}
	tiingo := body["providers"].(map[string]any)["tiingo"].(map[string]any)
	if v, present := tiingo["degraded_count_24h"]; !present || v != nil {
		t.Errorf("tiingo degraded_count_24h = %v, want null (not recorded)", v)
	}
}

func TestDataSources_OneFailingSectionDoesNotHideTheOther(t *testing.T) {
	st := statusFixture()
	st.chainErr = errors.New("relation momentum_chain_runs does not exist")
	body := decode(t, get(t, newTestServer(t, st, ny("2026-09-17 19:00")), "/api/v1/data-sources/status"))
	if body["daily_chain"] != "unavailable" {
		t.Errorf("daily_chain = %v, want \"unavailable\"", body["daily_chain"])
	}
	if p, ok := body["providers"].(map[string]any); !ok || p["tiingo"] == nil {
		t.Errorf("providers must still carry real data: %v", body["providers"])
	}
	if body["overall"] != "attention" {
		t.Errorf("overall = %v; an unavailable section is not healthy", body["overall"])
	}

	st = statusFixture()
	st.budgetErr = errors.New("boom")
	body = decode(t, get(t, newTestServer(t, st, ny("2026-09-17 19:00")), "/api/v1/data-sources/status"))
	if body["providers"] != "unavailable" || body["daily_chain"] == "unavailable" {
		t.Errorf("providers = %v, daily_chain = %v", body["providers"], body["daily_chain"])
	}
}

func TestDataSources_DatabaseDownIs503(t *testing.T) {
	st := statusFixture()
	st.chainErr, st.budgetErr = errors.New("conn refused"), errors.New("conn refused")
	st.pingErr = errors.New("conn refused")
	rec := get(t, newTestServer(t, st, ny("2026-09-17 19:00")), "/api/v1/data-sources/status")
	if rec.Code != http.StatusServiceUnavailable || decode(t, rec)["error"] != "database_unavailable" {
		t.Errorf("got %d %s", rec.Code, rec.Body)
	}
}

func TestDataSources_ShortCache(t *testing.T) {
	st := statusFixture()
	now := ny("2026-09-17 19:00")
	srv := newTestServer(t, st, now)
	clock := now
	srv.cfg.Now = func() time.Time { return clock }
	srv.statusCache.now = srv.cfg.Now
	first := decode(t, get(t, srv, "/api/v1/data-sources/status"))["checked_at"]
	clock = clock.Add(20 * time.Second)
	if decode(t, get(t, srv, "/api/v1/data-sources/status"))["checked_at"] != first || st.statusCalls != 1 {
		t.Errorf("a refresh within 30s must be served from cache (calls=%d)", st.statusCalls)
	}
	clock = clock.Add(15 * time.Second)
	if decode(t, get(t, srv, "/api/v1/data-sources/status"))["checked_at"] == first || st.statusCalls != 2 {
		t.Errorf("after 30s the page must re-query (calls=%d)", st.statusCalls)
	}
}

func toStrings(v any) []string {
	var out []string
	for _, x := range v.([]any) {
		out = append(out, x.(string))
	}
	return out
}
