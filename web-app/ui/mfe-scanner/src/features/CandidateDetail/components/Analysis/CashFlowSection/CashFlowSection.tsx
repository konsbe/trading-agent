import { CollapsibleCard } from '@trading-agent/shared-components';
import { CashFlowLine, CashFlowStatement } from '@/api';
import { EMPTY_VALUE, formatCompactUsd } from '@/common/format/format';
import FactGrid, { FactCell } from '../../FactGrid';
import { CashFlowSectionProps } from './types';
import '@/styles/scanner-global.css';
import '../analysis-styles.css';

export const ANNUAL_ONLY_NOTE = 'Annual figures only — quarterly cash-flow figures are year-to-date.';

export const NO_CASH_FLOW_FALLBACK = 'No cash-flow statement available.';

/** Lines the API serves as positive amounts paid out. */
const PAYMENT_KEYS = new Set(['capex', 'buybacks', 'dividends']);

/** A filing date (`YYYY-MM-DD`) as a calendar date, "31 Dec 2025" — read locally, never shifted by UTC. */
export const formatFilingDate = (value: string | null): string | null => {
    if (!value) return null;
    const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(value);
    if (!match) return value;
    return new Date(Number(match[1]), Number(match[2]) - 1, Number(match[3])).toLocaleDateString('en-GB', {
        day: 'numeric',
        month: 'short',
        year: 'numeric',
    });
};

/** "10-K · fiscal year ended 31 Dec 2025 · filed 18 Feb 2026"; parts the API left null are skipped. */
export const describeFiling = (cashFlow: CashFlowStatement): string => {
    const periodEnd = formatFilingDate(cashFlow.period_end);
    const filed = formatFilingDate(cashFlow.filed);
    const isAnnual = cashFlow.form?.startsWith('10-K') ?? true;
    return [
        cashFlow.form,
        periodEnd && `${isAnnual ? 'fiscal year ended' : 'period ended'} ${periodEnd}`,
        filed && `filed ${filed}`,
    ]
        .filter(Boolean)
        .join(' · ');
};

/** Compact USD as filed (−$25.93B for an outflow); a line the filing lacks is "—", not zero. */
export const buildCashFlowCells = (lines: CashFlowLine[]): FactCell[] =>
    lines.map(line => {
        if (line.value === null) return { key: line.key, label: line.label, value: EMPTY_VALUE, sub: 'not in filing' };
        return {
            key: line.key,
            label: line.label,
            value: formatCompactUsd(line.value),
            sub: PAYMENT_KEYS.has(line.key) ? 'amount paid' : undefined,
        };
    });

/**
 * The latest annual (10-K) cash-flow statement as filed, next to the balance
 * sheet. Plain figures: no red/green, which the design system keeps for price
 * moves. When there is no statement the card still renders, with the API's reason.
 */
const CashFlowSection = ({ cashFlow }: CashFlowSectionProps) => (
    <CollapsibleCard
        id="scanner-analysis-cash-flow"
        persistKey="scanner.detail.cash-flow"
        data-testid="analysis-cash-flow"
        title="Cash flow"
        meta={
            cashFlow.available ? (
                <span className="scanner-muted scanner-analysis__meta" data-testid="cash-flow-filing">
                    {describeFiling(cashFlow)}
                </span>
            ) : undefined
        }
    >
        {cashFlow.available ? (
            <>
                <FactGrid cells={buildCashFlowCells(cashFlow.lines)} testIdPrefix="cash-flow" />
                <p className="scanner-muted scanner-analysis__note" data-testid="cash-flow-note">
                    {ANNUAL_ONLY_NOTE}
                </p>
            </>
        ) : (
            <p className="scanner-muted scanner-analysis__note" data-testid="cash-flow-unavailable">
                {cashFlow.unavailable_reason || NO_CASH_FLOW_FALLBACK}
            </p>
        )}
    </CollapsibleCard>
);

export default CashFlowSection;
