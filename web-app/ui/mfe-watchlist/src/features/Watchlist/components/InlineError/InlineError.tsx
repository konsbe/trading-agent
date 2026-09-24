import { Button, CloseIcon } from '@trading-agent/shared-components';
import { InlineErrorProps } from './types';
import './InlineError-styles.css';

/** A failed save, shown next to what it affected; dismissible. */
const InlineError = ({ children, onDismiss, 'data-testid': testId }: InlineErrorProps) => (
    <div className="watchlist-inline-error" role="alert" data-testid={testId}>
        <span>{children}</span>
        {onDismiss && (
            <Button variant="ghost" size="sm" iconOnly aria-label="Dismiss" onClick={onDismiss}>
                <CloseIcon size={14} />
            </Button>
        )}
    </div>
);

export default InlineError;
