import { Bucket } from '@/api';

export interface DetailTitleProps {
    symbol: string;
    companyName?: string | null;
}

export interface DetailMetaProps {
    exchange: string | null;
    bucket: Bucket | null;
    /** Scan session date, `YYYY-MM-DD`. */
    asOf: string;
}
