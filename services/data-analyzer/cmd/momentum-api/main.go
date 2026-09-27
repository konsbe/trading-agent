// Command momentum-api serves the momentum scanner's persisted output to the
// web app: docs/MOMENTUM_SCANNER_API.md.
//
// It computes nothing of its own — no gates, no scoring, no features. It
// writes in two places only: the watchlist, and the full stock analysis
// endpoint's on-demand path, which runs the technical-analysis and
// fundamental-analysis workers' own per-symbol code (runner.ComputeAndStore,
// fundamental.AnalyzeSymbol) for a symbol outside their configured lists and
// writes exactly the rows those workers would.
//
// Unlike the other cmd/ binaries this is a long-running server, not a one-shot
// job.
//
// NO AUTHENTICATION. There is no identity provider to validate tokens against
// yet, so the server binds to localhost by default and must only be reachable
// on a trusted network. Real auth is a hard blocker before exposing it any
// further — see the README next to this file.
//
//	DATABASE_URL=... go run ./cmd/momentum-api
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
	_ "time/tzdata" // the session calendar needs America/New_York in a distroless image

	"github.com/joho/godotenv"

	"github.com/konsbe/trading-agent/services/data-analyzer/internal/config"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/db"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/fundamental"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/logx"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/momentumapi"
	symbolsets "github.com/konsbe/trading-agent/services/data-analyzer/internal/symbols"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/technical"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/technical/runner"
)

// maxCacheTTL is the ceiling from the spec: longer would let a stale cache hide
// a corrected re-run.
const maxCacheTTL = 5 * time.Minute

// maxStatusCacheTTL caps the Data Source status cache: its "last checked" must
// stay honest relative to when refresh was pressed (addendum §3).
const maxStatusCacheTTL = 60 * time.Second

