import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { ComputedSymbol } from '@/api';
import { formatClockTime } from '@/common/format/format';
import { HostModeProvider } from '@/providers/HostModeContext';
import { makeComputedSymbol, makeManualComputed, makeWatchlistQueued } from '@/test-utils/fixtures';
import ComputeState, { COMPUTING_LABEL } from '.';

interface RenderOptions {
    hosted?: boolean;
    variant?: 'inline' | 'table';
    timeout?: number | null;
}

const renderState = (item: ComputedSymbol, { hosted = false, variant = 'inline', timeout = 30 }: RenderOptions = {}) =>
    render(
        <HostModeProvider hosted={hosted}>
            <MemoryRouter>
                <ComputeState item={item} dataTimeoutMinutes={timeout} variant={variant} />
            </MemoryRouter>
        </HostModeProvider>
    );

const state = (symbol = 'DIA') => screen.getByTestId(`compute-state-${symbol}`);
const requested = formatClockTime('2026-09-27T13:57:46Z');

describe('ComputeState', () => {
    it('shows a queued request with its local queue time and a pulse, not a spinner', () => {
        renderState(makeManualComputed('waiting_for_data'));

        expect(state()).toHaveTextContent(`Waiting for data · requested ${requested}`);
        expect(state()).toHaveAttribute('role', 'status');
        expect(state().querySelector('.compute-state__pulse')).toBeInTheDocument();
        expect(state().querySelector('time')).toHaveAttribute('dateTime', '2026-09-27T13:57:46Z');
        expect(screen.queryByTestId('ta-spinner')).not.toBeInTheDocument();
    });

    it("reads computing like Stock Detail's on-demand analysis", () => {
        renderState(makeManualComputed('computing'));

        expect(state()).toHaveTextContent(`${COMPUTING_LABEL} · data fetched, picked up within a minute · requested ${requested}`);
    });

    it('shows the computed time, and links Stock Detail hosted for a non-crypto symbol', () => {
        renderState(makeManualComputed('computed'), { hosted: true });

        expect(state()).toHaveTextContent(`Computed ${formatClockTime('2026-09-27T14:12:10Z')}`);
        expect(screen.getByRole('link', { name: 'Stock Detail' })).toHaveAttribute('href', '/candidates/DIA');
    });

    it('does not link Stock Detail standalone or for crypto', () => {
        const { unmount } = renderState(makeManualComputed('computed'));
        expect(screen.queryByRole('link')).not.toBeInTheDocument();
        unmount();

        renderState(makeManualComputed('computed', { symbol: 'SOLUSDT', asset_type: 'crypto' }), { hosted: true });
        expect(screen.queryByRole('link')).not.toBeInTheDocument();
    });

    it("says data hasn't arrived with the queue time, the wait and the timeout, without animating", () => {
        const requestedAt = new Date(Date.now() - 42 * 60_000).toISOString();
        renderState(makeManualComputed('data_not_arrived', { manual_requested_at: requestedAt, queued_at: requestedAt }));

        expect(state()).toHaveTextContent(
            `Data hasn't arrived — requested ${formatClockTime(requestedAt)}, waited 42 min (gives up waiting after 30 min)`
        );
        expect(state().querySelector('.compute-state__pulse')).not.toBeInTheDocument();
    });

    it('says "queued (added to watchlist)" for a fetch queued by a watchlist addition, not "requested"', () => {
        renderState(makeWatchlistQueued('waiting_for_data'));

        const queued = formatClockTime('2026-09-27T17:56:04Z');
        expect(state('BP')).toHaveTextContent(`Waiting for data · queued ${queued} (added to watchlist)`);
        expect(state('BP')).not.toHaveTextContent('requested');
        expect(state('BP').querySelector('time')).toHaveAttribute('dateTime', '2026-09-27T17:56:04Z');
    });

    it('uses the newest queue: a watchlist addition after a Compute press reads "queued"', () => {
        renderState(
            makeWatchlistQueued('computing', {
                reasons: ['watchlist', 'manual'],
                manual_requested_at: '2026-09-27T17:50:00Z',
                queued_at: '2026-09-27T17:56:04Z',
            })
        );
        expect(state('BP')).toHaveTextContent(`· queued ${formatClockTime('2026-09-27T17:56:04Z')} (added to watchlist)`);
    });

    it('reads "requested" when the Compute press is the newest queue', () => {
        renderState(makeManualComputed('waiting_for_data', { reasons: ['watchlist', 'manual'] }));
        expect(state()).toHaveTextContent(`Waiting for data · requested ${requested}`);
    });

    it('says "queued" without "added to watchlist" when no watchlist reason is open', () => {
        // Same local day as the queue time, so the time renders clock-only whatever the real date is.
        const now = new Date('2026-09-27T17:59:00Z');
        jest.useFakeTimers({ now });
        try {
            renderState(makeWatchlistQueued('waiting_for_data', { reasons: ['followed'] }));
            const queued = formatClockTime('2026-09-27T17:56:04Z', now);
            expect(queued).not.toContain(',');
            expect(state('BP').textContent).toBe(`Waiting for data · queued ${queued}`);
            expect(state('BP')).not.toHaveTextContent('added to watchlist');
        } finally {
            jest.useRealTimers();
        }
    });

    it('omits the timeout when it is unknown', () => {
        renderState(makeManualComputed('data_not_arrived'), { timeout: null });
        expect(state()).not.toHaveTextContent('gives up');
    });

    it('shows a failure with the stored error behind a disclosure and as a tooltip', async () => {
        renderState(makeManualComputed('failed', { last_error: 'technical-analysis: no 1Day bars' }));

        const summary = screen.getByText('Computation failed');
        expect(summary).toHaveAttribute('title', 'technical-analysis: no 1Day bars');
        await userEvent.click(summary);
        expect(screen.getByTestId('compute-error-detail-DIA')).toBeVisible();
        expect(screen.getByTestId('compute-error-detail-DIA')).toHaveTextContent('technical-analysis: no 1Day bars');
    });

    it('says so when a failure stored no message', () => {
        renderState(makeManualComputed('failed'));
        expect(screen.getByTestId('compute-error-detail-DIA')).toHaveTextContent('No error message was stored.');
    });

    it('shows a scheduled symbol as waiting for the next daily pass', () => {
        renderState(makeComputedSymbol({ state: 'scheduled', computed_at: null }));
        expect(state('AMZN')).toHaveTextContent('Scheduled · next daily pass');
    });

    it('drops the time and link in the table variant, which has its own Last computed column', () => {
        renderState(makeComputedSymbol(), { hosted: true, variant: 'table' });

        expect(state('AMZN')).toHaveTextContent(/^Computed$/);
        expect(screen.queryByRole('link')).not.toBeInTheDocument();
    });

    it('shows an unknown state verbatim', () => {
        renderState(makeComputedSymbol({ state: 'paused' as ComputedSymbol['state'] }));
        expect(state('AMZN')).toHaveTextContent('paused');
    });
});
