import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { BucketResult, Candidate } from '@/api';
import { makeCandidate, makeCandidates } from '@/test-utils/fixtures';
import { COLUMNS } from '../CandidatesTable';
import BucketSection from './BucketSection';

const result = (candidates: Candidate[]): BucketResult => ({ total_candidates: candidates.length, candidates });

const renderBucket = (candidates: Candidate[]) =>
    render(
        <MemoryRouter>
            <BucketSection bucket="market" result={result(candidates)} />
        </MemoryRouter>
    );

const table = () => screen.getByTestId('scanner-bucket-market-table');
const bodyRows = () => within(table()).getAllByRole('row').slice(1);
const order = () => bodyRows().map(row => row.getAttribute('data-testid')!.replace('candidate-row-', ''));
const header = (label: string) => within(table()).getByRole('columnheader', { name: new RegExp(`^${label.replace(/[$%]/g, '\\$&')}`) });
const sortButton = (label: string) => within(header(label)).getByRole('button');

/** Same rows as utils/sortCandidates.test — one null-everything row (NUL). */
const rows = [
    makeCandidate({ symbol: 'BBB', close: 5, change_pct: -3, rvol_20: 2, dollar_volume: 2e6, rsi_14: 40, breakout_state: 'breakout', pct_of_52w_high: 0.5, catalyst_tier: 'B', momentum_score_100: 40, score_attainable: 90 }),
    makeCandidate({ symbol: 'AAA', close: 10, change_pct: 12, rvol_20: 8, dollar_volume: 9e6, rsi_14: 70, breakout_state: 'none', pct_of_52w_high: 0.9, catalyst_tier: 'none', momentum_score_100: 70, score_attainable: 75 }),
    makeCandidate({ symbol: 'NUL', exchange: null, company_name: null, close: null, change_pct: null, rvol_20: null, dollar_volume: null, rsi_14: null, breakout_state: null, pct_of_52w_high: null, catalyst_tier: null, momentum_score_100: null, score_attainable: null }),
    makeCandidate({ symbol: 'CCC', close: 1, change_pct: 4, rvol_20: 5, dollar_volume: 5e5, rsi_14: 55, breakout_state: 'breakout_from_consolidation', pct_of_52w_high: 0.1, catalyst_tier: 'A', momentum_score_100: 0, score_attainable: 60 }),
];

const ASCENDING: Record<string, string[]> = {
    Symbol: ['AAA', 'BBB', 'CCC', 'NUL'],
    Close: ['CCC', 'BBB', 'AAA', 'NUL'],
    'Change %': ['BBB', 'CCC', 'AAA', 'NUL'],
    RVOL: ['BBB', 'CCC', 'AAA', 'NUL'],
    '$ Volume': ['CCC', 'BBB', 'AAA', 'NUL'],
    RSI: ['BBB', 'CCC', 'AAA', 'NUL'],
    Breakout: ['AAA', 'BBB', 'CCC', 'NUL'],
    '% of 52w high': ['CCC', 'BBB', 'AAA', 'NUL'],
    Catalyst: ['AAA', 'BBB', 'CCC', 'NUL'],
    Score: ['CCC', 'BBB', 'AAA', 'NUL'],
};

/** Each row's visible score text and spoken label, keyed by symbol (own ceiling per row, none for a null score). */
const EXPECTED_SCORE: Record<string, { visible: string; label: string }> = {
    BBB: { visible: '40/90unvalidated', label: 'Score 40 of 90 attainable, unvalidated' },
    AAA: { visible: '70/75unvalidated', label: 'Score 70 of 75 attainable, unvalidated' },
    CCC: { visible: '0/60unvalidated', label: 'Score 0 of 60 attainable, unvalidated' },
    NUL: { visible: '—unvalidated', label: 'No score, unvalidated' },
};

const expectScoreMarkerOnEveryRow = () => {
    bodyRows().forEach(row => {
        const scoreCell = within(row).getByTestId('score-value').closest('td')!;
        expect(within(scoreCell).getByTestId('score-status')).toHaveTextContent('unvalidated');
        const expected = EXPECTED_SCORE[row.getAttribute('data-testid')!.replace('candidate-row-', '')];
        if (!expected) return;
        expect(scoreCell.querySelector('[aria-hidden="true"]')).toHaveTextContent(expected.visible);
        expect(within(scoreCell).getByTestId('score-label')).toHaveTextContent(expected.label);
    });
};

