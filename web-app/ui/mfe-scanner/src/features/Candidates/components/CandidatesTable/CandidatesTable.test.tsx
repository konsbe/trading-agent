import { readFileSync } from 'fs';
import { join } from 'path';
import { render, screen, within } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { makeCandidate } from '@/test-utils/fixtures';
import { DEFAULT_SORT, SortState } from '../../utils/sortCandidates';
import CandidatesTable from './CandidatesTable';

const renderTable = (rows = [makeCandidate({ symbol: 'LOBO', company_name: 'LOBO TECHNOLOGIES LTD-A', exchange: 'NASDAQ' })]) =>
    render(
        <MemoryRouter>
            <CandidatesTable id="t" caption="Market candidates" rows={rows} sort={DEFAULT_SORT} onSort={jest.fn()} />
        </MemoryRouter>
    );

/** jsdom does not apply stylesheets, so layout guarantees are checked against the CSS source. */
const css = readFileSync(join(__dirname, 'CandidatesTable-styles.css'), 'utf8');
const rule = (selector: string) => {
    const escaped = selector.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
    const match = new RegExp(`(^|\\n|,\\s*)${escaped}\\s*(,[^{]*)?\\{([^}]*)\\}`).exec(css);
    if (!match) throw new Error(`No CSS rule for ${selector}`);
    return match[3];
};

