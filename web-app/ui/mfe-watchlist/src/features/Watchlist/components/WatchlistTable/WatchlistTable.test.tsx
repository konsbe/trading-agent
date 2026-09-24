import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter, Route, Routes, useLocation } from 'react-router-dom';
import { HostModeProvider } from '@/providers/HostModeContext';
import { makeUncoveredItem, makeWatchlistItem } from '@/test-utils/fixtures';
import WatchlistTable from './WatchlistTable';
import { WatchlistTableProps } from './types';

const Location = () => <span data-testid="location">{useLocation().pathname}</span>;

/** Mounted like spog does: under `/watchlist/*`, next to the scanner's `/candidates/*`. */
const renderTable = (overrides: Partial<WatchlistTableProps> = {}, { hosted = false } = {}) => {
    const props: WatchlistTableProps = {
        id: 'watchlist-table',
        caption: 'Watched symbols',
        rows: [makeWatchlistItem()],
        saving: new Set(),
        onRemove: jest.fn(),
        ...overrides,
    };
    render(
        <HostModeProvider hosted={hosted}>
            <MemoryRouter initialEntries={['/watchlist']}>
                <Location />
                <Routes>
                    <Route path="/watchlist/*" element={<WatchlistTable {...props} />} />
                    <Route path="/candidates/:symbol" element={<>detail</>} />
                </Routes>
            </MemoryRouter>
        </HostModeProvider>
    );
    return props;
};

const location = () => screen.getByTestId('location');

const row = (symbol: string) => within(screen.getByTestId(`watchlist-row-${symbol}`));

