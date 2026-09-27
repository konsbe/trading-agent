import { render, screen } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import AppRouter from './AppRouter';

jest.mock('@/pages/AlarmHistoryPage', () => ({ __esModule: true, default: () => <div>alarm-history-page</div> }));

describe('AppRouter', () => {
    it('renders the alarm history page standalone at /', () => {
        render(
            <MemoryRouter initialEntries={['/']}>
                <AppRouter />
            </MemoryRouter>
        );
        expect(screen.getByText('alarm-history-page')).toBeInTheDocument();
    });

    it('renders the alarm history page hosted under spog alarm-history/*', () => {
        render(
            <MemoryRouter initialEntries={['/alarm-history']}>
                <Routes>
                    <Route path="alarm-history/*" element={<AppRouter />} />
                </Routes>
            </MemoryRouter>
        );
        expect(screen.getByText('alarm-history-page')).toBeInTheDocument();
    });
});
