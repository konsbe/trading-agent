import { formatTradingDay } from '@/common/format/format';
import { BUCKET_LABELS } from '@/features/Candidates/constants';
import { DetailMetaProps, DetailTitleProps } from './types';
import './DetailHeader-styles.css';

/** h1 content: ticker plus company name. */
export const DetailTitle = ({ symbol, companyName }: DetailTitleProps) => (
    <span className="scanner-detail-title">
        <span className="scanner-detail-title__ticker">{symbol}</span>
        {companyName && <span className="scanner-detail-title__company">{companyName}</span>}
    </span>
);

/** Exchange badge, bucket and the scan session the data describes. */
export const DetailMeta = ({ exchange, bucket, asOf }: DetailMetaProps) => (
    <span className="scanner-detail-meta" data-testid="symbol-meta">
        {exchange && <span className="scanner-detail-meta__badge" data-testid="exchange-badge">{exchange}</span>}
        {bucket && <span>{BUCKET_LABELS[bucket]}</span>}
        <span data-testid="as-of">as of {formatTradingDay(asOf, 'none')}, after market close</span>
    </span>
);
