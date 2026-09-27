import { Link } from 'react-router-dom';
import { ComputedSymbol } from '@/api';
import { isQueuedByRequest, queuedTime } from '@/common/compute/queue';
import { formatClockTime, formatDateTime, minutesSince } from '@/common/format/format';
import { scannerDetailPath } from '@/config/routes';
import { useIsHosted } from '@/providers/HostModeContext';
import { ComputeStateProps } from './types';
import './ComputeState-styles.css';

/** Stock Detail's own wording for an analysis being computed (mfe-scanner AnalysisSections). */
export const COMPUTING_LABEL = 'Computing analysis';

/** Stock Detail exists hosted only, and serves any symbol with daily bars — not crypto pairs. */
const useDetailLinkable = (item: ComputedSymbol) => useIsHosted() && item.asset_type !== 'crypto';

const Time = ({ iso }: { iso: string }) => (
    <time dateTime={iso} title={formatDateTime(iso)}>
        {formatClockTime(iso)}
    </time>
);

/** "requested 5:45 PM" for a Compute press; "queued 5:56 PM (added to watchlist)" for a watchlist addition. */
const QueuedAt = ({ item, iso }: { item: ComputedSymbol; iso: string }) =>
    isQueuedByRequest(item) ? (
        <>
            requested <Time iso={iso} />
        </>
    ) : (
        <>
            queued <Time iso={iso} />
            {item.reasons.includes('watchlist') && ' (added to watchlist)'}
        </>
    );

/**
 * One symbol's computation state in plain words. `waiting_for_data` is a
 * queued state (a slow pulse, not a spinner); `computing` reads like Stock
 * Detail's on-demand analysis; `data_not_arrived` names the queue time and the
 * timeout and never animates. `variant="table"` drops the computed time, which
 * the Computed Symbols table shows in its own column.
 */
const ComputeState = ({ item, dataTimeoutMinutes, variant = 'inline' }: ComputeStateProps) => {
    const linkable = useDetailLinkable(item);
    const queuedAt = queuedTime(item);
    const testId = `compute-state-${item.symbol}`;

    switch (item.state) {
        case 'waiting_for_data':
            return (
                <span className="compute-state is-waiting" role="status" data-testid={testId} data-state={item.state}>
                    <span className="compute-state__pulse" aria-hidden="true" />
                    <span>
                        Waiting for data
                        {queuedAt && (
                            <span className="compute-state__muted">
                                {' '}
                                · <QueuedAt item={item} iso={queuedAt} />
                            </span>
                        )}
                    </span>
                </span>
            );
        case 'computing':
            return (
                <span className="compute-state is-computing" role="status" data-testid={testId} data-state={item.state}>
                    <span className="compute-state__title">{COMPUTING_LABEL}</span>{' '}
                    <span className="compute-state__muted">
                        · data fetched, picked up within a minute
                        {queuedAt && (
                            <>
                                {' '}
                                · <QueuedAt item={item} iso={queuedAt} />
                            </>
                        )}
                    </span>
                </span>
            );
        case 'computed':
            return (
                <span className="compute-state is-computed" data-testid={testId} data-state={item.state}>
                    <span>
                        Computed
                        {variant === 'inline' && item.computed_at && (
                            <>
                                {' '}
                                <Time iso={item.computed_at} />
                            </>
                        )}
                    </span>
                    {variant === 'inline' && linkable && (
                        <Link className="compute-state__link" to={scannerDetailPath(item.symbol)} data-testid={`compute-detail-link-${item.symbol}`}>
                            Stock Detail
                        </Link>
                    )}
                </span>
            );
        case 'data_not_arrived': {
            const waited = queuedAt ? minutesSince(queuedAt) : null;
            return (
                <span className="compute-state is-not-arrived" role="status" data-testid={testId} data-state={item.state}>
                    <span className="compute-state__title">Data hasn&apos;t arrived</span>{' '}
                    <span className="compute-state__muted">
                        {queuedAt && (
                            <>
                                — <QueuedAt item={item} iso={queuedAt} />, waited {waited} min
                            </>
                        )}
                        {dataTimeoutMinutes !== null && ` (gives up waiting after ${dataTimeoutMinutes} min)`}
                    </span>
                </span>
            );
        }
        case 'failed':
            return (
                <details className="compute-state is-failed" data-testid={testId} data-state={item.state}>
                    <summary className="compute-state__title" title={item.last_error ?? undefined}>
                        Computation failed
                    </summary>
                    <span className="compute-state__detail" data-testid={`compute-error-detail-${item.symbol}`}>
                        {item.last_error ?? 'No error message was stored.'}
                    </span>
                </details>
            );
        case 'scheduled':
            return (
                <span className="compute-state is-scheduled" data-testid={testId} data-state={item.state}>
                    Scheduled <span className="compute-state__muted">· next daily pass</span>
                </span>
            );
        default:
            return (
                <span className="compute-state" data-testid={testId} data-state={item.state}>
                    {item.state}
                </span>
            );
    }
};

export default ComputeState;
