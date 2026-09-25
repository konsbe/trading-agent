import { AlertTriangleIcon, CheckCircleIcon, CircleDashedIcon, IconComponent, MinusCircleIcon } from '@trading-agent/shared-components';
import { IndicatorTone, ToneIndicatorProps } from './types';
import './ToneIndicator-styles.css';

/**
 * Stored macro tone → icon shape, accessible word and colour class (one
 * `--color-status-*` token each) — the same mapping as the Daily Market
 * Report's ToneIndicator. The tone comes from the API, never from a label or number.
 */
export const TONE_INDICATORS: Record<IndicatorTone, { Icon: IconComponent; word: string; className: string }> = {
    constructive: { Icon: CheckCircleIcon, word: 'constructive', className: 'scanner-tone--constructive' },
    neutral: { Icon: MinusCircleIcon, word: 'neutral', className: 'scanner-tone--neutral' },
    stressed: { Icon: AlertTriangleIcon, word: 'stressed', className: 'scanner-tone--stressed' },
    no_data: { Icon: CircleDashedIcon, word: 'no data', className: 'scanner-tone--nodata' },
};

const isIndicatorTone = (tone: unknown): tone is IndicatorTone =>
    typeof tone === 'string' && Object.prototype.hasOwnProperty.call(TONE_INDICATORS, tone);

/** Icon + colour for a stored macro tone. Null or an unknown tone renders nothing. */
const ToneIndicator = ({ tone, size = 16, 'data-testid': testId }: ToneIndicatorProps) => {
    if (!isIndicatorTone(tone)) return null;
    const { Icon, word, className } = TONE_INDICATORS[tone];
    return (
        <span className={`scanner-tone ${className}`} role="img" aria-label={`Status: ${word}`} data-tone={tone} data-testid={testId}>
            <Icon size={size} />
        </span>
    );
};

export default ToneIndicator;
