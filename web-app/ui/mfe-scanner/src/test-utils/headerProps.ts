import { SortState, TableHeaderProps } from '@trading-agent/shared-components';
import { columnLabel } from '@/features/Candidates/components/CandidatesTable';
import { DEFAULT_SORT } from '@/features/Candidates/constants';

/** `CandidatesTable`'s `headerProps` for a fixed sort, as `useTableView` would give it. */
export const staticHeaderProps =
    (sort: SortState = DEFAULT_SORT, onSort: (key: string) => void = () => undefined) =>
    (key: string): TableHeaderProps => ({
        label: columnLabel(key),
        sortable: true,
        active: sort.key === key,
        direction: sort.direction,
        onSort: () => onSort(key),
        'data-column': key,
    });
