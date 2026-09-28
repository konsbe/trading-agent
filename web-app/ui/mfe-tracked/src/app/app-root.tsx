import '@trading-agent/shared-components/theme.css';
import AppRouter from '@/router/AppRouter';
import HostedAppWrapper from './wrapper';

/**
 * Exposed as `mfe_tracked/./TrackedPositions`. Rendered inside spog's router,
 * so it must not create its own Router.
 */
const TrackedPositions = () => (
    <HostedAppWrapper>
        <AppRouter />
    </HostedAppWrapper>
);

export default TrackedPositions;
