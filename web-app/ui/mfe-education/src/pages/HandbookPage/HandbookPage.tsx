import ApiErrorState from '@/components/ApiErrorState';
import ContentStatus from '@/components/ContentStatus';
import DocLayout, { JumpNavGroup } from '@/components/DocLayout';
import DocSkeleton from '@/components/DocSkeleton';
import PageLayout from '@/components/PageLayout';
import { Handbook } from '@/api';
import { getErrorMessage } from '@/common/errors/errorMessages';
import HandbookSection from '@/features/Handbook/components/HandbookSection';
import { useHandbook } from '@/hooks/education';
import useHashScroll from '@/hooks/useHashScroll';
import { useIsHosted } from '@/providers/HostModeContext';

const toNavGroups = (handbook: Handbook): JumpNavGroup[] =>
    handbook.sections.map(section => ({
        id: section.id,
        title: section.title,
        items: section.entries.map(entry => ({ id: entry.id, title: entry.title })),
    }));

/**
 * "What does THIS APP mean when it shows me this?" — narrative sections with a
 * jump-nav; `/handbook#<entry-id>` scrolls to the entry. Hosted, spog's header
 * shows the title.
 */
const HandbookPage = () => {
    const isHosted = useIsHosted();
    const { data: handbook, error, isLoading, reload } = useHandbook();
    useHashScroll(Boolean(handbook));

    return (
        <PageLayout title={isHosted ? undefined : 'Handbook'}>
            {error && <ApiErrorState error={error} message={getErrorMessage(error, 'the Handbook')} onRetry={reload} />}
            {!error && isLoading && !handbook && <DocSkeleton label="Loading Handbook" />}
            {!error && handbook && (
                <DocLayout navLabel="Handbook contents" groups={toNavGroups(handbook)}>
                    <ContentStatus status={handbook.status} version={handbook.version} />
                    {handbook.sections.map(section => (
                        <HandbookSection key={section.id} section={section} />
                    ))}
                </DocLayout>
            )}
        </PageLayout>
    );
};

export default HandbookPage;
