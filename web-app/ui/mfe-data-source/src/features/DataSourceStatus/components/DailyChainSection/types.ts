import { DailyChainStatus, SectionUnavailable } from '@/api';

export interface DailyChainSectionProps {
    chain: DailyChainStatus | SectionUnavailable;
}
