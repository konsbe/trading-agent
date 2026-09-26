import { ScoreTier } from '@/api';
import { BandDisplayMap } from '../../../utils/analysisFormat';

export interface CompositeLineProps {
    composite: ScoreTier;
    /** Display text for specific tier codes; unmapped tiers are humanized. */
    labels?: BandDisplayMap;
    'data-testid'?: string;
}
