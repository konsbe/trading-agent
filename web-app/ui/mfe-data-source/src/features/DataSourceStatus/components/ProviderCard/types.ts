import { ProviderStatus } from '@/api';

export interface ProviderCardProps {
    providerKey: string;
    /** Absent when the server has no budget row for the provider. */
    provider?: ProviderStatus;
    /** Named in `overall_reasons` for its budget: the bar may switch to the warning status token. */
    overBudget: boolean;
}
