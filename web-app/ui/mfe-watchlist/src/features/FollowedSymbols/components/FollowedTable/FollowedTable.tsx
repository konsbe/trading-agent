import { Button, CloseIcon, EMPTY_VALUE } from '@trading-agent/shared-components';
import { FollowedSymbol } from '@/api';
import { getTrackingErrorMessage } from '@/common/errors/errorMessages';
import { formatDate, formatDateTime } from '@/common/format/format';
import { ASSET_TYPE_LABELS, labelOf, LISTING_LABELS, SOURCE_LABELS } from '@/common/format/trackingLabels';
import ComputeButton from '@/components/ComputeButton';
import { FollowedTableProps } from './types';
import '@/styles/tracking-table.css';
import './FollowedTable-styles.css';

const unfollowHint = (item: FollowedSymbol) =>
    item.source === 'env_seed'
        ? `Stop fetching ${item.symbol} every day. It was seeded from .env once: unfollowing removes it, and following it again lists it as added by you. Computed history is kept.`
        : `Stop fetching ${item.symbol} every day. Computed history is kept.`;

/** Every followed symbol, with Compute and Unfollow per row. */
const FollowedTable = ({ items, saving, errors, onUnfollow }: FollowedTableProps) => (
    <div className="tracking-table__wrap" role="region" aria-label="Followed symbols, scrolls horizontally" tabIndex={0}>
        <table className="tracking-table" data-testid="followed-table">
            <caption className="tracking-table__caption">Followed symbols</caption>
            <thead>
                <tr>
                    <th scope="col" data-column="symbol">Symbol</th>
                    <th scope="col" data-column="name">Name</th>
                    <th scope="col" data-column="type">Type</th>
                    <th scope="col" data-column="listing">Listing</th>
                    <th scope="col" data-column="source">Source</th>
                    <th scope="col" data-column="added">Added</th>
                    <th scope="col" data-column="compute">Compute</th>
                    <th scope="col" data-column="actions">
                        <span className="tracking-table__sr-only">Actions</span>
                    </th>
                </tr>
            </thead>
            <tbody>
                {items.map(item => {
                    const isSaving = saving.has(item.symbol);
                    const error = errors.get(item.symbol)?.action === 'unfollow' ? errors.get(item.symbol) : undefined;
                    return (
                        <tr key={item.symbol} className="tracking-table__row" data-testid={`followed-row-${item.symbol}`}>
                            <th scope="row" data-column="symbol">
                                <span className="tracking-table__ticker">{item.symbol}</span>
                            </th>
                            <td data-column="name">
                                <span className="tracking-table__name" title={item.name ?? undefined}>
                                    {item.name ?? EMPTY_VALUE}
                                </span>
                            </td>
                            <td data-column="type">{labelOf(ASSET_TYPE_LABELS, item.asset_type)}</td>
                            <td data-column="listing">{labelOf(LISTING_LABELS, item.listing)}</td>
                            <td data-column="source" className="tracking-table__muted">
                                {labelOf(SOURCE_LABELS, item.source)}
                            </td>
                            <td data-column="added" className="tracking-table__muted">
                                <time dateTime={item.added_at} title={formatDateTime(item.added_at)}>
                                    {formatDate(item.added_at)}
                                </time>
                            </td>
                            <td data-column="compute">
                                <ComputeButton symbol={item.symbol} label={item.name ?? item.symbol} />
                            </td>
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
