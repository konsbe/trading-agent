import { array, bool, nullable, obj, oneOf, optBool, optNum, optStr, str, timestamp, tradingDay } from '../parse';
import { CatalystTier, SymbolSearchResponse, SymbolSearchResult, WatchlistItem, WatchlistResponse } from './types';

const catalystTier = oneOf<CatalystTier>(['A', 'B', 'none']);

const parseWatchlistItem = (value: unknown, path: string): WatchlistItem => {
    const o = obj(value, path);
    return {
        symbol: str(o.symbol, `${path}.symbol`),
        company_name: optStr(o.company_name, `${path}.company_name`),
        exchange: optStr(o.exchange, `${path}.exchange`),
        added_at: timestamp(o.added_at, `${path}.added_at`),
        as_of: nullable(tradingDay)(o.as_of, `${path}.as_of`),
        is_stale: bool(o.is_stale, `${path}.is_stale`),
        close: optNum(o.close, `${path}.close`),
        change_pct: optNum(o.change_pct, `${path}.change_pct`),
        rvol_20: optNum(o.rvol_20, `${path}.rvol_20`),
        dollar_volume: optNum(o.dollar_volume, `${path}.dollar_volume`),
        rsi_14: optNum(o.rsi_14, `${path}.rsi_14`),
        breakout_state: optStr(o.breakout_state, `${path}.breakout_state`),
        pct_of_52w_high: optNum(o.pct_of_52w_high, `${path}.pct_of_52w_high`),
        catalyst_tier: nullable(catalystTier)(o.catalyst_tier, `${path}.catalyst_tier`),
        market_cap: optNum(o.market_cap, `${path}.market_cap`),
        market_cap_est: optNum(o.market_cap_est, `${path}.market_cap_est`),
        market_cap_is_proxy: optBool(o.market_cap_is_proxy, `${path}.market_cap_is_proxy`),
        momentum_score_100: optNum(o.momentum_score_100, `${path}.momentum_score_100`),
        score_attainable: optNum(o.score_attainable, `${path}.score_attainable`),
        // A build-level fact; an older server that omits it is still unvalidated.
        score_status: optStr(o.score_status, `${path}.score_status`) ?? 'unvalidated',
        is_candidate_today: optBool(o.is_candidate_today, `${path}.is_candidate_today`) ?? false,
    };
};

export const parseWatchlist = (value: unknown): WatchlistResponse => {
    const o = obj(value, '$');
    return {
        owner: str(o.owner, 'owner'),
        items: array(o.items, 'items', parseWatchlistItem),
    };
};

const parseSymbolSearchResult = (value: unknown, path: string): SymbolSearchResult => {
    const o = obj(value, path);
    return {
        symbol: str(o.symbol, `${path}.symbol`),
        company_name: optStr(o.company_name, `${path}.company_name`),
        exchange: optStr(o.exchange, `${path}.exchange`),
        is_eligible: bool(o.is_eligible, `${path}.is_eligible`),
    };
};

export const parseSymbolSearch = (value: unknown): SymbolSearchResponse => {
    const o = obj(value, '$');
    return {
        query: str(o.query, 'query'),
        results: array(o.results, 'results', parseSymbolSearchResult),
    };
};
