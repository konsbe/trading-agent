import { Fragment } from 'react';
import { AlertTriangleIcon, CollapsibleCard } from '@trading-agent/shared-components';
import { isSectionUnavailable, SessionStatus } from '@/api';
import { formatPercent, formatSessionDate } from '../../utils/format';
import { sessionDetail, sessionStatusLabel } from '../../utils/status';
import SectionUnavailable from '../SectionUnavailable';
import { DailyChainSectionProps } from './types';
import '@/styles/data-source-global.css';
import './DailyChainSection-styles.css';

const COLUMNS = 6;

const doneText = (done: boolean) => (done ? 'Done' : 'Not done');

/** Status as plain text only — no colour. not_run is weighted up (glyph + bold), not_recorded muted. */
const StatusCell = ({ session }: { session: SessionStatus }) => (
    <td>
        <div className="data-source-chain__status-cell">
            <span
                className={`data-source-chain__status data-source-chain__status--${session.status}`}
                data-testid={`chain-status-${session.session}`}
            >
                {session.status === 'not_run' && <AlertTriangleIcon size={14} className="data-source-chain__glyph" />}
                {sessionStatusLabel(session)}
            </span>
            {session.note && session.status !== 'failed' && (
                <span className="data-source-muted data-source-chain__note">{session.note}</span>
            )}
        </div>
    </td>
);

/** Section 3 — the last closed sessions of the daily chain, newest first. */
const DailyChainSection = ({ chain }: DailyChainSectionProps) => (
    <CollapsibleCard id="data-source-chain" persistKey="datasource.chain" title="Daily chain" data-testid="chain-section">
        {isSectionUnavailable(chain) ? (
            <SectionUnavailable
                data-testid="chain-unavailable"
                explanation="The status service couldn't read the daily chain's runs on this check."
            />
        ) : (
            <>
                {chain.last_clean_session !== (chain.sessions[0]?.session ?? null) && (
                    <p className="data-source-chain__last-clean" data-testid="last-clean-session">
                        Last clean session:{' '}
                        {chain.last_clean_session ? (
                            <time dateTime={chain.last_clean_session}>{formatSessionDate(chain.last_clean_session)}</time>
                        ) : (
                            'none on record'
                        )}
                    </p>
                )}

                {chain.sessions.length === 0 ? (
                    <p className="data-source-muted" data-testid="chain-empty">
                        No closed sessions to show.
                    </p>
                ) : (
                    <div className="data-source-chain__scroll">
                        <table className="data-source-chain__table" data-testid="chain-table">
                            <thead>
                                <tr>
                                    <th scope="col">Session</th>
                                    <th scope="col">Coverage now</th>
                                    <th scope="col">Attempts</th>
                                    <th scope="col">Scanner</th>
                                    <th scope="col">Tracker</th>
                                    <th scope="col">Status</th>
                                </tr>
                            </thead>
                            <tbody>
                                {chain.sessions.map(session => {
                                    const detail = sessionDetail(session);
                                    return (
                                        <Fragment key={session.session}>
                                            <tr
                                                className={`data-source-chain__row${detail ? ' has-detail' : ''}`}
                                                data-testid={`chain-row-${session.session}`}
                                            >
                                                <td className="data-source-mono">
                                                    <time dateTime={session.session}>{formatSessionDate(session.session)}</time>
                                                </td>
                                                <td className="data-source-mono">
                                                    {session.bars_coverage_now_pct === null
                                                        ? 'not computed'
                                                        : formatPercent(session.bars_coverage_now_pct)}
                                                </td>
                                                <td className="data-source-mono">{session.attempts}</td>
                                                <td>{doneText(session.scanner_completed)}</td>
                                                <td>{doneText(session.tracker_completed)}</td>
                                                <StatusCell session={session} />
                                            </tr>
                                            {detail && (
                                                <tr
                                                    className={`data-source-chain__detail-row is-${session.status}`}
                                                    data-testid={`chain-detail-${session.session}`}
                                                >
                                                    <td colSpan={COLUMNS}>
                                                        <p className="data-source-chain__detail">
                                                            <span className="data-source-chain__detail-label">{detail.label}:</span>{' '}
                                                            {detail.text}
                                                        </p>
                                                    </td>
                                                </tr>
                                            )}
                                        </Fragment>
                                    );
                                })}
                            </tbody>
                        </table>
                    </div>
                )}

                <p className="data-source-muted data-source-chain__footnote" data-testid="coverage-note">
                    Coverage now is computed at request time; it can differ from the coverage that gated that session's run if
                    bars were corrected or backfilled since.
                </p>
            </>
        )}
    </CollapsibleCard>
);

export default DailyChainSection;
