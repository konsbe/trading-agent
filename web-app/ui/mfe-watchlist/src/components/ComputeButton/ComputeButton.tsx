import { useCallback } from 'react';
import { Button } from '@trading-agent/shared-components';
import { ComputedSymbol } from '@/api';
import { getTrackingErrorMessage } from '@/common/errors/errorMessages';
import { hasAutomaticQueue, hasOpenManualRequest, isPending } from '@/common/compute/queue';
import ComputeState from '@/components/ComputeState';
import { useComputeStatus } from '@/providers/ComputeStatusContext';
import { ComputeButtonProps } from './types';
import './ComputeButton-styles.css';

const REASON_WORDS: Record<string, string> = {
    followed: 'followed',
    watchlist: 'on the watchlist',
    candidate: "today's candidate",
};

/** Tooltip for a symbol without a manual request. */
const computeHint = (item: ComputedSymbol | undefined): string => {
    const automatic = item?.reasons.filter(reason => reason !== 'manual').map(reason => REASON_WORDS[reason] ?? reason) ?? [];
    return automatic.length > 0
        ? `Already computed every day (${automatic.join(', ')}). Compute asks for a fresh fetch and computation now.`
        : "Queue this symbol: the data workers fetch it, then it's computed (usually 2–5 minutes).";
};

/**
 * Compute for one symbol, used by every table and search list. Without an open
 * manual request it's a button; once requested it shows the request's state
 * instead, kept current by the screen's ComputeStatusProvider. A fetch queued
 * by adding the symbol to the watchlist shows its state the same way while it
 * moves; if its data never arrives, Compute is offered next to it. A repeat PUT
 * keeps the same open request, so only `data_not_arrived` offers "Check again",
 * and it says so. 404 / 422 are shown inline in plain words.
 */
const ComputeButton = ({ symbol, label }: ComputeButtonProps) => {
    const { getItem, requesting, errors, compute, dataTimeoutMinutes } = useComputeStatus();
    const key = symbol.trim().toUpperCase();
    const item = getItem(key);
    const isRequesting = requesting.has(key);
    const error = errors.get(key);
    const handleCompute = useCallback(() => {
        compute(key);
    }, [compute, key]);

    const manual = hasOpenManualRequest(item) ? item : null;
    const queued = !manual && hasAutomaticQueue(item) ? item : null;
    const name = label ?? key;
    const shown = manual ?? queued;

    return (
        <div className="compute-control" data-testid={`compute-control-${key}`}>
            {shown && <ComputeState item={shown} dataTimeoutMinutes={dataTimeoutMinutes} />}
            {!manual && !(queued && isPending(queued)) && (
                <Button
                    variant="secondary"
                    size="sm"
                    className="compute-control__button"
                    aria-label={`Compute ${name}`}
                    aria-busy={isRequesting}
                    disabled={isRequesting}
                    title={computeHint(item)}
                    onClick={handleCompute}
                    data-testid={`compute-${key}`}
                >
                    {isRequesting ? 'Requesting…' : 'Compute'}
                </Button>
            )}
            {manual?.state === 'data_not_arrived' && (
                <span className="compute-control__retry">
                    <Button
                        variant="ghost"
                        size="sm"
                        aria-label={`Check again for ${name}`}
                        aria-busy={isRequesting}
                        disabled={isRequesting}
                        onClick={handleCompute}
                        data-testid={`compute-retry-${key}`}
                    >
                        {isRequesting ? 'Checking…' : 'Check again'}
                    </Button>
                    <span className="compute-control__hint">Same open request — this re-checks it; it doesn&apos;t start a new fetch.</span>
                </span>
            )}
            {error && (
                <span className="compute-control__error" role="alert" data-testid={`compute-error-${key}`}>
                    {getTrackingErrorMessage(error)}
                </span>
            )}
        </div>
    );
};

export default ComputeButton;
