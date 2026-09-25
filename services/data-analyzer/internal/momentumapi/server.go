package momentumapi

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/konsbe/trading-agent/services/data-analyzer/internal/momentum"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/store"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/technical"
)

// Store is the read surface the handlers need. The production implementation
// (DBStore) runs the internal/store queries; tests substitute a fake or run
// DBStore inside a rolled-back transaction.
type Store interface {
	LatestScanDate(ctx context.Context) (time.Time, bool, error)
	ScanSummary(ctx context.Context, date time.Time) (store.ScanSummary, error)
	Candidates(ctx context.Context, date, alertsSince time.Time) ([]store.CandidateRow, error)
	SymbolDetail(ctx context.Context, symbol string) (store.SymbolDetailRow, bool, error)
	LatestDailyBarTS(ctx context.Context, symbol string) (time.Time, bool, error)
	DailyBars(ctx context.Context, symbol string, from time.Time) ([]store.PriceBar, error)
	IntradayBars(ctx context.Context, symbol, interval string, sessions int) ([]store.PriceBar, error)
	SymbolKnown(ctx context.Context, symbol string) (bool, error)
	ListWatchlist(ctx context.Context, owner *string) ([]store.WatchlistItem, error)
	AddToWatchlist(ctx context.Context, owner *string, symbol string) (bool, error)
	RemoveFromWatchlist(ctx context.Context, owner *string, symbol string) (bool, error)
	SearchSymbols(ctx context.Context, query string, limit int) ([]store.SymbolMatch, error)
	ProviderBudgets(ctx context.Context, keys []string) ([]store.ProviderBudget, error)
	ChainRunsBetween(ctx context.Context, from, to time.Time) (map[string]store.ChainRun, time.Time, bool, error)
	LastCleanSession(ctx context.Context) (*time.Time, error)
	SessionCoverage(ctx context.Context, sessions []time.Time, source string) (map[string]float64, error)
	MarketReport(ctx context.Context, fixed []store.InstrumentRef, earningsSymbols, fredSeries []string, now time.Time) (store.MarketReportInputs, error)
	TrackedPositions(ctx context.Context, status store.TrackedStatusFilter, latestScan *time.Time) ([]store.TrackedPositionRow, error)
	TrackedCounts(ctx context.Context) (store.TrackedCounts, error)
	AnalysisFreshness(ctx context.Context, symbol string) (store.AnalysisFreshness, error)
	Analysis(ctx context.Context, symbol string) (store.AnalysisInputs, error)
	Ping(ctx context.Context) error
}

// DBStore adapts internal/store's read functions to Store.
type DBStore struct {
	Q      store.ReadWriter
	PingFn func(ctx context.Context) error
}

