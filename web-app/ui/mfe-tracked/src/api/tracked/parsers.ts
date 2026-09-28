import { array, bool, num, nullable, obj, oneOf, optNum, optStr, str, tradingDay } from '../parse';
import { TRACKED_BUCKETS, TRACKED_STATUSES, TrackedChain, TrackedResponse, TrackedRow, TrackedSummary } from './types';

const status = oneOf(TRACKED_STATUSES);
const bucket = oneOf(TRACKED_BUCKETS);
const optDay = nullable(tradingDay);

const count = (value: unknown, path: string): number => {
    const n = num(value, path);
    if (!Number.isInteger(n) || n < 0) throw new Error(`Invalid momentum-api response at ${path}: expected a non-negative integer, got ${n}`);
    return n;
};

const optCount = nullable(count);

const parseSummary = (value: unknown): TrackedSummary => {
    const o = obj(value, 'summary');
    return {
        active_count: count(o.active_count, 'summary.active_count'),
        closed_count: count(o.closed_count, 'summary.closed_count'),
    };
};

const parseChain = (value: unknown): TrackedChain => {
    const o = obj(value, 'chain');
    return {
        expected_session: tradingDay(o.expected_session, 'chain.expected_session'),
        last_scan_date: optDay(o.last_scan_date, 'chain.last_scan_date'),
        last_tracked_session: optDay(o.last_tracked_session, 'chain.last_tracked_session'),
        sessions_behind: optCount(o.sessions_behind, 'chain.sessions_behind'),
        tracker_behind: bool(o.tracker_behind, 'chain.tracker_behind'),
    };
};

const parseRow = (value: unknown, path: string): TrackedRow => {
    const o = obj(value, path);
    return {
        symbol: str(o.symbol, `${path}.symbol`),
        exchange: optStr(o.exchange, `${path}.exchange`),
        company_name: optStr(o.company_name, `${path}.company_name`),
        bucket: bucket(o.bucket, `${path}.bucket`),
        status: status(o.status, `${path}.status`),
        alerted_date: tradingDay(o.alerted_date, `${path}.alerted_date`),
        sessions_elapsed: optCount(o.sessions_elapsed, `${path}.sessions_elapsed`),
        last_evaluated_date: optDay(o.last_evaluated_date, `${path}.last_evaluated_date`),
        evaluation_behind: bool(o.evaluation_behind, `${path}.evaluation_behind`),
        reference_price: num(o.reference_price, `${path}.reference_price`),
        current_price: optNum(o.current_price, `${path}.current_price`),
        current_price_date: optDay(o.current_price_date, `${path}.current_price_date`),
        unrealized_pct: optNum(o.unrealized_pct, `${path}.unrealized_pct`),
        max_gain_pct: optNum(o.max_gain_pct, `${path}.max_gain_pct`),
        exit_reason: optStr(o.exit_reason, `${path}.exit_reason`),
        exit_reason_note: optStr(o.exit_reason_note, `${path}.exit_reason_note`),
        exit_price: optNum(o.exit_price, `${path}.exit_price`),
        exit_pct: optNum(o.exit_pct, `${path}.exit_pct`),
        closed_date: optDay(o.closed_date, `${path}.closed_date`),
    };
};

/** `GET /api/v1/scanner/tracked`: strict, so a shape change fails loudly as `invalid_response`. */
export const parseTracked = (value: unknown): TrackedResponse => {
    const o = obj(value, '$');
    return {
        summary: parseSummary(o.summary),
        chain: parseChain(o.chain),
        tracked: array(o.tracked, 'tracked', parseRow),
    };
};
