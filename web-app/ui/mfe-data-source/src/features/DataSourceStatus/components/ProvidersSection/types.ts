import { ProvidersStatus, SectionUnavailable } from '@/api';

export interface ProvidersSectionProps {
    providers: ProvidersStatus | SectionUnavailable;
    /** `overall_reasons`, to tell which provider is over its budget threshold. */
    reasons: string[];
}
