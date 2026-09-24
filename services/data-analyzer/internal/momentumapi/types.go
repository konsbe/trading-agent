package momentumapi

import (
	"math"
	"time"
)

const timeRFC3339 = time.RFC3339

// Response shapes for docs/MOMENTUM_SCANNER_API.md §2. Values are passed
// through as stored: no display formatting, no rounding.

type todayResponse struct {
	Scan    scanInfo          `json:"scan"`
	Buckets map[string]bucket `json:"buckets"`
}

type scanInfo struct {
	Date             string `json:"date"`
	CompletedAt      string `json:"completed_at"`
	UniverseScanned  int    `json:"universe_scanned"`
	UniverseEligible int    `json:"universe_eligible"`
	IsStale          bool   `json:"is_stale"`
}

type bucket struct {
	TotalCandidates int         `json:"total_candidates"`
	Candidates      []candidate `json:"candidates"`
}

type candidate struct {
	Symbol        string   `json:"symbol"`
	Exchange      *string  `json:"exchange"`
	CompanyName   *string  `json:"company_name"`
	Bucket        string   `json:"bucket"`
	Close         *float64 `json:"close"`
	ChangePct     *float64 `json:"change_pct"`
	RVol20        *float64 `json:"rvol_20"`
	DollarVolume  *float64 `json:"dollar_volume"`
	RSI14         *float64 `json:"rsi_14"`
	BreakoutState *string  `json:"breakout_state"`
	PctOf52wHigh  *float64 `json:"pct_of_52w_high"`
	CatalystTier  *string  `json:"catalyst_tier"`
	// Same three fields and meaning as the detail view: MarketCap is reported,
	// MarketCapEst is the shares×close estimate the gate used when it was not.
	MarketCap        *float64 `json:"market_cap"`
	MarketCapEst     *float64 `json:"market_cap_est"`
	MarketCapIsProxy bool     `json:"market_cap_is_proxy"`
	MomentumScore100 *int     `json:"momentum_score_100"`
	// ScoreAttainable is the row's own ceiling (see momentum.Attainable), so a
	// bare score never reads as "out of 100". Null exactly when the score is.
	ScoreAttainable *int   `json:"score_attainable"`
	ScoreStatus     string `json:"score_status"`
}

type detailResponse struct {
	Symbol      string  `json:"symbol"`
	Exchange    *string `json:"exchange"`
	CompanyName *string `json:"company_name"`
	Bucket      *string `json:"bucket"`
	// AsOf is the date of the symbol's own most recent scanner row, which is
	// older than LatestScanDate for a symbol the latest scan did not cover.
	// IsStale is the candidates endpoint's scan.is_stale rule applied to AsOf.
	// IsCandidateToday: passed its gates in the LATEST scan — a symbol that was
	// a candidate on an older date is never presented as one today.
	AsOf             string       `json:"as_of"`
	IsStale          bool         `json:"is_stale"`
	LatestScanDate   string       `json:"latest_scan_date"`
	IsCandidateToday bool         `json:"is_candidate_today"`
	GatesPassed      bool         `json:"gates_passed"`
	GateFailures     []string     `json:"gate_failures"`
	Gates            gatesDetail  `json:"gates"`
	Facts            factsDetail  `json:"facts"`
	EvidenceNote     string       `json:"evidence_note"`
	Score            *scoreDetail `json:"score"`
}

// gatesDetail explains §3.2 per gate. Thresholds are the scanner's gate
// configuration for the row's bucket; pass/fail comes from the stored
// gate_failures, not from re-evaluating the values here.
type gatesDetail struct {
	PassedCount int         `json:"passed_count"`
	Total       int         `json:"total"`
	Checks      []gateCheck `json:"checks"`
	// Unmapped lists stored failure codes no check claims, so none is hidden.
	Unmapped []string `json:"unmapped_failures,omitempty"`
}

type gateCheck struct {
	Key          string   `json:"key"`
	Label        string   `json:"label"`
	Passed       bool     `json:"passed"`
	Failures     []string `json:"failures"`
	Value        *float64 `json:"value"`
	ValueIsProxy bool     `json:"value_is_proxy,omitempty"`
	Min          *float64 `json:"min"`
	Max          *float64 `json:"max"`
}

