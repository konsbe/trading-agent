import { useCallback, useState } from 'react';
import { DirectoryResult } from '@/api';
import { ASSET_TYPE_LABELS, labelOf } from '@/common/format/trackingLabels';
import Tag from '@/components/Tag';
import useDirectorySearch from '@/hooks/tracking/useDirectorySearch';
import SearchPanel from '../SearchPanel';
import SearchResultRow from '../SearchResultRow';
import { followErrorOf, TrackingSearchProps } from '../types';

const keyOf = (result: DirectoryResult) => `${result.source}:${result.symbol}`;

/** The provider's type, unless it only repeats the asset type (crypto "spot"). */
const facts = (result: DirectoryResult): (string | null)[] =>
    result.source === 'binance_spot'
        ? [labelOf(ASSET_TYPE_LABELS, result.asset_type), 'Binance spot']
        : [result.type, labelOf(ASSET_TYPE_LABELS, result.asset_type), result.mic ? `venue ${result.mic}` : null];

/** Search B — every symbol in the directory (`/api/v1/symbols/directory`): ETFs, ADRs, OTC and crypto. */
const DirectorySearch = ({ isFollowed, saving, errors, onFollow }: TrackingSearchProps) => {
    const [query, setQuery] = useState('');
    const search = useDirectorySearch(query);

    const renderResult = useCallback(
        (result: DirectoryResult) => {
            const followed = result.followed || isFollowed(result.symbol);
            return (
                <SearchResultRow
                    symbol={result.symbol}
                    assetType={result.asset_type}
                    name={result.name}
                    facts={facts(result)}
                    markers={
                        <>
                            {result.in_universe && <Tag title="An eligible symbol in the scanner universe: the momentum scanner covers it">In scanner universe</Tag>}
                            {followed && <Tag tone="accent">Following</Tag>}
                        </>
                    }
                    isFollowed={followed}
                    isSaving={saving.has(result.symbol)}
                    followError={followErrorOf(errors, result.symbol)}
                    onFollow={onFollow}
                />
            );
        },
        [errors, isFollowed, onFollow, saving]
    );

    return (
        <SearchPanel
            id="search-directory"
            persistKey="watchlist.followed.search-directory"
            title="All symbols"
            label="Search all symbols — ETFs, ADRs, OTC and crypto"
            placeholder="Ticker or name, e.g. DIA, EWJ, SOLUSDT"
            hint="US-listed symbols of every type plus Binance spot pairs. Foreign listings (e.g. 2222.SR) can't be found by either search yet."
            query={query}
            onQueryChange={setQuery}
            search={search}
            getKey={keyOf}
            renderResult={renderResult}
        />
    );
};

export default DirectorySearch;
