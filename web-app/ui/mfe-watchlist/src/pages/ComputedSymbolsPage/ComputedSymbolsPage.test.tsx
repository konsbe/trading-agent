import { act, render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { ApiError, fetchComputedSymbols, stopCompute } from '@/api';
import { COMPUTE_POLL_MS } from '@/providers/ComputeStatusContext';
import { makeComputedSymbol, makeComputedSymbols, makeManualComputed } from '@/test-utils/fixtures';
import ComputedSymbolsPage, { COMPUTED_EXPLAINER, EMPTY_COMPUTED_MESSAGE } from '.';

jest.mock('@/api/tracking/trackingApi', () => ({
    fetchComputedSymbols: jest.fn(),
    requestCompute: jest.fn(),
    stopCompute: jest.fn(),
}));

const fetchMock = fetchComputedSymbols as jest.MockedFunction<typeof fetchComputedSymbols>;
const stopMock = stopCompute as jest.MockedFunction<typeof stopCompute>;

const ROWS = [
    makeComputedSymbol(),
    makeComputedSymbol({ symbol: 'NVDA', name: 'NVIDIA CORP', reasons: ['followed', 'watchlist'] }),
    makeComputedSymbol({ symbol: 'ABLV', name: null, reasons: ['candidate'], state: 'scheduled', computed_at: null }),
    makeManualComputed('computed'),
    makeManualComputed('waiting_for_data', { symbol: 'VTI', name: 'VANGUARD TOTAL STOCK MKT ETF', reasons: ['watchlist', 'manual'] }),
];

const renderPage = async (body = makeComputedSymbols(ROWS)) => {
    fetchMock.mockResolvedValue(body);
    render(<ComputedSymbolsPage />, { wrapper: MemoryRouter });
    await screen.findByTestId('computed-table');
};

const row = (symbol: string) => within(screen.getByTestId(`computed-row-${symbol}`));

beforeEach(() => window.sessionStorage.clear());
afterEach(() => jest.useRealTimers());

describe('ComputedSymbolsPage', () => {
    it('explains the reasons and lists every symbol with an open reason', async () => {
        await renderPage();

        expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('Computed Symbols (5)');
        expect(screen.getByText(COMPUTED_EXPLAINER)).toBeInTheDocument();
        expect(COMPUTED_EXPLAINER).toMatch(/watchlist, today's candidates and followed symbols/);
        expect(COMPUTED_EXPLAINER).toMatch(/never deleted/);
        expect(screen.getAllByTestId(/^computed-row-/)).toHaveLength(5);
    });

    it('shows reasons as chips, Manual accented', async () => {
        await renderPage();

        expect(row('NVDA').getByTestId('reason-NVDA-followed')).toHaveTextContent('Followed');
        expect(row('NVDA').getByTestId('reason-NVDA-watchlist')).toHaveTextContent('Watchlist');
        expect(row('ABLV').getByTestId('reason-ABLV-candidate')).toHaveTextContent("Today's candidate");
        expect(row('DIA').getByTestId('reason-DIA-manual')).toHaveTextContent('Manual');
        expect(row('DIA').getByTestId('reason-DIA-manual')).toHaveClass('is-accent');
        expect(row('NVDA').getByTestId('reason-NVDA-followed')).toHaveClass('is-neutral');
    });

    it('shows the state, and the last computed time or "not yet"', async () => {
        await renderPage();

        expect(row('AMZN').getByTestId('compute-state-AMZN')).toHaveTextContent(/^Computed$/);
        expect(screen.getByTestId('computed-row-AMZN').querySelector('[data-column="computed_at"] time')).toHaveAttribute(
            'dateTime',
            '2026-09-27T14:12:07Z'
        );
        expect(row('ABLV').getByTestId('compute-state-ABLV')).toHaveTextContent('Scheduled · next daily pass');
        expect(screen.getByTestId('computed-row-ABLV').querySelector('[data-column="computed_at"]')).toHaveTextContent('not yet');
        expect(row('VTI').getByTestId('compute-state-VTI')).toHaveTextContent(/^Waiting for data · requested/);
        expect(row('ABLV').getByText('—')).toBeInTheDocument();
    });

    it('offers Stop computing only for a manual reason, explaining that other reasons keep it computed', async () => {
        await renderPage();

        expect(row('AMZN').queryByRole('button')).not.toBeInTheDocument();
        expect(row('NVDA').queryByRole('button')).not.toBeInTheDocument();
        const vti = row('VTI').getByRole('button', { name: /^Stop computing VTI/ });
        expect(vti).toHaveAttribute('title', expect.stringMatching(/^Closes only your manual Compute request\. Other reasons keep it computed/));
        expect(row('VTI').getByText('Other reasons keep it computed')).toBeInTheDocument();
        expect(row('DIA').getByText('Leaves the list; computed history is kept')).toBeInTheDocument();
    });

    it('stops computing with DELETE and adopts the response', async () => {
        await renderPage();
        stopMock.mockResolvedValue(makeComputedSymbols(ROWS.filter(item => item.symbol !== 'DIA')));

        await userEvent.click(row('DIA').getByRole('button', { name: /^Stop computing DIA/ }));

        expect(stopMock).toHaveBeenCalledWith('DIA');
        expect(await screen.findByRole('heading', { level: 1, name: 'Computed Symbols (4)' })).toBeInTheDocument();
        expect(screen.queryByTestId('computed-row-DIA')).not.toBeInTheDocument();
    });

    it('shows a failed stop on its row', async () => {
        await renderPage();
        stopMock.mockRejectedValue(new ApiError(503, 'database_unavailable'));

        await userEvent.click(row('DIA').getByRole('button', { name: /^Stop computing DIA/ }));

        expect(await row('DIA').findByTestId('stop-error-DIA')).toHaveTextContent('The database is unavailable right now.');
    });

    it('says when it is polling, and polls while a request is pending', async () => {
        jest.useFakeTimers();
        await renderPage();

        expect(screen.getByTestId('computed-meta')).toHaveTextContent(
            `A request is pending — checking every ${COMPUTE_POLL_MS / 1000} s · a request waits up to 30 min for data`
        );
        fetchMock.mockResolvedValue(makeComputedSymbols(ROWS.slice(0, 4)));
        await act(async () => {
            jest.advanceTimersByTime(COMPUTE_POLL_MS);
        });
        expect(fetchMock).toHaveBeenCalledTimes(2);
        expect(screen.getByTestId('computed-meta')).toHaveTextContent(/^Nothing pending/);
    });

    it('lists the rows when the first one has no name (live 2222.SR)', async () => {
        await renderPage(makeComputedSymbols([makeComputedSymbol({ symbol: '2222.SR', name: null }), ...ROWS]));

        expect(screen.getByTestId('computed-row-2222.SR')).toBeInTheDocument();
        expect(screen.queryByText(EMPTY_COMPUTED_MESSAGE)).not.toBeInTheDocument();
    });

    it('shows the empty state', async () => {
        fetchMock.mockResolvedValue(makeComputedSymbols([]));
        render(<ComputedSymbolsPage />, { wrapper: MemoryRouter });

        expect(await screen.findByText(EMPTY_COMPUTED_MESSAGE)).toBeInTheDocument();
    });

    it('shows a failed load with Retry', async () => {
        fetchMock.mockRejectedValueOnce(new ApiError(0, 'network_error'));
        render(<ComputedSymbolsPage />, { wrapper: MemoryRouter });

        expect(screen.getByTestId('computed-loading')).toHaveAttribute('aria-busy', 'true');
        const error = await screen.findByTestId('api-error-state');
        expect(error).toHaveTextContent("Couldn't load the computed symbols. Couldn't reach momentum-api.");

        fetchMock.mockResolvedValue(makeComputedSymbols(ROWS));
        await userEvent.click(within(error).getByRole('button', { name: 'Retry' }));
        expect(await screen.findByTestId('computed-table')).toBeInTheDocument();
    });
});
