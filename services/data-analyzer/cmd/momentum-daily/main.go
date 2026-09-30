// Command momentum-daily runs the momentum chain unattended, once per NYSE
// session: wait for the session's daily bars to land, then momentum-scanner,
// then momentum-tracker.
//
// Why a coverage check instead of a fixed time: bar ingestion (data-universe's
// daily refresh) is not aligned to the close, and scanning a half-ingested day
// silently produces a scan with the wrong denominator and missing candidates —
// the same class of race as the corporate-action bug. So the chain starts only
// when the session's bars cover MOMENTUM_DAILY_MIN_COVERAGE of the scannable
// universe, and defers loudly (never runs on partial data) if they have not
// landed by MOMENTUM_DAILY_GIVE_UP_AFTER.
//
// Scheduling is by wall clock, checked every MOMENTUM_DAILY_POLL (1 minute):
// each check compares the real time with the session's close, so a machine
// that slept through the close acts within a minute of waking. A timer or a
// long sleep would not: Go measures durations on the monotonic clock, which
// stops while the host sleeps, and that is how 2026-09-28's bars landed
// sixteen hours late.
//
// CATCH-UP. A session whose chain did not complete in its window (deferred
// because its bars had not landed, or never attempted because the daemon or
// the machine was down) is run later, once its bars cover the threshold: on
// start and on every check, the last MOMENTUM_DAILY_CATCH_UP_SESSIONS NYSE
// sessions, oldest first, scanner then tracker, each with -session so it reads
// only data up to that session. Such a run is marked catch_up (migration 032):
// it never counts as a clean unattended session, and the bot posts no alerts
// for it. Only exhausted attempts are final.
//
// Progress is durable, never in memory: each step records its own completion
// in momentum_chain_runs (migration 025) — the scanner inside the same
// transaction as its scan, the tracker after its last write — and this daemon
// persists attempts and give-ups there too. So a restart, or a kill part-way
// through either step, can never make a session look finished when it is not,
// nor lose the retry count: an unmarked step is simply run again, which is
// safe because the scan is all-or-nothing and every tracker write is
// idempotent.
//
//	DATABASE_URL=... go run ./cmd/momentum-daily          # daemon
//	DATABASE_URL=... go run ./cmd/momentum-daily -once    # latest session, then exit
package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
	_ "time/tzdata"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	appconfig "github.com/konsbe/trading-agent/services/data-analyzer/internal/config"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/db"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/logx"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/momentumapi"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/store"
)

type config struct {
	grace       time.Duration // after the 16:00 NY close before a session is due
	poll        time.Duration
	waitLog     time.Duration // how often an unchanged "waiting for bars" is logged
	catchUpN    int           // NYSE sessions, ending at the due one, the catch-up covers
	minCoverage float64
	giveUpAfter time.Duration // after the close
	maxAttempts int
	binDir      string
	source      string
}

