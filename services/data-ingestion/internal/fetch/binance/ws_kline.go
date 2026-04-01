package binance

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/konsbe/trading-agent/services/data-ingestion/internal/store"
)

// StreamKlines opens a Binance combined stream for kline_1h style streams and emits closed bars on ch.
// Caller should drain ch; function returns when ctx is done or connection ends.
func StreamKlines(ctx context.Context, symbols []string, interval string, ch chan<- store.CryptoBar) error {
	if len(symbols) == 0 {
		return fmt.Errorf("no symbols")
	}
	streams := make([]string, 0, len(symbols))
	for _, s := range symbols {
		s = strings.ToLower(strings.TrimSpace(s))
		if s == "" {
			continue
		}
		streams = append(streams, fmt.Sprintf("%s@kline_%s", s, interval))
	}
	if len(streams) == 0 {
		return fmt.Errorf("no valid streams")
	}
	path := strings.Join(streams, "/")
	u := fmt.Sprintf("wss://stream.binance.com:9443/stream?streams=%s", path)
	d := websocket.Dialer{HandshakeTimeout: 15 * time.Second}
	conn, _, err := d.DialContext(ctx, u, nil)
	if err != nil {
		return err
	}
	defer conn.Close()

	go func() {
		<-ctx.Done()
		_ = conn.Close()
	}()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		_, msg, err := conn.ReadMessage()
		if err != nil {
			return err
		}
		var wrap struct {
			Stream string          `json:"stream"`
			Data   json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(msg, &wrap); err != nil {
			continue
		}
		var payload struct {
			K struct {
				T int64  `json:"t"` // start
				S string `json:"s"`
				I string `json:"i"`
				O string `json:"o"`
				H string `json:"h"`
				L string `json:"l"`
				C string `json:"c"`
				V string `json:"v"`
				X bool   `json:"x"` // is closed
			} `json:"k"`
		}
		if err := json.Unmarshal(wrap.Data, &payload); err != nil {
			continue
		}
		if !payload.K.X {
			continue
		}
		parse := func(s string) float64 {
			f, _ := strconv.ParseFloat(s, 64)
			return f
		}
		bar := store.CryptoBar{
			TS:       time.UnixMilli(payload.K.T).UTC(),
			Exchange: "binance",
			Symbol:   payload.K.S,
			Interval: payload.K.I,
			Open:     parse(payload.K.O),
			High:     parse(payload.K.H),
			Low:      parse(payload.K.L),
			Close:    parse(payload.K.C),
			Volume:   parse(payload.K.V),
			Source:   "binance_ws",
		}
		select {
		case ch <- bar:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
