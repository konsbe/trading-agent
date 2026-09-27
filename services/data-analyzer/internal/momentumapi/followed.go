package momentumapi

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/konsbe/trading-agent/services/data-analyzer/internal/store"
)

// Followed symbols and computation tracking (migration 030). momentum-api is
// only the UI-facing layer: it writes followed_symbols and manual Compute
// requests; the ingestion services read them from Postgres at their own
// cadence, and momentum-daily computes. It never calls a data provider.
//
//	GET    /api/v1/followed-symbols            list
//	PUT    /api/v1/followed-symbols/{symbol}   follow (idempotent)
//	DELETE /api/v1/followed-symbols/{symbol}   unfollow (idempotent)
//	GET    /api/v1/symbols/directory?q=        "all symbols" search (symbol_directory)
//	GET    /api/v1/computed-symbols            symbols with an open reason, with state
//	PUT    /api/v1/computed-symbols/{symbol}   Compute: open a manual reason
//	DELETE /api/v1/computed-symbols/{symbol}   Stop computing: close the manual reason
//
// Never cached: a list must reflect the write that just happened.

// FollowStore is the store half these endpoints need, separate from Store.
type FollowStore interface {
	ListFollowed(ctx context.Context) ([]store.FollowedRow, error)
	ClassifySymbol(ctx context.Context, symbol string) (store.SymbolClass, bool, error)
	AddFollowed(ctx context.Context, c store.SymbolClass) (bool, error)
	RemoveFollowed(ctx context.Context, symbol string) (bool, error)
	SearchDirectory(ctx context.Context, query string, limit int) ([]store.DirectoryMatch, error)
	ComputedSymbols(ctx context.Context) ([]store.ComputedRow, error)
	RequestCompute(ctx context.Context, c store.SymbolClass) (bool, error)
	StopCompute(ctx context.Context, symbol string) (bool, error)
}

func (s DBStore) ListFollowed(ctx context.Context) ([]store.FollowedRow, error) {
	return store.ListFollowed(ctx, s.Q)
}
func (s DBStore) ClassifySymbol(ctx context.Context, sym string) (store.SymbolClass, bool, error) {
	return store.ClassifySymbol(ctx, s.Q, sym)
}
func (s DBStore) AddFollowed(ctx context.Context, c store.SymbolClass) (bool, error) {
	return store.AddFollowed(ctx, s.Q.(store.TxBeginner), c)
}
func (s DBStore) RemoveFollowed(ctx context.Context, sym string) (bool, error) {
	return store.RemoveFollowed(ctx, s.Q.(store.TxBeginner), sym)
}
func (s DBStore) SearchDirectory(ctx context.Context, q string, limit int) ([]store.DirectoryMatch, error) {
	return store.SearchDirectory(ctx, s.Q, q, limit)
}
func (s DBStore) ComputedSymbols(ctx context.Context) ([]store.ComputedRow, error) {
	return store.ComputedSymbols(ctx, s.Q)
}
func (s DBStore) RequestCompute(ctx context.Context, c store.SymbolClass) (bool, error) {
	return store.RequestCompute(ctx, s.Q, c)
}
func (s DBStore) StopCompute(ctx context.Context, sym string) (bool, error) {
	return store.StopCompute(ctx, s.Q, sym)
}

type followedItem struct {
	Symbol    string  `json:"symbol"`
	Name      *string `json:"name"`
	AssetType string  `json:"asset_type"`
	Listing   string  `json:"listing"`
	NewsAlias *string `json:"news_alias"`
	Source    string  `json:"source"` // env_seed | user
	AddedAt   string  `json:"added_at"`
}

type followedResponse struct {
	Items []followedItem `json:"items"`
}

type directoryMatch struct {
	Symbol     string  `json:"symbol"`
	Name       *string `json:"name"`
	Type       *string `json:"type"`
	MIC        *string `json:"mic"`
	AssetType  string  `json:"asset_type"`
	Source     string  `json:"source"`
	InUniverse bool    `json:"in_universe"`
	Followed   bool    `json:"followed"`
}

type directorySearchResponse struct {
	Query   string           `json:"query"`
	Results []directoryMatch `json:"results"`
}

