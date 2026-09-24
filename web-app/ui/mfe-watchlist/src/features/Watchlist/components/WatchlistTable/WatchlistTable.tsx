import { KeyboardEvent, MouseEvent, useCallback } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { Button, CloseIcon } from '@trading-agent/shared-components';
import { WatchlistItem } from '@/api';
import {
    EMPTY_VALUE,
    formatMultiple,
    formatPrice,
    formatSignedPercent,
    formatTradingDay,
} from '@/common/format/format';
import { scannerDetailPath } from '@/config/routes';
import { useIsHosted } from '@/providers/HostModeContext';
import { WatchlistTableProps } from './types';
import './WatchlistTable-styles.css';

const SymbolCell = ({ item, linked }: { item: WatchlistItem; linked: boolean }) => (
    <div className="watchlist-table__symbol">
        {linked ? (
            <Link className="watchlist-table__ticker is-link" to={scannerDetailPath(item.symbol)}>
                {item.symbol}
            </Link>
        ) : (
            <span className="watchlist-table__ticker">{item.symbol}</span>
        )}
        <span className="watchlist-table__symbol-meta">
            <span className="watchlist-table__company" title={item.company_name ?? undefined} data-testid="company-name">
                {item.company_name ?? EMPTY_VALUE}
            </span>
            <span className="watchlist-table__exchange">{item.exchange ?? EMPTY_VALUE}</span>
        </span>
    </div>
);

/** Neutral note so an old price never reads as current, or why there is none. */
const priceHint = (item: WatchlistItem, isSaving: boolean): string | null => {
    if (item.as_of === null) return isSaving ? 'Adding…' : 'No price data';
    return item.is_stale ? `as of ${formatTradingDay(item.as_of, 'none')}` : null;
};

const PriceCell = ({ item, isSaving }: { item: WatchlistItem; isSaving: boolean }) => {
    const hint = priceHint(item, isSaving);
    return (
        <span className="watchlist-table__price">
            {hint && (
                <span className="watchlist-table__hint" data-testid="price-hint">
                    {hint}
                </span>
            )}
            <span data-testid="price-value">{item.as_of === null ? EMPTY_VALUE : formatPrice(item.close)}</span>
        </span>
    );
};

/** The only toned value: a price delta (zero and null stay neutral). */
const ChangeCell = ({ item }: { item: WatchlistItem }) => {
    const value = item.as_of === null ? null : item.change_pct;
    const tone = value === null || !Number.isFinite(value) || value === 0 ? '' : value > 0 ? ' is-price-up' : ' is-price-down';
    return (
        <span className={`watchlist-table__change${tone}`} data-testid="change-value">
            {formatSignedPercent(value)}
        </span>
    );
};

/**
 * The watched symbols with their latest stored price. Hosted, a symbol with
 * stored data opens the scanner's detail page: the ticker is a real link and
 * the row activates on click or Enter, like the candidates table. A symbol
 * without data (`as_of` null) isn't linked — its detail page would be a 404 —
 * and neither is anything standalone, where the detail route doesn't exist.
 */
const WatchlistTable = ({ id, caption, rows, saving, onRemove }: WatchlistTableProps) => {
    const isHosted = useIsHosted();
    const navigate = useNavigate();
    const isLinked = (item: WatchlistItem) => isHosted && item.as_of !== null;

    const handleRowClick = useCallback(
        (event: MouseEvent<HTMLTableRowElement>, symbol: string) => {
            // The ticker link navigates on its own; Remove must never navigate.
            if ((event.target as HTMLElement).closest('a, button')) return;
            navigate(scannerDetailPath(symbol));
        },
        [navigate]
    );

    const handleRowKeyDown = useCallback(
        (event: KeyboardEvent<HTMLTableRowElement>, symbol: string) => {
            if (event.key === 'Enter' && event.target === event.currentTarget) {
                event.preventDefault();
                navigate(scannerDetailPath(symbol));
            }
        },
        [navigate]
    );

    return (
        // Focusable, labelled scroll region so keyboard users can scroll when the table overflows.
        <div
            className="watchlist-table__wrap"
            role="region"
            aria-label={`${caption}, scrolls horizontally`}
            tabIndex={0}
            data-testid={`${id}-scroll`}
        >
            <table className="watchlist-table" id={id} data-testid={id}>
                <caption className="watchlist-table__caption">{caption}</caption>
                <thead>
                    <tr>
                        <th scope="col" data-column="symbol">Symbol</th>
                        <th scope="col" className="is-numeric" data-column="close">Price</th>
                        <th scope="col" className="is-numeric" data-column="change_pct">Change %</th>
                        <th scope="col" className="is-numeric" data-column="rvol_20" title="Relative volume vs. the 20-day average">
                            RVOL
                        </th>
                        <th scope="col" data-column="actions">
                            <span className="watchlist-table__sr-only">Actions</span>
                        </th>
                    </tr>
                </thead>
                <tbody>
                    {rows.map(item => {
                        const isSaving = saving.has(item.symbol);
                        const linked = isLinked(item);
                        return (
                            <tr
                                key={item.symbol}
                                className={`watchlist-table__row${linked ? ' is-linked' : ''}`}
                                data-testid={`watchlist-row-${item.symbol}`}
                                {...(linked && {
                                    tabIndex: 0,
                                    'aria-label': `${item.symbol}, open details`,
                                    onClick: (event: MouseEvent<HTMLTableRowElement>) => handleRowClick(event, item.symbol),
                                    onKeyDown: (event: KeyboardEvent<HTMLTableRowElement>) => handleRowKeyDown(event, item.symbol),
                                })}
                            >
                                <th scope="row" data-column="symbol">
                                    <SymbolCell item={item} linked={linked} />
                                </th>
                                <td className="is-numeric" data-column="close">
                                    <PriceCell item={item} isSaving={isSaving} />
                                </td>
                                <td className="is-numeric" data-column="change_pct">
                                    <ChangeCell item={item} />
                                </td>
                                <td className="is-numeric" data-column="rvol_20" data-testid="rvol-value">
                                    {item.as_of === null ? EMPTY_VALUE : formatMultiple(item.rvol_20)}
                                </td>
                                <td data-column="actions">
                                    <Button
                                        variant="ghost"
                                        size="sm"
                                        className="watchlist-table__remove"
                                        aria-label={`Remove ${item.symbol} from watchlist`}
                                        aria-busy={isSaving}
                                        disabled={isSaving}
                                        onClick={() => onRemove(item.symbol)}
                                        data-testid={`remove-${item.symbol}`}
                                    >
                                        <CloseIcon size={14} />
                                        Remove
                                    </Button>
                                </td>
                            </tr>
                        );
                    })}
                </tbody>
            </table>
        </div>
    );
};

export default WatchlistTable;
