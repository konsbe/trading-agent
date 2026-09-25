import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { HEURISTIC_CAVEAT, makeAnalysis, makeEmptyAnalysis } from '@/test-utils/analysisFixtures';
import HeuristicSignalsSection, { HEURISTIC_PERSIST_KEY } from './HeuristicSignalsSection';

const STORAGE_KEY = `ta-collapsible:${HEURISTIC_PERSIST_KEY}`;

beforeEach(() => window.sessionStorage.clear());

const renderSection = (analysis = makeAnalysis(), focused = false) =>
    render(<HeuristicSignalsSection signals={analysis.heuristic_signals} technical={analysis.technical} focused={focused} />);

describe('HeuristicSignalsSection', () => {
    it('renders the caveat first and verbatim, not collapsed or in a tooltip', () => {
        renderSection();

        const caveat = screen.getByTestId('heuristic-caveat');
        expect(caveat.textContent).toBe(HEURISTIC_CAVEAT);
        expect(caveat).toBeVisible();
        const content = screen.getByRole('region', { name: 'Classical technical signals' });
        expect(content.firstElementChild).toBe(screen.getByTestId('heuristic-caveat-callout'));
    });

    it('lists each API-flagged reading once with its badge and band', () => {
        renderSection();

        const rsi = screen.getByTestId('flagged-rsi');
        expect(rsi).toHaveTextContent('RSI 75.1notice(overbought)');
        expect(within(rsi).getByTestId('severity-badge')).toHaveAttribute('data-severity', 'notice');
        const vix = screen.getByTestId('flagged-vix');
        expect(vix).toHaveTextContent('VIX 26.2warning(elevated)');
        expect(within(vix).getByTestId('severity-badge')).toHaveAttribute('data-severity', 'warning');
        expect(within(screen.getByTestId('flagged-readings')).getAllByRole('listitem')).toHaveLength(2);
    });

    it('does not list a reading the API did not flag, whatever its value', () => {
        const analysis = makeAnalysis();
        analysis.technical.rsi_14 = { value: 91, band: 'overbought', severity: null };
        analysis.technical.bb_squeeze = { active: true, severity: null };
        renderSection(analysis);

        expect(screen.queryByTestId('flagged-rsi')).not.toBeInTheDocument();
        expect(screen.queryByTestId('flagged-bb_squeeze')).not.toBeInTheDocument();
        expect(screen.getByTestId('flagged-vix')).toBeInTheDocument();
    });

    it('lists a flagged BB squeeze with its state', () => {
        const analysis = makeAnalysis();
        analysis.technical.bb_squeeze = { active: true, severity: 'info' };
        renderSection(analysis);

        expect(screen.getByTestId('flagged-bb_squeeze')).toHaveTextContent('BB squeezeinfo(active)');
    });

    it('shows chart patterns as name · confirmed/unconfirmed with a severity badge', () => {
        renderSection();

        const bear = screen.getByTestId('pattern-bear_flag');
        expect(bear).toHaveTextContent('Bear flag· confirmednotice');
        expect(within(bear).getByTestId('severity-badge')).toHaveAttribute('data-severity', 'notice');
        expect(screen.getByTestId('pattern-double_top')).toHaveTextContent('Double top· unconfirmedinfo');
    });

    it('renders head & shoulders pattern names with the ampersand', () => {
        const analysis = makeAnalysis();
        analysis.heuristic_signals.chart_patterns = [
            { pattern: 'head_shoulders', confirmed: true, severity: 'notice' },
            { pattern: 'inv_head_shoulders', confirmed: false, severity: 'info' },
        ];
        renderSection(analysis);

        expect(screen.getByTestId('pattern-head_shoulders')).toHaveTextContent('Head & shoulders· confirmednotice');
        expect(screen.getByTestId('pattern-inv_head_shoulders')).toHaveTextContent('Inverse head & shoulders· unconfirmedinfo');
    });

    it('renders the action signal: label as-is, badge, "3/4" as text (no progress bar), reasoning verbatim', () => {
        renderSection();

        expect(screen.getByTestId('action-label')).toHaveTextContent(/^BUY_WATCH$/);
        expect(screen.getByTestId('action-severity')).toHaveTextContent('notice');
        expect(screen.getByTestId('action-confluence')).toHaveTextContent('Confluence 3/4');
        expect(screen.queryByRole('progressbar')).not.toBeInTheDocument();
        expect(screen.queryByRole('meter')).not.toBeInTheDocument();
        expect(within(screen.getByTestId('action-reasoning')).getAllByRole('listitem').map(li => li.textContent)).toEqual([
            'Low sweep (6 recent): stop-hunt below swing low detected',
            'Closed back above swept level — institutional accumulation pattern',
        ]);
        expect(screen.getByTestId('action-source')).toHaveTextContent('From liquidity sweep · VIX regime read by the rule: normal');
    });

    it('shows empty notes when nothing is flagged, detected or signalled', () => {
        renderSection(makeEmptyAnalysis());

        expect(screen.getByTestId('flagged-readings-empty')).toBeInTheDocument();
        expect(screen.getByTestId('chart-patterns-empty')).toBeInTheDocument();
        expect(screen.getByTestId('action-signal-empty')).toBeInTheDocument();
        expect(screen.getByTestId('heuristic-caveat')).toHaveTextContent(HEURISTIC_CAVEAT);
    });

    it('uses its own framing class and persists under scanner.detail.classical-signals', async () => {
        renderSection();

        expect(screen.getByTestId('analysis-heuristic')).toHaveClass('scanner-heuristic');
        await userEvent.click(screen.getByRole('button', { name: 'Classical technical signals' }));
        expect(window.sessionStorage.getItem(STORAGE_KEY)).toBe('false');
    });

    it('opens expanded, highlighted and scrolled into view when arrived at via the deep link', () => {
        window.sessionStorage.setItem(STORAGE_KEY, 'false');
        const scrollIntoView = jest.fn();
        Element.prototype.scrollIntoView = scrollIntoView;

        renderSection(makeAnalysis(), true);

        const toggle = screen.getByRole('button', { name: 'Classical technical signals' });
        expect(toggle).toHaveAttribute('aria-expanded', 'true');
        expect(screen.getByTestId('analysis-heuristic')).toHaveClass('is-focused');
        expect(scrollIntoView).toHaveBeenCalled();
        expect(toggle).toHaveFocus();
    });

    it('keeps a collapsed state when not deep-linked', () => {
        window.sessionStorage.setItem(STORAGE_KEY, 'false');
        renderSection();

        expect(screen.getByRole('button', { name: 'Classical technical signals' })).toHaveAttribute('aria-expanded', 'false');
    });
});
