import React from 'react';
import { formatScore } from '../../../format';
import { ScoreCellProps, ScoreFields } from './types';
import './MarketCells-styles.css';

/** Screen-reader text for a score cell: "Score 53 of 75 attainable, unvalidated" or "No score, unvalidated". */
export const scoreLabel = ({ momentum_score_100: score, score_attainable: ceiling, score_status: status }: ScoreFields): string => {
    if (score === null) return `No score, ${status}`;
    const ceilingText = ceiling === null ? '' : ` of ${formatScore(ceiling)} attainable`;
    return `Score ${formatScore(score)}${ceilingText}, ${status}`;
};

/**
 * "53/75 unvalidated": each row's own ceiling (never a fixed 75) so the score
 * doesn't read as out of 100, and the unvalidated marker travels with every
 * score in every row and sort order (spec §0). No score renders "— unvalidated".
 */
const ScoreCell = ({ row }: ScoreCellProps) => {
    const hasScore = row.momentum_score_100 !== null;
    const showCeiling = hasScore && row.score_attainable !== null;

    return (
        <span className="market-cell__score">
            <span className="market-cell__sr-only" data-testid="score-label">{scoreLabel(row)}</span>
            <span className="market-cell__score-visual" aria-hidden="true">
                <span className="market-cell__score-number">
                    <span className="market-cell__score-value" data-testid="score-value">
                        {formatScore(row.momentum_score_100)}
                    </span>
                    {showCeiling && (
                        <span className="market-cell__score-ceiling" data-testid="score-ceiling">
                            /{formatScore(row.score_attainable)}
                        </span>
                    )}
                </span>
                <span className="market-cell__score-status" data-testid="score-status">
                    {row.score_status}
                </span>
            </span>
        </span>
    );
};

export default ScoreCell;
