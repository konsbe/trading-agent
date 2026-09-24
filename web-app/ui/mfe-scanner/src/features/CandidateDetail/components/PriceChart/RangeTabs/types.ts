import { BarsRange } from '@/api';

export interface RangeTabsProps {
    value: BarsRange;
    onChange: (range: BarsRange) => void;
    label?: string;
    disabled?: boolean;
}
