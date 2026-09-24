import { render, screen } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import AppRouter from './AppRouter';

jest.mock('@/pages/DataSourcePage', () => ({ __esModule: true, default: () => <div>data-source-page</div> }));

describe('AppRouter', () => {
    it('renders the Data Source page standalone at /', () => {
        render(
            <MemoryRouter initialEntries={['/']}>
                <AppRouter />
            </MemoryRouter>
        );
        expect(screen.getByText('data-source-page')).toBeInTheDocument();
    });

    it('renders the Data Source page hosted under spog data-source/*', () => {
        render(
            <MemoryRouter initialEntries={['/data-source']}>
                <Routes>
                    <Route path="data-source/*" element={<AppRouter />} />
                </Routes>
            </MemoryRouter>
        );
        expect(screen.getByText('data-source-page')).toBeInTheDocument();
    });
});
