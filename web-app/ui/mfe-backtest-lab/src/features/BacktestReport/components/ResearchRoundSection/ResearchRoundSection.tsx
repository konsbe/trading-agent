import { Fragment, useCallback, useState } from 'react';
import { Button, ChevronDownIcon, CollapsibleCard } from '@trading-agent/shared-components';
import { formatInteger } from '../../utils/format';
import SampleNote from '../SampleNote';
import { buildRoundRows } from './rows';
import { ResearchRoundSectionProps } from './types';
import '@/styles/backtest-global.css';
import './ResearchRoundSection-styles.css';

/**
 * Section 4 — every hypothesis of the round, tested or abandoned, in one table.
 * Verdicts are plain text of equal weight; a `verdict_note` sits behind a
 * neutral disclosure. Open notes live here, above the card, so they survive a
 * collapse.
 */
const ResearchRoundSection = ({ round, sampleSize }: ResearchRoundSectionProps) => {
    const [openNotes, setOpenNotes] = useState<ReadonlySet<string>>(() => new Set());
    const toggleNote = useCallback(
        (id: string) =>
            setOpenNotes(prev => {
                const next = new Set(prev);
                if (next.has(id)) next.delete(id);
                else next.add(id);
                return next;
            }),
        []
    );
    const rows = buildRoundRows(round);

    return (
        <CollapsibleCard
            id="backtest-round1"
            persistKey="backtest.round1"
            className="backtest-round"
            data-testid="round1-section"
            title={round.title}
        >
            <div className="backtest-round__scroll">
                <table className="backtest-round__table" data-testid="round1-table">
                    <thead>
                        <tr>
                            <th scope="col">Hypothesis</th>
                            <th scope="col">Effect · OR [95% CI]</th>
                            <th scope="col">Verdict</th>
                        </tr>
                    </thead>
                    <tbody>
                        {rows.map(row => {
                            const noteId = `backtest-round1-note-${row.id}`;
                            const isOpen = openNotes.has(row.id);
                            return (
                                <Fragment key={row.id}>
                                    <tr className="backtest-round__row" data-testid={`round1-row-${row.id}`}>
                                        <td className="backtest-round__hypothesis">
                                            <p className="backtest-round__label">
                                                <span className="backtest-round__id backtest-mono">({row.id})</span>{' '}
                                                {row.label}
                                            </p>
                                            <p className="backtest-muted backtest-round__reason" data-testid={`round1-reason-${row.id}`}>
                                                {row.reason}
                                            </p>
                                        </td>
                                        <td className="backtest-round__effect backtest-mono" data-testid={`round1-effect-${row.id}`}>
                                            {row.effect}
                                        </td>
                                        <td className="backtest-round__verdict-cell">
                                            <span className="backtest-round__verdict" data-testid={`round1-verdict-${row.id}`}>
                                                {row.verdict}
                                            </span>
                                            {row.verdictNote && (
                                                <Button
                                                    variant="ghost"
                                                    size="sm"
                                                    className={`backtest-round__note-toggle${isOpen ? ' is-open' : ''}`}
                                                    aria-expanded={isOpen}
                                                    aria-controls={noteId}
                                                    onClick={() => toggleNote(row.id)}
                                                    data-testid={`round1-note-toggle-${row.id}`}
                                                >
                                                    Note
                                                    <span className="backtest-sr-only"> on hypothesis {row.id}</span>
                                                    <ChevronDownIcon size={14} className="backtest-round__note-chevron" />
                                                </Button>
                                            )}
                                        </td>
                                    </tr>
                                    {row.verdictNote && (
                                        <tr
                                            id={noteId}
                                            className="backtest-round__note-row"
                                            hidden={!isOpen}
                                            data-testid={`round1-note-${row.id}`}
                                        >
                                            <td colSpan={3}>
                                                <p className="backtest-round__note">{row.verdictNote}</p>
                                            </td>
                                        </tr>
                                    )}
                                </Fragment>
                            );
                        })}
                    </tbody>
                </table>
            </div>

            {sampleSize && (
                <SampleNote data-testid="round1-sample">
                    {formatInteger(sampleSize.episodes)} episodes · excludes {sampleSize.excludes} ·{' '}
                    {sampleSize.lockbox_opened ? 'lockbox opened' : 'lockbox not opened'}
                </SampleNote>
            )}
        </CollapsibleCard>
    );
};

export default ResearchRoundSection;
