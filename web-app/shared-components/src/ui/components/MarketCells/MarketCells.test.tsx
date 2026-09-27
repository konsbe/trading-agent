import React from 'react';
import { readFileSync } from 'fs';
import { join } from 'path';
import { render, screen } from '@testing-library/react';
import ChangeCell from './ChangeCell';
import MarketCapCell from './MarketCapCell';
import ScoreCell, { scoreLabel } from './ScoreCell';
import { MARKET_COLUMNS } from './marketColumns';
import { MarketRow } from './types';

const makeRow = (overrides: Partial<MarketRow> = {}): MarketRow => ({
    close: 3.42,
    change_pct: 15.5,
    rvol_20: 6.74,
    dollar_volume: 4553305.84,
    rsi_14: 71.6,
    breakout_state: 'breakout_from_consolidation',
    pct_of_52w_high: 0.79,
    catalyst_tier: null,
    market_cap: 4328181000,
    market_cap_est: null,
    market_cap_is_proxy: false,
    momentum_score_100: 53,
    score_attainable: 75,
    score_status: 'unvalidated',
    ...overrides,
});

describe('ChangeCell', () => {
    it.each<[number | null, string, string | null]>([
        [15.5, '+15.5%', 'is-price-up'],
        [-3.4, '−3.4%', 'is-price-down'],
        [0, '0.0%', null],
        [null, '—', null],
    ])('renders %p as %s, toned %p', (value, text, tone) => {
        render(<ChangeCell value={value} />);
        const cell = screen.getByTestId('change-value');
        expect(cell).toHaveTextContent(text);
        expect(cell).toHaveClass('market-cell__change');
        ['is-price-up', 'is-price-down'].forEach(cls =>
            cls === tone ? expect(cell).toHaveClass(cls) : expect(cell).not.toHaveClass(cls)
        );
    });
});

describe('MarketCapCell', () => {
    it('renders a reported cap unmarked, without a tooltip', () => {
        render(<MarketCapCell row={{ market_cap: 4328181000, market_cap_est: null, market_cap_is_proxy: false }} />);
        const cell = screen.getByTestId('market-cap-value');
        expect(cell).toHaveTextContent(/^\$4\.3B$/);
        expect(cell).not.toHaveClass('is-estimate');
        expect(cell).not.toHaveAttribute('title');
    });

    it('marks an estimate with "(est.)" and explains it', () => {
        render(<MarketCapCell row={{ market_cap: null, market_cap_est: 120e6, market_cap_is_proxy: true }} />);
        const cell = screen.getByTestId('market-cap-value');
        expect(cell).toHaveTextContent(/^\$120M \(est\.\)$/);
        expect(cell).toHaveClass('is-estimate');
        expect(cell).toHaveAttribute('title', 'Estimated: shares outstanding × close');
    });

    it('renders "—" when neither value exists', () => {
        render(<MarketCapCell row={{ market_cap: null, market_cap_est: null, market_cap_is_proxy: null }} />);
        expect(screen.getByTestId('market-cap-value')).toHaveTextContent(/^—$/);
    });
});

describe('ScoreCell', () => {
    it('shows the score against its own ceiling with the unvalidated marker', () => {
        render(<ScoreCell row={{ momentum_score_100: 53, score_attainable: 75, score_status: 'unvalidated' }} />);
        expect(screen.getByTestId('score-value')).toHaveTextContent(/^53$/);
        expect(screen.getByTestId('score-ceiling')).toHaveTextContent(/^\/75$/);
        expect(screen.getByTestId('score-status')).toHaveTextContent(/^unvalidated$/);
        expect(screen.getByTestId('score-label')).toHaveTextContent('Score 53 of 75 attainable, unvalidated');
        expect(screen.getByTestId('score-label')).toHaveClass('market-cell__sr-only');
    });

    it('renders a null score as "—", never 0, with no ceiling', () => {
        render(<ScoreCell row={{ momentum_score_100: null, score_attainable: null, score_status: 'unvalidated' }} />);
        expect(screen.getByTestId('score-value')).toHaveTextContent(/^—$/);
        expect(screen.queryByTestId('score-ceiling')).not.toBeInTheDocument();
        expect(screen.getByTestId('score-status')).toHaveTextContent('unvalidated');
        expect(screen.getByTestId('score-label')).toHaveTextContent('No score, unvalidated');
    });

    it('keeps a real zero score and omits a missing ceiling', () => {
        render(<ScoreCell row={{ momentum_score_100: 0, score_attainable: null, score_status: 'unvalidated' }} />);
        expect(screen.getByTestId('score-value')).toHaveTextContent(/^0$/);
        expect(screen.queryByTestId('score-ceiling')).not.toBeInTheDocument();
    });

    it('labels scores for screen readers', () => {
        expect(scoreLabel({ momentum_score_100: 40, score_attainable: null, score_status: 'unvalidated' })).toBe('Score 40, unvalidated');
    });
});

