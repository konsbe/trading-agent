// fundamental-analysis reads raw fundamental metrics that data-ingestion/data-fundamental
// has stored in the equity_fundamentals table (source = finnhub_*) and derives
// composite scores and growth rates that the analyst-bot can consume directly.
//
// Data flow:
//   data-fundamental (data-ingestion)
//     → equity_fundamentals (TimescaleDB, source = finnhub_*)
//     → fundamental-analysis (this binary, data-analyzer)
//     → equity_fundamentals (TimescaleDB, source = "fundamental_analysis")
//
// Derived metrics stored (period = "ttm" or "derived"):
//   - "composite_score"      weighted fundamental quality score (-1.0 … +1.0)
//   - "eps_growth_yoy"       year-over-year EPS growth percentage
//   - "revenue_growth_yoy"   year-over-year revenue growth percentage
//   - "fcf_yield"            FCF / market-cap proxy (when market price available)
//   - "earnings_surprise_avg" average surprise over last N reports
//
// TODO: migrate this worker to Python (analyst-bot or a dedicated service).
// pandas + a psycopg3 connection makes the pivoting and ratio math far simpler.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"github.com/berdelis/trading-agent/services/data-analyzer/internal/config"
	"github.com/berdelis/trading-agent/services/data-analyzer/internal/db"
	"github.com/berdelis/trading-agent/services/data-analyzer/internal/logx"
	"github.com/berdelis/trading-agent/services/data-analyzer/internal/store"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	_ = godotenv.Load()
	cfg, err := config.LoadFundamentalAnalysis()
	if err != nil {
		slog.Error("config", "err", err)
		os.Exit(1)
	}
	log := logx.New(cfg.LogLevel)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("db", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	w := &worker{cfg: cfg, pool: pool, log: log}

	log.Info("running initial fundamental analysis")
	w.analyzeAll(ctx)

	ticker := time.NewTicker(cfg.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Info("shutdown")
			return
		case <-ticker.C:
			w.analyzeAll(ctx)
		}
	}
}

type worker struct {
	cfg  config.FundamentalAnalysis
	pool *pgxpool.Pool
	log  *slog.Logger
}

func (w *worker) analyzeAll(ctx context.Context) {
	for _, sym := range w.cfg.Symbols {
		rows, err := store.QueryLatestMetrics(ctx, w.pool, sym)
		if err != nil {
			w.log.Error("query metrics", "symbol", sym, "err", err)
			continue
		}
		if len(rows) < w.cfg.MinMetrics {
			w.log.Debug("not enough metrics to score", "symbol", sym, "have", len(rows))
			continue
		}
		w.score(ctx, sym, rows)
	}
}

// score derives composite metrics from the raw fundamental rows and upserts them.
//
// TODO: replace this function with Python pandas logic:
//   df = pd.DataFrame(rows).pivot(index='period', columns='metric', values='value')
//   Then apply vectorised ratio calculations.
func (w *worker) score(ctx context.Context, symbol string, rows []store.FundamentalRow) {
	// Index raw metrics by name for easy lookup.
	latest := make(map[string]float64, len(rows))
	for _, r := range rows {
		if r.Value != nil {
			// last-write wins; rows are already DISTINCT ON (metric, period) newest-first.
			if _, exists := latest[r.Metric]; !exists {
				latest[r.Metric] = *r.Value
			}
		}
	}

	ts := time.Now().UTC()
	ptr := func(v float64) *float64 { return &v }

	upsert := func(metric string, value *float64, payload any) {
		if err := store.UpsertFundamentalDerived(ctx, w.pool, ts, symbol, "derived", metric, value, payload); err != nil {
			w.log.Error("upsert derived metric", "symbol", symbol, "metric", metric, "err", err)
		}
	}

	// ── Composite quality score ───────────────────────────────────────────────
	// Simple weighted score:
	//   +1 if P/E is positive and reasonable (0 < pe < 30)
	//   +1 if FCF TTM is positive
	//   +1 if gross margin > 20%
	//   +1 if EPS TTM > 0
	//   Score is normalised to [-1, +1].
	//
	// TODO: replace with a proper multi-factor model (Piotroski F-score, Altman Z-score,
	// or a trained classifier) in the Python analyst-bot.
	var rawScore, maxScore float64

	if pe, ok := latest["peNormalizedAnnual"]; ok && pe > 0 {
		maxScore++
		if pe < 30 {
			rawScore++
		}
	}
	if fcf, ok := latest["fcfPerShareTTM"]; ok {
		maxScore++
		if fcf > 0 {
			rawScore++
		}
	}
	if gm, ok := latest["grossMarginTTM"]; ok {
		maxScore++
		if gm > 0.20 {
			rawScore++
		}
	}
	if eps, ok := latest["epsTTM"]; ok {
		maxScore++
		if eps > 0 {
			rawScore++
		}
	}

	if maxScore > 0 {
		score := (rawScore/maxScore)*2 - 1 // maps [0,1] → [-1,+1]
		upsert("composite_score", ptr(score), map[string]any{
			"raw_score":  rawScore,
			"max_score":  maxScore,
			"components": []string{"pe_normalised", "fcf_per_share_ttm", "gross_margin_ttm", "eps_ttm"},
			"method":     "simple_threshold_v1",
		})
	}

	// ── EPS growth YoY ───────────────────────────────────────────────────────
	// Requires both "epsTTM" (current) and "epsAnnualised" or prior-year data.
	// TODO: query two consecutive annual periods from equity_fundamentals and
	// compute growth properly once multi-period rows are available.
	if eps, ok := latest["epsTTM"]; ok {
		if epsAnn, ok2 := latest["epsExclExtraItemsAnnual"]; ok2 && epsAnn != 0 {
			growth := (eps - epsAnn) / absFloat(epsAnn) * 100
			upsert("eps_growth_yoy", ptr(growth), map[string]any{
				"eps_ttm":    eps,
				"eps_annual": epsAnn,
				"note":       "YoY: TTM vs last annual; may overstate growth near year-end",
			})
		}
	}

	// ── Revenue growth YoY ──────────────────────────────────────────────────
	if rev, ok := latest["revenueTTM"]; ok {
		if revAnn, ok2 := latest["revenueAnnual"]; ok2 && revAnn != 0 {
			growth := (rev - revAnn) / absFloat(revAnn) * 100
			upsert("revenue_growth_yoy", ptr(growth), map[string]any{
				"revenue_ttm":    rev,
				"revenue_annual": revAnn,
			})
		}
	}

	// ── Average earnings surprise ─────────────────────────────────────────────
	// Aggregate from individual earnings rows stored by data-fundamental.
	// TODO: query the earnings series directly from the DB for a proper rolling average.
	if surprise, ok := latest["epsSurprisePct"]; ok {
		upsert("earnings_surprise_avg", ptr(surprise), map[string]any{
			"note": "Point-in-time last surprise; rolling average requires series query — TODO",
		})
	}

	w.log.Info("fundamental scores computed", "symbol", symbol, "metrics_used", len(latest))
}

func absFloat(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}