func main() {
	// Service-local .env first, then the repo root's (the documented home of
	// the shared config when run from services/data-analyzer). Load never
	// overrides variables already set, so the environment still wins.
	_ = godotenv.Load()
	_ = godotenv.Load("../../.env")
	log := logx.New(env("LOG_LEVEL", "info"))

	databaseURL := env("DATABASE_URL", "")
	if databaseURL == "" {
		log.Error("momentum-api: DATABASE_URL is required")
		os.Exit(1)
	}
	addr := env("MOMENTUM_API_ADDR", "127.0.0.1:8090")
	caveatsPath := env("MOMENTUM_CAVEATS_PATH", "../../shared/content/momentum_caveats.json")
	reportPath := env("MOMENTUM_BACKTEST_REPORT_PATH", "../../shared/content/backtest_lab_report.json")
	handbookPath := env("MOMENTUM_EDUCATION_HANDBOOK_PATH", "../../shared/content/handbook.json")
	masterClassPath := env("MOMENTUM_EDUCATION_MASTERCLASS_PATH", "../../shared/content/masterclass.json")
	marketTextPath := env("MOMENTUM_MARKET_REPORT_TEXT_PATH", "../../shared/content/market_report_descriptions.json")
	corrTextPath := env("MOMENTUM_CORRELATION_TEXT_PATH", "../../shared/content/correlation_sentences.json")
	corrLabelsPath := env("MOMENTUM_CORRELATION_LABELS_PATH", "../../shared/content/correlation_labels.json")
	scanGrace := duration(log, "MOMENTUM_API_SCAN_GRACE", 6*time.Hour)
	cacheTTL := duration(log, "MOMENTUM_API_CACHE_TTL", maxCacheTTL)
	if cacheTTL > maxCacheTTL {
		log.Warn("momentum-api: cache TTL capped", "requested", cacheTTL, "cap", maxCacheTTL)
		cacheTTL = maxCacheTTL
	}
	origins := csv(env("MOMENTUM_API_CORS_ORIGINS", "http://localhost:3000"))
	statusTTL := duration(log, "MOMENTUM_API_STATUS_CACHE_TTL", 30*time.Second)
	if statusTTL > maxStatusCacheTTL {
		log.Warn("momentum-api: status cache TTL capped", "requested", statusTTL, "cap", maxStatusCacheTTL)
		statusTTL = maxStatusCacheTTL
	}
	reportTTL := duration(log, "MOMENTUM_API_REPORT_CACHE_TTL", time.Hour)
	if reportTTL > 6*time.Hour {
		log.Warn("momentum-api: market report cache TTL capped at macro-analysis's 6h cadence", "requested", reportTTL)
		reportTTL = 6 * time.Hour
	}
	// Same variable momentum-daily reads, so "pending" ends exactly when it gives up.
	giveUpAfter := duration(log, "MOMENTUM_DAILY_GIVE_UP_AFTER", 14*time.Hour)
	attentionPct := 90.0
	if v, err := strconv.ParseFloat(env("MOMENTUM_API_BUDGET_ATTENTION_PCT", "90"), 64); err == nil && v > 0 && v <= 100 {
		attentionPct = v
	} else {
		log.Warn("momentum-api: invalid MOMENTUM_API_BUDGET_ATTENTION_PCT; using 90")
	}

	// The workers' own config, read from the same shared .env, so an on-demand
	// computation writes exactly what the worker would.
	taCfg, err := config.LoadTechnicalAnalysis()
	if err != nil {
		log.Error("momentum-api: technical-analysis config", "err", err)
		os.Exit(1)
	}
	faCfg, err := config.LoadFundamentalAnalysis()
	if err != nil {
		log.Error("momentum-api: fundamental-analysis config", "err", err)
		os.Exit(1)
	}
	analysisConcurrency := 2
	if v, err := strconv.Atoi(env("MOMENTUM_API_ANALYSIS_CONCURRENCY", "2")); err == nil && v > 0 {
		analysisConcurrency = v
	} else {
		log.Warn("momentum-api: invalid MOMENTUM_API_ANALYSIS_CONCURRENCY; using 2")
	}

	caveats, err := momentumapi.LoadCaveats(caveatsPath)
	if err != nil {
		log.Error("momentum-api: caveats", "err", err)
		os.Exit(1)
	}
	report, err := momentumapi.LoadBacktestReport(reportPath)
	if err != nil {
		log.Error("momentum-api: backtest lab report", "err", err,
			"fix", "set MOMENTUM_BACKTEST_REPORT_PATH to shared/content/backtest_lab_report.json (in Docker, mount ../shared/content)")
		os.Exit(1)
	}
	education, err := momentumapi.LoadEducation(handbookPath, masterClassPath, caveats)
	if err != nil {
		log.Error("momentum-api: education content", "err", err,
			"fix", "set MOMENTUM_EDUCATION_HANDBOOK_PATH / MOMENTUM_EDUCATION_MASTERCLASS_PATH to shared/content/handbook.json / masterclass.json (in Docker, mount ../shared/content)")
		os.Exit(1)
	}

	marketText, err := momentumapi.LoadMarketReportText(marketTextPath)
	if err != nil {
		log.Error("momentum-api: market report descriptions", "err", err,
			"fix", "set MOMENTUM_MARKET_REPORT_TEXT_PATH to shared/content/market_report_descriptions.json (in Docker, mount ../shared/content)")
		os.Exit(1)
	}

	corrText, err := momentumapi.LoadCorrelationText(corrTextPath)
	if err != nil {
		log.Error("momentum-api: correlation sentences", "err", err,
			"fix", "set MOMENTUM_CORRELATION_TEXT_PATH to shared/content/correlation_sentences.json (in Docker, mount ../shared/content)")
		os.Exit(1)
	}

	corrLabels, err := momentumapi.LoadCorrelationLabels(corrLabelsPath)
	if err != nil {
		log.Error("momentum-api: correlation labels", "err", err,
			"fix", "set MOMENTUM_CORRELATION_LABELS_PATH to shared/content/correlation_labels.json (in Docker, mount ../shared/content)")
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := db.Connect(ctx, databaseURL)
	if err != nil {
		log.Error("momentum-api: database", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	if _, err := momentumapi.IsTradingDay(time.Now().AddDate(0, 0, 60)); err != nil {
		log.Warn("momentum-api: session calendar runs out within 60 days", "err", err)
	}

	srv := momentumapi.NewServer(momentumapi.Config{
		Store:              momentumapi.DBStore{Q: pool, PingFn: pool.Ping},
		Follow:             momentumapi.DBStore{Q: pool, PingFn: pool.Ping},
		ComputeDataTimeout: duration(log, "MOMENTUM_API_COMPUTE_DATA_TIMEOUT", 30*time.Minute),
		Caveats:            caveats,
		BacktestReport:     report,
		Education:          education,
		Log:                log,
		SessionReadyAfter:  scanGrace,
		CacheTTL:           cacheTTL,
		CORSOrigins:        origins,

		StatusCacheTTL:     statusTTL,
		ChainGiveUpAfter:   giveUpAfter,
		BudgetAttentionPct: attentionPct,
		SessionsShown:      7,
		BarSource:          env("MOMENTUM_DAILY_BAR_SOURCE", "tiingo"),

		MarketReportCacheTTL: reportTTL,
		EarningsCovered: func(ctx context.Context) []string {
			return symbolsets.Followed(ctx, pool, log, "momentum-api",
				"earnings calendar (MACRO_INTEL_EARNINGS_SYMBOLS)", []string{"equity"}, earningsSymbols())
		},
		GPRSourceConfigured: strings.TrimSpace(os.Getenv("GPR_CSV_URL")) != "",
		MarketReportText:    marketText,
		CorrelationText:     corrText,
		CorrelationLabels:   corrLabels,

		AnalysisNames: technical.NamesFor(technical.Emitter{Cfg: taCfg}),
		AnalysisCompute: func(ctx context.Context, symbol string, parts momentumapi.AnalysisParts) error {
			var errs []error
			if parts.Technical {
				if _, err := runner.ComputeAndStore(ctx, pool, symbol, "equity", momentumapi.AnalysisInterval, taCfg, log); err != nil {
					errs = append(errs, err)
				}
			}
			if parts.Fundamentals {
				if _, err := fundamental.AnalyzeSymbol(ctx, pool, symbol, faCfg, log); err != nil {
					errs = append(errs, err)
				}
			}
			return errors.Join(errs...)
		},
		AnalysisConcurrency:      analysisConcurrency,
		AnalysisTimeout:          duration(log, "MOMENTUM_API_ANALYSIS_TIMEOUT", 2*time.Minute),
		AnalysisRetryAfter:       duration(log, "MOMENTUM_API_ANALYSIS_RETRY_AFTER", 3*time.Second),
		AnalysisFailedRetryAfter: duration(log, "MOMENTUM_API_ANALYSIS_FAILED_RETRY_AFTER", time.Minute),
		FundamentalsMaxAge:       duration(log, "MOMENTUM_API_FUNDAMENTALS_MAX_AGE", 26*time.Hour),
	})
	httpServer := &http.Server{
		Addr:              addr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownCtx)
	}()

	log.Info("momentum-api: listening", "addr", addr, "cors_origins", origins, "cache_ttl", cacheTTL.String(),
		"scan_grace", scanGrace.String(), "auth", "none — trusted network only")
	if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Error("momentum-api: serve", "err", err)
		os.Exit(1)
	}
}

func env(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func duration(log *slog.Logger, key string, def time.Duration) time.Duration {
	raw := env(key, "")
	if raw == "" {
		return def
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		log.Warn("momentum-api: invalid duration, using default", "key", key, "value", raw, "default", def)
		return def
	}
	return d
}

func csv(raw string) []string {
	var out []string
	for _, p := range strings.Split(raw, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// earningsSymbols is the .env fallback, mirroring data-macro-intel's precedence (data-ingestion
// internal/config: MACRO_INTEL_EARNINGS_SYMBOLS, else ALPACA_DATA_SYMBOLS,
// else EQUITY_SYMBOLS_STOCKS + _ETFS + _COMMODITY_ETFS) — the symbols it
// actually fetches earnings for. Both read the shared .env.
func earningsSymbols() []string {
	for _, key := range []string{"MACRO_INTEL_EARNINGS_SYMBOLS", "ALPACA_DATA_SYMBOLS"} {
		if s := csv(env(key, "")); len(s) > 0 {
			return s
		}
	}
	var out []string
	for _, key := range []string{"EQUITY_SYMBOLS_STOCKS", "EQUITY_SYMBOLS_ETFS", "EQUITY_SYMBOLS_COMMODITY_ETFS"} {
		out = append(out, csv(env(key, ""))...)
	}
	return out
}
