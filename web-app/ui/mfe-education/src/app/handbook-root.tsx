import '@trading-agent/shared-components/theme.css';
import HandbookPage from '@/pages/HandbookPage';
import HostedAppWrapper from './wrapper';

/**
 * Exposed as `mfe_education/./Handbook`. Rendered inside spog's `handbook/*` route,
 * so it must not create its own Router.
 */
const Handbook = () => (
    <HostedAppWrapper>
        <HandbookPage />
    </HostedAppWrapper>
);

export default Handbook;
