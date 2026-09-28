import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter, Route, Routes, useLocation } from 'react-router-dom';
import { fetchComputedSymbols, fetchFollowedSymbols, searchDirectory, searchSymbols } from '@/api';
import { HostModeProvider } from '@/providers/HostModeContext';
import {
    makeComputedSymbols,
    makeDirectorySearch,
    makeFollowedSymbols,
    makeManualComputed,
    makeSymbolSearch,
} from '@/test-utils/fixtures';
import FollowedSymbolsPage from '.';

jest.mock('@/api/tracking/trackingApi', () => ({
    fetchFollowedSymbols: jest.fn(),
    followSymbol: jest.fn(),
    unfollowSymbol: jest.fn(),
    searchDirectory: jest.fn(),
    fetchComputedSymbols: jest.fn(),
    requestCompute: jest.fn(),
    stopCompute: jest.fn(),
}));
jest.mock('@/api/watchlist/watchlistApi', () => ({ searchSymbols: jest.fn() }));

const fetchFollowedMock = fetchFollowedSymbols as jest.MockedFunction<typeof fetchFollowedSymbols>;
const directoryMock = searchDirectory as jest.MockedFunction<typeof searchDirectory>;
const universeMock = searchSymbols as jest.MockedFunction<typeof searchSymbols>;
const fetchComputedMock = fetchComputedSymbols as jest.MockedFunction<typeof fetchComputedSymbols>;

const Location = () => {
    const { pathname, search } = useLocation();
    return <span data-testid="location">{`${pathname}${search}`}</span>;
};
const Detail = () => <span data-testid="detail-state">{JSON.stringify(useLocation().state)}</span>;

const renderPage = async ({ hosted = true, url = '/followed-symbols' } = {}) => {
    render(
        <HostModeProvider hosted={hosted}>
            <MemoryRouter initialEntries={[url]}>
                <Location />
                <Routes>
                    <Route path="/followed-symbols/*" element={<FollowedSymbolsPage />} />
                    <Route path="/candidates/:symbol" element={<Detail />} />
                </Routes>
            </MemoryRouter>
        </HostModeProvider>
    );
    await screen.findByTestId('followed-table');
};

const rowSymbols = () => screen.queryAllByTestId(/^followed-row-/).map(el => el.getAttribute('data-testid')!.replace('followed-row-', ''));
const location = () => screen.getByTestId('location').textContent;
const SEED = '2026-09-27T13:31:25Z';

beforeEach(() => {
    window.sessionStorage.clear();
    fetchFollowedMock.mockResolvedValue(
        makeFollowedSymbols([
            { symbol: 'SPY', added_at: SEED, source: 'env_seed' },
            { symbol: 'OPTX', name: 'SYNTEC OPTICS', asset_type: 'equity', added_at: '2026-09-28T06:26:58Z' },
            { symbol: 'BTCUSDT', name: 'BTC', asset_type: 'crypto', listing: 'crypto', source: 'env_seed', added_at: SEED },
            { symbol: 'AMZN', name: 'AMAZON.COM INC', asset_type: 'equity', source: 'env_seed', added_at: SEED },
        ])
    );
    fetchComputedMock.mockResolvedValue(makeComputedSymbols([]));
    universeMock.mockResolvedValue(makeSymbolSearch('vg', ['VG', 'VGZ']));
    directoryMock.mockResolvedValue(
        makeDirectorySearch('dia', [{}, { symbol: 'DIAUSDT', name: 'DIA', type: 'spot', mic: null, asset_type: 'crypto', source: 'binance_spot' }])
    );
});

describe('FollowedSymbolsPage table view', () => {
    it('defaults to newest first, ties (every .env seed) by symbol', async () => {
        await renderPage();

        expect(rowSymbols()).toEqual(['OPTX', 'AMZN', 'BTCUSDT', 'SPY']);
        expect(screen.getByTestId('followed-sort-label')).toHaveTextContent('Sorted by Added, newest first');
    });

    it('sorts every column but Unfollow, and by Compute state', async () => {
        fetchComputedMock.mockResolvedValue(
            makeComputedSymbols([makeManualComputed('failed', { symbol: 'SPY' }), makeManualComputed('computed', { symbol: 'AMZN' })])
        );
        await renderPage();
        await screen.findByTestId('compute-state-SPY');

        for (const name of ['Symbol', 'Name', 'Type', 'Listing', 'Source', 'Added', 'Compute']) {
            expect(screen.getByRole('button', { name })).toBeInTheDocument();
        }
        expect(within(screen.getByRole('columnheader', { name: 'Actions' })).queryByRole('button')).not.toBeInTheDocument();

        await userEvent.click(screen.getByRole('button', { name: 'Compute' }));
        expect(rowSymbols()).toEqual(['SPY', 'AMZN', 'BTCUSDT', 'OPTX']);
        expect(location()).toBe('/followed-symbols?followed_sort=compute%3Aasc');

        await userEvent.click(screen.getByRole('button', { name: 'Type' }));
        expect(rowSymbols()).toEqual(['BTCUSDT', 'AMZN', 'OPTX', 'SPY']);
    });

    it('searches the shown text (type label) with a row count', async () => {
        await renderPage();

        await userEvent.type(screen.getByTestId('followed-search-input'), 'crypto');
        await waitFor(() => expect(rowSymbols()).toEqual(['BTCUSDT']));
        expect(screen.getByTestId('followed-search-count')).toHaveTextContent('1 of 4 rows');
        expect(location()).toBe('/followed-symbols?followed_q=crypto');
    });

    it('links stocks and funds to Stock Detail with "Back to Followed Symbols"; crypto is text', async () => {
        await renderPage({ url: '/followed-symbols?followed_q=s' });

        expect(within(screen.getByTestId('followed-row-BTCUSDT')).queryByRole('link')).not.toBeInTheDocument();
        await userEvent.click(within(screen.getByTestId('followed-row-SPY')).getByRole('link', { name: 'SPY' }));
        expect(location()).toBe('/candidates/SPY');
        expect(JSON.parse(screen.getByTestId('detail-state').textContent!)).toEqual({
            from: '/followed-symbols?followed_q=s',
            fromLabel: 'Followed Symbols',
        });
    });

    it('links search results for stocks and funds only', async () => {
        await renderPage();

        await userEvent.type(screen.getByLabelText('Search the scanner universe'), 'vg');
        expect(await screen.findByTestId('result-symbol-VGZ')).toHaveAttribute('href', '/candidates/VGZ');

        await userEvent.type(screen.getByLabelText('Search all symbols — ETFs, ADRs, OTC and crypto'), 'dia');
        expect(await screen.findByTestId('result-symbol-DIA')).toHaveAttribute('href', '/candidates/DIA');
        expect(screen.getByTestId('result-symbol-DIAUSDT').tagName).toBe('SPAN');
    });

    it('links nothing standalone', async () => {
        await renderPage({ hosted: false });
        expect(within(screen.getByTestId('followed-table')).queryAllByRole('link')).toHaveLength(0);
    });
});
