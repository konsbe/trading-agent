import { BarsRange } from '@/api';

export interface PriceChartProps {
    symbol: string;
    /** Range selected on first render; the user can change it. */
    defaultRange?: BarsRange;
}