func (s DBStore) LatestScanDate(ctx context.Context) (time.Time, bool, error) {
	return store.LatestScanDate(ctx, s.Q)
}
func (s DBStore) ScanSummary(ctx context.Context, d time.Time) (store.ScanSummary, error) {
	return store.GetScanSummary(ctx, s.Q, d)
}
func (s DBStore) Candidates(ctx context.Context, d, alertsSince time.Time) ([]store.CandidateRow, error) {
	return store.Candidates(ctx, s.Q, d, alertsSince)
}
func (s DBStore) SymbolDetail(ctx context.Context, sym string) (store.SymbolDetailRow, bool, error) {
	return store.SymbolDetail(ctx, s.Q, sym)
}
func (s DBStore) LatestDailyBarTS(ctx context.Context, sym string) (time.Time, bool, error) {
	return store.LatestDailyBarTS(ctx, s.Q, sym)
}
func (s DBStore) DailyBars(ctx context.Context, sym string, from time.Time) ([]store.PriceBar, error) {
	return store.DailyBars(ctx, s.Q, sym, from)
}
func (s DBStore) IntradayBars(ctx context.Context, sym, interval string, sessions int) ([]store.PriceBar, error) {
	return store.IntradayBars(ctx, s.Q, sym, interval, sessions)
}
func (s DBStore) SymbolKnown(ctx context.Context, sym string) (bool, error) {
	return store.SymbolKnown(ctx, s.Q, sym)
}
func (s DBStore) ListWatchlist(ctx context.Context, owner *string) ([]store.WatchlistItem, error) {
	return store.ListWatchlist(ctx, s.Q, owner)
}
func (s DBStore) SearchSymbols(ctx context.Context, query string, limit int) ([]store.SymbolMatch, error) {
	return store.SearchSymbols(ctx, s.Q, query, limit)
}
func (s DBStore) ProviderBudgets(ctx context.Context, keys []string) ([]store.ProviderBudget, error) {
	return store.ProviderBudgets(ctx, s.Q, keys)
}
func (s DBStore) ChainRunsBetween(ctx context.Context, from, to time.Time) (map[string]store.ChainRun, time.Time, bool, error) {
	return store.ChainRunsBetween(ctx, s.Q, from, to)
}
func (s DBStore) LastCleanSession(ctx context.Context) (*time.Time, error) {
	return store.LastCleanSession(ctx, s.Q)
}
func (s DBStore) MarketReport(ctx context.Context, fixed []store.InstrumentRef, earningsSymbols, fredSeries []string, now time.Time) (store.MarketReportInputs, error) {
	return store.LoadMarketReport(ctx, s.Q, fixed, earningsSymbols, fredSeries, now)
}
func (s DBStore) SessionCoverage(ctx context.Context, sessions []time.Time, source string) (map[string]float64, error) {
	return store.SessionCoverage(ctx, s.Q, sessions, source)
}
func (s DBStore) AddToWatchlist(ctx context.Context, owner *string, sym string) (bool, error) {
	return store.AddToWatchlist(ctx, s.Q, owner, sym)
}
func (s DBStore) RemoveFromWatchlist(ctx context.Context, owner *string, sym string) (bool, error) {
	return store.RemoveFromWatchlist(ctx, s.Q, owner, sym)
}
func (s DBStore) TrackedPositions(ctx context.Context, st store.TrackedStatusFilter, latest *time.Time) ([]store.TrackedPositionRow, error) {
	return store.TrackedPositions(ctx, s.Q, st, latest)
}
func (s DBStore) TrackedCounts(ctx context.Context) (store.TrackedCounts, error) {
	return store.GetTrackedCounts(ctx, s.Q)
}
func (s DBStore) AnalysisFreshness(ctx context.Context, sym string) (store.AnalysisFreshness, error) {
	return store.AnalysisFreshnessFor(ctx, s.Q, sym, AnalysisInterval)
}
func (s DBStore) Analysis(ctx context.Context, sym string) (store.AnalysisInputs, error) {
	return store.LoadAnalysis(ctx, s.Q, sym, AnalysisInterval, analysisHeadlines)
}
func (s DBStore) Ping(ctx context.Context) error { return s.PingFn(ctx) }

// RecentAlertWindow is how far back a candidate's recent_alert looks: the
// newest equity fired_alerts row fired within this window before the request
// (full-stock-analysis addendum §4). Wide enough to span the gap between the
// evening alert scan and the next morning's view, short enough that the badge
// means "worth a look now". The 5-minute response cache can hold an alert up
// to one TTL past the window.
const RecentAlertWindow = 24 * time.Hour

// Buckets always present in /today, even when empty: zero candidates is the
// empty state, not a missing bucket.
var bucketNames = []string{"market", "penny"}

