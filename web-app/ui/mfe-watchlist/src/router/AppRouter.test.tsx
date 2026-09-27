import { render, screen } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import AppRouter from './AppRouter';

jest.mock('@/pages/WatchlistPage', () => ({ __esModule: true, default: () => <div>watchlist-page</div> }));
jest.mock('@/pages/FollowedSymbolsPage', () => ({ __esModule: true, default: () => <div>followed-page</div> }));
jest.mock('@/pages/ComputedSymbolsPage', () => ({ __esModule: true, default: () => <div>computed-page</div> }));

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

    it.each([
        ['/followed-symbols', 'followed-page'],
        ['/computed-symbols', 'computed-page'],
    ])('renders %s standalone (spog mounts those roots directly)', (path, page) => {
        render(
            <MemoryRouter initialEntries={[path]}>
                <AppRouter />
            </MemoryRouter>
        );
        expect(screen.getByText(page)).toBeInTheDocument();
        expect(screen.queryByText('watchlist-page')).not.toBeInTheDocument();
    });
});
