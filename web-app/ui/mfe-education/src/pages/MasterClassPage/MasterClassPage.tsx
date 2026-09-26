import ApiErrorState from '@/components/ApiErrorState';
import ContentStatus from '@/components/ContentStatus';
import DocLayout, { JumpNavGroup } from '@/components/DocLayout';
import DocSkeleton from '@/components/DocSkeleton';
import PageLayout from '@/components/PageLayout';
import { MasterClass } from '@/api';
import { getErrorMessage } from '@/common/errors/errorMessages';
import MasterClassModule, { moduleLabel } from '@/features/MasterClass/components/MasterClassModule';
import { useMasterClass } from '@/hooks/education';
import useHashScroll from '@/hooks/useHashScroll';
import { useIsHosted } from '@/providers/HostModeContext';

const toNavGroups = (masterClass: MasterClass): JumpNavGroup[] =>
    masterClass.modules.map(module => {
        const label = moduleLabel(module.number);
        return {
            id: module.id,
            title: label ? `${label} · ${module.title}` : module.title,
            items: module.entries.map(entry => ({ id: entry.id, title: entry.title })),
        };
    });

/**
 * "What does the stock market itself mean by this?" — modules of entries, each
 * summary-first with a collapsible full explanation, and a jump-nav;
 * `/masterclass#<entry-id>` scrolls to the entry. Hosted, spog's header shows the title.
 */
const MasterClassPage = () => {
    const isHosted = useIsHosted();
    const { data: masterClass, error, isLoading, reload } = useMasterClass();
    useHashScroll(Boolean(masterClass));

    return (
        <PageLayout title={isHosted ? undefined : 'MasterClass'}>
            {error && <ApiErrorState error={error} message={getErrorMessage(error, 'the MasterClass')} onRetry={reload} />}
            {!error && isLoading && !masterClass && <DocSkeleton label="Loading MasterClass" />}
            {!error && masterClass && (
                <DocLayout navLabel="MasterClass contents" groups={toNavGroups(masterClass)}>
                    <ContentStatus status={masterClass.status} version={masterClass.version} />
                    {masterClass.modules.map(module => (
                        <MasterClassModule key={module.id} module={module} />
                    ))}
                </DocLayout>
            )}
        </PageLayout>
    );
};

export default MasterClassPage;
