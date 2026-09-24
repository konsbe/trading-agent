package store

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// Read path for momentum-api (docs/MOMENTUM_SCANNER_API.md). Read-only: nothing
// here issues INSERT or UPDATE, and nothing recomputes a score — every value is
// what momentum-scanner already persisted. The read logic mirrors
// services/analyst-bot/db/queries/momentum.py, so the web app and Discord agree
// on what "today's candidates" means.

// Querier is the subset of pgxpool.Pool / pgx.Tx the reads need. Tests run the
// same queries inside a rolled-back transaction so fixtures never touch the
// live scan that the bot and scanner read.
type Querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// LatestScanDate is the most recent momentum_features.ts, or ok=false when no
// scan has ever been persisted.
//
// Read from momentum_features, not momentum_scores as the bot's /scanner does:
// scores exist only for gate-passers, so a day with zero candidates has no
// score rows and would silently fall back to an older scan.
func LatestScanDate(ctx context.Context, q Querier) (date time.Time, ok bool, err error) {
	var ts *time.Time
	if err := q.QueryRow(ctx, `SELECT max(ts) FROM momentum_features`).Scan(&ts); err != nil {
		return time.Time{}, false, fmt.Errorf("latest scan date: %w", err)
	}
	if ts == nil {
		return time.Time{}, false, nil
	}
	return ts.UTC(), true, nil
}

// ScanSummary describes one persisted scan.
type ScanSummary struct {
	Date time.Time

	// CompletedAt is when the scan's last row was written: the later of
	// max(computed_at) on its features and max(scored_at) on its scores. Both
	// are reset to now() on every upsert, so a re-run moves it forward. It is
	// NOT a run-completion marker — no such record exists — so a run that
	// crashed midway still reports the time of its last write.
	CompletedAt time.Time

	UniverseScanned  int
	UniverseEligible int
}

func GetScanSummary(ctx context.Context, q Querier, date time.Time) (ScanSummary, error) {
	var featuresAt, scoresAt *time.Time
	s := ScanSummary{Date: date}
	err := q.QueryRow(ctx, `
SELECT
    (SELECT max(computed_at) FROM momentum_features WHERE ts = $1),
    (SELECT max(scored_at)   FROM momentum_scores   WHERE ts = $1),
    (SELECT count(DISTINCT symbol) FROM momentum_features WHERE ts = $1),
    -- §3.1 eligibility, the canonical denominator. Symbols the provider no
    -- longer serves stay eligible; the scanner skipping them shows up as
    -- universe_scanned < universe_eligible, not as a smaller denominator.
    (SELECT count(*) FROM universe_symbols WHERE is_eligible)`,
		date).Scan(&featuresAt, &scoresAt, &s.UniverseScanned, &s.UniverseEligible)
	if err != nil {
		return ScanSummary{}, fmt.Errorf("scan summary %s: %w", date.Format(time.DateOnly), err)
	}
	if featuresAt != nil {
		s.CompletedAt = featuresAt.UTC()
	}
	if scoresAt != nil && scoresAt.After(s.CompletedAt) {
		s.CompletedAt = scoresAt.UTC()
	}
	return s, nil
}

// CandidateRow is one gate-passing symbol for the list view.
type CandidateRow struct {
	Symbol      string
	Exchange    *string
	CompanyName *string
	Bucket      string

	// Close is the features row's own close: the price as traded on the scan
	// day. equity_ohlcv.close is back-adjusted by later dividends and splits,
	// so joining it would serve a price that drifts after the fact.
	Close         *float64
	ChangePct     *float64
	RVol20        *float64
	DollarVolume  *float64
	RSI14         *float64
	BreakoutState *string
	PctOf52wHigh  *float64
	CatalystTier  *string
	MomentumScore *int

	// ScoreNullInputs is the score row's null_inputs, read so the list can show
	// each score against its own attainable ceiling. Nil when there is no score.
	ScoreNullInputs []string
}

