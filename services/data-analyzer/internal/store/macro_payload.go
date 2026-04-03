package store

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5/pgxpool"
)

// QueryMacroPayloadString returns payload[key] as string when present (latest row per metric).
func QueryMacroPayloadString(ctx context.Context, pool *pgxpool.Pool, metric, key string) (string, bool) {
	_, raw, ok := QueryMacroDerivedLatest(ctx, pool, metric)
	if !ok || len(raw) == 0 {
		return "", false
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return "", false
	}
	v, ok := m[key]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok && s != ""
}
