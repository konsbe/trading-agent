import '@trading-agent/shared-components/theme.css';
import AppRouter from '@/router/AppRouter';
import HostedAppWrapper from './wrapper';

/**
 * Exposed as `mfe_market_report/./MarketReport`. Rendered inside spog's router, so it
 * must not create its own Router.
 */
const MarketReport = () => (
    <HostedAppWrapper>
        <AppRouter />
    </HostedAppWrapper>
);

export default MarketReport;
