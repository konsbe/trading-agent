import { Tone } from '@/api';

/** The tones that get an indicator; `display_only` rows show plain numbers. */
export type IndicatorTone = Exclude<Tone, 'display_only'>;

export interface ToneIndicatorProps {
    tone: Tone | null | undefined;
    size?: number;
    'data-testid'?: string;
}
