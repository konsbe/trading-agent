import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import AppRouter from './AppRouter';

jest.mock('@/pages/HandbookPage', () => ({ __esModule: true, default: () => <div>handbook-page</div> }));
jest.mock('@/pages/MasterClassPage', () => ({ __esModule: true, default: () => <div>masterclass-page</div> }));
jest.mock('@/pages/GlossaryPage', () => ({ __esModule: true, default: () => <div>glossary-page</div> }));

const renderAt = (path: string) =>
    render(
        <MemoryRouter initialEntries={[path]}>
            <AppRouter />
        </MemoryRouter>
    );

describe('AppRouter (standalone)', () => {
    it('opens the Handbook at /', () => {
        renderAt('/');
        expect(screen.getByText('handbook-page')).toBeInTheDocument();
    });

    it.each([
        ['/handbook', 'handbook-page'],
        ['/masterclass', 'masterclass-page'],
        ['/glossary', 'glossary-page'],
    ])('renders %s', (path, text) => {
        renderAt(path);
        expect(screen.getByText(text)).toBeInTheDocument();
    });
});
