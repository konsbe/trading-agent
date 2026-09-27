package momentumapi

import (
	"net/http"
	"strings"
	"time"

	"github.com/konsbe/trading-agent/services/data-analyzer/internal/momentum"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/store"
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
	s.cache.drop(marketReportCacheKey) // its instrument list includes the watchlist
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
	s.cache.drop(marketReportCacheKey)
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
			Volume:    finite(it.Volume),

			DollarVolume:     finite(it.DollarVolume),
			RSI14:            finite(it.RSI14),
			BreakoutState:    it.BreakoutState,
			PctOf52wHigh:     finite(it.PctOf52wHigh),
			CatalystTier:     it.CatalystTier,
			MarketCap:        finite(it.MarketCap),
			MarketCapEst:     finite(it.MarketCapEst),
			MarketCapIsProxy: it.MarketCapIsProxy,
			IsCandidateToday: it.IsCandidateToday,
			MomentumScore100: it.MomentumScore,
			ScoreStatus:      momentum.ScoreStatus,
		}
		if it.MomentumScore != nil {
			a := momentum.Attainable(it.ScoreNullInputs)
			item.ScoreAttainable = &a
		}
		asOf := it.AsOf
		if asOf != nil {
			item.DataSource = strPtr("scanner")
		} else {
			fb, err := s.cfg.Store.BarFallback(r.Context(), it.Symbol)
			if err != nil {
				s.storeError(w, r, err)
				return
			}
			if fb != nil {
				applyBarFallback(&item, fb)
				asOf = &fb.AsOf
			}
		}
		if asOf != nil {
			d := asOf.Format(time.DateOnly)
			item.AsOf = &d
			stale, err := IsStale(*asOf, s.cfg.Now(), s.cfg.SessionReadyAfter)
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

// applyBarFallback fills a watchlist row that has no scanner row from the
// symbol's own daily bars (momentum.ComputeAt, the scanner's feature code) and
// the fundamentals provider's market cap. Scanner-only fields (catalyst tier,
// estimated market cap, score) stay null.
func applyBarFallback(item *watchlistItem, fb *store.BarFallback) {
	f := fb.Features
	bars := "daily_bars:" + fb.Source
	item.DataSource = strPtr("daily_bars")
	item.Sources = map[string]string{}
	set := func(field string, dst **float64, v *float64) {
		if *dst = finite(v); *dst != nil {
			item.Sources[field] = bars
		}
	}
	set("close", &item.Close, f.Close)
	set("change_pct", &item.ChangePct, f.ChangePct)
	set("volume", &item.Volume, f.Volume)
	set("dollar_volume", &item.DollarVolume, f.DollarVolume)
	set("rvol_20", &item.RVol20, f.RVol20)
	set("rsi_14", &item.RSI14, f.RSI14)
	set("pct_of_52w_high", &item.PctOf52wHigh, f.PctOf52wHigh)
	if f.BreakoutState != nil {
		item.BreakoutState = strPtr(string(*f.BreakoutState))
		item.Sources["breakout_state"] = bars
	}
	item.MarketCap = finite(fb.MarketCap)
	if item.MarketCap != nil {
		item.Sources["market_cap"] = "finnhub_metric"
	} else {
		item.MarketCapNote = fb.MarketCapNote
	}
}
