import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { makeCashFlow, makeUnavailableCashFlow, SHEL_CASH_FLOW_REASON } from '@/test-utils/analysisFixtures';
import CashFlowSection, { ANNUAL_ONLY_NOTE, describeFiling, formatFilingDate, NO_CASH_FLOW_FALLBACK } from '.';

beforeEach(() => window.sessionStorage.clear());

const value = (key: string) => screen.getByTestId(`cash-flow-${key}-value`);
const cell = (key: string) => screen.getByTestId(`cash-flow-${key}`);

describe('CashFlowSection', () => {
    it('shows the filing in the header: form, fiscal year end and filed date', () => {
        render(<CashFlowSection cashFlow={makeCashFlow()} />);

        expect(screen.getByTestId('cash-flow-filing')).toHaveTextContent(/^10-K · fiscal year ended 31 Dec 2025 · filed 18 Feb 2026$/);
    });

    it('renders the six lines with their served labels in compact USD, outflows with a minus sign', () => {
        render(<CashFlowSection cashFlow={makeCashFlow()} />);

        expect(within(cell('operating')).getByRole('term')).toHaveTextContent('Net cash from operating activities');
        expect(value('operating')).toHaveTextContent(/^\$51\.97B$/);
        expect(value('investing')).toHaveTextContent(/^−\$25\.93B$/);
        expect(value('financing')).toHaveTextContent(/^−\$39\.08B$/);
        expect(value('capex')).toHaveTextContent(/^\$28\.36B$/);
        expect(value('buybacks')).toHaveTextContent(/^\$20\.27B$/);
        expect(value('dividends')).toHaveTextContent(/^\$17\.23B$/);
    });

    it('marks payments as amounts paid and uses no price colours', () => {
        render(<CashFlowSection cashFlow={makeCashFlow()} />);

        ['capex', 'buybacks', 'dividends'].forEach(key => expect(cell(key)).toHaveTextContent('amount paid'));
        ['operating', 'investing', 'financing'].forEach(key => expect(cell(key)).not.toHaveTextContent('amount paid'));
        screen.getAllByTestId(/^cash-flow-[a-z]+-value$/).forEach(el => expect(el.className).not.toMatch(/price-(up|down)/));
    });

    it('shows a line the filing lacks as "—" with "not in filing", never as zero', () => {
        const cashFlow = makeCashFlow();
        cashFlow.lines[4] = { ...cashFlow.lines[4], value: null };
        render(<CashFlowSection cashFlow={cashFlow} />);

        expect(value('buybacks')).toHaveTextContent(/^—$/);
        expect(cell('buybacks')).toHaveTextContent('not in filing');
    });

    it('carries the annual-only note', () => {
        render(<CashFlowSection cashFlow={makeCashFlow()} />);

        expect(screen.getByTestId('cash-flow-note')).toHaveTextContent(ANNUAL_ONLY_NOTE);
    });

    it('never renders a blank card when unavailable: it shows the reason, no lines, no filing', () => {
        render(<CashFlowSection cashFlow={makeUnavailableCashFlow()} />);

        expect(screen.getByRole('heading', { name: 'Cash flow' })).toBeInTheDocument();
        expect(screen.getByTestId('cash-flow-unavailable')).toHaveTextContent(SHEL_CASH_FLOW_REASON);
        expect(screen.queryByTestId('cash-flow-filing')).not.toBeInTheDocument();
        expect(screen.queryByTestId('cash-flow-note')).not.toBeInTheDocument();
        expect(screen.queryAllByTestId(/^cash-flow-[a-z]+-value$/)).toHaveLength(0);
    });

    it('falls back to a plain sentence when unavailable without a reason', () => {
        render(<CashFlowSection cashFlow={makeUnavailableCashFlow(null)} />);

        expect(screen.getByTestId('cash-flow-unavailable')).toHaveTextContent(NO_CASH_FLOW_FALLBACK);
    });

    it('is a collapsible card that remembers its state', async () => {
        const user = userEvent.setup();
        const { unmount } = render(<CashFlowSection cashFlow={makeCashFlow()} />);

        const toggle = screen.getByRole('button', { name: 'Cash flow' });
        expect(toggle).toHaveAttribute('aria-expanded', 'true');
        await user.click(toggle);
        expect(toggle).toHaveAttribute('aria-expanded', 'false');
        expect(screen.queryByTestId('cash-flow-operating')).not.toBeInTheDocument();
        unmount();

        render(<CashFlowSection cashFlow={makeCashFlow()} />);
        expect(screen.getByRole('button', { name: 'Cash flow' })).toHaveAttribute('aria-expanded', 'false');
    });
});

describe('describeFiling / formatFilingDate', () => {
    it('reads a filing date as a calendar date', () => {
        expect(formatFilingDate('2025-12-31')).toBe('31 Dec 2025');
        expect(formatFilingDate(null)).toBeNull();
        expect(formatFilingDate('Dec 2025')).toBe('Dec 2025');
    });

    it('says "period ended" for a non-annual form and skips null parts', () => {
        expect(describeFiling(makeCashFlow({ form: '10-Q', filed: null }))).toBe('10-Q · period ended 31 Dec 2025');
        expect(describeFiling(makeCashFlow({ form: null, period_end: null }))).toBe('filed 18 Feb 2026');
    });
});
