import { FilterChipProps } from './types';
import './FilterChip-styles.css';

/** A toggle button for one filter value; active chips use the primary container (Stitch "active filter"). */
const FilterChip = ({ label, pressed, onToggle, 'data-testid': testId }: FilterChipProps) => (
    <button
        type="button"
        className={`alarm-chip${pressed ? ' is-active' : ''}`}
        aria-pressed={pressed}
        onClick={onToggle}
        data-testid={testId}
    >
        {label}
    </button>
);

export default FilterChip;
