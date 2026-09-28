import { SeverityBadge, StockDetailLink } from '@trading-agent/shared-components';
import { formatDateTime } from '@/common/format/format';
import { humanizeCode } from '@/common/format/humanize';
import { CLASSICAL_SIGNALS_ID } from '@/types/constants';
import { CANDIDATES_ORIGIN_LABEL } from '../../constants';
import { AlertBadgeProps } from './types';
import './AlertBadge-styles.css';

/** `liquidity_sweep` → "liquidity sweep"; a leading acronym keeps its case ("RSI overbought"). */
export const alertTypeLabel = (alertType: string): string => {
    const label = humanizeCode(alertType);
    return /^[A-Z]{2}/.test(label) ? label : label.charAt(0).toLowerCase() + label.slice(1);
};

/**
 * The candidate's most recent fired alert, beside the ticker: severity tag plus
 * the alert type. Links to Stock Detail's Classical technical signals section.
 */
const AlertBadge = ({ symbol, alert }: AlertBadgeProps) => {
    const type = alertTypeLabel(alert.alert_type);
    return (
        <StockDetailLink
            className="scanner-alert-badge"
            symbol={symbol}
            originLabel={CANDIDATES_ORIGIN_LABEL}
            hash={CLASSICAL_SIGNALS_ID}
            title={`${alert.message} · ${formatDateTime(alert.fired_at)}`}
            aria-label={`${alert.severity} — ${type}: open classical technical signals for ${symbol}`}
            data-testid={`alert-badge-${symbol}`}
        >
            <SeverityBadge severity={alert.severity} detail={type} />
        </StockDetailLink>
    );
};

export default AlertBadge;
