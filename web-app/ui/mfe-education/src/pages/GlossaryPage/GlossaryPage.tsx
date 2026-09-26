import { useMemo, useState } from 'react';
import { MFEDataWrapper } from '@trading-agent/shared-components';
import ApiErrorState from '@/components/ApiErrorState';
import DocSkeleton from '@/components/DocSkeleton';
import PageLayout from '@/components/PageLayout';
import { getErrorMessage } from '@/common/errors/errorMessages';
import GlossaryList from '@/features/Glossary/components/GlossaryList';
import GlossarySearch from '@/features/Glossary/components/GlossarySearch';
import { filterTerms, sortTerms } from '@/features/Glossary/utils/glossary';
import { useGlossary } from '@/hooks/education';
import { useIsHosted } from '@/providers/HostModeContext';
import './GlossaryPage-styles.css';

/** Terms are extracted from Handbook and MasterClass as sections are written, so none is expected today. */
export const EMPTY_GLOSSARY_MESSAGE = 'Glossary terms are added as Handbook and MasterClass sections are written';

export const noResultsMessage = (query: string): string => `No terms match “${query.trim()}”.`;

/** MFEDataWrapper only takes a style object for its empty-state stack. */
const EMPTY_STATE_STYLE = { padding: 'var(--space-lg) 0' };

/**
 * Search-first flat list of every term, alphabetised; each row links to its
 * Handbook or MasterClass entry. Hosted, spog's header shows the title.
 */
const GlossaryPage = () => {
    const isHosted = useIsHosted();
    const { data: glossary, error, isLoading, reload } = useGlossary();
    const [query, setQuery] = useState('');

    const sorted = useMemo(() => sortTerms(glossary?.terms ?? []), [glossary]);
    const results = useMemo(() => filterTerms(sorted, query), [sorted, query]);

    return (
        <PageLayout title={isHosted ? undefined : 'Glossary'}>
            {error && <ApiErrorState error={error} message={getErrorMessage(error, 'the Glossary')} onRetry={reload} />}
            {!error && isLoading && !glossary && <DocSkeleton label="Loading Glossary" />}
            {!error && glossary && (
                <div className="education-glossary-page">
                    <MFEDataWrapper
                        data={sorted}
                        noDataMessage={EMPTY_GLOSSARY_MESSAGE}
                        showEmptyIllustration={false}
                        emptyStateStackStyle={EMPTY_STATE_STYLE}
                    >
                        <GlossarySearch value={query} onChange={setQuery} resultCount={results.length} totalCount={sorted.length} />
                        <MFEDataWrapper
                            data={results}
                            noDataMessage={noResultsMessage(query)}
                            showEmptyIllustration={false}
                            emptyStateStackStyle={EMPTY_STATE_STYLE}
                        >
                            <GlossaryList terms={results} />
                        </MFEDataWrapper>
                    </MFEDataWrapper>
                </div>
            )}
        </PageLayout>
    );
};

export default GlossaryPage;
