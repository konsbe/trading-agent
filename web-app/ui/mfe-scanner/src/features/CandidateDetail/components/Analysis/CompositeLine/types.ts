import { ScoreTier } from '@/api';
import { BandDisplayMap } from '../../../utils/analysisFormat';

export interface CompositeLineProps {
    composite: ScoreTier;
    /** Display text for specific tier codes; unmapped tiers are humanized. */
    labels?: BandDisplayMap;
    /** Replaces the score/tier text entirely (e.g. "not evaluated"). */
    valueText?: string;
    'data-testid'?: string;
}
