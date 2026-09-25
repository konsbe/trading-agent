import { AlertTriangleIcon, CheckCircleIcon, CircleDashedIcon, IconComponent, MinusCircleIcon } from '@trading-agent/shared-components';
import { IndicatorTone, ToneIndicatorProps } from './types';
import './ToneIndicator-styles.css';

/**
 * The stored tone → icon shape, accessible word and colour class (each class
 * reads one `--color-status-*` token). This table is the whole mapping: the
 * tone comes from the backend and is never derived from a label or a number.
 */
export const TONE_INDICATORS: Record<IndicatorTone, { Icon: IconComponent; word: string; className: string }> = {
    constructive: { Icon: CheckCircleIcon, word: 'constructive', className: 'market-report-tone--constructive' },
    neutral: { Icon: MinusCircleIcon, word: 'neutral', className: 'market-report-tone--neutral' },
    stressed: { Icon: AlertTriangleIcon, word: 'stressed', className: 'market-report-tone--stressed' },
    no_data: { Icon: CircleDashedIcon, word: 'no data', className: 'market-report-tone--nodata' },
};

const isIndicatorTone = (tone: unknown): tone is IndicatorTone =>
    typeof tone === 'string' && Object.prototype.hasOwnProperty.call(TONE_INDICATORS, tone);

/** Icon + colour for a stored tone. Null, missing or `display_only` renders nothing. */
const ToneIndicator = ({ tone, size = 18, 'data-testid': testId }: ToneIndicatorProps) => {
    if (!isIndicatorTone(tone)) return null;
    const { Icon, word, className } = TONE_INDICATORS[tone];
    return (
        <span className={`market-report-tone ${className}`} role="img" aria-label={`Status: ${word}`} data-tone={tone} data-testid={testId}>
            <Icon size={size} />
        </span>
    );
};

export default ToneIndicator;
