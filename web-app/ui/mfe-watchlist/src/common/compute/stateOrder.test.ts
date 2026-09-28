import { makeComputedSymbol, makeManualComputed, makeWatchlistQueued } from '@/test-utils/fixtures';
import { computeColumn } from './computeColumn';
import { COMPUTE_STATE_ORDER, computeStateRank, computeStateText } from './stateOrder';

describe('compute state order', () => {
    it('ranks attention first, then moving, then settled; unknown after known; none is missing', () => {
        expect(COMPUTE_STATE_ORDER).toEqual(['failed', 'data_not_arrived', 'waiting_for_data', 'computing', 'scheduled', 'computed']);
        expect(computeStateRank(makeComputedSymbol({ state: 'failed' }))).toBe(0);
        expect(computeStateRank(makeComputedSymbol({ state: 'computed' }))).toBe(5);
        expect(computeStateRank(makeComputedSymbol({ state: 'paused' as never }))).toBe(6);
        expect(computeStateRank(undefined)).toBeNull();
    });

    it('reads each state as ComputeState headlines it', () => {
        expect(computeStateText(makeComputedSymbol({ state: 'data_not_arrived' }))).toBe("Data hasn't arrived");
        expect(computeStateText(makeComputedSymbol({ state: 'paused' as never }))).toBe('paused');
        expect(computeStateText(undefined)).toBe('');
    });

    it("sorts a Compute column by the symbol's state, searching only the state a control shows", () => {
        const items = new Map([
            ['DIA', makeManualComputed('waiting_for_data')],
            ['BP', makeWatchlistQueued('computed')],
        ]);
        const column = computeColumn<{ symbol: string }>(symbol => items.get(symbol));

        expect(column.sortValue!({ symbol: 'DIA' })).toBe(2);
        expect(column.sortValue!({ symbol: 'BP' })).toBe(5);
        expect(column.sortValue!({ symbol: 'NONE' })).toBeNull();
        expect(column.searchText!({ symbol: 'DIA' })).toBe('Waiting for data');
        // A settled automatic queue shows a plain Compute button: nothing to search.
        expect(column.searchText!({ symbol: 'BP' })).toBe('');
        expect(column.initialDirection).toBe('asc');
    });
});
