import { EMPTY_VALUE, formatTradingDay } from '@/common/format/format';
import { BUCKET_LABELS } from '@/features/Candidates/constants';
import { DetailMetaProps, DetailTitleProps } from './types';
import './DetailHeader-styles.css';

/** h1 content: ticker plus company name ("—" once loaded without one). */
export const DetailTitle = ({ symbol, companyName }: DetailTitleProps) => (
    <span className="scanner-detail-title">
        <span className="scanner-detail-title__ticker">{symbol}</span>
        {companyName !== undefined && (
            <span className="scanner-detail-title__company" data-testid="company-name">
                {companyName ?? EMPTY_VALUE}
            </span>
        )}
    </span>
);

/** Exchange badge, bucket and the session the data describes; nulls are "—". */
export const DetailMeta = ({ exchange, bucket, asOf }: DetailMetaProps) => (
    <span className="scanner-detail-meta" data-testid="symbol-meta">
        <span className="scanner-detail-meta__badge" data-testid="exchange-badge" title={exchange ? undefined : 'Exchange not available'}>
            {exchange ?? EMPTY_VALUE}
        </span>
        <span data-testid="bucket">{bucket ? BUCKET_LABELS[bucket] : `Bucket ${EMPTY_VALUE}`}</span>
        <span data-testid="as-of">as of {formatTradingDay(asOf, 'none')}, after market close</span>
    </span>
);
