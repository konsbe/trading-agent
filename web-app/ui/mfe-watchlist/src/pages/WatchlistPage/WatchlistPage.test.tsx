import { act, render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { COLLAPSIBLE_STORAGE_PREFIX } from '@trading-agent/shared-components';
import {
    addToWatchlist,
    ApiError,
    fetchComputedSymbols,
    fetchWatchlist,
    removeFromWatchlist,
    searchSymbols,
    WatchlistResponse,
} from '@/api';
import { COMPUTE_POLL_MS } from '@/providers/ComputeStatusContext';
import { HostModeProvider } from '@/providers/HostModeContext';
import {
    makeComputedSymbols,
    makeDailyBarsItem,
    makeUncoveredItem,
    makeWatchlist,
    makeWatchlistItem,
    makeWatchlistQueued,
} from '@/test-utils/fixtures';
import WatchlistPage from './WatchlistPage';

jest.mock('@/api/watchlist/watchlistApi', () => ({
    fetchWatchlist: jest.fn(),
    addToWatchlist: jest.fn(),
    removeFromWatchlist: jest.fn(),
    searchSymbols: jest.fn(),
}));

jest.mock('@/api/tracking/trackingApi', () => ({
    fetchComputedSymbols: jest.fn(() => Promise.resolve({ data_timeout_minutes: 30, items: [] })),
    requestCompute: jest.fn(),
    stopCompute: jest.fn(),
}));

const fetchMock = fetchWatchlist as jest.MockedFunction<typeof fetchWatchlist>;
const addMock = addToWatchlist as jest.MockedFunction<typeof addToWatchlist>;
const removeMock = removeFromWatchlist as jest.MockedFunction<typeof removeFromWatchlist>;
const searchMock = searchSymbols as jest.MockedFunction<typeof searchSymbols>;
const computedMock = fetchComputedSymbols as jest.MockedFunction<typeof fetchComputedSymbols>;

const deferred = <T,>() => {
    let resolve!: (value: T) => void;
    let reject!: (reason: unknown) => void;
    const promise = new Promise<T>((res, rej) => {
        resolve = res;
        reject = rej;
    });
    return { promise, resolve, reject };
};

const rowSymbols = () => screen.queryAllByTestId(/^watchlist-row-/).map(el => el.getAttribute('data-testid')!.replace('watchlist-row-', ''));

const renderLoaded = async (list: WatchlistResponse) => {
    fetchMock.mockResolvedValue(list);
    render(<WatchlistPage />, { wrapper: MemoryRouter });
    await screen.findByRole('combobox', { name: 'Add symbol' });
};

beforeEach(() => {
    searchMock.mockResolvedValue({ query: '', results: [] });
    computedMock.mockResolvedValue(makeComputedSymbols([]));
});

afterEach(() => jest.useRealTimers());

describe('WatchlistPage', () => {
    it('shows a skeleton while the list loads, without the add flow', () => {
        fetchMock.mockReturnValue(new Promise(() => {}));
        render(<WatchlistPage />, { wrapper: MemoryRouter });

        expect(screen.getByTestId('watchlist-skeleton')).toHaveAttribute('aria-busy', 'true');
        expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent(/^Watchlist$/);
        expect(screen.queryByRole('combobox')).not.toBeInTheDocument();
    });

    it('shows the API error with Retry, and loads on retry', async () => {
        fetchMock.mockRejectedValueOnce(new ApiError(503, 'database_unavailable'));
        render(<WatchlistPage />, { wrapper: MemoryRouter });

        const error = await screen.findByTestId('api-error-state');
        expect(error).toHaveTextContent('The watchlist database is unavailable right now.');
        expect(screen.queryByRole('combobox')).not.toBeInTheDocument();

        fetchMock.mockResolvedValue(makeWatchlist(['VGZ']));
        await userEvent.click(within(error).getByRole('button', { name: 'Retry' }));

        await screen.findByTestId('watchlist-row-VGZ');
        expect(screen.queryByTestId('api-error-state')).not.toBeInTheDocument();
        expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('Watchlist (1)');
    });

    it('shows the shared empty state with the add input above it', async () => {
        await renderLoaded(makeWatchlist([]));

        const empty = screen.getByText('Your watchlist is empty.');
        const input = screen.getByRole('combobox', { name: 'Add symbol' });
        expect(input.compareDocumentPosition(empty) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
        expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('Watchlist (0)');
        expect(screen.queryByRole('table')).not.toBeInTheDocument();
    });

    it('lists the symbols newest first with the count in the title', async () => {
        await renderLoaded({
            owner: 'unauthenticated',
            items: [makeWatchlistItem({ symbol: 'TSLA' }), makeUncoveredItem('DVY'), makeWatchlistItem({ symbol: 'FSLY' })],
        });

        expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('Watchlist (3)');
        expect(rowSymbols()).toEqual(['TSLA', 'DVY', 'FSLY']);
        expect(within(screen.getByTestId('watchlist-row-DVY')).getByTestId('price-hint')).toHaveTextContent('No price data');
    });

    it('keeps the title when hosted (the shell shows the pill)', async () => {
        fetchMock.mockResolvedValue(makeWatchlist(['VGZ']));
        render(
            <HostModeProvider hosted>
                <WatchlistPage />
            </HostModeProvider>,
            { wrapper: MemoryRouter }
        );

        expect(await screen.findByRole('heading', { level: 1 })).toHaveTextContent('Watchlist (1)');
        expect(screen.queryByText('Screener — not a forecast')).not.toBeInTheDocument();
    });

    it('wraps the list in a collapsible card whose state persists', async () => {
        await renderLoaded(makeWatchlist(['VGZ']));

        const toggle = screen.getByRole('button', { name: 'Watched symbols' });
        expect(toggle).toHaveAttribute('aria-expanded', 'true');

        await userEvent.click(toggle);

        expect(toggle).toHaveAttribute('aria-expanded', 'false');
        expect(screen.queryByRole('table')).not.toBeInTheDocument();
        expect(screen.getByRole('combobox', { name: 'Add symbol' })).toBeInTheDocument();
        expect(window.sessionStorage.getItem(`${COLLAPSIBLE_STORAGE_PREFIX}watchlist.list`)).not.toBeNull();
    });

    describe('remove', () => {
        it('removes the row and updates the count', async () => {
            await renderLoaded(makeWatchlist(['A', 'B']));
            removeMock.mockResolvedValue(makeWatchlist(['B']));

            await userEvent.click(screen.getByRole('button', { name: 'Remove A from watchlist' }));

            expect(removeMock).toHaveBeenCalledWith('A');
            await waitFor(() => expect(rowSymbols()).toEqual(['B']));
            expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('Watchlist (1)');
            expect(screen.queryByRole('alert')).not.toBeInTheDocument();
        });

        it('puts a failed remove back in place with a dismissible message', async () => {
            await renderLoaded(makeWatchlist(['A', 'B', 'C']));
            const del = deferred<WatchlistResponse>();
            removeMock.mockReturnValue(del.promise);

            await userEvent.click(screen.getByRole('button', { name: 'Remove B from watchlist' }));
            expect(rowSymbols()).toEqual(['A', 'C']);

            del.reject(new ApiError(503, 'database_unavailable'));

            const message = await screen.findByTestId('remove-error');
            expect(message).toHaveTextContent("Couldn't remove B: The watchlist database is unavailable right now.");
            expect(rowSymbols()).toEqual(['A', 'B', 'C']);
            expect(screen.queryByTestId('api-error-state')).not.toBeInTheDocument();

            await userEvent.click(within(message).getByRole('button', { name: 'Dismiss' }));
            expect(screen.queryByTestId('remove-error')).not.toBeInTheDocument();
        });
    });

    describe('add', () => {
        it('adds a typed ticker on Enter, disabling its remove while pending', async () => {
            await renderLoaded(makeWatchlist(['VGZ']));
            const put = deferred<WatchlistResponse>();
            addMock.mockReturnValue(put.promise);

            await userEvent.type(screen.getByRole('combobox', { name: 'Add symbol' }), 'fsly{Enter}');

            expect(addMock).toHaveBeenCalledWith('FSLY');
            expect(rowSymbols()).toEqual(['FSLY', 'VGZ']);
            expect(screen.getByRole('button', { name: 'Remove FSLY from watchlist' })).toBeDisabled();
            expect(within(screen.getByTestId('watchlist-row-FSLY')).getByTestId('price-hint')).toHaveTextContent('Adding…');

            put.resolve({ owner: 'unauthenticated', items: [makeWatchlistItem({ symbol: 'FSLY', close: 29.65 }), makeWatchlistItem()] });

            await waitFor(() => expect(screen.getByRole('button', { name: 'Remove FSLY from watchlist' })).toBeEnabled());
            expect(within(screen.getByTestId('watchlist-row-FSLY')).getByTestId('price-value')).toHaveTextContent('$29.65');
            expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('Watchlist (2)');
        });

        it('shows unknown_symbol inline near the input and drops the optimistic row', async () => {
            await renderLoaded(makeWatchlist([]));
            addMock.mockRejectedValue(new ApiError(404, 'unknown_symbol'));

            await userEvent.type(screen.getByRole('combobox', { name: 'Add symbol' }), 'zzzz{Enter}');

            const message = await screen.findByTestId('add-symbol-error');
            expect(message).toHaveTextContent("Couldn't add ZZZZ: That symbol isn't in the scanner's universe.");
            expect(within(screen.getByTestId('add-symbol')).getByRole('alert')).toBe(message);
            expect(rowSymbols()).toEqual([]);
            expect(screen.getByText('Your watchlist is empty.')).toBeInTheDocument();
            expect(screen.queryByTestId('api-error-state')).not.toBeInTheDocument();

            await userEvent.click(within(message).getByRole('button', { name: 'Dismiss' }));
            expect(screen.queryByTestId('add-symbol-error')).not.toBeInTheDocument();
        });

        it('adds a picked search result and marks it "Added" afterwards', async () => {
            await renderLoaded(makeWatchlist([]));
            searchMock.mockResolvedValue({
                query: 'dvy',
                results: [{ symbol: 'DVY', company_name: 'ISHARES SELECT DIVIDEND ETF', exchange: 'NASDAQ', is_eligible: false }],
            });
            addMock.mockResolvedValue({ owner: 'unauthenticated', items: [makeUncoveredItem('DVY')] });
            const input = screen.getByRole('combobox', { name: 'Add symbol' });

            await userEvent.type(input, 'dvy');
            await userEvent.click(await screen.findByTestId('symbol-option-DVY'));

            expect(addMock).toHaveBeenCalledWith('DVY');
            await waitFor(() => expect(rowSymbols()).toEqual(['DVY']));

            await userEvent.type(input, 'dvy');
            expect(await screen.findByTestId('symbol-option-DVY')).toHaveTextContent('Added');
        });

        it("re-reads the compute states after an add, so the new row shows its queued fetch", async () => {
            await renderLoaded(makeWatchlist(['VGZ']));
            expect(computedMock).toHaveBeenCalledTimes(1);
            computedMock.mockResolvedValue(makeComputedSymbols([makeWatchlistQueued('waiting_for_data')]));
            addMock.mockResolvedValue({ owner: 'unauthenticated', items: [makeUncoveredItem('BP'), makeWatchlistItem()] });

            await userEvent.type(screen.getByRole('combobox', { name: 'Add symbol' }), 'bp{Enter}');

            expect(await within(screen.getByTestId('watchlist-row-BP')).findByTestId('compute-state-BP')).toHaveTextContent(
                /^Waiting for data · queued .+ \(added to watchlist\)$/
            );
            expect(computedMock).toHaveBeenCalledTimes(2);
        });

        it('does not re-read the compute states after a failed add', async () => {
            await renderLoaded(makeWatchlist([]));
            addMock.mockRejectedValue(new ApiError(404, 'unknown_symbol'));

            await userEvent.type(screen.getByRole('combobox', { name: 'Add symbol' }), 'zzzz{Enter}');
            await screen.findByTestId('add-symbol-error');

            expect(computedMock).toHaveBeenCalledTimes(1);
        });
    });

    describe('reload when a computation finishes', () => {
        const advancePoll = () =>
            act(async () => {
                jest.advanceTimersByTime(COMPUTE_POLL_MS);
            });

        it('reloads the watchlist once when a row goes from waiting / computing to computed, showing its new values', async () => {
            jest.useFakeTimers();
            fetchMock.mockResolvedValue({ owner: 'unauthenticated', items: [makeUncoveredItem('BP')] });
            computedMock.mockResolvedValue(makeComputedSymbols([makeWatchlistQueued('waiting_for_data')]));
            render(<WatchlistPage />, { wrapper: MemoryRouter });
            await screen.findByTestId('compute-state-BP');
            expect(fetchMock).toHaveBeenCalledTimes(1);

            computedMock.mockResolvedValue(makeComputedSymbols([makeWatchlistQueued('computing')]));
            await advancePoll();
            expect(screen.getByTestId('compute-state-BP')).toHaveAttribute('data-state', 'computing');
            expect(fetchMock).toHaveBeenCalledTimes(1);

            fetchMock.mockResolvedValue({ owner: 'unauthenticated', items: [makeDailyBarsItem({ symbol: 'BP', close: 44.15 })] });
            computedMock.mockResolvedValue(makeComputedSymbols([makeWatchlistQueued('computed')]));
            await advancePoll();

            await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(2));
            expect(await within(screen.getByTestId('watchlist-row-BP')).findByTestId('price-value')).toHaveTextContent('$44.15');
            expect(screen.getByTestId('daily-bars-BP')).toBeInTheDocument();

            await advancePoll();
            await advancePoll();
            expect(fetchMock).toHaveBeenCalledTimes(2);
        });

        it('does not reload for a symbol that was already computed on load', async () => {
            fetchMock.mockResolvedValue(makeWatchlist(['VGZ']));
            computedMock.mockResolvedValue(makeComputedSymbols([makeWatchlistQueued('computed', { symbol: 'VGZ' })]));
            render(<WatchlistPage />, { wrapper: MemoryRouter });
            await screen.findByRole('button', { name: 'Compute VGZ' });

            expect(fetchMock).toHaveBeenCalledTimes(1);
        });

        it('does not reload for a symbol that is not on the watchlist', async () => {
            jest.useFakeTimers();
            fetchMock.mockResolvedValue(makeWatchlist(['VGZ']));
            computedMock.mockResolvedValue(makeComputedSymbols([makeWatchlistQueued('waiting_for_data', { symbol: 'DIA' })]));
            render(<WatchlistPage />, { wrapper: MemoryRouter });
            await screen.findByTestId('watchlist-row-VGZ');
            await waitFor(() => expect(computedMock).toHaveBeenCalledTimes(1));

            computedMock.mockResolvedValue(makeComputedSymbols([makeWatchlistQueued('computed', { symbol: 'DIA' })]));
            await advancePoll();
            await waitFor(() => expect(computedMock).toHaveBeenCalledTimes(2));

            expect(fetchMock).toHaveBeenCalledTimes(1);
        });
    });
});
