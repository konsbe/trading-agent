import { KeyboardEvent, MouseEvent, useCallback } from 'react';
import { useLocation, useNavigate } from 'react-router-dom';
import {
    Button,
    CloseIcon,
    EMPTY_VALUE,
    formatPrice,
    MARKET_COLUMNS,
    SortableHeader,
    StockDetailLink,
    stockDetailLink,
} from '@trading-agent/shared-components';
import { WatchlistItem } from '@/api';
import { DAILY_BARS_LABEL, dailyBarsTooltip, isDailyBarsRow, marketCapNoteLabel } from '@/common/format/dataSource';
import { formatDate, formatDateTime, formatTradingDay } from '@/common/format/format';
import ComputeButton from '@/components/ComputeButton';
import { ORIGIN_LABELS, useCanOpenStockDetail } from '@/providers/StockDetailOrigin';
import { hasMarketCapNote, marketValues } from './columns';
import { WatchlistTableProps } from './types';
import './WatchlistTable-styles.css';

const MARKET_COLUMN_BY_KEY = new Map(MARKET_COLUMNS.map(column => [column.key as string, column]));

const SymbolCell = ({ item, linked }: { item: WatchlistItem; linked: boolean }) => (
    <div className="watchlist-table__symbol">
        {linked ? (
            <StockDetailLink className="watchlist-table__ticker is-link" symbol={item.symbol} originLabel={ORIGIN_LABELS.watchlist} />
        ) : (
            <span className="watchlist-table__ticker">{item.symbol}</span>
        )}
        <span className="watchlist-table__symbol-meta">
            <span className="watchlist-table__company" title={item.company_name ?? undefined} data-testid="company-name">
                {item.company_name ?? EMPTY_VALUE}
            </span>
            <span className="watchlist-table__exchange">{item.exchange ?? EMPTY_VALUE}</span>
        </span>
        {isDailyBarsRow(item) && (
            <span className="watchlist-table__source" title={dailyBarsTooltip(item)} data-testid={`daily-bars-${item.symbol}`}>
                {DAILY_BARS_LABEL}
            </span>
        )}
    </div>
);

/** A daily-bars row's null market cap with the API's reason; the figure is never converted. */
const MarketCapNote = ({ item }: { item: WatchlistItem }) => {
    const [reason, ...rest] = marketCapNoteLabel(item.market_cap_note ?? '').split(' — ');
    return (
        <span className="watchlist-table__cap-note">
            <span data-testid="market-cap-value">{EMPTY_VALUE}</span>
            <span className="watchlist-table__hint" title={item.market_cap_note ?? undefined} data-testid={`market-cap-note-${item.symbol}`}>
                <span className="watchlist-table__nowrap">{reason}</span>
                {rest.length > 0 && (
                    <>
                        {' '}
                        <span className="watchlist-table__nowrap">— {rest.join(' — ')}</span>
                    </>
                )}
            </span>
        </span>
    );
};

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

const AddedCell = ({ iso }: { iso: string }) => (
    <time className="watchlist-table__muted" dateTime={iso} title={formatDateTime(iso)}>
        {formatDate(iso)}
    </time>
);

/**
 * The watched symbols with the candidates table's market columns (same cells,
 * formatters, labels and tooltips from shared-components), in the order the
 * screen's table view gives them; every column but Remove sorts from its
 * header. Hosted, a stock or fund with stored data opens Stock Detail
 * ("Back to Watchlist" returns to this sort and search): the ticker is a real
 * link and the row activates on click or Enter, like the candidates table. A
 * symbol without data (`as_of` null) isn't linked — its detail page would be
 * a 404 — nor is a crypto pair, nor anything standalone, where the detail
 * route doesn't exist. A row computed from the symbol's own daily bars
 * (outside the scanner's universe) carries a "from daily bars" marker and
 * never a score.
 */
const WatchlistTable = ({ id, caption, rows, columns, headerProps, saving, onRemove }: WatchlistTableProps) => {
    const canOpen = useCanOpenStockDetail();
    const navigate = useNavigate();
    const { pathname, search } = useLocation();
    const isLinked = (item: WatchlistItem) => item.as_of !== null && canOpen({ symbol: item.symbol });

    const openSymbol = useCallback(
        (symbol: string) => {
            const target = stockDetailLink(symbol, { label: ORIGIN_LABELS.watchlist, from: `${pathname}${search}` });
            navigate(target.pathname, { state: target.state });
        },
        [navigate, pathname, search]
    );

    const handleRowClick = useCallback(
        (event: MouseEvent<HTMLTableRowElement>, symbol: string) => {
            // The ticker link navigates on its own; Remove and Compute must never navigate.
            if ((event.target as HTMLElement).closest('a, button, summary, [data-column="compute"]')) return;
            openSymbol(symbol);
        },
        [openSymbol]
    );

    const handleRowKeyDown = useCallback(
        (event: KeyboardEvent<HTMLTableRowElement>, symbol: string) => {
            if (event.key === 'Enter' && event.target === event.currentTarget) {
                event.preventDefault();
                openSymbol(symbol);
            }
        },
        [openSymbol]
    );

    const renderCell = (key: string, item: WatchlistItem, isSaving: boolean) => {
        if (key === 'close') return <PriceCell item={item} isSaving={isSaving} />;
        if (key === 'market_cap' && hasMarketCapNote(item)) return <MarketCapNote item={item} />;
        if (key === 'added_at') return <AddedCell iso={item.added_at} />;
        if (key === 'compute') return <ComputeButton symbol={item.symbol} />;
        return MARKET_COLUMN_BY_KEY.get(key)?.render(marketValues(item)) ?? null;
    };

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
                        {columns.map(column => (
                            <SortableHeader
                                key={column.key}
                                {...headerProps(column.key)}
                                align={column.numeric ? 'end' : 'start'}
                                className={column.numeric ? 'is-numeric' : undefined}
                                title={column.description}
                            />
                        ))}
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
                                {columns.map(column =>
                                    column.key === 'symbol' ? (
                                        <th key={column.key} scope="row" data-column="symbol">
                                            <SymbolCell item={item} linked={linked} />
                                        </th>
                                    ) : (
                                        <td
                                            key={column.key}
                                            className={column.numeric ? 'is-numeric' : undefined}
                                            data-column={column.key}
                                        >
                                            {renderCell(column.key, item, isSaving)}
                                        </td>
                                    )
                                )}
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
