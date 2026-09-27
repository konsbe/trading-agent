export interface FilterChipProps {
    label: string;
    pressed: boolean;
    onToggle: () => void;
    'data-testid'?: string;
}