type Config struct {
	Store   Store
	Caveats Caveats
	// BacktestReport is the frozen Backtest Lab report (loaded at startup).
	BacktestReport BacktestReport
	Log            *slog.Logger

	// SessionReadyAfter is how long after the 16:00 New York close a session's
	// scan is expected to exist before is_stale flips (see ExpectedSession).
	SessionReadyAfter time.Duration
	CacheTTL          time.Duration
	CORSOrigins       []string

	// Data Source status page. StatusCacheTTL is deliberately short (30–60s):
	// long enough to absorb a double-clicked refresh, short enough that
	// "last checked" stays honest. ChainGiveUpAfter mirrors momentum-daily's
	// MOMENTUM_DAILY_GIVE_UP_AFTER so "pending" ends when the daemon gives up.
	StatusCacheTTL     time.Duration
	ChainGiveUpAfter   time.Duration
	BudgetAttentionPct float64
	SessionsShown      int
	BarSource          string

	// MarketReportCacheTTL: the report is regenerated every 6h, so it is not
	// held to the 5-minute data TTL; watchlist writes drop it early.
	MarketReportCacheTTL time.Duration
	// EarningsCoveredSymbols is the list data-macro-intel fetches earnings for
	// (read by main from the same env vars, same precedence), so the report can
	// tell "not ingested" from "no earnings in the window".
	EarningsCoveredSymbols []string
	// GPRSourceConfigured mirrors data-macro-intel's GPR_CSV_URL: without it the
	// geopolitical-risk index is never ingested.
	GPRSourceConfigured bool

	// Full stock analysis (GET /today/{symbol}/analysis). AnalysisNames are the
	// stored indicator names (technical.NamesFor the worker's config).
	// AnalysisCompute runs the workers' per-symbol code for a symbol whose
	// stored analysis is missing or out of date; nil disables on-demand
	// computation (stored rows are served as they are).
	AnalysisNames   technical.Names
	AnalysisCompute AnalysisComputer
	// AnalysisConcurrency bounds simultaneous computations (default 2);
	// AnalysisTimeout bounds each, queueing included (default 2m).
	AnalysisConcurrency int
	AnalysisTimeout     time.Duration
	// AnalysisRetryAfter is the poll interval suggested with "computing"
	// (default 3s); AnalysisFailedRetryAfter how long a failure is reported
	// before a request starts a new attempt (default 1m).
	AnalysisRetryAfter       time.Duration
	AnalysisFailedRetryAfter time.Duration
	// FundamentalsMaxAge is how old derived fundamentals may be before they are
	// recomputed (default 26h: the worker's 24h cadence plus slack).
	FundamentalsMaxAge time.Duration

	// Now is injectable for tests; defaults to time.Now.
	Now func() time.Time
}

type Server struct {
	cfg         Config
	cache       *responseCache
	statusCache *responseCache
	analysis    *analysisJobs
}

func NewServer(cfg Config) *Server {
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.Log == nil {
		cfg.Log = slog.Default()
	}
	if cfg.SessionsShown <= 0 {
		cfg.SessionsShown = 7
	}
	if cfg.ChainGiveUpAfter <= 0 {
		cfg.ChainGiveUpAfter = 14 * time.Hour
	}
	if cfg.BudgetAttentionPct <= 0 {
		cfg.BudgetAttentionPct = 90
	}
	if cfg.BarSource == "" {
		cfg.BarSource = "tiingo"
	}
	if cfg.MarketReportCacheTTL <= 0 {
		cfg.MarketReportCacheTTL = time.Hour
	}
	if cfg.AnalysisConcurrency <= 0 {
		cfg.AnalysisConcurrency = 2
	}
	if cfg.AnalysisTimeout <= 0 {
		cfg.AnalysisTimeout = 2 * time.Minute
	}
	if cfg.AnalysisRetryAfter <= 0 {
		cfg.AnalysisRetryAfter = 3 * time.Second
	}
	if cfg.AnalysisFailedRetryAfter <= 0 {
		cfg.AnalysisFailedRetryAfter = time.Minute
	}
	if cfg.FundamentalsMaxAge <= 0 {
		cfg.FundamentalsMaxAge = 26 * time.Hour
	}
	srv := &Server{cfg: cfg, cache: newResponseCache(cfg.CacheTTL, cfg.Now),
		statusCache: newResponseCache(cfg.StatusCacheTTL, cfg.Now)}
	if cfg.AnalysisCompute != nil {
		srv.analysis = newAnalysisJobs(cfg.AnalysisCompute, cfg.AnalysisConcurrency, cfg.AnalysisTimeout, cfg.Now, cfg.Log)
	}
	return srv
}