// Candidates returns every gate-passing row for the scan date, both buckets,
// ordered by bucket then rvol_20 DESC (the list's default display order).
//
// No LIMIT: the frontend re-sorts the full set client-side. LEFT JOIN on
// scores so a candidate without a score row is still returned with a null
// score, and LEFT JOIN on universe_symbols for the same reason — a missing
// directory row must not silently drop a real candidate.
func Candidates(ctx context.Context, q Querier, date time.Time) ([]CandidateRow, error) {
	rows, err := q.Query(ctx, `
SELECT mf.symbol, u.exchange, u.name, mf.bucket,
       mf.close, mf.change_pct, mf.rvol_20, mf.dollar_volume, mf.rsi_14,
       mf.breakout_state, mf.pct_of_52w_high, mf.catalyst_tier,
       ms.momentum_score_100, ms.null_inputs
FROM momentum_features mf
LEFT JOIN universe_symbols u ON u.symbol = mf.symbol
LEFT JOIN momentum_scores ms ON ms.symbol = mf.symbol AND ms.ts = mf.ts
WHERE mf.ts = $1 AND mf.gates_passed
ORDER BY mf.bucket, mf.rvol_20 DESC NULLS LAST, mf.symbol`, date)
	if err != nil {
		return nil, fmt.Errorf("candidates %s: %w", date.Format(time.DateOnly), err)
	}
	defer rows.Close()

	var out []CandidateRow
	for rows.Next() {
		var c CandidateRow
		var bucket *string
		if err := rows.Scan(&c.Symbol, &c.Exchange, &c.CompanyName, &bucket,
			&c.Close, &c.ChangePct, &c.RVol20, &c.DollarVolume, &c.RSI14,
			&c.BreakoutState, &c.PctOf52wHigh, &c.CatalystTier,
			&c.MomentumScore, &c.ScoreNullInputs); err != nil {
			return nil, fmt.Errorf("scan candidate: %w", err)
		}
		if bucket == nil {
			// A gate pass always assigns a bucket; a null here is a writer bug,
			// and serving the row without one would hide it.
			return nil, fmt.Errorf("candidate %s passed the gates with no bucket", c.Symbol)
		}
		c.Bucket = *bucket
		out = append(out, c)
	}
	return out, rows.Err()
}

// ScoreRow is the persisted §4.4 breakdown for one symbol on one day.
type ScoreRow struct {
	Total        int
	RVol         *float64
	VolAccel     *float64
	Catalyst     *float64
	Float        *float64
	VWAP         *float64
	Breakout     *float64
	High52w      *float64
	Penalties    []string
	PenaltyTotal *float64
	NullInputs   []string
}

// DetailFacts are the stored §3 features the detail view shows. Every value is
// what momentum-scanner persisted; nothing is recomputed.
type DetailFacts struct {
	Close            *float64
	PriorClose       *float64
	ChangePct        *float64
	GapPct           *float64
	Volume           *float64
	AvgVol20         *float64
	DollarVolume     *float64
	RVol20           *float64
	VolAccel         *float64
	ATRPct           *float64
	RSI14            *float64
	High52w          *float64
	PctOf52wHigh     *float64
	Resistance20     *float64
	BreakoutState    *string
	WasConsolidating *bool
	VWAP20           *float64
	AboveVWAP        *bool
	VWAPDistPct      *float64
	FloatSharesEst   *float64
	FloatIsProxy     bool
	MarketCap        *float64
	MarketCapEst     *float64
	MarketCapIsProxy bool
	CatalystTier     *string
	CatalystHeadline *string
	ComputedAt       time.Time
}

// SymbolDetailRow is one symbol's features row for the scan date, with its
// score when it has one.
type SymbolDetailRow struct {
	Symbol       string
	Exchange     *string
	CompanyName  *string
	Bucket       *string
	TS           time.Time
	GatesPassed  bool
	GateFailures []string
	Facts        DetailFacts
	Score        *ScoreRow
}

