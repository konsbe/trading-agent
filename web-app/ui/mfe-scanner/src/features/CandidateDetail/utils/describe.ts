import { GateCheck, PenaltyRule, SubScoreKey, SymbolFacts } from '@/api';
import {
    EMPTY_VALUE,
    formatBreakoutState,
    formatMultiple,
    formatNumber,
    formatPlain,
    formatPrice,
    formatSignedPercent,
    formatSignedUsd,
    formatCompact,
    formatUsdShort,
} from '@/common/format/format';
import { humanizeCode } from '@/common/format/humanize';
import { MINUS } from '@/common/format/sign';

const isNum = (value: number | null | undefined): value is number => typeof value === 'number' && Number.isFinite(value);

export const capitalize = (text: string) => text.charAt(0).toUpperCase() + text.slice(1);

/** U+2212, used where a value is shown as a deduction ("−10 pts", "−12.5% from peak"). */
export { MINUS };

// ---- Gates ----------------------------------------------------------------

interface GateFormat {
    name: string;
    /** Absent for bound-only gates. */
    value?: (n: number) => string;
    bound: (n: number) => string;
    range?: (min: number, max: number) => string;
}

const GATE_FORMATS: Record<string, GateFormat> = {
    price: { name: 'Price', value: formatPrice, bound: formatPrice },
    history: { name: 'History', bound: n => `${formatPlain(n)} bars` },
    change_pct: {
        name: 'Day change',
        value: formatSignedPercent,
        bound: n => `${formatPlain(n)}%`,
        range: (min, max) => `${formatPlain(min)}–${formatPlain(max)}%`,
    },
    rvol_20: { name: 'RVOL', value: formatMultiple, bound: n => `${formatNumber(n, 1)}×` },
    dollar_volume: { name: 'Dollar volume', value: formatUsdShort, bound: formatUsdShort },
    market_cap: { name: 'Market cap', value: formatUsdShort, bound: formatUsdShort },
};

/**
 * Gates that never carry a per-symbol value, only a bound. The scanner doesn't
 * store the bar count, so `history` always arrives with `value: null` and the
 * 252 minimum: bound-only by definition, not missing data, so it gets no "—".
 * "—" stays reserved for gates that do have a per-symbol value which is
 * missing this time (e.g. a null RVOL → "RVOL — ≥ 3.0×").
 */
export const BOUND_ONLY_GATES: ReadonlySet<string> = new Set(['history']);

/**
 * Value against its threshold in plain language: "RVOL 6.45× ≥ 3.0×",
 * "Day change +21.2% within 8–25%", "RVOL — ≥ 3.0×" (value missing),
 * "History ≥ 252 bars" (bound-only). A null threshold is unbounded and is left out.
 */
export const gateLine = (check: GateCheck): string => {
    const format = GATE_FORMATS[check.key] ?? { name: check.label, value: formatPlain, bound: formatPlain };
    const parts = [format.name];
    if (!BOUND_ONLY_GATES.has(check.key)) {
        const value = format.value ?? formatPlain;
        parts.push(isNum(check.value) ? value(check.value) + (check.value_is_proxy ? ' (est.)' : '') : EMPTY_VALUE);
    }
    if (isNum(check.min) && isNum(check.max)) {
        parts.push(`within ${format.range ? format.range(check.min, check.max) : `${format.bound(check.min)}–${format.bound(check.max)}`}`);
    } else if (isNum(check.min)) {
        parts.push(`≥ ${format.bound(check.min)}`);
    } else if (isNum(check.max)) {
        parts.push(`≤ ${format.bound(check.max)}`);
    }
    return parts.join(' ');
};

// ---- Catalyst -------------------------------------------------------------

/** `null` tier means never checked (data not ingested) — not the same as `"none"`. */
export const catalystText = (facts: SymbolFacts): string => {
    if (facts.catalyst_headline) return facts.catalyst_headline;
    if (facts.catalyst_tier === null) return 'Not checked — catalyst data not yet ingested';
    if (facts.catalyst_tier === 'none') return 'None found';
    return `Tier ${facts.catalyst_tier} catalyst`;
};

// ---- Facts ----------------------------------------------------------------

export type PriceDirection = 'up' | 'down' | 'flat';

export const priceDirection = (facts: SymbolFacts): PriceDirection | null => {
    const delta = isNum(facts.change_pct) ? facts.change_pct : facts.change_abs;
    if (!isNum(delta)) return null;
    return delta > 0 ? 'up' : delta < 0 ? 'down' : 'flat';
};

