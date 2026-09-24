import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { ApiError } from '@/api';
import useWatchlist, { UseWatchlist } from '@/hooks/watchlist/useWatchlist';
import WatchlistButton from './WatchlistButton';

jest.mock('@/hooks/watchlist/useWatchlist', () => ({ __esModule: true, default: jest.fn() }));

const useWatchlistMock = useWatchlist as jest.MockedFunction<typeof useWatchlist>;
const add = jest.fn();
const remove = jest.fn();

const mockWatchlist = (state: Partial<UseWatchlist> & { watched?: boolean } = {}) => {
    const { watched = false, ...rest } = state;
    useWatchlistMock.mockReturnValue({
        items: [],
        isLoading: false,
        error: null,
        saving: new Set(),
        isWatched: () => watched,
        add,
        remove,
        ...rest,
    });
};

const button = () => screen.getByTestId('watchlist-button');

describe('WatchlistButton', () => {
    it('offers "Add to watchlist" (not pressed) and adds on click', async () => {
        mockWatchlist();
        render(<WatchlistButton symbol="VGZ" />);

        expect(button()).toHaveTextContent('Add to watchlist');
        expect(button()).toHaveAttribute('aria-pressed', 'false');
        expect(button()).toBeEnabled();
        expect(screen.getByText('Saved to the shared watchlist (no sign-in yet)')).toBeInTheDocument();

        await userEvent.click(button());
        expect(add).toHaveBeenCalledWith('VGZ');
        expect(remove).not.toHaveBeenCalled();
    });

    it('shows "In watchlist — remove" (pressed) and removes on click', async () => {
        mockWatchlist({ watched: true });
        render(<WatchlistButton symbol="VGZ" />);

        expect(button()).toHaveTextContent('In watchlist — remove');
        expect(button()).toHaveAttribute('aria-pressed', 'true');

        await userEvent.click(button());
        expect(remove).toHaveBeenCalledWith('VGZ');
    });

    it('is disabled and busy while saving this symbol', () => {
        mockWatchlist({ watched: true, saving: new Set(['VGZ']) });
        render(<WatchlistButton symbol="vgz" />);

        expect(button()).toBeDisabled();
        expect(button()).toHaveAttribute('aria-busy', 'true');
    });

    it('is disabled while the initial list loads', () => {
        mockWatchlist({ isLoading: true });
        render(<WatchlistButton symbol="VGZ" />);

        expect(button()).toBeDisabled();
        expect(button()).toHaveTextContent('Loading watchlist…');
    });

    it('shows an inline error after a failed save', () => {
        mockWatchlist({ error: new ApiError(404, 'unknown_symbol') });
        render(<WatchlistButton symbol="ZZZZ" />);

        expect(screen.getByRole('alert')).toHaveTextContent("Couldn't update the watchlist: That symbol isn't in the scanner's universe.");
        expect(button()).toBeEnabled();
    });
});
