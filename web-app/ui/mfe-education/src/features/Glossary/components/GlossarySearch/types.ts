export interface GlossarySearchProps {
    value: string;
    onChange: (value: string) => void;
    resultCount: number;
    totalCount: number;
}
