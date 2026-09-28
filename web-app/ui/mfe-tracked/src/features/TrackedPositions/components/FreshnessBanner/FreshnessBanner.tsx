import { chainFreshness } from '../../utils/freshness';
import { FreshnessBannerProps } from './types';
import './FreshnessBanner-styles.css';

/**
 * One system-level note above the tabs when the daily chain itself is behind
 * (every row is old at once). Informational, not an error: no data is wrong,
 * it is old. Renders nothing when the chain is current.
 */
const FreshnessBanner = ({ chain }: FreshnessBannerProps) => {
    const freshness = chainFreshness(chain);
    if (!freshness) return null;
    return (
        <p className="tracked-freshness" role="status" data-testid="freshness-banner" data-variant={freshness.variant}>
            {freshness.message}
        </p>
    );
};

export default FreshnessBanner;
