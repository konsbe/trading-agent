package momentumapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/konsbe/trading-agent/services/data-analyzer/internal/momentum"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/store"
)

func trackedFixture() *fakeStore {
	st := fixtureStore()
	alerted := time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)
	evaluated := time.Date(2026, 9, 16, 0, 0, 0, 0, time.UTC)
	exitTS := time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC)
	reason := func(r string) *string { return &r }
	st.trackedCounts = store.TrackedCounts{Active: 2, Closed: 4}
	st.tracked = []store.TrackedPositionRow{
		// active, current price joined, evaluated through the latest scan
		{Symbol: "NEXR", Exchange: ptr("NASDAQ"), Bucket: "penny", Status: "active", AlertedTS: alerted,
			SessionsElapsed: 3, LastEvaluatedTS: &scanDay, ReferencePrice: 1.64, LatestClose: ptr(1.71), MaxGainPct: ptr(9.15)},
		// active, symbol stopped scanning: join missed, tracker behind
		{Symbol: "GONE", Bucket: "market", Status: "active", AlertedTS: alerted,
			SessionsElapsed: 2, LastEvaluatedTS: &evaluated, ReferencePrice: 5, MaxGainPct: ptr(0.0)},
	}
	for _, r := range []string{momentum.ExitBreakoutFailed, momentum.ExitLostVWAP, momentum.ExitMomentumStalled, momentum.ExitStopATR} {
		st.tracked = append(st.tracked, store.TrackedPositionRow{
			Symbol: "C_" + r, Bucket: "market", Status: "closed", AlertedTS: alerted, SessionsElapsed: 1,
			LastEvaluatedTS: &exitTS, ReferencePrice: 10, LatestClose: ptr(11.0), MaxGainPct: ptr(0.0),
			ExitReason: reason(r), ExitTS: &exitTS, ExitPrice: ptr(9.5), ExitPct: ptr(-5.0),
		})
	}
	return st
}

func trackedRows(t *testing.T, srv *Server, query string) (map[string]any, []map[string]any) {
	t.Helper()
	rec := get(t, srv, "/api/v1/scanner/tracked"+query)
	if rec.Code != http.StatusOK {
		t.Fatalf("%s: %d %s", query, rec.Code, rec.Body)
	}
	body := decode(t, rec)
	var rows []map[string]any
	for _, r := range body["tracked"].([]any) {
		rows = append(rows, r.(map[string]any))
	}
	return body, rows
}

func TestTracked_DefaultsToActiveWithSummaryOfAll(t *testing.T) {
	st := trackedFixture()
	body, rows := trackedRows(t, newTestServer(t, st, freshNow), "")
	if st.trackedAsked[0] != "active" || len(rows) != 2 {
		t.Fatalf("asked %v, got %d rows", st.trackedAsked, len(rows))
	}
	sum := body["summary"].(map[string]any)
	if sum["active_count"] != 2.0 || sum["closed_count"] != 4.0 {
		t.Errorf("summary = %v; counts are table-wide, not filtered", sum)
	}
	if st.trackedLatest[0] == nil || !st.trackedLatest[0].Equal(scanDay) {
		t.Errorf("latest scan passed to the join = %v, want %v", st.trackedLatest[0], scanDay)
	}
}

func TestTracked_ActiveRows(t *testing.T) {
	_, rows := trackedRows(t, newTestServer(t, trackedFixture(), freshNow), "?status=active")
	nexr, gone := rows[0], rows[1]
	if nexr["current_price"] != 1.71 || nexr["current_price_date"] != "2026-09-17" || nexr["evaluation_behind"] != false {
		t.Errorf("NEXR = %v", nexr)
	}
	if u := nexr["unrealized_pct"].(float64); u < 4.268 || u > 4.269 {
		t.Errorf("unrealized_pct = %v, want (1.71/1.64 - 1)*100 ≈ 4.27", u)
	}
	if nexr["sessions_elapsed"] != 3.0 || nexr["last_evaluated_date"] != "2026-09-17" || nexr["alerted_date"] != "2026-09-14" {
		t.Errorf("NEXR stored fields = %v", nexr)
	}
	if gone["current_price"] != nil || gone["unrealized_pct"] != nil {
		t.Errorf("GONE (join missed) = %v, want null current price, not a dropped row", gone)
	}
	if gone["evaluation_behind"] != true {
		t.Errorf("GONE evaluated through 09-16 with a 09-17 scan: evaluation_behind = %v", gone["evaluation_behind"])
	}
}

