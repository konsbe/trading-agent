package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/konsbe/trading-agent/services/data-analyzer/internal/momentum"
)

// §8.2 responsibilities 1 and 3: persist computed features and scores.
//
// These were the missing link in the live path. Everything built before this
// computed in memory and reported — momentum-dryrun, momentum-backtest and the
// replay are all read-only — so momentum_features and momentum_scores stayed
// empty while analyst-bot queried them. Unit tests on both sides passed; the
// composition had no data flowing through it.

const upsertFeaturesSQL = `
INSERT INTO momentum_features (
    ts, symbol, close, volume, prior_close, change_pct, gap_pct, dollar_volume,
    atr_14, atr_pct, avg_vol_20, rvol_20, vol_accel,
    resistance_20, range_20, was_consolidating, breakout_state,
    high_52w, pct_of_52w_high, vwap_20, above_vwap, vwap_dist_pct,
    float_shares_est, float_is_proxy, rsi_14,
    catalyst_tier, change_pct_5d,
    market_cap, market_cap_est, market_cap_is_proxy,
    bucket, gates_passed, gate_failures, computed_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8,
    $9, $10, $11, $12, $13,
    $14, $15, $16, $17,
    $18, $19, $20, $21, $22,
    $23, $24, $25,
    $26, $27,
    $28, $29, $30,
    $31, $32, $33, now()
)
ON CONFLICT (symbol, ts) DO UPDATE SET
    close = EXCLUDED.close, volume = EXCLUDED.volume,
    prior_close = EXCLUDED.prior_close, change_pct = EXCLUDED.change_pct,
    gap_pct = EXCLUDED.gap_pct, dollar_volume = EXCLUDED.dollar_volume,
    atr_14 = EXCLUDED.atr_14, atr_pct = EXCLUDED.atr_pct,
    avg_vol_20 = EXCLUDED.avg_vol_20, rvol_20 = EXCLUDED.rvol_20,
    vol_accel = EXCLUDED.vol_accel, resistance_20 = EXCLUDED.resistance_20,
    range_20 = EXCLUDED.range_20, was_consolidating = EXCLUDED.was_consolidating,
    breakout_state = EXCLUDED.breakout_state, high_52w = EXCLUDED.high_52w,
    pct_of_52w_high = EXCLUDED.pct_of_52w_high, vwap_20 = EXCLUDED.vwap_20,
    above_vwap = EXCLUDED.above_vwap, vwap_dist_pct = EXCLUDED.vwap_dist_pct,
    float_shares_est = EXCLUDED.float_shares_est, float_is_proxy = EXCLUDED.float_is_proxy,
    rsi_14 = EXCLUDED.rsi_14, catalyst_tier = EXCLUDED.catalyst_tier,
    change_pct_5d = EXCLUDED.change_pct_5d,
    market_cap = EXCLUDED.market_cap, market_cap_est = EXCLUDED.market_cap_est,
    market_cap_is_proxy = EXCLUDED.market_cap_is_proxy,
    bucket = EXCLUDED.bucket, gates_passed = EXCLUDED.gates_passed,
    gate_failures = EXCLUDED.gate_failures, computed_at = now()`

// FeatureRow bundles everything one momentum_features row needs.
type FeatureRow struct {
	TS     time.Time
	Symbol string

	Features *momentum.Features
	Gate     momentum.GateResult

	// FloatSharesEst is §3.9's proxy, always flagged as such in Phase 1.
	FloatSharesEst *float64

	// MarketCap is the value the GATE was evaluated against. MarketCapEst is the
	// §3.9 estimate when one was used. Stored separately so a row records both
	// what was known and what was assumed.
	MarketCap    *float64
	MarketCapEst *float64

	CatalystTier *string
}

