import PageLayout from '@/components/PageLayout';
import { useIsHosted } from '@/providers/HostModeContext';

/** Placeholder. Content comes from `GET /api/v1/education/handbook`. Hosted, spog's header shows the title. */
const HandbookPage = () => {
    const isHosted = useIsHosted();

    return (
        <PageLayout title={isHosted ? undefined : 'Handbook'}>
            <p>Handbook content coming.</p>
        </PageLayout>
    );
};

export default HandbookPage;
