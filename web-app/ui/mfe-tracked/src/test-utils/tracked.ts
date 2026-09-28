import { TrackedChain, TrackedResponse, TrackedRow } from '@/api';

export const LOST_VWAP_NOTE = 'Closed below its rolling 20-day VWAP.';
export const BREAKOUT_FAILED_NOTE =
    'Closed back below the 20-day resistance level it broke out over at alert time. Measured to fire on the first session in most cases (651 of 1,545 replayed exits, at a median peak of 0.00%): an overly sensitive rule, not a confirmed breakout failure.';

/** Evaluated through the latest scan, up 4%. */
export const makeActiveRow = (overrides: Partial<TrackedRow> = {}): TrackedRow => ({
    symbol: 'DNA',
    exchange: 'NYSE',
    company_name: 'GINKGO BIOWORKS HOLDINGS INC',
    bucket: 'market',
    status: 'active',
    alerted_date: '2026-09-24',
    sessions_elapsed: 1,
    last_evaluated_date: '2026-09-25',
    evaluation_behind: false,
    reference_price: 10.25,
    current_price: 10.66,
    current_price_date: '2026-09-25',
    unrealized_pct: 4.0000000000000036,
    max_gain_pct: 4.0000000000000036,
    exit_reason: null,
    exit_reason_note: null,
    exit_price: null,
    exit_pct: null,
    closed_date: null,
    ...overrides,
});

/** Alerted on the latest session and not evaluated yet (the API serves null sessions and null %). */
export const makeUnevaluatedRow = (overrides: Partial<TrackedRow> = {}): TrackedRow =>
    makeActiveRow({
        symbol: 'ABLV',
        exchange: 'NASDAQ',
        company_name: 'ABLE VIEW GLOBAL INC-B',
        bucket: 'penny',
        alerted_date: '2026-09-25',
        sessions_elapsed: null,
        last_evaluated_date: null,
        reference_price: 1.3,
        current_price: 1.3,
        unrealized_pct: null,
        max_gain_pct: 0,
        ...overrides,
    });

export const makeClosedRow = (overrides: Partial<TrackedRow> = {}): TrackedRow => ({
    symbol: 'EZGO',
    exchange: 'NASDAQ',
    company_name: 'EZGO TECHNOLOGIES LTD',
    bucket: 'penny',
    status: 'closed',
    alerted_date: '2026-09-24',
    sessions_elapsed: 1,
    last_evaluated_date: '2026-09-25',
    evaluation_behind: false,
    reference_price: 1.09,
    current_price: null,
    current_price_date: null,
    unrealized_pct: null,
    max_gain_pct: 0,
    exit_reason: 'breakout_failed',
    exit_reason_note: BREAKOUT_FAILED_NOTE,
    exit_price: 0.91,
    exit_pct: -16.51376146788991,
    closed_date: '2026-09-25',
    ...overrides,
});

export const makeChain = (overrides: Partial<TrackedChain> = {}): TrackedChain => ({
    expected_session: '2026-09-25',
    last_scan_date: '2026-09-25',
    last_tracked_session: '2026-09-25',
    sessions_behind: 0,
    tracker_behind: false,
    ...overrides,
});

/** Counts default to the rows given, as the API's summary would. */
export const makeTracked = (
    tracked: TrackedRow[] = [makeUnevaluatedRow(), makeActiveRow(), makeClosedRow()],
    { chain = makeChain(), activeCount, closedCount }: { chain?: TrackedChain; activeCount?: number; closedCount?: number } = {}
): TrackedResponse => ({
    summary: {
        active_count: activeCount ?? tracked.filter(row => row.status === 'active').length,
        closed_count: closedCount ?? tracked.filter(row => row.status === 'closed').length,
    },
    chain,
    tracked,
});
