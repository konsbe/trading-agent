import { GlossaryTerm } from '@/api';

export interface GlossaryListProps {
    /** Already sorted and filtered. */
    terms: GlossaryTerm[];
}