// Mutually exclusive by status: pinned rather than left to convention.
func TestTracked_UnrealizedAndExitAreMutuallyExclusive(t *testing.T) {
	_, rows := trackedRows(t, newTestServer(t, trackedFixture(), freshNow), "?status=all")
	if len(rows) != 6 {
		t.Fatalf("all = %d rows", len(rows))
	}
	for _, r := range rows {
		switch r["status"] {
		case "active":
			if r["exit_pct"] != nil || r["exit_price"] != nil || r["closed_date"] != nil {
				t.Errorf("active %v carries exit fields", r["symbol"])
			}
		case "closed":
			if r["unrealized_pct"] != nil || r["current_price"] != nil {
				t.Errorf("closed %v carries unrealized_pct/current_price", r["symbol"])
			}
			if r["exit_pct"] != -5.0 || r["closed_date"] != "2026-09-15" {
				t.Errorf("closed %v = %v", r["symbol"], r)
			}
		}
	}
}

// Every non-null exit_reason carries its own note, byte-for-byte from the
// shared caveats file — the same enforcement as EVIDENCE_CAVEAT.
func TestTracked_ExitReasonNotesAreTheSharedTextPerRule(t *testing.T) {
	raw, err := os.ReadFile(sharedCaveatsPath(t))
	if err != nil {
		t.Fatal(err)
	}
	var file struct {
		Notes map[string]string `json:"exit_reason_notes"`
	}
	if err := json.Unmarshal(raw, &file); err != nil {
		t.Fatal(err)
	}
	_, rows := trackedRows(t, newTestServer(t, trackedFixture(), freshNow), "?status=closed")
	seen := map[string]bool{}
	for _, r := range rows {
		reason, _ := r["exit_reason"].(string)
		note, _ := r["exit_reason_note"].(string)
		if reason == "" || note == "" {
			t.Fatalf("closed row %v: reason %q note %q", r["symbol"], reason, note)
		}
		if note != file.Notes[reason] {
			t.Errorf("%s note differs from shared exit_reason_notes.%s", reason, reason)
		}
		if seen[note] {
			t.Errorf("note for %s is shared with another rule; notes must be specific per rule", reason)
		}
		seen[note] = true
	}
	for _, r := range rows {
		if r["status"] == "active" && r["exit_reason_note"] != nil {
			t.Errorf("active row with a note: %v", r)
		}
	}
}

func TestTracked_NoScanStillListsRowsWithoutPrices(t *testing.T) {
	st := trackedFixture()
	st.hasScan = false
	_, rows := trackedRows(t, newTestServer(t, st, freshNow), "")
	if len(rows) != 2 || rows[0]["current_price"] != nil || rows[0]["evaluation_behind"] != false {
		t.Errorf("rows without a scan = %v", rows)
	}
	if st.trackedLatest[0] != nil {
		t.Error("no scan: the price join must get a nil date")
	}
}

func TestTracked_EmptyIsNotAnError(t *testing.T) {
	st := fixtureStore()
	body, rows := trackedRows(t, newTestServer(t, st, freshNow), "?status=closed")
	if len(rows) != 0 || body["tracked"] == nil {
		t.Errorf("empty = %v; want tracked: [] not null", body)
	}
	if sum := body["summary"].(map[string]any); sum["active_count"] != 0.0 || sum["closed_count"] != 0.0 {
		t.Errorf("summary = %v", sum)
	}
}

func TestTracked_Errors(t *testing.T) {
	rec := get(t, newTestServer(t, trackedFixture(), freshNow), "/api/v1/scanner/tracked?status=open")
	if rec.Code != http.StatusBadRequest || decode(t, rec)["error"] != "invalid_status_param" {
		t.Errorf("bad status: %d %s", rec.Code, rec.Body)
	}
	st := trackedFixture()
	st.queryErr, st.pingErr = errors.New("down"), errors.New("down")
	rec = get(t, newTestServer(t, st, freshNow), "/api/v1/scanner/tracked")
	if rec.Code != http.StatusServiceUnavailable || decode(t, rec)["error"] != "database_unavailable" {
		t.Errorf("db down: %d %s", rec.Code, rec.Body)
	}
}

func TestLoadCaveats_RequiresEveryExitReasonNote(t *testing.T) {
	c := loadSharedCaveats(t)
	for _, r := range exitReasons {
		if c.ExitReasonNotes[r] == "" {
			t.Errorf("shared file has no note for %s", r)
		}
	}
	path := t.TempDir() + "/missing-note.json"
	body, _ := json.Marshal(map[string]any{
		"evidence_caveat": "x", "research_score_caveat": "y",
		"exit_reason_notes": map[string]string{"breakout_failed": "z"},
	})
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadCaveats(path); err == nil {
		t.Error("a caveats file missing exit-reason notes must fail at startup")
	}
}
