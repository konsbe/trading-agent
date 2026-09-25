import { render, screen } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import AppRouter from './AppRouter';

jest.mock('@/pages/MarketReportPage', () => ({ __esModule: true, default: () => <div>market-report-page</div> }));

describe('AppRouter', () => {
    it('renders the Daily Market Report page standalone at /', () => {
        render(
            <MemoryRouter initialEntries={['/']}>
                <AppRouter />
            </MemoryRouter>
        );
        expect(screen.getByText('market-report-page')).toBeInTheDocument();
    });

    it('renders the Daily Market Report page hosted under spog market-report/*', () => {
        render(
            <MemoryRouter initialEntries={['/market-report']}>
                <Routes>
                    <Route path="market-report/*" element={<AppRouter />} />
                </Routes>
            </MemoryRouter>
        );
        expect(screen.getByText('market-report-page')).toBeInTheDocument();
    });
});
