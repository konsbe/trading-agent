import { render, screen } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import AppRouter from './AppRouter';

jest.mock('@/pages/BacktestLabPage', () => ({ __esModule: true, default: () => <div>backtest-lab-page</div> }));

describe('AppRouter', () => {
    it('renders the Backtest Lab page standalone at /', () => {
        render(
            <MemoryRouter initialEntries={['/']}>
                <AppRouter />
            </MemoryRouter>
        );
        expect(screen.getByText('backtest-lab-page')).toBeInTheDocument();
    });

    it('renders the Backtest Lab page hosted under spog backtest-lab/*', () => {
        render(
            <MemoryRouter initialEntries={['/backtest-lab']}>
                <Routes>
                    <Route path="backtest-lab/*" element={<AppRouter />} />
                </Routes>
            </MemoryRouter>
        );
        expect(screen.getByText('backtest-lab-page')).toBeInTheDocument();
    });
});
