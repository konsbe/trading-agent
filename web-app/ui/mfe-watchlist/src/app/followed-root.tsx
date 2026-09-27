import '@trading-agent/shared-components/theme.css';
import FollowedSymbolsPage from '@/pages/FollowedSymbolsPage';
import HostedAppWrapper from './wrapper';

/**
 * Exposed as `mfe_watchlist/./FollowedSymbols`. Rendered inside spog's
 * `followed-symbols/*` route, so it must not create its own Router.
 */
const FollowedSymbols = () => (
    <HostedAppWrapper>
        <FollowedSymbolsPage />
    </HostedAppWrapper>
);

export default FollowedSymbols;
