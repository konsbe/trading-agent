import { AssetType, ComputeReason, FollowSource, Listing } from '@/api';

export const ASSET_TYPE_LABELS: Record<AssetType, string> = {
    equity: 'Equity',
    etf: 'ETF',
    crypto: 'Crypto',
};

export const LISTING_LABELS: Record<Listing, string> = {
    us: 'US',
    foreign: 'Foreign',
    crypto: 'Crypto',
};

export const SOURCE_LABELS: Record<FollowSource, string> = {
    env_seed: 'Seeded from .env',
    user: 'Added by you',
};

export const REASON_LABELS: Record<ComputeReason, string> = {
    followed: 'Followed',
    watchlist: 'Watchlist',
    candidate: "Today's candidate",
    manual: 'Manual',
};

export const REASON_DESCRIPTIONS: Record<ComputeReason, string> = {
    followed: 'On the Followed Symbols list',
    watchlist: 'On the watchlist',
    candidate: "In today's scanner candidates",
    manual: 'You pressed Compute',
};

/** Unknown server values fall through verbatim rather than disappearing. */
export const labelOf = <K extends string>(labels: Record<K, string>, value: string): string =>
    (labels as Record<string, string>)[value] ?? value;
