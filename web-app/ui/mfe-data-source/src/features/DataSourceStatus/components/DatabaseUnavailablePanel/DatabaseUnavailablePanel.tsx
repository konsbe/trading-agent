import { AlertTriangleIcon } from '@trading-agent/shared-components';
import { DatabaseUnavailablePanelProps } from './types';
import './DatabaseUnavailablePanel-styles.css';

export const DATABASE_UNAVAILABLE_TEXT =
    "The status service can't reach the database — that is the operational problem this page reports.";

/**
 * The 503: on this page it is the finding, not a soft fetch failure, so it gets
 * a full-width panel in the error status language. Checking again is the
 * header's Refresh.
 */
const DatabaseUnavailablePanel = ({ error }: DatabaseUnavailablePanelProps) => (
    <section className="data-source-db-down" role="alert" aria-labelledby="data-source-db-down-title" data-testid="database-unavailable">
        <AlertTriangleIcon size={28} className="data-source-db-down__icon" />
        <div className="data-source-db-down__body">
            <h2 id="data-source-db-down-title" className="data-source-db-down__title">
                Database unreachable
            </h2>
            <p className="data-source-db-down__text">{DATABASE_UNAVAILABLE_TEXT}</p>
            <p className="data-source-db-down__code">
                {error.code}
                {error.status > 0 ? ` · HTTP ${error.status}` : ''} · providers and the daily chain can't be checked until it is back.
                Use Refresh to check again.
            </p>
        </div>
    </section>
);

export default DatabaseUnavailablePanel;
