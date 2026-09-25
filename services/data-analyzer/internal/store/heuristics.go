package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/heuristics"
)

// Reads and writes for cmd/heuristics-replay (migration 027).

// HeuristicUniverseRule is the replay universe, stated as it is queried.
const HeuristicUniverseRule = "equity_ohlcv symbols with interval '1Day' whose first bar is on or before 2016-09-30 (equities only; crypto excluded this round)"

// QueryHeuristicUniverse returns every equity symbol whose first 1Day bar is on
// or before cutoff, alphabetically.
func QueryHeuristicUniverse(ctx context.Context, q Querier, cutoff time.Time) ([]string, error) {
	rows, err := q.Query(ctx, `
		SELECT symbol FROM equity_ohlcv
		WHERE interval = '1Day'
		GROUP BY symbol
		HAVING min(ts) <= $1
		ORDER BY symbol`, cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// QueryHeuristicLockbox loads the region and the pilot symbols it excludes.
func QueryHeuristicLockbox(ctx context.Context, q Querier, regionKey string) (heuristics.Lockbox, error) {
	var lb heuristics.Lockbox
	var cohort string
	if err := q.QueryRow(ctx, `
		SELECT start_date::timestamp AT TIME ZONE 'UTC', end_date::timestamp AT TIME ZONE 'UTC', exclude_cohort
		FROM phase2_lockbox_region WHERE region_key = $1`, regionKey).Scan(&lb.Start, &lb.End, &cohort); err != nil {
		return lb, fmt.Errorf("lockbox region %s: %w", regionKey, err)
	}
	lb.Start, lb.End = lb.Start.UTC(), lb.End.UTC()
	if cohort != "phase1_pilot_450" {
		return lb, fmt.Errorf("lockbox region %s excludes cohort %q; this loader only knows momentum_pilot_cohort (phase1_pilot_450)", regionKey, cohort)
	}
	rows, err := q.Query(ctx, `SELECT symbol FROM momentum_pilot_cohort`)
	if err != nil {
		return lb, err
	}
	defer rows.Close()
	lb.Pilot = map[string]bool{}
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return lb, err
		}
		lb.Pilot[s] = true
	}
	return lb, rows.Err()
}

// QueryFREDSeries loads a whole FRED series, ascending, keyed by UTC date.
func QueryFREDSeries(ctx context.Context, q Querier, seriesID string) (heuristics.VIXSeries, error) {
	var v heuristics.VIXSeries
	rows, err := q.Query(ctx, `SELECT ts, value FROM macro_fred WHERE series_id = $1 ORDER BY ts`, seriesID)
	if err != nil {
		return v, err
	}
	defer rows.Close()
	for rows.Next() {
		var ts time.Time
		var val float64
		if err := rows.Scan(&ts, &val); err != nil {
			return v, err
		}
		u := ts.UTC()
		v.Dates = append(v.Dates, time.Date(u.Year(), u.Month(), u.Day(), 0, 0, 0, 0, time.UTC))
		v.Values = append(v.Values, val)
	}
	return v, rows.Err()
}

var episodeColumns = []string{
	"symbol", "t", "close_t",
	"rsi14", "macd_hist", "macd_bull_cross", "macd_bear_cross", "bb_squeeze", "trend_direction",
	"bull_flag", "bear_flag", "hs_found", "hs_neckline_break", "inv_hs_found", "inv_hs_neckline_break",
	"total_sweeps", "low_sweep_on_bar", "low_sweep_level", "high_sweep_on_bar", "high_sweep_level",
	"last_sweep_kind", "last_sweep_bar_close", "last_sweep_swept_level",
	"last_bullish_ob", "last_bearish_ob", "vix", "vix_regime", "rule_action", "rule_confluence",
	"sessions_since_prev_fire",
	"uptrend", "atr14", "atr_pct",
	"fwd_return_5s", "fwd_return_10s", "fwd_return_20s",
	"fwd_abs_move_5s", "fwd_abs_move_10s", "fwd_abs_move_20s",
	"label_complete", "in_lockbox", "harness_version",
}

func comparisonColumns() []string {
	cols := []string{"symbol", "t", "close_t"}
	for _, sig := range heuristics.Signals {
		cols = append(cols, "fired_"+string(sig))
	}
	return append(cols,
		"uptrend", "atr14", "atr_pct",
		"fwd_return_5s", "fwd_return_10s", "fwd_return_20s",
		"fwd_abs_move_5s", "fwd_abs_move_10s", "fwd_abs_move_20s",
		"label_complete", "in_lockbox", "harness_version")
}

func optF(ok bool, v float64) *float64 {
	if !ok {
		return nil
	}
	return &v
}

func labelValues(r heuristics.Row) []any {
	l := r.Labels
	return []any{
		l.FwdReturn[0], l.FwdReturn[1], l.FwdReturn[2],
		l.FwdAbsMove[0], l.FwdAbsMove[1], l.FwdAbsMove[2],
		l.Complete, r.InLockbox,
	}
}

