package marketops

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/konsbe/trading-agent/services/data-analyzer/internal/config"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/store"
)

const (
	// MoMetric is stored in macro_derived with source market_operations.
	MoMetric = "mo_reference_snapshot"
	// MoSource tags rows written by this worker.
	MoSource = "market_operations"
)

// BuildSnapshot returns the JSON payload and optional scalar (VIX) for macro_derived.
func BuildSnapshot(ctx context.Context, pool *pgxpool.Pool, cfg config.MarketOperations) (map[string]any, *float64) {
	now := time.Now().UTC()
	out := map[string]any{
		"reference":         "services/data-analyzer/cmd/market-operations/market_operations_reference.html",
		"as_of":             now.Format(time.RFC3339),
		"reference_modules": ReferenceModules(),
		"disclaimer":        "Context only — not buy/sell advice. Per-symbol execution stats are computed in analyst-bot.",
	}

	vix, ok, _ := store.QueryLatestFREDValue(ctx, pool, "VIXCLS")
	regime, label := classifyVIX(vix, ok, cfg)

	g := map[string]any{
		"vix_regime": regime,
		"vix_label":  label,
	}
	if ok {
		g["vix"] = vix
	}
	out["global"] = g

	var val *float64
	if ok {
		val = &vix
	}
	return out, val
}

func classifyVIX(vix float64, ok bool, cfg config.MarketOperations) (regime, label string) {
	if !ok {
		return "unknown", "VIXCLS not available in macro_fred — run data-equity FRED ingestion."
	}
	switch {
	case vix < cfg.VIXLowMax:
		return "low", fmt.Sprintf("VIX %.1f — complacency risk; trend strategies may underperform on vol spikes.", vix)
	case vix < cfg.VIXNormalMax:
		return "normal", fmt.Sprintf("VIX %.1f — typical range; size and stops using baseline rules.", vix)
	case vix < cfg.VIXElevatedMax:
		return "elevated", fmt.Sprintf("VIX %.1f — elevated fear; wider noise, reduce size or widen risk awareness.", vix)
	default:
		return "stress", fmt.Sprintf("VIX %.1f — stress regime; prioritize survival, liquidity, and gap risk.", vix)
	}
}
