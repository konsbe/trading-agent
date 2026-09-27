import { FollowedSymbol } from '@/api';
import { FollowError } from '@/hooks/tracking/useFollowedSymbols';

export interface FollowedTableProps {
    items: FollowedSymbol[];
    /** Symbols with a follow / unfollow in flight. */
    saving: ReadonlySet<string>;
    errors: ReadonlyMap<string, FollowError>;
    onUnfollow: (symbol: string) => void;
}
