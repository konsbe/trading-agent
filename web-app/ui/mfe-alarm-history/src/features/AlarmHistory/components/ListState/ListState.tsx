import { Skeleton } from '@trading-agent/shared-components';
import { getFilterErrorMessage, isFilterError } from '@/common/errors/errorMessages';
import ApiErrorState from '@/components/ApiErrorState';
import { ListStateProps } from './types';
import './ListState-styles.css';

export const EMPTY_TEXT = 'No alerts in this period';

/**
 * Loading skeleton, failure, rejected filters (HTTP 400, in plain words) or the
 * neutral empty state; otherwise the list itself.
 */
const ListState = ({ isLoading, error, isEmpty, onRetry, emptyDetail, skeletonRows = 6, children, 'data-testid': testId }: ListStateProps) => {
    if (isLoading) {
        return (
            <div className="alarm-list-state__skeleton" aria-busy="true" aria-label="Loading alerts" data-testid={`${testId}-loading`}>
                {Array.from({ length: skeletonRows }, (_, i) => (
                    <Skeleton key={i} height="2.25rem" radius="var(--radius-md)" />
                ))}
            </div>
        );
    }
    if (error && isFilterError(error)) {
        return (
            <p className="alarm-list-state__filter-error" role="alert" data-testid={`${testId}-filter-error`} data-error-code={error.code}>
                {getFilterErrorMessage(error)}
            </p>
        );
    }
    if (error) return <ApiErrorState error={error} onRetry={onRetry} />;
    if (isEmpty) {
        return (
            <div className="alarm-list-state__empty" data-testid={`${testId}-empty`}>
                <p className="alarm-list-state__empty-title">{EMPTY_TEXT}</p>
                {emptyDetail && <p className="alarm-list-state__empty-detail">{emptyDetail}</p>}
            </div>
        );
    }
    return <>{children}</>;
};

export default ListState;
