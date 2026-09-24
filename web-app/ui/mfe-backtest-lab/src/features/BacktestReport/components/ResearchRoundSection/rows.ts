import { ResearchRound } from '@/api';
import { formatEffect } from '../../utils/format';
import { RoundRow } from './types';

export const ABANDONED_VERDICT = 'Abandoned before testing';
export const NO_EFFECT = '—';

/** Tested and abandoned hypotheses as one list, in id order (a, b, c, …). */
export const buildRoundRows = ({ hypotheses, abandoned }: ResearchRound): RoundRow[] =>
    [
        ...hypotheses.map(h => ({
            id: h.id,
            label: h.label,
            reason: h.reason,
            effect: formatEffect(h.best_effect.odds_ratio, h.best_effect.ci),
            verdict: h.verdict,
            verdictNote: h.verdict_note,
        })),
        ...abandoned.map(a => ({
            id: a.id,
            label: a.label,
            reason: a.reason,
            effect: NO_EFFECT,
            verdict: ABANDONED_VERDICT,
        })),
    ].sort((x, y) => x.id.localeCompare(y.id));
