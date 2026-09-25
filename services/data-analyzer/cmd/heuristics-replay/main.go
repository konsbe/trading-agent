// heuristics-replay walks every universe symbol's stored daily history through
// heuristics.ComputeAt — the live technical-analysis indicators plus the
// analyst-bot liquidity-sweep rule, evaluated at each historical close — and
// writes per-signal episode tables and the comparison-day sample (migration
// 027). docs/HEURISTIC_SIGNALS_PREREGISTRATION.md §5 steps 3-4.
//
// It computes labels but no statistics: nothing here evaluates a hypothesis.
//
// Bars: store.QueryEquityBars, the live worker's own loader (one bar per UTC
// session, preferred source by bar_source_rank: tiingo over yahoo_finance),
// with the full history instead of the worker's 500-bar limit. ComputeAt then
// re-applies the 500-bar window per day.
//
//	heuristics-replay                       # full universe
//	heuristics-replay -symbols AAPL,MSFT -dry-run
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"github.com/konsbe/trading-agent/services/data-analyzer/internal/db"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/heuristics"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/logx"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/store"
)

const (
	lockboxRegion  = "phase2_lockbox_v2"
	vixSeriesID    = "VIXCLS"
	universeCutoff = "2016-10-01" // first bar strictly before this date
	fullHistory    = 1_000_000    // bars; far above any symbol's history
)

func main() {
	symbolsFlag := flag.String("symbols", "", "comma-separated symbols (default: the full universe)")
	limit := flag.Int("limit", 0, "process only the first N universe symbols (0 = all)")
	workers := flag.Int("workers", 8, "parallel symbols")
	dryRun := flag.Bool("dry-run", false, "compute and log counts; write nothing")
	version := flag.String("version", "heuristics-v1", "harness_version stamped on every row")
	flag.Parse()

	_ = godotenv.Load()
	log := logx.New(os.Getenv("LOG_LEVEL"))
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Error("DATABASE_URL is required")
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := db.Connect(ctx, dsn)
	if err != nil {
		log.Error("db", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	cutoff, _ := time.Parse("2006-01-02", universeCutoff)
	universe, err := store.QueryHeuristicUniverse(ctx, pool, cutoff)
	if err != nil {
		log.Error("universe", "err", err)
		os.Exit(1)
	}
	symbols := universe
	partial := false
	if s := strings.TrimSpace(*symbolsFlag); s != "" {
		symbols = nil
		for _, x := range strings.Split(s, ",") {
			if x = strings.TrimSpace(x); x != "" {
				symbols = append(symbols, x)
			}
		}
		partial = true
	}
	if *limit > 0 && *limit < len(symbols) {
		symbols = symbols[:*limit]
		partial = true
	}

	lb, err := store.QueryHeuristicLockbox(ctx, pool, lockboxRegion)
	if err != nil {
		log.Error("lockbox", "err", err)
		os.Exit(1)
	}
	vix, err := store.QueryFREDSeries(ctx, pool, vixSeriesID)
	if err != nil || len(vix.Dates) == 0 {
		log.Error("vix series", "err", err, "rows", len(vix.Dates))
		os.Exit(1)
	}

	rc := heuristics.DefaultReplayConfig()
	params := map[string]any{
		"replay":           rc,
		"signals":          heuristics.Signals,
		"label_horizons":   heuristics.LabelHorizons,
		"lockbox_region":   lockboxRegion,
		"lockbox_start":    lb.Start.Format("2006-01-02"),
		"lockbox_end":      lb.End.Format("2006-01-02"),
		"lockbox_pilot_n":  len(lb.Pilot),
		"lockbox_purge":    "row is lockbox when t or any of t+1..t+20 sessions is in the region (non-pilot symbol); all label columns NULL",
		"vix":              "latest VIXCLS observation dated <= session date; classified with the analyst-bot fixed thresholds >35/>20/<12",
		"bars":             "store.QueryEquityBars (live worker loader): one bar per UTC session, bar_source_rank preference, full history",
		"comparison":       fmt.Sprintf("session index i %% %d == %d among evaluated sessions", rc.ComparisonEvery, rc.ComparisonOffset),
		"evaluated_from":   fmt.Sprintf("session index >= %d (window >= %d bars)", rc.MinBars-1, rc.MinBars),
		"episode_gap_rule": fmt.Sprintf(">= %d evaluated sessions without a firing of the same type", rc.GapSessions),
		"universe_cutoff":  universeCutoff,
		"crypto":           "excluded this round",
		"dry_run":          *dryRun,
	}

	log.Info("heuristics replay starting",
		"version", *version, "symbols", len(symbols), "universe", len(universe),
		"workers", *workers, "dry_run", *dryRun, "pilot", len(lb.Pilot), "vix_rows", len(vix.Dates))

	var runID int64
	if !*dryRun {
		sha, dirty := gitState()
		runID, err = store.StartHeuristicRun(ctx, pool, store.HeuristicRun{
			HarnessVersion: *version,
			UniverseRule:   store.HeuristicUniverseRule,
			SymbolCount:    len(symbols),
			StartedAt:      time.Now().UTC(),
			GitCommit:      sha,
			GitDirty:       dirty,
			Params:         params,
		})
		if err != nil {
			log.Error("manifest", "err", err)
			os.Exit(1)
		}
		log.Info("manifest row", "run_id", runID, "git", sha, "dirty", dirty)
	}

	var (
		done, failed, rowsWritten, evaluated atomic.Int64
		mu                                   sync.Mutex
		episodes                             = map[heuristics.Signal]int{}
		comparison                           int
	)
	start := time.Now()
	jobs := make(chan string)
	var wg sync.WaitGroup
	for w := 0; w < *workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for sym := range jobs {
				n, res, err := replayOne(ctx, pool, sym, vix, lb, rc, *version, *dryRun)
				if err != nil {
					failed.Add(1)
					log.Error("symbol failed", "symbol", sym, "err", err)
				} else {
					rowsWritten.Add(int64(n))
					evaluated.Add(int64(res.Evaluated))
					mu.Lock()
					for sig, rows := range res.Episodes {
						episodes[sig] += len(rows)
					}
					comparison += len(res.Comparison)
					mu.Unlock()
				}
				done.Add(1)
			}
		}()
	}

	progressDone := make(chan struct{})
	go func() {
		t := time.NewTicker(30 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-progressDone:
				return
			case <-t.C:
				d := done.Load()
				el := time.Since(start)
				eta := time.Duration(0)
				if d > 0 {
					eta = time.Duration(float64(el) / float64(d) * float64(int64(len(symbols))-d))
				}
				log.Info("progress", "done", d, "of", len(symbols), "failed", failed.Load(),
					"rows", rowsWritten.Load(), "elapsed", el.Round(time.Second).String(), "eta", eta.Round(time.Second).String())
				if runID != 0 {
					if err := store.UpdateHeuristicRun(ctx, pool, runID, int(d), "running", false); err != nil {
						log.Warn("manifest progress", "err", err)
					}
				}
			}
		}
	}()

