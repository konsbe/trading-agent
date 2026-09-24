import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { ApiError, searchSymbols, SymbolSearchResponse } from '@/api';
import { makeSearchResult } from '@/test-utils/fixtures';
import AddSymbol from './AddSymbol';
import { AddSymbolProps } from './types';

jest.mock('@/api/watchlist/watchlistApi', () => ({
    fetchWatchlist: jest.fn(),
    addToWatchlist: jest.fn(),
    removeFromWatchlist: jest.fn(),
    searchSymbols: jest.fn(),
}));

const searchMock = searchSymbols as jest.MockedFunction<typeof searchSymbols>;

const RESULTS: SymbolSearchResponse = {
    query: 'on',
    results: [
        makeSearchResult({ symbol: 'ON', company_name: 'ON SEMICONDUCTOR', exchange: 'NASDAQ' }),
        makeSearchResult({ symbol: 'ONCO', company_name: 'ONCONETIX INC', exchange: 'NASDAQ' }),
        makeSearchResult({ symbol: 'ONFD', company_name: 'ON FUND', exchange: 'NYSE', is_eligible: false }),
    ],
};

const renderAdd = (overrides: Partial<AddSymbolProps> = {}) => {
    const props: AddSymbolProps = { isWatched: () => false, onAdd: jest.fn(), onDismissError: jest.fn(), ...overrides };
    const utils = render(<AddSymbol {...props} />);
    return { ...utils, props, input: screen.getByRole('combobox', { name: 'Add symbol' }) };
};

const typeAndWait = async (input: HTMLElement, text: string) => {
    await userEvent.type(input, text);
    await screen.findByRole('listbox');
};

beforeEach(() => searchMock.mockResolvedValue(RESULTS));

