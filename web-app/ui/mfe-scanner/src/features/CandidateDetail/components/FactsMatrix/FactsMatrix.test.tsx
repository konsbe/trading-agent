import { readFileSync } from 'fs';
import { join } from 'path';
import { render, screen } from '@testing-library/react';
import { makeFacts, makeNullFacts } from '@/test-utils/fixtures';
import FactsMatrix from './FactsMatrix';

const value = (key: string) => screen.getByTestId(`fact-${key}-value`);

describe('FactsMatrix', () => {
    it('renders every fact from the fixture', () => {
        render(<FactsMatrix facts={makeFacts()} />);

        expect(value('close')).toHaveTextContent('$2.74');
        expect(screen.getByTestId('fact-close')).toHaveTextContent('Prior close $2.26');
        expect(value('change')).toHaveTextContent('+$0.48 (+21.2%)');
        expect(value('volume')).toHaveTextContent('7.9M shares');
        expect(value('avg_volume')).toHaveTextContent('1.2M shares');
        expect(value('dollar_volume')).toHaveTextContent('$21.7M');
        expect(value('rvol')).toHaveTextContent('6.45×');
        expect(value('float')).toHaveTextContent('146.0M (est.)');
        expect(value('rsi')).toHaveTextContent('73.5');
        expect(value('high_52w')).toHaveTextContent('$3.13');
        expect(screen.getByTestId('fact-high_52w')).toHaveTextContent('−12.5% from peak');
        expect(value('breakout')).toHaveTextContent('breakout from consolidation');
        expect(value('gap')).toHaveTextContent('+16.8%');
        expect(value('vwap')).toHaveTextContent('15.9% above');
        expect(value('atr')).toHaveTextContent('5.2%');
        expect(value('market_cap')).toHaveTextContent(/^\$390M$/);
        expect(value('catalyst')).toHaveTextContent(/^—$/);
        expect(screen.getByTestId('fact-catalyst')).toHaveTextContent('Not checked — catalyst data not yet ingested');
        expect(screen.queryByText(/spread/i)).not.toBeInTheDocument();
    });

    it('colours only the day change, with the price-up/down classes, and leaves zero neutral', () => {
        const { rerender } = render(<FactsMatrix facts={makeFacts()} />);
        expect(value('change')).toHaveClass('is-price-up');
        expect(document.querySelectorAll('.is-price-up, .is-price-down')).toHaveLength(1);

        rerender(<FactsMatrix facts={makeFacts({ change_abs: -0.2, change_pct: -3 })} />);
        expect(value('change')).toHaveClass('is-price-down');

        rerender(<FactsMatrix facts={makeFacts({ change_abs: 0, change_pct: 0 })} />);
        expect(value('change')).not.toHaveClass('is-price-up');
        expect(value('change')).not.toHaveClass('is-price-down');
    });

    it('uses the price aliases in CSS (never raw colours)', () => {
        const css = readFileSync(join(__dirname, 'FactsMatrix-styles.css'), 'utf8');
        expect(css).toMatch(/\.is-price-up\s*\{\s*color:\s*var\(--color-price-up\)/);
        expect(css).toMatch(/\.is-price-down\s*\{\s*color:\s*var\(--color-price-down\)/);
        expect(css).not.toMatch(/#[0-9a-f]{3,6}\b/i);
    });

    it('renders every fact as "—" when all facts are null: no 0, NaN, "No", empty value or tone', () => {
        render(<FactsMatrix facts={makeNullFacts()} />);

        const cells = screen.getAllByTestId(/^fact-[a-z_0-9]+-value$/);
        expect(cells).toHaveLength(15);
        cells.forEach(cell => expect(cell).toHaveTextContent(/^—$/));
        expect(screen.getByTestId('fact-close')).toHaveTextContent('Prior close —');
        expect(screen.getByTestId('fact-vwap')).toHaveTextContent('20-day VWAP —');
        expect(screen.getByTestId('fact-catalyst')).toHaveTextContent('Not checked — catalyst data not yet ingested');
        expect(document.querySelectorAll('.is-price-up, .is-price-down')).toHaveLength(0);
        expect(screen.getByTestId('facts-matrix').textContent).not.toMatch(/NaN|undefined|null|Infinity|\bNo\b|\(est\.\)|0\.00×|\$0(?![.\d])|(^|\s)0%/);
        expect(screen.queryByText(/computed/)).not.toBeInTheDocument();
    });

    it('does not mark a null float or market cap as an estimate even when the proxy flag is set', () => {
        render(<FactsMatrix facts={makeNullFacts({ float_is_proxy: true, market_cap_is_proxy: true })} />);
        expect(value('float')).toHaveTextContent(/^—$/);
        expect(value('market_cap')).toHaveTextContent(/^—$/);
    });

    it('renders nulls as "—", a new high, and "None found" for a checked-but-empty catalyst', () => {
        render(
            <FactsMatrix
                facts={makeFacts({ close: null, prior_close: null, volume: null, rsi_14: null, gap_pct: null, pct_of_52w_high: 1.02, catalyst_tier: 'none', computed_at: null })}
            />
        );

        expect(value('close')).toHaveTextContent(/^—$/);
        expect(value('volume')).toHaveTextContent(/^—$/);
        expect(value('rsi')).toHaveTextContent(/^—$/);
        expect(value('gap')).toHaveTextContent(/^—$/);
        expect(screen.getByTestId('fact-high_52w')).toHaveTextContent('New 52-week high');
        expect(value('catalyst')).toHaveTextContent(/^None found$/);
    });
});
