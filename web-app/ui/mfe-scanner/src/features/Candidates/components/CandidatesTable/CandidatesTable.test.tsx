import { readFileSync } from 'fs';
import { join } from 'path';
import { render, screen, within } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { makeCandidate } from '@/test-utils/fixtures';
import { DEFAULT_SORT } from '../../utils/sortCandidates';
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
            expect(css).toMatch(/@container \(min-width: 1180px\)\s*\{\s*\.scanner-table td\[data-column='breakout_state'\]\s*\{\s*max-width: none;\s*white-space: nowrap;/);
            expect(rule('.scanner-table__wrap')).toMatch(/container-type:\s*inline-size/);
            expect(rule('.scanner-table__symbol')).toMatch(/line-height:\s*1\.3/);
        });

        it('lets Breakout wrap and keeps each score on one line with its marker', () => {
            expect(rule(".scanner-table td[data-column='breakout_state']")).toMatch(/white-space:\s*normal/);
            expect(rule(".scanner-table td[data-column='momentum_score_100']")).toMatch(/white-space:\s*nowrap/);
        });
    });
});
