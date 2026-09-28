import { makeAlert, makeGroup } from '@/test-utils/alerts';
import { appendPage, GROUP_SHAPE, groupKey, mergeFirstPage, Page, RAW_SHAPE } from './paging';
import { FiredAlert } from '@/api';

/** Alert `id` fired `id` minutes after a base time, so a higher id is newer. */
const at = (id: number, extra: Partial<FiredAlert> = {}) =>
    makeAlert({ id, fired_at: new Date(Date.UTC(2026, 8, 27, 0, id)).toISOString().replace('.000', ''), ...extra });

const page = (ids: number[], hasMore: boolean): Page<FiredAlert> => ({
    items: ids.map(id => at(id)),
    hasMore,
    nextBefore: hasMore ? ids[ids.length - 1] : null,
    nextOffset: null,
});

const ids = (p: Page<FiredAlert>) => p.items.map(a => a.id);

describe('appendPage', () => {
    it('appends the older page and takes its cursor', () => {
        const merged = appendPage(page([10, 9, 8], true), page([7, 6], false), RAW_SHAPE);

        expect(ids(merged)).toEqual([10, 9, 8, 7, 6]);
        expect(merged).toMatchObject({ hasMore: false, nextBefore: null, nextOffset: null });
    });

    it('never lists an item twice', () => {
        expect(ids(appendPage(page([10, 9], true), page([9, 8], true), RAW_SHAPE))).toEqual([10, 9, 8]);
    });

    it('dedupes groups by symbol + type', () => {
        const a = makeGroup({ symbol: 'XOM', alert_type: 'bb_squeeze' });
        const b = makeGroup({ symbol: 'XOM', alert_type: 'liquidity_sweep' });
        const merged = appendPage({ items: [a], hasMore: true, nextBefore: 1, nextOffset: null }, { items: [a, b], hasMore: false, nextBefore: null, nextOffset: null }, GROUP_SHAPE);

        expect(merged.items.map(groupKey)).toEqual(['XOM|bb_squeeze', 'XOM|liquidity_sweep']);
    });
});

describe('mergeFirstPage', () => {
    it('keeps rows loaded with "Load older" and their cursor, adding the new ones on top', () => {
        // Loaded: page 1 = 10..8, page 2 = 7..5 (more after 5). New alerts 12, 11 arrived.
        const current = page([10, 9, 8, 7, 6, 5], true);
        const fresh = page([12, 11, 10], true);

        const merged = mergeFirstPage(current, fresh, RAW_SHAPE);

        expect(ids(merged)).toEqual([12, 11, 10, 9, 8, 7, 6, 5]);
        expect(merged).toMatchObject({ hasMore: true, nextBefore: 5, nextOffset: null });
    });

    it('takes the fresh page as the whole list when it has no more', () => {
        const merged = mergeFirstPage(page([10, 9, 8, 7], true), page([11, 10, 9, 8, 7, 6], false), RAW_SHAPE);

        expect(ids(merged)).toEqual([11, 10, 9, 8, 7, 6]);
        expect(merged.hasMore).toBe(false);
    });

    it('restarts paging when more than a page of new alerts arrived (no overlap)', () => {
        const fresh = page([20, 19, 18], true);

        expect(mergeFirstPage(page([10, 9, 8], true), fresh, RAW_SHAPE)).toBe(fresh);
    });

    it('uses the fresh page when nothing was listed', () => {
        const fresh = page([3, 2], true);

        expect(mergeFirstPage({ items: [], hasMore: false, nextBefore: null, nextOffset: null }, fresh, RAW_SHAPE)).toBe(fresh);
    });

    it('drops listed rows that fall inside the fresh page but are gone from it', () => {
        // 9 no longer matches (e.g. it was removed); 8 and older are kept.
        const merged = mergeFirstPage(page([10, 9, 8, 7], true), page([11, 10, 8], true), RAW_SHAPE);

        expect(ids(merged)).toEqual([11, 10, 8, 7]);
    });

    it('moves an updated group to the top without duplicating it', () => {
        const g = (symbol: string, id: number, count = 3) =>
            makeGroup({ symbol, alert_type: 'liquidity_sweep', count }, { id, fired_at: at(id).fired_at });
        const current = { items: [g('A', 10), g('B', 9), g('C', 8), g('D', 7)], hasMore: true, nextBefore: 7, nextOffset: null };
        // D fired again (id 12): it leads the fresh page with a higher count.
        const fresh = { items: [g('D', 12, 4), g('A', 10), g('B', 9)], hasMore: true, nextBefore: 9, nextOffset: null };

        const merged = mergeFirstPage(current, fresh, GROUP_SHAPE);

        expect(merged.items.map(x => `${x.symbol}×${x.count}`)).toEqual(['D×4', 'A×3', 'B×3', 'C×3']);
        expect(merged.nextBefore).toBe(7);
    });
});
