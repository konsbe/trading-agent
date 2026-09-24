package momentumapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

// Backtest Lab (docs/MOMENTUM_SCANNER_API.md, Addendum: Backtest Lab).
//
//	GET /api/v1/backtest-lab/report
//
// A frozen, dated report of the finished Phase 1/2 research, read from
// shared/content/backtest_lab_report.json at startup. Nothing here queries live
// data or computes a statistic: the numbers were computed once, under a
// pre-registered protocol, and published. Serving them from a query would let
// them drift from the report and imply the analysis can be re-run, which Phase
// 2's closure (§2.4) rejects.

// BacktestReport is the loaded, validated report, served byte-for-byte.
type BacktestReport struct {
	body []byte
}

// backtestReportShape is the part of the report the loader checks. The body is
// served as authored; this only refuses a file that could not be the report.
type backtestReportShape struct {
	Report struct {
		Version    string `json:"version"`
		Status     string `json:"status"`
		ClosedDate string `json:"closed_date"`
		Headline   string `json:"headline"`
	} `json:"report"`
	EntryGate *struct {
		Result *struct {
			PValue      *float64 `json:"p_value"`
			MHOddsRatio *float64 `json:"mh_odds_ratio"`
		} `json:"result"`
	} `json:"entry_gate"`
	ResearchRound1 *struct {
		Hypotheses []struct {
			ID      string `json:"id"`
			Verdict string `json:"verdict"`
		} `json:"hypotheses"`
	} `json:"research_round_1"`
	ClosingStatement string `json:"closing_statement"`
}

// LoadBacktestReport reads and validates the report. Like the caveats file,
// there is no compiled-in fallback: a missing or malformed file fails startup.
func LoadBacktestReport(path string) (BacktestReport, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return BacktestReport{}, fmt.Errorf("read backtest lab report: %w", err)
	}
	var shape backtestReportShape
	if err := json.Unmarshal(raw, &shape); err != nil {
		return BacktestReport{}, fmt.Errorf("parse backtest lab report %s: %w", path, err)
	}
	r := shape.Report
	switch {
	case r.Version == "" || r.Status == "" || r.ClosedDate == "" || r.Headline == "":
		return BacktestReport{}, fmt.Errorf("backtest lab report %s: report.version, status, closed_date and headline are required", path)
	case shape.EntryGate == nil || shape.EntryGate.Result == nil ||
		shape.EntryGate.Result.PValue == nil || shape.EntryGate.Result.MHOddsRatio == nil:
		return BacktestReport{}, fmt.Errorf("backtest lab report %s: entry_gate.result p_value and mh_odds_ratio are required", path)
	case shape.ResearchRound1 == nil || len(shape.ResearchRound1.Hypotheses) == 0:
		return BacktestReport{}, fmt.Errorf("backtest lab report %s: research_round_1.hypotheses is required", path)
	case shape.ClosingStatement == "":
		return BacktestReport{}, fmt.Errorf("backtest lab report %s: closing_statement is required", path)
	}
	for _, h := range shape.ResearchRound1.Hypotheses {
		if h.ID == "" || h.Verdict == "" {
			return BacktestReport{}, fmt.Errorf("backtest lab report %s: every hypothesis needs an id and a verdict", path)
		}
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, raw); err != nil {
		return BacktestReport{}, fmt.Errorf("compact backtest lab report %s: %w", path, err)
	}
	return BacktestReport{body: compact.Bytes()}, nil
}

// backtestReportMaxAge: the report changes only when someone deliberately
// authors a new version, never on a schedule, so it is not held to the
// 5-minute TTL of daily-changing data.
const backtestReportMaxAge = "public, max-age=86400"

func (s *Server) handleBacktestReport(w http.ResponseWriter, _ *http.Request) {
	if len(s.cfg.BacktestReport.body) == 0 {
		// main refuses to start without the report; only a miswired Server gets here.
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "backtest_report_not_loaded"})
		return
	}
	w.Header().Set("Cache-Control", backtestReportMaxAge)
	writeBody(w, http.StatusOK, s.cfg.BacktestReport.body)
}