// factsDetail is the stored feature row, as persisted by momentum-scanner.
type factsDetail struct {
	Close            *float64 `json:"close"`
	PriorClose       *float64 `json:"prior_close"`
	ChangePct        *float64 `json:"change_pct"`
	ChangeAbs        *float64 `json:"change_abs"`
	GapPct           *float64 `json:"gap_pct"`
	Volume           *float64 `json:"volume"`
	AvgVolume20      *float64 `json:"avg_volume_20"`
	DollarVolume     *float64 `json:"dollar_volume"`
	RVol20           *float64 `json:"rvol_20"`
	VolAccel         *float64 `json:"vol_accel"`
	ATRPct           *float64 `json:"atr_pct"`
	RSI14            *float64 `json:"rsi_14"`
	High52w          *float64 `json:"high_52w"`
	PctOf52wHigh     *float64 `json:"pct_of_52w_high"`
	Resistance20     *float64 `json:"resistance_20"`
	BreakoutState    *string  `json:"breakout_state"`
	WasConsolidating *bool    `json:"was_consolidating"`
	VWAP20           *float64 `json:"vwap_20"`
	AboveVWAP        *bool    `json:"above_vwap"`
	VWAPDistPct      *float64 `json:"vwap_dist_pct"`
	FloatSharesEst   *float64 `json:"float_shares_est"`
	FloatIsProxy     bool     `json:"float_is_proxy"`
	MarketCap        *float64 `json:"market_cap"`
	MarketCapEst     *float64 `json:"market_cap_est"`
	MarketCapIsProxy bool     `json:"market_cap_is_proxy"`
	CatalystTier     *string  `json:"catalyst_tier"`
	CatalystHeadline *string  `json:"catalyst_headline"`
	ComputedAt       string   `json:"computed_at"`
}

type scoreDetail struct {
	Total        int                 `json:"total"`
	Attainable   int                 `json:"attainable"`
	Allocated    int                 `json:"allocated"`
	Status       string              `json:"status"`
	ModelVersion string              `json:"model_version"`
	SubScores    map[string]*float64 `json:"sub_scores"`
	// Weights are each component's maximum points under ModelVersion.
	Weights      map[string]int `json:"weights"`
	Penalties    []string       `json:"penalties"`
	PenaltyRules []penaltyRule  `json:"penalty_rules"`
	PenaltyTotal *float64       `json:"penalty_total"`
	NullInputs   []string       `json:"null_inputs"`
	Caveat       string         `json:"caveat"`
}

type penaltyRule struct {
	Code    string `json:"code"`
	Points  int    `json:"points"`
	Applied bool   `json:"applied"`
}

type barsResponse struct {
	Symbol string `json:"symbol"`
	Range  string `json:"range"`
	// Interval is the resolution actually served ("5Min" or "1Day").
	Interval string `json:"interval"`
	// Fallback explains a coarser series than the range implies, e.g.
	// "no_intraday_data" when 1D/5D are served from daily bars.
	Fallback *string `json:"fallback"`
	// Adjusted is true for daily bars (split- and dividend-adjusted).
	Adjusted bool  `json:"adjusted"`
	Bars     []bar `json:"bars"`
}

type bar struct {
	Time   int64   `json:"time"` // unix seconds, UTC
	Open   float64 `json:"open"`
	High   float64 `json:"high"`
	Low    float64 `json:"low"`
	Close  float64 `json:"close"`
	Volume float64 `json:"volume"`
}

type watchlistResponse struct {
	// Owner is "unauthenticated" until auth exists.
	Owner string          `json:"owner"`
	Items []watchlistItem `json:"items"`
}

type watchlistItem struct {
	Symbol      string  `json:"symbol"`
	CompanyName *string `json:"company_name"`
	Exchange    *string `json:"exchange"`
	AddedAt     string  `json:"added_at"`

	// From the symbol's most recent momentum_features row (any row, not only
	// gate passes). AsOf is that row's date; IsStale uses the same rule as the
	// candidates endpoint's scan.is_stale, applied to AsOf, and is true when
	// there is no row at all. Values are null when there is no row.
	AsOf      *string  `json:"as_of"`
	IsStale   bool     `json:"is_stale"`
	Close     *float64 `json:"close"`
	ChangePct *float64 `json:"change_pct"`
	RVol20    *float64 `json:"rvol_20"`
}

type symbolSearchResponse struct {
	Query   string        `json:"query"`
	Results []symbolMatch `json:"results"`
}

type symbolMatch struct {
	Symbol      string  `json:"symbol"`
	CompanyName *string `json:"company_name"`
	Exchange    *string `json:"exchange"`
	// IsEligible is false for a symbol the scanner does not cover: it can be
	// watched, but it will have no price data.
	IsEligible bool `json:"is_eligible"`
}

type errorResponse struct {
	Error string `json:"error"`
}

// finite drops NaN and ±Inf, which Postgres double precision can hold but JSON
// cannot encode; they read as missing rather than failing the whole response.
func finite(v *float64) *float64 {
	if v == nil || math.IsNaN(*v) || math.IsInf(*v, 0) {
		return nil
	}
	return v
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
