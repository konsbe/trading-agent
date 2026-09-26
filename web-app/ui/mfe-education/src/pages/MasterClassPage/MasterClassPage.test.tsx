import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { HostModeProvider } from '@/providers/HostModeContext';
import { masterClassFixture, mockResponse } from '@/test-utils/fixtures';
import MasterClassPage from './MasterClassPage';

const fetchMock = jest.fn();
const scrollIntoView = jest.fn();

const renderAt = (path = '/masterclass', { hosted = false } = {}) =>
    render(
        <MemoryRouter initialEntries={[path]}>
            <HostModeProvider hosted={hosted}>
                <MasterClassPage />
            </HostModeProvider>
        </MemoryRouter>
    );

describe('MasterClassPage', () => {
    beforeEach(() => {
        (global as any).fetch = fetchMock;
        fetchMock.mockReset();
        scrollIntoView.mockReset();
        Element.prototype.scrollIntoView = scrollIntoView;
    });

    it('shows a loading skeleton, then every module and entry', async () => {
        fetchMock.mockResolvedValue(mockResponse(200, masterClassFixture()));
        renderAt();

        expect(screen.getByRole('status', { name: 'Loading MasterClass' })).toBeInTheDocument();
        expect(await screen.findAllByTestId('masterclass-module')).toHaveLength(2);
        expect(screen.getAllByTestId('masterclass-entry').map(e => e.id)).toEqual(['rsi', 'macd', 'pe-ratio']);
        expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('MasterClass');
    });

    it('shows every summary with all explanations collapsed', async () => {
        fetchMock.mockResolvedValue(mockResponse(200, masterClassFixture()));
        renderAt();

        const toggles = await screen.findAllByRole('button', { name: 'Full explanation' });
        for (const toggle of toggles) await userEvent.click(toggle);

        expect(toggles.every(t => t.getAttribute('aria-expanded') === 'false')).toBe(true);
        expect(screen.getAllByTestId('entry-summary').map(s => s.textContent)).toEqual([
            'In shortRSI measures whether a stock has been bought or sold too fast recently.',
            'In shortMACD compares a fast and a slow average.',
            'In shortPrice divided by earnings per share.',
        ]);
    });

    it('lists modules and entries in the jump-nav as anchors', async () => {
        fetchMock.mockResolvedValue(mockResponse(200, masterClassFixture()));
        renderAt();

        const nav = await screen.findByRole('navigation', { name: 'MasterClass contents' });
        expect(within(nav).getAllByRole('link').map(a => [a.textContent, a.getAttribute('href')])).toEqual([
            ['Module 3 · Technical indicators, standards-based', '/masterclass#module-3'],
            ['RSI (Relative Strength Index)', '/masterclass#rsi'],
            ['MACD', '/masterclass#macd'],
            ['Fundamental analysis', '/masterclass#module-4'],
            ['P/E ratio', '/masterclass#pe-ratio'],
        ]);
    });

    it('deep-links: /masterclass#<entry-id> scrolls to the entry once loaded', async () => {
        fetchMock.mockResolvedValue(mockResponse(200, masterClassFixture()));
        renderAt('/masterclass#pe-ratio');

        await screen.findAllByTestId('masterclass-entry');
        expect(scrollIntoView.mock.contexts[0]).toBe(document.getElementById('pe-ratio'));
        expect(document.getElementById('pe-ratio')).toHaveFocus();
    });

    it('shows the error with a working Retry', async () => {
        fetchMock.mockResolvedValueOnce(mockResponse(200, { modules: 'nope' })).mockResolvedValueOnce(mockResponse(200, masterClassFixture()));
        renderAt();

        const alert = await screen.findByRole('alert');
        expect(alert).toHaveTextContent('The Education content service returned an unexpected response.');
        await userEvent.click(within(alert).getByRole('button', { name: 'Retry' }));

        expect(await screen.findAllByTestId('masterclass-module')).toHaveLength(2);
    });

    it('leaves the title to the shell header when hosted', async () => {
        fetchMock.mockResolvedValue(mockResponse(200, masterClassFixture()));
        renderAt('/masterclass', { hosted: true });

        await screen.findAllByTestId('masterclass-module');
        expect(screen.queryByRole('heading', { level: 1 })).not.toBeInTheDocument();
    });
});
