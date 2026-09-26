import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { HostModeProvider } from '@/providers/HostModeContext';
import { glossaryFixture, glossaryTerm, mockResponse } from '@/test-utils/fixtures';
import GlossaryPage from './GlossaryPage';

const fetchMock = jest.fn();

const terms = [
    glossaryTerm({ term: 'RSI', synonyms: ['Relative Strength Index'], definition: 'A momentum oscillator.' }),
    glossaryTerm({ term: 'P/E ratio', synonyms: ['PE ratio'], definition: 'Price over earnings.', entry_id: 'pe-ratio', section_or_module_id: 'module-4' }),
    glossaryTerm({ term: 'gate', definition: 'A pass/fail filter.', source: 'handbook', entry_id: 'gates', section_or_module_id: 'scanner' }),
];

const renderPage = ({ hosted = false } = {}) =>
    render(
        <MemoryRouter initialEntries={['/glossary']}>
            <HostModeProvider hosted={hosted}>
                <GlossaryPage />
            </HostModeProvider>
        </MemoryRouter>
    );

const rowTerms = () => screen.getAllByTestId('glossary-row').map(row => row.querySelector('dfn')!.textContent);

describe('GlossaryPage', () => {
    beforeEach(() => {
        (global as any).fetch = fetchMock;
        fetchMock.mockReset();
    });

    it('shows the calm empty state, with its exact text, when there are no terms (today)', async () => {
        fetchMock.mockResolvedValue(mockResponse(200, glossaryFixture([])));
        renderPage();

        const message = await screen.findByText('Glossary terms are added as Handbook and MasterClass sections are written');
        expect(message.textContent).toBe('Glossary terms are added as Handbook and MasterClass sections are written');
        expect(screen.queryByTestId('doc-skeleton')).not.toBeInTheDocument();
        expect(screen.queryByRole('alert')).not.toBeInTheDocument();
        expect(screen.queryByRole('img')).not.toBeInTheDocument();
        expect(screen.queryByRole('searchbox')).not.toBeInTheDocument();
    });

    it('shows a loading skeleton first', () => {
        fetchMock.mockReturnValue(new Promise(() => undefined));
        renderPage();

        expect(screen.getByRole('status', { name: 'Loading Glossary' })).toBeInTheDocument();
    });

    it('puts the search box first and lists terms alphabetically', async () => {
        fetchMock.mockResolvedValue(mockResponse(200, glossaryFixture(terms)));
        renderPage();

        const search = await screen.findByRole('searchbox', { name: 'Search terms' });
        expect(search.compareDocumentPosition(screen.getByTestId('glossary-list')) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
        expect(rowTerms()).toEqual(['gate', 'P/E ratio', 'RSI']);
        expect(screen.getByText('3 terms')).toBeInTheDocument();
    });

    it('filters on synonyms, case-insensitively', async () => {
        fetchMock.mockResolvedValue(mockResponse(200, glossaryFixture(terms)));
        renderPage();

        await userEvent.type(await screen.findByRole('searchbox'), 'pe RATIO');
        expect(rowTerms()).toEqual(['P/E ratio']);
        expect(screen.getByText('1 of 3 terms')).toBeInTheDocument();

        await userEvent.clear(screen.getByRole('searchbox'));
        await userEvent.type(screen.getByRole('searchbox'), 'strength');
        expect(rowTerms()).toEqual(['RSI']);
    });

    it('links each row to its entry anchor', async () => {
        fetchMock.mockResolvedValue(mockResponse(200, glossaryFixture(terms)));
        renderPage();

        const rows = await screen.findAllByTestId('glossary-row');
        expect(within(rows[0]).getByRole('link')).toHaveAttribute('href', '/handbook#gates');
        expect(within(rows[1]).getByRole('link')).toHaveAttribute('href', '/masterclass#pe-ratio');
    });

    it('shows a no-results state and keeps the search box', async () => {
        fetchMock.mockResolvedValue(mockResponse(200, glossaryFixture(terms)));
        renderPage();

        await userEvent.type(await screen.findByRole('searchbox'), '  zzz ');

        expect(screen.getByText('No terms match “zzz”.')).toBeInTheDocument();
        expect(screen.queryByTestId('glossary-list')).not.toBeInTheDocument();
        expect(screen.getByRole('searchbox')).toHaveValue('  zzz ');
        expect(screen.queryByText('Glossary terms are added as Handbook and MasterClass sections are written')).not.toBeInTheDocument();
    });

    it('shows the error with a working Retry', async () => {
        fetchMock.mockRejectedValueOnce(new TypeError('Failed to fetch')).mockResolvedValueOnce(mockResponse(200, glossaryFixture([])));
        renderPage();

        const alert = await screen.findByRole('alert');
        expect(alert).toHaveTextContent("Couldn't reach the Education content service.");
        await userEvent.click(within(alert).getByRole('button', { name: 'Retry' }));

        expect(await screen.findByText('Glossary terms are added as Handbook and MasterClass sections are written')).toBeInTheDocument();
    });

    it('shows its title standalone and leaves it to the shell header when hosted', async () => {
        fetchMock.mockResolvedValue(mockResponse(200, glossaryFixture([])));
        const { unmount } = renderPage();
        expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('Glossary');
        unmount();

        renderPage({ hosted: true });
        await screen.findByText('Glossary terms are added as Handbook and MasterClass sections are written');
        expect(screen.queryByRole('heading', { level: 1 })).not.toBeInTheDocument();
    });
});
