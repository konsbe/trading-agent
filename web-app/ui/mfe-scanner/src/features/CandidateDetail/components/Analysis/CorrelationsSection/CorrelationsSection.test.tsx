import { render, screen, within } from '@testing-library/react';
import { makeAnalysis, makeEmptyAnalysis } from '@/test-utils/analysisFixtures';
import CorrelationsSection from './CorrelationsSection';

beforeEach(() => window.sessionStorage.clear());

describe('CorrelationsSection', () => {
    it('shows the composite, cluster health as plain text and the signal sentences', () => {
        render(<CorrelationsSection correlations={makeAnalysis().correlations} />);

        expect(screen.getByTestId('correlations-composite')).toHaveTextContent('Composite0.25 · neutral');
        const clusters = within(screen.getByTestId('correlation-clusters')).getAllByRole('listitem');
        expect(clusters.map(li => li.textContent)).toEqual(['Earnings Quality — healthy', 'Leverage Liquidity — weak']);
        expect(screen.getByTestId('master-signal')).toHaveTextContent('Net signal: Bullish · fired: quality growth');
        expect(screen.getByTestId('aligned-signals')).toHaveTextContent('Revenue and EPS growing together — genuine organic quality growth');
        expect(screen.getByTestId('divergent-signals')).toHaveTextContent('Debt rising faster than cash flow');
        expect(within(screen.getByTestId('analysis-correlations')).queryByTestId('severity-badge')).not.toBeInTheDocument();
    });

    it('says so when nothing is stored', () => {
        render(<CorrelationsSection correlations={makeEmptyAnalysis().correlations} />);

        expect(screen.getByTestId('correlations-composite')).toHaveTextContent('Composite—');
        expect(screen.getByTestId('correlation-clusters-empty')).toBeInTheDocument();
        expect(screen.getByTestId('master-signal')).toHaveTextContent('Net signal: —');
        expect(screen.getByTestId('aligned-signals-empty')).toBeInTheDocument();
        expect(screen.getByTestId('divergent-signals-empty')).toBeInTheDocument();
    });
});
