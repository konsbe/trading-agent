import { act, render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { ApiError, fetchTracked, TrackedResponse } from '@/api';
import { HostModeProvider } from '@/providers/HostModeContext';
import { PAGE_INTRO } from '@/features/TrackedPositions';
import {
    BREAKOUT_FAILED_NOTE,
    LOST_VWAP_NOTE,
    makeActiveRow,
    makeChain,
    makeClosedRow,
    makeTracked,
    makeUnevaluatedRow,
} from '@/test-utils/tracked';
import TrackedPositionsPage from '.';

jest.mock('@/api/tracked/trackedApi', () => ({ fetchTracked: jest.fn() }));

const fetchMock = fetchTracked as jest.MockedFunction<typeof fetchTracked>;

const ROWS = [
    makeUnevaluatedRow(),
    makeUnevaluatedRow({ symbol: 'AESI', exchange: 'NYSE', bucket: 'market', reference_price: 12.47, current_price: 12.47 }),
    makeActiveRow(),
    makeActiveRow({ symbol: 'RSKD', reference_price: 7.91, current_price: 7.78, unrealized_pct: -1.643489254108721 }),
    makeActiveRow({ symbol: 'INVZ', alerted_date: '2026-09-23', sessions_elapsed: 1, last_evaluated_date: '2026-09-24', evaluation_behind: true, unrealized_pct: 2.5 }),
    makeClosedRow(),
    makeClosedRow({ symbol: 'IFBD', exit_reason: 'lost_vwap', exit_reason_note: LOST_VWAP_NOTE, exit_price: 1.34, exit_pct: 18.584070796460207 }),
];

const LIVE = () => makeTracked(ROWS, { activeCount: 20, closedCount: 16 });

const renderPage = async ({
    body = LIVE(),
    url = '/',
    hosted = false,
}: { body?: TrackedResponse; url?: string; hosted?: boolean } = {}) => {
    fetchMock.mockResolvedValue(body);
    const view = render(
        <MemoryRouter initialEntries={[url]}>
            <HostModeProvider hosted={hosted}>
                <TrackedPositionsPage />
            </HostModeProvider>
        </MemoryRouter>
    );
    await screen.findByRole('tablist');
    return view;
};

const row = (key: string) => within(screen.getByTestId(`tracked-row-${key}`));
const cell = (key: string, column: string) =>
    screen.getByTestId(`tracked-row-${key}`).querySelector(`[data-column="${column}"]`) as HTMLElement;

describe('TrackedPositionsPage', () => {
    it('shows the title, the research framing and tabs counted from summary', async () => {
        await renderPage();

        expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('Tracked Positions');
        expect(screen.getByText(PAGE_INTRO)).toBeInTheDocument();
        expect(screen.getByRole('tab', { name: 'Active (20)' })).toHaveAttribute('aria-selected', 'true');
        expect(screen.getByRole('tab', { name: 'Closed (16)' })).toHaveAttribute('aria-selected', 'false');
        expect(fetchMock).toHaveBeenCalledTimes(1);
        expect(fetchMock).toHaveBeenCalledWith('all', expect.objectContaining({ signal: expect.any(AbortSignal) }));
    });

    it('never uses portfolio or live-trading language', async () => {
        const { container } = await renderPage();
        // Company names are data ("… HOLDINGS INC"), not the page's copy.
        screen.getAllByTestId('company-name').forEach(el => el.remove());
        const text = container.textContent ?? '';

        expect(text).not.toMatch(/P&L|P\/L|portfolio|holdings|performance|returns|streaming|\blive\b/i);
    });

    it('lists the active columns, newest alert first, without the closed-only ones', async () => {
        await renderPage();
        const headers = within(screen.getByTestId('tracked-active-table')).getAllByRole('columnheader').map(th => th.textContent);

        expect(headers).toEqual([
            'Symbol / Exchange / Bucket',
            'Alerted Date',
            'Sessions Elapsed',
            'Reference Price',
            'Current Price',
            'Unrealized %',
        ]);
        expect(screen.getAllByTestId(/^tracked-row-/)).toHaveLength(5);
        expect(row('DNA-2026-09-24').getByTestId('exchange')).toHaveTextContent('NYSE');
        expect(row('DNA-2026-09-24').getByTestId('bucket')).toHaveTextContent('Market');
        expect(cell('DNA-2026-09-24', 'alerted_date')).toHaveTextContent('Sep 24, 2026');
        expect(cell('DNA-2026-09-24', 'reference_price')).toHaveTextContent('$10.25');
        expect(cell('DNA-2026-09-24', 'current_price')).toHaveTextContent('$10.66');
        expect(cell('DNA-2026-09-24', 'sessions_elapsed')).toHaveTextContent(/^1$/);
    });

    describe('rows not yet evaluated', () => {
        it('read "Not yet evaluated" and "—", never 0 or 0.0%, and are not coloured', async () => {
            await renderPage();

            const sessions = cell('ABLV-2026-09-25', 'sessions_elapsed');
            const unrealized = cell('ABLV-2026-09-25', 'unrealized_pct');
            expect(sessions).toHaveTextContent(/^Not yet evaluated$/);
            expect(unrealized).toHaveTextContent(/^—$/);
            expect(unrealized.textContent).not.toMatch(/0/);
            expect(unrealized.querySelector('.is-price-up, .is-price-down')).toBeNull();
            expect(cell('ABLV-2026-09-25', 'current_price')).toHaveTextContent('$1.30');
        });

        it('are recognised from a null sessions_elapsed even when last_evaluated_date is set', async () => {
            await renderPage({ body: makeTracked([makeUnevaluatedRow({ last_evaluated_date: '2026-09-25' })]) });

            expect(cell('ABLV-2026-09-25', 'sessions_elapsed')).toHaveTextContent(/^Not yet evaluated$/);
            expect(cell('ABLV-2026-09-25', 'unrealized_pct')).toHaveTextContent(/^—$/);
        });

        it('get one calm note above the table, counting them', async () => {
            await renderPage();

            expect(screen.getByTestId('not-evaluated-note')).toHaveTextContent(
                '2 rows have not been evaluated yet — the next tracker run evaluates them.'
            );
        });

        it('say "1 row has … it" for a single row, and there is no note when every row is evaluated', async () => {
            const { unmount } = await renderPage({ body: makeTracked([makeUnevaluatedRow(), makeActiveRow()]) });
            expect(screen.getByTestId('not-evaluated-note')).toHaveTextContent('1 row has not been evaluated yet — the next tracker run evaluates it.');
            unmount();

            await renderPage({ body: makeTracked([makeActiveRow()]) });
            expect(screen.queryByTestId('not-evaluated-note')).not.toBeInTheDocument();
        });
    });

    it('notes neutrally when a row is evaluated only as of an earlier date', async () => {
        await renderPage();

        const note = row('INVZ-2026-09-23').getByTestId('evaluation-behind');
        expect(note).toHaveTextContent('as of Sep 24, 2026');
        expect(note).toHaveAttribute('title', expect.stringMatching(/^Sessions elapsed as of Sep 24, 2026/));
        expect(row('DNA-2026-09-24').queryByTestId('evaluation-behind')).not.toBeInTheDocument();
    });

    it('colours only Unrealized % and Exit %, with the price tokens', async () => {
        const { container } = await renderPage();

        expect(cell('DNA-2026-09-24', 'unrealized_pct').firstChild).toHaveClass('is-price-up');
        expect(cell('DNA-2026-09-24', 'unrealized_pct')).toHaveTextContent('+4.0%');
        expect(cell('RSKD-2026-09-24', 'unrealized_pct').firstChild).toHaveClass('is-price-down');

        await userEvent.click(screen.getByRole('tab', { name: 'Closed (16)' }));
        expect(cell('EZGO-2026-09-24', 'exit_pct').firstChild).toHaveClass('is-price-down');
        expect(cell('EZGO-2026-09-24', 'exit_pct')).toHaveTextContent('−16.5%');
        expect(cell('IFBD-2026-09-24', 'exit_pct').firstChild).toHaveClass('is-price-up');

        const toned = Array.from(container.querySelectorAll('.is-price-up, .is-price-down'));
        expect(toned.length).toBeGreaterThan(0);
        toned.forEach(el => expect(el.closest('td')?.getAttribute('data-column')).toMatch(/^(unrealized_pct|exit_pct)$/));
    });

    describe('closed tab', () => {
        it('opens from the tab or from ?tab=closed, with Closed Date, Exit Reason and Exit % columns and no exit price', async () => {
            await renderPage({ url: '/?tab=closed' });

            expect(screen.getByRole('tab', { name: 'Closed (16)' })).toHaveAttribute('aria-selected', 'true');
            const headers = within(screen.getByTestId('tracked-closed-table')).getAllByRole('columnheader').map(th => th.textContent);
            expect(headers).toEqual([
                'Symbol / Exchange / Bucket',
                'Alerted Date',
                'Closed Date',
                'Sessions Elapsed',
                'Reference Price',
                'Exit Reason',
                'Exit %',
            ]);
            expect(screen.getAllByTestId(/^tracked-row-/)).toHaveLength(2);
            expect(screen.getByTestId('tracked-closed-table').querySelector('[data-column="exit_price"]')).toBeNull();
            expect(fetchMock).toHaveBeenCalledTimes(1);
        });

        it('shows the closed date like the alerted date, and "—" without one', async () => {
            await renderPage({
                url: '/?tab=closed',
                body: makeTracked([makeClosedRow(), makeClosedRow({ symbol: 'AAA', closed_date: null })]),
            });

            expect(cell('EZGO-2026-09-24', 'alerted_date')).toHaveTextContent(/^Sep 24, 2026$/);
            expect(cell('EZGO-2026-09-24', 'closed_date')).toHaveTextContent(/^Sep 25, 2026$/);
            expect(cell('AAA-2026-09-24', 'closed_date')).toHaveTextContent(/^—$/);
        });

        it('shows the exit reason as plain text, not a badge', async () => {
            await renderPage({ url: '/?tab=closed' });

            const reason = row('EZGO-2026-09-24').getByRole('button', { name: 'breakout_failed' });
            expect(reason).toHaveTextContent(/^breakout_failed$/);
            expect(reason.className).not.toMatch(/badge|severity/);
        });

        it('expands the note inline on click, and it stays open until clicked again', async () => {
            await renderPage({ url: '/?tab=closed' });
            const toggle = row('EZGO-2026-09-24').getByRole('button', { name: 'breakout_failed' });
            const panel = document.getElementById(toggle.getAttribute('aria-controls') as string) as HTMLElement;

            expect(toggle).toHaveAttribute('aria-expanded', 'false');
            expect(panel).not.toBeVisible();

            await userEvent.click(toggle);
            expect(toggle).toHaveAttribute('aria-expanded', 'true');
            expect(panel).toBeVisible();
            expect(screen.getByTestId('exit-note-EZGO-2026-09-24')).toHaveTextContent(BREAKOUT_FAILED_NOTE);
            expect(screen.queryByRole('tooltip')).not.toBeInTheDocument();

            await userEvent.click(document.body);
            expect(panel).toBeVisible();

            await userEvent.click(toggle);
            expect(toggle).toHaveAttribute('aria-expanded', 'false');
            expect(panel).not.toBeVisible();
        });

        it('toggles the note from the keyboard with Enter and Space', async () => {
            await renderPage({ url: '/?tab=closed' });
            const toggle = row('IFBD-2026-09-24').getByRole('button', { name: 'lost_vwap' });

            toggle.focus();
            await userEvent.keyboard('{Enter}');
            expect(toggle).toHaveAttribute('aria-expanded', 'true');
            expect(screen.getByTestId('exit-note-IFBD-2026-09-24')).toHaveTextContent(/^lost_vwapClosed below its rolling 20-day VWAP\.$/);

            await userEvent.keyboard(' ');
            expect(toggle).toHaveAttribute('aria-expanded', 'false');
        });

        it('shows a reason without a note as plain text, a missing reason and exchange as "—"', async () => {
            await renderPage({
                url: '/?tab=closed',
                body: makeTracked([
                    makeClosedRow({ symbol: 'AAA', exit_reason_note: null }),
                    makeClosedRow({ symbol: 'BBB', exchange: null, exit_reason: null, exit_reason_note: null }),
                ]),
            });

            expect(cell('AAA-2026-09-24', 'exit_reason')).toHaveTextContent(/^breakout_failed$/);
            expect(row('AAA-2026-09-24').queryByRole('button')).not.toBeInTheDocument();
            expect(cell('BBB-2026-09-24', 'exit_reason')).toHaveTextContent(/^—$/);
            expect(row('BBB-2026-09-24').getByTestId('exchange')).toHaveTextContent('—');
        });

        it('keeps each note independent and open across a tab switch', async () => {
            await renderPage({ url: '/?tab=closed' });
            await userEvent.click(row('EZGO-2026-09-24').getByRole('button', { name: 'breakout_failed' }));

            expect(row('IFBD-2026-09-24').getByRole('button', { name: 'lost_vwap' })).toHaveAttribute('aria-expanded', 'false');

            await userEvent.click(screen.getByRole('tab', { name: 'Active (20)' }));
            await userEvent.click(screen.getByRole('tab', { name: 'Closed (16)' }));
            expect(row('EZGO-2026-09-24').getByRole('button', { name: 'breakout_failed' })).toHaveAttribute('aria-expanded', 'true');
        });
    });

    it('moves between tabs with the arrow keys, Home and End', async () => {
        await renderPage();
        const active = screen.getByRole('tab', { name: 'Active (20)' });
        const closed = screen.getByRole('tab', { name: 'Closed (16)' });

        expect(active).toHaveAttribute('tabIndex', '0');
        expect(closed).toHaveAttribute('tabIndex', '-1');
        active.focus();
        await userEvent.keyboard('{ArrowRight}');
        expect(closed).toHaveAttribute('aria-selected', 'true');
        expect(closed).toHaveFocus();
        expect(screen.getByRole('tabpanel')).toHaveAttribute('aria-labelledby', closed.id);

        await userEvent.keyboard('{ArrowRight}');
        expect(active).toHaveAttribute('aria-selected', 'true');
        await userEvent.keyboard('{End}');
        expect(closed).toHaveAttribute('aria-selected', 'true');
        await userEvent.keyboard('{Home}');
        expect(active).toHaveAttribute('aria-selected', 'true');
        await userEvent.keyboard('{ArrowLeft}');
        expect(closed).toHaveAttribute('aria-selected', 'true');
    });

    describe('freshness banner', () => {
        it('is absent when the chain is current, even with old row dates', async () => {
            await renderPage({ body: makeTracked([makeActiveRow({ alerted_date: '2026-01-02', last_evaluated_date: '2026-01-05' })]) });

            expect(screen.queryByTestId('freshness-banner')).not.toBeInTheDocument();
        });

        it('says how many trading sessions have no new scan, above the tabs', async () => {
            await renderPage({ body: makeTracked(ROWS, { chain: makeChain({ expected_session: '2026-09-29', sessions_behind: 2 }) }) });

            const banner = screen.getByTestId('freshness-banner');
            expect(banner).toHaveTextContent(
                'No new scan for 2 trading sessions — the latest is Sep 25, 2026. Tracked figures below are as of that date.'
            );
            expect(banner.compareDocumentPosition(screen.getByRole('tablist')) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
            expect(banner).not.toHaveAttribute('role', 'alert');
            expect(banner.querySelector('svg')).toBeNull();
            expect(screen.getAllByTestId('freshness-banner')).toHaveLength(1);
            expect(screen.getByTestId('not-evaluated-note')).toBeInTheDocument();
        });

        it('says when tracking has not caught up with a fresh scan', async () => {
            await renderPage({
                body: makeTracked(ROWS, { chain: makeChain({ last_scan_date: '2026-09-28', expected_session: '2026-09-28', tracker_behind: true }) }),
            });

            expect(screen.getByTestId('freshness-banner')).toHaveTextContent(
                'The Sep 28, 2026 scan exists but tracking has not been updated since Sep 25, 2026.'
            );
        });

        it('prefers "no new scan" when both apply', async () => {
            await renderPage({ body: makeTracked(ROWS, { chain: makeChain({ sessions_behind: 1, tracker_behind: true }) }) });

            expect(screen.getByTestId('freshness-banner')).toHaveAttribute('data-variant', 'no_new_scan');
            expect(screen.getByTestId('freshness-banner')).toHaveTextContent(/^No new scan for 1 trading session —/);
        });
    });

    describe('empty states', () => {
        it('reads "Nothing currently tracked" with no active rows', async () => {
            await renderPage({ body: makeTracked([makeClosedRow()]) });

            expect(screen.getByRole('tab', { name: 'Active (0)' })).toBeInTheDocument();
            expect(screen.getByText('Nothing currently tracked')).toBeInTheDocument();
            expect(screen.queryByTestId('tracked-active-table')).not.toBeInTheDocument();
        });

        it('reads "No closed positions yet" with no closed rows', async () => {
            await renderPage({ body: makeTracked([makeActiveRow()]), url: '/?tab=closed' });

            expect(screen.getByText('No closed positions yet')).toBeInTheDocument();
            expect(screen.queryByTestId('tracked-closed-table')).not.toBeInTheDocument();
        });
    });

    it('shows a skeleton while loading', async () => {
        let resolve: (body: TrackedResponse) => void = () => undefined;
        fetchMock.mockReturnValue(new Promise(r => (resolve = r)));
        render(<TrackedPositionsPage />, { wrapper: MemoryRouter });

        expect(screen.getByTestId('tracked-skeleton')).toHaveAttribute('aria-busy', 'true');
        await act(async () => resolve(LIVE()));
        expect(screen.queryByTestId('tracked-skeleton')).not.toBeInTheDocument();
    });

    it.each([
        [503, 'database_unavailable', 'The tracking database is unavailable right now.'],
        [500, 'session_calendar_unavailable', 'The market session calendar is unavailable right now.'],
    ])('shows the error state for HTTP %i and retries', async (status, code, message) => {
        fetchMock.mockRejectedValueOnce(new ApiError(status, code)).mockResolvedValueOnce(LIVE());
        render(<TrackedPositionsPage />, { wrapper: MemoryRouter });

        expect(await screen.findByTestId('api-error-state')).toHaveTextContent(message);
        await userEvent.click(screen.getByRole('button', { name: 'Retry' }));
        expect(await screen.findByRole('tablist')).toBeInTheDocument();
        expect(fetchMock).toHaveBeenCalledTimes(2);
    });

    it('links the ticker to Stock Detail only when hosted', async () => {
        const { unmount } = await renderPage({ hosted: true });
        expect(row('DNA-2026-09-24').getByRole('link', { name: 'DNA' })).toHaveAttribute('href', '/candidates/DNA');
        unmount();

        await renderPage();
        expect(row('DNA-2026-09-24').queryByRole('link')).not.toBeInTheDocument();
    });

    it('keeps a row whose symbol stopped scanning, with "—" for its price and Unrealized %', async () => {
        await renderPage({ body: makeTracked([makeActiveRow({ current_price: null, current_price_date: null, unrealized_pct: null })]) });

        expect(cell('DNA-2026-09-24', 'current_price')).toHaveTextContent(/^—$/);
        expect(cell('DNA-2026-09-24', 'unrealized_pct')).toHaveTextContent(/^—$/);
        expect(cell('DNA-2026-09-24', 'unrealized_pct').querySelector('.is-price-up, .is-price-down')).toBeNull();
    });

    describe('refresh on tab focus', () => {
        const setVisibility = (state: 'visible' | 'hidden') => {
            Object.defineProperty(document, 'visibilityState', { configurable: true, get: () => state });
            document.dispatchEvent(new Event('visibilitychange'));
        };

        afterEach(() => Object.defineProperty(document, 'visibilityState', { configurable: true, get: () => 'visible' }));

        it('re-reads when the browser tab becomes visible again', async () => {
            await renderPage();
            fetchMock.mockResolvedValueOnce(makeTracked(ROWS, { activeCount: 21, closedCount: 16 }));

            act(() => setVisibility('hidden'));
            expect(fetchMock).toHaveBeenCalledTimes(1);
            await act(async () => setVisibility('visible'));

            expect(fetchMock).toHaveBeenCalledTimes(2);
            expect(await screen.findByRole('tab', { name: 'Active (21)' })).toBeInTheDocument();
        });

        it('keeps the rows and says so when a re-read fails', async () => {
            await renderPage();
            fetchMock.mockRejectedValueOnce(new ApiError(503, 'database_unavailable'));

            await act(async () => setVisibility('visible'));

            expect(await screen.findByTestId('refresh-error')).toHaveTextContent(
                "Couldn't check for updates: The tracking database is unavailable right now. The rows below are from the last load."
            );
            expect(screen.queryByTestId('api-error-state')).not.toBeInTheDocument();
            expect(screen.getAllByTestId(/^tracked-row-/)).toHaveLength(5);
        });
    });

    it('keeps a symbol tracked twice as two rows', async () => {
        await renderPage({
            body: makeTracked([makeUnevaluatedRow({ symbol: 'SDEV' }), makeActiveRow({ symbol: 'SDEV' })]),
        });

        expect(screen.getByTestId('tracked-row-SDEV-2026-09-25')).toBeInTheDocument();
        expect(screen.getByTestId('tracked-row-SDEV-2026-09-24')).toBeInTheDocument();
    });
});
