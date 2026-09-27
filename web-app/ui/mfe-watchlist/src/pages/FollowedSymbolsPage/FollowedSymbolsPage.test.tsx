import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import {
    ApiError,
    fetchComputedSymbols,
    fetchFollowedSymbols,
    followSymbol,
    requestCompute,
    searchDirectory,
    searchSymbols,
    unfollowSymbol,
} from '@/api';
import {
    makeComputedSymbols,
    makeDirectorySearch,
    makeFollowedSymbols,
    makeManualComputed,
    makeSymbolSearch,
} from '@/test-utils/fixtures';
import FollowedSymbolsPage, { EMPTY_FOLLOWED_MESSAGE, FOLLOWED_EXPLAINER } from '.';

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
const followMock = followSymbol as jest.MockedFunction<typeof followSymbol>;
const unfollowMock = unfollowSymbol as jest.MockedFunction<typeof unfollowSymbol>;
const directoryMock = searchDirectory as jest.MockedFunction<typeof searchDirectory>;
const universeMock = searchSymbols as jest.MockedFunction<typeof searchSymbols>;
const fetchComputedMock = fetchComputedSymbols as jest.MockedFunction<typeof fetchComputedSymbols>;
const computeMock = requestCompute as jest.MockedFunction<typeof requestCompute>;

const renderPage = async () => {
    render(<FollowedSymbolsPage />, { wrapper: MemoryRouter });
    await screen.findByTestId('followed-table');
};

const universeInput = () => screen.getByLabelText('Search the scanner universe');
const directoryInput = () => screen.getByLabelText('Search all symbols — ETFs, ADRs, OTC and crypto');

beforeEach(() => {
    window.sessionStorage.clear();
    fetchFollowedMock.mockResolvedValue(
        makeFollowedSymbols([{}, { symbol: 'BTCUSDT', name: 'BTC', asset_type: 'crypto', listing: 'crypto', source: 'env_seed' }])
    );
    fetchComputedMock.mockResolvedValue(makeComputedSymbols([]));
    universeMock.mockResolvedValue(makeSymbolSearch('vg', ['VG', 'VGZ']));
    directoryMock.mockResolvedValue(
        makeDirectorySearch('dia', [
            {},
            { symbol: 'DGX', name: 'QUEST DIAGNOSTICS INC', type: 'Common Stock', mic: 'XNYS', asset_type: 'equity', in_universe: true },
            { symbol: 'DIAUSDT', name: 'DIA', type: 'spot', mic: null, asset_type: 'crypto', source: 'binance_spot' },
            { symbol: 'IWMX', followed: true },
        ])
    );
});