describe('MARKET_COLUMNS', () => {
    const renderColumn = (key: string, row: MarketRow) => {
        const column = MARKET_COLUMNS.find(c => c.key === key)!;
        return render(<div>{column.render(row)}</div>).container;
    };

    it("uses the candidates table's order, labels and tooltips", () => {
        expect(MARKET_COLUMNS.map(c => c.label)).toEqual([
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
        ]);
        expect(MARKET_COLUMNS.filter(c => !c.numeric).map(c => c.key)).toEqual(['breakout_state', 'catalyst_tier']);
        expect(MARKET_COLUMNS.find(c => c.key === 'rvol_20')?.description).toBe('Relative volume vs. the 20-day average');
        expect(MARKET_COLUMNS.find(c => c.key === 'rsi_14')?.description).toBe('14-day RSI');
        expect(MARKET_COLUMNS.find(c => c.key === 'market_cap')?.description).toMatch(/estimate.*\(est\.\)/);
        expect(MARKET_COLUMNS.find(c => c.key === 'momentum_score_100')?.description).toMatch(/^Research score — unvalidated/);
    });

    it.each<[string, string]>([
        ['close', '$3.42'],
        ['change_pct', '+15.5%'],
        ['rvol_20', '6.74×'],
        ['dollar_volume', '$4.55M'],
        ['market_cap', '$4.3B'],
        ['rsi_14', '72'],
        ['breakout_state', 'breakout from consolidation'],
        ['pct_of_52w_high', '79.00%'],
        ['catalyst_tier', ''],
        ['momentum_score_100', 'Score 53 of 75 attainable, unvalidated53/75unvalidated'],
    ])('renders %s as %p', (key, text) => {
        expect(renderColumn(key, makeRow()).textContent).toBe(text);
    });

    it('renders every null market value as "—" and a null score in its no-score state', () => {
        const nulls = makeRow({
            close: null,
            change_pct: null,
            rvol_20: null,
            dollar_volume: null,
            rsi_14: null,
            breakout_state: null,
            pct_of_52w_high: null,
            market_cap: null,
            momentum_score_100: null,
            score_attainable: null,
        });
        ['close', 'change_pct', 'rvol_20', 'dollar_volume', 'market_cap', 'rsi_14', 'breakout_state', 'pct_of_52w_high'].forEach(key =>
            expect(renderColumn(key, nulls).textContent).toBe('—')
        );
        expect(renderColumn('momentum_score_100', nulls).textContent).toBe('No score, unvalidated—unvalidated');
    });
});

describe('MarketCells stylesheet', () => {
    const css = readFileSync(join(__dirname, 'MarketCells-styles.css'), 'utf8');

    it('uses only the price aliases for the change colours, and no hex colours', () => {
        expect(css).toMatch(/\.market-cell__change\.is-price-up\s*\{\s*color:\s*var\(--color-price-up\)/);
        expect(css).toMatch(/\.market-cell__change\.is-price-down\s*\{\s*color:\s*var\(--color-price-down\)/);
        expect(css).not.toMatch(/#[0-9a-f]{3,6}\b/i);
        expect(css).not.toMatch(/data-theme|prefers-color-scheme/);
    });
});
