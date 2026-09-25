import { CollapsibleCard } from '@trading-agent/shared-components';
import { BalanceSheetAnalysis } from '@/api';
import { formatNumber, formatPercent } from '@/common/format/format';
import FactGrid, { FactCell } from '../../FactGrid';
import { withBand } from '../../../utils/analysisFormat';
import CompositeLine from '../CompositeLine';
import { BalanceSheetSectionProps } from './types';
import '@/styles/scanner-global.css';
import '../analysis-styles.css';

export const buildBalanceSheetCells = (b: BalanceSheetAnalysis): FactCell[] => [
    { key: 'roe', label: 'ROE', value: withBand(formatPercent(b.roe.value, 2), b.roe.band) },
    { key: 'roa', label: 'ROA', value: withBand(formatPercent(b.roa.value, 2), b.roa.band) },
    { key: 'roic', label: 'ROIC', value: withBand(formatPercent(b.roic.value, 2), b.roic.band) },
    { key: 'current_ratio', label: 'Current ratio', value: withBand(formatNumber(b.current_ratio.value, 2), b.current_ratio.band) },
    { key: 'quick_ratio', label: 'Quick ratio', value: withBand(formatNumber(b.quick_ratio.value, 2), b.quick_ratio.band) },
    { key: 'debt_to_equity', label: 'Debt / equity', value: withBand(formatNumber(b.debt_to_equity.value, 2), b.debt_to_equity.band) },
    { key: 'net_debt_ebitda', label: 'Net debt / EBITDA', value: withBand(formatNumber(b.net_debt_ebitda.value, 2), b.net_debt_ebitda.band) },
];

/** Section 3 — balance-sheet composite tier, then the stored ratios as a plain grid. */
const BalanceSheetSection = ({ balanceSheet }: BalanceSheetSectionProps) => (
    <CollapsibleCard
        id="scanner-analysis-balance-sheet"
        persistKey="scanner.detail.balance-sheet"
        data-testid="analysis-balance-sheet"
        title="Balance sheet"
    >
        <CompositeLine composite={balanceSheet.composite} data-testid="balance-sheet-composite" />
        <FactGrid cells={buildBalanceSheetCells(balanceSheet)} testIdPrefix="balance-sheet" />
    </CollapsibleCard>
);

export default BalanceSheetSection;
