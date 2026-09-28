import { act, render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter, Route, Routes, useLocation } from 'react-router-dom';
import { FiredAlert } from '@/api';
import { HostModeProvider } from '@/providers/HostModeContext';
import { makeAlert } from '@/test-utils/alerts';
import { createAlertsServer } from '@/test-utils/alertsServer';
import AlarmHistoryScreen, { SEARCH_DEBOUNCE_MS } from './AlarmHistoryScreen';

// 22:00 in Athens (jest.config.js pins TZ): the default range is Sep 21–27, until 2026-09-27T21:00Z.
const NOW = new Date('2026-09-27T19:00:00Z');
const minutesAgo = (m: number) => new Date(NOW.getTime() - m * 60_000).toISOString().replace('.000', '');

let nextId = 1;
const series = (count: number, overrides: Partial<FiredAlert>, startMinutesAgo: number, stepMinutes = 240): FiredAlert[] =>
    Array.from({ length: count }, (_, i) =>
        makeAlert({ id: nextId++, fired_at: minutesAgo(startMinutesAgo + i * stepMinutes), ...overrides })
    );

const dataset = () => {
    nextId = 1;
    return [
        ...series(12, { symbol: 'XOM', exchange_type: 'equity', alert_type: 'liquidity_sweep', severity: 'notice' }, 30),
        ...series(3, { symbol: 'TIAUSDT', exchange_type: 'crypto', alert_type: 'liquidity_sweep', severity: 'notice', message: 'Liquidity sweep detected (4 sweeps)' }, 20),
        ...series(1, { symbol: 'AAPL', exchange_type: 'equity', alert_type: 'vix_elevated', severity: 'warning', message: 'VIX 31.2 — regime: elevated' }, 10),
        ...series(2, { symbol: 'MSFT', exchange_type: 'equity', alert_type: 'bb_squeeze', severity: 'info', message: 'Bollinger Squeeze active' }, 40),
    ];
};

const Location = () => {
    const { pathname, search } = useLocation();
    return <span data-testid="location">{`${pathname}${search}`}</span>;
};
const Detail = () => {
    const { pathname, hash, state } = useLocation();
    return <span data-testid="detail">{JSON.stringify({ pathname, hash, state })}</span>;
};

const renderScreen = (url = '/alarm-history', props: { pageSize?: number; refreshIntervalMs?: number } = {}) =>
    render(
        <MemoryRouter initialEntries={[url]}>
            <HostModeProvider hosted>
                <Location />
                <Routes>
                    <Route path="/alarm-history/*" element={<AlarmHistoryScreen {...props} />} />
                    <Route path="/candidates/:symbol" element={<Detail />} />
                </Routes>
            </HostModeProvider>
        </MemoryRouter>
    );

const groupKeys = () => screen.queryAllByTestId(/^group-row-/).map(r => r.getAttribute('data-testid')!.replace('group-row-', ''));
const rawIds = () => within(screen.getByTestId('alarm-raw-table')).getAllByTestId(/^alert-row-/).map(r => r.getAttribute('data-testid'));
const location = () => screen.getByTestId('location').textContent;

/** A pinned `until`: an ISO instant within a second after `at` (user-event's delays advance the fake clock). */
const expectPinnedAt = (until: string | undefined, at: Date) => {
    expect(until).toMatch(/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3}Z$/);
    const diff = Date.parse(until!) - at.getTime();
    expect(diff).toBeGreaterThanOrEqual(0);
    expect(diff).toBeLessThan(1_000);
};
const later = (ms: number) => new Date(NOW.getTime() + ms);

