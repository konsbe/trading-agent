import { ResearchRound, SampleSize } from '@/api';

export interface ResearchRoundSectionProps {
    round: ResearchRound;
    /** The round's published sample; omitted when the report's sample belongs elsewhere. */
    sampleSize?: SampleSize;
}

export interface RoundRow {
    id: string;
    label: string;
    reason: string;
    effect: string;
    verdict: string;
    verdictNote?: string;
}
