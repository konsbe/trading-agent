import PageLayout from '@/components/PageLayout';
import { useIsHosted } from '@/providers/HostModeContext';

/** Placeholder. Content comes from `GET /api/v1/education/glossary`. Hosted, spog's header shows the title. */
const GlossaryPage = () => {
    const isHosted = useIsHosted();

    return (
        <PageLayout title={isHosted ? undefined : 'Glossary'}>
            <p>Glossary content coming.</p>
        </PageLayout>
    );
};

export default GlossaryPage;
