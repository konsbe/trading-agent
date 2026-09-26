import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { HostModeProvider } from '@/providers/HostModeContext';
import { CAVEAT_TEXT, handbookFixture, mockResponse } from '@/test-utils/fixtures';
import HandbookPage from './HandbookPage';

const fetchMock = jest.fn();
const scrollIntoView = jest.fn();

const renderAt = (path = '/handbook', { hosted = false } = {}) =>
    render(
        <MemoryRouter initialEntries={[path]}>
            <HostModeProvider hosted={hosted}>
                <HandbookPage />
            </HostModeProvider>
        </MemoryRouter>
    );

describe('HandbookPage', () => {
    beforeEach(() => {
        (global as any).fetch = fetchMock;
        fetchMock.mockReset();
        scrollIntoView.mockReset();
        Element.prototype.scrollIntoView = scrollIntoView;
    });

    it('shows a loading skeleton, then the Handbook', async () => {
        fetchMock.mockResolvedValue(mockResponse(200, handbookFixture()));
        renderAt();

        expect(screen.getByRole('status', { name: 'Loading Handbook' })).toBeInTheDocument();
        expect(await screen.findByRole('region', { name: 'Stock Detail — full analysis' })).toBeInTheDocument();
        expect(screen.queryByTestId('doc-skeleton')).not.toBeInTheDocument();
        expect(screen.getByTestId('content-status')).toHaveTextContent('Draft · version 0.1.0');
        expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('Handbook');
    });

    it('renders the caveat text exactly as the API served it', async () => {
        fetchMock.mockResolvedValue(mockResponse(200, handbookFixture()));
        renderAt();

        expect((await screen.findByTestId('caveat-text')).textContent).toBe(CAVEAT_TEXT);
    });

    it('lists every section and entry in the jump-nav as anchors', async () => {
        fetchMock.mockResolvedValue(mockResponse(200, handbookFixture()));
        renderAt();

        const nav = await screen.findByRole('navigation', { name: 'Handbook contents' });
        expect(within(nav).getAllByRole('link').map(a => a.getAttribute('href'))).toEqual([
            '/handbook#stock-detail',
            '/handbook#classical-technical-signals',
            '/handbook#severity-badges',
            '/handbook#backtest-lab',
            '/handbook#committed-before-result',
        ]);
        for (const link of within(nav).getAllByRole('link')) {
            expect(document.getElementById(link.getAttribute('href')!.split('#')[1])).not.toBeNull();
        }
    });

    it('scrolls to an entry from the jump-nav', async () => {
        fetchMock.mockResolvedValue(mockResponse(200, handbookFixture()));
        renderAt();

        await userEvent.click(await screen.findByRole('link', { name: 'Severity badges' }));

        expect(scrollIntoView.mock.contexts.at(-1)).toBe(document.getElementById('severity-badges'));
        expect(document.getElementById('severity-badges')).toHaveFocus();
    });

    it('deep-links: /handbook#<entry-id> scrolls to the entry once loaded', async () => {
        fetchMock.mockResolvedValue(mockResponse(200, handbookFixture()));
        renderAt('/handbook#committed-before-result');

        await screen.findByRole('region', { name: 'Backtest Lab' });
        expect(scrollIntoView.mock.contexts[0]).toBe(document.getElementById('committed-before-result'));
    });

    it('shows the error with a working Retry', async () => {
        fetchMock.mockResolvedValueOnce(mockResponse(500, { error: 'internal_error' })).mockResolvedValueOnce(mockResponse(200, handbookFixture()));
        renderAt();

        const alert = await screen.findByRole('alert');
        expect(alert).toHaveTextContent('The Education content service hit an internal error.');
        await userEvent.click(within(alert).getByRole('button', { name: 'Retry' }));

        expect(await screen.findByRole('region', { name: 'Stock Detail — full analysis' })).toBeInTheDocument();
        expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    });

    it('names the Handbook in the fallback error copy', async () => {
        fetchMock.mockResolvedValue(mockResponse(404, undefined));
        renderAt();

        expect(await screen.findByRole('alert')).toHaveTextContent('Something went wrong loading the Handbook.');
    });

    it('leaves the title to the shell header when hosted', async () => {
        fetchMock.mockResolvedValue(mockResponse(200, handbookFixture()));
        renderAt('/handbook', { hosted: true });

        await screen.findByRole('region', { name: 'Stock Detail — full analysis' });
        expect(screen.queryByRole('heading', { level: 1 })).not.toBeInTheDocument();
    });
});
