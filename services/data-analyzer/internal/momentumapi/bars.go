package momentumapi

import (
	"net/http"
	"strings"
	"time"

	"github.com/konsbe/trading-agent/services/data-analyzer/internal/store"
)

// Price history for the detail chart: GET /api/v1/scanner/symbols/{symbol}/bars?range=...
//
// 1D and 5D use stored intraday bars (5-minute, written by data-ingestion's
// intraday-bars job for candidates and watchlist symbols). When a symbol has
// none, they fall back to daily bars and say so in `fallback`, rather than
// pretending a single daily bar is an intraday chart.

const intradayInterval = "5Min"

var validRanges = map[string]bool{"1D": true, "5D": true, "1M": true, "6M": true, "1Y": true, "ALL": true}

var symbolPattern = func(s string) bool {
	if len(s) == 0 || len(s) > 15 {
		return false
	}
	for _, r := range s {
		if !(r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '.' || r == '-') {
			return false
		}
	}
	return true
}

func (s *Server) handleBars(w http.ResponseWriter, r *http.Request) {
	symbol := strings.ToUpper(strings.TrimSpace(r.PathValue("symbol")))
	rng := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("range")))
	if !symbolPattern(symbol) {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid_symbol"})
		return
	}
	if !validRanges[rng] {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid_range"})
		return
	}
	key := "bars:" + symbol + ":" + rng
	if body, ok := s.cache.get(key); ok {
		writeBody(w, http.StatusOK, body)
		return
	}
	ctx := r.Context()

	resp := barsResponse{Symbol: symbol, Range: rng, Bars: []bar{}}
	var bars []store.PriceBar

	if rng == "1D" || rng == "5D" {
		sessions := 1
		if rng == "5D" {
			sessions = 5
		}
		intraday, err := s.cfg.Store.IntradayBars(ctx, symbol, intradayInterval, sessions)
		if err != nil {
			s.storeError(w, r, err)
			return
		}
		if len(intraday) > 0 {
			resp.Interval = intradayInterval
			bars = intraday
		}
	}

	if bars == nil {
		latest, ok, err := s.cfg.Store.LatestDailyBarTS(ctx, symbol)
		if err != nil {
			s.storeError(w, r, err)
			return
		}
		if !ok {
			writeJSON(w, http.StatusNotFound, errorResponse{Error: "no_data_for_symbol"})
			return
		}
		var from time.Time
		switch rng {
		case "1D", "5D":
			from = latest.AddDate(0, 0, -14) // trimmed to the last N sessions below
		case "1M":
			from = latest.AddDate(0, -1, 0)
		case "6M":
			from = latest.AddDate(0, -6, 0)
		case "1Y":
			from = latest.AddDate(-1, 0, 0)
		}
		daily, err := s.cfg.Store.DailyBars(ctx, symbol, from)
		if err != nil {
			s.storeError(w, r, err)
			return
		}
		if n := map[string]int{"1D": 1, "5D": 5}[rng]; n > 0 {
			if len(daily) > n {
				daily = daily[len(daily)-n:]
			}
			fb := "no_intraday_data"
			resp.Fallback = &fb
		}
		resp.Interval = "1Day"
		resp.Adjusted = true
		bars = daily
	}

	for _, b := range bars {
		resp.Bars = append(resp.Bars, bar{
			Time: b.TS.Unix(), Open: b.Open, High: b.High, Low: b.Low, Close: b.Close, Volume: b.Volume,
		})
	}
	s.respondCached(w, key, resp)
}
