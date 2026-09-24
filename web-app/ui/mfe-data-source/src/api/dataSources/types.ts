/**
 * `GET /api/v1/data-sources/status` — docs/MOMENTUM_SCANNER_API.md, Data Source
 * addendum §2.3 (what is served; §2.1's example is outdated) and
 * services/data-analyzer/internal/momentumapi/data_sources.go.
 *
 * Live operational state, computed when asked (server-cached ≤60 s): `checked_at`
 * is the query time, so a cached answer shows its real age.
 */

/** A section whose query failed; the other section is still served. */
export const SECTION_UNAVAILABLE = 'unavailable';
export type SectionUnavailable = typeof SECTION_UNAVAILABLE;

export const isSectionUnavailable = <T,>(section: T | SectionUnavailable): section is SectionUnavailable =>
    section === SECTION_UNAVAILABLE;

export type OverallHealth = 'healthy' | 'attention';

export const OVERALL_HEALTH: readonly OverallHealth[] = ['healthy', 'attention'];

/** The providers the server reports, in its display order (JSON object keys arrive sorted). */
export const PROVIDER_DISPLAY_ORDER = ['tiingo', 'finnhub'] as const;

export type ProviderKey = (typeof PROVIDER_DISPLAY_ORDER)[number] | (string & {});

export interface ProviderStatus {
    role: string;
    /** Today's count in the provider's own reset window (0 when the stored window is not today's). */
    daily_used: number;
    /** `YYYY-MM-DD` in `daily_reset_tz`. */
    daily_window_start: string | null;
    /** e.g. "EST" (Tiingo), "UTC" (Finnhub). */
    daily_reset_tz: string;
    /** Null when the provider has no daily cap (Finnhub). */
    daily_limit: number | null;
    /** Percent of `daily_limit`, 1 decimal; null exactly when `daily_limit` is. */
    daily_used_pct: number | null;
    /** The enforced request rate. */
    rate_per_sec: number;
    /** `rate_per_sec` × 86,400 — capacity, never a quota and never a denominator. */
    theoretical_daily_capacity: number;
    /** Always null today: degradations are counted in process memory, never persisted. */
    degraded_count_24h: number | null;
}

/** Keyed by provider; a provider with no budget row is missing (and named in `overall_reasons`). */
export type ProvidersStatus = Partial<Record<ProviderKey, ProviderStatus>>;

export type SessionRunStatus = 'clean' | 'completed_after_retry' | 'pending' | 'failed' | 'not_run' | 'not_recorded';

export const SESSION_RUN_STATUSES: readonly SessionRunStatus[] = [
    'clean',
    'completed_after_retry',
    'pending',
    'failed',
    'not_run',
    'not_recorded',
];

export interface SessionStatus {
    /** NYSE session, `YYYY-MM-DD`. */
    session: string;
    /** Computed now with momentum-daily's definition; can differ from the coverage that gated the run. */
    bars_coverage_now_pct: number | null;
    attempts: number;
    scanner_completed: boolean;
    tracker_completed: boolean;
    /** `last_error`, set only when the run gave up. */
    gave_up_reason: string | null;
    last_error: string | null;
    status: SessionRunStatus;
    /** Explains a status that needs one (not_run, not_recorded, failed without a recorded give-up). */
    note?: string;
}

export interface DailyChainStatus {
    last_clean_session: string | null;
    /** The last `sessions_shown` closed NYSE sessions, newest first. */
    sessions: SessionStatus[];
    sessions_shown: number;
}

export interface DataSourceStatus {
    /** RFC 3339, UTC. */
    checked_at: string;
    providers: ProvidersStatus | SectionUnavailable;
    daily_chain: DailyChainStatus | SectionUnavailable;
    /** `attention` whenever `overall_reasons` is non-empty (including an unavailable section). */
    overall: OverallHealth;
    overall_reasons: string[];
}
