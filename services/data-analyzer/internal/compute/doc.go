// Package compute contains all technical-indicator math for the data-analyzer service.
//
// Current implementation: pure Go, hand-rolled indicator formulas operating on
// []Bar slices read from TimescaleDB.
//
// TODO: migrate this entire package to Python (data-analyzer service or a future
// analyst-bot service). Python's ecosystem (pandas-ta, ta-lib, numpy, scipy) is
// significantly richer for financial computation and will be required when the
// model-training layer consumes these indicators. When migrating:
//
//   1. Replace each .go file with an equivalent pandas-ta / ta-lib call in Python.
//   2. Keep the same indicator naming convention (e.g. "rsi_14", "macd_12_26_9")
//      so that the technical_indicators table schema is unchanged.
//   3. The store layer (UpsertIndicator) maps 1-to-1 to a psycopg3 / asyncpg upsert.
//   4. Remove this package and the cmd/technical-analysis Go binary once Python
//      parity is confirmed in production.
package compute
