import { render, screen, within } from '@testing-library/react';
import { makeAnalysis, makeEmptyAnalysis } from '@/test-utils/analysisFixtures';
import FundamentalsSection from './FundamentalsSection';

beforeEach(() => window.sessionStorage.clear());

const value = (key: string) => screen.getByTestId(`fundamentals-${key}-value`);

describe('FundamentalsSection', () => {
    it('shows the composite line, then the tiers and readings', () => {
        const { fundamentals, fundamentals_computed_at } = makeAnalysis();
        render(<FundamentalsSection fundamentals={fundamentals} computedAt={fundamentals_computed_at} />);

        expect(screen.getByTestId('fundamentals-composite')).toHaveTextContent('Composite0.70 · strong');
        expect(value('eps_strength')).toHaveTextContent('Strong');
        expect(value('revenue')).toHaveTextContent('Strong');
        expect(value('pe_vs_5y')).toHaveTextContent('+12.4% (expensive)');
        expect(value('fcf_yield')).toHaveTextContent('3.25% (fair)');
        expect(value('gross_margin')).toHaveTextContent('59.89% (excellent)');
        expect(screen.getByTestId('fundamentals-gross_margin')).toHaveTextContent('Trend improving');
        expect(value('net_margin')).toHaveTextContent('45.10% (excellent)');
        expect(value('ttm_pe')).toHaveTextContent('27.3');
        expect(value('market_cap')).toHaveTextContent('$1.2T');
    });

    it('labels the P/E cell as a trailing-P/E band with no 5-year claim', () => {
        render(<FundamentalsSection fundamentals={makeAnalysis().fundamentals} computedAt={null} />);

        const cell = screen.getByTestId('fundamentals-pe_vs_5y');
        expect(within(cell).getByRole('term')).toHaveTextContent(/^P\/E band \(trailing\)$/);
        expect(cell).toHaveTextContent('Band on trailing P/E — no 5-year comparison is available');
        expect(cell).not.toHaveTextContent('vs own 5-year mean P/E');
        expect(screen.queryByText('P/E vs 5Y')).not.toBeInTheDocument();
    });

    it('shows the served P/E band when the numeric value is null', () => {
        const { fundamentals } = makeAnalysis();
        render(<FundamentalsSection fundamentals={{ ...fundamentals, pe_vs_5y: { value: null, band: 'growth_fair' } }} computedAt={null} />);

        expect(value('pe_vs_5y')).toHaveTextContent(/^Growth fair$/);
    });

    it('displays the gross-margin tier strong_moat as "high"; other tiers are humanized as before', () => {
        const { fundamentals } = makeAnalysis();
        const withTier = (tier: string) => ({ ...fundamentals, gross_margin: { ...fundamentals.gross_margin, tier } });

        const { unmount } = render(<FundamentalsSection fundamentals={withTier('strong_moat')} computedAt={null} />);
        expect(value('gross_margin')).toHaveTextContent(/^59\.89% \(high\)$/);
        unmount();

        render(<FundamentalsSection fundamentals={withTier('margin_pressure')} computedAt={null} />);
        expect(value('gross_margin')).toHaveTextContent(/^59\.89% \(margin pressure\)$/);
    });

    it('does not remap strong_moat on net margin', () => {
        const { fundamentals } = makeAnalysis();
        render(<FundamentalsSection fundamentals={{ ...fundamentals, net_margin: { ...fundamentals.net_margin, tier: 'strong_moat' } }} computedAt={null} />);

        expect(value('net_margin')).toHaveTextContent('45.10% (strong moat)');
    });

    it('lists PEG and earnings surprise as "—" (the API serves no value for them)', () => {
        render(<FundamentalsSection fundamentals={makeAnalysis().fundamentals} computedAt={null} />);

        expect(value('peg')).toHaveTextContent(/^—$/);
        expect(value('earnings_surprise')).toHaveTextContent(/^—$/);
    });

    it('renders null fields as "—" with no badges', () => {
        render(<FundamentalsSection fundamentals={makeEmptyAnalysis().fundamentals} computedAt={null} state="no_data" />);

        screen.getAllByTestId(/^fundamentals-[a-z_0-9]+-value$/).forEach(cell => expect(cell).toHaveTextContent(/^—$/));
        expect(screen.getByTestId('fundamentals-composite')).toHaveTextContent('Composite—');
        expect(screen.getByTestId('fundamentals-no-data')).toBeInTheDocument();
        expect(within(screen.getByTestId('analysis-fundamentals')).queryByTestId('severity-badge')).not.toBeInTheDocument();
    });

    it('is collapsible', () => {
        render(<FundamentalsSection fundamentals={makeAnalysis().fundamentals} computedAt={null} />);
        expect(screen.getByRole('button', { name: 'Fundamentals' })).toHaveAttribute('aria-controls', 'scanner-analysis-fundamentals-content');
    });
});
