import { EMPTY_VALUE } from '@trading-agent/shared-components';
import { getTrackingErrorMessage } from '@/common/errors/errorMessages';
import ComputeButton from '@/components/ComputeButton';
import FollowButton from '../FollowButton';
import { SearchResultRowProps } from './types';
import './SearchResultRow-styles.css';

/** One search match: identity, facts, markers, then Follow and Compute side by side. */
const SearchResultRow = ({ symbol, name, facts, markers, isFollowed, isSaving, followError, onFollow }: SearchResultRowProps) => (
    <div className="tracking-result" data-testid={`result-${symbol}`}>
        <div className="tracking-result__identity">
            <span className="tracking-result__ticker">{symbol}</span>
            <span className="tracking-result__name" title={name ?? undefined}>
                {name ?? EMPTY_VALUE}
            </span>
        </div>
        <div className="tracking-result__facts">
            {facts.filter(Boolean).join(' · ') || EMPTY_VALUE}
            {markers}
        </div>
        <div className="tracking-result__actions">
            <FollowButton symbol={symbol} isFollowed={isFollowed} isSaving={isSaving} onFollow={onFollow} />
            <ComputeButton symbol={symbol} />
        </div>
        {followError && (
            <p className="tracking-result__error" role="alert" data-testid={`follow-error-${symbol}`}>
                Couldn&apos;t follow {symbol}: {getTrackingErrorMessage(followError)}
            </p>
        )}
    </div>
);

export default SearchResultRow;
