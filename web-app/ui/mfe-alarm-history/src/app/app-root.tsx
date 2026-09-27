import '@trading-agent/shared-components/theme.css';
import AppRouter from '@/router/AppRouter';
import HostedAppWrapper from './wrapper';

/**
 * Exposed as `mfe_alarm_history/./AlarmHistory`. Rendered inside spog's router,
 * so it must not create its own Router.
 */
const AlarmHistory = () => (
    <HostedAppWrapper>
        <AppRouter />
    </HostedAppWrapper>
);

export default AlarmHistory;
