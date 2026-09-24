import { SymbolFacts, SymbolScore } from '@/api';

export interface ScoreBreakdownProps {
    score: SymbolScore;
    /** Used to explain each sub-score and penalty in plain language. */
    facts: SymbolFacts;
}
