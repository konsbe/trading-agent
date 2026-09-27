import { useCallback, useState } from 'react';
import { SymbolSearchResult } from '@/api';
import Tag from '@/components/Tag';
import useSymbolSearch from '@/hooks/watchlist/useSymbolSearch';
import SearchPanel from '../SearchPanel';
import SearchResultRow from '../SearchResultRow';
import { followErrorOf, TrackingSearchProps } from '../types';

const keyOf = (result: SymbolSearchResult) => result.symbol;

/** Search A — the scanner universe (`/api/v1/symbols`): the US common stocks the scanner covers. */
const UniverseSearch = ({ isFollowed, saving, errors, onFollow }: TrackingSearchProps) => {
    const [query, setQuery] = useState('');
    const search = useSymbolSearch(query);

    const renderResult = useCallback(
        (result: SymbolSearchResult) => (
            <SearchResultRow
                symbol={result.symbol}
                name={result.company_name}
                facts={[result.exchange]}
                markers={
                    <>
                        {!result.is_eligible && (
                            <Tag title="In the universe but outside the scanner's eligibility filters, so it has no scanner price row">not scanned</Tag>
                        )}
                        {isFollowed(result.symbol) && <Tag tone="accent">Following</Tag>}
                    </>
                }
                isFollowed={isFollowed(result.symbol)}
                isSaving={saving.has(result.symbol)}
                followError={followErrorOf(errors, result.symbol)}
                onFollow={onFollow}
            />
        ),
        [errors, isFollowed, onFollow, saving]
    );

    return (
        <SearchPanel
            id="search-universe"
            persistKey="watchlist.followed.search-universe"
            title="Scanner universe"
            label="Search the scanner universe"
            placeholder="Ticker or company name"
            hint="US common stocks the momentum scanner covers."
            query={query}
            onQueryChange={setQuery}
            search={search}
            getKey={keyOf}
            renderResult={renderResult}
        />
    );
};

export default UniverseSearch;
