import { KeyboardEvent, MouseEvent, useCallback } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { Button, CloseIcon, EMPTY_VALUE, formatPrice, MARKET_COLUMNS, MarketRow } from '@trading-agent/shared-components';
import { WatchlistItem } from '@/api';
import { DAILY_BARS_LABEL, dailyBarsTooltip, isDailyBarsRow, marketCapNoteLabel } from '@/common/format/dataSource';
import { formatTradingDay } from '@/common/format/format';
import ComputeButton from '@/components/ComputeButton';
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

const hasMarketCapNote = (item: WatchlistItem) =>
    isDailyBarsRow(item) && item.market_cap === null && item.market_cap_est === null && Boolean(item.market_cap_note);

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

const NO_MARKET_DATA: Partial<MarketRow> = {
    close: null,
    change_pct: null,
    rvol_20: null,
    dollar_volume: null,
    rsi_14: null,
    breakout_state: null,
    pct_of_52w_high: null,
    market_cap: null,
    market_cap_est: null,
    momentum_score_100: null,
    score_attainable: null,
};

/** Without a features row (`as_of` null) no market value is shown, even if one is present. */
const marketValues = (item: WatchlistItem): MarketRow => (item.as_of === null ? { ...item, ...NO_MARKET_DATA } : item);

/**
 * The watched symbols with the candidates table's market columns (same cells,
 * formatters, labels and tooltips from shared-components). Hosted, a symbol
 * with stored data opens the scanner's detail page: the ticker is a real link
 * and the row activates on click or Enter, like the candidates table. A symbol
 * without data (`as_of` null) isn't linked — its detail page would be a 404 —
 * and neither is anything standalone, where the detail route doesn't exist.
 * A row computed from the symbol's own daily bars (outside the scanner's
 * universe) carries a "from daily bars" marker and never a score.
 */
const WatchlistTable = ({ id, caption, rows, saving, onRemove }: WatchlistTableProps) => {
    const isHosted = useIsHosted();
    const navigate = useNavigate();
    const isLinked = (item: WatchlistItem) => isHosted && item.as_of !== null;

    const handleRowClick = useCallback(
        (event: MouseEvent<HTMLTableRowElement>, symbol: string) => {
            // The ticker link navigates on its own; Remove and Compute must never navigate.
            if ((event.target as HTMLElement).closest('a, button, summary, [data-column="compute"]')) return;
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
                        {MARKET_COLUMNS.map(column => (
                            <th
                                key={column.key}
                                scope="col"
                                className={column.numeric ? 'is-numeric' : undefined}
                                title={column.description}
                                data-column={column.key}
                            >
                                {column.label}
                            </th>
                        ))}
                        <th scope="col" data-column="compute" title="Ask the pipeline to fetch and compute this symbol now">
                            Compute
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
                        const values = marketValues(item);
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
                                {MARKET_COLUMNS.map(column => (
                                    <td
                                        key={column.key}
                                        className={column.numeric ? 'is-numeric' : undefined}
                                        data-column={column.key}
                                    >
                                        {column.key === 'close' ? (
                                            <PriceCell item={item} isSaving={isSaving} />
                                        ) : column.key === 'market_cap' && hasMarketCapNote(item) ? (
                                            <MarketCapNote item={item} />
                                        ) : (
                                            column.render(values)
                                        )}
                                    </td>
                                ))}
                                <td data-column="compute">
                                    <ComputeButton symbol={item.symbol} />
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
