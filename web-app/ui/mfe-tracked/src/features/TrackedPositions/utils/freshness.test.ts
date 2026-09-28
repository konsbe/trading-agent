import { makeChain } from '@/test-utils/tracked';
import { chainFreshness } from './freshness';

describe('chainFreshness', () => {
    it('shows nothing when the chain is current', () => {
        expect(chainFreshness(makeChain())).toBeNull();
    });

    it('counts trading sessions, singular for one', () => {
        expect(chainFreshness(makeChain({ expected_session: '2026-09-28', sessions_behind: 1 }))).toEqual({
            variant: 'no_new_scan',
            message: 'No new scan for 1 trading session — the latest is Sep 25, 2026. Tracked figures below are as of that date.',
        });
        expect(chainFreshness(makeChain({ expected_session: '2026-09-29', sessions_behind: 2 }))?.message).toBe(
            'No new scan for 2 trading sessions — the latest is Sep 25, 2026. Tracked figures below are as of that date.'
        );
    });

    it('reports the tracker falling behind a fresh scan', () => {
        expect(chainFreshness(makeChain({ last_scan_date: '2026-09-28', last_tracked_session: '2026-09-25', tracker_behind: true }))).toEqual({
            variant: 'tracker_behind',
            message: 'The Sep 28, 2026 scan exists but tracking has not been updated since Sep 25, 2026.',
        });
    });

    it('says so when tracking never completed', () => {
        expect(chainFreshness(makeChain({ last_tracked_session: null, tracker_behind: true }))?.message).toBe(
            'The Sep 25, 2026 scan exists but tracking has not been updated for it yet.'
        );
    });

    it('gives "no new scan" precedence over "tracker behind"', () => {
        expect(chainFreshness(makeChain({ sessions_behind: 3, tracker_behind: true }))?.variant).toBe('no_new_scan');
    });

    it('shows nothing without a scan at all', () => {
        expect(chainFreshness(makeChain({ last_scan_date: null, last_tracked_session: null, sessions_behind: null, tracker_behind: true }))).toBeNull();
    });
});