// Computation states, for the computed-symbols view and the Compute button. A
// manual Compute request and a watchlist addition both queue a fetch; "queued"
// below means either.
const (
	computeComputed       = "computed"         // computed since the reason (or request) opened
	computeScheduled      = "scheduled"        // automatic reason, not computed yet: the next daily pass
	computeWaitingForData = "waiting_for_data" // queued; ingestion has not fetched it yet
	computeDataNotArrived = "data_not_arrived" // queued longer than the timeout, data still missing
	computeComputing      = "computing"        // data fetched; the next compute poll picks it up
	computeFailed         = "failed"           // the last computation failed (last_error)
)

type computedItem struct {
	Symbol            string   `json:"symbol"`
	Name              *string  `json:"name"`
	AssetType         string   `json:"asset_type"`
	Reasons           []string `json:"reasons"`
	ManualRequestedAt *string  `json:"manual_requested_at"`
	// QueuedAt: newest open manual or watchlist reason — when the fetch was queued.
	QueuedAt              *string `json:"queued_at"`
	State                 string  `json:"state"`
	BarsFetchedAt         *string `json:"bars_fetched_at"`
	FundamentalsFetchedAt *string `json:"fundamentals_fetched_at"`
	ComputedAt            *string `json:"computed_at"`
	LastError             *string `json:"last_error"`
	StatementsStatus      *string `json:"statements_status"`
	StatementsReason      *string `json:"statements_reason"`
}

type computedResponse struct {
	// DataTimeoutMinutes: a manual request whose data has not arrived after
	// this long reads data_not_arrived instead of waiting indefinitely.
	DataTimeoutMinutes int            `json:"data_timeout_minutes"`
	Items              []computedItem `json:"items"`
}

func tsPtr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	v := t.UTC().Format(time.RFC3339)
	return &v
}

// computeState derives a row's state. Pure, so the rules are testable.
func computeState(r store.ComputedRow, now time.Time, timeout time.Duration) string {
	failed := r.LastError != nil && strings.HasPrefix(*r.LastError, "compute:")
	if r.QueuedSince == nil {
		switch {
		case failed:
			return computeFailed
		case r.ComputedAt != nil:
			return computeComputed
		}
		return computeScheduled
	}
	since := *r.QueuedSince
	if r.ComputedAt != nil && !r.ComputedAt.Before(since) {
		return computeComputed
	}
	after := func(t *time.Time) bool { return t != nil && !t.Before(since) }
	dataReady := after(r.BarsFetchedAt) && (r.AssetType == "crypto" || after(r.FundamentalsFetchedAt))
	switch {
	case dataReady && failed:
		return computeFailed
	case dataReady:
		return computeComputing
	case now.Sub(since) > timeout:
		return computeDataNotArrived
	}
	return computeWaitingForData
}

func (s *Server) followStore(w http.ResponseWriter) (FollowStore, bool) {
	if s.cfg.Follow == nil {
		writeJSON(w, http.StatusNotImplemented, errorResponse{Error: "followed_symbols_unavailable"})
		return nil, false
	}
	return s.cfg.Follow, true
}

// classifyOr404 resolves a path symbol, answering 400 / 404 / 422 itself.
func (s *Server) classifyOr404(w http.ResponseWriter, r *http.Request, fs FollowStore) (store.SymbolClass, bool) {
	symbol := strings.ToUpper(strings.TrimSpace(r.PathValue("symbol")))
	if !symbolPattern(symbol) {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid_symbol"})
		return store.SymbolClass{}, false
	}
	c, found, err := fs.ClassifySymbol(r.Context(), symbol)
	switch {
	case errors.Is(err, store.ErrNotComputable):
		writeJSON(w, http.StatusUnprocessableEntity, errorResponse{Error: "not_computable"})
		return c, false
	case err != nil:
		s.storeError(w, r, err)
		return c, false
	case !found:
		writeJSON(w, http.StatusNotFound, errorResponse{Error: "unknown_symbol"})
		return c, false
	}
	return c, true
}

func (s *Server) handleFollowedList(w http.ResponseWriter, r *http.Request) {
	if fs, ok := s.followStore(w); ok {
		s.writeFollowed(w, r, fs, http.StatusOK)
	}
}

func (s *Server) handleFollowedAdd(w http.ResponseWriter, r *http.Request) {
	fs, ok := s.followStore(w)
	if !ok {
		return
	}
	c, ok := s.classifyOr404(w, r, fs)
	if !ok {
		return
	}
	added, err := fs.AddFollowed(r.Context(), c)
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	status := http.StatusOK
	if added {
		status = http.StatusCreated
	}
	s.writeFollowed(w, r, fs, status)
}

