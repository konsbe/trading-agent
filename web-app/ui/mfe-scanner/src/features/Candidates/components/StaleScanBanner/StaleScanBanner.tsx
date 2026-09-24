import { staleScanMessage } from '@/common/errors/errorMessages';
import { StaleScanBannerProps } from './types';
import './StaleScanBanner-styles.css';

/** Shown above the (still valid) data when `scan.is_stale` is true. */
const StaleScanBanner = ({ scanDate }: StaleScanBannerProps) => (
    <p className="scanner-stale-banner" role="status" data-testid="stale-banner">
        {staleScanMessage(scanDate)}
    </p>
);

export default StaleScanBanner;
