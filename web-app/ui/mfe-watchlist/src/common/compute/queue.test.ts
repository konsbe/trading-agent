import { makeComputedSymbol, makeManualComputed, makeWatchlistQueued } from '@/test-utils/fixtures';
import { isPending, isQueuedByRequest, queuedTime } from './queue';

describe('queuedTime', () => {
    it('is queued_at, falling back to the manual request time', () => {
        expect(queuedTime(makeWatchlistQueued('waiting_for_data'))).toBe('2026-09-27T17:56:04Z');
        expect(queuedTime({ queued_at: null, manual_requested_at: '2026-09-27T13:57:46Z' })).toBe('2026-09-27T13:57:46Z');
        expect(queuedTime(makeComputedSymbol())).toBeNull();
    });
});

describe('isQueuedByRequest', () => {
    it('is true when the Compute press is the newest queue', () => {
        expect(isQueuedByRequest(makeManualComputed('waiting_for_data'))).toBe(true);
        expect(isQueuedByRequest({ queued_at: null, manual_requested_at: '2026-09-27T13:57:46Z' })).toBe(true);
    });

    it('is false for a watchlist queue, including one newer than a Compute press', () => {
        expect(isQueuedByRequest(makeWatchlistQueued('waiting_for_data'))).toBe(false);
        expect(isQueuedByRequest({ queued_at: '2026-09-27T17:56:04Z', manual_requested_at: '2026-09-27T17:50:00Z' })).toBe(false);
    });
});

describe('isPending', () => {
    it('is true for a moving queue of either kind', () => {
        expect(isPending(makeWatchlistQueued('computing'))).toBe(true);
        expect(isPending(makeManualComputed('waiting_for_data'))).toBe(true);
        expect(isPending(makeWatchlistQueued('failed'))).toBe(false);
        expect(isPending(makeComputedSymbol({ state: 'computing' }))).toBe(false);
    });
});
