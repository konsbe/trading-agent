import { ApiError } from '@/api';
import { FollowError } from '@/hooks/tracking/useFollowedSymbols';

/** What both search panels need from the followed list. */
export interface TrackingSearchProps {
    isFollowed: (symbol: string) => boolean;
    saving: ReadonlySet<string>;
    errors: ReadonlyMap<string, FollowError>;
    onFollow: (symbol: string) => void;
}

/** A failed follow for this symbol (an unfollow failure belongs to the table). */
export const followErrorOf = (errors: ReadonlyMap<string, FollowError>, symbol: string): ApiError | null => {
    const failed = errors.get(symbol);
    return failed?.action === 'follow' ? failed.error : null;
};
