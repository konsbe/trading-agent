import { Fragment } from 'react';
import { ChevronDownIcon, SeverityBadge } from '@trading-agent/shared-components';
import { alertTypeLabel } from '@/common/format/alertTypes';
import { formatDateTime, formatFiredAt, formatRepeats } from '@/common/format/format';
import { groupKey } from '../../utils/paging';
import AlertSymbol from '../AlertSymbol';
import GroupAlerts from '../GroupAlerts';
import { GroupedAlertsTableProps } from './types';
import '@/styles/alarm-table.css';
import './GroupedAlertsTable-styles.css';

const COLUMNS = 7;

const domId = (key: string) => `alarm-group-${key.replace(/[^A-Za-z0-9_-]/g, '-')}`;

/**
 * One row per symbol + alert type in the range, newest (by last fired) first:
 * last fired, symbol, type, the latest alert's severity and message, and the
 * repeat count with its first–last span. A group with repeats expands to its
 * individual alerts below the row.
 */
const GroupedAlertsTable = ({ groups, expanded, onToggle, query, refreshToken }: GroupedAlertsTableProps) => (
    <div className="alarm-table__wrap" role="region" aria-label="Alerts by symbol and type, scrolls horizontally" tabIndex={0}>
        <table className="alarm-table alarm-grouped" id="alarm-grouped-table" data-testid="alarm-grouped-table">
            <caption className="alarm-table__caption">Alerts grouped by symbol and alert type, newest first</caption>
            <thead>
                <tr>
                    <th scope="col" data-column="expand">
                        <span className="alarm-table__sr-only">Show alerts</span>
                    </th>
                    <th scope="col" data-column="fired">Last fired</th>
                    <th scope="col" data-column="symbol">Symbol</th>
                    <th scope="col" data-column="type">Alert type</th>
                    <th scope="col" data-column="severity">Severity</th>
                    <th scope="col" data-column="message">Latest message</th>
                    <th scope="col" data-column="repeats">Repeats</th>
                </tr>
            </thead>
            <tbody>
                {groups.map(group => {
                    const key = groupKey(group);
                    const id = domId(key);
                    const canExpand = group.count > 1;
                    const isOpen = canExpand && expanded.has(key);
                    const label = `${group.symbol} ${alertTypeLabel(group.alert_type)}`;
                    return (
                        <Fragment key={key}>
                            <tr
                                className={`alarm-table__row alarm-grouped__row${isOpen ? ' is-open' : ''}`}
                                data-testid={`group-row-${key}`}
                            >
                                <td data-column="expand">
                                    {canExpand && (
                                        <button
                                            type="button"
                                            className="alarm-grouped__toggle"
                                            aria-expanded={isOpen}
                                            aria-controls={`${id}-alerts`}
                                            aria-label={`${isOpen ? 'Hide' : 'Show'} the ${group.count} ${label} alerts`}
                                            onClick={() => onToggle(key)}
                                            data-testid={`group-toggle-${key}`}
                                        >
                                            <ChevronDownIcon size={16} className="alarm-grouped__chevron" />
                                        </button>
                                    )}
                                </td>
                                <td data-column="fired">
                                    <time
                                        className="alarm-table__time"
                                        dateTime={group.last_fired_at}
                                        title={formatDateTime(group.last_fired_at)}
                                    >
                                        {formatFiredAt(group.last_fired_at)}
                                    </time>
                                </td>
                                <td data-column="symbol">
                                    <AlertSymbol row={group} />
                                </td>
                                <td data-column="type">{alertTypeLabel(group.alert_type)}</td>
                                <td data-column="severity">
                                    <SeverityBadge severity={group.latest.severity} />
                                </td>
                                <td data-column="message">{group.latest.message}</td>
                                <td data-column="repeats">
                                    <span
                                        className="alarm-grouped__repeats"
                                        title={`First ${formatDateTime(group.first_fired_at)} · last ${formatDateTime(group.last_fired_at)}`}
                                        data-testid={`group-repeats-${key}`}
                                    >
                                        {formatRepeats(group.count, group.first_fired_at, group.last_fired_at)}
                                    </span>
                                </td>
                            </tr>
                            {isOpen && (
                                <tr className="alarm-grouped__expansion" id={`${id}-alerts`}>
                                    <td colSpan={COLUMNS}>
                                        <GroupAlerts group={group} query={query} refreshToken={refreshToken} />
                                    </td>
                                </tr>
                            )}
                        </Fragment>
                    );
                })}
            </tbody>
        </table>
    </div>
);

export default GroupedAlertsTable;