// Handler returns the HTTP routes wrapped in CORS handling.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealth)
	mux.HandleFunc("GET /api/v1/scanner/today", s.handleToday)
	mux.HandleFunc("GET /api/v1/scanner/today/{symbol}", s.handleDetail)
	mux.HandleFunc("GET /api/v1/scanner/today/{symbol}/analysis", s.handleAnalysis)
	mux.HandleFunc("GET /api/v1/scanner/symbols/{symbol}/bars", s.handleBars)
	mux.HandleFunc("GET /api/v1/scanner/tracked", s.handleTracked)
	mux.HandleFunc("GET /api/v1/watchlist", s.handleWatchlistList)
	mux.HandleFunc("PUT /api/v1/watchlist/{symbol}", s.handleWatchlistAdd)
	mux.HandleFunc("DELETE /api/v1/watchlist/{symbol}", s.handleWatchlistRemove)
	mux.HandleFunc("GET /api/v1/symbols", s.handleSymbolSearch)
	mux.HandleFunc("GET /api/v1/backtest-lab/report", s.handleBacktestReport)
	mux.HandleFunc("GET /api/v1/data-sources/status", s.handleDataSourcesStatus)
	mux.HandleFunc("GET /api/v1/market-report/today", s.handleMarketReport)
	return s.cors(mux)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if err := s.cfg.Store.Ping(r.Context()); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "database_unavailable"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleToday(w http.ResponseWriter, r *http.Request) {
	const key = "today"
	if body, ok := s.cache.get(key); ok {
		writeBody(w, http.StatusOK, body)
		return
	}
	ctx := r.Context()

	date, ok, err := s.cfg.Store.LatestScanDate(ctx)
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	if !ok {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "no_scan_available"})
		return
	}
	summary, err := s.cfg.Store.ScanSummary(ctx, date)
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	now := s.cfg.Now()
	rows, err := s.cfg.Store.Candidates(ctx, date, now.Add(-RecentAlertWindow))
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	stale, err := IsStale(date, now, s.cfg.SessionReadyAfter)
	if err != nil {
		s.cfg.Log.Error("momentum-api: is_stale", "err", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "session_calendar_unavailable"})
		return
	}

	resp := todayResponse{
		Scan: scanInfo{
			Date:             date.Format(time.DateOnly),
			CompletedAt:      summary.CompletedAt.UTC().Format(time.RFC3339),
			UniverseScanned:  summary.UniverseScanned,
			UniverseEligible: summary.UniverseEligible,
			IsStale:          stale,
		},
		Buckets: map[string]bucket{},
	}
	for _, name := range bucketNames {
		resp.Buckets[name] = bucket{Candidates: []candidate{}}
	}
	for _, row := range rows {
		b, known := resp.Buckets[row.Bucket]
		if !known {
			s.cfg.Log.Error("momentum-api: candidate in unknown bucket", "symbol", row.Symbol, "bucket", row.Bucket)
			writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "internal_error"})
			return
		}
		b.Candidates = append(b.Candidates, toCandidate(row))
		b.TotalCandidates = len(b.Candidates)
		resp.Buckets[row.Bucket] = b
	}
	s.respondCached(w, key, resp)
}

func (s *Server) handleDetail(w http.ResponseWriter, r *http.Request) {
	symbol := strings.ToUpper(strings.TrimSpace(r.PathValue("symbol")))
	key := "detail:" + symbol
	if body, ok := s.cache.get(key); ok {
		writeBody(w, http.StatusOK, body)
		return
	}
	ctx := r.Context()

	date, ok, err := s.cfg.Store.LatestScanDate(ctx)
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	if !ok {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "no_scan_available"})
		return
	}
	row, found, err := s.cfg.Store.SymbolDetail(ctx, symbol)
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	if !found {
		// A real absence: the scanner has never written a row for it.
		writeJSON(w, http.StatusNotFound, errorResponse{Error: "no_data_for_symbol"})
		return
	}
	stale, err := IsStale(row.TS, s.cfg.Now(), s.cfg.SessionReadyAfter)
	if err != nil {
		s.cfg.Log.Error("momentum-api: detail is_stale", "err", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "session_calendar_unavailable"})
		return
	}
	latest := date.UTC().Format(time.DateOnly)
	asOf := row.TS.UTC().Format(time.DateOnly)

	resp := detailResponse{
		Symbol:           row.Symbol,
		Exchange:         row.Exchange,
		CompanyName:      row.CompanyName,
		Bucket:           row.Bucket,
		AsOf:             asOf,
		IsStale:          stale,
		LatestScanDate:   latest,
		IsCandidateToday: row.GatesPassed && asOf == latest,
		GatesPassed:      row.GatesPassed,
		GateFailures:     nonNil(row.GateFailures),
		Gates:            buildGateChecks(row),
		Facts:            buildFacts(row.Facts),
		EvidenceNote:     s.cfg.Caveats.Evidence,
	}
	if sc := row.Score; sc != nil {
		resp.Score = &scoreDetail{
			Total:        sc.Total,
			Attainable:   momentum.Attainable(sc.NullInputs),
			Allocated:    momentum.WeightAllocated,
			Status:       momentum.ScoreStatus,
			ModelVersion: momentum.ScoreModelVersion,
			SubScores: map[string]*float64{
				"rvol":      finite(sc.RVol),
				"vol_accel": finite(sc.VolAccel),
				"catalyst":  finite(sc.Catalyst),
				"float":     finite(sc.Float),
				"vwap":      finite(sc.VWAP),
				"breakout":  finite(sc.Breakout),
				"high52w":   finite(sc.High52w),
			},
			Weights:      componentWeights(),
			PenaltyRules: buildPenaltyRules(sc.Penalties),
			Penalties:    nonNil(sc.Penalties),
			PenaltyTotal: finite(sc.PenaltyTotal),
			NullInputs:   nonNil(sc.NullInputs),
			Caveat:       s.cfg.Caveats.ResearchScore,
		}
	}
	s.respondCached(w, key, resp)
}

