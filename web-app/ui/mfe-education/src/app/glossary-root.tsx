import '@trading-agent/shared-components/theme.css';
import GlossaryPage from '@/pages/GlossaryPage';
import HostedAppWrapper from './wrapper';

/**
 * Exposed as `mfe_education/./Glossary`. Rendered inside spog's `glossary/*` route,
 * so it must not create its own Router.
 */
const Glossary = () => (
    <HostedAppWrapper>
        <GlossaryPage />
    </HostedAppWrapper>
);

export default Glossary;
