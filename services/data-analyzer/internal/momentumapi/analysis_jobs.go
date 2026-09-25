package momentumapi

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// AnalysisParts says which of a symbol's stored analyses are out of date.
type AnalysisParts struct {
	Technical    bool
	Fundamentals bool
}

// AnalysisComputer runs the technical-analysis / fundamental-analysis workers'
// own per-symbol code (runner.ComputeAndStore, fundamental.AnalyzeSymbol) for
// one symbol, which writes the rows the analysis endpoint then reads. Symbols
// with too little data to analyse are a successful no-op, not an error.
type AnalysisComputer func(ctx context.Context, symbol string, parts AnalysisParts) error

type jobState int

const (
	jobRunning jobState = iota
	jobDone
	jobFailed
)

// freshnessKey is the input a computation ran against: the symbol's latest
// bar and newest raw fundamental row. A finished job only vouches for the
// stored rows while these are unchanged.
type freshnessKey struct {
	bar, raw time.Time
}

type analysisJob struct {
	state    jobState
	key      freshnessKey
	err      error
	finished time.Time
}

// analysisJobs runs on-demand computations: at most one per symbol at a time
// (concurrent requests share it), at most `limit` at once overall, each
// bounded by timeout. The stored rows are the cache; this only remembers how
// the last run for each symbol ended.
type analysisJobs struct {
	run     AnalysisComputer
	timeout time.Duration
	now     func() time.Time
	log     *slog.Logger
	sem     chan struct{}

	mu   sync.Mutex
	jobs map[string]*analysisJob
	// wg tracks running goroutines so tests can wait for them.
	wg sync.WaitGroup
}

func newAnalysisJobs(run AnalysisComputer, limit int, timeout time.Duration, now func() time.Time, log *slog.Logger) *analysisJobs {
	return &analysisJobs{run: run, timeout: timeout, now: now, log: log,
		sem: make(chan struct{}, limit), jobs: map[string]*analysisJob{}}
}

// get returns a copy of the symbol's last job, if any.
func (j *analysisJobs) get(symbol string) (analysisJob, bool) {
	j.mu.Lock()
	defer j.mu.Unlock()
	job, ok := j.jobs[symbol]
	if !ok {
		return analysisJob{}, false
	}
	return *job, true
}

// start begins a computation unless one is already running for the symbol.
// It reports whether this call started it.
func (j *analysisJobs) start(symbol string, key freshnessKey, parts AnalysisParts) bool {
	j.mu.Lock()
	if job, ok := j.jobs[symbol]; ok && job.state == jobRunning {
		j.mu.Unlock()
		return false
	}
	job := &analysisJob{state: jobRunning, key: key}
	j.jobs[symbol] = job
	j.mu.Unlock()

	j.wg.Add(1)
	go func() {
		defer j.wg.Done()
		err := j.execute(symbol, parts)
		j.mu.Lock()
		defer j.mu.Unlock()
		job.finished = j.now()
		if err != nil {
			job.state, job.err = jobFailed, err
			j.log.Error("momentum-api: on-demand analysis failed", "symbol", symbol, "err", err)
			return
		}
		job.state = jobDone
	}()
	return true
}

func (j *analysisJobs) execute(symbol string, parts AnalysisParts) (err error) {
	// The timeout covers waiting for a slot too, so a long queue ends in a
	// clear failure rather than an endless "computing".
	ctx, cancel := context.WithTimeout(context.Background(), j.timeout)
	defer cancel()
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()
	select {
	case j.sem <- struct{}{}:
		defer func() { <-j.sem }()
	case <-ctx.Done():
		return fmt.Errorf("waiting for a computation slot: %w", ctx.Err())
	}
	start := j.now()
	err = j.run(ctx, symbol, parts)
	if err == nil && ctx.Err() != nil {
		err = ctx.Err()
	}
	if errors.Is(err, context.DeadlineExceeded) {
		err = fmt.Errorf("timed out after %s: %w", j.timeout, err)
	}
	j.log.Info("momentum-api: on-demand analysis", "symbol", symbol, "technical", parts.Technical,
		"fundamentals", parts.Fundamentals, "took", j.now().Sub(start).String(), "err", err)
	return err
}
