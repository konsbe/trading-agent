import { formatInteger, formatPercent, providerName } from '../../utils/format';
import { ProviderCardProps } from './types';
import '@/styles/data-source-global.css';
import './ProviderCard-styles.css';

/**
 * One provider's budget. A provider without a daily limit (Finnhub) gets no
 * bar and no percentage — its theoretical capacity is not a quota.
 */
const ProviderCard = ({ providerKey, provider, overBudget }: ProviderCardProps) => {
    const name = providerName(providerKey);
    const testId = `provider-${providerKey}`;

    if (!provider) {
        return (
            <article className="data-source-provider is-missing" aria-label={name} data-testid={testId}>
                <h3 className="data-source-provider__name">{name}</h3>
                <p className="data-source-provider__missing" data-testid={`${testId}-missing`}>
                    No budget row for {name}
                </p>
            </article>
        );
    }

    const limited = provider.daily_limit !== null && provider.daily_used_pct !== null;
    const usageId = `${testId}-usage`;

    return (
        <article className="data-source-provider" aria-label={name} data-testid={testId}>
            <header className="data-source-provider__header">
                <h3 className="data-source-provider__name">{name}</h3>
                <p className="data-source-muted data-source-provider__role">{provider.role}</p>
            </header>

            {limited ? (
                <>
                    <p className="data-source-provider__usage" id={usageId} data-testid={usageId}>
                        <span className="data-source-mono">
                            {formatInteger(provider.daily_used)} / {formatInteger(provider.daily_limit!)} (
                            {formatPercent(provider.daily_used_pct!)})
                        </span>{' '}
                        <span className="data-source-muted">requests today</span>
                    </p>
                    <div
                        className={`data-source-provider__bar${overBudget ? ' is-over-budget' : ''}`}
                        role="progressbar"
                        aria-labelledby={usageId}
                        aria-valuemin={0}
                        aria-valuemax={provider.daily_limit!}
                        aria-valuenow={provider.daily_used}
                        aria-valuetext={`${formatInteger(provider.daily_used)} of ${formatInteger(provider.daily_limit!)} requests (${formatPercent(provider.daily_used_pct!)})`}
                        data-testid={`${testId}-bar`}
                    >
                        <span
                            className="data-source-provider__fill"
                            style={{ width: `${Math.min(100, Math.max(0, provider.daily_used_pct!))}%` }}
                        />
                    </div>
                </>
            ) : (
                <>
                    <p className="data-source-provider__usage" data-testid={usageId}>
                        <span className="data-source-mono">{formatInteger(provider.daily_used)}</span> used today · no daily
                        limit (rate-limited: {provider.rate_per_sec} req/s)
                    </p>
                    <p className="data-source-muted data-source-provider__small" data-testid={`${testId}-capacity`}>
                        theoretical capacity at this rate: {formatInteger(provider.theoretical_daily_capacity)}/day — not a quota
                    </p>
                </>
            )}

            <dl className="data-source-provider__facts">
                <div>
                    <dt>Window</dt>
                    <dd data-testid={`${testId}-window`}>
                        {provider.daily_window_start ?? 'none recorded'} (resets {provider.daily_reset_tz})
                    </dd>
                </div>
                <div>
                    <dt>Degraded (24h)</dt>
                    <dd data-testid={`${testId}-degraded`}>
                        {provider.degraded_count_24h === null ? 'not tracked' : formatInteger(provider.degraded_count_24h)}
                    </dd>
                </div>
            </dl>
        </article>
    );
};

export default ProviderCard;
