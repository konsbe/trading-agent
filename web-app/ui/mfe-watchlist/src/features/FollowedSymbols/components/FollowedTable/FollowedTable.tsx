import { ReactNode } from 'react';
import { Button, CloseIcon, EMPTY_VALUE, SortableHeader } from '@trading-agent/shared-components';
import { FollowedSymbol } from '@/api';
import { getTrackingErrorMessage } from '@/common/errors/errorMessages';
import { formatDate, formatDateTime } from '@/common/format/format';
import { ASSET_TYPE_LABELS, labelOf, LISTING_LABELS, SOURCE_LABELS } from '@/common/format/trackingLabels';
import ComputeButton from '@/components/ComputeButton';
import SymbolLink from '@/components/SymbolLink';
import { FollowedTableProps } from './types';
import '@/styles/tracking-table.css';
import './FollowedTable-styles.css';

const unfollowHint = (item: FollowedSymbol) =>
    item.source === 'env_seed'
        ? `Stop fetching ${item.symbol} every day. It was seeded from .env once: unfollowing removes it, and following it again lists it as added by you. Computed history is kept.`
        : `Stop fetching ${item.symbol} every day. Computed history is kept.`;

const MUTED_COLUMNS = new Set(['source', 'added_at']);

const renderCell = (key: string, item: FollowedSymbol): ReactNode => {
    switch (key) {
        case 'name':
            return (
                <span className="tracking-table__name" title={item.name ?? undefined}>
                    {item.name ?? EMPTY_VALUE}
                </span>
            );
        case 'type':
            return labelOf(ASSET_TYPE_LABELS, item.asset_type);
        case 'listing':
            return labelOf(LISTING_LABELS, item.listing);
        case 'source':
            return labelOf(SOURCE_LABELS, item.source);
        case 'added_at':
            return (
                <time dateTime={item.added_at} title={formatDateTime(item.added_at)}>
                    {formatDate(item.added_at)}
                </time>
            );
        case 'compute':
            return <ComputeButton symbol={item.symbol} label={item.name ?? item.symbol} />;
        default:
            return null;
    }
};

/**
 * Every followed symbol in the screen's sort, with Compute and Unfollow per
 * row. Stocks and funds link to Stock Detail (hosted); crypto pairs don't.
 */
const FollowedTable = ({ id, rows, columns, headerProps, saving, errors, onUnfollow }: FollowedTableProps) => (
    <div className="tracking-table__wrap ta-fit-scroll" role="region" aria-label="Followed symbols, scrolls" tabIndex={0}>
        <table className="tracking-table" id={id} data-testid="followed-table">
            <caption className="tracking-table__caption">Followed symbols</caption>
            <thead>
                <tr>
                    {columns.map(column => (
                        <SortableHeader key={column.key} {...headerProps(column.key)} />
                    ))}
                    <th scope="col" data-column="actions">
                        <span className="tracking-table__sr-only">Actions</span>
                    </th>
                </tr>
            </thead>
            <tbody>
                {rows.map(item => {
                    const isSaving = saving.has(item.symbol);
                    const error = errors.get(item.symbol)?.action === 'unfollow' ? errors.get(item.symbol) : undefined;
                    return (
                        <tr key={item.symbol} className="tracking-table__row" data-testid={`followed-row-${item.symbol}`}>
                            {columns.map(column =>
                                column.key === 'symbol' ? (
                                    <th key={column.key} scope="row" data-column="symbol">
                                        <SymbolLink className="tracking-table__ticker" symbol={item.symbol} assetType={item.asset_type} />
                                    </th>
                                ) : (
                                    <td
                                        key={column.key}
                                        data-column={column.key}
                                        className={MUTED_COLUMNS.has(column.key) ? 'tracking-table__muted' : undefined}
                                    >
                                        {renderCell(column.key, item)}
                                    </td>
                                )
                            )}
                            <td data-column="actions">
                                <Button
                                    variant="ghost"
                                    size="sm"
                                    className="tracking-table__action"
                                    aria-label={`Unfollow ${item.symbol}`}
                                    aria-busy={isSaving}
                                    disabled={isSaving}
                                    title={unfollowHint(item)}
                                    onClick={() => onUnfollow(item.symbol)}
                                    data-testid={`unfollow-${item.symbol}`}
                                >
                                    <CloseIcon size={14} />
                                    {isSaving ? 'Unfollowing…' : 'Unfollow'}
                                </Button>
                                {error && (
                                    <span className="followed-table__error" role="alert" data-testid={`unfollow-error-${item.symbol}`}>
                                        {getTrackingErrorMessage(error.error)}
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

export default FollowedTable;
