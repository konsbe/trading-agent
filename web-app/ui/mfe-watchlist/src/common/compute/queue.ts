/**
 * A computed symbol's queued fetch. Both a Compute press (manual reason) and
 * adding the symbol to the watchlist queue one; `queued_at` is the newest.
 */

import { ComputedSymbol } from '@/api';

type QueueFields = Pick<ComputedSymbol, 'queued_at' | 'manual_requested_at'>;

/** When the open fetch was queued, or null when none is. */
export const queuedTime = (item: QueueFields): string | null => item.queued_at ?? item.manual_requested_at;

/** True when the newest queue is the user's Compute press, so it reads "requested"; otherwise it reads "queued". */
export const isQueuedByRequest = (item: QueueFields): boolean => {
    if (item.manual_requested_at === null) return false;
    if (item.queued_at === null) return true;
    return Date.parse(item.queued_at) <= Date.parse(item.manual_requested_at);
};

/** The queued fetch is still moving; `data_not_arrived` and `failed` are final until the user acts. */
export const isPending = (item: ComputedSymbol): boolean =>
    queuedTime(item) !== null && (item.state === 'waiting_for_data' || item.state === 'computing');

export const hasOpenManualRequest = (item: ComputedSymbol | undefined): item is ComputedSymbol =>
    Boolean(item && item.manual_requested_at !== null && item.reasons.includes('manual'));

/** A fetch queued without a Compute press (a watchlist addition) that is moving or has timed out. */
export const hasAutomaticQueue = (item: ComputedSymbol | undefined): item is ComputedSymbol =>
    Boolean(item && queuedTime(item) !== null && (isPending(item) || item.state === 'data_not_arrived'));

/** The item whose state a Compute control shows next to (or instead of) its button, or null for a plain button. */
export const shownComputeItem = (item: ComputedSymbol | undefined): ComputedSymbol | null =>
    hasOpenManualRequest(item) || hasAutomaticQueue(item) ? item : null;
