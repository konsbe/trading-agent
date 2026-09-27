import { render, screen, within } from '@testing-library/react';
import { CorrelationsAnalysis } from '@/api';
import { makeAnalysis, makeEmptyAnalysis } from '@/test-utils/analysisFixtures';
import CorrelationsSection from './CorrelationsSection';

beforeEach(() => window.sessionStorage.clear());

const withCorrelations = (patch: Partial<CorrelationsAnalysis>): CorrelationsAnalysis => ({ ...makeAnalysis().correlations, ...patch });

const withMaster = (net_label: string | null, fired_labels: string[]) => {
    const { correlations } = makeAnalysis();
    return {
        ...correlations,
        master_signals: { ...correlations.master_signals, net_label, fired: fired_labels.map((_, i) => `code_${i}`), fired_labels },
    };
};

describe('CorrelationsSection', () => {
    it('shows the served composite, cluster, pattern and heading labels and the signal sentences', () => {
        render(<CorrelationsSection correlations={makeAnalysis().correlations} />);

        expect(screen.getByTestId('correlations-composite').textContent).toBe('Composite0.25 · mixed, leaning agree');
        const clusters = within(screen.getByTestId('correlation-clusters')).getAllByRole('listitem');
        expect(clusters.map(li => li.textContent)).toEqual(['Earnings Quality — mostly agree', 'Leverage & Liquidity — mixed, leaning conflict']);
        expect(screen.getByRole('heading', { name: 'Combined patterns' })).toBeInTheDocument();
        expect(screen.getByTestId('master-signal').textContent).toBe('Net count: −1 · met: strong EPS with weak cash signs');
        expect(screen.getByTestId('aligned-signals')).toHaveTextContent('Revenue and EPS growing together — genuine organic quality growth');
        expect(screen.getByTestId('divergent-signals')).toHaveTextContent('Debt rising faster than cash flow');
        expect(within(screen.getByTestId('analysis-correlations')).queryByTestId('severity-badge')).not.toBeInTheDocument();
    });

    it('renders served labels verbatim, never re-deriving them from the codes', () => {
        const { correlations } = makeAnalysis();
        render(
            <CorrelationsSection
                correlations={withCorrelations({
                    composite: { score: 0.15, tier: 'healthy' },
                    composite_label: 'Served composite',
                    clusters: [{ ...correlations.clusters[0], name: 'earnings_quality', tier: 'healthy', name_label: 'Served name', tier_label: 'served tier' }],
                    master_signals: { net_signal: 'bullish', net_label: 'served net', fired: ['value_trap'], fired_labels: ['served pattern'] },
                    labels: { patterns_heading: 'Served heading', net_count: 'Served count', met: 'served met' },
                })}
            />,
        );

        expect(screen.getByTestId('correlations-composite').textContent).toBe('Composite0.15 · Served composite');
        expect(within(screen.getByTestId('correlation-clusters')).getByRole('listitem').textContent).toBe('Served name — served tier');
        expect(screen.getByRole('heading', { name: 'Served heading' })).toBeInTheDocument();
        expect(screen.queryByRole('heading', { name: 'Combined patterns' })).not.toBeInTheDocument();
        expect(screen.getByTestId('master-signal').textContent).toBe('Served count: served net · served met: served pattern');
    });

    it.each([
        ['+2 or more'],
        ['+1'],
        ['0'],
        ['−1'],
        ['−2 or less'],
    ])('shows the served net label "%s" with no patterns met', label => {
        render(<CorrelationsSection correlations={withMaster(label, [])} />);
        expect(screen.getByTestId('master-signal').textContent).toBe(`Net count: ${label}`);
    });

    it('lists every served pattern label met, in order', () => {
        render(
            <CorrelationsSection
                correlations={withMaster('−2 or less', ['low P/E with quality conditions', 'cash strength with flat EPS', 'debt and liquidity strain'])}
            />,
        );

        expect(screen.getByTestId('master-signal').textContent).toBe(
            'Net count: −2 or less · met: low P/E with quality conditions, cash strength with flat EPS, debt and liquidity strain',
        );
    });

    it('shows a cluster served as "not evaluated" (checks_run 0) verbatim, and a null tier label as "—"', () => {
        const { correlations } = makeAnalysis();
        render(
            <CorrelationsSection
                correlations={withCorrelations({
                    clusters: [
                        { ...correlations.clusters[0], score: null, tier: null, tier_label: 'not evaluated', checks_run: 0 },
                        { ...correlations.clusters[1], score: null, tier: null, tier_label: null, checks_run: 2 },
                    ],
                })}
            />,
        );

        const clusters = within(screen.getByTestId('correlation-clusters')).getAllByRole('listitem');
        expect(clusters.map(li => li.textContent)).toEqual(['Earnings Quality — not evaluated', 'Leverage & Liquidity — —']);
    });

    it('shows the composite served as "not evaluated" alone when no cluster ran a check', () => {
        const { correlations } = makeAnalysis();
        render(
            <CorrelationsSection
                correlations={withCorrelations({
                    composite: { score: null, tier: null },
                    composite_label: 'not evaluated',
                    clusters: correlations.clusters.map(c => ({ ...c, score: null, tier: null, tier_label: 'not evaluated', checks_run: 0 })),
                })}
            />,
        );

        expect(screen.getByTestId('correlations-composite').textContent).toBe('Compositenot evaluated');
        const clusters = within(screen.getByTestId('correlation-clusters')).getAllByRole('listitem');
        expect(clusters.map(li => li.textContent)).toEqual(['Earnings Quality — not evaluated', 'Leverage & Liquidity — not evaluated']);
    });

    it('says so when nothing is stored', () => {
        render(<CorrelationsSection correlations={makeEmptyAnalysis().correlations} />);

        expect(screen.getByTestId('correlations-composite').textContent).toBe('Composite—');
        expect(screen.getByTestId('correlation-clusters-empty')).toBeInTheDocument();
        expect(screen.getByRole('heading', { name: 'Combined patterns' })).toBeInTheDocument();
        expect(screen.getByTestId('master-signal').textContent).toBe('Net count: —');
        expect(screen.getByTestId('aligned-signals-empty')).toBeInTheDocument();
        expect(screen.getByTestId('divergent-signals-empty')).toBeInTheDocument();
    });

    it('still renders when an older API serves no label text', () => {
        const { correlations } = makeAnalysis();
        render(
            <CorrelationsSection
                correlations={withCorrelations({
                    composite_label: null,
                    clusters: [{ ...correlations.clusters[0], name_label: 'earnings_quality', tier_label: null }],
                    master_signals: { net_signal: 'bearish', net_label: null, fired: ['value_trap'], fired_labels: [] },
                    labels: { patterns_heading: null, net_count: null, met: null },
                })}
            />,
        );

        expect(screen.getByTestId('correlations-composite').textContent).toBe('Composite0.25');
        expect(within(screen.getByTestId('correlation-clusters')).getByRole('listitem').textContent).toBe('earnings_quality — —');
        expect(screen.queryByRole('heading', { name: 'Combined patterns' })).not.toBeInTheDocument();
        expect(screen.getByTestId('master-signal').textContent).toBe('—');
    });
});
