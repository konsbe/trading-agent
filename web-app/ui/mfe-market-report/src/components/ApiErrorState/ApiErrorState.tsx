import { Button } from '@trading-agent/shared-components';
import { getErrorMessage } from '@/common/errors/errorMessages';
import { ApiErrorStateProps } from './types';
import './ApiErrorState-styles.css';

/** Failure state for real errors (outage, internal error, network) with an optional Retry. */
const ApiErrorState = ({ error, message, onRetry }: ApiErrorStateProps) => (
    <div className="market-report-error-state" role="alert" data-testid="api-error-state" data-error-code={error.code}>
        <p className="market-report-error-state__message">{message ?? getErrorMessage(error)}</p>
        <p className="market-report-error-state__code">
            {error.code}
            {error.status > 0 ? ` · HTTP ${error.status}` : ''}
        </p>
        {onRetry && (
            <Button variant="secondary" size="sm" onClick={onRetry}>
                Retry
            </Button>
        )}
    </div>
);

export default ApiErrorState;
