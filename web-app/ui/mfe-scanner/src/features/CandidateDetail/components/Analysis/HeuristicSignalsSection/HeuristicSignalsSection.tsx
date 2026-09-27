import { useEffect, useState } from 'react';
import { COLLAPSIBLE_STORAGE_PREFIX, CollapsibleCard, SeverityBadge } from '@trading-agent/shared-components';
import { ActionSignal } from '@/api';
import { CLASSICAL_SIGNALS_ID } from '@/types/constants';
import { chartPatternLabel, labelText, sentenceCaseCode } from '../../../utils/analysisFormat';
import { flaggedReadings } from './flaggedReadings';
import { HeuristicSignalsSectionProps } from './types';
import '@/styles/scanner-global.css';
import '../analysis-styles.css';
import './HeuristicSignalsSection-styles.css';

export const HEURISTIC_PERSIST_KEY = 'scanner.detail.classical-signals';

/**
 * Seeds the card's persisted state as expanded before it mounts, so a deep
 * link always opens it; the card still owns its toggle and persistence.
 */
const useExpandOnArrival = (focused: boolean) => {
    useState(() => {
        if (!focused) return;
        try {
            window.sessionStorage.setItem(`${COLLAPSIBLE_STORAGE_PREFIX}${HEURISTIC_PERSIST_KEY}`, 'true');
        } catch {
            // Storage blocked: the card falls back to its default (expanded).
        }
    });

    useEffect(() => {
        if (!focused) return;
        const card = document.getElementById(CLASSICAL_SIGNALS_ID);
        card?.scrollIntoView?.({ behavior: 'smooth', block: 'start' });
        document.getElementById(`${CLASSICAL_SIGNALS_ID}-toggle`)?.focus({ preventScroll: true });
    }, [focused]);
};

const ActionSignalBlock = ({ signal }: { signal: ActionSignal }) => (
    <div className="scanner-heuristic__action" data-testid="action-signal">
        <p className="scanner-heuristic__action-head">
            <code className="scanner-heuristic__action-label" data-testid="action-label">
                {signal.action}
            </code>
            <SeverityBadge severity={signal.severity} data-testid="action-severity" />
            <span className="scanner-heuristic__confluence" data-testid="action-confluence">
                Confluence {signal.confluence.score}/{signal.confluence.max}
            </span>
        </p>
        <p className="scanner-muted scanner-analysis__note" data-testid="action-source">
            From {sentenceCaseCode(signal.alert_type).toLowerCase()}
            {signal.vix_regime && ` · VIX regime read by the rule: ${labelText(signal.vix_regime).toLowerCase()}`}
        </p>
        {signal.reasoning.length > 0 && (
            <ul className="scanner-analysis__list" data-testid="action-reasoning">
                {signal.reasoning.map((line, i) => (
                    <li key={`${i}-${line}`}>{line}</li>
                ))}
            </ul>
        )}
    </div>
);

/**
 * Section 6 — classical TA heuristics, framed apart from the neutral sections.
 * The caveat comes first, verbatim; then every reading the API flagged (the one
 * place on the page that lists "what's currently flagged"), chart patterns, and
 * the action signal — each with the API's severity badge.
 */
const HeuristicSignalsSection = ({ signals, technical, focused = false }: HeuristicSignalsSectionProps) => {
    useExpandOnArrival(focused);
    const readings = flaggedReadings(technical);

    return (
        <CollapsibleCard
            id={CLASSICAL_SIGNALS_ID}
            persistKey={HEURISTIC_PERSIST_KEY}
            className={`scanner-heuristic${focused ? ' is-focused' : ''}`}
            data-testid="analysis-heuristic"
            title="Classical technical signals"
            meta={<span className="scanner-heuristic__kind">Heuristic</span>}
        >
            <aside className="scanner-caveat" aria-label="Heuristic signals caveat" data-testid="heuristic-caveat-callout">
                <p className="scanner-caveat__title">Heuristic signals caveat</p>
                <p className="scanner-caveat__text" data-testid="heuristic-caveat">
                    {signals.caveat}
                </p>
            </aside>

            <div className="scanner-analysis__group">
                <h3 className="scanner-analysis__group-title">Flagged readings</h3>
                {readings.length === 0 ? (
                    <p className="scanner-muted scanner-analysis__note" data-testid="flagged-readings-empty">
                        No technical readings flagged.
                    </p>
                ) : (
                    <ul className="scanner-heuristic__items" data-testid="flagged-readings">
                        {readings.map(r => (
                            <li key={r.key} className="scanner-heuristic__item" data-testid={`flagged-${r.key}`}>
                                <span className="scanner-heuristic__reading">{r.reading}</span>
                                <SeverityBadge severity={r.severity} />
                                {r.band && <span className="scanner-muted">({r.band})</span>}
                            </li>
                        ))}
                    </ul>
                )}
            </div>

            <div className="scanner-analysis__group">
                <h3 className="scanner-analysis__group-title">Chart patterns</h3>
                {signals.chart_patterns.length === 0 ? (
                    <p className="scanner-muted scanner-analysis__note" data-testid="chart-patterns-empty">
                        No chart patterns detected.
                    </p>
                ) : (
                    <ul className="scanner-heuristic__items" data-testid="chart-patterns">
                        {signals.chart_patterns.map((p, i) => (
                            <li key={`${p.pattern}-${i}`} className="scanner-heuristic__item" data-testid={`pattern-${p.pattern}`}>
                                <span>{chartPatternLabel(p.pattern)}</span>
                                <span className="scanner-muted">· {p.confirmed ? 'confirmed' : 'unconfirmed'}</span>
                                <SeverityBadge severity={p.severity} />
                            </li>
                        ))}
                    </ul>
                )}
            </div>

            <div className="scanner-analysis__group">
                <h3 className="scanner-analysis__group-title">Action signal</h3>
                {signals.action_signal ? (
                    <ActionSignalBlock signal={signals.action_signal} />
                ) : (
                    <p className="scanner-muted scanner-analysis__note" data-testid="action-signal-empty">
                        No action signal.
                    </p>
                )}
            </div>
        </CollapsibleCard>
    );
};

export default HeuristicSignalsSection;
