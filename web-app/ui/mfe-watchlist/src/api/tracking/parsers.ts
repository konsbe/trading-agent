import { array, bool, num, obj, oneOf, optStr, str, timestamp } from '../parse';
import {
    AssetType,
    COMPUTE_REASONS,
    COMPUTE_STATES,
    ComputedSymbol,
    ComputedSymbolsResponse,
    DirectoryResult,
    DirectorySearchResponse,
    DirectorySource,
    FollowedSymbol,
    FollowedSymbolsResponse,
    FollowSource,
    Listing,
} from './types';

const assetType = oneOf<AssetType>(['equity', 'etf', 'crypto']);
const listing = oneOf<Listing>(['us', 'foreign', 'crypto']);
const followSource = oneOf<FollowSource>(['env_seed', 'user']);
const directorySource = oneOf<DirectorySource>(['finnhub_us', 'binance_spot']);
const computeState = oneOf(COMPUTE_STATES);
const computeReason = oneOf(COMPUTE_REASONS);

const optTimestamp = (value: unknown, path: string): string | null =>
    value === null || value === undefined ? null : timestamp(value, path);

const parseFollowedSymbol = (value: unknown, path: string): FollowedSymbol => {
    const o = obj(value, path);
    return {
        symbol: str(o.symbol, `${path}.symbol`),
        name: optStr(o.name, `${path}.name`),
        asset_type: assetType(o.asset_type, `${path}.asset_type`),
        listing: listing(o.listing, `${path}.listing`),
        news_alias: optStr(o.news_alias, `${path}.news_alias`),
        source: followSource(o.source, `${path}.source`),
        added_at: timestamp(o.added_at, `${path}.added_at`),
    };
};

export const parseFollowedSymbols = (value: unknown): FollowedSymbolsResponse => {
    const o = obj(value, '$');
    return { items: array(o.items, 'items', parseFollowedSymbol) };
};

const parseDirectoryResult = (value: unknown, path: string): DirectoryResult => {
    const o = obj(value, path);
    return {
        symbol: str(o.symbol, `${path}.symbol`),
        name: optStr(o.name, `${path}.name`),
        type: optStr(o.type, `${path}.type`),
        mic: optStr(o.mic, `${path}.mic`),
        asset_type: assetType(o.asset_type, `${path}.asset_type`),
        source: directorySource(o.source, `${path}.source`),
        in_universe: bool(o.in_universe, `${path}.in_universe`),
        followed: bool(o.followed, `${path}.followed`),
    };
};

export const parseDirectorySearch = (value: unknown): DirectorySearchResponse => {
    const o = obj(value, '$');
    return {
        query: str(o.query, 'query'),
        results: array(o.results, 'results', parseDirectoryResult),
    };
};

const parseComputedSymbol = (value: unknown, path: string): ComputedSymbol => {
    const o = obj(value, path);
    return {
        symbol: str(o.symbol, `${path}.symbol`),
        name: optStr(o.name, `${path}.name`),
        asset_type: assetType(o.asset_type, `${path}.asset_type`),
        reasons: array(o.reasons, `${path}.reasons`, computeReason),
        manual_requested_at: optTimestamp(o.manual_requested_at, `${path}.manual_requested_at`),
        state: computeState(o.state, `${path}.state`),
        bars_fetched_at: optTimestamp(o.bars_fetched_at, `${path}.bars_fetched_at`),
        fundamentals_fetched_at: optTimestamp(o.fundamentals_fetched_at, `${path}.fundamentals_fetched_at`),
        computed_at: optTimestamp(o.computed_at, `${path}.computed_at`),
        last_error: optStr(o.last_error, `${path}.last_error`),
        statements_status: optStr(o.statements_status, `${path}.statements_status`),
        statements_reason: optStr(o.statements_reason, `${path}.statements_reason`),
    };
};

export const parseComputedSymbols = (value: unknown): ComputedSymbolsResponse => {
    const o = obj(value, '$');
    return {
        data_timeout_minutes: num(o.data_timeout_minutes, 'data_timeout_minutes'),
        items: array(o.items, 'items', parseComputedSymbol),
    };
};
