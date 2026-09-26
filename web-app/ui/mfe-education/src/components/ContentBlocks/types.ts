import { ContentBlock } from '@/api';

export interface ContentBlocksProps {
    blocks: ContentBlock[];
    /** Level for `heading` blocks; one below the entry title. Default 4. */
    headingLevel?: 3 | 4 | 5;
}
