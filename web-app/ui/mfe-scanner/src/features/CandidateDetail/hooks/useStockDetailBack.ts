import { useState } from 'react';
import { useLocation } from 'react-router-dom';
import { PageHeaderBack, readStockDetailState, STOCK_DETAIL_BASE_PATH, StockDetailState } from '@trading-agent/shared-components';

export interface StockDetailBack extends PageHeaderBack {
    label: string;
    /** Without the arrow, for in-text links ("Back to Candidates"). */
    inlineLabel: string;
    to: string;
}

export const ALL_CANDIDATES_BACK: StockDetailBack = {
    label: '← All candidates',
    inlineLabel: 'Back to candidates',
    to: STOCK_DETAIL_BASE_PATH,
};

/**
 * Stock Detail's back action. Opened from a list that recorded itself
 * (`stockDetailLink`): "← Back to {label}" to that exact URL, sort and search
 * included — a link, not history back, because this page pushes its own
 * entries (#classical-signals, chart ranges). The origin is kept while the
 * page stays mounted, so an in-page entry without router state doesn't lose
 * it. Deep link: "← All candidates".
 */
const useStockDetailBack = (): StockDetailBack => {
    const incoming = readStockDetailState(useLocation().state);
    const [origin, setOrigin] = useState<StockDetailState | null>(incoming);

    if (incoming && (incoming.from !== origin?.from || incoming.fromLabel !== origin?.fromLabel)) {
        setOrigin(incoming);
    }

    const current = incoming ?? origin;
    return current ? { label: `← Back to ${current.fromLabel}`, inlineLabel: `Back to ${current.fromLabel}`, to: current.from } : ALL_CANDIDATES_BACK;
};

export default useStockDetailBack;
