import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter, Route, Routes, useLocation } from 'react-router-dom';
import { fetchTracked } from '@/api';
import { HostModeProvider } from '@/providers/HostModeContext';
import { LOST_VWAP_NOTE, makeActiveRow, makeClosedRow, makeTracked, makeUnevaluatedRow } from '@/test-utils/tracked';
import TrackedPositionsPage from '.';

jest.mock('@/api/tracked/trackedApi', () => ({ fetchTracked: jest.fn() }));

const fetchMock = fetchTracked as jest.MockedFunction<typeof fetchTracked>;

const ROWS = [
    makeActiveRow({ symbol: 'RSKD', alerted_date: '2026-09-24', unrealized_pct: -1.6 }),
    makeUnevaluatedRow(),
    makeActiveRow({ symbol: 'INVZ', alerted_date: '2026-09-23', sessions_elapsed: 2, unrealized_pct: 2.5 }),
    makeActiveRow({ symbol: 'DNA', alerted_date: '2026-09-24', unrealized_pct: 4 }),
    makeClosedRow({ symbol: 'EZGO', alerted_date: '2026-09-22', exit_pct: -16.5 }),
    makeClosedRow({ symbol: 'IFBD', alerted_date: '2026-09-24', exit_reason: 'lost_vwap', exit_reason_note: LOST_VWAP_NOTE, exit_pct: 18.6 }),
    makeClosedRow({ symbol: 'AMIX', alerted_date: '2026-09-24', exit_pct: null }),
];

const Location = () => {
    const { pathname, search } = useLocation();
    return <span data-testid="location">{`${pathname}${search}`}</span>;
};
const Detail = () => <span data-testid="detail-state">{JSON.stringify(useLocation().state)}</span>;

const renderPage = async (url = '/tracked-positions', hosted = true) => {
    fetchMock.mockResolvedValue(makeTracked(ROWS));
    render(
        <MemoryRouter initialEntries={[url]}>
            <HostModeProvider hosted={hosted}>
                <Location />
                <Routes>
                    <Route path="/tracked-positions/*" element={<TrackedPositionsPage />} />
                    <Route path="/candidates/:symbol" element={<Detail />} />
                </Routes>
            </HostModeProvider>
        </MemoryRouter>
    );
    await screen.findByRole('tablist');
};

const symbols = () =>
    screen
        .queryAllByTestId(/^tracked-row-/)
        .map(el => el.getAttribute('data-testid')!.replace('tracked-row-', '').replace(/-\d{4}-\d{2}-\d{2}$/, ''));
const location = () => screen.getByTestId('location').textContent;

beforeEach(() => window.sessionStorage.clear());

describe('TrackedPositionsPage table view', () => {
    it('lists Active most recent alert first, ties by symbol', async () => {
        await renderPage();

        expect(symbols()).toEqual(['ABLV', 'DNA', 'RSKD', 'INVZ']);
        expect(screen.getByTestId('tracked-active-sort-label')).toHaveTextContent('Newest alert first');
        expect(screen.getByRole('columnheader', { name: 'Alerted Date' })).toHaveAttribute('aria-sort', 'descending');
    });

    it('lists Closed most recent alert first too', async () => {
        await renderPage('/tracked-positions?tab=closed');
        expect(symbols()).toEqual(['AMIX', 'IFBD', 'EZGO']);
    });

    it('sorts every column, "—" last both ways, in the URL', async () => {
        await renderPage('/tracked-positions?tab=closed');
        const table = screen.getByTestId('tracked-closed-table');
        for (const name of ['Symbol / Exchange / Bucket', 'Alerted Date', 'Closed Date', 'Sessions Elapsed', 'Reference Price', 'Exit Reason', 'Exit %']) {
            expect(within(table).getByRole('button', { name })).toBeInTheDocument();
        }

        await userEvent.click(within(table).getByRole('button', { name: 'Exit %' }));
        expect(symbols()).toEqual(['IFBD', 'EZGO', 'AMIX']);
        expect(location()).toBe('/tracked-positions?tab=closed&closed_sort=exit_pct%3Adesc');
        await userEvent.click(within(table).getByRole('button', { name: 'Exit %' }));
        expect(symbols()).toEqual(['EZGO', 'IFBD', 'AMIX']);
        expect(screen.getByTestId('tracked-closed-sort-label')).toHaveTextContent('Sorted by Exit %, ascending');
    });

    it('keeps "Not yet evaluated" last when sorting sessions and Unrealized %', async () => {
        await renderPage();

        await userEvent.click(screen.getByRole('button', { name: 'Unrealized %' }));
        expect(symbols()).toEqual(['DNA', 'INVZ', 'RSKD', 'ABLV']);
        await userEvent.click(screen.getByRole('button', { name: 'Sessions Elapsed' }));
        expect(symbols().at(-1)).toBe('ABLV');
        await userEvent.click(screen.getByRole('button', { name: 'Sessions Elapsed' }));
        expect(symbols().at(-1)).toBe('ABLV');
    });

    it('searches the text shown (exit reason) and restores sort + search + tab from the URL', async () => {
        await renderPage('/tracked-positions?tab=closed&closed_sort=symbol%3Aasc&closed_q=breakout');

        expect(screen.getByRole('tab', { name: /Closed/ })).toHaveAttribute('aria-selected', 'true');
        expect(symbols()).toEqual(['AMIX', 'EZGO']);
        expect(screen.getByTestId('tracked-closed-search-input')).toHaveValue('breakout');

        await userEvent.clear(screen.getByTestId('tracked-closed-search-input'));
        await userEvent.type(screen.getByTestId('tracked-closed-search-input'), 'nothing');
        expect(await screen.findByTestId('tracked-closed-no-match')).toHaveTextContent('No closed tracked positions match “nothing”');
    });

    it('keeps each tab its own sort', async () => {
        await renderPage('/tracked-positions?active_sort=symbol%3Aasc');
        expect(symbols()).toEqual(['ABLV', 'DNA', 'INVZ', 'RSKD']);

        await userEvent.click(screen.getByRole('tab', { name: /Closed/ }));
        await waitFor(() => expect(symbols()).toEqual(['AMIX', 'IFBD', 'EZGO']));
        expect(location()).toBe('/tracked-positions?active_sort=symbol%3Aasc&tab=closed');
    });

    it('opens Stock Detail with "Back to Tracked Positions" and this tab, sort and search', async () => {
        await renderPage('/tracked-positions?tab=closed&closed_sort=exit_pct%3Adesc&closed_q=i');

        await userEvent.click(screen.getByRole('link', { name: 'IFBD' }));
        expect(JSON.parse(screen.getByTestId('detail-state').textContent!)).toEqual({
            from: '/tracked-positions?tab=closed&closed_sort=exit_pct%3Adesc&closed_q=i',
            fromLabel: 'Tracked Positions',
        });
    });

    it('links nothing standalone', async () => {
        await renderPage('/tracked-positions', false);
        expect(screen.queryAllByRole('link')).toHaveLength(0);
    });
});
