import { Button, Spinner } from '@trading-agent/shared-components';
import { getErrorMessage } from '@/common/errors/errorMessages';
import { ListFooterProps } from './types';
import './ListFooter-styles.css';

/** How much is listed, and "Load older" while the API has more. */
const ListFooter = ({ count, noun, hasMore, isLoadingOlder, olderError, onLoadOlder, 'data-testid': testId }: ListFooterProps) => (
    <div className="alarm-list-footer" data-testid={testId}>
        <span className="alarm-list-footer__count">
            {hasMore ? `Showing the newest ${count} ${noun}` : `Showing all ${count} ${noun}`}
        </span>
        {hasMore && (
            <Button variant="secondary" size="sm" onClick={onLoadOlder} disabled={isLoadingOlder} data-testid={`${testId}-load-older`}>
                {isLoadingOlder ? <Spinner size="sm" /> : null}
                {isLoadingOlder ? 'Loading older…' : 'Load older'}
            </Button>
        )}
        {olderError && (
            <span className="alarm-list-footer__error" role="alert" data-testid={`${testId}-older-error`}>
                {getErrorMessage(olderError)} Try Load older again.
            </span>
        )}
    </div>
);

export default ListFooter;
