export type IndicatorTone = 'constructive' | 'neutral' | 'stressed' | 'no_data';

export interface ToneIndicatorProps {
    tone: string | null | undefined;
    size?: number;
    'data-testid'?: string;
}
