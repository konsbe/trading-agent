import '@trading-agent/shared-components/theme.css';
import ComputedSymbolsPage from '@/pages/ComputedSymbolsPage';
import HostedAppWrapper from './wrapper';

/**
 * Exposed as `mfe_watchlist/./ComputedSymbols`. Rendered inside spog's
 * `computed-symbols/*` route, so it must not create its own Router.
 */
const ComputedSymbols = () => (
    <HostedAppWrapper>
        <ComputedSymbolsPage />
    </HostedAppWrapper>
);

export default ComputedSymbols;
