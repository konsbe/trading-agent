import { render, screen } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import AppRouter from './AppRouter';

jest.mock('@/pages/CandidatesPage', () => ({ __esModule: true, default: () => <div>candidates-page</div> }));
jest.mock('@/pages/CandidateDetailPage', () => {
    const { useParams } = require('react-router-dom');
    return { __esModule: true, default: () => <div>detail-page {useParams().symbol}</div> };
});

describe('AppRouter', () => {
    it.each([
        ['standalone at /', '/', 'candidates-page'],
        ['standalone at /:symbol', '/VGZ', 'detail-page VGZ'],
    ])('%s', (_label, path, text) => {
        render(
            <MemoryRouter initialEntries={[path]}>
                <AppRouter />
            </MemoryRouter>
        );
        expect(screen.getByText(text)).toBeInTheDocument();
    });

    it.each([
        ['/candidates', 'candidates-page'],
        ['/candidates/NEXR', 'detail-page NEXR'],
    ])('hosted under spog candidates/*: %s', (path, text) => {
        render(
            <MemoryRouter initialEntries={[path]}>
                <Routes>
                    <Route path="candidates/*" element={<AppRouter />} />
                </Routes>
            </MemoryRouter>
        );
        expect(screen.getByText(text)).toBeInTheDocument();
    });
});
