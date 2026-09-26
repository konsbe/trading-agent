import { render, screen, within } from '@testing-library/react';
import { makeAnalysis, makeEmptyAnalysis } from '@/test-utils/analysisFixtures';
import BalanceSheetSection from './BalanceSheetSection';

beforeEach(() => window.sessionStorage.clear());

const value = (key: string) => screen.getByTestId(`balance-sheet-${key}-value`);

describe('BalanceSheetSection', () => {
    it('shows the composite tier, then the ratios with their stored bands', () => {
        render(<BalanceSheetSection balanceSheet={makeAnalysis().balance_sheet} />);

        expect(screen.getByTestId('balance-sheet-composite')).toHaveTextContent('Composite1.00 · healthy');
        expect(value('roe')).toHaveTextContent('35.12% (excellent)');
        expect(value('roa')).toHaveTextContent('23.35% (strong)');
        expect(value('roic')).toHaveTextContent('28.40% (excellent)');
        expect(value('current_ratio')).toHaveTextContent('2.62 (safe)');
        expect(value('quick_ratio')).toHaveTextContent('2.42 (safe)');
        expect(value('debt_to_equity')).toHaveTextContent('0.25 (low)');
        expect(value('net_debt_ebitda')).toHaveTextContent(/^—$/);
    });

    it('labels the leverage ratio "Net debt / operating income"', () => {
        render(<BalanceSheetSection balanceSheet={makeAnalysis().balance_sheet} />);

        const cell = screen.getByTestId('balance-sheet-net_debt_ebitda');
        expect(within(cell).getByRole('term')).toHaveTextContent(/^Net debt \/ operating income$/);
        expect(screen.queryByText(/EBITDA/)).not.toBeInTheDocument();
    });

    it('displays the ROE band destroying_value as "low"; excellent and adequate unchanged', () => {
        const { balance_sheet } = makeAnalysis();
        const withRoe = (band: string, v: number | null = 5.2) => ({ ...balance_sheet, roe: { value: v, band } });

        const { rerender } = render(<BalanceSheetSection balanceSheet={withRoe('destroying_value')} />);
        expect(value('roe')).toHaveTextContent(/^5\.20% \(low\)$/);

        rerender(<BalanceSheetSection balanceSheet={withRoe('destroying_value', null)} />);
        expect(value('roe')).toHaveTextContent(/^Low$/);

        rerender(<BalanceSheetSection balanceSheet={withRoe('excellent')} />);
        expect(value('roe')).toHaveTextContent(/^5\.20% \(excellent\)$/);

        rerender(<BalanceSheetSection balanceSheet={withRoe('adequate')} />);
        expect(value('roe')).toHaveTextContent(/^5\.20% \(adequate\)$/);
    });

    it('displays the leverage band negative_ebitda as "operating loss"; other bands humanized', () => {
        const { balance_sheet } = makeAnalysis();
        const withLeverage = (value: number | null, band: string) => ({ ...balance_sheet, net_debt_ebitda: { value, band } });

        const { rerender } = render(<BalanceSheetSection balanceSheet={withLeverage(null, 'negative_ebitda')} />);
        expect(value('net_debt_ebitda')).toHaveTextContent(/^Operating loss$/);

        rerender(<BalanceSheetSection balanceSheet={withLeverage(-1.5, 'negative_ebitda')} />);
        expect(value('net_debt_ebitda')).toHaveTextContent(/^−1\.50 \(operating loss\)$/);

        rerender(<BalanceSheetSection balanceSheet={withLeverage(1.2, 'net_cash')} />);
        expect(value('net_debt_ebitda')).toHaveTextContent(/^1\.20 \(net cash\)$/);
        expect(screen.queryByText(/EBITDA/)).not.toBeInTheDocument();
    });

    it('does not remap destroying_value on other ratios', () => {
        const { balance_sheet } = makeAnalysis();
        render(<BalanceSheetSection balanceSheet={{ ...balance_sheet, roic: { value: 3.1, band: 'destroying_value' } }} />);

        expect(value('roic')).toHaveTextContent('3.10% (destroying value)');
    });

    it('renders null fields as "—" with no badges', () => {
        render(<BalanceSheetSection balanceSheet={makeEmptyAnalysis().balance_sheet} />);

        screen.getAllByTestId(/^balance-sheet-[a-z_]+-value$/).forEach(cell => expect(cell).toHaveTextContent(/^—$/));
        expect(within(screen.getByTestId('analysis-balance-sheet')).queryByTestId('severity-badge')).not.toBeInTheDocument();
    });
});
