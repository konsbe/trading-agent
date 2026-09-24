import { render, screen } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import AppRouter from './AppRouter';

jest.mock('@/pages/WatchlistPage', () => ({ __esModule: true, default: () => <div>watchlist-page</div> }));

describe('AppRouter', () => {
    it('renders the watchlist page standalone at /', () => {
        render(
            <MemoryRouter initialEntries={['/']}>
                <AppRouter />
            </MemoryRouter>
        );
        expect(screen.getByText('watchlist-page')).toBeInTheDocument();
    });

    it('renders the watchlist page hosted under spog watchlist/*', () => {
        render(
            <MemoryRouter initialEntries={['/watchlist']}>
                <Routes>
                    <Route path="watchlist/*" element={<AppRouter />} />
                </Routes>
            </MemoryRouter>
        );
        expect(screen.getByText('watchlist-page')).toBeInTheDocument();
    });
});
