import { SeverityBadge } from '@trading-agent/shared-components';
import { alertTypeLabel } from '@/common/format/alertTypes';
import { formatDateTime, formatFiredAt } from '@/common/format/format';
import AlertSymbol from '../AlertSymbol';
import { AlertsTableProps } from './types';
import '@/styles/alarm-table.css';

/**
 * Individual alerts, newest first: fired time (local), symbol, alert type,
 * severity and the bot's message. A group's expansion omits the type column
 * (it is the group's).
 */
const AlertsTable = ({ id, caption, alerts, showType = true }: AlertsTableProps) => (
    <div className="alarm-table__wrap" role="region" aria-label={`${caption}, scrolls horizontally`} tabIndex={0}>
        <table className="alarm-table" id={id} data-testid={id}>
            <caption className="alarm-table__caption">{caption}</caption>
            <thead>
                <tr>
                    <th scope="col" data-column="fired">Fired</th>
                    <th scope="col" data-column="symbol">Symbol</th>
                    {showType && <th scope="col" data-column="type">Alert type</th>}
                    <th scope="col" data-column="severity">Severity</th>
                    <th scope="col" data-column="message">Message</th>
                </tr>
            </thead>
            <tbody>
                {alerts.map(alert => (
                    <tr key={alert.id} className="alarm-table__row" data-testid={`alert-row-${alert.id}`}>
                        <td data-column="fired">
                            <time className="alarm-table__time" dateTime={alert.fired_at} title={formatDateTime(alert.fired_at)}>
                                {formatFiredAt(alert.fired_at)}
                            </time>
                        </td>
                        <td data-column="symbol">
                            <AlertSymbol row={alert} />
                        </td>
                        {showType && <td data-column="type">{alertTypeLabel(alert.alert_type)}</td>}
                        <td data-column="severity">
                            <SeverityBadge severity={alert.severity} />
                        </td>
                        <td data-column="message">{alert.message}</td>
                    </tr>
                ))}
            </tbody>
        </table>
    </div>
);

export default AlertsTable;
