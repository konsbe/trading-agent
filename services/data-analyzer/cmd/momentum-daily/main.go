// Command momentum-daily runs the momentum chain unattended, once per NYSE
// session: wait for the session's daily bars to land, then momentum-scanner,
// then momentum-tracker.
//
// Why a coverage check instead of a fixed time: bar ingestion (data-universe's
// daily refresh) is not aligned to the close, and scanning a half-ingested day
// silently produces a scan with the wrong denominator and missing candidates —
// the same class of race as the corporate-action bug. So the chain starts only
// when the session's bars cover MOMENTUM_DAILY_MIN_COVERAGE of the scannable
// universe, and gives up loudly (never runs on partial data) if they have not
// landed by MOMENTUM_DAILY_GIVE_UP_AFTER.
//
// Both steps are idempotent (the scanner upserts; the tracker skips bars it has
// already evaluated), so a restart that re-runs a session is safe.
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

	"github.com/konsbe/trading-agent/services/data-analyzer/internal/db"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/logx"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/momentumapi"
)

type config struct {
	grace       time.Duration // after the 16:00 NY close before a session is due
	poll        time.Duration
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
		poll:        durationEnv("MOMENTUM_DAILY_POLL", 15*time.Minute),
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
		"min_coverage", cfg.minCoverage, "give_up_after", cfg.giveUpAfter.String(), "bin_dir", cfg.binDir)

	st := &state{attempts: map[string]int{}}
	for {
		done := tick(ctx, log, pool, cfg, st, time.Now())
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

// state is in memory on purpose: after a restart the chain re-runs the latest
// session, which is safe because both steps are idempotent.
type state struct {
	completed string // latest session the chain finished (or gave up on)
	attempts  map[string]int
}

type decision int

const (
	decideWait decision = iota // bars not landed yet, keep polling
	decideRun
	decideGiveUp
	decideDone
)

// decide is the pure scheduling rule: given the due session, whether it was
// already handled, the current bar coverage and the time, what to do.
func decide(session time.Time, completed string, coverage, minCoverage float64,
	attempts, maxAttempts int, now time.Time, giveUpAfter time.Duration) decision {
	key := session.Format(time.DateOnly)
	if completed == key {
		return decideDone
	}
	if coverage >= minCoverage {
		if attempts >= maxAttempts {
			return decideGiveUp
		}
		return decideRun
	}
	if !now.Before(sessionClose(session).Add(giveUpAfter)) {
		return decideGiveUp
	}
	return decideWait
}

func sessionClose(session time.Time) time.Time {
	ny, _ := time.LoadLocation("America/New_York")
	return time.Date(session.Year(), session.Month(), session.Day(), 16, 0, 0, 0, ny)
}

// tick handles the currently due session and reports whether it is finished
// (run or given up) — used by -once.
func tick(ctx context.Context, log *slog.Logger, pool *pgxpool.Pool, cfg config, st *state, now time.Time) bool {
	session, err := momentumapi.ExpectedSession(now, cfg.grace)
	if err != nil {
		log.Error("momentum-daily: session calendar", "err", err)
		return false
	}
	key := session.Format(time.DateOnly)
	if st.completed == key {
		return true
	}

	coverage, landed, eligible, err := barCoverage(ctx, pool, session, cfg.source)
	if err != nil {
		log.Error("momentum-daily: bar coverage", "session", key, "err", err)
		return false
	}

	switch decide(session, st.completed, coverage, cfg.minCoverage, st.attempts[key], cfg.maxAttempts, now, cfg.giveUpAfter) {
	case decideDone:
		return true
	case decideWait:
		log.Info("momentum-daily: waiting for bars", "session", key, "coverage", round(coverage),
			"landed", landed, "eligible", eligible, "need", cfg.minCoverage)
		return false
	case decideGiveUp:
		log.Error("momentum-daily: giving up on session — chain NOT run", "session", key,
			"coverage", round(coverage), "landed", landed, "eligible", eligible,
			"attempts", st.attempts[key], "reason", giveUpReason(coverage, cfg))
		st.completed = key
		return true
	}

	st.attempts[key]++
	log.Info("momentum-daily: bars landed, running chain", "session", key,
		"coverage", round(coverage), "attempt", st.attempts[key])
	for _, step := range []string{"momentum-scanner", "momentum-tracker"} {
		if err := runStep(ctx, log, filepath.Join(cfg.binDir, step)); err != nil {
			log.Error("momentum-daily: step failed; retrying next poll", "session", key, "step", step, "err", err)
			return false
		}
	}
	log.Info("momentum-daily: chain complete", "session", key)
	st.completed = key
	return true
}

func giveUpReason(coverage float64, cfg config) string {
	if coverage >= cfg.minCoverage {
		return fmt.Sprintf("chain failed %d times", cfg.maxAttempts)
	}
	return fmt.Sprintf("bars below %.0f%% coverage %s after the close", cfg.minCoverage*100, cfg.giveUpAfter)
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
func runStep(ctx context.Context, log *slog.Logger, bin string) error {
	cmd := exec.CommandContext(ctx, bin)
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