// SymbolDetail returns the symbol's row for the scan date, or ok=false when it
// has none. Gate-failed rows are returned too: "why is this not on the list"
// is the question the detail view answers.
func SymbolDetail(ctx context.Context, q Querier, date time.Time, symbol string) (SymbolDetailRow, bool, error) {
	var d SymbolDetailRow
	f := &d.Facts
	var total *int
	var penalties []byte
	var sc ScoreRow
	err := q.QueryRow(ctx, `
SELECT mf.symbol, u.exchange, u.name, mf.bucket, mf.ts, mf.gates_passed, mf.gate_failures,
       mf.close, mf.prior_close, mf.change_pct, mf.gap_pct, mf.volume, mf.avg_vol_20,
       mf.dollar_volume, mf.rvol_20, mf.vol_accel, mf.atr_pct, mf.rsi_14,
       mf.high_52w, mf.pct_of_52w_high, mf.resistance_20, mf.breakout_state, mf.was_consolidating,
       mf.vwap_20, mf.above_vwap, mf.vwap_dist_pct, mf.float_shares_est, mf.float_is_proxy,
       mf.market_cap, mf.market_cap_est, mf.market_cap_is_proxy,
       mf.catalyst_tier, mf.catalyst_headline, mf.computed_at,
       ms.momentum_score_100,
       ms.score_rvol, ms.score_vol_accel, ms.score_catalyst, ms.score_float,
       ms.score_vwap, ms.score_breakout, ms.score_52w,
       ms.penalties, ms.penalty_total, ms.null_inputs
FROM momentum_features mf
LEFT JOIN universe_symbols u ON u.symbol = mf.symbol
LEFT JOIN momentum_scores ms ON ms.symbol = mf.symbol AND ms.ts = mf.ts
WHERE mf.ts = $1 AND mf.symbol = upper($2)`, date, symbol).Scan(
		&d.Symbol, &d.Exchange, &d.CompanyName, &d.Bucket, &d.TS, &d.GatesPassed, &d.GateFailures,
		&f.Close, &f.PriorClose, &f.ChangePct, &f.GapPct, &f.Volume, &f.AvgVol20,
		&f.DollarVolume, &f.RVol20, &f.VolAccel, &f.ATRPct, &f.RSI14,
		&f.High52w, &f.PctOf52wHigh, &f.Resistance20, &f.BreakoutState, &f.WasConsolidating,
		&f.VWAP20, &f.AboveVWAP, &f.VWAPDistPct, &f.FloatSharesEst, &f.FloatIsProxy,
		&f.MarketCap, &f.MarketCapEst, &f.MarketCapIsProxy,
		&f.CatalystTier, &f.CatalystHeadline, &f.ComputedAt,
		&total,
		&sc.RVol, &sc.VolAccel, &sc.Catalyst, &sc.Float,
		&sc.VWAP, &sc.Breakout, &sc.High52w,
		&penalties, &sc.PenaltyTotal, &sc.NullInputs,
	)
	if err == pgx.ErrNoRows {
		return SymbolDetailRow{}, false, nil
	}
	if err != nil {
		return SymbolDetailRow{}, false, fmt.Errorf("symbol detail %s: %w", symbol, err)
	}
	d.TS = d.TS.UTC()
	f.ComputedAt = f.ComputedAt.UTC()
	if total != nil {
		sc.Total = *total
		p, err := parsePenalties(penalties)
		if err != nil {
			return SymbolDetailRow{}, false, fmt.Errorf("symbol detail %s: %w", symbol, err)
		}
		sc.Penalties = p
		d.Score = &sc
	}
	return d, true, nil
}

// parsePenalties reads momentum_scores.penalties: a jsonb array of reason names,
// as UpsertScore writes it (the points are in penalty_total). The column's
// DEFAULT is the empty object '{}', so that one value also means "no
// penalties"; any other shape is an error rather than a guess.
func parsePenalties(raw []byte) ([]string, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) || bytes.Equal(trimmed, []byte("{}")) {
		return nil, nil
	}
	var out []string
	if err := json.Unmarshal(trimmed, &out); err != nil {
		return nil, fmt.Errorf("penalties %s: want a JSON array of reason names: %w", trimmed, err)
	}
	return out, nil
}
