import { Button, Spinner } from '@trading-agent/shared-components';
import { getErrorMessage } from '@/common/errors/errorMessages';
import { ListFooterProps } from './types';
import './ListFooter-styles.css';

/**
 * How much is listed, and the next page while the API has more: "Load older"
 * for the newest-first list, "Load more" for a sorted or searched one.
 */
const ListFooter = ({ count, noun, hasMore, isLoadingOlder, olderError, onLoadOlder, sorted = false, 'data-testid': testId }: ListFooterProps) => {
    const action = sorted ? 'Load more' : 'Load older';
    const status = hasMore
        ? sorted
            ? `Showing the first ${count} ${noun}`
            : `Showing the newest ${count} ${noun}`
        : `Showing all ${count} ${noun}`;
    return (
        <div className="alarm-list-footer" data-testid={testId}>
            <span className="alarm-list-footer__count">{status}</span>
            {hasMore && (
                <Button variant="secondary" size="sm" onClick={onLoadOlder} disabled={isLoadingOlder} data-testid={`${testId}-load-older`}>
                    {isLoadingOlder ? <Spinner size="sm" /> : null}
                    {isLoadingOlder ? (sorted ? 'Loading more…' : 'Loading older…') : action}
                </Button>
            )}
            {olderError && (
                <span className="alarm-list-footer__error" role="alert" data-testid={`${testId}-older-error`}>
                    {getErrorMessage(olderError)} Try {action} again.
                </span>
            )}
        </div>
    );
};

export default ListFooter;
