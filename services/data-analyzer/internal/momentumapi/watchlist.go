package momentumapi

import (
	"net/http"
	"strings"
	"time"
)

// Watchlist endpoints: the one write path in this service, designed separately
// from the read-only scanner routes (docs/MOMENTUM_SCANNER_API.md §2.5).
//
//	GET    /api/v1/watchlist           list
//	PUT    /api/v1/watchlist/{symbol}  add (idempotent)
//	DELETE /api/v1/watchlist/{symbol}  remove (idempotent)
//	GET    /api/v1/symbols?q=          search, for the add flow
//
// Never cached: a list must reflect the write that just happened.

// unauthenticatedOwner labels the shared list in responses. The database
// stores it as owner_sub NULL, never as this string.
const unauthenticatedOwner = "unauthenticated"

// ownerFromRequest returns the signed-in user's subject, or nil for the shared
// unauthenticated list. There is no auth yet, so it is always nil; when token
// validation lands, this is the single place that reads the verified subject.
func ownerFromRequest(*http.Request) *string { return nil }

func ownerLabel(owner *string) string {
	if owner == nil {
		return unauthenticatedOwner
	}
	return *owner
}

func (s *Server) handleWatchlistList(w http.ResponseWriter, r *http.Request) {
	s.writeWatchlist(w, r, http.StatusOK)
}

func (s *Server) handleWatchlistAdd(w http.ResponseWriter, r *http.Request) {
	symbol := strings.ToUpper(strings.TrimSpace(r.PathValue("symbol")))
	if !symbolPattern(symbol) {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid_symbol"})
		return
	}
	ctx := r.Context()
	known, err := s.cfg.Store.SymbolKnown(ctx, symbol)
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	if !known {
		writeJSON(w, http.StatusNotFound, errorResponse{Error: "unknown_symbol"})
		return
	}
	added, err := s.cfg.Store.AddToWatchlist(ctx, ownerFromRequest(r), symbol)
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	status := http.StatusOK
	if added {
		status = http.StatusCreated
	}
	s.writeWatchlist(w, r, status)
}

func (s *Server) handleWatchlistRemove(w http.ResponseWriter, r *http.Request) {
	symbol := strings.ToUpper(strings.TrimSpace(r.PathValue("symbol")))
	if !symbolPattern(symbol) {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid_symbol"})
		return
	}
	if _, err := s.cfg.Store.RemoveFromWatchlist(r.Context(), ownerFromRequest(r), symbol); err != nil {
		s.storeError(w, r, err)
		return
	}
	s.writeWatchlist(w, r, http.StatusOK)
}

// writeWatchlist responds with the caller's current list, so the client never
// needs a second request after a write.
func (s *Server) writeWatchlist(w http.ResponseWriter, r *http.Request, status int) {
	owner := ownerFromRequest(r)
	items, err := s.cfg.Store.ListWatchlist(r.Context(), owner)
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	resp := watchlistResponse{Owner: ownerLabel(owner), Items: []watchlistItem{}}
	for _, it := range items {
		item := watchlistItem{
			Symbol: it.Symbol, CompanyName: it.CompanyName, Exchange: it.Exchange,
			AddedAt:   it.AddedAt.UTC().Format(time.RFC3339),
			IsStale:   true,
			Close:     finite(it.Close),
			ChangePct: finite(it.ChangePct),
			RVol20:    finite(it.RVol20),
		}
		if it.AsOf != nil {
			d := it.AsOf.Format(time.DateOnly)
			item.AsOf = &d
			stale, err := IsStale(*it.AsOf, s.cfg.Now(), s.cfg.SessionReadyAfter)
			if err != nil {
				s.cfg.Log.Error("momentum-api: watchlist is_stale", "err", err)
				writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "session_calendar_unavailable"})
				return
			}
			item.IsStale = stale
		}
		resp.Items = append(resp.Items, item)
	}
	writeJSON(w, status, resp)
}

const (
	symbolSearchLimit    = 20
	symbolSearchMaxQuery = 40
)

// handleSymbolSearch backs the watchlist's add flow. Read-only; PUT
// /watchlist/{symbol} stays the authority on what can be added (its 404
// unknown_symbol), this only helps the user find a symbol.
func (s *Server) handleSymbolSearch(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" || len([]rune(q)) > symbolSearchMaxQuery {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid_query"})
		return
	}
	matches, err := s.cfg.Store.SearchSymbols(r.Context(), q, symbolSearchLimit)
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	resp := symbolSearchResponse{Query: q, Results: []symbolMatch{}}
	for _, m := range matches {
		resp.Results = append(resp.Results, symbolMatch{
			Symbol: m.Symbol, CompanyName: m.CompanyName, Exchange: m.Exchange, IsEligible: m.IsEligible,
		})
	}
	writeJSON(w, http.StatusOK, resp)
}