describe('BucketSection', () => {
    it('defaults to RVOL descending with nulls last and a descriptive sort label', () => {
        renderBucket(rows);

        expect(order()).toEqual(['AAA', 'CCC', 'BBB', 'NUL']);
        expect(header('RVOL')).toHaveAttribute('aria-sort', 'descending');
        expect(screen.getByTestId('bucket-market-sort-label')).toHaveTextContent('Sorted by RVOL — descriptive, not predictive');
        COLUMNS.filter(c => c.key !== 'rvol_20').forEach(c => expect(header(c.label)).toHaveAttribute('aria-sort', 'none'));
    });

    it('has one sortable header per column, all ten of them', () => {
        renderBucket(rows);
        expect(Object.keys(ASCENDING)).toEqual(COLUMNS.map(c => c.label));
        expect(within(table()).getAllByRole('columnheader')).toHaveLength(10);
    });

    it.each(Object.entries(ASCENDING))('sorts by %s both ways, nulls last, score marker on every row', async (label, ascending) => {
        renderBucket(rows);

        await userEvent.click(sortButton(label));
        if (header(label).getAttribute('aria-sort') !== 'ascending') await userEvent.click(sortButton(label));

        expect(header(label)).toHaveAttribute('aria-sort', 'ascending');
        expect(order()).toEqual(ascending);
        expect(screen.getByTestId('bucket-market-sort-label')).toHaveTextContent(`Sorted by ${label} — descriptive, not predictive`);
        expectScoreMarkerOnEveryRow();

        await userEvent.click(sortButton(label));

        expect(header(label)).toHaveAttribute('aria-sort', 'descending');
        const descending = label === 'Symbol' ? [...ascending].reverse() : [...ascending.slice(0, 3)].reverse().concat('NUL');
        expect(order()).toEqual(descending);
        expectScoreMarkerOnEveryRow();
    });

    it('starts Symbol ascending and numeric columns descending on first click', async () => {
        renderBucket(rows);

        await userEvent.click(sortButton('Symbol'));
        expect(header('Symbol')).toHaveAttribute('aria-sort', 'ascending');

        await userEvent.click(sortButton('Score'));
        expect(header('Score')).toHaveAttribute('aria-sort', 'descending');
        expect(order()).toEqual(['AAA', 'BBB', 'CCC', 'NUL']);
        expect(header('Symbol')).toHaveAttribute('aria-sort', 'none');
    });

    it('sorts from the keyboard', async () => {
        renderBucket(rows);

        sortButton('Close').focus();
        await userEvent.keyboard('{Enter}');
        expect(header('Close')).toHaveAttribute('aria-sort', 'descending');
        expect(order()).toEqual(['AAA', 'BBB', 'CCC', 'NUL']);

        await userEvent.keyboard(' ');
        expect(header('Close')).toHaveAttribute('aria-sort', 'ascending');
    });

    it('renders a null score as "—" (never 0) with the marker, and a real 0 as 0', () => {
        renderBucket(rows);

        const nul = screen.getByTestId('candidate-row-NUL');
        expect(within(nul).getByTestId('score-value')).toHaveTextContent(/^—$/);
        expect(within(nul).getByTestId('score-status')).toHaveTextContent('unvalidated');
        expect(within(screen.getByTestId('candidate-row-CCC')).getByTestId('score-value')).toHaveTextContent(/^0$/);
    });

    describe('score ceiling', () => {
        const scoreCell = (symbol: string) => within(screen.getByTestId(`candidate-row-${symbol}`)).getByTestId('score-value').closest('td')!;

        it("shows each row's own score_attainable, not a fixed 75", () => {
            renderBucket([
                makeCandidate({ symbol: 'C75', momentum_score_100: 53, score_attainable: 75 }),
                makeCandidate({ symbol: 'C90', momentum_score_100: 53, score_attainable: 90 }),
            ]);

            expect(within(scoreCell('C75')).getByTestId('score-ceiling')).toHaveTextContent(/^\/75$/);
            expect(within(scoreCell('C90')).getByTestId('score-ceiling')).toHaveTextContent(/^\/90$/);
            expect(scoreCell('C90').querySelector('[aria-hidden="true"]')).toHaveTextContent(/^53\/90unvalidated$/);
            expect(within(scoreCell('C90')).getByTestId('score-label')).toHaveTextContent(/^Score 53 of 90 attainable, unvalidated$/);
        });

        it('shows no ceiling for a null score', () => {
            renderBucket(rows);

            expect(within(scoreCell('NUL')).queryByTestId('score-ceiling')).not.toBeInTheDocument();
            expect(within(scoreCell('NUL')).getByTestId('score-label')).toHaveTextContent(/^No score, unvalidated$/);
        });

        it('shows the bare score when score_attainable is missing, without inventing a denominator', () => {
            renderBucket([makeCandidate({ symbol: 'BARE', momentum_score_100: 53, score_attainable: null })]);

            expect(within(scoreCell('BARE')).queryByTestId('score-ceiling')).not.toBeInTheDocument();
            expect(scoreCell('BARE').querySelector('[aria-hidden="true"]')).toHaveTextContent(/^53unvalidated$/);
            expect(within(scoreCell('BARE')).getByTestId('score-label')).toHaveTextContent(/^Score 53, unvalidated$/);
        });

        it('sorts Score by momentum_score_100 only, not by score/ceiling, nulls last', async () => {
            renderBucket([
                makeCandidate({ symbol: 'LOWRATIO', momentum_score_100: 60, score_attainable: 90 }),
                makeCandidate({ symbol: 'NOSCORE', momentum_score_100: null, score_attainable: null }),
                makeCandidate({ symbol: 'HIRATIO', momentum_score_100: 55, score_attainable: 60 }),
            ]);

            await userEvent.click(sortButton('Score'));
            expect(header('Score')).toHaveAttribute('aria-sort', 'descending');
            expect(order()).toEqual(['LOWRATIO', 'HIRATIO', 'NOSCORE']);

            await userEvent.click(sortButton('Score'));
            expect(order()).toEqual(['HIRATIO', 'LOWRATIO', 'NOSCORE']);
        });

        it('mentions the ceiling in the Score header tooltip', () => {
            renderBucket(rows);
            expect(header('Score')).toHaveAttribute('title', expect.stringMatching(/attainable ceiling/));
        });
    });

    it('takes the score marker from score_status, not a hard-coded string', () => {
        renderBucket([makeCandidate({ symbol: 'NEW', score_status: 'validated-v3' })]);
        expect(within(screen.getByTestId('candidate-row-NEW')).getByTestId('score-status')).toHaveTextContent('validated-v3');
    });

    it('explains the score in the Score header tooltip', () => {
        renderBucket(rows);
        expect(header('Score')).toHaveAttribute('title', expect.stringMatching(/unvalidated/i));
    });

    describe('windowing', () => {
        it('shows the top 10 of the current sort, then "and N more" reveals and collapses the rest', async () => {
            renderBucket(makeCandidates(25));

            expect(bodyRows()).toHaveLength(10);
            expect(order()[0]).toBe('S01');
            const toggle = screen.getByRole('button', { name: 'and 15 more' });
            expect(toggle).toHaveAttribute('aria-expanded', 'false');
            expect(toggle).toHaveAttribute('aria-controls', 'scanner-bucket-market-table');

            await userEvent.click(toggle);
            expect(bodyRows()).toHaveLength(25);
            expect(toggle).toHaveAttribute('aria-expanded', 'true');
            expect(toggle).toHaveTextContent('Show top 10 only');

            await userEvent.click(toggle);
            expect(bodyRows()).toHaveLength(10);
        });

        it('windows whichever sort is active', async () => {
            renderBucket(makeCandidates(25));

            await userEvent.click(sortButton('RVOL'));
            expect(header('RVOL')).toHaveAttribute('aria-sort', 'ascending');
            expect(order()).toEqual(['S25', 'S24', 'S23', 'S22', 'S21', 'S20', 'S19', 'S18', 'S17', 'S16']);
            expect(screen.getByRole('button', { name: 'and 15 more' })).toBeInTheDocument();
            expectScoreMarkerOnEveryRow();
        });

        it('shows no toggle for 10 or fewer candidates', () => {
            renderBucket(makeCandidates(10));
            expect(bodyRows()).toHaveLength(10);
            expect(screen.queryByTestId('bucket-market-toggle')).not.toBeInTheDocument();
        });
    });

    describe('collapsible card', () => {
        const toggle = () => screen.getByRole('button', { name: /^Market/ });
        const renderPenny = (candidates: Candidate[]) =>
            render(
                <MemoryRouter>
                    <BucketSection bucket="penny" result={result(candidates)} />
                </MemoryRouter>
            );

        it('has a header toggle with the title and count, controlling the table region', () => {
            renderBucket(rows);

            expect(toggle()).toHaveAttribute('aria-expanded', 'true');
            expect(toggle()).toHaveAccessibleName('Market 4 candidates');
            expect(toggle().closest('h2')).not.toBeNull();
            expect(document.getElementById(toggle().getAttribute('aria-controls')!)).toContainElement(table());
        });

        it('shows the sort label in the header meta, outside the toggle, and keeps it while collapsed', async () => {
            renderBucket(rows);

            const label = screen.getByTestId('bucket-market-sort-label');
            expect(label.closest('.ta-collapsible-card__meta')).not.toBeNull();
            expect(toggle()).not.toContainElement(label);

            await userEvent.click(toggle());
            expect(toggle()).toHaveAttribute('aria-expanded', 'false');
            expect(screen.queryByRole('table')).not.toBeInTheDocument();
            expect(screen.getByTestId('bucket-market-sort-label')).toHaveTextContent('Sorted by RVOL — descriptive, not predictive');
            expect(screen.getByTestId('bucket-market-count')).toHaveTextContent('4');
        });

        it('keeps the sort order across collapse and expand (Enter/Space on the header)', async () => {
            renderBucket(rows);
            await userEvent.click(sortButton('Symbol'));
            expect(order()).toEqual(['AAA', 'BBB', 'CCC', 'NUL']);

            toggle().focus();
            await userEvent.keyboard('{Enter}');
            expect(screen.getByTestId('bucket-market-sort-label')).toHaveTextContent('Sorted by Symbol');
            await userEvent.keyboard(' ');

            expect(toggle()).toHaveAttribute('aria-expanded', 'true');
            expect(header('Symbol')).toHaveAttribute('aria-sort', 'ascending');
            expect(order()).toEqual(['AAA', 'BBB', 'CCC', 'NUL']);
        });

        it('keeps "and N more" expansion across collapse and expand', async () => {
            renderBucket(makeCandidates(25));
            await userEvent.click(screen.getByRole('button', { name: 'and 15 more' }));
            expect(bodyRows()).toHaveLength(25);

            await userEvent.click(toggle());
            await userEvent.click(toggle());

            expect(bodyRows()).toHaveLength(25);
            expect(screen.getByTestId('bucket-market-toggle')).toHaveTextContent('Show top 10 only');
        });

        it('remembers each bucket separately in the session', async () => {
            const { unmount } = renderBucket(rows);
            await userEvent.click(toggle());
            expect(sessionStorage.getItem('ta-collapsible:scanner.list.market')).toBe('false');
            unmount();

            renderBucket(rows);
            expect(toggle()).toHaveAttribute('aria-expanded', 'false');

            renderPenny(rows.map(c => ({ ...c, bucket: 'penny' as const })));
            expect(screen.getByRole('button', { name: /^Penny/ })).toHaveAttribute('aria-expanded', 'true');
        });

        it('collapses the empty state too, with no sort label', async () => {
            renderPenny([]);
            const pennyToggle = screen.getByRole('button', { name: /^Penny/ });
            expect(screen.queryByTestId('bucket-penny-sort-label')).not.toBeInTheDocument();

            await userEvent.click(pennyToggle);
            expect(screen.queryByTestId('bucket-penny-empty')).not.toBeInTheDocument();
            await userEvent.click(pennyToggle);
            expect(screen.getByTestId('bucket-penny-empty')).toBeInTheDocument();
        });
    });

    it('renders a calm empty state for a bucket with zero candidates', () => {
        render(
            <MemoryRouter>
                <BucketSection bucket="penny" result={result([])} />
            </MemoryRouter>
        );

        expect(screen.getByTestId('bucket-penny-empty')).toHaveTextContent('No Penny candidates in this scan');
        expect(screen.queryByRole('table')).not.toBeInTheDocument();
        expect(screen.queryByRole('alert')).not.toBeInTheDocument();
        expect(screen.getByTestId('bucket-penny-count')).toHaveTextContent('0');
    });
});
