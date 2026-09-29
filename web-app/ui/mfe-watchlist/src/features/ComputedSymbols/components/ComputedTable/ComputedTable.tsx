import { ReactNode } from 'react';
import { Button, CloseIcon, EMPTY_VALUE, SortableHeader } from '@trading-agent/shared-components';
import { ComputedSymbol } from '@/api';
import { getTrackingErrorMessage } from '@/common/errors/errorMessages';
import { formatDateTime } from '@/common/format/format';
import { ASSET_TYPE_LABELS, labelOf, REASON_DESCRIPTIONS, REASON_LABELS } from '@/common/format/trackingLabels';
import ComputeState from '@/components/ComputeState';
import SymbolLink from '@/components/SymbolLink';
import Tag from '@/components/Tag';
import { COMPUTED_COLUMNS, NOT_COMPUTED_TEXT } from './columns';
import { ComputedTableProps } from './types';
import '@/styles/tracking-table.css';
import './ComputedTable-styles.css';

const isManual = (item: ComputedSymbol) => item.reasons.includes('manual');

/** Only the manual reason is closed; say what happens to the symbol afterwards. */
const stopHint = (item: ComputedSymbol): string =>
    item.reasons.some(reason => reason !== 'manual')
        ? 'Other reasons keep it computed'
        : 'Leaves the list; computed history is kept';

const renderCell = (key: string, item: ComputedSymbol, dataTimeoutMinutes: number | null): ReactNode => {
    switch (key) {
        case 'name':
            return (
                <span className="tracking-table__name" title={item.name ?? undefined}>
                    {item.name ?? EMPTY_VALUE}
                </span>
            );
        case 'type':
            return labelOf(ASSET_TYPE_LABELS, item.asset_type);
        case 'reasons':
            return (
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
            );
        case 'state':
            return <ComputeState item={item} dataTimeoutMinutes={dataTimeoutMinutes} variant="table" />;
        case 'computed_at':
            return item.computed_at ? <time dateTime={item.computed_at}>{formatDateTime(item.computed_at)}</time> : NOT_COMPUTED_TEXT;
        default:
            return null;
    }
};

/**
 * Every symbol with an open reason in the screen's sort: why, where its
 * computation stands, and Stop computing for manual requests. Stocks and
 * funds link to Stock Detail (hosted); crypto pairs don't.
 */
const ComputedTable = ({ id, items, headerProps, dataTimeoutMinutes, requesting, errors, onStop }: ComputedTableProps) => (
    <div className="tracking-table__wrap ta-fit-scroll" role="region" aria-label="Computed symbols, scrolls" tabIndex={0}>
        <table className="tracking-table computed-table" id={id} data-testid="computed-table">
            <caption className="tracking-table__caption">Computed symbols</caption>
            <thead>
                <tr>
                    {COMPUTED_COLUMNS.map(column => (
                        <SortableHeader key={column.key} {...headerProps(column.key)} />
                    ))}
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
                            {COMPUTED_COLUMNS.map(column =>
                                column.key === 'symbol' ? (
                                    <th key={column.key} scope="row" data-column="symbol">
                                        <SymbolLink className="tracking-table__ticker" symbol={item.symbol} assetType={item.asset_type} />
                                    </th>
                                ) : (
                                    <td
                                        key={column.key}
                                        data-column={column.key}
                                        className={column.key === 'computed_at' ? 'tracking-table__muted' : undefined}
                                    >
                                        {renderCell(column.key, item, dataTimeoutMinutes)}
                                    </td>
                                )
                            )}
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
