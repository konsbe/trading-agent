// heuristics-lockbox computes, once, the forward labels for H2's lockbox rows
// (docs/HEURISTIC_SIGNALS_PREREGISTRATION.md §5g): rsi_oversold lockbox
// episodes and the lockbox comparison days on which rsi_oversold did not fire.
//
// One shot: a heuristic_lockbox_runs row is inserted before any label is
// computed, so a second run fails on its primary key. Labels come from
// heuristics.LabelsFor on store.QueryEquityBars — the replay's own code path.
// No statistic is computed here; scripts/heuristics_lockbox_h2.py does that.
//
//	heuristics-lockbox -hypothesis H2 -confirm-one-shot
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"github.com/konsbe/trading-agent/services/data-analyzer/internal/compute"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/db"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/heuristics"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/logx"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/store"
)

const fullHistory = 1_000_000

// Only H2 cleared its round-1 bar; no other hypothesis may open the lockbox.
var allowed = map[string]string{"H2": "rsi_oversold"}

type row struct {
	symbol string
	t      time.Time
	grp    int16
	atrPct *float64
}

func main() {
	hyp := flag.String("hypothesis", "", "hypothesis to evaluate (only H2 is allowed)")
	confirm := flag.Bool("confirm-one-shot", false, "required: the lockbox is spent by this run")
	flag.Parse()

	_ = godotenv.Load()
	log := logx.New(os.Getenv("LOG_LEVEL"))
	signal, ok := allowed[*hyp]
	if !ok || !*confirm {
		log.Error("refusing: need -hypothesis H2 -confirm-one-shot", "hypothesis", *hyp)
		os.Exit(2)
	}
	ctx := context.Background()
	pool, err := db.Connect(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Error("db", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	sha, _ := exec.Command("git", "rev-parse", "HEAD").Output()
	if _, err := pool.Exec(ctx, `INSERT INTO heuristic_lockbox_runs (hypothesis, git_commit) VALUES ($1, $2)`,
		*hyp, strings.TrimSpace(string(sha))); err != nil {
		log.Error("one-shot guard: this hypothesis has already opened the lockbox", "hypothesis", *hyp, "err", err)
		os.Exit(1)
	}

	written, missing, err := run(ctx, pool, signal)
	status := "complete"
	if err != nil {
		status = "failed"
		log.Error("lockbox labels", "err", err)
	}
	if _, uerr := pool.Exec(ctx, `UPDATE heuristic_lockbox_runs SET finished_at = now(), status = $2,
		rows_written = $3, rows_missing = $4 WHERE hypothesis = $1`, *hyp, status, written, missing); uerr != nil {
		log.Error("guard update", "err", uerr)
	}
	log.Info("lockbox labels done", "hypothesis", *hyp, "status", status, "rows", written, "missing_bar", missing)
	if err != nil {
		os.Exit(1)
	}
}

func run(ctx context.Context, pool *pgxpool.Pool, signal string) (int, int, error) {
	q := fmt.Sprintf(`
		SELECT symbol, t, 1::smallint, atr_pct FROM heuristic_ep_%[1]s WHERE in_lockbox
		UNION ALL
		SELECT symbol, t, 0::smallint, atr_pct FROM heuristic_comparison_days
		WHERE in_lockbox AND NOT fired_%[1]s
		ORDER BY 1, 2`, signal)
	rs, err := pool.Query(ctx, q)
	if err != nil {
		return 0, 0, err
	}
	bySymbol := map[string][]row{}
	var order []string
	for rs.Next() {
		var r row
		if err := rs.Scan(&r.symbol, &r.t, &r.grp, &r.atrPct); err != nil {
			rs.Close()
			return 0, 0, err
		}
		if _, seen := bySymbol[r.symbol]; !seen {
			order = append(order, r.symbol)
		}
		bySymbol[r.symbol] = append(bySymbol[r.symbol], r)
	}
	rs.Close()
	if err := rs.Err(); err != nil {
		return 0, 0, err
	}

	written, missing := 0, 0
	for _, sym := range order {
		bars, err := store.QueryEquityBars(ctx, pool, sym, "1Day", fullHistory)
		if err != nil {
			return written, missing, fmt.Errorf("%s bars: %w", sym, err)
		}
		index := sessionIndex(bars)
		out := make([][]any, 0, len(bySymbol[sym]))
		for _, r := range bySymbol[sym] {
			i, ok := index[r.t.Format("2006-01-02")]
			if !ok {
				missing++
				continue
			}
			l := heuristics.LabelsFor(bars, i, false)
			complete := l.Complete != nil && *l.Complete
			out = append(out, []any{sym, r.t, r.grp, r.atrPct,
				l.FwdReturn[0], l.FwdReturn[1], l.FwdReturn[2], l.FwdAbsMove[1], complete})
		}
		n, err := pool.CopyFrom(ctx, pgx.Identifier{"heuristic_lockbox_h2_rows"},
			[]string{"symbol", "t", "grp", "atr_pct", "fwd_return_5s", "fwd_return_10s", "fwd_return_20s",
				"fwd_abs_move_10s", "label_complete"}, pgx.CopyFromRows(out))
		if err != nil {
			return written, missing, fmt.Errorf("%s write: %w", sym, err)
		}
		written += int(n)
	}
	if written == 0 {
		return 0, missing, errors.New("no lockbox rows labelled")
	}
	return written, missing, nil
}

// sessionIndex maps a bar's UTC session date to its index, the key the replay
// used for t.
func sessionIndex(bars []compute.Bar) map[string]int {
	m := make(map[string]int, len(bars))
	for i, b := range bars {
		m[b.TS.UTC().Format("2006-01-02")] = i
	}
	return m
}