func episodeValues(r heuristics.Row, version string) []any {
	s := r.Snap
	var trend, lastKind, ruleAction *string
	var lastClose, lastLevel, lowLevel, highLevel *float64
	var ruleConf *int
	if s.TrendOK {
		trend = &s.TrendDir
	}
	if s.LastSweep != nil {
		lastKind, lastClose, lastLevel = &s.LastSweep.Kind, &s.LastSweep.BarClose, &s.LastSweep.SweptLevel
	}
	low, high := s.OnBarSweep("low_sweep"), s.OnBarSweep("high_sweep")
	if low != nil {
		lowLevel = &low.SweptLevel
	}
	if high != nil {
		highLevel = &high.SweptLevel
	}
	if s.SweepRule != nil {
		ruleAction, ruleConf = &s.SweepRule.Action, &s.SweepRule.Confluence
	}
	var vixRegime *string
	if s.VIXOK {
		vixRegime = &s.VIXRegime
	}
	var squeeze *bool
	if s.SqueezeOK {
		squeeze = &s.Squeeze
	}
	vals := []any{
		r.Symbol, r.T, s.Close,
		optF(s.RSIOK, s.RSI), optF(s.MACDOK, s.MACDHist), s.MACDBullCross, s.MACDBearCross, squeeze, trend,
		s.BullFlag, s.BearFlag, s.HSFound, s.HSNecklineBreak, s.InvHSFound, s.InvHSNecklineBreak,
		s.TotalSweeps, low != nil, lowLevel, high != nil, highLevel,
		lastKind, lastClose, lastLevel,
		s.LastBullishOB, s.LastBearishOB, optF(s.VIXOK, s.VIX), vixRegime, ruleAction, ruleConf,
		r.GapSessions,
		r.Uptrend(), optF(s.ATROK, s.ATR), r.ATRPct(),
	}
	vals = append(vals, labelValues(r)...)
	return append(vals, version)
}

func comparisonValues(r heuristics.Row, version string) []any {
	vals := []any{r.Symbol, r.T, r.Snap.Close}
	for _, sig := range heuristics.Signals {
		vals = append(vals, r.Fired[sig])
	}
	vals = append(vals, r.Uptrend(), optF(r.Snap.ATROK, r.Snap.ATR), r.ATRPct())
	vals = append(vals, labelValues(r)...)
	return append(vals, version)
}

// Beginner is a pool or a transaction (whose Begin opens a savepoint).
type Beginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

// ReplaceHeuristicSymbol atomically replaces one symbol's rows in every
// migration-027 table with res. Re-running a symbol is therefore idempotent.
func ReplaceHeuristicSymbol(ctx context.Context, db Beginner, symbol, version string, res heuristics.SymbolResult) (int, error) {
	tx, err := db.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	written := 0
	for _, sig := range heuristics.Signals {
		table := sig.EpisodeTable()
		if _, err := tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s WHERE symbol = $1`, pgx.Identifier{table}.Sanitize()), symbol); err != nil {
			return 0, fmt.Errorf("delete %s: %w", table, err)
		}
		rows := res.Episodes[sig]
		if len(rows) == 0 {
			continue
		}
		n, err := tx.CopyFrom(ctx, pgx.Identifier{table}, episodeColumns, pgx.CopyFromSlice(len(rows), func(i int) ([]any, error) {
			return episodeValues(rows[i], version), nil
		}))
		if err != nil {
			return 0, fmt.Errorf("copy %s: %w", table, err)
		}
		written += int(n)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM heuristic_comparison_days WHERE symbol = $1`, symbol); err != nil {
		return 0, fmt.Errorf("delete comparison: %w", err)
	}
	if len(res.Comparison) > 0 {
		n, err := tx.CopyFrom(ctx, pgx.Identifier{"heuristic_comparison_days"}, comparisonColumns(),
			pgx.CopyFromSlice(len(res.Comparison), func(i int) ([]any, error) {
				return comparisonValues(res.Comparison[i], version), nil
			}))
		if err != nil {
			return 0, fmt.Errorf("copy comparison: %w", err)
		}
		written += int(n)
	}
	return written, tx.Commit(ctx)
}

// HeuristicRun is one manifest row.
type HeuristicRun struct {
	HarnessVersion string
	UniverseRule   string
	SymbolCount    int
	StartedAt      time.Time
	GitCommit      string
	GitDirty       bool
	Params         any
	Notes          string
}

func StartHeuristicRun(ctx context.Context, q Querier, r HeuristicRun) (int64, error) {
	pj, err := json.Marshal(r.Params)
	if err != nil {
		return 0, err
	}
	var id int64
	err = q.QueryRow(ctx, `
		INSERT INTO heuristic_replay_runs
		    (harness_version, universe_rule, symbol_count, status, started_at, git_commit, git_dirty, params, notes)
		VALUES ($1, $2, $3, 'running', $4, $5, $6, $7, $8)
		RETURNING run_id`,
		r.HarnessVersion, r.UniverseRule, r.SymbolCount, r.StartedAt, r.GitCommit, r.GitDirty, pj, r.Notes).Scan(&id)
	return id, err
}

func UpdateHeuristicRun(ctx context.Context, db Beginner, runID int64, symbolsDone int, status string, finished bool) error {
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var fin *time.Time
	if finished {
		now := time.Now().UTC()
		fin = &now
	}
	if _, err := tx.Exec(ctx, `
		UPDATE heuristic_replay_runs
		SET symbols_done = $2, status = $3, finished_at = COALESCE($4::timestamptz, finished_at)
		WHERE run_id = $1`, runID, symbolsDone, status, fin); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