describe('AlarmHistoryScreen server-side sort and search', () => {
    let server: ReturnType<typeof createAlertsServer>;
    let user: ReturnType<typeof userEvent.setup>;

    beforeEach(() => {
        window.sessionStorage.clear();
        jest.useFakeTimers({ now: NOW });
        user = userEvent.setup({ advanceTimers: jest.advanceTimersByTime });
        server = createAlertsServer(dataset());
        (global as any).fetch = server.fetch;
        Object.defineProperty(document, 'visibilityState', { configurable: true, get: () => 'visible' });
    });

    afterEach(() => jest.useRealTimers());

    const lastQuery = () => Object.fromEntries(server.requests()[server.requests().length - 1]);

    const search = async (text: string) => {
        await user.type(screen.getByTestId('alarm-search-input'), text);
        act(() => {
            jest.advanceTimersByTime(SEARCH_DEBOUNCE_MS);
        });
    };

    it('sorts every column but the expander, on the server', async () => {
        renderScreen();
        await waitFor(() => expect(groupKeys()).toHaveLength(4));

        const table = screen.getByTestId('alarm-grouped-table');
        for (const name of ['Last fired', 'Symbol', 'Alert type', 'Severity', 'Latest message', 'Repeats']) {
            expect(within(table).getByRole('button', { name })).toBeInTheDocument();
        }
        expect(within(table).getByRole('columnheader', { name: 'Show alerts' })).not.toHaveAttribute('aria-sort');
        expect(within(table).getByRole('columnheader', { name: 'Last fired' })).toHaveAttribute('aria-sort', 'descending');

        await user.click(within(table).getByRole('button', { name: 'Repeats' }));

        await waitFor(() => expect(groupKeys()).toEqual(['XOM|liquidity_sweep', 'TIAUSDT|liquidity_sweep', 'MSFT|bb_squeeze', 'AAPL|vix_elevated']));
        // Pinned: the first load's time, earlier than the range's end.
        const { until, ...rest } = lastQuery();
        expect(rest).toEqual({ since: '2026-09-21T00:00:00+03:00', limit: '100', mode: 'grouped', sort: 'count', dir: 'desc' });
        expectPinnedAt(until, NOW);
        expect(location()).toBe('/alarm-history?groups_sort=count%3Adesc');
        expect(table.querySelector('caption')).toHaveTextContent('sorted by Repeats, descending');
    });

    it('sorts groups by their latest message, ascending first, and back', async () => {
        renderScreen();
        await waitFor(() => expect(groupKeys()).toHaveLength(4));
        const table = screen.getByTestId('alarm-grouped-table');

        await user.click(within(table).getByRole('button', { name: 'Latest message' }));

        await waitFor(() => expect(groupKeys()).toEqual(['MSFT|bb_squeeze', 'TIAUSDT|liquidity_sweep', 'XOM|liquidity_sweep', 'AAPL|vix_elevated']));
        expect(lastQuery()).toMatchObject({ mode: 'grouped', sort: 'message', dir: 'asc' });
        expect(location()).toBe('/alarm-history?groups_sort=message%3Aasc');
        const sorted = screen.getByTestId('alarm-grouped-table');
        expect(within(sorted).getByRole('columnheader', { name: 'Latest message' })).toHaveAttribute('aria-sort', 'ascending');

        await user.click(within(sorted).getByRole('button', { name: 'Latest message' }));
        await waitFor(() => expect(groupKeys()[0]).toBe('AAPL|vix_elevated'));
        expect(lastQuery()).toMatchObject({ sort: 'message', dir: 'desc' });
    });

    it('sorts every alert by its message', async () => {
        renderScreen('/alarm-history?view=all');
        await waitFor(() => expect(screen.getByTestId('alarm-raw-table')).toBeInTheDocument());

        await user.click(within(screen.getByTestId('alarm-raw-table')).getByRole('button', { name: 'Message' }));

        await waitFor(() => expect(lastQuery()).toMatchObject({ mode: 'raw', sort: 'message', dir: 'asc' }));
        await waitFor(() =>
            expect(within(screen.getByTestId('alarm-raw-table')).getAllByTestId(/^alert-row-/)[0]).toHaveTextContent('Bollinger Squeeze active')
        );
        expect(location()).toBe('/alarm-history?view=all&alerts_sort=message%3Aasc');
    });

    it('keeps the default newest-first view on the id cursor, without sort params or a pinned until', async () => {
        renderScreen('/alarm-history?groups_sort=fired%3Adesc');
        await waitFor(() => expect(groupKeys()).toHaveLength(4));

        expect(lastQuery()).toEqual({
            since: '2026-09-21T00:00:00+03:00',
            until: '2026-09-28T00:00:00+03:00',
            limit: '100',
            mode: 'grouped',
        });
    });

    it('searches every record on the server, in the URL', async () => {
        renderScreen();
        await waitFor(() => expect(groupKeys()).toHaveLength(4));

        await search('sweep');

        await waitFor(() => expect(groupKeys()).toEqual(['TIAUSDT|liquidity_sweep', 'XOM|liquidity_sweep']));
        expect(lastQuery()).toMatchObject({ q: 'sweep', mode: 'grouped' });
        expectPinnedAt(lastQuery().until, NOW);
        expect(lastQuery().sort).toBeUndefined();
        expect(location()).toBe('/alarm-history?groups_q=sweep');
    });

    it('says when nothing matches the search', async () => {
        renderScreen();
        await waitFor(() => expect(groupKeys()).toHaveLength(4));

        await search('zzz');

        expect(await screen.findByTestId('alarm-list-empty')).toHaveTextContent('No alerts in these filters match “zzz”.');
    });

    it('pages a sorted view with Load more by offset, keeping the pinned until', async () => {
        renderScreen('/alarm-history?groups_sort=symbol%3Aasc', { pageSize: 2 });
        await waitFor(() => expect(groupKeys()).toEqual(['AAPL|vix_elevated', 'MSFT|bb_squeeze']));
        expect(lastQuery()).toMatchObject({ sort: 'symbol', dir: 'asc' });
        expect(lastQuery().offset).toBeUndefined();
        const pinned = lastQuery().until;
        expectPinnedAt(pinned, NOW);

        const footer = screen.getByTestId('grouped-footer');
        expect(footer).toHaveTextContent('Showing the first 2 groups');
        act(() => {
            jest.advanceTimersByTime(20_000);
        });
        await user.click(within(footer).getByRole('button', { name: 'Load more' }));

        await waitFor(() => expect(groupKeys()).toEqual(['AAPL|vix_elevated', 'MSFT|bb_squeeze', 'TIAUSDT|liquidity_sweep', 'XOM|liquidity_sweep']));
        expect(lastQuery()).toMatchObject({ offset: '2', until: pinned, sort: 'symbol', dir: 'asc' });
        expect(lastQuery().before).toBeUndefined();
        expect(screen.getByTestId('grouped-footer')).toHaveTextContent('Showing all 4 groups');
    });

    it('starts a new view on refresh: page 1 again, pinned to the refresh time', async () => {
        renderScreen('/alarm-history?groups_sort=symbol%3Aasc', { pageSize: 2 });
        await waitFor(() => expect(groupKeys()).toHaveLength(2));
        const firstPinned = lastQuery().until;
        await user.click(screen.getByRole('button', { name: 'Load more' }));
        await waitFor(() => expect(groupKeys()).toHaveLength(4));

        server.add(makeAlert({ id: 99, symbol: 'AAA', alert_type: 'rsi_overbought', fired_at: minutesAgo(-0.5) }));
        act(() => {
            jest.advanceTimersByTime(60_000);
        });

        await waitFor(() => expect(groupKeys()).toEqual(['AAA|rsi_overbought', 'AAPL|vix_elevated']));
        expect(lastQuery()).toMatchObject({ sort: 'symbol', dir: 'asc' });
        expect(lastQuery().offset).toBeUndefined();
        expect(lastQuery().until).not.toBe(firstPinned);
        expectPinnedAt(lastQuery().until, later(60_000));
        expect(screen.getByTestId('grouped-footer')).toHaveTextContent('Showing the first 2 groups');
    });

    it("sends the range's end when it is earlier than the pinned time", async () => {
        renderScreen('/alarm-history?from=2026-09-21&to=2026-09-26&groups_sort=severity%3Adesc');
        await waitFor(() => expect(server.requests()).toHaveLength(1));

        expect(lastQuery()).toMatchObject({ since: '2026-09-21T00:00:00+03:00', until: '2026-09-27T00:00:00+03:00', sort: 'severity' });
        expect(screen.getByTestId('filter-to')).toHaveValue('2026-09-26');
    });

    it('shows a rejected sort or search (HTTP 400) in plain words', async () => {
        server.failNext(400, 'invalid_query');
        renderScreen('/alarm-history?groups_q=sweep');

        expect(await screen.findByTestId('alarm-list-filter-error')).toHaveTextContent('Search is limited to 60 characters.');
    });

    it('restores the raw view with its own sort and search from the URL', async () => {
        renderScreen('/alarm-history?view=all&alerts_sort=severity%3Adesc&alerts_q=xom&groups_q=other');
        await waitFor(() => expect(screen.getByTestId('alarm-raw-table')).toBeInTheDocument());

        expect(screen.getByTestId('show-all-input')).toBeChecked();
        expect(screen.getByTestId('alarm-search-input')).toHaveValue('xom');
        expect(lastQuery()).toMatchObject({ mode: 'raw', sort: 'severity', dir: 'desc', q: 'xom' });
        expect(rawIds()).toHaveLength(12);
        expect(within(screen.getByTestId('alarm-raw-table')).getByRole('columnheader', { name: 'Severity' })).toHaveAttribute(
            'aria-sort',
            'descending'
        );
    });

    it('keeps the view switch and filters in the URL', async () => {
        renderScreen();
        await waitFor(() => expect(groupKeys()).toHaveLength(4));

        await user.click(screen.getByTestId('show-all-input'));
        await user.click(screen.getByTestId('filter-severity-notice'));
        await waitFor(() => expect(location()).toBe('/alarm-history?view=all&severity=notice'));

        await user.type(screen.getByTestId('filter-symbol'), 'xom');
        act(() => {
            jest.advanceTimersByTime(500);
        });
        await waitFor(() => expect(location()).toBe('/alarm-history?view=all&severity=notice&symbol=XOM'));
    });

    it('filters an expanded group by the search too, paging it by offset', async () => {
        renderScreen('/alarm-history?groups_q=sweep');
        await user.click(await screen.findByTestId('group-toggle-XOM|liquidity_sweep'));

        const table = await screen.findByTestId('group-alerts-XOM|liquidity_sweep-table');
        expect(within(table).getAllByTestId(/^alert-row-/)).toHaveLength(5);
        expect(lastQuery()).toMatchObject({ symbol: 'XOM', mode: 'raw', q: 'sweep' });
        expect(lastQuery().sort).toBeUndefined();
        const pinned = lastQuery().until;
        expectPinnedAt(pinned, NOW);

        await user.click(screen.getByTestId('group-alerts-XOM|liquidity_sweep-footer-load-older'));
        await waitFor(() => expect(within(table).getAllByTestId(/^alert-row-/)).toHaveLength(10));
        expect(lastQuery()).toMatchObject({ offset: '5', until: pinned });
    });

    it('opens Stock Detail with "Back to Alarm History" and the sorted, searched view', async () => {
        renderScreen('/alarm-history?groups_sort=symbol%3Aasc&groups_q=xom');
        await waitFor(() => expect(groupKeys()).toEqual(['XOM|liquidity_sweep']));

        await user.click(screen.getByTestId('symbol-link-XOM'));

        expect(JSON.parse(screen.getByTestId('detail').textContent!)).toEqual({
            pathname: '/candidates/XOM',
            hash: '#classical-signals',
            state: { from: '/alarm-history?groups_sort=symbol%3Aasc&groups_q=xom', fromLabel: 'Alarm History' },
        });
    });
});
