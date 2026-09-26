import '@trading-agent/shared-components/theme.css';
import MasterClassPage from '@/pages/MasterClassPage';
import HostedAppWrapper from './wrapper';

/**
 * Exposed as `mfe_education/./MasterClass`. Rendered inside spog's `masterclass/*` route,
 * so it must not create its own Router.
 */
const MasterClass = () => (
    <HostedAppWrapper>
        <MasterClassPage />
    </HostedAppWrapper>
);

export default MasterClass;