describe('CandidatesTable', () => {
    it('keeps the full company name in the DOM and in a title for the ellipsized cell', () => {
        renderTable();

        const company = screen.getByTestId('company-name');
        expect(company).toHaveTextContent(/^LOBO TECHNOLOGIES LTD-A$/);
        expect(company).toHaveAttribute('title', 'LOBO TECHNOLOGIES LTD-A');
        expect(within(screen.getByTestId('candidate-row-LOBO')).getByText('NASDAQ')).toBeInTheDocument();
    });

    it('renders "—" without a title when the company name is null', () => {
        renderTable([makeCandidate({ symbol: 'NUL', company_name: null })]);

        const company = screen.getByTestId('company-name');
        expect(company).toHaveTextContent(/^—$/);
        expect(company).not.toHaveAttribute('title');
    });

    it('wraps the table in a labelled, keyboard-focusable scroll region', () => {
        renderTable();

        const region = screen.getByRole('region', { name: 'Market candidates, scrolls horizontally' });
        expect(region).toHaveAttribute('tabindex', '0');
        expect(region).toHaveClass('scanner-table__wrap');
        expect(within(region).getByRole('table', { name: 'Market candidates' })).toBeInTheDocument();
    });

    describe('layout CSS', () => {
        it('scrolls horizontally inside the card instead of widening (and being clipped by) the shell', () => {
            const wrap = rule('.scanner-table__wrap');
            expect(wrap).toMatch(/overflow-x:\s*auto/);
            expect(wrap).toMatch(/contain:\s*inline-size/);
            expect(wrap).toMatch(/min-width:\s*0/);
            expect(wrap).not.toMatch(/overflow(-x)?:\s*hidden/);
        });

        it('pins the Symbol column so scrolling never hides the ticker', () => {
            const sticky = rule(".scanner-table__row > th[data-column='symbol']");
            expect(sticky).toMatch(/position:\s*sticky/);
            expect(sticky).toMatch(/left:\s*0/);
        });

        it('gives the name a readable floor and keeps the exchange from shrinking it away', () => {
            expect(rule('.scanner-table__symbol')).toMatch(/min-width:\s*10rem/);
            expect(rule('.scanner-table__symbol')).toMatch(/width:\s*clamp\(10rem,\s*19cqi,\s*20rem\)/);
            expect(rule('.scanner-table__wrap')).toMatch(/container-type:\s*inline-size/);
            expect(rule('.scanner-table__company')).toMatch(/text-overflow:\s*ellipsis/);
            expect(rule('.scanner-table__company')).toMatch(/min-width:\s*8ch/);
            expect(rule('.scanner-table__exchange')).toMatch(/flex:\s*0 0 auto/);
        });

        it('keeps Breakout on one line when the table area is wide, and compacts the stacked symbol lines', () => {
            expect(css).toMatch(/@container \(min-width: 1320px\)\s*\{\s*\.scanner-table td\[data-column='breakout_state'\]\s*\{\s*max-width: none;\s*white-space: nowrap;/);
            expect(rule('.scanner-table__wrap')).toMatch(/container-type:\s*inline-size/);
            expect(rule('.scanner-table__symbol')).toMatch(/line-height:\s*1\.3/);
        });

        it('lets Breakout wrap and keeps each score on one line with its marker', () => {
            expect(rule(".scanner-table td[data-column='breakout_state']")).toMatch(/white-space:\s*normal/);
            expect(rule(".scanner-table td[data-column='momentum_score_100']")).toMatch(/white-space:\s*nowrap/);
        });

        it('keeps header labels on one line once the table area is wide enough, wrapping below that', () => {
            expect(rule('.scanner-table thead th')).toMatch(/white-space:\s*normal/);
            expect(css).toMatch(/@container \(min-width: 1190px\)\s*\{\s*\.scanner-table thead th\s*\{\s*white-space: nowrap;/);
        });

        it('centres the sort indicator on the label in a fixed box, so no state shifts the label', () => {
            expect(rule('.scanner-table__sort')).toMatch(/align-items:\s*center/);
            const indicator = rule('.scanner-table__sort-indicator');
            expect(indicator).toMatch(/flex:\s*0 0 auto/);
            expect(indicator).toMatch(/width:\s*8px/);
            expect(indicator).toMatch(/height:\s*12px/);
            expect(indicator).toMatch(/align-items:\s*center/);
            expect(indicator).toMatch(/justify-content:\s*center/);
        });
    });

    describe('market cap column', () => {
        const cell = (symbol: string) =>
            within(screen.getByTestId(`candidate-row-${symbol}`)).getByTestId('market-cap-value');

        it('sits right after $ Volume as a numeric column with an estimate-aware tooltip', () => {
            renderTable();
            const headers = screen.getAllByRole('columnheader').map(th => th.getAttribute('data-column'));
            expect(headers.indexOf('market_cap')).toBe(headers.indexOf('dollar_volume') + 1);

            const header = screen.getByRole('columnheader', { name: /^Market cap/ });
            expect(header).toHaveClass('is-numeric');
            expect(header).toHaveAttribute('title', expect.stringMatching(/estimate.*\(est\.\)/));
            expect(cell('LOBO').closest('td')).toHaveClass('is-numeric');
        });

        it('renders a reported value unmarked, an estimate with its marker, and "—" when neither exists', () => {
            renderTable([
                makeCandidate({ symbol: 'REP', market_cap: 4328181000, market_cap_est: null, market_cap_is_proxy: false }),
                makeCandidate({ symbol: 'EST', market_cap: null, market_cap_est: 120e6, market_cap_is_proxy: true }),
                makeCandidate({ symbol: 'PRX', market_cap: 390473360, market_cap_est: null, market_cap_is_proxy: true }),
                makeCandidate({ symbol: 'NIL', market_cap: null, market_cap_est: null, market_cap_is_proxy: false }),
            ]);

            expect(cell('REP')).toHaveTextContent(/^\$4\.3B$/);
            expect(cell('REP')).not.toHaveClass('is-estimate');
            expect(cell('REP')).not.toHaveAttribute('title');

            expect(cell('EST')).toHaveTextContent(/^\$120M \(est\.\)$/);
            expect(cell('EST')).toHaveClass('is-estimate');
            expect(cell('EST')).toHaveAttribute('title', 'Estimated: shares outstanding × close');
            expect(cell('PRX')).toHaveTextContent(/^\$390M \(est\.\)$/);

            expect(cell('NIL')).toHaveTextContent(/^—$/);
            expect(cell('NIL')).not.toHaveClass('is-estimate');
        });

        it('stays neutral (never price-toned)', () => {
            renderTable([makeCandidate({ symbol: 'REP' })]);
            expect(cell('REP').closest('td')!.querySelector('.is-price-up, .is-price-down')).toBeNull();
            expect(cell('REP')).not.toHaveClass('is-price-up');
            expect(cell('REP')).not.toHaveClass('is-price-down');
        });
    });

    describe('sort headers', () => {
        const renderSorted = (sort: SortState) =>
            render(
                <MemoryRouter>
                    <CandidatesTable id="t" caption="Market candidates" rows={[makeCandidate()]} sort={sort} onSort={jest.fn()} />
                </MemoryRouter>
            );
        const indicator = (label: RegExp) =>
            within(screen.getByRole('columnheader', { name: label })).getByTestId('sort-indicator');

        it.each<[SortState['direction'], 'ascending' | 'descending']>([
            ['asc', 'ascending'],
            ['desc', 'descending'],
        ])('marks the %s column with aria-sort and a single decorative arrow', (direction, aria) => {
            renderSorted({ key: 'market_cap', direction });

            const header = screen.getByRole('columnheader', { name: /^Market cap/ });
            expect(header).toHaveAttribute('aria-sort', aria);
            expect(within(header).getByRole('button')).toHaveClass('is-active');
            expect(indicator(/^Market cap/)).toHaveAttribute('data-sort-state', direction);
            expect(indicator(/^Market cap/)).toHaveAttribute('aria-hidden', 'true');
            expect(indicator(/^Market cap/)).toHaveClass('is-active');
            expect(indicator(/^Market cap/).querySelectorAll('path')).toHaveLength(1);
        });

        it('shows the two-way arrow, inactive, on every unsorted header and keeps the label as the accessible name', () => {
            renderSorted({ key: 'market_cap', direction: 'desc' });

            const unsorted = screen.getAllByRole('columnheader').filter(th => th.getAttribute('aria-sort') === 'none');
            expect(unsorted).toHaveLength(10);
            unsorted.forEach(th => {
                const mark = within(th).getByTestId('sort-indicator');
                expect(mark).toHaveAttribute('data-sort-state', 'none');
                expect(mark).toHaveAttribute('aria-hidden', 'true');
                expect(mark).not.toHaveClass('is-active');
                expect(mark.querySelectorAll('path')).toHaveLength(2);
            });
            expect(within(screen.getByRole('columnheader', { name: /^Close/ })).getByRole('button')).toHaveAccessibleName('Close');
        });
    });
});
