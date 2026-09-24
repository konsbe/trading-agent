import { Bucket } from '@/api';

export const BUCKET_LABELS: Record<Bucket, string> = {
    market: 'Market',
    penny: 'Penny',
};

/** Rows shown per bucket before "and N more" — a display default, not a data limit. */
export const WINDOW_SIZE = 10;
