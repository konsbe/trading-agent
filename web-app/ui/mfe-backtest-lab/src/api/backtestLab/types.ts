/**
 * `GET /api/v1/backtest-lab/report` — docs/MOMENTUM_SCANNER_API.md, Backtest Lab
 * addendum §2. A frozen, dated report served verbatim from
 * `shared/content/backtest_lab_report.json`; nothing in it is live or re-runnable.
 */

export interface ReportMeta {
    version: string;
    /** Always "closed" in the current build; a field so a future round has somewhere to say otherwise. */
    status: 'closed' | (string & {});
    /** `YYYY-MM-DD`. */
    closed_date: string;
    headline: string;
    screener_note: string;
}

export interface EntryGateSample {
    candidates: number;
    episodes: number;
    /** Percent (9.90 = 9.90%). */
    base_rate_pct: number;
    excludes: string;
}

export interface EntryGateResult {
    chi_square: number;
    p_value: number;
    mh_odds_ratio: number;
    required: string;
    verdict: string;
}

export interface EntryGate {
    title: string;
    /** The fact that makes the report trustworthy rather than post-hoc; foreground it. */
    rule_committed_before_result: boolean;
    test: string;
    sample: EntryGateSample;
    result: EntryGateResult;
    route_taken: string;
    note: string;
}

export interface V2ScoreFinding {
    title: string;
    sample: string;
    pooled_result: { p_value: number; verdict: string };
    stratified_result: { penny_bucket_p: number; market_bucket_p: number; verdict: string };
    /** Percent of the apparent separation explained by bucket composition. */
    composition_share_pct: number;
    explanation: string;
}

export interface FunnelStep {
    label: string;
    odds_ratio: number;
    /** `odds_ratio - 1`; absent on the final step. */
    excess_odds?: number;
    /** Percent of the crude effect explained by the stratification; absent on the crude and final steps. */
    composition_share_pct?: number;
    /** Only on the final step. */
    verdict?: string;
}

export interface StratificationFunnel {
    title: string;
    sample: string;
    steps: FunnelStep[];
}

export interface BestEffect {
    odds_ratio: number;
    /** 95% confidence interval `[lower, upper]`. */
    ci: [number, number];
}

export interface Hypothesis {
    id: string;
    label: string;
    best_effect: BestEffect;
    verdict: 'PASS' | 'FAIL' | (string & {});
    /** Only where a bare PASS/FAIL would misrepresent what happened (currently hypothesis b). */
    verdict_note?: string;
    reason: string;
}

/** Dropped before the round ran; has no result. */
export interface AbandonedHypothesis {
    id: string;
    label: string;
    reason: string;
}

export interface ResearchRound {
    title: string;
    hypotheses: Hypothesis[];
    abandoned: AbandonedHypothesis[];
}

export interface SampleSize {
    /** The section this sample belongs to (e.g. "research_round_1"). */
    applies_to: string;
    episodes: number;
    excludes: string;
    lockbox_opened: boolean;
}

export interface BacktestLabReport {
    report: ReportMeta;
    entry_gate: EntryGate;
    v2_score_finding: V2ScoreFinding;
    rvol_stratification_funnel: StratificationFunnel;
    research_round_1: ResearchRound;
    sample_size: SampleSize;
    closing_statement: string;
}
