import { AlertGroup, FiredAlert } from '@/api';

/** How a list's items are identified and ordered (newest first, ties by id). */
export interface ListShape<T> {
    keyOf: (item: T) => string;
    /** The alert the item is ordered by: the row itself, or a group's latest alert. */
    orderOf: (item: T) => { fired_at: string; id: number };
}

export const RAW_SHAPE: ListShape<FiredAlert> = {
    keyOf: alert => String(alert.id),
    orderOf: alert => alert,
};

export const GROUP_SHAPE: ListShape<AlertGroup> = {
    keyOf: group => groupKey(group),
    orderOf: group => group.latest,
};

export const groupKey = (group: { symbol: string; alert_type: string }): string => `${group.symbol}|${group.alert_type}`;

/** Positive when `a` is older than `b` (newest-first order). */
const compareOrder = (a: { fired_at: string; id: number }, b: { fired_at: string; id: number }): number => {
    const diff = Date.parse(b.fired_at) - Date.parse(a.fired_at);
    return diff !== 0 ? diff : b.id - a.id;
};

export interface Page<T> {
    items: T[];
    hasMore: boolean;
    nextBefore: number | null;
}

/** "Load older": the next page appended; anything already listed keeps its place. */
export const appendPage = <T>(current: Page<T>, older: Page<T>, shape: ListShape<T>): Page<T> => {
    const seen = new Set(current.items.map(shape.keyOf));
    return {
        items: [...current.items, ...older.items.filter(item => !seen.has(shape.keyOf(item)))],
        hasMore: older.hasMore,
        nextBefore: older.nextBefore,
    };
};

/**
 * A refreshed first page folded into what is listed, so rows loaded with
 * "Load older" stay put:
 * - the whole result fits in the fresh page → it replaces the list;
 * - the fresh page doesn't reach the newest listed item (more new alerts than
 *   a page since the last check) → it replaces the list, pagination restarts;
 * - otherwise the fresh page replaces the head (new rows, updated groups) and
 *   the listed items older than its last one are kept, with their cursor.
 */
export const mergeFirstPage = <T>(current: Page<T>, fresh: Page<T>, shape: ListShape<T>): Page<T> => {
    if (!fresh.hasMore || current.items.length === 0 || fresh.items.length === 0) return fresh;
    const oldestFresh = shape.orderOf(fresh.items[fresh.items.length - 1]);
    const newestCurrent = shape.orderOf(current.items[0]);
    if (compareOrder(oldestFresh, newestCurrent) < 0) return fresh;

    const freshKeys = new Set(fresh.items.map(shape.keyOf));
    const tail = current.items.filter(
        item => !freshKeys.has(shape.keyOf(item)) && compareOrder(shape.orderOf(item), oldestFresh) > 0
    );
    if (tail.length === 0) return fresh;
    return { items: [...fresh.items, ...tail], hasMore: current.hasMore, nextBefore: current.nextBefore };
};
