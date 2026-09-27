import { render, screen, within } from '@testing-library/react';
import { makeAnalysis, makeEmptyAnalysis } from '@/test-utils/analysisFixtures';
import CorrelationsSection from './CorrelationsSection';

beforeEach(() => window.sessionStorage.clear());

const withMaster = (net_signal: string | null, fired: string[]) => {
    const { correlations } = makeAnalysis();
    return { ...correlations, master_signals: { net_signal, fired } };
};

describe('CorrelationsSection', () => {
    it('shows the composite, cluster health as plain text and the signal sentences', () => {
        render(<CorrelationsSection correlations={makeAnalysis().correlations} />);

        expect(screen.getByTestId('correlations-composite')).toHaveTextContent('Composite0.25 · neutral');
        const clusters = within(screen.getByTestId('correlation-clusters')).getAllByRole('listitem');
        expect(clusters.map(li => li.textContent)).toEqual(['Earnings Quality — mostly agree', 'Leverage Liquidity — weak']);
        expect(screen.getByRole('heading', { name: 'Combined patterns' })).toBeInTheDocument();
        expect(screen.queryByRole('heading', { name: 'Master signal' })).not.toBeInTheDocument();
        expect(screen.getByTestId('master-signal')).toHaveTextContent('Net count: +1 · met: quality growth');
        expect(screen.getByTestId('aligned-signals')).toHaveTextContent('Revenue and EPS growing together — genuine organic quality growth');
        expect(screen.getByTestId('divergent-signals')).toHaveTextContent('Debt rising faster than cash flow');
        expect(within(screen.getByTestId('analysis-correlations')).queryByTestId('severity-badge')).not.toBeInTheDocument();
    });

    it.each([
        ['healthy', 'mostly agree'],
        ['mixed_positive', 'mixed, leaning agree'],
        ['mixed_negative', 'mixed, leaning conflict'],
        ['alert', 'mostly conflict'],
        ['new_tier', 'new tier'],
    ])('shows cluster and composite tier %s as "%s"', (tier, label) => {
        const { correlations } = makeAnalysis();
        render(
            <CorrelationsSection
                correlations={{
                    ...correlations,
                    composite: { score: 0.15, tier },
                    clusters: [{ ...correlations.clusters[0], tier }],
                }}
            />,
        );

        expect(screen.getByTestId('correlations-composite')).toHaveTextContent(`Composite0.15 · ${label}`);
        expect(within(screen.getByTestId('correlation-clusters')).getByRole('listitem')).toHaveTextContent(`Earnings Quality — ${label}`);
    });

    it.each([
        ['strongly_bullish', '+2 or more'],
        ['bullish', '+1'],
        ['neutral', '0'],
        ['bearish', '−1'],
        ['strongly_bearish', '−2 or less'],
        ['unmapped_signal', 'unmapped signal'],
    ])('shows net_signal %s as net count "%s"', (code, label) => {
        render(<CorrelationsSection correlations={withMaster(code, [])} />);
        expect(screen.getByTestId('master-signal').textContent).toBe(`Net count: ${label}`);
    });

    it('names every combined pattern met and humanizes unknown ones', () => {
        render(
            <CorrelationsSection
                correlations={withMaster('strongly_bearish', [
                    'bullish_convergence',
                    'hidden_value',
                    'deterioration_warning',
                    'value_trap',
                    'leverage_cycle_warning',
                    'brand_new_pattern',
                ])}
            />,
        );

        expect(screen.getByTestId('master-signal').textContent).toBe(
            'Net count: −2 or less · met: low P/E with quality conditions, cash strength with flat EPS, strong EPS with weak cash signs, ' +
                'low P/E with weak conditions, debt and liquidity strain, brand new pattern',
        );
    });

    it('reads a cluster that ran no checks as "not evaluated", and a null tier with checks as "—"', () => {
        const { correlations } = makeAnalysis();
        render(
            <CorrelationsSection
                correlations={{
                    ...correlations,
                    clusters: [
                        { ...correlations.clusters[0], score: null, tier: null, checks_run: 0 },
                        { ...correlations.clusters[1], score: null, tier: null, checks_run: 2 },
                    ],
                }}
            />,
        );

        const clusters = within(screen.getByTestId('correlation-clusters')).getAllByRole('listitem');
        expect(clusters.map(li => li.textContent)).toEqual(['Earnings Quality — not evaluated', 'Leverage Liquidity — —']);
    });

    it('reads the composite as "not evaluated" when no cluster ran a check', () => {
        const { correlations } = makeAnalysis();
        render(
            <CorrelationsSection
                correlations={{
                    ...correlations,
                    composite: { score: null, tier: null },
                    clusters: correlations.clusters.map(c => ({ ...c, score: null, tier: null, checks_run: 0 })),
                }}
            />,
        );

        expect(screen.getByTestId('correlations-composite').textContent).toBe('Compositenot evaluated');
        const clusters = within(screen.getByTestId('correlation-clusters')).getAllByRole('listitem');
        expect(clusters.map(li => li.textContent)).toEqual(['Earnings Quality — not evaluated', 'Leverage Liquidity — not evaluated']);
    });

    it('keeps the stored composite tier when only some clusters ran checks', () => {
        const { correlations } = makeAnalysis();
        render(
            <CorrelationsSection
                correlations={{
                    ...correlations,
                    composite: { score: 0.6, tier: 'mixed_positive' },
                    clusters: [{ ...correlations.clusters[0], score: null, tier: null, checks_run: 0 }, correlations.clusters[1]],
                }}
            />,
        );

        expect(screen.getByTestId('correlations-composite').textContent).toBe('Composite0.60 · mixed, leaning agree');
        const clusters = within(screen.getByTestId('correlation-clusters')).getAllByRole('listitem');
        expect(clusters.map(li => li.textContent)).toEqual(['Earnings Quality — not evaluated', 'Leverage Liquidity — weak']);
    });

    it('says so when nothing is stored', () => {
        render(<CorrelationsSection correlations={makeEmptyAnalysis().correlations} />);

        expect(screen.getByTestId('correlations-composite')).toHaveTextContent('Composite—');
        expect(screen.getByTestId('correlation-clusters-empty')).toBeInTheDocument();
        expect(screen.getByTestId('master-signal').textContent).toBe('Net count: —');
        expect(screen.getByTestId('aligned-signals-empty')).toBeInTheDocument();
        expect(screen.getByTestId('divergent-signals-empty')).toBeInTheDocument();
    });
});
