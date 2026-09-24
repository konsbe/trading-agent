import { fireEvent, render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { ApiError, BacktestLabReport } from '@/api';
import useBacktestReport, { UseBacktestReport } from '@/hooks/backtestLab/useBacktestReport';
import { HostModeProvider } from '@/providers/HostModeContext';
import { makeReport } from '@/test-utils/fixtures';
import BacktestLabPage from './BacktestLabPage';

jest.mock('@/hooks/backtestLab/useBacktestReport', () => ({ __esModule: true, default: jest.fn() }));

const hookMock = useBacktestReport as jest.MockedFunction<typeof useBacktestReport>;

const withState = (state: Partial<UseBacktestReport>) =>
    hookMock.mockReturnValue({ report: null, error: null, isLoading: false, ...state });

const renderReport = (report: BacktestLabReport = makeReport(), { hosted = true } = {}) => {
    withState({ report });
    const utils = render(
        <HostModeProvider hosted={hosted}>
            <BacktestLabPage />
        </HostModeProvider>
    );
    return { ...utils, report };
};

const toggleOf = (sectionTestId: string) =>
    screen.getByTestId(sectionTestId).querySelector<HTMLButtonElement>('.ta-collapsible-card__toggle')!;

beforeEach(() => window.sessionStorage.clear());

describe('BacktestLabPage states', () => {
    it('shows a skeleton while loading', () => {
        withState({ isLoading: true });
        render(<BacktestLabPage />);

        expect(screen.getByTestId('report-skeleton')).toHaveAttribute('aria-busy', 'true');
        expect(screen.queryByTestId('backtest-report')).not.toBeInTheDocument();
    });

    it('shows the error state without a retry', () => {
        withState({ error: new ApiError(0, 'network_error') });
        render(<BacktestLabPage />);

        expect(screen.getByRole('alert')).toHaveTextContent("Couldn't reach the Backtest Lab service.");
        expect(screen.queryByRole('button')).not.toBeInTheDocument();
        expect(screen.queryByTestId('backtest-report')).not.toBeInTheDocument();
        expect(screen.queryByTestId('report-skeleton')).not.toBeInTheDocument();
    });

    it('shows its own title standalone and leaves it to the shell when hosted', () => {
        const { unmount } = renderReport(makeReport(), { hosted: false });
        expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('Backtest Lab');
        unmount();

        renderReport();
        expect(screen.queryByRole('heading', { level: 1 })).not.toBeInTheDocument();
    });
});

describe('BacktestLabPage header', () => {
    it('shows the closed date as a readable, non-live subtitle', () => {
        renderReport();

        const subtitle = screen.getByTestId('report-subtitle');
        expect(subtitle).toHaveTextContent('Phase 2 research report — closed Sep 22, 2026.');
        expect(within(subtitle).getByText('Sep 22, 2026')).toHaveAttribute('dateTime', '2026-09-22');
    });

    it('shows the headline verbatim before every section', () => {
        const { report } = renderReport();

        const headline = screen.getByTestId('report-headline-text');
        expect(headline.textContent).toBe(report.report.headline);
        expect(headline.compareDocumentPosition(screen.getByTestId('entry-gate-section'))).toBe(
            Node.DOCUMENT_POSITION_FOLLOWING
        );
    });
});

describe('Section 1 — entry gate', () => {
    it('is always expanded and has no toggle', () => {
        const { report } = renderReport();
        const section = screen.getByTestId('entry-gate-section');

        expect(within(section).getByRole('heading', { level: 2 })).toHaveTextContent(report.entry_gate.title);
        expect(within(section).queryByRole('button')).not.toBeInTheDocument();
        expect(section).not.toHaveAttribute('aria-expanded');
    });

    it('foregrounds the rule-committed marker with an icon', () => {
        renderReport();

        const marker = screen.getByTestId('rule-committed-marker');
        expect(marker).toHaveTextContent('Rule committed before the result was seen');
        expect(marker.querySelector('svg')).toBeInTheDocument();
    });

    it('says so plainly when the rule was not committed first', () => {
        const report = makeReport();
        report.entry_gate.rule_committed_before_result = false;
        renderReport(report);

        expect(screen.queryByTestId('rule-committed-marker')).not.toBeInTheDocument();
        expect(screen.getByTestId('rule-not-committed')).toBeInTheDocument();
    });

    it('shows the stats at their authored precision', () => {
        renderReport();

        expect(screen.getByTestId('gate-stat-chi-square-value').textContent).toBe('0.00');
        expect(screen.getByTestId('gate-stat-p-value-value').textContent).toBe('0.947');
        expect(screen.getByTestId('gate-stat-mh-or-value').textContent).toBe('0.991');
    });

    it('states the requirement, verdict, route, test, note and sample as plain text', () => {
        const { report } = renderReport();
        const gate = report.entry_gate;

        expect(screen.getByTestId('gate-required').textContent).toBe('positive direction AND p < 0.05');
        expect(screen.getByTestId('gate-verdict').textContent).toBe(gate.result.verdict);
        expect(screen.getByTestId('gate-route')).toHaveTextContent(gate.route_taken);
        expect(screen.getByText(gate.test)).toBeInTheDocument();
        expect(screen.getByTestId('gate-note').textContent).toBe(gate.note);
        expect(screen.getByTestId('gate-sample')).toHaveTextContent(
            'Sample: 9,407 candidates · 6,936 episodes · base rate 9.90% · excludes the lockbox region and population A (in-sample pilot)'
        );
    });
});

describe('Section 2 — v2 score finding', () => {
    it('contrasts the pooled pass with the per-bucket p-values', () => {
        const { report } = renderReport();
        const pooled = screen.getByTestId('v2-pooled');
        const stratified = screen.getByTestId('v2-stratified');

        expect(pooled).toHaveTextContent('looked like a pass');
        expect(within(pooled).getByTestId('v2-pooled-p').textContent).toBe('0.017');
        expect(within(stratified).getByTestId('v2-penny-p').textContent).toBe('0.891');
        expect(within(stratified).getByTestId('v2-market-p').textContent).toBe('0.645');
        expect(screen.getByTestId('v2-stratified-verdict').textContent).toBe('separates in NEITHER bucket');
        expect(screen.getByTestId('v2-explanation').textContent).toBe(report.v2_score_finding.explanation);
        expect(screen.getByTestId('v2-sample')).toHaveTextContent(report.v2_score_finding.sample);
    });

    it('calls out the composition share', () => {
        renderReport();
        expect(screen.getByTestId('v2-composition')).toHaveTextContent('97%of the apparent effect was bucket composition');
    });
});

describe('Section 3 — rvol stratification funnel', () => {
    it('renders every step label exactly, with its odds ratio', () => {
        const { report } = renderReport();
        const steps = report.rvol_stratification_funnel.steps;

        steps.forEach((step, index) => {
            expect(screen.getByTestId(`funnel-step-${index}-label`).textContent).toBe(step.label);
            expect(screen.getByTestId(`funnel-step-${index}-or`)).toHaveTextContent(`Odds ratio ${step.odds_ratio.toFixed(3)}`);
        });
        expect(screen.getByTestId('funnel-step-3-label').textContent).toBe(
            'Bucket x ATR tercile, on the corrected candidate set (gate v2 with us-gaap fallback)'
        );
        expect(screen.getByTestId('funnel-step-3-or')).toHaveTextContent('0.991');
    });

    it('shows excess odds, composition share and the final verdict', () => {
        renderReport();

        expect(screen.getByTestId('funnel-step-0')).toHaveTextContent('excess odds 0.529');
        expect(screen.getByTestId('funnel-step-1')).toHaveTextContent('59% of the crude effect was composition');
        expect(screen.getByTestId('funnel-step-2')).toHaveTextContent('64% of the crude effect was composition');
        expect(screen.getByTestId('funnel-verdict').textContent).toBe('indistinguishable from no effect');
        expect(screen.getByTestId('funnel-step-3')).toContainElement(screen.getByTestId('funnel-verdict'));
    });

    it('sizes bars on one linear scale from 0 with a labelled OR 1.0 line', () => {
        const { report } = renderReport();
        const steps = report.rvol_stratification_funnel.steps;
        const chart = screen.getByTestId('funnel-chart');
        const scaleMax = Number(chart.getAttribute('data-scale-max'));

        steps.forEach((step, index) => {
            const bar = screen.getByTestId(`funnel-bar-${index}`);
            const width = Number(bar.getAttribute('data-width'));
            expect(width).toBeCloseTo((step.odds_ratio / scaleMax) * 100, 6);
            expect(bar.style.width).toBe(`${width}%`);
            expect(bar.closest('[aria-hidden="true"]')).not.toBeNull();
        });
        const [first, , , last] = steps.map((_, i) => Number(screen.getByTestId(`funnel-bar-${i}`).getAttribute('data-width')));
        expect(last / first).toBeCloseTo(steps[3].odds_ratio / steps[0].odds_ratio, 6);

        const refPct = (1 / scaleMax) * 100;
        const refLabel = screen.getByTestId('funnel-reference-label');
        expect(refLabel).toHaveTextContent('OR = 1.0 · no effect');
        expect(Number(refLabel.getAttribute('data-position'))).toBeCloseTo(refPct, 6);
        expect(chart.style.getPropertyValue('--backtest-funnel-ref')).toBe(`${refPct}%`);
        expect(screen.getByTestId('funnel-sample')).toHaveTextContent(report.rvol_stratification_funnel.sample);
    });
});

describe('Section 4 — research round 1', () => {
    it('shows the title and one row per hypothesis, tested or abandoned, in id order', () => {
        const { report } = renderReport();
        const section = screen.getByTestId('round1-section');

        expect(within(section).getByRole('heading', { level: 2 })).toHaveTextContent(report.research_round_1.title);
        const rows = within(screen.getByTestId('round1-table')).getAllByTestId(/^round1-row-/);
        expect(rows.map(r => r.getAttribute('data-testid'))).toEqual(
            ['a', 'b', 'c', 'd', 'e'].map(id => `round1-row-${id}`)
        );
    });

    it('shows effect as odds ratio plus CI, and every reason as muted text', () => {
        const { report } = renderReport();

        expect(screen.getByTestId('round1-effect-a').textContent).toBe('1.195 [0.986, 1.449]');
        expect(screen.getByTestId('round1-effect-b').textContent).toBe('1.616 [1.389, 1.880]');
        expect(screen.getByTestId('round1-effect-d').textContent).toBe('1.014 [0.830, 1.239]');
        [...report.research_round_1.hypotheses, ...report.research_round_1.abandoned].forEach(h => {
            const reason = screen.getByTestId(`round1-reason-${h.id}`);
            expect(reason.textContent).toBe(h.reason);
            expect(reason.className).toBe('backtest-muted backtest-round__reason');
        });
    });

    it('renders every verdict as plain text with the same classes and no colour or badge', () => {
        renderReport();
        const verdicts = ['a', 'b', 'c', 'd', 'e'].map(id => screen.getByTestId(`round1-verdict-${id}`));

        expect(verdicts.map(v => v.textContent)).toEqual(['FAIL', 'FAIL', 'Abandoned before testing', 'FAIL', 'FAIL']);
        verdicts.forEach(v => {
            expect(v.className).toBe('backtest-round__verdict');
            expect(v.children).toHaveLength(0);
        });
        const table = screen.getByTestId('round1-table');
        table.querySelectorAll('*').forEach(el => {
            expect(el.getAttribute('class') ?? '').not.toMatch(/price|success|error|danger|pass|fail|badge|pill/i);
        });
        expect(table.querySelectorAll('svg')).toHaveLength(1); // the note chevron only
    });

    it('shows the abandoned row with an em dash effect', () => {
        renderReport();

        expect(screen.getByTestId('round1-verdict-c').textContent).toBe('Abandoned before testing');
        expect(screen.getByTestId('round1-effect-c').textContent).toBe('—');
    });

    it('offers the verdict note only on b, hidden until toggled', async () => {
        const user = userEvent.setup();
        const { report } = renderReport();
        const note = report.research_round_1.hypotheses.find(h => h.id === 'b')!.verdict_note!;

        const toggles = screen.getAllByTestId(/^round1-note-toggle-/);
        expect(toggles).toHaveLength(1);
        const toggle = screen.getByTestId('round1-note-toggle-b');
        expect(toggle).toHaveAccessibleName('Note on hypothesis b');
        expect(toggle).toHaveAttribute('aria-expanded', 'false');
        const row = screen.getByTestId('round1-note-b');
        expect(toggle).toHaveAttribute('aria-controls', row.id);
        expect(row).not.toBeVisible();

        await user.click(toggle);
        expect(toggle).toHaveAttribute('aria-expanded', 'true');
        expect(row).toBeVisible();
        expect(row.textContent).toBe(note);
        expect(screen.getByTestId('round1-row-b').compareDocumentPosition(row)).toBe(Node.DOCUMENT_POSITION_FOLLOWING);

        await user.click(toggle);
        expect(toggle).toHaveAttribute('aria-expanded', 'false');
        expect(row).not.toBeVisible();
    });

    it('keeps an open note open across a collapse', async () => {
        const user = userEvent.setup();
        renderReport();

        await user.click(screen.getByTestId('round1-note-toggle-b'));
        await user.click(toggleOf('round1-section'));
        expect(screen.queryByTestId('round1-note-b')).not.toBeInTheDocument();
        await user.click(toggleOf('round1-section'));
        expect(screen.getByTestId('round1-note-b')).toBeVisible();
    });

    it('shows the round sample with the lockbox state', () => {
        renderReport();
        expect(screen.getByTestId('round1-sample')).toHaveTextContent(
            'Sample: 7,579 episodes · excludes the lockbox region and the in-sample pilot window · lockbox not opened'
        );
    });

    it('says when the lockbox was opened, and drops a sample meant for another section', () => {
        const report = makeReport();
        report.sample_size.lockbox_opened = true;
        const { unmount } = renderReport(report);
        expect(screen.getByTestId('round1-sample')).toHaveTextContent('lockbox opened');
        unmount();

        const other = makeReport();
        other.sample_size.applies_to = 'research_round_2';
        renderReport(other);
        expect(screen.queryByTestId('round1-sample')).not.toBeInTheDocument();
    });
});

describe('Section 5 — closing statement', () => {
    it('is the final, always-expanded block with the verbatim statement and screener line', () => {
        const { report } = renderReport();
        const section = screen.getByTestId('closing-section');

        expect(screen.getByTestId('closing-statement').textContent).toBe(report.closing_statement);
        expect(screen.getByTestId('closing-screener-line').textContent).toBe(
            'The screener (gate-pass alerts, no score) is what ships as a result of this finding.'
        );
        expect(within(section).queryByRole('button')).not.toBeInTheDocument();
        expect(screen.getByTestId('backtest-report').lastElementChild).toBe(section);
    });
});

describe('collapsible sections', () => {
    it.each([
        ['v2-score-section', 'backtest.v2'],
        ['funnel-section', 'backtest.funnel'],
        ['round1-section', 'backtest.round1'],
    ])('%s collapses to its header and persists under %s', async (testId, key) => {
        const user = userEvent.setup();
        renderReport();
        const section = screen.getByTestId(testId);
        const toggle = toggleOf(testId);
        const contentId = toggle.getAttribute('aria-controls')!;

        await user.click(toggle);
        expect(toggle).toHaveAttribute('aria-expanded', 'false');
        expect(section.querySelector(`#${contentId}`)).not.toBeVisible();
        expect(section.querySelector(`#${contentId}`)!.childElementCount).toBe(0);
        expect(window.sessionStorage.getItem(`ta-collapsible:${key}`)).toBe('false');

        toggle.focus();
        await user.keyboard('{Enter}');
        expect(toggle).toHaveAttribute('aria-expanded', 'true');
        await user.keyboard(' ');
        expect(toggle).toHaveAttribute('aria-expanded', 'false');
    });

    it('restores a persisted collapsed state', () => {
        window.sessionStorage.setItem('ta-collapsible:backtest.funnel', 'false');
        renderReport();

        expect(within(screen.getByTestId('funnel-section')).getByRole('button')).toHaveAttribute('aria-expanded', 'false');
        expect(screen.queryByTestId('funnel-chart')).not.toBeInTheDocument();
    });

    it('never makes sections 1 or 5 collapsible', () => {
        renderReport();
        const toggles = document.querySelectorAll('.ta-collapsible-card__toggle');

        expect(toggles).toHaveLength(3);
        ['entry-gate-section', 'closing-section'].forEach(id => {
            expect(screen.getByTestId(id).querySelector('[aria-expanded]')).toBeNull();
        });
    });
});

describe('frozen-report guard', () => {
    it('renders no links, no controls besides collapse toggles and the note, and no live wording', () => {
        const { container } = renderReport();
        fireEvent.click(screen.getByTestId('round1-note-toggle-b'));

        expect(container.querySelectorAll('a')).toHaveLength(0);
        expect(container.querySelectorAll('input, select, textarea, form')).toHaveLength(0);
        const buttons = screen.getAllByRole('button');
        expect(buttons).toHaveLength(4);
        buttons.forEach(button => {
            const isCollapse = button.classList.contains('ta-collapsible-card__toggle');
            const isNote = button.getAttribute('data-testid') === 'round1-note-toggle-b';
            expect(isCollapse || isNote).toBe(true);
        });

        const text = container.textContent ?? '';
        expect(text).not.toMatch(/refresh|re-?run|retry|reload|score:|\/candidates/i);
        container.querySelectorAll('*').forEach(el => {
            expect(el.getAttribute('class') ?? '').not.toMatch(/price-up|price-down|success|error/i);
        });
    });
});