func (s *Server) handleFollowedRemove(w http.ResponseWriter, r *http.Request) {
	fs, ok := s.followStore(w)
	if !ok {
		return
	}
	symbol := strings.ToUpper(strings.TrimSpace(r.PathValue("symbol")))
	if !symbolPattern(symbol) {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid_symbol"})
		return
	}
	if _, err := fs.RemoveFollowed(r.Context(), symbol); err != nil {
		s.storeError(w, r, err)
		return
	}
	s.writeFollowed(w, r, fs, http.StatusOK)
}

func (s *Server) writeFollowed(w http.ResponseWriter, r *http.Request, fs FollowStore, status int) {
	rows, err := fs.ListFollowed(r.Context())
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	resp := followedResponse{Items: []followedItem{}}
	for _, f := range rows {
		resp.Items = append(resp.Items, followedItem{Symbol: f.Symbol, Name: f.Name, AssetType: f.AssetType,
			Listing: f.Listing, NewsAlias: f.NewsAlias, Source: f.Source, AddedAt: f.AddedAt.UTC().Format(time.RFC3339)})
	}
	writeJSON(w, status, resp)
}

// handleDirectorySearch is the "all symbols" search: ETFs, ADRs, OTC and crypto
// outside the scanner's universe (GET /api/v1/symbols covers the universe).
func (s *Server) handleDirectorySearch(w http.ResponseWriter, r *http.Request) {
	fs, ok := s.followStore(w)
	if !ok {
		return
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" || len([]rune(q)) > symbolSearchMaxQuery {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid_query"})
		return
	}
	matches, err := fs.SearchDirectory(r.Context(), q, symbolSearchLimit)
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	resp := directorySearchResponse{Query: q, Results: []directoryMatch{}}
	for _, m := range matches {
		resp.Results = append(resp.Results, directoryMatch{Symbol: m.Symbol, Name: m.Name, Type: m.Type, MIC: m.MIC,
			AssetType: m.AssetType, Source: m.Source, InUniverse: m.InUniverse, Followed: m.Followed})
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleComputedList(w http.ResponseWriter, r *http.Request) {
	if fs, ok := s.followStore(w); ok {
		s.writeComputed(w, r, fs, http.StatusOK)
	}
}

func (s *Server) handleComputeRequest(w http.ResponseWriter, r *http.Request) {
	fs, ok := s.followStore(w)
	if !ok {
		return
	}
	c, ok := s.classifyOr404(w, r, fs)
	if !ok {
		return
	}
	if _, err := fs.RequestCompute(r.Context(), c); err != nil {
		s.storeError(w, r, err)
		return
	}
	s.writeComputed(w, r, fs, http.StatusAccepted)
}

func (s *Server) handleComputeStop(w http.ResponseWriter, r *http.Request) {
	fs, ok := s.followStore(w)
	if !ok {
		return
	}
	symbol := strings.ToUpper(strings.TrimSpace(r.PathValue("symbol")))
	if !symbolPattern(symbol) {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid_symbol"})
		return
	}
	if _, err := fs.StopCompute(r.Context(), symbol); err != nil {
		s.storeError(w, r, err)
		return
	}
	s.writeComputed(w, r, fs, http.StatusOK)
}

func (s *Server) writeComputed(w http.ResponseWriter, r *http.Request, fs FollowStore, status int) {
	rows, err := fs.ComputedSymbols(r.Context())
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	now, timeout := s.cfg.Now(), s.cfg.ComputeDataTimeout
	resp := computedResponse{DataTimeoutMinutes: int(timeout / time.Minute), Items: []computedItem{}}
	for _, c := range rows {
		resp.Items = append(resp.Items, computedItem{
			Symbol: c.Symbol, Name: c.Name, AssetType: c.AssetType, Reasons: c.Reasons,
			ManualRequestedAt: tsPtr(c.ManualSince), QueuedAt: tsPtr(c.QueuedSince), State: computeState(c, now, timeout),
			BarsFetchedAt: tsPtr(c.BarsFetchedAt), FundamentalsFetchedAt: tsPtr(c.FundamentalsFetchedAt),
			ComputedAt: tsPtr(c.ComputedAt), LastError: c.LastError,
			StatementsStatus: c.StatementsStatus, StatementsReason: c.StatementsReason,
		})
	}
	writeJSON(w, status, resp)
}
