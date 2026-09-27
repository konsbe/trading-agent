import { Button, CloseIcon, EMPTY_VALUE } from '@trading-agent/shared-components';
import { ComputedSymbol } from '@/api';
import { getTrackingErrorMessage } from '@/common/errors/errorMessages';
import { formatDateTime } from '@/common/format/format';
import { ASSET_TYPE_LABELS, labelOf, REASON_DESCRIPTIONS, REASON_LABELS } from '@/common/format/trackingLabels';
import ComputeState from '@/components/ComputeState';
import Tag from '@/components/Tag';
import { ComputedTableProps } from './types';
import '@/styles/tracking-table.css';
import './ComputedTable-styles.css';

const isManual = (item: ComputedSymbol) => item.reasons.includes('manual');

/** Only the manual reason is closed; say what happens to the symbol afterwards. */
const stopHint = (item: ComputedSymbol): string =>
    item.reasons.some(reason => reason !== 'manual')
        ? 'Other reasons keep it computed'
        : 'Leaves the list; computed history is kept';

/** Every symbol with an open reason: why, where its computation stands, and Stop computing for manual requests. */
const ComputedTable = ({ items, dataTimeoutMinutes, requesting, errors, onStop }: ComputedTableProps) => (
    <div className="tracking-table__wrap" role="region" aria-label="Computed symbols, scrolls horizontally" tabIndex={0}>
        <table className="tracking-table computed-table" data-testid="computed-table">
            <caption className="tracking-table__caption">Computed symbols</caption>
            <thead>
                <tr>
                    <th scope="col" data-column="symbol">Symbol</th>
                    <th scope="col" data-column="name">Name</th>
                    <th scope="col" data-column="type">Type</th>
                    <th scope="col" data-column="reasons">Reasons</th>
                    <th scope="col" data-column="state">State</th>
                    <th scope="col" data-column="computed">Last computed</th>
                    <th scope="col" data-column="actions">
                        <span className="tracking-table__sr-only">Actions</span>
                    </th>
                </tr>
            </thead>
            <tbody>
                {items.map(item => {
                    const busy = requesting.has(item.symbol);
                    const error = errors.get(item.symbol);
                    return (
                        <tr key={item.symbol} className="tracking-table__row" data-testid={`computed-row-${item.symbol}`}>
                            <th scope="row" data-column="symbol">
                                <span className="tracking-table__ticker">{item.symbol}</span>
                            </th>
                            <td data-column="name">
                                <span className="tracking-table__name" title={item.name ?? undefined}>
                                    {item.name ?? EMPTY_VALUE}
                                </span>
                            </td>
                            <td data-column="type">{labelOf(ASSET_TYPE_LABELS, item.asset_type)}</td>
                            <td data-column="reasons">
                                <span className="tracking-table__chips">
                                    {item.reasons.map(reason => (
                                        <Tag
                                            key={reason}
                                            tone={reason === 'manual' ? 'accent' : 'neutral'}
                                            title={labelOf(REASON_DESCRIPTIONS, reason)}
                                            data-testid={`reason-${item.symbol}-${reason}`}
                                        >
                                            {labelOf(REASON_LABELS, reason)}
                                        </Tag>
                                    ))}
                                </span>
                            </td>
                            <td data-column="state">
                                <ComputeState item={item} dataTimeoutMinutes={dataTimeoutMinutes} variant="table" />
                            </td>
                            <td data-column="computed" className="tracking-table__muted">
                                {item.computed_at ? (
                                    <time dateTime={item.computed_at}>{formatDateTime(item.computed_at)}</time>
                                ) : (
                                    'not yet'
                                )}
                            </td>
                            <td data-column="actions">
                                {isManual(item) && (
                                    <span className="computed-table__stop">
                                        <Button
                                            variant="ghost"
                                            size="sm"
                                            className="tracking-table__action"
                                            aria-label={`Stop computing ${item.symbol} (closes only your manual request)`}
                                            aria-busy={busy}
                                            disabled={busy}
                                            title="Closes only your manual Compute request. Other reasons keep it computed; computed history is never deleted."
                                            onClick={() => onStop(item.symbol)}
                                            data-testid={`stop-${item.symbol}`}
                                        >
                                            <CloseIcon size={14} />
                                            {busy ? 'Stopping…' : 'Stop computing'}
                                        </Button>
                                        <span className="computed-table__hint">{stopHint(item)}</span>
                                    </span>
                                )}
                                {error && (
                                    <span className="computed-table__error" role="alert" data-testid={`stop-error-${item.symbol}`}>
                                        {getTrackingErrorMessage(error)}
                                    </span>
                                )}
                            </td>
                        </tr>
                    );
                })}
            </tbody>
        </table>
    </div>
);

export default ComputedTable;