func toCandidate(row store.CandidateRow) candidate {
	var attainable *int
	if row.MomentumScore != nil {
		a := momentum.Attainable(row.ScoreNullInputs)
		attainable = &a
	}
	return candidate{
		Symbol:           row.Symbol,
		Exchange:         row.Exchange,
		CompanyName:      row.CompanyName,
		Bucket:           row.Bucket,
		Close:            finite(row.Close),
		ChangePct:        finite(row.ChangePct),
		RVol20:           finite(row.RVol20),
		DollarVolume:     finite(row.DollarVolume),
		RSI14:            finite(row.RSI14),
		BreakoutState:    row.BreakoutState,
		PctOf52wHigh:     finite(row.PctOf52wHigh),
		CatalystTier:     row.CatalystTier,
		MarketCap:        finite(row.MarketCap),
		MarketCapEst:     finite(row.MarketCapEst),
		MarketCapIsProxy: row.MarketCapIsProxy,
		MomentumScore100: row.MomentumScore,
		ScoreAttainable:  attainable,
		ScoreStatus:      momentum.ScoreStatus,
		RecentAlert:      toRecentAlert(row.RecentAlert),
	}
}

func toRecentAlert(a *store.RecentAlert) *recentAlert {
	if a == nil {
		return nil
	}
	return &recentAlert{AlertType: a.AlertType, Severity: a.Severity, Message: a.Message,
		FiredAt: a.FiredAt.UTC().Format(time.RFC3339)}
}

// storeError distinguishes an unreachable database (503) from a failing query
// against a healthy one (500), so a SQL bug never masquerades as an outage.
func (s *Server) storeError(w http.ResponseWriter, r *http.Request, err error) {
	if pingErr := s.cfg.Store.Ping(r.Context()); pingErr != nil {
		s.cfg.Log.Warn("momentum-api: database unavailable", "err", err, "ping", pingErr)
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "database_unavailable"})
		return
	}
	s.cfg.Log.Error("momentum-api: query failed", "path", r.URL.Path, "err", err)
	writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "internal_error"})
}

func (s *Server) respondCached(w http.ResponseWriter, key string, v any) {
	body, err := json.Marshal(v)
	if err != nil {
		s.cfg.Log.Error("momentum-api: encode response", "err", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "internal_error"})
		return
	}
	s.cache.set(key, body)
	writeBody(w, http.StatusOK, body)
}

func (s *Server) cors(next http.Handler) http.Handler {
	allowed := map[string]bool{}
	for _, o := range s.cfg.CORSOrigins {
		allowed[o] = true
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && (allowed[origin] || allowed["*"]) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type")
			w.Header().Add("Vary", "Origin")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	body, err := json.Marshal(v)
	if err != nil {
		http.Error(w, `{"error":"internal_error"}`, http.StatusInternalServerError)
		return
	}
	writeBody(w, status, body)
}

func writeBody(w http.ResponseWriter, status int, body []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}
