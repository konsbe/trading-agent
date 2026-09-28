import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter, Route, Routes, useLocation } from 'react-router-dom';
import { fetchComputedSymbols, fetchWatchlist, searchSymbols, WatchlistItem } from '@/api';
import { HostModeProvider } from '@/providers/HostModeContext';
import { makeComputedSymbols, makeManualComputed, makeWatchlistItem } from '@/test-utils/fixtures';
import WatchlistPage from './WatchlistPage';

jest.mock('@/api/watchlist/watchlistApi', () => ({
    fetchWatchlist: jest.fn(),
    addToWatchlist: jest.fn(),
    removeFromWatchlist: jest.fn(),
    searchSymbols: jest.fn(),
}));

jest.mock('@/api/tracking/trackingApi', () => ({
    fetchComputedSymbols: jest.fn(),
    requestCompute: jest.fn(),
    stopCompute: jest.fn(),
}));

const fetchMock = fetchWatchlist as jest.MockedFunction<typeof fetchWatchlist>;
const searchMock = searchSymbols as jest.MockedFunction<typeof searchSymbols>;
const computedMock = fetchComputedSymbols as jest.MockedFunction<typeof fetchComputedSymbols>;

const Location = () => {
    const { pathname, search } = useLocation();
    return <span data-testid="location">{`${pathname}${search}`}</span>;
};

const Detail = () => <span data-testid="detail-state">{JSON.stringify(useLocation().state)}</span>;

const ITEMS: WatchlistItem[] = [
    makeWatchlistItem({ symbol: 'NVDA', company_name: 'NVIDIA CORP', added_at: '2026-09-26T08:00:00Z', rvol_20: 0.7 }),
    makeWatchlistItem({ symbol: 'AMD', company_name: 'ADVANCED MICRO DEVICES', added_at: '2026-09-25T08:00:00Z', rvol_20: 2.1 }),
    makeWatchlistItem({ symbol: 'VGZ', added_at: '2026-09-26T08:00:00Z', rvol_20: null }),
    makeWatchlistItem({ symbol: 'BTCUSDT', company_name: 'Bitcoin', added_at: '2026-09-20T08:00:00Z', rvol_20: 1.1 }),
];

const renderPage = async ({ hosted = true, url = '/watchlist' } = {}) => {
    render(
        <HostModeProvider hosted={hosted}>
            <MemoryRouter initialEntries={[url]}>
                <Location />
                <Routes>
                    <Route path="/watchlist/*" element={<WatchlistPage />} />
                    <Route path="/candidates/:symbol" element={<Detail />} />
                </Routes>
            </MemoryRouter>
        </HostModeProvider>
    );
    await screen.findByTestId('watchlist-table');
};

const rowSymbols = () => screen.queryAllByTestId(/^watchlist-row-/).map(el => el.getAttribute('data-testid')!.replace('watchlist-row-', ''));
const header = (name: string) => screen.getByRole('button', { name });
const location = () => screen.getByTestId('location').textContent;

beforeEach(() => {
    window.sessionStorage.clear();
    fetchMock.mockResolvedValue({ owner: 'unauthenticated', items: ITEMS });
    searchMock.mockResolvedValue({ query: '', results: [] });
    computedMock.mockResolvedValue(makeComputedSymbols([]));
});

describe('WatchlistPage table view', () => {
    it('defaults to newest added first, ties by symbol, and says so', async () => {
        await renderPage();

        expect(rowSymbols()).toEqual(['NVDA', 'VGZ', 'AMD', 'BTCUSDT']);
        expect(screen.getByTestId('watchlist-sort-label')).toHaveTextContent('Sorted by Added, newest first');
        expect(screen.getByRole('columnheader', { name: 'Added' })).toHaveAttribute('aria-sort', 'descending');
        expect(location()).toBe('/watchlist');
    });

    it('sorts every column but Remove from its header, missing values last both ways, in the URL', async () => {
        await renderPage();

        await userEvent.click(header('RVOL'));
        expect(rowSymbols()).toEqual(['AMD', 'BTCUSDT', 'NVDA', 'VGZ']);
        expect(location()).toBe('/watchlist?watchlist_sort=rvol_20%3Adesc');
        await userEvent.click(header('RVOL'));
        expect(rowSymbols()).toEqual(['NVDA', 'BTCUSDT', 'AMD', 'VGZ']);
        expect(screen.getByTestId('watchlist-sort-label')).toHaveTextContent('Sorted by RVOL, ascending');

        expect(screen.getByRole('columnheader', { name: 'Actions' })).not.toHaveAttribute('aria-sort');
        expect(within(screen.getByRole('columnheader', { name: 'Actions' })).queryByRole('button')).not.toBeInTheDocument();
    });

    it('sorts Compute by state, attention first', async () => {
        computedMock.mockResolvedValue(
            makeComputedSymbols([
                makeManualComputed('computed', { symbol: 'NVDA' }),
                makeManualComputed('failed', { symbol: 'AMD' }),
                makeManualComputed('waiting_for_data', { symbol: 'VGZ' }),
            ])
        );
        await renderPage();
        await screen.findByTestId('compute-state-AMD');

        await userEvent.click(header('Compute'));
        expect(rowSymbols()).toEqual(['AMD', 'VGZ', 'NVDA', 'BTCUSDT']);
        await userEvent.click(header('Compute'));
        expect(rowSymbols()).toEqual(['NVDA', 'VGZ', 'AMD', 'BTCUSDT']);
    });

    it('filters by the text shown (company name) and restores sort + search from the URL', async () => {
        await renderPage({ url: '/watchlist?watchlist_sort=symbol%3Aasc&watchlist_q=micro' });

        expect(rowSymbols()).toEqual(['AMD']);
        expect(screen.getByTestId('watchlist-search-input')).toHaveValue('micro');
        expect(screen.getByTestId('watchlist-search-count')).toHaveTextContent('1 of 4');

        await userEvent.clear(screen.getByTestId('watchlist-search-input'));
        await userEvent.type(screen.getByTestId('watchlist-search-input'), 'zzz');
        expect(await screen.findByTestId('watchlist-no-match')).toHaveTextContent('No watched symbols match “zzz”');
    });

    it('links stocks to Stock Detail with "Back to Watchlist" and the current sort + search; crypto pairs are text', async () => {
        await renderPage({ url: '/watchlist?watchlist_sort=symbol%3Aasc' });

        expect(within(screen.getByTestId('watchlist-row-BTCUSDT')).queryByRole('link')).not.toBeInTheDocument();
        expect(screen.getByTestId('watchlist-row-BTCUSDT')).not.toHaveClass('is-linked');

        await userEvent.click(within(screen.getByTestId('watchlist-row-AMD')).getByRole('link', { name: 'AMD' }));
        expect(location()).toBe('/candidates/AMD');
        expect(JSON.parse(screen.getByTestId('detail-state').textContent!)).toEqual({
            from: '/watchlist?watchlist_sort=symbol%3Aasc',
            fromLabel: 'Watchlist',
        });
    });

    it('opens Stock Detail from a row click with the same origin', async () => {
        await renderPage();

        await userEvent.click(within(screen.getByTestId('watchlist-row-NVDA')).getByText('NVIDIA CORP'));
        expect(location()).toBe('/candidates/NVDA');
        expect(JSON.parse(screen.getByTestId('detail-state').textContent!)).toEqual({ from: '/watchlist', fromLabel: 'Watchlist' });
    });

    it('links nothing standalone', async () => {
        await renderPage({ hosted: false });
        expect(within(screen.getByTestId('watchlist-table')).queryAllByRole('link')).toHaveLength(0);
    });
});
