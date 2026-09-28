import { useMemo } from 'react';
import { EMPTY_VALUE, SortState, TableColumn } from '@trading-agent/shared-components';
import { ComputedSymbol, FollowedSymbol } from '@/api';
import { computeColumn } from '@/common/compute/computeColumn';
import { formatDate } from '@/common/format/format';
import { ASSET_TYPE_LABELS, labelOf, LISTING_LABELS, SOURCE_LABELS } from '@/common/format/trackingLabels';
import { useComputeStatus } from '@/providers/ComputeStatusContext';

/** Newest followed first (ties — e.g. every `.env` seed — by symbol). */
export const FOLLOWED_DEFAULT_SORT: SortState = { key: 'added_at', direction: 'desc' };

export const FOLLOWED_URL_KEY = 'followed';

export const FOLLOWED_DATE_KEYS = ['added_at'];

const timeValue = (iso: string | null): number | null => {
    const time = iso ? Date.parse(iso) : NaN;
    return Number.isNaN(time) ? null : time;
};

/** Every column but Unfollow (an action), sorted and searched by what the cell shows. */
export const buildFollowedColumns = (getItem: (symbol: string) => ComputedSymbol | undefined): TableColumn<FollowedSymbol>[] => [
    { key: 'symbol', label: 'Symbol', initialDirection: 'asc' },
    { key: 'name', label: 'Name', sortValue: item => item.name, searchText: item => item.name ?? EMPTY_VALUE, initialDirection: 'asc' },
    { key: 'type', label: 'Type', sortValue: item => labelOf(ASSET_TYPE_LABELS, item.asset_type) },
    { key: 'listing', label: 'Listing', sortValue: item => labelOf(LISTING_LABELS, item.listing) },
    { key: 'source', label: 'Source', sortValue: item => labelOf(SOURCE_LABELS, item.source) },
    {
        key: 'added_at',
        label: 'Added',
        sortValue: item => timeValue(item.added_at),
        searchText: item => formatDate(item.added_at),
        initialDirection: 'desc',
    },
    computeColumn<FollowedSymbol>(getItem),
];

export const useFollowedColumns = (): TableColumn<FollowedSymbol>[] => {
    const { getItem } = useComputeStatus();
    return useMemo(() => buildFollowedColumns(getItem), [getItem]);
};
