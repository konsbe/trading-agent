import { AlarmFilters } from '../../hooks/useAlarmFilters';

export interface AlarmFiltersProps {
    filters: AlarmFilters;
    /** Every alert type that has fired (the API's `types`). */
    typeOptions: readonly string[];
}
