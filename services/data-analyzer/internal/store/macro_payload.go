package store

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// QueryMacroPayloadString returns payload[key] as string when present (latest row per metric).
func QueryMacroPayloadString(ctx context.Context, pool *pgxpool.Pool, metric, key string) (string, bool) {
	m, ok := QueryMacroDerivedPayloadMap(ctx, pool, metric)
	if !ok {
		return "", false
	}
	v, ok := m[key]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok && s != ""
}
