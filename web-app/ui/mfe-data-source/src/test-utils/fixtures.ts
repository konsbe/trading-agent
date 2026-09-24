import liveStatus from './data-sources-status.live.json';
import { DataSourceStatus, SessionStatus } from '@/api';

/** Captured from `GET http://localhost:8090/api/v1/data-sources/status` on 2026-09-24 (healthy; finnhub has no daily limit). */
export const LIVE_STATUS_JSON: unknown = liveStatus;

/** A fresh, mutable deep copy of the live body. */
export const makeStatusBody = (): any => JSON.parse(JSON.stringify(liveStatus));

export const makeStatus = (): DataSourceStatus => makeStatusBody() as DataSourceStatus;

const CLEAN_SESSION: SessionStatus = {
    session: '2026-09-24',
    bars_coverage_now_pct: 99.8,
    attempts: 1,
    scanner_completed: true,
    tracker_completed: true,
    gave_up_reason: null,
    last_error: null,
    status: 'clean',
};

/** One session row per status, shaped like the server builds them (data_sources.go sessionStatusOf). */
export const SESSION_ROWS: Record<SessionStatus['status'], SessionStatus> = {
    clean: CLEAN_SESSION,
    completed_after_retry: { ...CLEAN_SESSION, attempts: 3, last_error: 'bars coverage 91.2% < 97%' , status: 'completed_after_retry' },
    pending: {
        ...CLEAN_SESSION,
        bars_coverage_now_pct: 12.4,
        attempts: 1,
        scanner_completed: false,
        tracker_completed: false,
        last_error: 'bars coverage 12.4% < 97%',
        status: 'pending',
    },
    failed: {
        ...CLEAN_SESSION,
        attempts: 6,
        scanner_completed: false,
        tracker_completed: false,
        gave_up_reason: 'bars coverage 88.0% < 97%',
        last_error: 'bars coverage 88.0% < 97%',
        status: 'failed',
    },
    not_run: {
        ...CLEAN_SESSION,
        bars_coverage_now_pct: null,
        attempts: 0,
        scanner_completed: false,
        tracker_completed: false,
        status: 'not_run',
        note: 'no attempt was recorded — momentum-daily was not running, or the machine was off',
    },
    not_recorded: {
        ...CLEAN_SESSION,
        attempts: 0,
        scanner_completed: false,
        tracker_completed: false,
        status: 'not_recorded',
        note: 'before the chain recorded its runs (momentum_chain_runs)',
    },
};

/** Live body whose chain lists one row of every status, and `attention` with its reasons. */
export const makeEveryStatusBody = (): any => {
    const body = makeStatusBody();
    body.daily_chain.sessions = Object.values(SESSION_ROWS).map((row, i) => ({ ...row, session: `2026-09-${String(24 - i).padStart(2, '0')}` }));
    body.overall = 'attention';
    body.overall_reasons = ['latest finished session 2026-09-23 is completed_after_retry'];
    return body;
};

/** Minimal `fetch` Response stand-in (jsdom has no Response). */
export const mockResponse = (status: number, body: unknown, { raw = false } = {}): Response =>
    ({
        ok: status >= 200 && status < 300,
        status,
        text: () => Promise.resolve(raw ? String(body) : body === undefined ? '' : JSON.stringify(body)),
    }) as unknown as Response;
