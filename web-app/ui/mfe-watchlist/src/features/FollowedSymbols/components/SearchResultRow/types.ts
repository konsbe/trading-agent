import { ReactNode } from 'react';
import { ApiErrorShape } from '@/api';

export interface SearchResultRowProps {
    symbol: string;
    /** `equity` | `etf` | `crypto`; omitted for scanner-universe results (all stocks). Crypto isn't linked to Stock Detail. */
    assetType?: string | null;
    name: string | null;
    /** Joined with " · "; empty values are skipped. */
    facts: (string | null)[];
    /** Tags after the facts (in universe, following, not scanned). */
    markers?: ReactNode;
    isFollowed: boolean;
    isSaving: boolean;
    followError: ApiErrorShape | null;
    onFollow: (symbol: string) => void;
}