describe('WatchlistTable', () => {
    it('renders the identity cell like the candidates table', () => {
        renderTable({ rows: [makeWatchlistItem({ symbol: 'FSLY', company_name: 'FASTLY INC', exchange: 'NASDAQ' })] });

        const cells = row('FSLY');
        expect(cells.getByRole('rowheader')).toHaveTextContent('FSLY');
        expect(cells.getByTestId('company-name')).toHaveTextContent('FASTLY INC');
        expect(cells.getByText('NASDAQ')).toHaveClass('watchlist-table__exchange');
        expect(screen.getByRole('table', { name: 'Watched symbols' })).toBeInTheDocument();
    });

    it('shows "—" for a missing company name or exchange', () => {
        renderTable({ rows: [makeWatchlistItem({ company_name: null, exchange: null })] });

        expect(row('VGZ').getByTestId('company-name')).toHaveTextContent('—');
    });

    it('shows a current price, the change toned up, and RVOL as a multiple', () => {
        renderTable({ rows: [makeWatchlistItem({ close: 29.65, change_pct: 13.6886, rvol_20: 4.2, is_stale: false })] });

        const cells = row('VGZ');
        expect(cells.getByTestId('price-value')).toHaveTextContent('$29.65');
        expect(cells.queryByTestId('price-hint')).not.toBeInTheDocument();
        expect(cells.getByTestId('change-value')).toHaveTextContent('+13.7%');
        expect(cells.getByTestId('change-value')).toHaveClass('is-price-up');
        expect(cells.getByTestId('rvol-value')).toHaveTextContent('4.20×');
    });

    it('tones a negative change down and leaves zero neutral', () => {
        renderTable({
            rows: [makeWatchlistItem({ symbol: 'DOWN', change_pct: -3.4 }), makeWatchlistItem({ symbol: 'FLAT', change_pct: 0 })],
        });

        expect(row('DOWN').getByTestId('change-value')).toHaveTextContent('-3.4%');
        expect(row('DOWN').getByTestId('change-value')).toHaveClass('is-price-down');
        expect(row('FLAT').getByTestId('change-value')).toHaveTextContent('0.0%');
        expect(row('FLAT').getByTestId('change-value')).not.toHaveClass('is-price-up');
        expect(row('FLAT').getByTestId('change-value')).not.toHaveClass('is-price-down');
    });

    it('keeps sub-dollar prices at four decimals', () => {
        renderTable({ rows: [makeWatchlistItem({ close: 0.91 })] });

        expect(row('VGZ').getByTestId('price-value')).toHaveTextContent('$0.9100');
    });

    it('marks a stale price with a neutral "as of" note', () => {
        renderTable({ rows: [makeWatchlistItem({ as_of: '2026-09-21', is_stale: true, close: 1.2 })] });

        const hint = row('VGZ').getByTestId('price-hint');
        expect(hint).toHaveTextContent('as of Sep 21, 2026');
        expect(hint).toHaveClass('watchlist-table__hint');
        expect(row('VGZ').getByTestId('price-value')).toHaveTextContent('$1.20');
    });

    it('shows "—" and "No price data" when there is no features row', () => {
        renderTable({ rows: [makeUncoveredItem('DVY')] });

        const cells = row('DVY');
        expect(cells.getByTestId('price-value')).toHaveTextContent('—');
        expect(cells.getByTestId('price-hint')).toHaveTextContent('No price data');
        expect(cells.getByTestId('change-value')).toHaveTextContent('—');
        expect(cells.getByTestId('change-value')).not.toHaveClass('is-price-up');
        expect(cells.getByTestId('rvol-value')).toHaveTextContent('—');
    });

    it('never shows market values for a row without as_of, even if present', () => {
        renderTable({ rows: [makeWatchlistItem({ as_of: null, is_stale: true, close: 5, change_pct: 2, rvol_20: 3 })] });

        expect(row('VGZ').getByTestId('price-value')).toHaveTextContent('—');
        expect(row('VGZ').getByTestId('change-value')).toHaveTextContent('—');
        expect(row('VGZ').getByTestId('rvol-value')).toHaveTextContent('—');
    });

    it('shows "—" for a null RVOL', () => {
        renderTable({ rows: [makeWatchlistItem({ rvol_20: null })] });

        expect(row('VGZ').getByTestId('rvol-value')).toHaveTextContent('—');
    });

    it('says "Adding…" for an optimistic row still being saved', () => {
        renderTable({ rows: [makeUncoveredItem('NEW')], saving: new Set(['NEW']) });

        expect(row('NEW').getByTestId('price-hint')).toHaveTextContent('Adding…');
    });

    it('removes a row by its labelled action', async () => {
        const { onRemove } = renderTable();

        await userEvent.click(screen.getByRole('button', { name: 'Remove VGZ from watchlist' }));

        expect(onRemove).toHaveBeenCalledWith('VGZ');
    });

    it('disables remove while that symbol is saving', () => {
        renderTable({
            rows: [makeWatchlistItem({ symbol: 'A' }), makeWatchlistItem({ symbol: 'B' })],
            saving: new Set(['A']),
        });

        expect(screen.getByRole('button', { name: 'Remove A from watchlist' })).toBeDisabled();
        expect(screen.getByRole('button', { name: 'Remove B from watchlist' })).toBeEnabled();
    });

    it('has no score, breakout or signal framing', () => {
        renderTable();

        const text = screen.getByTestId('watchlist-table').textContent ?? '';
        expect(text).not.toMatch(/score|breakout|buy|sell|alert|hot/i);
    });

    describe('opening the scanner detail page (hosted)', () => {
        const hosted = (overrides: Partial<WatchlistTableProps> = {}) => renderTable(overrides, { hosted: true });

        it("makes the ticker a real link to the shell's /candidates/<SYMBOL> route", () => {
            hosted({ rows: [makeWatchlistItem({ symbol: 'BRK.B' })] });

            const link = row('BRK.B').getByRole('link', { name: 'BRK.B' });
            expect(link).toHaveAttribute('href', '/candidates/BRK.B');
            expect(link).toHaveClass('watchlist-table__ticker');
        });

        it('navigates within the shell when the ticker is clicked', async () => {
            hosted();
            await userEvent.click(screen.getByRole('link', { name: 'VGZ' }));
            expect(location()).toHaveTextContent(/^\/candidates\/VGZ$/);
        });

        it('opens the detail from anywhere on the row, and with Enter on the focused row', async () => {
            hosted({ rows: [makeWatchlistItem({ symbol: 'TSLA' }), makeWatchlistItem({ symbol: 'FSLY' })] });

            const tsla = screen.getByTestId('watchlist-row-TSLA');
            expect(tsla).toHaveAttribute('tabindex', '0');
            expect(tsla).toHaveAttribute('aria-label', 'TSLA, open details');
            await userEvent.click(within(tsla).getByTestId('rvol-value'));
            expect(location()).toHaveTextContent(/^\/candidates\/TSLA$/);
        });

        it('activates with Enter only when the row itself has focus', async () => {
            hosted({ rows: [makeWatchlistItem({ symbol: 'FSLY' })] });

            screen.getByRole('button', { name: 'Remove FSLY from watchlist' }).focus();
            await userEvent.keyboard('{Enter}');
            expect(location()).toHaveTextContent(/^\/watchlist$/);

            screen.getByTestId('watchlist-row-FSLY').focus();
            await userEvent.keyboard('{Enter}');
            expect(location()).toHaveTextContent(/^\/candidates\/FSLY$/);
        });

        it('never navigates from Remove, by click or keyboard, even while it is disabled', async () => {
            const { onRemove } = hosted({
                rows: [makeWatchlistItem({ symbol: 'A' }), makeWatchlistItem({ symbol: 'B' })],
                saving: new Set(['B']),
            });

            await userEvent.click(screen.getByRole('button', { name: 'Remove A from watchlist' }));
            expect(onRemove).toHaveBeenCalledWith('A');
            expect(location()).toHaveTextContent(/^\/watchlist$/);

            screen.getByRole('button', { name: 'Remove A from watchlist' }).focus();
            await userEvent.keyboard(' ');
            expect(location()).toHaveTextContent(/^\/watchlist$/);

            await userEvent.click(screen.getByRole('button', { name: 'Remove B from watchlist' }));
            expect(location()).toHaveTextContent(/^\/watchlist$/);
        });

        it("doesn't link a symbol with no stored data: its detail page would be a 404", async () => {
            hosted({ rows: [makeUncoveredItem('DVY')] });

            const dvy = screen.getByTestId('watchlist-row-DVY');
            expect(within(dvy).queryByRole('link')).not.toBeInTheDocument();
            expect(dvy).not.toHaveAttribute('tabindex');
            expect(dvy).not.toHaveClass('is-linked');
            await userEvent.click(within(dvy).getByTestId('price-value'));
            expect(location()).toHaveTextContent(/^\/watchlist$/);
        });

        it('links a stale symbol (it still has data)', () => {
            hosted({ rows: [makeWatchlistItem({ symbol: 'BRR', as_of: '2026-09-21', is_stale: true })] });
            expect(row('BRR').getByRole('link', { name: 'BRR' })).toHaveAttribute('href', '/candidates/BRR');
        });
    });

    it("doesn't link anything standalone, where the scanner's detail route doesn't exist", async () => {
        renderTable();

        expect(screen.queryByRole('link')).not.toBeInTheDocument();
        expect(screen.getByTestId('watchlist-row-VGZ')).not.toHaveAttribute('tabindex');
        await userEvent.click(row('VGZ').getByTestId('rvol-value'));
        expect(location()).toHaveTextContent(/^\/watchlist$/);
    });
});