export const dayChangeText = (facts: SymbolFacts): string => {
    // Same precision as the close (formatPrice): a sub-dollar move would otherwise round to "$0.00".
    const digits = isNum(facts.close) && Math.abs(facts.close) < 1 ? 4 : 2;
    const abs = isNum(facts.change_abs) ? formatSignedUsd(facts.change_abs, digits) : null;
    const pct = isNum(facts.change_pct) ? formatSignedPercent(facts.change_pct) : null;
    if (abs && pct) return `${abs} (${pct})`;
    return abs ?? pct ?? EMPTY_VALUE;
};

/** "−12.5% from peak", or "New 52-week high" when the close is at/above it. */
export const fromPeakText = (ratio: number | null): string | null => {
    if (!isNum(ratio)) return null;
    if (ratio >= 1) return 'New 52-week high';
    return `${MINUS}${formatNumber((1 - ratio) * 100, 1)}% from peak`;
};

export const vwapDistanceText = (facts: SymbolFacts): string => {
    if (!isNum(facts.vwap_dist_pct)) return EMPTY_VALUE;
    const above = facts.above_vwap ?? facts.vwap_dist_pct >= 0;
    return `${formatNumber(Math.abs(facts.vwap_dist_pct), 1)}% ${above ? 'above' : 'below'}`;
};

// ---- Score ----------------------------------------------------------------

/** One-line explanation of each sub-score from the stored facts. */
export const subScoreExplanation = (key: SubScoreKey, facts: SymbolFacts): string => {
    switch (key) {
        case 'rvol':
            return isNum(facts.rvol_20) ? `${formatMultiple(facts.rvol_20)} relative volume vs 20-day average` : 'Relative volume not available';
        case 'vol_accel':
            return isNum(facts.vol_accel) ? `Volume acceleration ${formatMultiple(facts.vol_accel)}` : 'Volume acceleration not available';
        case 'catalyst':
            if (facts.catalyst_tier === null && !facts.catalyst_headline) return 'Not checked — no catalyst data';
            return catalystText(facts);
        case 'float':
            return isNum(facts.float_shares_est)
                ? `${facts.float_is_proxy ? 'Estimated float' : 'Float'} ${formatCompact(facts.float_shares_est, 1)} shares`
                : 'Float not available';
        case 'vwap':
            return isNum(facts.vwap_dist_pct) ? `Closed ${vwapDistanceText(facts)} 20-day VWAP` : 'VWAP distance not available';
        case 'breakout':
            return facts.breakout_state ? capitalize(formatBreakoutState(facts.breakout_state)) : 'Breakout state not available';
        case 'high52w':
            return isNum(facts.pct_of_52w_high)
                ? `${formatNumber(facts.pct_of_52w_high * 100, 1)}% of 52-week high`
                : '52-week high not available';
        default:
            return '';
    }
};

interface PenaltyDescription {
    label: string;
    value: string | null;
}

const PENALTIES: Record<string, (facts: SymbolFacts) => PenaltyDescription> = {
    exhausted_momentum_rsi_gt_85: facts => ({
        label: 'RSI above 85 (exhausted momentum)',
        value: isNum(facts.rsi_14) ? `RSI ${formatNumber(facts.rsi_14, 1)}` : null,
    }),
    already_extended_change_gt_20: facts => ({
        label: 'Day change above 20% (already extended)',
        value: isNum(facts.change_pct) ? formatSignedPercent(facts.change_pct) : null,
    }),
    volume_decaying_accel_lt_1_rvol_ge_3: facts => ({
        label: 'Volume decelerating (acceleration < 1 with RVOL ≥ 3)',
        value:
            isNum(facts.vol_accel) || isNum(facts.rvol_20)
                ? `Acceleration ${formatMultiple(facts.vol_accel)}, RVOL ${formatMultiple(facts.rvol_20)}`
                : null,
    }),
};

export const penaltyDescription = (rule: PenaltyRule, facts: SymbolFacts): PenaltyDescription =>
    PENALTIES[rule.code]?.(facts) ?? { label: humanizeCode(rule.code), value: null };

/** "−10 pts" when applied, "0 pts" otherwise. */
export const penaltyPoints = (rule: PenaltyRule): string =>
    rule.applied && rule.points !== 0 ? `${MINUS}${formatPlain(Math.abs(rule.points))} pts` : '0 pts';

export const penaltyTotalText = (total: number): string =>
    total === 0 ? '0 pts' : `${MINUS}${formatPlain(Math.abs(total))} pts`;
