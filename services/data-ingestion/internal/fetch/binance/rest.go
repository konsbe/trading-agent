package binance

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/berdelis/trading-agent/services/data-ingestion/internal/store"
	"golang.org/x/time/rate"
)

const restBase = "https://api.binance.com"

type REST struct {
	HTTP     *http.Client
	Limiter  *rate.Limiter
	Exchange string
}

func NewREST() *REST {
	return &REST{
		HTTP:     &http.Client{Timeout: 25 * time.Second},
		Limiter:  rate.NewLimiter(rate.Every(200*time.Millisecond), 5),
		Exchange: "binance",
	}
}

// FetchLatestKlines returns up to limit closed candles for symbol (e.g. BTCUSDT) and interval (e.g. 1h).
func (r *REST) FetchLatestKlines(ctx context.Context, symbol, interval string, limit int) ([]store.CryptoBar, error) {
	if err := r.Limiter.Wait(ctx); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 100
	}
	q := url.Values{}
	q.Set("symbol", symbol)
	q.Set("interval", interval)
	q.Set("limit", strconv.Itoa(limit))
	u := fmt.Sprintf("%s/api/v3/klines?%s", restBase, q.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := r.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("binance klines: %s", resp.Status)
	}
	var raw [][]json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}
	out := make([]store.CryptoBar, 0, len(raw))
	for _, row := range raw {
		if len(row) < 6 {
			continue
		}
		var openMs int64
		if err := json.Unmarshal(row[0], &openMs); err != nil {
			continue
		}
		parse := func(i int) float64 {
			var s string
			_ = json.Unmarshal(row[i], &s)
			f, _ := strconv.ParseFloat(s, 64)
			return f
		}
		out = append(out, store.CryptoBar{
			TS:       time.UnixMilli(openMs).UTC(),
			Exchange: r.Exchange,
			Symbol:   symbol,
			Interval: interval,
			Open:     parse(1),
			High:     parse(2),
			Low:      parse(3),
			Close:    parse(4),
			Volume:   parse(5),
			Source:   "binance_rest",
		})
	}
	return out, nil
}
