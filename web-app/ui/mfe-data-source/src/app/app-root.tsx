import '@trading-agent/shared-components/theme.css';
import AppRouter from '@/router/AppRouter';
import HostedAppWrapper from './wrapper';

/**
 * Exposed as `mfe_data_source/./DataSource`. Rendered inside spog's router, so it
 * must not create its own Router.
 */
const DataSource = () => (
    <HostedAppWrapper>
        <AppRouter />
    </HostedAppWrapper>
);

export default DataSource;
