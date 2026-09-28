import { useCallback, useMemo, useRef } from 'react';
import { useSearchParams } from 'react-router-dom';
import {
    filterRows,
    initialSortDirection,
    isSortable,
    parseSort,
    queryParamName,
    serializeSort,
    sortParamName,
    sortRows,
} from './tableView';
import { SortState, TableHeaderProps, TableView, UseTableViewOptions } from './types';

/**
 * Client-side sort + search for one table, kept in the query string
 * (`${urlKey}_sort=<key>:<dir>`, `${urlKey}_q=<text>`) so reload, back and a
 * "Back to …" link restore the same view. Writes replace the current history
 * entry and leave every other param alone; the default sort is never written.
 */
const useTableView = <Row>({ rows, columns, defaultSort, urlKey, tieBreak }: UseTableViewOptions<Row>): TableView<Row> => {
    const [searchParams, setSearchParams] = useSearchParams();
    // Two writes in one tick (e.g. sort then query) must build on each other, not on the same render's params.
    const latestParams = useRef(searchParams);
    latestParams.current = searchParams;

    const sortParam = searchParams.get(sortParamName(urlKey));
    const sort = useMemo<SortState>(() => parseSort(sortParam, columns) ?? defaultSort, [sortParam, columns, defaultSort]);
    const query = searchParams.get(queryParamName(urlKey)) ?? '';

    const writeParam = useCallback(
        (name: string, value: string | null) => {
            const current = latestParams.current;
            if ((current.get(name) ?? null) === value) return;
            const next = new URLSearchParams(current);
            if (value === null) next.delete(name);
            else next.set(name, value);
            latestParams.current = next;
            setSearchParams(next, { replace: true, preventScrollReset: true });
        },
        [setSearchParams]
    );

    const toggleSort = useCallback(
        (key: string) => {
            const column = columns.find(c => c.key === key);
            if (!column || !isSortable(column)) return;
            const next: SortState =
                sort.key === key
                    ? { key, direction: sort.direction === 'asc' ? 'desc' : 'asc' }
                    : { key, direction: initialSortDirection(column, rows) };
            const isDefault = next.key === defaultSort.key && next.direction === defaultSort.direction;
            writeParam(sortParamName(urlKey), isDefault ? null : serializeSort(next));
        },
        [columns, rows, sort, defaultSort, urlKey, writeParam]
    );

    const setQuery = useCallback(
        (next: string) => writeParam(queryParamName(urlKey), next.trim() ? next : null),
        [urlKey, writeParam]
    );

    const visible = useMemo(
        () => sortRows(filterRows(rows, columns, query), columns, sort, tieBreak),
        [rows, columns, query, sort, tieBreak]
    );

    const headerProps = useCallback(
        (key: string): TableHeaderProps => {
            const column = columns.find(c => c.key === key);
            return {
                label: column?.label ?? key,
                sortable: column ? isSortable(column) : false,
                active: sort.key === key,
                direction: sort.direction,
                onSort: () => toggleSort(key),
                'data-column': key,
            };
        },
        [columns, sort, toggleSort]
    );

    return {
        rows: visible,
        sort,
        toggleSort,
        query,
        setQuery,
        headerProps,
        total: rows.length,
        shown: visible.length,
    };
};

export default useTableView;
