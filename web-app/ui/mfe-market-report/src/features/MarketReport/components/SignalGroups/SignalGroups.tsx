import { MacroSignal } from '@/api';
import { formatCompactNumber, formatDate, formatNumber } from '../../utils/format';
import { humanizeMetric } from '../../utils/humanize';
import { signalLabel, yieldLevels } from '../../utils/payload';
import { groupSignalsByTier, SignalGroup } from '../../utils/signals';
import ToneIndicator from '../ToneIndicator';
import { SignalGroupsProps } from './types';
import '@/styles/market-report-global.css';
import './SignalGroups-styles.css';

const groupHeading = ({ tier, group }: SignalGroup): string =>
    tier === null ? 'Other signals' : group ? `Tier ${tier} · ${group}` : `Tier ${tier}`;

/** `display_only` rows (treasury yields): their stored levels as plain numbers. */
const Levels = ({ payload, testId }: { payload: unknown; testId: string }) => {
    const levels = yieldLevels(payload);
    if (levels.length === 0) return null;
    return (
        <span className="market-report-signal__levels market-report-mono" data-testid={`${testId}-levels`}>
            {levels.map(({ tenor, value }) => `${tenor} ${formatNumber(value)}%`).join(' · ')}
        </span>
    );
};

const SignalRow = ({ name, signal }: { name: string; signal: MacroSignal }) => {
    const testId = `signal-${name}`;
    const displayOnly = signal.tone === 'display_only';
    const label = displayOnly ? null : signalLabel(signal.payload);
    return (
        <li className="market-report-signal" data-testid={testId}>
            <span className="market-report-signal__indicator">
                <ToneIndicator tone={signal.tone} size={16} data-testid={`${testId}-tone`} />
            </span>
            <span className="market-report-signal__name">{humanizeMetric(name)}</span>
            {displayOnly ? (
                <Levels payload={signal.payload} testId={testId} />
            ) : (
                signal.value !== null && (
                    <span className="market-report-signal__value market-report-mono" data-testid={`${testId}-value`}>
                        {formatCompactNumber(signal.value)}
                    </span>
                )
            )}
            <span className="market-report-signal__meta market-report-muted market-report-small">
                {label && (
                    <span className="market-report-signal__label" data-testid={`${testId}-label`}>
                        {label}
                    </span>
                )}
                {label && ' · '}
                <span className="market-report-signal__as-of">as of {formatDate(signal.as_of)}</span>
            </span>
        </li>
    );
};

/** A classification's signals, by stored tier (1, 2, 3, then untiered), each with its own tone. */
const SignalGroups = ({ signals, 'data-testid': testId }: SignalGroupsProps) => (
    <div className="market-report-signal-groups" data-testid={testId}>
        {groupSignalsByTier(signals).map(group => (
            <section key={group.key} className="market-report-signal-group" data-testid="signal-group" data-tier={group.tier ?? 'none'}>
                <h4 className="market-report-eyebrow market-report-signal-group__heading">{groupHeading(group)}</h4>
                <ul className="market-report-signal-group__list">
                    {group.signals.map(([name, signal]) => (
                        <SignalRow key={name} name={name} signal={signal} />
                    ))}
                </ul>
            </section>
        ))}
    </div>
);

export default SignalGroups;
