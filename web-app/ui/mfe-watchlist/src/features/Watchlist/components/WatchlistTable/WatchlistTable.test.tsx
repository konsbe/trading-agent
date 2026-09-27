import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter, Route, Routes, useLocation } from 'react-router-dom';
import { HostModeProvider } from '@/providers/HostModeContext';
import { findAsciiMinus } from '@/test-utils/asciiMinus';
import { makeNonCandidateItem, makeUncoveredItem, makeWatchlistItem } from '@/test-utils/fixtures';
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

const cell = (symbol: string, column: string) =>
    screen.getByTestId(`watchlist-row-${symbol}`).querySelector<HTMLElement>(`[data-column='${column}']`)!;

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
        expect(cell('VGZ', 'rvol_20')).toHaveTextContent(/^4\.20×$/);
    });

    it('tones a negative change down and leaves zero neutral', () => {
        renderTable({
            rows: [makeWatchlistItem({ symbol: 'DOWN', change_pct: -3.4 }), makeWatchlistItem({ symbol: 'FLAT', change_pct: 0 })],
        });

        expect(row('DOWN').getByTestId('change-value')).toHaveTextContent('−3.4%');
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
        ['rvol_20', 'dollar_volume', 'market_cap', 'rsi_14', 'breakout_state', 'pct_of_52w_high'].forEach(column =>
            expect(cell('DVY', column)).toHaveTextContent(/^—$/)
        );
        expect(cells.getByTestId('score-value')).toHaveTextContent(/^—$/);
    });

    it('never shows market values for a row without as_of, even if present', () => {
        renderTable({ rows: [makeWatchlistItem({ as_of: null, is_stale: true, close: 5, change_pct: 2, rvol_20: 3 })] });

        expect(row('VGZ').getByTestId('price-value')).toHaveTextContent('—');
        expect(row('VGZ').getByTestId('change-value')).toHaveTextContent('—');
        ['rvol_20', 'dollar_volume', 'market_cap', 'rsi_14', 'breakout_state', 'pct_of_52w_high'].forEach(column =>
            expect(cell('VGZ', column)).toHaveTextContent(/^—$/)
        );
        expect(row('VGZ').getByTestId('score-value')).toHaveTextContent(/^—$/);
    });

    it('shows "—" for a null RVOL', () => {
        renderTable({ rows: [makeWatchlistItem({ rvol_20: null })] });

        expect(cell('VGZ', 'rvol_20')).toHaveTextContent(/^—$/);
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

    it('renders negative changes with "−" and no ASCII hyphen-minus anywhere in the table (live NVDA −1.5%)', () => {
        renderTable({
            rows: [
                makeWatchlistItem({ symbol: 'NVDA', close: 225.51, change_pct: -1.4680823174728075, rvol_20: 0.6718976256402801 }),
                makeWatchlistItem({ symbol: 'TINY', close: 0.12, change_pct: -0.04 }),
            ],
        });

        expect(row('NVDA').getByTestId('change-value')).toHaveTextContent(/^−1\.5%$/);
        expect(findAsciiMinus(screen.getByTestId('watchlist-table'))).toEqual([]);
    });

    it('has no buy/sell or signal framing', () => {
        renderTable({ rows: [makeWatchlistItem(), makeNonCandidateItem()] });

        const text = screen.getByTestId('watchlist-table').textContent ?? '';
        expect(text).not.toMatch(/\b(buy|sell|alert|hot)\b/i);
    });

    describe('market columns (same as the candidates table)', () => {
        it("shows the candidates table's columns, labels and tooltips in its order, between Symbol and Remove", () => {
            renderTable();

            const headers = screen.getAllByRole('columnheader');
            expect(headers.map(th => th.textContent)).toEqual([
                'Symbol',
                'Close',
                'Change %',
                'RVOL',
                '$ Volume',
                'Market cap',
                'RSI',
                'Breakout',
                '% of 52w high',
                'Catalyst',
                'Score',
                'Actions',
            ]);
            const header = (column: string) => headers.find(th => th.getAttribute('data-column') === column)!;
            expect(header('rvol_20')).toHaveAttribute('title', 'Relative volume vs. the 20-day average');
            expect(header('rsi_14')).toHaveAttribute('title', '14-day RSI');
            expect(header('market_cap')).toHaveAttribute('title', expect.stringMatching(/estimate.*\(est\.\)/));
            expect(header('momentum_score_100')).toHaveAttribute('title', expect.stringMatching(/^Research score — unvalidated/));
            expect(header('breakout_state')).not.toHaveClass('is-numeric');
            expect(header('catalyst_tier')).not.toHaveClass('is-numeric');
            expect(header('dollar_volume')).toHaveClass('is-numeric');
            expect(header('momentum_score_100')).toHaveClass('is-numeric');
        });

        it('shows a candidate-today row with its "53/75 unvalidated" score', () => {
            renderTable({ rows: [makeWatchlistItem({ symbol: 'LOBO', momentum_score_100: 53, score_attainable: 75, is_candidate_today: true })] });

            const cells = row('LOBO');
            expect(cells.getByTestId('score-value')).toHaveTextContent(/^53$/);
            expect(cells.getByTestId('score-ceiling')).toHaveTextContent(/^\/75$/);
            expect(cells.getByTestId('score-status')).toHaveTextContent(/^unvalidated$/);
            expect(cells.getByTestId('score-label')).toHaveTextContent('Score 53 of 75 attainable, unvalidated');
            expect(cell('LOBO', 'momentum_score_100')).toHaveTextContent(/53\/75unvalidated$/);
        });

        it('shows a non-candidate row with no score (never 0) but real RSI, breakout, $ volume and market cap', () => {
            renderTable({ rows: [makeNonCandidateItem()] });

            const cells = row('NVDA');
            expect(cells.getByTestId('score-value')).toHaveTextContent(/^—$/);
            expect(cells.queryByTestId('score-ceiling')).not.toBeInTheDocument();
            expect(cells.getByTestId('score-status')).toHaveTextContent('unvalidated');
            expect(cells.getByTestId('score-label')).toHaveTextContent('No score, unvalidated');
            expect(cell('NVDA', 'momentum_score_100')).not.toHaveTextContent(/\b0\b/);

            expect(cell('NVDA', 'close')).toHaveTextContent(/^\$225\.51$/);
            expect(cell('NVDA', 'change_pct')).toHaveTextContent(/^−1\.5%$/);
            expect(cell('NVDA', 'rvol_20')).toHaveTextContent(/^0\.67×$/);
            expect(cell('NVDA', 'dollar_volume')).toHaveTextContent(/^\$38\.45B$/);
            expect(cell('NVDA', 'market_cap')).toHaveTextContent(/^\$5\.5T$/);
            expect(cell('NVDA', 'rsi_14')).toHaveTextContent(/^48$/);
            expect(cell('NVDA', 'breakout_state')).toHaveTextContent(/^none$/);
            expect(cell('NVDA', 'pct_of_52w_high')).toHaveTextContent(/^79\.00%$/);
        });

        it('marks a proxy market cap "(est.)" and formats breakout like the candidates table', () => {
            renderTable({
                rows: [
                    makeNonCandidateItem({
                        symbol: 'INFQ',
                        market_cap: null,
                        market_cap_est: 120e6,
                        market_cap_is_proxy: true,
                        breakout_state: 'breakout_from_consolidation',
                    }),
                ],
            });

            const cap = row('INFQ').getByTestId('market-cap-value');
            expect(cap).toHaveTextContent(/^\$120M \(est\.\)$/);
            expect(cap).toHaveClass('is-estimate');
            expect(cap).toHaveAttribute('title', 'Estimated: shares outstanding × close');
            expect(cell('INFQ', 'breakout_state')).toHaveTextContent(/^breakout from consolidation$/);
        });

        it('renders a null catalyst like the candidates table (empty), and resolved tiers as text', () => {
            renderTable({
                rows: [
                    makeWatchlistItem({ symbol: 'NUL', catalyst_tier: null }),
                    makeWatchlistItem({ symbol: 'NON', catalyst_tier: 'none' }),
                    makeWatchlistItem({ symbol: 'TA', catalyst_tier: 'A' }),
                ],
            });

            expect(cell('NUL', 'catalyst_tier')).toBeEmptyDOMElement();
            expect(cell('NON', 'catalyst_tier')).toHaveTextContent(/^None$/);
            expect(cell('TA', 'catalyst_tier')).toHaveTextContent(/^Tier A$/);
        });

        it('keeps a stale row\'s market columns (it still has data)', () => {
            renderTable({ rows: [makeNonCandidateItem({ as_of: '2026-09-21', is_stale: true })] });

            expect(row('NVDA').getByTestId('price-hint')).toHaveTextContent('as of Sep 21, 2026');
            expect(cell('NVDA', 'rsi_14')).toHaveTextContent(/^48$/);
        });
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
            await userEvent.click(cell('TSLA', 'rvol_20'));
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
        await userEvent.click(cell('VGZ', 'rvol_20'));
        expect(location()).toHaveTextContent(/^\/watchlist$/);
    });
});
