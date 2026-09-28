import { useMemo } from 'react';
import {
    columnSearchText,
    columnSortValue,
    EMPTY_VALUE,
    MARKET_COLUMNS,
    MarketRow,
    SortState,
    TableColumn,
} from '@trading-agent/shared-components';
import { ComputedSymbol, WatchlistItem } from '@/api';
import { computeColumn } from '@/common/compute/computeColumn';
import { DAILY_BARS_LABEL, isDailyBarsRow, marketCapNoteLabel } from '@/common/format/dataSource';
import { formatDate } from '@/common/format/format';
import { useComputeStatus } from '@/providers/ComputeStatusContext';

export interface WatchlistColumn extends TableColumn<WatchlistItem> {
    numeric: boolean;
    /** Header tooltip. */
    description?: string;
}

/** Newest added first, as the API serves the list. */
export const WATCHLIST_DEFAULT_SORT: SortState = { key: 'added_at', direction: 'desc' };

export const WATCHLIST_URL_KEY = 'watchlist';

export const WATCHLIST_DATE_KEYS = ['added_at'];

const NO_MARKET_DATA: Partial<MarketRow> = {
    close: null,
    change_pct: null,
    rvol_20: null,
    dollar_volume: null,
    rsi_14: null,
    breakout_state: null,
    pct_of_52w_high: null,
    market_cap: null,
    market_cap_est: null,
    momentum_score_100: null,
    score_attainable: null,
};

/** Without a features row (`as_of` null) no market value is shown, even if one is present. */
export const marketValues = (item: WatchlistItem): MarketRow => (item.as_of === null ? { ...item, ...NO_MARKET_DATA } : item);

export const hasMarketCapNote = (item: WatchlistItem) =>
    isDailyBarsRow(item) && item.market_cap === null && item.market_cap_est === null && Boolean(item.market_cap_note);

const marketColumnSearchText = (column: (typeof MARKET_COLUMNS)[number], item: WatchlistItem): string => {
    if (column.key === 'market_cap' && hasMarketCapNote(item)) return `${EMPTY_VALUE} ${marketCapNoteLabel(item.market_cap_note ?? '')}`;
    return columnSearchText(column, marketValues(item)) ?? '';
};

/**
 * Symbol, the shared market columns (sorted and searched on the values the
 * row shows: none without a features row), Added, then Compute (by state).
 * Remove is an action column and not listed.
 */
export const buildWatchlistColumns = (getItem: (symbol: string) => ComputedSymbol | undefined): WatchlistColumn[] => [
    {
        key: 'symbol',
        label: 'Symbol',
        numeric: false,
        sortValue: item => item.symbol,
        searchText: item =>
            [item.symbol, item.company_name ?? EMPTY_VALUE, item.exchange ?? EMPTY_VALUE, isDailyBarsRow(item) ? DAILY_BARS_LABEL : '']
                .join(' ')
                .trim(),
        initialDirection: 'asc',
    },
    ...MARKET_COLUMNS.map<WatchlistColumn>(column => ({
        key: column.key,
        label: column.label,
        numeric: column.numeric,
        description: column.description,
        initialDirection: column.initialDirection,
        sortValue: item => columnSortValue(column, marketValues(item)),
        searchText: item => marketColumnSearchText(column, item),
    })),
    {
        key: 'added_at',
        label: 'Added',
        numeric: false,
        description: 'When the symbol was added to the watchlist',
        sortValue: item => {
            const time = Date.parse(item.added_at);
            return Number.isNaN(time) ? null : time;
        },
        searchText: item => formatDate(item.added_at),
        initialDirection: 'desc',
    },
    {
        ...computeColumn<WatchlistItem>(getItem),
        numeric: false,
        description: 'Ask the pipeline to fetch and compute this symbol now. Sorts by computation state: needs attention first.',
    },
];

export const useWatchlistColumns = (): WatchlistColumn[] => {
    const { getItem } = useComputeStatus();
    return useMemo(() => buildWatchlistColumns(getItem), [getItem]);
};
