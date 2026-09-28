import { ReactNode } from 'react';
import { TableColumn, TableHeaderProps } from '@trading-agent/shared-components';
import { Candidate } from '@/api';

export interface CandidateColumn extends TableColumn<Candidate> {
    numeric: boolean;
    /** Header tooltip. */
    description?: string;
    render: (candidate: Candidate) => ReactNode;
}

export interface CandidatesTableProps {
    id: string;
    /** Accessible table name, e.g. "Market candidates". */
    caption: string;
    rows: Candidate[];
    /** `useTableView().headerProps`: sort state and handler per column. */
    headerProps: (key: string) => TableHeaderProps;
}
