import { CollapsibleCard } from '@trading-agent/shared-components';
import { isSectionUnavailable, PROVIDER_DISPLAY_ORDER } from '@/api';
import { isProviderOverBudget } from '../../utils/status';
import ProviderCard from '../ProviderCard';
import SectionUnavailable from '../SectionUnavailable';
import { ProvidersSectionProps } from './types';
import './ProvidersSection-styles.css';

/** Section 2 — one card per provider, in the server's display order. */
const ProvidersSection = ({ providers, reasons }: ProvidersSectionProps) => (
    <CollapsibleCard
        id="data-source-providers"
        persistKey="datasource.providers"
        title="Providers"
        data-testid="providers-section"
    >
        {isSectionUnavailable(providers) ? (
            <SectionUnavailable
                data-testid="providers-unavailable"
                explanation="The status service couldn't read the provider budgets on this check."
            />
        ) : (
            <div className="data-source-providers">
                {PROVIDER_DISPLAY_ORDER.map(key => (
                    <ProviderCard
                        key={key}
                        providerKey={key}
                        provider={providers[key]}
                        overBudget={isProviderOverBudget(key, reasons)}
                    />
                ))}
            </div>
        )}
    </CollapsibleCard>
);

export default ProvidersSection;