func main() {
	once := flag.Bool("once", false, "handle the latest due session and exit")
	flag.Parse()

	_ = godotenv.Load()
	_ = godotenv.Load("../../.env")
	log := logx.New(env("LOG_LEVEL", "info"))

	dsn := env("DATABASE_URL", "")
	if dsn == "" {
		log.Error("momentum-daily: DATABASE_URL is required")
		os.Exit(1)
	}
	exe, _ := os.Executable()
	cfg := config{
		grace:       durationEnv("MOMENTUM_DAILY_GRACE", 2*time.Hour),
		poll:        durationEnv("MOMENTUM_DAILY_POLL", time.Minute),
		waitLog:     durationEnv("MOMENTUM_DAILY_WAIT_LOG_EVERY", 15*time.Minute),
		catchUpN:    intEnv("MOMENTUM_DAILY_CATCH_UP_SESSIONS", 5),
		minCoverage: floatEnv("MOMENTUM_DAILY_MIN_COVERAGE", 0.95),
		giveUpAfter: durationEnv("MOMENTUM_DAILY_GIVE_UP_AFTER", 14*time.Hour),
		maxAttempts: intEnv("MOMENTUM_DAILY_MAX_ATTEMPTS", 3),
		binDir:      env("MOMENTUM_DAILY_BIN_DIR", filepath.Dir(exe)),
		source:      env("MOMENTUM_DAILY_BAR_SOURCE", "tiingo"),
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	pool, err := db.Connect(ctx, dsn)
	if err != nil {
		log.Error("momentum-daily: database", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	if _, err := momentumapi.IsTradingDay(time.Now().AddDate(0, 0, 60)); err != nil {
		log.Warn("momentum-daily: session calendar runs out within 60 days", "err", err)
	}
	log.Info("momentum-daily: started", "grace", cfg.grace.String(), "poll", cfg.poll.String(),
		"min_coverage", cfg.minCoverage, "give_up_after", cfg.giveUpAfter.String(),
		"catch_up_sessions", cfg.catchUpN, "bin_dir", cfg.binDir)

	ta, err := appconfig.LoadTechnicalAnalysis()
	if err != nil {
		log.Error("momentum-daily: technical config", "err", err)
		os.Exit(1)
	}
	fa, err := appconfig.LoadFundamentalAnalysis()
	if err != nil {
		log.Error("momentum-daily: fundamental config", "err", err)
		os.Exit(1)
	}
	comp := &computer{pool: pool, ta: ta, fa: fa, log: log}
	if !*once {
		go comp.manualLoop(ctx, durationEnv("MOMENTUM_DAILY_MANUAL_POLL", time.Minute))
	}

	st := &logState{waits: map[string]waitLogged{}}
	for {
		now := time.Now()
		done := false
		// Oldest first: a session still being caught up holds the in-window
		// run of the newer one, so the tracker never steps past a session
		// before that session's candidates are opened.
		if !catchUp(ctx, log, pool, cfg, st, now) {
			done = tick(ctx, log, pool, cfg, st, now)
		}
		comp.dailyPass(ctx)
		if *once && done {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(cfg.poll):
		}
	}
}

// logState only de-duplicates log lines across polls. It holds no progress:
// everything that decides what to run is read from momentum_chain_runs.
type logState struct {
	doneLogged  string
	finalLogged map[string]bool
	waits       map[string]waitLogged
}

type waitLogged struct {
	landed int
	at     time.Time
}

// logWait reports whether a "waiting for bars" line is due: coverage moved,
// or waitLog passed since the last one.
func (st *logState) logWait(key string, landed int, now time.Time, every time.Duration) bool {
	w, seen := st.waits[key]
	if seen && w.landed == landed && now.Sub(w.at) < every {
		return false
	}
	st.waits[key] = waitLogged{landed, now}
	return true
}

type decision int

const (
	decideWait decision = iota // bars not landed yet, keep polling
	decideRun
	decideDefer  // window over without the bars: stop polling, leave it to catch-up
	decideGiveUp // attempts exhausted: final
	decideDone
)

// runState is what the scheduling rule needs from the session's persisted row.
type runState struct {
	trackerDone bool // tracker_completed_at set: the whole chain finished
	gaveUp      bool
	attempts    int
}

func runStateOf(r store.ChainRun, found bool) runState {
	if !found {
		return runState{}
	}
	return runState{trackerDone: r.TrackerCompletedAt != nil, gaveUp: r.GaveUpAt != nil, attempts: r.Attempts}
}

// decide is the pure scheduling rule: given the session's persisted state, the
// current bar coverage and the time, what to do.
func decide(session time.Time, rs runState, coverage, minCoverage float64,
	maxAttempts int, now time.Time, giveUpAfter time.Duration) decision {
	if rs.trackerDone || rs.gaveUp {
		return decideDone
	}
	if coverage >= minCoverage {
		if rs.attempts >= maxAttempts {
			return decideGiveUp
		}
		return decideRun
	}
	if !now.Before(sessionClose(session).Add(giveUpAfter)) {
		return decideDefer
	}
	return decideWait
}

// needsCatchUp: a session the in-window path no longer runs — any session
// before the due one, or the due one once deferred — whose chain is unfinished.
func needsCatchUp(session, due time.Time, rs runState) bool {
	return !rs.trackerDone && (session.Before(due) || rs.gaveUp)
}

// decideCatchUp is the catch-up rule: run once the bars cover the threshold,
// for as long as attempts remain. No time limit: the window is over already.
func decideCatchUp(rs runState, coverage, minCoverage float64, maxAttempts int) decision {
	switch {
	case rs.trackerDone:
		return decideDone
	case rs.attempts >= maxAttempts:
		return decideGiveUp
	case coverage >= minCoverage:
		return decideRun
	}
	return decideWait
}

func sessionClose(session time.Time) time.Time {
	ny, _ := time.LoadLocation("America/New_York")
	return time.Date(session.Year(), session.Month(), session.Day(), 16, 0, 0, 0, ny)
}

// tick handles the currently due session and reports whether it is finished
// (run or given up) — used by -once.
func tick(ctx context.Context, log *slog.Logger, pool *pgxpool.Pool, cfg config, st *logState, now time.Time) bool {
	session, err := momentumapi.ExpectedSession(now, cfg.grace)
	if err != nil {
		log.Error("momentum-daily: session calendar", "err", err)
		return false
	}
	key := session.Format(time.DateOnly)

	run, found, err := store.LoadChainRun(ctx, pool, session)
	if err != nil {
		log.Error("momentum-daily: chain state", "session", key, "err", err)
		return false
	}
	rs := runStateOf(run, found)
	if rs.trackerDone || rs.gaveUp {
		if st.doneLogged != key {
			log.Info("momentum-daily: session already handled in its window", "session", key,
				"chain_complete", rs.trackerDone, "deferred_or_given_up", rs.gaveUp, "attempts", rs.attempts)
			st.doneLogged = key
		}
		return true
	}

	coverage, landed, eligible, err := barCoverage(ctx, pool, session, cfg.source)
	if err != nil {
		log.Error("momentum-daily: bar coverage", "session", key, "err", err)
		return false
	}

	switch decide(session, rs, coverage, cfg.minCoverage, cfg.maxAttempts, now, cfg.giveUpAfter) {
	case decideDone:
		return true
	case decideWait:
		if st.logWait(key, landed, now, cfg.waitLog) {
			log.Info("momentum-daily: waiting for bars", "session", key, "coverage", round(coverage),
				"landed", landed, "eligible", eligible, "need", cfg.minCoverage)
		}
		return false
	case decideDefer, decideGiveUp:
		reason := giveUpReason(coverage, cfg)
		msg := "momentum-daily: giving up on session — chain NOT run; attempts exhausted, final"
		if coverage < cfg.minCoverage {
			msg = "momentum-daily: deferring session — chain NOT run in its window; catch-up runs it once its bars land"
		}
		log.Error(msg, "session", key,
			"coverage", round(coverage), "landed", landed, "eligible", eligible,
			"attempts", rs.attempts, "reason", reason)
		if err := store.MarkChainGaveUp(ctx, pool, session, reason); err != nil {
			log.Error("momentum-daily: could not record give-up; will retry", "session", key, "err", err)
			return false
		}
		return true
	}

	// Counted BEFORE running, so a crash or kill mid-run still uses up an
	// attempt and a step that dies every time cannot be retried forever.
	attempt, err := store.BeginChainAttempt(ctx, pool, session)
	if err != nil {
		log.Error("momentum-daily: could not record attempt; not running", "session", key, "err", err)
		return false
	}
	log.Info("momentum-daily: bars landed, running chain", "session", key,
		"coverage", round(coverage), "attempt", attempt, "of", cfg.maxAttempts)

	if !runChain(ctx, log, pool, cfg, session, run) {
		return false
	}
	log.Info("momentum-daily: chain complete", "session", key, "attempt", attempt)
	st.doneLogged = key
	return true
}

// runChain runs the scanner (unless its scan already committed) and then the
// tracker for session, verifying each step's marker. args go to both steps
// (-session for a catch-up). It reports whether both completed.
func runChain(ctx context.Context, log *slog.Logger, pool *pgxpool.Pool, cfg config,
	session time.Time, run store.ChainRun, args ...string) bool {
	key := session.Format(time.DateOnly)
	fail := func(step string, err error) bool {
		log.Error("momentum-daily: step failed; retrying next poll", "session", key, "step", step, "err", err)
		if rerr := store.RecordChainError(ctx, pool, session, step+": "+err.Error()); rerr != nil {
			log.Error("momentum-daily: could not record step error", "session", key, "err", rerr)
		}
		return false
	}

	// A scan that already committed is not redone: re-scanning after the bot
	// may have posted it could change the candidate set under those alerts.
	if run.ScannerCompletedAt == nil {
		if err := runStep(ctx, log, filepath.Join(cfg.binDir, "momentum-scanner"), args...); err != nil {
			return fail("momentum-scanner", err)
		}
		if r, ok, err := store.LoadChainRun(ctx, pool, session); err != nil || !ok || r.ScannerCompletedAt == nil {
			return fail("momentum-scanner", fmt.Errorf("exited 0 but did not mark session %s scanned "+
				"(it scanned a different session, or the marker write failed): %v", key, err))
		}
	} else {
		log.Info("momentum-daily: scan already committed for session; not re-scanning", "session", key)
	}

	if err := runStep(ctx, log, filepath.Join(cfg.binDir, "momentum-tracker"), args...); err != nil {
		return fail("momentum-tracker", err)
	}
	if r, ok, err := store.LoadChainRun(ctx, pool, session); err != nil || !ok || r.TrackerCompletedAt == nil {
		return fail("momentum-tracker", fmt.Errorf("exited 0 but did not mark session %s tracked: %v", key, err))
	}
	return true
}

// catchUp runs, oldest first, every recent session that needsCatchUp and whose
// bars now cover the threshold. It reports whether it is holding the newer
// sessions: a session it ran and could not finish this check.
func catchUp(ctx context.Context, log *slog.Logger, pool *pgxpool.Pool, cfg config, st *logState, now time.Time) bool {
	due, err := momentumapi.ExpectedSession(now, cfg.grace)
	if err != nil {
		log.Error("momentum-daily: session calendar", "err", err)
		return false
	}
	sessions, err := recentSessions(due, cfg.catchUpN)
	if err != nil {
		log.Error("momentum-daily: session calendar", "err", err)
		return false
	}
	for _, session := range sessions {
		key := session.Format(time.DateOnly)
		run, found, err := store.LoadChainRun(ctx, pool, session)
		if err != nil {
			log.Error("momentum-daily: chain state", "session", key, "err", err)
			return true
		}
		rs := runStateOf(run, found)
		if !needsCatchUp(session, due, rs) {
			continue
		}
		coverage, landed, eligible, err := barCoverage(ctx, pool, session, cfg.source)
		if err != nil {
			log.Error("momentum-daily: bar coverage", "session", key, "err", err)
			return true
		}
		switch decideCatchUp(rs, coverage, cfg.minCoverage, cfg.maxAttempts) {
		case decideGiveUp:
			if !st.finalLogged[key] {
				log.Error("momentum-daily: not catching up session — attempts exhausted, final",
					"session", key, "attempts", rs.attempts, "last_error", run.LastError)
				if st.finalLogged == nil {
					st.finalLogged = map[string]bool{}
				}
				st.finalLogged[key] = true
			}
			continue
		case decideWait:
			if st.logWait("catch-up "+key, landed, now, cfg.waitLog) {
				log.Info("momentum-daily: catch-up waiting for bars", "session", key, "coverage", round(coverage),
					"landed", landed, "eligible", eligible, "need", cfg.minCoverage)
			}
			continue
		case decideDone:
			continue
		}

		attempt, err := store.BeginCatchUpAttempt(ctx, pool, session)
		if err != nil {
			log.Error("momentum-daily: could not record catch-up attempt; not running", "session", key, "err", err)
			return true
		}
		log.Info("momentum-daily: catching up session after its window", "session", key,
			"coverage", round(coverage), "attempt", attempt, "of", cfg.maxAttempts)
		if !runChain(ctx, log, pool, cfg, session, run, "-session", key) {
			return true
		}
		log.Info("momentum-daily: catch-up complete", "session", key, "attempt", attempt)
	}
	return false
}

// recentSessions is the n NYSE sessions ending at due, oldest first.
func recentSessions(due time.Time, n int) ([]time.Time, error) {
	out := []time.Time{due}
	for day := due.AddDate(0, 0, -1); len(out) < n; day = day.AddDate(0, 0, -1) {
		ok, err := momentumapi.IsTradingDay(day)
		if err != nil {
			return nil, err
		}
		if ok {
			out = append(out, day)
		}
		if due.Sub(day) > 30*24*time.Hour {
			return nil, fmt.Errorf("fewer than %d NYSE sessions in the 30 days before %s", n, due.Format(time.DateOnly))
		}
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, nil
}

func giveUpReason(coverage float64, cfg config) string {
	if coverage >= cfg.minCoverage {
		return fmt.Sprintf("chain failed %d times", cfg.maxAttempts)
	}
	return fmt.Sprintf("bars below %.0f%% coverage %s after the close — deferred; catch-up runs it once they land",
		cfg.minCoverage*100, cfg.giveUpAfter)
}

// barCoverage is the share of the scannable universe (eligible, provider still
// serving it — the scanner's own input set) with a daily bar for the session.
func barCoverage(ctx context.Context, pool *pgxpool.Pool, session time.Time, source string) (float64, int, int, error) {
	var landed, eligible int
	err := pool.QueryRow(ctx, `
SELECT
  (SELECT count(DISTINCT o.symbol)
     FROM equity_ohlcv o
     JOIN universe_symbols u ON u.symbol = o.symbol AND u.is_eligible AND u.data_unavailable_reason IS NULL
    WHERE o.interval = '1Day' AND o.source = $2 AND o.ts = $1),
  (SELECT count(*) FROM universe_symbols WHERE is_eligible AND data_unavailable_reason IS NULL)`,
		session, source).Scan(&landed, &eligible)
	if err != nil {
		return 0, 0, 0, err
	}
	if eligible == 0 {
		return 0, 0, 0, errors.New("no scannable symbols in universe_symbols")
	}
	return float64(landed) / float64(eligible), landed, eligible, nil
}

// runStep executes a sibling binary, streaming its output into the log.
func runStep(ctx context.Context, log *slog.Logger, bin string, args ...string) error {
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Env = os.Environ()
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	cmd.Stderr = cmd.Stdout
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start %s: %w", bin, err)
	}
	stream(log, filepath.Base(bin), stdout)
	return cmd.Wait()
}

func stream(log *slog.Logger, step string, r io.Reader) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		if line := strings.TrimSpace(sc.Text()); line != "" {
			log.Info("momentum-daily: "+step, "out", line)
		}
	}
}

func round(v float64) float64 { return float64(int(v*10000)) / 10000 }

func env(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func durationEnv(key string, def time.Duration) time.Duration {
	if d, err := time.ParseDuration(env(key, "")); err == nil && d > 0 {
		return d
	}
	return def
}

func intEnv(key string, def int) int {
	if v, err := strconv.Atoi(env(key, "")); err == nil && v > 0 {
		return v
	}
	return def
}

func floatEnv(key string, def float64) float64 {
	if v, err := strconv.ParseFloat(env(key, ""), 64); err == nil && v > 0 && v <= 1 {
		return v
	}
	return def
}