// UpsertFeatures persists one §3 feature row.
//
// Gate results are stored alongside the features rather than in a separate
// table: §3.2's failures are only meaningful next to the values that caused
// them, and /score's "why is this not a candidate" answer needs both in one
// read.
func UpsertFeatures(ctx context.Context, pool Execer, r FeatureRow) error {
	f := r.Features
	if f == nil {
		return fmt.Errorf("upsert features %s: nil features", r.Symbol)
	}
	var breakout *string
	if f.BreakoutState != nil {
		s := string(*f.BreakoutState)
		breakout = &s
	}
	var bucket *string
	if r.Gate.Bucketed {
		s := string(r.Gate.Bucket)
		bucket = &s
	}
	// gate_failures is text[], not text. Passing the joined string produced
	// "malformed array literal" on all 450 rows — a failure only reachable by
	// actually writing to the table, which is precisely the class of bug this
	// persistence step existed to expose.
	//
	// Empty slice, not nil: the column is NOT NULL, and that constraint settles a
	// question I initially got backwards. NULL would have meant "not evaluated",
	// but every row written here HAS been evaluated by construction — so '{}'
	// unambiguously means "evaluated, passed everything", and there is no third
	// state to preserve. The passing row was the only one that failed to insert,
	// which is a fitting way to find out.
	failures := r.Gate.Failures
	if failures == nil {
		failures = []string{}
	}

	_, err := pool.Exec(ctx, upsertFeaturesSQL,
		r.TS, r.Symbol, f.Close, f.Volume, f.PriorClose, f.ChangePct, f.GapPct, f.DollarVolume,
		f.ATR14, f.ATRPct, f.AvgVol20, f.RVol20, f.VolAccel,
		f.Resistance20, f.Range20, f.WasConsolidating, breakout,
		f.High52w, f.PctOf52wHigh, f.VWAP20, f.AboveVWAP, f.VWAPDistPct,
		r.FloatSharesEst, r.FloatSharesEst != nil, f.RSI14,
		r.CatalystTier, f.ChangePct5d,
		r.MarketCap, r.MarketCapEst, r.Gate.MarketCapWasProxy,
		bucket, r.Gate.Passed, failures,
	)
	if err != nil {
		return fmt.Errorf("upsert features %s: %w", r.Symbol, err)
	}
	return nil
}

const upsertScoreSQL = `
INSERT INTO momentum_scores (
    ts, symbol, bucket, momentum_score_100,
    score_vol_accel, score_rvol, score_breakout, score_catalyst,
    score_float, score_vwap, score_52w,
    subtotal_before_penalties, penalty_total, penalties, null_inputs, scored_at
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15, now())
ON CONFLICT (symbol, ts) DO UPDATE SET
    bucket = EXCLUDED.bucket,
    momentum_score_100 = EXCLUDED.momentum_score_100,
    score_vol_accel = EXCLUDED.score_vol_accel, score_rvol = EXCLUDED.score_rvol,
    score_breakout = EXCLUDED.score_breakout, score_catalyst = EXCLUDED.score_catalyst,
    score_float = EXCLUDED.score_float, score_vwap = EXCLUDED.score_vwap,
    score_52w = EXCLUDED.score_52w,
    subtotal_before_penalties = EXCLUDED.subtotal_before_penalties,
    penalty_total = EXCLUDED.penalty_total,
    penalties = EXCLUDED.penalties, null_inputs = EXCLUDED.null_inputs,
    scored_at = now()`

// UpsertScore persists one §4 score with its full breakdown.
//
// §4.4 requires every sub-score separately, and §4.1 v2 is the proof of why: the
// per-component decomposition that identified `breakout` and `high52w` as
// inverted was only computable because each was stored on its own. The zeroed
// components are still written for the same reason — their relationship to
// outcomes still needs measuring at larger scale.
func UpsertScore(ctx context.Context, pool Execer, ts time.Time, symbol string, s momentum.Score) error {
	// penalties is jsonb and null_inputs is text[] — two different shapes for
	// two similar-looking fields. Marshalled explicitly rather than joined.
	penalties := s.Penalties
	if penalties == nil {
		penalties = []string{}
	}
	penaltyJSON, err := json.Marshal(penalties)
	if err != nil {
		return fmt.Errorf("marshal penalties %s: %w", symbol, err)
	}
	// Never a nil slice: pgx sends nil as SQL NULL, and null_inputs is NOT
	// NULL. A score with every input present would otherwise fail to write —
	// and, since the scan commits as one transaction, fail the whole scan.
	nulls := []string{}
	if len(s.NullInputs) > 0 {
		nulls = s.NullInputs
	}

	_, err = pool.Exec(ctx, upsertScoreSQL,
		ts, symbol, string(s.Bucket), s.Total,
		s.Sub.VolAccel, s.Sub.RVol, s.Sub.Breakout, s.Sub.Catalyst,
		s.Sub.Float, s.Sub.VWAP, s.Sub.High52w,
		s.Raw, s.PenaltyPoints, penaltyJSON, nulls,
	)
	if err != nil {
		return fmt.Errorf("upsert score %s: %w", symbol, err)
	}
	return nil
}
