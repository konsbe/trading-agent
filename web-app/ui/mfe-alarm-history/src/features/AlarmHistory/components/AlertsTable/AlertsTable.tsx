import { SeverityBadge, SortableHeader } from '@trading-agent/shared-components';
import { formatBarDate, formatDateTime, formatFiredAt } from '@/common/format/format';
import { useTypeLabel } from '../../providers/TypeLabelsContext';
import { RAW_COLUMNS } from '../../utils/alertColumns';
import AlertSymbol from '../AlertSymbol';
import { AlertsTableProps } from './types';
import '@/styles/alarm-table.css';

/**
 * Individual alerts in the order served: fired time (local, with the onset
 * bar's date below it when the alert has one), symbol, alert type, severity
 * and the bot's message. With `headerProps` the headers sort (server-side);
 * a group's expansion has none (newest first) and omits the type column (it
 * is the group's). `fit`: the page's own list, whose rows scroll under the
 * sticky header while the page stays put.
 */
const AlertsTable = ({ id, caption, alerts, showType = true, headerProps, fit = false }: AlertsTableProps) => {
    const typeLabel = useTypeLabel();
    const columns = RAW_COLUMNS.filter(column => showType || column.key !== 'alert_type');
    return (
        <div
            className={`alarm-table__wrap${fit ? ' ta-fit-scroll' : ''}`}
            role="region"
            aria-label={`${caption}, ${fit ? 'scrolls' : 'scrolls horizontally'}`}
            tabIndex={0}
        >
            <table className="alarm-table" id={id} data-testid={id}>
                <caption className="alarm-table__caption">{caption}</caption>
                <thead>
                    <tr>
                        {columns.map(column =>
                            headerProps ? (
                                <SortableHeader key={column.key} {...headerProps(column.key)} />
                            ) : (
                                <th key={column.key} scope="col" data-column={column.key}>
                                    {column.label}
                                </th>
                            )
                        )}
                    </tr>
                </thead>
                <tbody>
                    {alerts.map(alert => (
                        <tr key={alert.id} className="alarm-table__row" data-testid={`alert-row-${alert.id}`}>
                            <td data-column="fired">
                                <time className="alarm-table__time" dateTime={alert.fired_at} title={formatDateTime(alert.fired_at)}>
                                    {formatFiredAt(alert.fired_at)}
                                </time>
                                {alert.bar_date && (
                                    <span
                                        className="alarm-table__bar"
                                        title="The bar the condition started on"
                                        data-testid={`alert-bar-${alert.id}`}
                                    >
                                        bar <time dateTime={alert.bar_date}>{formatBarDate(alert.bar_date)}</time>
                                    </span>
                                )}
                            </td>
                            <td data-column="symbol">
                                <AlertSymbol row={alert} />
                            </td>
                            {showType && <td data-column="type">{typeLabel(alert.alert_type)}</td>}
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
};

export default AlertsTable;
