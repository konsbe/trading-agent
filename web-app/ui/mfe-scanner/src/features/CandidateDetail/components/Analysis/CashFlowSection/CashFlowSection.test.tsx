import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {
    FUND_CASH_FLOW_REASON,
    makeCashFlow,
    makeShelCashFlow,
    makeTsmCashFlow,
    makeUnavailableCashFlow,
    NOT_COMPARABLE_NOTE,
} from '@/test-utils/analysisFixtures';
import { formatCompactAmount } from '@/common/format/format';
import CashFlowSection, {
    ANNUAL_ONLY_NOTE,
    describeCurrency,
    describeFiling,
    describeNewerFiling,
    formatFilingDate,
    NO_CASH_FLOW_FALLBACK,
} from '.';

beforeEach(() => window.sessionStorage.clear());

const value = (key: string) => screen.getByTestId(`cash-flow-${key}-value`);
const cell = (key: string) => screen.getByTestId(`cash-flow-${key}`);

describe('CashFlowSection', () => {
    it('shows the filing in the header: form, named fiscal year, filed date and served source', () => {
        render(<CashFlowSection cashFlow={makeCashFlow()} />);

        expect(screen.getByTestId('cash-flow-filing')).toHaveTextContent(
            /^10-K · fiscal year 2025, ended 31 Dec 2025 · filed 18 Feb 2026 · 10-K via Finnhub$/
        );
        expect(screen.getByTestId('cash-flow-currency')).toHaveTextContent(/^Figures in USD, as reported$/);
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

    it('shows a line the filing lacks as "—" with the served note, never as zero', () => {
        const cashFlow = makeCashFlow();
        cashFlow.lines[4] = { ...cashFlow.lines[4], value: null, missing_note: 'not in filing' };
        render(<CashFlowSection cashFlow={cashFlow} />);

        expect(value('buybacks')).toHaveTextContent(/^—$/);
        expect(cell('buybacks')).toHaveTextContent('not in filing');
        expect(cell('buybacks')).not.toHaveTextContent('amount paid');
    });

    it('adds no client-side note to a null line the API gave no reason for', () => {
        const cashFlow = makeCashFlow();
        cashFlow.lines[4] = { ...cashFlow.lines[4], value: null, missing_note: null };
        render(<CashFlowSection cashFlow={cashFlow} />);

        expect(cell('buybacks')).not.toHaveTextContent('not in filing');
        expect(value('buybacks')).toHaveTextContent(/^—$/);
    });

    describe('a 20-F in its reporting currency (live TSM)', () => {
        it('names the form, fiscal year, filing, source and the currency, not converted', () => {
            render(<CashFlowSection cashFlow={makeTsmCashFlow()} />);

            expect(screen.getByTestId('cash-flow-filing')).toHaveTextContent(
                /^20-F · fiscal year 2024, ended 31 Dec 2024 · filed 17 Apr 2025 · 20-F via SEC EDGAR$/
            );
            expect(screen.getByTestId('cash-flow-currency')).toHaveTextContent(/^Figures in TWD, as reported — not converted$/);
        });

        it('formats every figure with the ISO code, never a "$"', () => {
            render(<CashFlowSection cashFlow={makeTsmCashFlow()} />);

            expect(value('operating')).toHaveTextContent(/^TWD 1,826\.18B$/);
            expect(value('investing')).toHaveTextContent(/^−TWD 864\.84B$/);
            expect(value('financing')).toHaveTextContent(/^−TWD 346\.30B$/);
            expect(value('capex')).toHaveTextContent(/^TWD 956\.01B$/);
            expect(value('dividends')).toHaveTextContent(/^TWD 363\.06B$/);
            screen.getAllByTestId(/^cash-flow-[a-z]+-value$/).forEach(el => expect(el).not.toHaveTextContent('$'));
        });

        it('shows the buybacks line as "—" with "not reported as a comparable line"', () => {
            render(<CashFlowSection cashFlow={makeTsmCashFlow()} />);

            expect(value('buybacks')).toHaveTextContent(/^—$/);
            expect(cell('buybacks')).toHaveTextContent(NOT_COMPARABLE_NOTE);
        });

        it('says a newer 20-F is on SEC EDGAR but not in companyfacts yet, and keeps the annual-only note', () => {
            render(<CashFlowSection cashFlow={makeTsmCashFlow()} />);

            expect(screen.getByTestId('cash-flow-newer-filing')).toHaveTextContent(
                /^A newer 20-F \(fiscal year ended 31 Dec 2025, filed 16 Apr 2026\) is on SEC EDGAR but not in its companyfacts data yet\.$/
            );
            expect(screen.getByTestId('cash-flow-note')).toHaveTextContent(ANNUAL_ONLY_NOTE);
        });
    });

    it('shows a USD 20-F in compact USD with its source, and capex "not reported as a comparable line" (live SHEL)', () => {
        render(<CashFlowSection cashFlow={makeShelCashFlow()} />);

        expect(screen.getByTestId('cash-flow-filing')).toHaveTextContent(
            /^20-F · fiscal year 2025, ended 31 Dec 2025 · filed 12 Mar 2026 · 20-F via SEC EDGAR$/
        );
        expect(value('operating')).toHaveTextContent(/^\$42\.86B$/);
        expect(value('capex')).toHaveTextContent(/^—$/);
        expect(cell('capex')).toHaveTextContent(NOT_COMPARABLE_NOTE);
        expect(screen.queryByTestId('cash-flow-newer-filing')).not.toBeInTheDocument();
    });

    it('carries the annual-only note', () => {
        render(<CashFlowSection cashFlow={makeCashFlow()} />);

        expect(screen.getByTestId('cash-flow-note')).toHaveTextContent(ANNUAL_ONLY_NOTE);
    });

    it('never renders a blank card when unavailable: it shows the reason, no lines, no filing', () => {
        render(<CashFlowSection cashFlow={makeUnavailableCashFlow()} />);

        expect(screen.getByRole('heading', { name: 'Cash flow' })).toBeInTheDocument();
        expect(screen.getByTestId('cash-flow-unavailable')).toHaveTextContent(FUND_CASH_FLOW_REASON);
        expect(screen.queryByTestId('cash-flow-filing')).not.toBeInTheDocument();
        expect(screen.queryByTestId('cash-flow-currency')).not.toBeInTheDocument();
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

describe('describeFiling / formatFilingDate / describeCurrency / describeNewerFiling', () => {
    it('reads a filing date as a calendar date', () => {
        expect(formatFilingDate('2025-12-31')).toBe('31 Dec 2025');
        expect(formatFilingDate(null)).toBeNull();
        expect(formatFilingDate('Dec 2025')).toBe('Dec 2025');
    });

    it('says "period ended" for a non-annual form and skips null parts', () => {
        expect(describeFiling(makeCashFlow({ form: '10-Q', filed: null, source: null }))).toBe('10-Q · period ended 31 Dec 2025');
        expect(describeFiling(makeCashFlow({ form: null, period_end: null, fiscal_year: null, source: null }))).toBe('filed 18 Feb 2026');
    });

    it('falls back to "fiscal year ended" without a fiscal year (older API)', () => {
        expect(describeFiling(makeCashFlow({ fiscal_year: null, source: null }))).toBe(
            '10-K · fiscal year ended 31 Dec 2025 · filed 18 Feb 2026'
        );
        expect(describeFiling(makeCashFlow({ period_end: null }))).toBe('10-K · fiscal year 2025 · filed 18 Feb 2026 · 10-K via Finnhub');
    });

    it('names the currency once, and says a non-USD figure is not converted', () => {
        expect(describeCurrency('USD')).toBe('Figures in USD, as reported');
        expect(describeCurrency('twd')).toBe('Figures in TWD, as reported — not converted');
        expect(describeCurrency(null)).toBeNull();
    });

    it('describes a newer filing, skipping parts the API left null', () => {
        expect(describeNewerFiling({ form: '20-F', filed: null, period_end: null })).toBe(
            'A newer 20-F is on SEC EDGAR but not in its companyfacts data yet.'
        );
        expect(describeNewerFiling({ form: null, filed: '2026-04-16', period_end: null })).toBe(
            'A newer annual report (filed 16 Apr 2026) is on SEC EDGAR but not in its companyfacts data yet.'
        );
    });
});

describe('formatCompactAmount', () => {
    it('keeps compact USD for USD and a missing currency', () => {
        expect(formatCompactAmount(51_970_000_000, 'USD')).toBe('$51.97B');
        expect(formatCompactAmount(-25_927_000_000, null)).toBe('−$25.93B');
    });

    it('names any other currency by its ISO code with the same scale', () => {
        expect(formatCompactAmount(1_826_177_100_000, 'TWD')).toBe('TWD 1,826.18B');
        expect(formatCompactAmount(-4_500_000, 'EUR')).toBe('−EUR 4.50M');
        expect(formatCompactAmount(12_300, 'GBP')).toBe('GBP 12.3K');
        expect(formatCompactAmount(900, 'JPY')).toBe('JPY 900');
        expect(formatCompactAmount(null, 'TWD')).toBe('—');
    });
});
