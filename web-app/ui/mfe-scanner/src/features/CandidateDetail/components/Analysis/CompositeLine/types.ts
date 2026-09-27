import { ScoreTier } from '@/api';

export interface CompositeLineProps {
    composite: ScoreTier;
    /** Replaces the score/tier text entirely (e.g. a served label). */
    valueText?: string;
    'data-testid'?: string;
}
