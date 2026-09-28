import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter, Route, Routes, useLocation } from 'react-router-dom';
import { fetchComputedSymbols } from '@/api';
import { HostModeProvider } from '@/providers/HostModeContext';
import { makeComputedSymbol, makeComputedSymbols, makeManualComputed } from '@/test-utils/fixtures';
import ComputedSymbolsPage from '.';

jest.mock('@/api/tracking/trackingApi', () => ({
    fetchComputedSymbols: jest.fn(),
    requestCompute: jest.fn(),
    stopCompute: jest.fn(),
}));

const fetchMock = fetchComputedSymbols as jest.MockedFunction<typeof fetchComputedSymbols>;

const Location = () => {
    const { pathname, search } = useLocation();
    return <span data-testid="location">{`${pathname}${search}`}</span>;
};
const Detail = () => <span data-testid="detail-state">{JSON.stringify(useLocation().state)}</span>;

/** As served: alphabetical. */
const ROWS = [
    makeComputedSymbol({ symbol: 'AMZN', computed_at: '2026-09-27T14:27:37Z' }),
    makeComputedSymbol({ symbol: 'BTCUSDT', name: 'BTC', asset_type: 'crypto', computed_at: '2026-09-28T06:24:25Z' }),
    makeComputedSymbol({ symbol: 'CRDO', reasons: ['watchlist'], computed_at: '2026-09-28T06:24:25Z' }),
    makeComputedSymbol({ symbol: 'NEWX', reasons: ['candidate'], state: 'scheduled', computed_at: null }),
    makeManualComputed('failed', { symbol: 'ZZZ', asset_type: 'equity', computed_at: '2026-09-26T10:00:00Z' }),
];

const renderPage = async ({ hosted = true, url = '/computed-symbols' } = {}) => {
    fetchMock.mockResolvedValue(makeComputedSymbols(ROWS));
    render(
        <HostModeProvider hosted={hosted}>
            <MemoryRouter initialEntries={[url]}>
                <Location />
                <Routes>
                    <Route path="/computed-symbols/*" element={<ComputedSymbolsPage />} />
                    <Route path="/candidates/:symbol" element={<Detail />} />
                </Routes>
            </MemoryRouter>
        </HostModeProvider>
    );
    await screen.findByTestId('computed-table');
};

const rowSymbols = () => screen.queryAllByTestId(/^computed-row-/).map(el => el.getAttribute('data-testid')!.replace('computed-row-', ''));
const location = () => screen.getByTestId('location').textContent;

beforeEach(() => window.sessionStorage.clear());

describe('ComputedSymbolsPage table view', () => {
    it('defaults to newest last computed first, ties by symbol, "not yet" last', async () => {
        await renderPage();

        expect(rowSymbols()).toEqual(['BTCUSDT', 'CRDO', 'AMZN', 'ZZZ', 'NEWX']);
        expect(screen.getByRole('columnheader', { name: 'Last computed' })).toHaveAttribute('aria-sort', 'descending');
        expect(screen.getByTestId('computed-sort-label')).toHaveTextContent('Sorted by Last computed, newest first');
        expect(location()).toBe('/computed-symbols');
    });

    it('keeps "not yet" last when Last computed is ascending', async () => {
        await renderPage();

        await userEvent.click(screen.getByRole('button', { name: 'Last computed' }));
        expect(rowSymbols()).toEqual(['ZZZ', 'AMZN', 'BTCUSDT', 'CRDO', 'NEWX']);
        expect(location()).toBe('/computed-symbols?computed_sort=computed_at%3Aasc');
    });

    it('sorts State attention first (failed … scheduled … computed), then reversed', async () => {
        await renderPage();

        await userEvent.click(screen.getByRole('button', { name: 'State' }));
        expect(rowSymbols()).toEqual(['ZZZ', 'NEWX', 'AMZN', 'BTCUSDT', 'CRDO']);
        await userEvent.click(screen.getByRole('button', { name: 'State' }));
        expect(rowSymbols()).toEqual(['AMZN', 'BTCUSDT', 'CRDO', 'NEWX', 'ZZZ']);
    });

    it('sorts every column but the Stop computing action', async () => {
        await renderPage();

        for (const name of ['Symbol', 'Name', 'Type', 'Reasons', 'State', 'Last computed']) {
            expect(screen.getByRole('button', { name })).toBeInTheDocument();
        }
        expect(within(screen.getByRole('columnheader', { name: 'Actions' })).queryByRole('button')).not.toBeInTheDocument();
    });

    it('searches reasons and states as shown', async () => {
        await renderPage({ url: '/computed-symbols?computed_q=failed' });
        expect(rowSymbols()).toEqual(['ZZZ']);

        await userEvent.clear(screen.getByTestId('computed-search-input'));
        await userEvent.type(screen.getByTestId('computed-search-input'), "today's candidate");
        await waitFor(() => expect(rowSymbols()).toEqual(['NEWX']));
        expect(screen.getByTestId('computed-search-count')).toHaveTextContent('1 of 5 rows');
    });

    it('links stocks to Stock Detail with "Back to Computed Symbols"; crypto is text', async () => {
        await renderPage({ url: '/computed-symbols?computed_sort=symbol%3Aasc' });

        expect(within(screen.getByTestId('computed-row-BTCUSDT')).queryByRole('link')).not.toBeInTheDocument();
        await userEvent.click(within(screen.getByTestId('computed-row-CRDO')).getByRole('link', { name: 'CRDO' }));
        expect(JSON.parse(screen.getByTestId('detail-state').textContent!)).toEqual({
            from: '/computed-symbols?computed_sort=symbol%3Aasc',
            fromLabel: 'Computed Symbols',
        });
    });
});
