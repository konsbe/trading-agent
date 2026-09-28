import { render, screen } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import AppRouter from './AppRouter';

jest.mock('@/pages/TrackedPositionsPage', () => ({ __esModule: true, default: () => <div>tracked-page</div> }));

describe('AppRouter', () => {
    it('renders the tracked positions page standalone at /', () => {
        render(
            <MemoryRouter initialEntries={['/']}>
                <AppRouter />
            </MemoryRouter>
        );
        expect(screen.getByText('tracked-page')).toBeInTheDocument();
    });

    it('renders the tracked positions page hosted under spog tracked-positions/*', () => {
        render(
            <MemoryRouter initialEntries={['/tracked-positions']}>
                <Routes>
                    <Route path="tracked-positions/*" element={<AppRouter />} />
                </Routes>
            </MemoryRouter>
        );
        expect(screen.getByText('tracked-page')).toBeInTheDocument();
    });
});
