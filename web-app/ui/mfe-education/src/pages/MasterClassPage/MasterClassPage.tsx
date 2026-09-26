import PageLayout from '@/components/PageLayout';
import { useIsHosted } from '@/providers/HostModeContext';

/** Placeholder. Content comes from `GET /api/v1/education/masterclass`. Hosted, spog's header shows the title. */
const MasterClassPage = () => {
    const isHosted = useIsHosted();

    return (
        <PageLayout title={isHosted ? undefined : 'MasterClass'}>
            <p>MasterClass content coming.</p>
        </PageLayout>
    );
};

export default MasterClassPage;
