import '@trading-agent/shared-components/theme.css';
import AppRouter from '@/router/AppRouter';
import HostedAppWrapper from './wrapper';

/**
 * Exposed as `mfe_scanner/./Scanner`. Rendered inside spog's router, so it
 * must not create its own Router.
 */
const Scanner = () => (
    <HostedAppWrapper>
        <AppRouter />
    </HostedAppWrapper>
);

export default Scanner;
