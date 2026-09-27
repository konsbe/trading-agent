import { useCallback, useEffect, useMemo, useState } from 'react';
import { AlertsQuery } from '@/api';
import { DayRange, defaultRange, rangeBounds } from '@/common/dates/localDays';

export const SYMBOL_DEBOUNCE_MS = 400;

export interface AlarmFilters {
    /** The symbol box as typed. */
    symbolInput: string;
    /** The symbol applied to the query (debounced, trimmed, upper-case). */
    symbol: string;
    alertTypes: string[];
    severities: string[];
    range: DayRange;
    /** True when the range is the default last-7-days range. */
    isDefaultRange: boolean;
    setSymbolInput: (value: string) => void;
    /** Applies the typed symbol now (Enter). */
    applySymbol: () => void;
    toggleAlertType: (type: string) => void;
    toggleSeverity: (severity: string) => void;
    setFrom: (day: string) => void;
    setTo: (day: string) => void;
    resetRange: () => void;
    /** The filters as an API query (no mode / paging). */
    query: AlertsQuery;
}

const toggle = (list: string[], value: string) =>
    list.includes(value) ? list.filter(v => v !== value) : [...list, value];

const normalizeSymbol = (value: string) => value.trim().toUpperCase();

/** Filter state for the page; any change yields a new `query` (and so restarts paging). */
const useAlarmFilters = (now: () => Date = () => new Date()): AlarmFilters => {
    const [initialRange] = useState(() => defaultRange(now()));
    const [symbolInput, setSymbolInput] = useState('');
    const [symbol, setSymbol] = useState('');
    const [alertTypes, setAlertTypes] = useState<string[]>([]);
    const [severities, setSeverities] = useState<string[]>([]);
    const [range, setRange] = useState<DayRange>(initialRange);

    useEffect(() => {
        const next = normalizeSymbol(symbolInput);
        if (next === symbol) return undefined;
        const timer = setTimeout(() => setSymbol(next), SYMBOL_DEBOUNCE_MS);
        return () => clearTimeout(timer);
    }, [symbolInput, symbol]);

    const applySymbol = useCallback(() => setSymbol(normalizeSymbol(symbolInput)), [symbolInput]);
    const toggleAlertType = useCallback((type: string) => setAlertTypes(list => toggle(list, type)), []);
    const toggleSeverity = useCallback((severity: string) => setSeverities(list => toggle(list, severity)), []);
    const setFrom = useCallback((from: string) => setRange(r => ({ ...r, from })), []);
    const setTo = useCallback((to: string) => setRange(r => ({ ...r, to })), []);
    const resetRange = useCallback(() => setRange(defaultRange(now())), [now]);

    const query = useMemo<AlertsQuery>(
        () => ({
            ...(symbol ? { symbol } : {}),
            ...(alertTypes.length > 0 ? { alertTypes: [...alertTypes].sort() } : {}),
            ...(severities.length > 0 ? { severities: [...severities].sort() } : {}),
            ...rangeBounds(range),
        }),
        [symbol, alertTypes, severities, range]
    );

    const today = defaultRange(now());
    const isDefaultRange = range.from === today.from && range.to === today.to;

    return {
        symbolInput,
        symbol,
        alertTypes,
        severities,
        range,
        isDefaultRange,
        setSymbolInput,
        applySymbol,
        toggleAlertType,
        toggleSeverity,
        setFrom,
        setTo,
        resetRange,
        query,
    };
};

export default useAlarmFilters;
