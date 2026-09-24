import '@trading-agent/shared-components/theme.css';
import AppRouter from '@/router/AppRouter';
import HostedAppWrapper from './wrapper';

/**
 * Exposed as `mfe_backtest_lab/./BacktestLab`. Rendered inside spog's router, so it
 * must not create its own Router.
 */
const BacktestLab = () => (
    <HostedAppWrapper>
        <AppRouter />
    </HostedAppWrapper>
);

export default BacktestLab;
