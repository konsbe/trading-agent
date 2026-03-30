// Package ccxt documents how to plug multi-exchange OHLCV without locking the monolith to one vendor.
//
// The upstream CCXT Go module (github.com/ccxt/ccxt/go/v4) pulls a very large dependency graph.
// This repo uses Binance REST/WebSocket for low-latency crypto candles and leaves a clean seam here:
//
//   - Implement internal/fetch/ccxt.Exchange with FetchOHLCV(ctx, symbol, timeframe string) ([]store.CryptoBar, error)
//   - Optionally add a thin wrapper around github.com/ccxt/ccxt/go/v4 in ccxt_live.go behind build tag ccxt
//
// Python-side Qlib pipelines can still consume the same Timescale tables regardless of ingest language.
package ccxt
