import { CollapsibleCard } from '@trading-agent/shared-components';
import { CashFlowLine, CashFlowStatement, NewerFiling } from '@/api';
import { EMPTY_VALUE, formatCompactAmount } from '@/common/format/format';
import FactGrid, { FactCell } from '../../FactGrid';
import { CashFlowSectionProps } from './types';
import '@/styles/scanner-global.css';
import '../analysis-styles.css';
import './CashFlowSection-styles.css';

export const ANNUAL_ONLY_NOTE = 'Annual figures only — quarterly cash-flow figures are year-to-date.';

export const NO_CASH_FLOW_FALLBACK = 'No cash-flow statement available.';

/** Lines the API serves as positive amounts paid out. */
const PAYMENT_KEYS = new Set(['capex', 'buybacks', 'dividends']);

/** Annual report forms: a US 10-K, a foreign issuer's 20-F / 40-F (and their amendments). */
const ANNUAL_FORM = /^(10-K|20-F|40-F)/;

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

/** "fiscal year 2024, ended 31 Dec 2024"; "fiscal year ended …" without a year; "period ended …" for a non-annual form. */
const describePeriod = (cashFlow: CashFlowStatement): string | null => {
    const periodEnd = formatFilingDate(cashFlow.period_end);
    const isAnnual = cashFlow.form === null || ANNUAL_FORM.test(cashFlow.form);
    if (!isAnnual) return periodEnd && `period ended ${periodEnd}`;
    if (cashFlow.fiscal_year !== null) return `fiscal year ${cashFlow.fiscal_year}${periodEnd ? `, ended ${periodEnd}` : ''}`;
    return periodEnd && `fiscal year ended ${periodEnd}`;
};

/**
 * "20-F · fiscal year 2024, ended 31 Dec 2024 · filed 17 Apr 2025 · 20-F via SEC EDGAR";
 * the source label is shown as served, and parts the API left null are skipped.
 */
export const describeFiling = (cashFlow: CashFlowStatement): string => {
    const filed = formatFilingDate(cashFlow.filed);
    return [cashFlow.form, describePeriod(cashFlow), filed && `filed ${filed}`, cashFlow.source].filter(Boolean).join(' · ');
};

/** The statement's currency, named once and never converted; null when the API didn't say. */
export const describeCurrency = (currency: string | null): string | null => {
    if (!currency) return null;
    const code = currency.toUpperCase();
    return code === 'USD' ? 'Figures in USD, as reported' : `Figures in ${code}, as reported — not converted`;
};

/** "A newer 20-F (fiscal year ended 31 Dec 2025, filed 16 Apr 2026) is on SEC EDGAR but not in its companyfacts data yet." */
export const describeNewerFiling = (newer: NewerFiling): string => {
    const periodEnd = formatFilingDate(newer.period_end);
    const filed = formatFilingDate(newer.filed);
    const details = [periodEnd && `fiscal year ended ${periodEnd}`, filed && `filed ${filed}`].filter(Boolean).join(', ');
    return `A newer ${newer.form ?? 'annual report'}${details ? ` (${details})` : ''} is on SEC EDGAR but not in its companyfacts data yet.`;
};

/** Compact figures in the served currency (−$25.93B, TWD 1,826.18B); a line the filing lacks is "—" with the served reason, never zero. */
export const buildCashFlowCells = (lines: CashFlowLine[], currency: string | null = null): FactCell[] =>
    lines.map(line => {
        if (line.value === null) {
            return { key: line.key, label: line.label, value: EMPTY_VALUE, sub: line.missing_note ?? undefined };
        }
        return {
            key: line.key,
            label: line.label,
            value: formatCompactAmount(line.value, currency),
            sub: PAYMENT_KEYS.has(line.key) ? 'amount paid' : undefined,
        };
    });

/**
 * The latest annual cash-flow statement as filed (a 10-K via Finnhub, or a
 * 20-F via SEC EDGAR), next to the balance sheet, in the filing's own
 * currency. Plain figures: no red/green, which the design system keeps for
 * price moves. When there is no statement the card still renders, with the API's reason.
 */
const CashFlowSection = ({ cashFlow }: CashFlowSectionProps) => {
    const currency = describeCurrency(cashFlow.currency);
    return (
        <CollapsibleCard
            id="scanner-analysis-cash-flow"
            persistKey="scanner.detail.cash-flow"
            data-testid="analysis-cash-flow"
            title="Cash flow"
            meta={
                cashFlow.available ? (
                    <span className="scanner-muted scanner-analysis__meta cash-flow__meta">
                        <span data-testid="cash-flow-filing">{describeFiling(cashFlow)}</span>
                        {currency && <span data-testid="cash-flow-currency">{currency}</span>}
                    </span>
                ) : undefined
            }
        >
            {cashFlow.available ? (
                <>
                    <FactGrid cells={buildCashFlowCells(cashFlow.lines, cashFlow.currency)} testIdPrefix="cash-flow" />
                    {cashFlow.newer_filing && (
                        <p className="scanner-muted scanner-analysis__note" data-testid="cash-flow-newer-filing">
                            {describeNewerFiling(cashFlow.newer_filing)}
                        </p>
                    )}
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
};

export default CashFlowSection;
