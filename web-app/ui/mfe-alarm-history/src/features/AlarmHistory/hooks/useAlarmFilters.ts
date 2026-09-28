import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { AlertsQuery } from '@/api';
import { DayRange, defaultRange, rangeBounds } from '@/common/dates/localDays';
import useParamWriter from '@/common/url/useParamWriter';

export const SYMBOL_DEBOUNCE_MS = 400;

/** Query-string names of the filters (only set when not the default). */
export const FILTER_PARAMS = {
    symbol: 'symbol',
    alertTypes: 'type',
    severities: 'severity',
    from: 'from',
    to: 'to',
} as const;

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
    /** The filters as an API query (no mode / paging / sort). */
    query: AlertsQuery;
}

const toggle = (list: string[], value: string) =>
    list.includes(value) ? list.filter(v => v !== value) : [...list, value];

const normalizeSymbol = (value: string) => value.trim().toUpperCase();

const readList = (raw: string | null): string[] =>
    raw
        ? raw
              .split(',')
              .map(v => v.trim())
              .filter(Boolean)
        : [];

const writeList = (list: string[]): string | null => (list.length > 0 ? [...list].sort().join(',') : null);

/**
 * Filter state for the page, kept in the query string (`symbol`, `type`,
 * `severity`, `from`, `to`; defaults omitted) so reload, back and "Back to
 * Alarm History" restore it. Any change yields a new `query` (and so restarts
 * paging). The symbol box applies after a pause or on Enter.
 */
const useAlarmFilters = (now: () => Date = () => new Date()): AlarmFilters => {
    const [params, write] = useParamWriter();
    const nowRef = useRef(now);
    nowRef.current = now;
    /** The default range the URL omits; fixed at mount, renewed by "Last 7 days". */
    const [today, setToday] = useState(() => defaultRange(now()));

    const symbol = normalizeSymbol(params.get(FILTER_PARAMS.symbol) ?? '');
    const typesParam = params.get(FILTER_PARAMS.alertTypes);
    const severitiesParam = params.get(FILTER_PARAMS.severities);
    const alertTypes = useMemo(() => readList(typesParam), [typesParam]);
    const severities = useMemo(() => readList(severitiesParam), [severitiesParam]);
    const fromParam = params.get(FILTER_PARAMS.from);
    const toParam = params.get(FILTER_PARAMS.to);
    const range = useMemo<DayRange>(
        () => ({ from: fromParam ?? today.from, to: toParam ?? today.to }),
        [fromParam, toParam, today]
    );

    const [symbolInput, setSymbolInput] = useState(symbol);
    // Follow an outside change of the applied symbol (back / forward) without clobbering typing.
    const lastApplied = useRef(symbol);
    useEffect(() => {
        if (symbol !== lastApplied.current) {
            lastApplied.current = symbol;
            setSymbolInput(symbol);
        }
    }, [symbol]);

    const applyNow = useCallback(
        (value: string) => {
            const next = normalizeSymbol(value);
            lastApplied.current = next;
            write({ [FILTER_PARAMS.symbol]: next || null });
        },
        [write]
    );

    useEffect(() => {
        const next = normalizeSymbol(symbolInput);
        if (next === symbol) return undefined;
        const timer = setTimeout(() => applyNow(next), SYMBOL_DEBOUNCE_MS);
        return () => clearTimeout(timer);
    }, [symbolInput, symbol, applyNow]);

    const applySymbol = useCallback(() => applyNow(symbolInput), [applyNow, symbolInput]);

    const toggleAlertType = useCallback(
        (type: string) => write({ [FILTER_PARAMS.alertTypes]: writeList(toggle(alertTypes, type)) }),
        [alertTypes, write]
    );
    const toggleSeverity = useCallback(
        (severity: string) => write({ [FILTER_PARAMS.severities]: writeList(toggle(severities, severity)) }),
        [severities, write]
    );

    const writeRange = useCallback(
        (next: DayRange) => {
            const isDefault = next.from === today.from && next.to === today.to;
            write({ [FILTER_PARAMS.from]: isDefault ? null : next.from, [FILTER_PARAMS.to]: isDefault ? null : next.to });
        },
        [today, write]
    );
    const setFrom = useCallback((from: string) => writeRange({ ...range, from }), [range, writeRange]);
    const setTo = useCallback((to: string) => writeRange({ ...range, to }), [range, writeRange]);
    const resetRange = useCallback(() => {
        setToday(defaultRange(nowRef.current()));
        write({ [FILTER_PARAMS.from]: null, [FILTER_PARAMS.to]: null });
    }, [write]);

    const query = useMemo<AlertsQuery>(
        () => ({
            ...(symbol ? { symbol } : {}),
            ...(alertTypes.length > 0 ? { alertTypes: [...alertTypes].sort() } : {}),
            ...(severities.length > 0 ? { severities: [...severities].sort() } : {}),
            ...rangeBounds(range),
        }),
        [symbol, alertTypes, severities, range]
    );

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