describe('FollowedSymbolsPage', () => {
    it('explains the list and shows every followed symbol with its facts', async () => {
        await renderPage();

        expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('Followed Symbols (2)');
        expect(screen.getByText(FOLLOWED_EXPLAINER)).toBeInTheDocument();
        const iwm = within(screen.getByTestId('followed-row-IWM'));
        expect(iwm.getByRole('rowheader')).toHaveTextContent('IWM');
        expect(screen.getByTestId('followed-row-IWM')).toHaveTextContent(/ISHARES RUSSELL 2000 ETF\s*ETF\s*US\s*Added by you\s*Sep 27, 2026/);
        expect(screen.getByTestId('followed-row-BTCUSDT')).toHaveTextContent(/Crypto\s*Crypto\s*Seeded from .env/);
        expect(iwm.getByRole('button', { name: 'Compute ISHARES RUSSELL 2000 ETF' })).toBeInTheDocument();
        expect(iwm.getByRole('button', { name: 'Unfollow IWM' })).toBeEnabled();
    });

    it('has two separate, labelled searches, with the foreign-listing hint under Search B', async () => {
        await renderPage();

        expect(universeInput()).not.toBe(directoryInput());
        expect(screen.getByTestId('search-universe')).toHaveTextContent('US common stocks the momentum scanner covers.');
        expect(screen.getByTestId('search-directory')).toHaveTextContent("Foreign listings (e.g. 2222.SR) can't be found by either search yet.");
    });

    it('Search A queries only the universe and marks followed / not-scanned matches', async () => {
        universeMock.mockResolvedValue({
            query: 'i',
            results: [
                { symbol: 'IWM', company_name: 'ISHARES RUSSELL 2000 ETF', exchange: 'NYSE Arca', is_eligible: true },
                { symbol: 'VGI', company_name: 'VIRTUS GLOBAL', exchange: 'NYSE', is_eligible: false },
            ],
        });
        await renderPage();

        await userEvent.type(universeInput(), 'i');
        const results = await screen.findByTestId('search-universe-results');
        expect(directoryMock).not.toHaveBeenCalled();
        expect(within(results).getByTestId('result-IWM')).toHaveTextContent('Following');
        expect(within(results).getByRole('button', { name: 'Following IWM' })).toBeDisabled();
        expect(within(results).getByTestId('result-VGI')).toHaveTextContent('not scanned');
        expect(within(results).getByRole('button', { name: 'Follow VGI' })).toBeEnabled();
        expect(within(results).getByRole('button', { name: 'Compute VGI' })).toBeEnabled();
    });

    it('Search B shows type, asset type, venue and in-universe / followed markers', async () => {
        await renderPage();

        await userEvent.type(directoryInput(), 'dia');
        const results = await screen.findByTestId('search-directory-results');
        expect(universeMock).not.toHaveBeenCalled();
        expect(within(results).getByTestId('result-DIA')).toHaveTextContent(/SS SPDR DOW JONES INDUS AVG\s*ETP · ETF · venue ARCX/);
        expect(within(results).getByTestId('result-DGX')).toHaveTextContent(/Common Stock · Equity · venue XNYS\s*In scanner universe/);
        expect(within(results).getByTestId('result-DIAUSDT')).toHaveTextContent('Crypto · Binance spot');
        expect(within(results).getByRole('button', { name: 'Following IWMX' })).toBeDisabled();
    });

    it('follows from a result, then lists it and marks the result as followed', async () => {
        await renderPage();
        followMock.mockResolvedValue(makeFollowedSymbols([{ symbol: 'DIA', name: 'SS SPDR DOW JONES INDUS AVG' }, {}]));

        await userEvent.type(directoryInput(), 'dia');
        const results = await screen.findByTestId('search-directory-results');
        await userEvent.click(within(results).getByRole('button', { name: 'Follow DIA' }));

        expect(followMock).toHaveBeenCalledWith('DIA');
        expect(await screen.findByTestId('followed-row-DIA')).toBeInTheDocument();
        expect(within(results).getByRole('button', { name: 'Following DIA' })).toBeDisabled();
        await waitFor(() => expect(fetchComputedMock).toHaveBeenCalledTimes(2));
    });

    it.each([
        [404, 'unknown_symbol', "That symbol isn't in the scanner's universe or the all-symbols directory."],
        [422, 'not_computable', "This listing type (warrants, units, …) can't be computed."],
    ])('shows a refused follow (HTTP %i) inline on the result', async (status, code, message) => {
        await renderPage();
        followMock.mockRejectedValue(new ApiError(status, code));

        await userEvent.type(directoryInput(), 'dia');
        await userEvent.click(await screen.findByRole('button', { name: 'Follow DIA' }));

        expect(await screen.findByTestId('follow-error-DIA')).toHaveTextContent(`Couldn't follow DIA: ${message}`);
    });

    it('computes from a search result and shows the queued state there', async () => {
        await renderPage();
        computeMock.mockResolvedValue(makeComputedSymbols([makeManualComputed('waiting_for_data', { symbol: 'VTI' })]));
        directoryMock.mockResolvedValue(makeDirectorySearch('vti', [{ symbol: 'VTI', name: 'VANGUARD TOTAL STOCK MKT ETF' }]));

        await userEvent.type(directoryInput(), 'vti');
        await userEvent.click(await screen.findByRole('button', { name: 'Compute VTI' }));

        expect(computeMock).toHaveBeenCalledWith('VTI');
        expect(await screen.findByTestId('compute-state-VTI')).toHaveTextContent(/^Waiting for data/);
    });

    it('says when a search has no matches, and clears with Escape', async () => {
        directoryMock.mockResolvedValue(makeDirectorySearch('zzzz', []));
        await renderPage();

        await userEvent.type(directoryInput(), 'zzzz');
        expect(await screen.findByText('No matches for “zzzz”.')).toHaveAttribute('data-testid', 'search-directory-status');
        await userEvent.keyboard('{Escape}');
        expect(directoryInput()).toHaveValue('');
        expect(screen.queryByTestId('search-directory-status')).not.toBeInTheDocument();
    });

    it('shows a rejected query in plain words', async () => {
        directoryMock.mockRejectedValue(new ApiError(400, 'invalid_query'));
        await renderPage();

        await userEvent.type(directoryInput(), 'x');
        expect(await screen.findByText('Enter 1–40 characters to search.')).toHaveAttribute('data-testid', 'search-directory-status');
    });

    it('unfollows a row, and shows a failed unfollow on that row', async () => {
        await renderPage();
        unfollowMock.mockRejectedValueOnce(new ApiError(0, 'network_error'));

        await userEvent.click(screen.getByRole('button', { name: 'Unfollow IWM' }));
        expect(await screen.findByTestId('unfollow-error-IWM')).toHaveTextContent("Couldn't reach momentum-api.");

        unfollowMock.mockResolvedValue(makeFollowedSymbols([{ symbol: 'BTCUSDT', name: 'BTC', asset_type: 'crypto', listing: 'crypto', source: 'env_seed' }]));
        await userEvent.click(screen.getByRole('button', { name: 'Unfollow IWM' }));
        await waitFor(() => expect(screen.queryByTestId('followed-row-IWM')).not.toBeInTheDocument());
        expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('Followed Symbols (1)');
    });

    it('explains what unfollowing a seeded symbol does', async () => {
        await renderPage();
        expect(screen.getByRole('button', { name: 'Unfollow BTCUSDT' })).toHaveAttribute(
            'title',
            expect.stringMatching(/seeded from \.env once: unfollowing removes it, and following it again lists it as added by you/)
        );
    });

    it('lists the rows when the first one has no name (live 2222.SR)', async () => {
        fetchFollowedMock.mockResolvedValue(makeFollowedSymbols([{ symbol: '2222.SR', name: null, listing: 'foreign', source: 'env_seed' }, {}]));
        await renderPage();

        expect(screen.getByTestId('followed-row-2222.SR')).toHaveTextContent(/2222\.SR\s*—/);
        expect(screen.queryByText(EMPTY_FOLLOWED_MESSAGE)).not.toBeInTheDocument();
    });

    it('shows the empty state', async () => {
        fetchFollowedMock.mockResolvedValue({ items: [] });
        render(<FollowedSymbolsPage />, { wrapper: MemoryRouter });

        expect(await screen.findByText(EMPTY_FOLLOWED_MESSAGE)).toBeInTheDocument();
    });

    it('shows a failed load with Retry', async () => {
        fetchFollowedMock.mockRejectedValueOnce(new ApiError(503, 'database_unavailable'));
        render(<FollowedSymbolsPage />, { wrapper: MemoryRouter });

        expect(screen.getByTestId('followed-loading')).toHaveAttribute('aria-busy', 'true');
        const error = await screen.findByTestId('api-error-state');
        expect(error).toHaveTextContent("Couldn't load the followed symbols. The database is unavailable right now.");
        await userEvent.click(within(error).getByRole('button', { name: 'Retry' }));
        expect(await screen.findByTestId('followed-table')).toBeInTheDocument();
    });
});