feed:
	for _, s := range symbols {
		select {
		case <-ctx.Done():
			break feed
		case jobs <- s:
		}
	}
	close(jobs)
	wg.Wait()
	close(progressDone)

	status := "complete"
	switch {
	case ctx.Err() != nil || failed.Load() > 0:
		status = "failed"
	case partial:
		status = "partial"
	}
	if runID != 0 {
		if err := store.UpdateHeuristicRun(context.Background(), pool, runID, int(done.Load()), status, true); err != nil {
			log.Error("manifest finish", "err", err)
		}
	}

	for _, sig := range heuristics.Signals {
		log.Info("episodes", "signal", string(sig), "count", episodes[sig])
	}
	log.Info("heuristics replay finished", "status", status, "symbols", done.Load(), "failed", failed.Load(),
		"evaluated_sessions", evaluated.Load(), "comparison_rows", comparison, "rows_written", rowsWritten.Load(),
		"elapsed", time.Since(start).Round(time.Second).String())
	if status == "failed" {
		os.Exit(1)
	}
}

func replayOne(ctx context.Context, pool *pgxpool.Pool, sym string, vix heuristics.VIXSeries, lb heuristics.Lockbox,
	rc heuristics.ReplayConfig, version string, dryRun bool) (int, heuristics.SymbolResult, error) {
	bars, err := store.QueryEquityBars(ctx, pool, sym, "1Day", fullHistory)
	if err != nil {
		return 0, heuristics.SymbolResult{}, fmt.Errorf("bars: %w", err)
	}
	res := heuristics.ReplaySymbol(sym, bars, vix, lb, rc)
	if dryRun {
		return 0, res, nil
	}
	n, err := store.ReplaceHeuristicSymbol(ctx, pool, sym, version, res)
	return n, res, err
}

func gitState() (sha string, dirty bool) {
	out, err := exec.Command("git", "rev-parse", "HEAD").Output()
	if err != nil {
		return "", false
	}
	st, _ := exec.Command("git", "status", "--porcelain", "--untracked-files=no").Output()
	return strings.TrimSpace(string(out)), len(strings.TrimSpace(string(st))) > 0
}
