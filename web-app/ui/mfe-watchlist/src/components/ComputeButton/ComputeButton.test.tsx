import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { ApiError, ComputedSymbolsResponse, fetchComputedSymbols, requestCompute } from '@/api';
import { makeComputedSymbol, makeComputedSymbols, makeManualComputed } from '@/test-utils/fixtures';
import { renderWithCompute } from '@/test-utils/renderWithCompute';
import ComputeButton from '.';

jest.mock('@/api/tracking/trackingApi', () => ({
    fetchComputedSymbols: jest.fn(),
    requestCompute: jest.fn(),
    stopCompute: jest.fn(),
}));

const fetchMock = fetchComputedSymbols as jest.MockedFunction<typeof fetchComputedSymbols>;
const computeMock = requestCompute as jest.MockedFunction<typeof requestCompute>;

const renderButton = async (body: ComputedSymbolsResponse = makeComputedSymbols([]), symbol = 'DIA', options = {}) => {
    fetchMock.mockResolvedValue(body);
    const view = renderWithCompute(<ComputeButton symbol={symbol} />, options);
    await waitFor(() => expect(fetchMock).toHaveBeenCalled());
    return view;
};

describe('ComputeButton', () => {
    it('is a Compute button for a symbol without a manual request', async () => {
        await renderButton();

        const button = screen.getByRole('button', { name: 'Compute DIA' });
        expect(button).toBeEnabled();
        expect(button).toHaveAttribute('title', expect.stringMatching(/^Queue this symbol/));
    });

    it('stays pressable for a symbol computed automatically, and says it asks for a fresh computation', async () => {
        await renderButton(makeComputedSymbols([makeComputedSymbol({ reasons: ['followed', 'watchlist'] })]), 'AMZN');

        await waitFor(() =>
            expect(screen.getByRole('button', { name: 'Compute AMZN' })).toHaveAttribute(
                'title',
                'Already computed every day (followed, on the watchlist). Compute asks for a fresh fetch and computation now.'
            )
        );
        expect(screen.queryByTestId('compute-state-AMZN')).not.toBeInTheDocument();
    });

    it('PUTs, shows "Requesting…" while in flight, then the queued state instead of the button', async () => {
        await renderButton();
        let resolve!: (body: ComputedSymbolsResponse) => void;
        computeMock.mockReturnValue(new Promise(res => (resolve = res)));

        await userEvent.click(screen.getByRole('button', { name: 'Compute DIA' }));
        expect(computeMock).toHaveBeenCalledWith('DIA');
        expect(screen.getByRole('button', { name: 'Compute DIA' })).toBeDisabled();
        expect(screen.getByRole('button', { name: 'Compute DIA' })).toHaveTextContent('Requesting…');

        resolve(makeComputedSymbols([makeManualComputed('waiting_for_data')]));
        expect(await screen.findByTestId('compute-state-DIA')).toHaveTextContent(/^Waiting for data/);
        expect(screen.queryByRole('button', { name: 'Compute DIA' })).not.toBeInTheDocument();
    });

    it('shows an open request already in progress on load (e.g. computing), without a button', async () => {
        await renderButton(makeComputedSymbols([makeManualComputed('computing')]));

        expect(await screen.findByTestId('compute-state-DIA')).toHaveAttribute('data-state', 'computing');
        expect(screen.queryByRole('button')).not.toBeInTheDocument();
    });

    it('offers "Check again" only for data_not_arrived, explaining it re-checks the same request', async () => {
        await renderButton(makeComputedSymbols([makeManualComputed('data_not_arrived')]));

        const retry = await screen.findByRole('button', { name: 'Check again for DIA' });
        expect(screen.getByText(/Same open request — this re-checks it; it doesn't start a new fetch\./)).toBeInTheDocument();

        computeMock.mockResolvedValue(makeComputedSymbols([makeManualComputed('computing')]));
        await userEvent.click(retry);
        expect(computeMock).toHaveBeenCalledWith('DIA');
        expect(await screen.findByTestId('compute-state-DIA')).toHaveAttribute('data-state', 'computing');
        expect(screen.queryByRole('button', { name: 'Check again for DIA' })).not.toBeInTheDocument();
    });

    it.each([
        [404, 'unknown_symbol', "That symbol isn't in the scanner's universe or the all-symbols directory."],
        [422, 'not_computable', "This listing type (warrants, units, …) can't be computed."],
    ])('shows a refused compute (HTTP %i) inline in plain words and keeps the button', async (status, code, message) => {
        await renderButton(makeComputedSymbols([]), 'ABCDW');
        computeMock.mockRejectedValue(new ApiError(status, code));

        await userEvent.click(screen.getByRole('button', { name: 'Compute ABCDW' }));

        expect(await screen.findByRole('alert')).toHaveTextContent(message);
        expect(screen.getByRole('button', { name: 'Compute ABCDW' })).toBeEnabled();
    });

    it('uses the given label as the accessible name', async () => {
        fetchMock.mockResolvedValue(makeComputedSymbols([]));
        renderWithCompute(<ComputeButton symbol="iwm" label="iShares Russell 2000" />);

        expect(await screen.findByRole('button', { name: 'Compute iShares Russell 2000' })).toBeInTheDocument();
        expect(screen.getByTestId('compute-control-IWM')).toBeInTheDocument();
    });
});
