import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { makeAnalysis, makeEmptyAnalysis } from '@/test-utils/analysisFixtures';
import ContextSection from './ContextSection';

beforeEach(() => window.sessionStorage.clear());

const value = (key: string) => screen.getByTestId(`context-${key}-value`);

describe('ContextSection', () => {
    it('shows the stored labels verbatim, with the Market Report tone indicators', () => {
        render(<ContextSection context={makeAnalysis().context_vs_benchmark} />);

        expect(value('benchmark')).toHaveTextContent('SPY');
        expect(value('market_cycle')).toHaveTextContent(/^pullback_healthy$/);
        expect(screen.getByTestId('context-market_cycle-tone')).toHaveAccessibleName('Status: neutral');
        expect(screen.getByTestId('context-market_cycle-tone')).toHaveClass('scanner-tone--neutral');
        expect(value('price_phase')).toHaveTextContent(/^pullback$/);
        expect(value('drawdown')).toHaveTextContent('−5.58%');
        expect(value('correlation_regime')).toHaveTextContent(/^stagflation_risk$/);
        expect(screen.getByTestId('context-correlation_regime-tone')).toHaveAccessibleName('Status: stressed');
        expect(value('relative_strength')).toHaveTextContent(/^—$/);
    });

    it('shows relative strength in percentage points when served', () => {
        const { context_vs_benchmark: context } = makeAnalysis();
        render(<ContextSection context={{ ...context, relative_strength_20d_pp: 2.85 }} />);

        expect(value('relative_strength')).toHaveTextContent('2.85 pp');
    });

    it('renders nulls as "—" with no tone indicator for a null tone', () => {
        render(<ContextSection context={makeEmptyAnalysis().context_vs_benchmark} />);

        const cells = screen.getAllByTestId(/^context-[a-z_]+-value$/);
        expect(cells).toHaveLength(6);
        cells.forEach(cell => expect(cell).toHaveTextContent(/^—$/));
        expect(within(screen.getByTestId('analysis-context')).queryByRole('img')).not.toBeInTheDocument();
    });

    it('ignores a tone word the Market Report has no indicator for', () => {
        const { context_vs_benchmark: context } = makeAnalysis();
        render(<ContextSection context={{ ...context, market_cycle_tone: 'yellow' }} />);

        expect(screen.queryByTestId('context-market_cycle-tone')).not.toBeInTheDocument();
        expect(value('market_cycle')).toHaveTextContent('pullback_healthy');
    });

    it('has no severity badges', () => {
        render(<ContextSection context={makeAnalysis().context_vs_benchmark} />);
        expect(within(screen.getByTestId('analysis-context')).queryAllByTestId(/severity/)).toHaveLength(0);
    });

    it('persists its collapsed state under mfe-scanner.detail.context', async () => {
        render(<ContextSection context={makeAnalysis().context_vs_benchmark} />);

        await userEvent.click(screen.getByRole('button', { name: 'Context vs benchmark' }));
        expect(window.sessionStorage.getItem('ta-collapsible:mfe-scanner.detail.context')).toBe('false');
    });
});