describe('AddSymbol', () => {
    it('is a labelled, collapsed combobox until something is typed', () => {
        const { input } = renderAdd();

        expect(input).toHaveAttribute('aria-expanded', 'false');
        expect(input).toHaveAttribute('aria-autocomplete', 'list');
        expect(screen.queryByTestId('add-symbol-popup')).not.toBeInTheDocument();
        expect(searchMock).not.toHaveBeenCalled();
    });

    it('shows "Searching…" then the matches with ticker, company, exchange', async () => {
        const { input } = renderAdd();

        await userEvent.type(input, 'on');
        expect(screen.getByTestId('add-symbol-status')).toHaveTextContent('Searching…');

        const listbox = await screen.findByRole('listbox', { name: 'Matching symbols' });
        expect(searchMock).toHaveBeenLastCalledWith('on', expect.anything());
        expect(input).toHaveAttribute('aria-expanded', 'true');
        expect(input).toHaveAttribute('aria-controls', listbox.id);
        const option = screen.getByRole('option', { name: /ONCO/ });
        expect(option).toHaveTextContent('ONCONETIX INC');
        expect(option).toHaveTextContent('NASDAQ');
    });

    it('marks symbols the scanner does not cover with a neutral hint', async () => {
        const { input } = renderAdd();
        await typeAndWait(input, 'on');

        expect(screen.getByTestId('symbol-option-ONFD')).toHaveTextContent('not scanned — no price data');
        expect(screen.getByTestId('symbol-option-ONCO')).not.toHaveTextContent('not scanned');
    });

    it('shows "No matches" when the search is empty', async () => {
        searchMock.mockResolvedValue({ query: 'zzz', results: [] });
        const { input } = renderAdd();

        await userEvent.type(input, 'zzz');

        await waitFor(() => expect(screen.getByTestId('add-symbol-status')).toHaveTextContent('No matches for “zzz”'));
        expect(input).toHaveAttribute('aria-expanded', 'false');
    });

    it('shows the mapped message when the search fails', async () => {
        searchMock.mockRejectedValue(new ApiError(400, 'invalid_query'));
        const { input } = renderAdd();

        await userEvent.type(input, 'x');

        await waitFor(() => expect(screen.getByTestId('add-symbol-status')).toHaveTextContent('Enter 1–40 characters to search.'));
    });

    it('moves through options with the arrow keys (wrapping) via aria-activedescendant', async () => {
        const { input } = renderAdd();
        await typeAndWait(input, 'on');

        await userEvent.keyboard('{ArrowDown}');
        const first = screen.getByTestId('symbol-option-ON');
        expect(input).toHaveAttribute('aria-activedescendant', first.id);
        expect(first).toHaveAttribute('aria-selected', 'true');

        await userEvent.keyboard('{ArrowDown}');
        expect(input).toHaveAttribute('aria-activedescendant', screen.getByTestId('symbol-option-ONCO').id);
        expect(first).toHaveAttribute('aria-selected', 'false');

        await userEvent.keyboard('{ArrowUp}{ArrowUp}');
        expect(input).toHaveAttribute('aria-activedescendant', screen.getByTestId('symbol-option-ONFD').id);
    });

    it('adds the highlighted match on Enter, with its details as the seed, and resets', async () => {
        const { input, props } = renderAdd();
        await typeAndWait(input, 'on');

        await userEvent.keyboard('{ArrowDown}{ArrowDown}{Enter}');

        expect(props.onAdd).toHaveBeenCalledWith('ONCO', { company_name: 'ONCONETIX INC', exchange: 'NASDAQ' });
        expect(input).toHaveValue('');
        expect(screen.queryByTestId('add-symbol-popup')).not.toBeInTheDocument();
    });

    it('adds a match on click', async () => {
        const { input, props } = renderAdd();
        await typeAndWait(input, 'on');

        await userEvent.click(screen.getByTestId('symbol-option-ONFD'));

        expect(props.onAdd).toHaveBeenCalledWith('ONFD', { company_name: 'ON FUND', exchange: 'NYSE' });
    });

    it('shows already-watched matches as "Added" and does not re-add them', async () => {
        const { input, props } = renderAdd({ isWatched: symbol => symbol === 'ONCO' });
        await typeAndWait(input, 'on');

        const added = screen.getByTestId('symbol-option-ONCO');
        expect(added).toHaveTextContent('Added');
        expect(added).toHaveAttribute('aria-disabled', 'true');

        await userEvent.keyboard('{ArrowDown}{ArrowDown}');
        expect(input).toHaveAttribute('aria-activedescendant', added.id);
        await userEvent.keyboard('{Enter}');
        await userEvent.click(added);

        expect(props.onAdd).not.toHaveBeenCalled();
        expect(screen.getByRole('listbox')).toBeInTheDocument();
    });

    it('tries the exact typed ticker on Enter when nothing is highlighted', async () => {
        searchMock.mockResolvedValue({ query: 'zzzz', results: [] });
        const { input, props } = renderAdd();

        await userEvent.type(input, 'zzzz{Enter}');

        expect(props.onAdd).toHaveBeenCalledWith('ZZZZ', undefined);
    });

    it('seeds the exact-ticker add from a matching result', async () => {
        const { input, props } = renderAdd();
        await typeAndWait(input, 'onco');

        await userEvent.keyboard('{Enter}');

        expect(props.onAdd).toHaveBeenCalledWith('ONCO', { company_name: 'ONCONETIX INC', exchange: 'NASDAQ' });
    });

    it('says so instead of re-adding a typed ticker that is already watched', async () => {
        searchMock.mockResolvedValue({ query: 'onco', results: [] });
        const { input, props } = renderAdd({ isWatched: symbol => symbol === 'ONCO' });

        await userEvent.type(input, 'onco{Enter}');

        expect(props.onAdd).not.toHaveBeenCalled();
        expect(screen.getByTestId('add-symbol-notice')).toHaveTextContent('ONCO is already in your watchlist.');
    });

    it('does nothing on Enter with a blank query', async () => {
        const { input, props } = renderAdd();

        await userEvent.type(input, '   {Enter}');

        expect(props.onAdd).not.toHaveBeenCalled();
    });

    it('closes on Escape, then clears on a second Escape', async () => {
        const { input } = renderAdd();
        await typeAndWait(input, 'on');

        await userEvent.keyboard('{Escape}');
        expect(screen.queryByRole('listbox')).not.toBeInTheDocument();
        expect(input).toHaveAttribute('aria-expanded', 'false');
        expect(input).toHaveValue('on');

        await userEvent.keyboard('{Escape}');
        expect(input).toHaveValue('');
    });

    it('reopens with the arrow keys after Escape', async () => {
        const { input } = renderAdd();
        await typeAndWait(input, 'on');
        await userEvent.keyboard('{Escape}{ArrowDown}');

        expect(screen.getByRole('listbox')).toBeInTheDocument();
    });

    it('closes on blur', async () => {
        const { input } = renderAdd();
        await typeAndWait(input, 'on');

        await userEvent.tab();

        expect(screen.queryByTestId('add-symbol-popup')).not.toBeInTheDocument();
    });

    it('shows a failed add inline, dismissible, and clears it on typing', async () => {
        const { input, props, rerender } = renderAdd({ error: { symbol: 'ZZZZ', message: "That symbol isn't in the scanner's universe." } });

        expect(screen.getByRole('alert')).toHaveTextContent("Couldn't add ZZZZ: That symbol isn't in the scanner's universe.");

        await userEvent.click(screen.getByRole('button', { name: 'Dismiss' }));
        expect(props.onDismissError).toHaveBeenCalledTimes(1);

        await userEvent.type(input, 'a');
        expect(props.onDismissError).toHaveBeenCalledTimes(2);

        rerender(<AddSymbol {...props} error={null} />);
        expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    });
});
