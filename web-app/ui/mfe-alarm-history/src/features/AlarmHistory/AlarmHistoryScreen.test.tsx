import { act, render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { FiredAlert } from '@/api';
import { HostModeProvider } from '@/providers/HostModeContext';
import { CAVEAT, makeAlert, TYPE_LABELS } from '@/test-utils/alerts';
import { createAlertsServer } from '@/test-utils/alertsServer';
import AlarmHistoryScreen from './AlarmHistoryScreen';

// 22:00 in Athens (jest.config.js pins TZ): the default range is Sep 21–27.
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

const renderScreen = (props: { pageSize?: number; refreshIntervalMs?: number } = {}, hosted = true) =>
    render(
        <MemoryRouter>
            <HostModeProvider hosted={hosted}>
                <AlarmHistoryScreen {...props} />
            </HostModeProvider>
        </MemoryRouter>
    );

const setVisibility = (state: 'visible' | 'hidden') => {
    Object.defineProperty(document, 'visibilityState', { configurable: true, get: () => state });
    document.dispatchEvent(new Event('visibilitychange'));
};

const groupRows = () => screen.queryAllByTestId(/^group-row-/);
const rawRows = () => within(screen.getByTestId('alarm-raw-table')).getAllByTestId(/^alert-row-/);

describe('AlarmHistoryScreen', () => {
    let server: ReturnType<typeof createAlertsServer>;
    let user: ReturnType<typeof userEvent.setup>;

    beforeEach(() => {
        jest.useFakeTimers({ now: NOW });
        user = userEvent.setup({ advanceTimers: jest.advanceTimersByTime });
        server = createAlertsServer(dataset());
        (global as any).fetch = server.fetch;
        Object.defineProperty(document, 'visibilityState', { configurable: true, get: () => 'visible' });
    });

    afterEach(() => {
        jest.useRealTimers();
    });

    const lastQuery = () => Object.fromEntries(server.requests()[server.requests().length - 1]);

    it('loads grouped by default for the last 7 local days, newest group first', async () => {
        renderScreen();
        expect(screen.getByTestId('alarm-list-loading')).toBeInTheDocument();

        await waitFor(() => expect(groupRows()).toHaveLength(4));
        expect(lastQuery()).toEqual({
            since: '2026-09-21T00:00:00+03:00',
            until: '2026-09-28T00:00:00+03:00',
            limit: '100',
            mode: 'grouped',
        });
        expect(groupRows().map(r => r.getAttribute('data-testid'))).toEqual([
            'group-row-AAPL|vix_elevated',
            'group-row-TIAUSDT|liquidity_sweep',
            'group-row-XOM|liquidity_sweep',
            'group-row-MSFT|bb_squeeze',
        ]);
        expect(screen.getByTestId('range-default-note')).toHaveTextContent('The last 7 days, today included (default)');
        expect(screen.getByTestId('grouped-footer')).toHaveTextContent('Showing all 4 groups');
    });

    it('shows a group with its label, latest severity and message, and the repeat span', async () => {
        renderScreen();
        const row = await screen.findByTestId('group-row-XOM|liquidity_sweep');

        expect(row).toHaveTextContent('Liquidity sweep');
        expect(within(row).getByTestId('severity-badge')).toHaveTextContent('notice');
        expect(row).toHaveTextContent('Liquidity sweep detected (4 sweeps)');
        expect(screen.getByTestId('group-repeats-XOM|liquidity_sweep')).toHaveTextContent(/^×12 · Sep 26, 1:30 AM – Sep 27, 9:30 PM$/);
    });

    it('renders the caveat verbatim from the response and the records note', async () => {
        renderScreen();

        expect(await screen.findByTestId('alarm-caveat')).toHaveTextContent(CAVEAT);
        expect(screen.getByTestId('records-note')).toHaveTextContent(
            "Records start Sep 26, 2026, 1:30 AM GMT+3. Only alerts actually posted to Discord are recorded; the momentum screener's alerts are not included."
        );
    });

    describe('type labels, onsets and bar dates', () => {
        const onsetDataset = () => [
            makeAlert({ id: 1, symbol: 'XOM', alert_type: 'liquidity_sweep', fired_at: minutesAgo(600), bar_date: null }),
            makeAlert({ id: 2, symbol: 'XOM', alert_type: 'liquidity_sweep', fired_at: minutesAgo(60), bar_date: '2026-09-26' }),
            makeAlert({ id: 3, symbol: 'AAPL', alert_type: 'macd_bull_cross', fired_at: minutesAgo(30), bar_date: '2026-09-27' }),
        ];
        const serve = (options: Parameters<typeof createAlertsServer>[1] = {}) => {
            server = createAlertsServer(onsetDataset(), options);
            (global as any).fetch = server.fetch;
        };
        const chipLabels = () => within(screen.getByTestId('filter-types')).getAllByRole('button').map(b => b.textContent);

        it('labels types only from the API, showing an unlabelled type as its id', async () => {
            serve({ typeLabels: { ...TYPE_LABELS, liquidity_sweep: 'Sweep of liquidity' } });
            renderScreen();
            const xom = await screen.findByTestId('group-row-XOM|liquidity_sweep');

            expect(chipLabels()).toEqual(['macd_bull_cross', 'Sweep of liquidity']);
            expect(within(xom).getByText('Sweep of liquidity')).toHaveAttribute('data-column', 'type');
            expect(screen.getByTestId('group-toggle-XOM|liquidity_sweep')).toHaveAccessibleName('Show the 2 XOM Sweep of liquidity alerts');
            expect(within(screen.getByTestId('group-row-AAPL|macd_bull_cross')).getByText('macd_bull_cross')).toHaveAttribute(
                'data-column',
                'type'
            );

            await user.click(screen.getByTestId('group-toggle-XOM|liquidity_sweep'));
            expect(await screen.findByRole('table', { name: 'XOM Sweep of liquidity alerts, newest first' })).toBeInTheDocument();

            await user.click(screen.getByTestId('show-all-input'));
            await waitFor(() => expect(rawRows()).toHaveLength(3));
            expect(within(screen.getByTestId('alert-row-2')).getByText('Sweep of liquidity')).toBeInTheDocument();
            expect(within(screen.getByTestId('alert-row-3')).getByText('macd_bull_cross')).toBeInTheDocument();
        });

        it('shows the onset bar under the fired time in raw rows, and nothing extra for alerts from before', async () => {
            serve();
            renderScreen();
            await screen.findByTestId('group-row-XOM|liquidity_sweep');
            expect(screen.queryAllByTestId(/^alert-bar-/)).toHaveLength(0);

            await user.click(screen.getByTestId('show-all-input'));
            await waitFor(() => expect(rawRows()).toHaveLength(3));

            expect(screen.getByTestId('alert-bar-2')).toHaveTextContent(/^bar Sep 26$/);
            expect(screen.getByTestId('alert-bar-3')).toHaveTextContent(/^bar Sep 27$/);
            expect(within(screen.getByTestId('alert-bar-2')).getByText('Sep 26')).toHaveAttribute('datetime', '2026-09-26');
            expect(screen.queryByTestId('alert-bar-1')).not.toBeInTheDocument();
        });

        it('shows the onset bar in an expanded group\'s rows', async () => {
            serve();
            renderScreen();

            await user.click(await screen.findByTestId('group-toggle-XOM|liquidity_sweep'));
            const table = await screen.findByTestId('group-alerts-XOM|liquidity_sweep-table');

            expect(within(table).getAllByTestId(/^alert-row-/)).toHaveLength(2);
            expect(within(table).getByTestId('alert-bar-2')).toHaveTextContent('bar Sep 26');
            expect(within(table).queryByTestId('alert-bar-1')).not.toBeInTheDocument();
        });

        it('dates the switch to onset alerts once the API reports it', async () => {
            serve({ onsetsSince: '2026-09-27T05:00:00Z' });
            renderScreen();

            expect(await screen.findByTestId('onsets-note')).toHaveTextContent(
                'Until Sep 27, 2026, 8:00 AM GMT+3, alerts re-posted an ongoing condition every few hours while it stayed true. From then on, each alert marks an onset: the condition started on that bar after at least 5 sessions without it.'
            );
        });

        it('says onset alerts are still to come while onsets_since is null', async () => {
            serve();
            renderScreen();

            expect(await screen.findByTestId('onsets-note')).toHaveTextContent(
                'Alerts so far re-posted an ongoing condition every few hours while it stayed true; onset-only alerts start with the next release.'
            );
        });

        it('keeps working against an API from before labels, onsets and bar dates', async () => {
            serve({ legacy: true });
            renderScreen();
            await screen.findByTestId('group-row-XOM|liquidity_sweep');

            expect(chipLabels()).toEqual(['liquidity_sweep', 'macd_bull_cross']);
            expect(screen.getByTestId('onsets-note')).toHaveTextContent('onset-only alerts start with the next release.');

            await user.click(screen.getByTestId('show-all-input'));
            await waitFor(() => expect(rawRows()).toHaveLength(3));
            expect(screen.queryAllByTestId(/^alert-bar-/)).toHaveLength(0);
            expect(within(screen.getByTestId('alert-row-2')).getByText('liquidity_sweep')).toBeInTheDocument();
        });
    });

    it('says nothing has been recorded when records_start is null', async () => {
        server = createAlertsServer([]);
        (global as any).fetch = server.fetch;
        renderScreen();

        expect(await screen.findByTestId('records-note')).toHaveTextContent('No alerts have been recorded yet.');
        expect(screen.getByTestId('alarm-list-empty')).toHaveTextContent('No alerts in this period');
    });

    it('links equity rows to Stock Detail classical signals, not crypto or market-wide rows', async () => {
        renderScreen();
        await screen.findByTestId('group-row-XOM|liquidity_sweep');

        expect(screen.getByTestId('symbol-link-XOM')).toHaveAttribute('href', '/candidates/XOM#classical-signals');
        expect(screen.getByTestId('symbol-link-MSFT')).toHaveAttribute('href', '/candidates/MSFT#classical-signals');
        expect(screen.queryByTestId('symbol-link-TIAUSDT')).not.toBeInTheDocument();
        expect(screen.getByTestId('symbol-text-TIAUSDT')).toBeInTheDocument();
        expect(screen.queryByTestId('symbol-link-AAPL')).not.toBeInTheDocument();
        expect(screen.getByTestId('group-row-AAPL|vix_elevated')).toHaveTextContent('market-wide');
    });

    it('links nothing standalone, where spog routes do not exist', async () => {
        renderScreen({}, false);
        await screen.findByTestId('group-row-XOM|liquidity_sweep');

        expect(screen.queryAllByTestId(/^symbol-link-/)).toHaveLength(0);
    });

    it('expands a group to its alerts with the same filters and loads older within it', async () => {
        renderScreen();
        const toggle = await screen.findByTestId('group-toggle-XOM|liquidity_sweep');
        expect(screen.queryByTestId('group-toggle-AAPL|vix_elevated')).not.toBeInTheDocument();
        expect(toggle).toHaveAttribute('aria-expanded', 'false');

        await user.click(toggle);

        const table = await screen.findByTestId('group-alerts-XOM|liquidity_sweep-table');
        expect(toggle).toHaveAttribute('aria-expanded', 'true');
        expect(document.getElementById(toggle.getAttribute('aria-controls')!)).toContainElement(table);
        expect(within(table).getAllByTestId(/^alert-row-/)).toHaveLength(5);
        expect(lastQuery()).toEqual({
            symbol: 'XOM',
            alert_type: 'liquidity_sweep',
            since: '2026-09-21T00:00:00+03:00',
            until: '2026-09-28T00:00:00+03:00',
            limit: '5',
            mode: 'raw',
        });
        expect(within(table).getAllByTestId('symbol-link-XOM')).toHaveLength(5);

        await user.click(screen.getByTestId('group-alerts-XOM|liquidity_sweep-footer-load-older'));
        await waitFor(() => expect(within(table).getAllByTestId(/^alert-row-/)).toHaveLength(10));
        expect(lastQuery().before).toBe('5');

        await user.click(screen.getByTestId('group-alerts-XOM|liquidity_sweep-footer-load-older'));
        await waitFor(() => expect(within(table).getAllByTestId(/^alert-row-/)).toHaveLength(12));
        expect(screen.getByTestId('group-alerts-XOM|liquidity_sweep-footer')).toHaveTextContent('Showing all 12 alerts');

        await user.click(toggle);
        expect(screen.queryByTestId('group-alerts-XOM|liquidity_sweep-table')).not.toBeInTheDocument();
    });

    it('pages groups with Load older, appending', async () => {
        renderScreen({ pageSize: 2 });
        await waitFor(() => expect(groupRows()).toHaveLength(2));
        expect(screen.getByTestId('grouped-footer')).toHaveTextContent('Showing the newest 2 groups');

        await user.click(screen.getByTestId('grouped-footer-load-older'));

        await waitFor(() => expect(groupRows()).toHaveLength(4));
        expect(lastQuery()).toMatchObject({ mode: 'grouped', limit: '2', before: expect.any(String) });
        expect(groupRows()[3]).toHaveAttribute('data-testid', 'group-row-MSFT|bb_squeeze');
        expect(screen.queryByTestId('grouped-footer-load-older')).not.toBeInTheDocument();
    });

    it('"Show every alert" switches to the raw list, which pages with Load older', async () => {
        renderScreen({ pageSize: 7 });
        await waitFor(() => expect(groupRows().length).toBeGreaterThan(0));

        await user.click(screen.getByTestId('show-all-input'));

        await waitFor(() => expect(rawRows()).toHaveLength(7));
        expect(screen.getByRole('heading', { name: 'Every alert' })).toBeInTheDocument();
        expect(lastQuery()).toMatchObject({ mode: 'raw', limit: '7' });
        const firstPage = rawRows().map(r => r.getAttribute('data-testid'));

        await user.click(screen.getByTestId('raw-footer-load-older'));
        await waitFor(() => expect(rawRows()).toHaveLength(14));
        expect(rawRows().slice(0, 7).map(r => r.getAttribute('data-testid'))).toEqual(firstPage);

        await user.click(screen.getByTestId('raw-footer-load-older'));
        await waitFor(() => expect(rawRows()).toHaveLength(18));
        expect(screen.getByTestId('raw-footer')).toHaveTextContent('Showing all 18 alerts');

        const crypto = within(screen.getByTestId('alarm-raw-table')).getAllByTestId('symbol-text-TIAUSDT')[0];
        expect(crypto.closest('a')).toBeNull();
    });

    it('filters by type and severity (multi), resetting pagination', async () => {
        renderScreen({ pageSize: 2 });
        await waitFor(() => expect(groupRows()).toHaveLength(2));
        await user.click(screen.getByTestId('grouped-footer-load-older'));
        await waitFor(() => expect(groupRows()).toHaveLength(4));

        await user.click(screen.getByTestId('filter-type-liquidity_sweep'));

        await waitFor(() => expect(groupRows()).toHaveLength(2));
        expect(screen.getByTestId('filter-type-liquidity_sweep')).toHaveAttribute('aria-pressed', 'true');
        expect(lastQuery()).toMatchObject({ alert_type: 'liquidity_sweep' });
        expect(lastQuery().before).toBeUndefined();

        await user.click(screen.getByTestId('filter-type-bb_squeeze'));
        await waitFor(() => expect(lastQuery().alert_type).toBe('bb_squeeze,liquidity_sweep'));

        await user.click(screen.getByTestId('filter-severity-info'));
        await waitFor(() => expect(groupRows()).toHaveLength(1));
        expect(lastQuery()).toMatchObject({ severity: 'info', alert_type: 'bb_squeeze,liquidity_sweep' });
        expect(groupRows()[0]).toHaveAttribute('data-testid', 'group-row-MSFT|bb_squeeze');
    });

    it('offers the type options from the response', async () => {
        renderScreen();
        await screen.findByTestId('group-row-XOM|liquidity_sweep');

        expect(within(screen.getByTestId('filter-types')).getAllByRole('button').map(b => b.textContent)).toEqual([
            'Bollinger squeeze',
            'Liquidity sweep',
            'VIX elevated',
        ]);
    });

    it('filters by symbol after typing (debounced), upper-cased', async () => {
        renderScreen();
        await screen.findByTestId('group-row-XOM|liquidity_sweep');

        await user.type(screen.getByTestId('filter-symbol'), 'xom');
        await act(async () => {
            jest.advanceTimersByTime(500);
        });

        await waitFor(() => expect(groupRows()).toHaveLength(1));
        expect(lastQuery().symbol).toBe('XOM');
    });

    it('shows a rejected symbol in plain words, inline', async () => {
        renderScreen();
        await screen.findByTestId('group-row-XOM|liquidity_sweep');

        await user.type(screen.getByTestId('filter-symbol'), 'bad$ymbol{enter}');

        expect(await screen.findByTestId('alarm-list-filter-error')).toHaveTextContent(
            "That symbol isn't valid. Use letters, digits, dots or dashes, e.g. AAPL or BTCUSDT."
        );
        expect(screen.queryByTestId('api-error-state')).not.toBeInTheDocument();
    });

    it('sends local-day bounds for a chosen range and shows the neutral empty state', async () => {
        renderScreen();
        await screen.findByTestId('group-row-XOM|liquidity_sweep');

        await user.clear(screen.getByTestId('filter-from'));
        await user.type(screen.getByTestId('filter-from'), '2026-09-01');
        await user.clear(screen.getByTestId('filter-to'));
        await user.type(screen.getByTestId('filter-to'), '2026-09-10');

        const empty = await screen.findByTestId('alarm-list-empty');
        expect(empty).toHaveTextContent('No alerts in this period');
        expect(empty).toHaveTextContent('Nothing is recorded before Sep 26, 2026, 1:30 AM GMT+3, so this period has no record at all.');
        expect(lastQuery()).toMatchObject({ since: '2026-09-01T00:00:00+03:00', until: '2026-09-11T00:00:00+03:00' });

        await user.click(screen.getByTestId('range-reset'));
        await waitFor(() => expect(groupRows()).toHaveLength(4));
        expect(screen.getByTestId('range-default-note')).toBeInTheDocument();
    });

    it('shows the empty state without the record note for a period after records start', async () => {
        renderScreen();
        await screen.findByTestId('group-row-XOM|liquidity_sweep');
        await user.click(screen.getByTestId('filter-severity-warning'));
        await user.click(screen.getByTestId('filter-type-bb_squeeze'));
        await user.clear(screen.getByTestId('filter-from'));
        await user.type(screen.getByTestId('filter-from'), '2026-09-27');

        const empty = await screen.findByTestId('alarm-list-empty');
        expect(empty).toHaveTextContent(/^No alerts in this period$/);
    });

    it('shows ApiErrorState on failure and recovers with Retry', async () => {
        server.failNext(503, 'database_unavailable');
        renderScreen();

        expect(await screen.findByTestId('api-error-state')).toHaveTextContent('The alert database is unavailable right now.');

        await user.click(screen.getByRole('button', { name: 'Retry' }));
        await waitFor(() => expect(groupRows()).toHaveLength(4));
    });

    it('shows a Load older failure inline and keeps the rows', async () => {
        renderScreen({ pageSize: 2 });
        await waitFor(() => expect(groupRows()).toHaveLength(2));
        server.failNext(503, 'database_unavailable');

        await user.click(screen.getByTestId('grouped-footer-load-older'));

        expect(await screen.findByTestId('grouped-footer-older-error')).toHaveTextContent(
            'The alert database is unavailable right now. Try Load older again.'
        );
        expect(groupRows()).toHaveLength(2);
    });

    describe('refresh', () => {
        it('shows the last-checked time and re-reads the first page every 60 s while visible', async () => {
            renderScreen();
            await screen.findByTestId('group-row-XOM|liquidity_sweep');
            expect(screen.getByTestId('last-checked')).toHaveTextContent('Last checked 10:00:00 PM');
            const before = server.requests().length;

            server.add(makeAlert({ id: 999, symbol: 'NVDA', exchange_type: 'equity', alert_type: 'rsi_overbought', fired_at: minutesAgo(-1) }));
            await act(async () => {
                jest.advanceTimersByTime(60_000);
            });

            await screen.findByTestId('group-row-NVDA|rsi_overbought');
            expect(server.requests().length).toBe(before + 1);
            expect(lastQuery().before).toBeUndefined();
            expect(screen.getByTestId('last-checked')).toHaveTextContent('Last checked 10:01:00 PM');
            expect(screen.getByTestId('filter-types')).toHaveTextContent('RSI overbought');
        });

        it('keeps rows loaded with Load older and expanded groups', async () => {
            renderScreen({ pageSize: 2 });
            await waitFor(() => expect(groupRows()).toHaveLength(2));
            await user.click(screen.getByTestId('grouped-footer-load-older'));
            await waitFor(() => expect(groupRows()).toHaveLength(4));
            await user.click(screen.getByTestId('group-toggle-XOM|liquidity_sweep'));
            await screen.findByTestId('group-alerts-XOM|liquidity_sweep-table');

            server.add(makeAlert({ id: 998, symbol: 'MSFT', exchange_type: 'equity', alert_type: 'bb_squeeze', severity: 'info', fired_at: minutesAgo(-1) }));
            await act(async () => {
                jest.advanceTimersByTime(60_000);
            });

            await waitFor(() => expect(groupRows()[0]).toHaveAttribute('data-testid', 'group-row-MSFT|bb_squeeze'));
            expect(groupRows().map(r => r.getAttribute('data-testid'))).toEqual([
                'group-row-MSFT|bb_squeeze',
                'group-row-AAPL|vix_elevated',
                'group-row-TIAUSDT|liquidity_sweep',
                'group-row-XOM|liquidity_sweep',
            ]);
            expect(screen.getByTestId('group-repeats-MSFT|bb_squeeze')).toHaveTextContent(/^×3/);
            expect(screen.getByTestId('group-toggle-XOM|liquidity_sweep')).toHaveAttribute('aria-expanded', 'true');
            expect(screen.getByTestId('group-alerts-XOM|liquidity_sweep-table')).toBeInTheDocument();
        });

        it('keeps raw rows loaded with Load older and prepends new ones', async () => {
            renderScreen({ pageSize: 5 });
            await waitFor(() => expect(groupRows().length).toBeGreaterThan(0));
            await user.click(screen.getByTestId('show-all-input'));
            await waitFor(() => expect(rawRows()).toHaveLength(5));
            await user.click(screen.getByTestId('raw-footer-load-older'));
            await waitFor(() => expect(rawRows()).toHaveLength(10));

            server.add(makeAlert({ id: 997, symbol: 'NVDA', exchange_type: 'equity', alert_type: 'rsi_overbought', fired_at: minutesAgo(-1) }));
            await act(async () => {
                jest.advanceTimersByTime(60_000);
            });

            await waitFor(() => expect(rawRows()).toHaveLength(11));
            expect(rawRows()[0]).toHaveAttribute('data-testid', 'alert-row-997');
            await user.click(screen.getByTestId('raw-footer-load-older'));
            await waitFor(() => expect(rawRows()).toHaveLength(16));
        });

        it('does not poll while hidden, and refreshes at once when the tab becomes visible', async () => {
            renderScreen();
            await screen.findByTestId('group-row-XOM|liquidity_sweep');
            const before = server.requests().length;

            act(() => setVisibility('hidden'));
            await act(async () => {
                jest.advanceTimersByTime(5 * 60_000);
            });
            expect(server.requests().length).toBe(before);

            await act(async () => {
                setVisibility('visible');
            });
            await waitFor(() => expect(server.requests().length).toBe(before + 1));
        });

        it('keeps the rows and says so when a refresh fails', async () => {
            renderScreen();
            await screen.findByTestId('group-row-XOM|liquidity_sweep');
            server.failNext(503, 'database_unavailable');

            await act(async () => {
                jest.advanceTimersByTime(60_000);
            });

            expect(await screen.findByTestId('refresh-error')).toHaveTextContent("couldn't check for new alerts");
            expect(groupRows()).toHaveLength(4);
            expect(screen.getByTestId('last-checked')).toHaveTextContent('Last checked 10:00:00 PM');
        });

        it('stops polling after unmount', async () => {
            const { unmount } = renderScreen();
            await screen.findByTestId('group-row-XOM|liquidity_sweep');
            const before = server.requests().length;

            unmount();
            await act(async () => {
                jest.advanceTimersByTime(3 * 60_000);
                document.dispatchEvent(new Event('visibilitychange'));
            });

            expect(server.requests().length).toBe(before);
        });
    });

    it('collapses the alerts card from its header, keeping the view switch outside the toggle', async () => {
        renderScreen();
        await screen.findByTestId('group-row-XOM|liquidity_sweep');
        const toggle = screen.getByRole('button', { name: /Alerts by symbol and type/ });

        await user.click(screen.getByTestId('show-all-input'));
        expect(toggle).toHaveAttribute('aria-expanded', 'true');

        await user.click(screen.getByRole('button', { name: /Every alert/ }));
        expect(screen.getByRole('button', { name: /Every alert/ })).toHaveAttribute('aria-expanded', 'false');
        expect(screen.queryByTestId('alarm-raw-table')).not.toBeInTheDocument();
        expect(screen.getByTestId('show-all')).toBeInTheDocument();
    });
});
