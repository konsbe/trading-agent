import { makeActiveRow, makeChain, makeClosedRow, makeTracked, makeUnevaluatedRow } from '@/test-utils/tracked';
import { parseTracked } from './parsers';

const body = () => JSON.parse(JSON.stringify(makeTracked())) as Record<string, any>;

describe('parseTracked', () => {
    it('parses summary, chain and rows as served', () => {
        const parsed = parseTracked(body());

        expect(parsed.summary).toEqual({ active_count: 2, closed_count: 1 });
        expect(parsed.chain).toEqual(makeChain());
        expect(parsed.tracked).toEqual([makeUnevaluatedRow(), makeActiveRow(), makeClosedRow()]);
    });

    it('keeps nulls as null, never 0', () => {
        const input = body();
        input.chain = { ...input.chain, last_scan_date: null, last_tracked_session: null, sessions_behind: null };
        input.tracked[1] = { ...input.tracked[1], exchange: null, company_name: null, current_price: null, current_price_date: null, unrealized_pct: null };

        const parsed = parseTracked(input);

        expect(parsed.chain).toMatchObject({ last_scan_date: null, last_tracked_session: null, sessions_behind: null });
        expect(parsed.tracked[0]).toMatchObject({ last_evaluated_date: null, sessions_elapsed: null, unrealized_pct: null });
        expect(parsed.tracked[1]).toMatchObject({ exchange: null, company_name: null, current_price: null, unrealized_pct: null });
    });

    it('accepts a null sessions_elapsed on a row not yet evaluated', () => {
        const input = body();
        input.tracked[0] = { ...input.tracked[0], sessions_elapsed: null, unrealized_pct: null, last_evaluated_date: null };

        expect(parseTracked(input).tracked[0]).toMatchObject({ sessions_elapsed: null, unrealized_pct: null, last_evaluated_date: null });
    });

    it('accepts an empty list', () => {
        expect(parseTracked({ summary: { active_count: 0, closed_count: 0 }, chain: makeChain(), tracked: [] }).tracked).toEqual([]);
    });

    it.each([
        ['a missing chain', (b: Record<string, any>) => delete b.chain, /chain: expected object/],
        ['a missing summary', (b: Record<string, any>) => delete b.summary, /summary: expected object/],
        ['an unknown status', (b: Record<string, any>) => (b.tracked[0].status = 'open'), /tracked\[0\]\.status: expected active\|closed/],
        ['an unknown bucket', (b: Record<string, any>) => (b.tracked[0].bucket = 'mega'), /tracked\[0\]\.bucket/],
        ['a non-date alerted_date', (b: Record<string, any>) => (b.tracked[0].alerted_date = '25/09/2026'), /alerted_date: expected YYYY-MM-DD/],
        ['a fractional count', (b: Record<string, any>) => (b.summary.active_count = 1.5), /summary\.active_count: expected a non-negative integer/],
        ['a negative sessions_behind', (b: Record<string, any>) => (b.chain.sessions_behind = -1), /chain\.sessions_behind/],
        ['a fractional sessions_elapsed', (b: Record<string, any>) => (b.tracked[1].sessions_elapsed = 1.5), /tracked\[1\]\.sessions_elapsed: expected a non-negative integer/],
        ['a negative sessions_elapsed', (b: Record<string, any>) => (b.tracked[1].sessions_elapsed = -1), /tracked\[1\]\.sessions_elapsed: expected a non-negative integer/],
        ['a string tracker_behind', (b: Record<string, any>) => (b.chain.tracker_behind = 'false'), /chain\.tracker_behind: expected boolean/],
        ['a string price', (b: Record<string, any>) => (b.tracked[2].exit_pct = '-16.5'), /tracked\[2\]\.exit_pct: expected number/],
    ])('rejects %s', (_label, mutate, message) => {
        const input = body();
        mutate(input);
        expect(() => parseTracked(input)).toThrow(message);
    });
});
