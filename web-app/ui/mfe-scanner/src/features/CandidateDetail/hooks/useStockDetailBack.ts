import { useState } from 'react';
import { useLocation } from 'react-router-dom';
import { PageHeaderIconBack, readStockDetailState, STOCK_DETAIL_BASE_PATH, StockDetailState } from '@trading-agent/shared-components';

export interface StockDetailBack {
    /** Link text with the arrow ("← Back to Watchlist"), for the footer. */
    label: string;
    /** Without the arrow, for in-text links ("Back to Candidates"). */
    inlineLabel: string;
    /** The header bar's arrow button: its accessible name ("Back to Watchlist", "All candidates"). */
    arrow: PageHeaderIconBack;
    to: string;
}

export const ALL_CANDIDATES_BACK: StockDetailBack = {
    label: '← All candidates',
    inlineLabel: 'Back to candidates',
    arrow: { label: 'All candidates', to: STOCK_DETAIL_BASE_PATH, iconOnly: true },
    to: STOCK_DETAIL_BASE_PATH,
};

/**
 * Stock Detail's back action. Opened from a list that recorded itself
 * (`stockDetailLink`): "Back to {label}" to that exact URL, sort and search
 * included — a link, not history back, because this page pushes its own
 * entries (#classical-signals, chart ranges). The origin is kept while the
 * page stays mounted, so an in-page entry without router state doesn't lose
 * it. Deep link: "All candidates". The header shows it as an arrow button
 * (`arrow`); the footer and notices as text.
 */
const useStockDetailBack = (): StockDetailBack => {
    const incoming = readStockDetailState(useLocation().state);
    const [origin, setOrigin] = useState<StockDetailState | null>(incoming);

    if (incoming && (incoming.from !== origin?.from || incoming.fromLabel !== origin?.fromLabel)) {
        setOrigin(incoming);
    }

    const current = incoming ?? origin;
    if (!current) return ALL_CANDIDATES_BACK;
    const inlineLabel = `Back to ${current.fromLabel}`;
    return { label: `← ${inlineLabel}`, inlineLabel, arrow: { label: inlineLabel, to: current.from, iconOnly: true }, to: current.from };
};

export default useStockDetailBack;
