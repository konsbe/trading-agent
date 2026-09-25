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

    it('renders null fields as "—" with no badges', () => {
        render(<BalanceSheetSection balanceSheet={makeEmptyAnalysis().balance_sheet} />);

        screen.getAllByTestId(/^balance-sheet-[a-z_]+-value$/).forEach(cell => expect(cell).toHaveTextContent(/^—$/));
        expect(within(screen.getByTestId('analysis-balance-sheet')).queryByTestId('severity-badge')).not.toBeInTheDocument();
    });
});
